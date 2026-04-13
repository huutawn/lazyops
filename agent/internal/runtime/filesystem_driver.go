package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
)

type FilesystemDriver struct {
	logger         *slog.Logger
	root           string
	stateKey       string
	agentImageRef  string
	fetcher        AssetFetcher
	gateway        *GatewayManager
	sidecar        *SidecarManager
	mesh           *MeshManager
	processManager *ProcessManager
	now            func() time.Time
}

func NewFilesystemDriver(logger *slog.Logger, root string) *FilesystemDriver {
	pm := NewProcessManager(logger, root)
	sidecar := NewSidecarManager(logger, root).WithProcessManager(pm)
	return &FilesystemDriver{
		logger:         logger,
		root:           root,
		fetcher:        NewLocalCacheFetcher(filepath.Join(root, "cache", "assets")),
		gateway:        NewGatewayManager(logger, root),
		sidecar:        sidecar,
		mesh:           NewMeshManager(logger, root),
		processManager: pm,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (d *FilesystemDriver) WithStateEncryptionKey(key string) *FilesystemDriver {
	if d == nil {
		return d
	}
	d.stateKey = strings.TrimSpace(key)
	if d.processManager != nil {
		d.processManager.stateEncryptionKey = d.stateKey
	}
	return d
}

func (d *FilesystemDriver) WithAgentImageRef(imageRef string) *FilesystemDriver {
	if d == nil {
		return d
	}
	d.agentImageRef = strings.TrimSpace(imageRef)
	if d.processManager != nil {
		d.processManager.agentImageRef = d.agentImageRef
	}
	if d.sidecar != nil {
		d.sidecar.companionImageRef = d.agentImageRef
	}
	return d
}

func (d *FilesystemDriver) hydrateRuntimeContextFromWorkspace(layout WorkspaceLayout, runtimeCtx RuntimeContext) RuntimeContext {
	manifest, err := loadWorkspaceManifest(layout)
	if err != nil || len(manifest.Services) == 0 {
		return runtimeCtx
	}
	if strings.TrimSpace(manifest.Revision.RevisionID) != strings.TrimSpace(runtimeCtx.Revision.RevisionID) {
		return runtimeCtx
	}
	return withRuntimeServices(runtimeCtx, manifest.Services)
}

func (d *FilesystemDriver) assignRuntimePorts(runtimeCtx RuntimeContext) (RuntimeContext, error) {
	if runtimeCtx.Binding.RuntimeMode != contracts.RuntimeModeStandalone || len(runtimeCtx.Services) == 0 {
		return runtimeCtx, nil
	}

	services := append([]ServiceRuntimeContext(nil), runtimeCtx.Services...)
	reserved := make(map[int]string, len(services))
	for i := range services {
		service := &services[i]
		if !shouldAssignStandaloneRuntimePort(runtimeCtx, *service) {
			continue
		}

		declaredPort := declaredHealthcheckPort(*service)
		if declaredPort > 0 && !serviceLocalListenerPortConflicts(*service, declaredPort) && d.runtimePortAvailable(declaredPort, reserved) {
			service.RuntimePort = declaredPort
			reserved[declaredPort] = service.Name
			continue
		}

		allocatedPort, err := d.allocateStandaloneRuntimePort(runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, service.Name, reserved)
		if err != nil {
			return RuntimeContext{}, err
		}
		service.RuntimePort = allocatedPort
		reserved[allocatedPort] = service.Name
	}

	return withRuntimeServices(runtimeCtx, services), nil
}

func (d *FilesystemDriver) runtimePortAvailable(port int, reserved map[int]string) bool {
	if port <= 0 {
		return false
	}
	if _, exists := reserved[port]; exists {
		return false
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func (d *FilesystemDriver) allocateStandaloneRuntimePort(projectID, bindingID, serviceName string, reserved map[int]string) (int, error) {
	seedInput := strings.TrimSpace(projectID) + "|" + strings.TrimSpace(bindingID) + "|" + strings.TrimSpace(serviceName)
	seed := sha256.Sum256([]byte(seedInput))
	seedValue := binary.BigEndian.Uint32(seed[:4])

	span := standaloneRuntimePortRangeEnd - standaloneRuntimePortRangeStart + 1
	for attempt := 0; attempt < span; attempt++ {
		candidate := standaloneRuntimePortRangeStart + int((seedValue+uint32(attempt))%uint32(span))
		if d.runtimePortAvailable(candidate, reserved) {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("no runtime ports available for service %q in standalone range %d-%d", serviceName, standaloneRuntimePortRangeStart, standaloneRuntimePortRangeEnd)
}

func (d *FilesystemDriver) PrepareReleaseWorkspace(ctx context.Context, runtimeCtx RuntimeContext) (_ PreparedWorkspace, err error) {
	if runtimeCtx.Binding.RuntimeMode == contracts.RuntimeModeDistributedK3s {
		return PreparedWorkspace{}, fmt.Errorf("filesystem runtime driver does not support %q", runtimeCtx.Binding.RuntimeMode)
	}
	if filepath.Clean(d.root) == "." {
		return PreparedWorkspace{}, fmt.Errorf("runtime root must be configured")
	}
	if d.fetcher == nil {
		return PreparedWorkspace{}, fmt.Errorf("artifact fetcher is required")
	}

	runtimeCtx, err = d.assignRuntimePorts(runtimeCtx)
	if err != nil {
		return PreparedWorkspace{}, err
	}

	layout := workspaceLayout(d.root, runtimeCtx)
	defer func() {
		if err != nil {
			_ = os.RemoveAll(layout.Root)
		}
	}()

	for _, path := range []string{
		layout.Root,
		layout.Artifacts,
		layout.Config,
		layout.Sidecars,
		layout.Gateway,
		layout.Mesh,
		layout.Services,
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return PreparedWorkspace{}, err
		}
	}

	artifact, err := d.fetcher.FetchRevisionAssets(ctx, runtimeCtx, layout)
	if err != nil {
		return PreparedWorkspace{}, err
	}

	serviceManifests := make(map[string]string, len(runtimeCtx.Services))
	for _, service := range runtimeCtx.Services {
		serviceDir := filepath.Join(layout.Services, service.Name)
		if err := os.MkdirAll(serviceDir, 0o755); err != nil {
			return PreparedWorkspace{}, err
		}

		serviceManifestPath := filepath.Join(serviceDir, "service.json")
		if err := writeJSON(serviceManifestPath, service); err != nil {
			return PreparedWorkspace{}, err
		}

		hydratedConfigPath := filepath.Join(serviceDir, "runtime.json")
		if err := writeJSON(hydratedConfigPath, map[string]any{
			"service":        service,
			"artifact_ref":   artifact.ArtifactRef,
			"image_ref":      artifact.ImageRef,
			"workspace_root": layout.Root,
			"network_name":   bindingNetworkName(runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID),
		}); err != nil {
			return PreparedWorkspace{}, err
		}

		serviceManifests[service.Name] = serviceManifestPath
	}

	if err := writeJSON(filepath.Join(layout.Config, "project.json"), runtimeCtx.Project); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Config, "binding.json"), runtimeCtx.Binding); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Config, "revision.json"), runtimeCtx.Revision); err != nil {
		return PreparedWorkspace{}, err
	}

	artifactPlan := ArtifactPlan{
		Status:        artifact.Status,
		RevisionID:    runtimeCtx.Revision.RevisionID,
		ArtifactRef:   artifact.ArtifactRef,
		ImageRef:      artifact.ImageRef,
		CacheKey:      artifact.CacheKey,
		CachePath:     artifact.CachePath,
		WorkspacePath: artifact.WorkspacePath,
	}
	if err := writeJSON(filepath.Join(layout.Artifacts, "manifest.json"), artifactPlan); err != nil {
		return PreparedWorkspace{}, err
	}

	sidecarPlan := SidecarPlan{
		EnabledServices: serviceNames(runtimeCtx.Services),
		Compatibility:   runtimeCtx.Revision.CompatibilityPolicy,
		Precedence:      sidecarPrecedence(),
	}
	sidecarConfigPath := filepath.Join(layout.Sidecars, "config.json")
	if err := writeJSON(sidecarConfigPath, map[string]any{
		"services":             runtimeCtx.Services,
		"compatibility_policy": sidecarPlan.Compatibility,
		"dependencies":         runtimeCtx.Revision.DependencyBindings,
	}); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Sidecars, "plan.json"), sidecarPlan); err != nil {
		return PreparedWorkspace{}, err
	}

	gatewayPlan := GatewayPlan{
		Provider:            "caddy",
		PublicServices:      publicServiceNames(runtimeCtx.Services),
		MagicDomain:         "sslip.io",
		FallbackMagicDomain: "nip.io",
		HostToken:           gatewayHostToken(runtimeCtx),
	}
	gatewayConfigPath := filepath.Join(layout.Gateway, "gateway.json")
	if err := writeJSON(gatewayConfigPath, map[string]any{
		"provider":              gatewayPlan.Provider,
		"public_services":       gatewayPlan.PublicServices,
		"magic_domain":          gatewayPlan.MagicDomain,
		"fallback_magic_domain": gatewayPlan.FallbackMagicDomain,
		"domain_policy":         runtimeCtx.Binding.DomainPolicy,
		"revision_id":           runtimeCtx.Revision.RevisionID,
		"deployment_binding":    runtimeCtx.Binding.BindingID,
	}); err != nil {
		return PreparedWorkspace{}, err
	}
	if err := writeJSON(filepath.Join(layout.Gateway, "plan.json"), gatewayPlan); err != nil {
		return PreparedWorkspace{}, err
	}

	var meshResult MeshFoundationResult
	var meshSnapshot *MeshFoundationSnapshot
	if runtimeCtx.Binding.RuntimeMode == contracts.RuntimeModeDistributedMesh {
		if d.mesh == nil {
			d.mesh = NewMeshManager(d.logger, d.root)
		}
		d.mesh.now = d.now
		meshResult, err = d.mesh.BuildFoundation(ctx, runtimeCtx, layout)
		if err != nil {
			return PreparedWorkspace{}, err
		}
		meshSnapshot = &meshResult.Snapshot
	}

	manifest := WorkspaceManifest{
		PreparedAt:   d.now(),
		Project:      runtimeCtx.Project,
		Binding:      runtimeCtx.Binding,
		Revision:     runtimeCtx.Revision,
		Services:     runtimeCtx.Services,
		Layout:       layout,
		ArtifactPlan: artifactPlan,
		GatewayPlan:  gatewayPlan,
		SidecarPlan:  sidecarPlan,
		MeshSnapshot: meshSnapshot,
	}
	if err := capabilityNoContainerLeak(manifest); err != nil {
		return PreparedWorkspace{}, err
	}

	manifestPath := filepath.Join(layout.Root, "workspace.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return PreparedWorkspace{}, err
	}

	if d.logger != nil {
		d.logger.Info("prepared release workspace",
			"project_id", runtimeCtx.Project.ProjectID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"workspace_root", layout.Root,
			"artifact_cache_key", artifact.CacheKey,
			"services", len(runtimeCtx.Services),
		)
	}

	return PreparedWorkspace{
		Layout:            layout,
		ManifestPath:      manifestPath,
		ServiceManifests:  serviceManifests,
		Artifact:          artifact,
		SidecarConfigPath: sidecarConfigPath,
		GatewayConfigPath: gatewayConfigPath,
		MeshStatePath:     meshResult.WorkspaceStatePath,
		ServiceCachePath:  meshResult.WorkspaceServiceCachePath,
	}, nil
}

func (d *FilesystemDriver) RenderGatewayConfig(ctx context.Context, runtimeCtx RuntimeContext) (GatewayRenderResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	runtimeCtx = d.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)
	if d.gateway == nil {
		d.gateway = NewGatewayManager(d.logger, d.root)
	}
	d.gateway.now = d.now
	return d.gateway.RenderGatewayConfig(ctx, runtimeCtx, layout)
}

func (d *FilesystemDriver) RenderSidecars(ctx context.Context, runtimeCtx RuntimeContext) (SidecarRenderResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	runtimeCtx = d.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)
	if d.sidecar == nil {
		d.sidecar = NewSidecarManager(d.logger, d.root)
	}
	d.sidecar.now = d.now
	d.sidecar.companionImageRef = d.agentImageRef
	return d.sidecar.RenderSidecars(ctx, runtimeCtx, layout)
}

func (d *FilesystemDriver) ReconcileRevision(ctx context.Context, runtimeCtx RuntimeContext) (ReconcileRevisionResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	runtimeCtx = d.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)

	appliedSteps := []string{
		"validate_revision_workspace",
		"sync_dependency_bindings",
		"verify_sidecar_config",
		"verify_gateway_config",
		"record_revision_state",
	}

	reconcilePath := filepath.Join(layout.Config, "reconcile.json")
	reconcileState := map[string]any{
		"revision_id":   runtimeCtx.Revision.RevisionID,
		"applied_steps": appliedSteps,
		"reconciled_at": d.now().Format(time.RFC3339),
	}
	if err := writeJSON(reconcilePath, reconcileState); err != nil {
		return ReconcileRevisionResult{}, fmt.Errorf("write reconcile state: %w", err)
	}

	if d.logger != nil {
		d.logger.Info("revision reconciled",
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"applied_steps", len(appliedSteps),
		)
	}

	return ReconcileRevisionResult{
		RevisionID:   runtimeCtx.Revision.RevisionID,
		AppliedSteps: appliedSteps,
		Summary:      fmt.Sprintf("revision %s reconciled with %d steps", runtimeCtx.Revision.RevisionID, len(appliedSteps)),
		CompletedAt:  d.now(),
	}, nil
}

func (d *FilesystemDriver) StartReleaseCandidate(ctx context.Context, runtimeCtx RuntimeContext) (CandidateRecord, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	runtimeCtx = d.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)
	manifest, err := loadWorkspaceManifest(layout)
	if err != nil {
		return CandidateRecord{}, fmt.Errorf("workspace manifest is missing for revision %q: %w", runtimeCtx.Revision.RevisionID, err)
	}

	startedServices := make([]string, 0, len(runtimeCtx.Services))
	startFailed := make([]string, 0)
	if d.processManager != nil {
		if err := d.processManager.ensureDockerNetwork(ctx, bindingNetworkName(runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID), runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID); err != nil {
			return CandidateRecord{}, err
		}
		for _, service := range runtimeCtx.Services {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(service.Name)), "lazyops-internal-") {
				continue
			}
			configPath := filepath.Join(layout.Services, service.Name, "runtime.json")
			if _, statErr := os.Stat(configPath); statErr != nil {
				if os.IsNotExist(statErr) {
					continue
				}
				return CandidateRecord{}, fmt.Errorf("inspect runtime config for service %q: %w", service.Name, statErr)
			}
			processName := workloadProcessKey(runtimeCtx, service.Name)
			if _, startErr := d.processManager.StartProcess(ctx, processName, configPath); startErr != nil {
				startFailed = append(startFailed, service.Name)
				if d.logger != nil {
					d.logger.Warn("failed to start candidate workload service",
						"service", service.Name,
						"revision_id", runtimeCtx.Revision.RevisionID,
						"error", startErr.Error(),
					)
				}
				continue
			}
			startedServices = append(startedServices, service.Name)
		}

		for _, service := range runtimeCtx.Services {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(service.Name)), "lazyops-internal-") {
				continue
			}
			sidecarConfigPath := filepath.Join(
				d.root,
				"projects",
				runtimeCtx.Project.ProjectID,
				"bindings",
				runtimeCtx.Binding.BindingID,
				"sidecars",
				"live",
				"services",
				service.Name,
				"config.json",
			)
			if _, err := os.Stat(sidecarConfigPath); err != nil {
				continue
			}
			processName := sidecarProcessKey(runtimeCtx, service.Name)
			if _, err := d.processManager.StartProcess(ctx, processName, sidecarConfigPath); err != nil && d.logger != nil {
				d.logger.Warn("failed to start compatibility sidecar",
					"service", service.Name,
					"revision_id", runtimeCtx.Revision.RevisionID,
					"error", err.Error(),
				)
			}
		}
	}

	candidatePath := candidateManifestPath(layout)
	if _, err := os.Stat(candidatePath); err == nil {
		existing, err := loadCandidateRecord(layout)
		if err != nil {
			return CandidateRecord{}, err
		}
		if existing.State == CandidateStatePrepared {
			if err := transitionCandidateState(&existing, CandidateStateStarting, "candidate workload starting", d.now()); err != nil {
				return CandidateRecord{}, err
			}
			if err := saveCandidateRecord(existing); err != nil {
				return CandidateRecord{}, err
			}
		}
		return existing, nil
	}

	candidate, err := seedCandidateFromWorkspace(layout, manifest, d.now())
	if err != nil {
		return CandidateRecord{}, err
	}
	if err := saveCandidateRecord(candidate); err != nil {
		return CandidateRecord{}, err
	}

	if d.logger != nil {
		d.logger.Info("recorded release candidate skeleton",
			"revision_id", candidate.RevisionID,
			"workspace_root", candidate.WorkspaceRoot,
			"state", candidate.State,
			"started_services", len(startedServices),
			"failed_services", len(startFailed),
		)
	}
	return candidate, nil
}

func (d *FilesystemDriver) ProvisionInternalServices(ctx context.Context, request ProvisionInternalServicesRequest) (ProvisionInternalServicesResult, error) {
	projectID := strings.TrimSpace(request.ProjectID)
	if projectID == "" {
		return ProvisionInternalServicesResult{}, &OperationError{
			Code:      "invalid_project_id",
			Message:   "project_id is required",
			Retryable: false,
		}
	}
	bindingID := strings.TrimSpace(request.BindingID)
	if bindingID == "" {
		return ProvisionInternalServicesResult{}, &OperationError{
			Code:      "invalid_binding_id",
			Message:   "binding_id is required",
			Retryable: false,
		}
	}

	desired := make(map[string]contracts.InternalServiceProvisionSpec, len(request.Services))
	for _, item := range request.Services {
		kind := strings.ToLower(strings.TrimSpace(item.Kind))
		if kind == "" {
			continue
		}
		if _, ok := internalServiceRuntimeSpecs[kind]; !ok {
			return ProvisionInternalServicesResult{}, &OperationError{
				Code:      "unsupported_internal_service_kind",
				Message:   fmt.Sprintf("unsupported internal service kind %q", kind),
				Retryable: false,
			}
		}
		desired[kind] = item
	}

	created := make([]string, 0, len(desired))
	updated := make([]string, 0, len(desired))
	removed := make([]string, 0, len(internalServiceRuntimeSpecs))
	if len(desired) > 0 {
		if err := d.ensureBindingNetwork(ctx, projectID, bindingID); err != nil {
			return ProvisionInternalServicesResult{}, err
		}
	}

	for kind, definition := range internalServiceRuntimeSpecs {
		containerName := internalServiceContainerName(projectID, bindingID, kind)
		spec, keep := desired[kind]
		exists, err := d.internalServiceContainerExists(ctx, containerName)
		if err != nil {
			return ProvisionInternalServicesResult{}, err
		}

		if !keep {
			if exists {
				if err := d.removeInternalServiceContainer(ctx, containerName); err != nil {
					return ProvisionInternalServicesResult{}, err
				}
				removed = append(removed, kind)
			}
			continue
		}

		if exists {
			running, err := d.internalServiceContainerRunning(ctx, containerName)
			if err != nil {
				return ProvisionInternalServicesResult{}, err
			}
			// Keep internal services warm across rollouts to avoid restarting
			// stateful dependencies (e.g., postgres) on every deploy.
			if running {
				updated = append(updated, kind)
				continue
			}
		}

		if err := d.recreateInternalServiceContainer(ctx, projectID, bindingID, containerName, definition, spec.Port); err != nil {
			return ProvisionInternalServicesResult{}, err
		}
		if exists {
			updated = append(updated, kind)
		} else {
			created = append(created, kind)
		}
	}

	sort.Strings(created)
	sort.Strings(updated)
	sort.Strings(removed)

	summary := fmt.Sprintf("internal services applied (created=%d, updated=%d, removed=%d)", len(created), len(updated), len(removed))
	if d.logger != nil {
		d.logger.Info("internal services provisioned",
			"project_id", projectID,
			"created", len(created),
			"updated", len(updated),
			"removed", len(removed),
		)
	}

	return ProvisionInternalServicesResult{
		ProjectID: projectID,
		BindingID: bindingID,
		Created:   created,
		Updated:   updated,
		Removed:   removed,
		Summary:   summary,
		AppliedAt: d.now(),
	}, nil
}

func (d *FilesystemDriver) SleepService(ctx context.Context, runtimeCtx RuntimeContext, serviceName string) error {
	if d.processManager != nil {
		processName := workloadProcessKey(runtimeCtx, serviceName)
		if err := d.processManager.StopProcess(processName); err != nil {
			return fmt.Errorf("failed to stop service %s: %w", serviceName, err)
		}
	}

	layout := workspaceLayout(d.root, runtimeCtx)
	sleepPath := filepath.Join(layout.Root, "services", serviceName, "sleep.json")
	state := map[string]any{
		"service_name": serviceName,
		"status":       "sleeping",
		"slept_at":     d.now().Format(time.RFC3339),
	}
	return writeJSON(sleepPath, state)
}

func (d *FilesystemDriver) WakeService(ctx context.Context, runtimeCtx RuntimeContext, serviceName string) error {
	layout := workspaceLayout(d.root, runtimeCtx)
	sleepPath := filepath.Join(layout.Root, "services", serviceName, "sleep.json")

	if d.processManager != nil {
		configPath := filepath.Join(layout.Root, "services", serviceName, "runtime.json")
		if _, err := os.Stat(configPath); err == nil {
			processName := workloadProcessKey(runtimeCtx, serviceName)
			if _, err := d.processManager.RestartProcess(ctx, processName, configPath); err != nil {
				return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
			}
		}
	}

	if err := os.Remove(sleepPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove sleep state for %s: %w", serviceName, err)
	}
	return nil
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func serviceNames(services []ServiceRuntimeContext) []string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		names = append(names, service.Name)
	}
	sort.Strings(names)
	return names
}

func publicServiceNames(services []ServiceRuntimeContext) []string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		if service.Public {
			names = append(names, service.Name)
		}
	}
	sort.Strings(names)
	return names
}

type internalServiceRuntimeSpec struct {
	Image            string
	Port             int
	ContainerDataDir string
	HostDataDirName  string
	Env              map[string]string
	Command          []string
}

var internalServiceRuntimeSpecs = map[string]internalServiceRuntimeSpec{
	"postgres": {
		Image:            "postgres:16-alpine",
		Port:             5432,
		ContainerDataDir: "/var/lib/postgresql/data",
		HostDataDirName:  "postgres",
		Env: map[string]string{
			"POSTGRES_DB":       "app",
			"POSTGRES_USER":     "lazyops",
			"POSTGRES_PASSWORD": "lazyops",
		},
	},
	"mysql": {
		Image:            "mysql:8.4",
		Port:             3306,
		ContainerDataDir: "/var/lib/mysql",
		HostDataDirName:  "mysql",
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "lazyops",
			"MYSQL_DATABASE":      "app",
			"MYSQL_USER":          "lazyops",
			"MYSQL_PASSWORD":      "lazyops",
		},
	},
	"redis": {
		Image:            "redis:7-alpine",
		Port:             6379,
		ContainerDataDir: "/data",
		HostDataDirName:  "redis",
		Command:          []string{"redis-server", "--appendonly", "yes"},
	},
	"rabbitmq": {
		Image:            "rabbitmq:3.13-management-alpine",
		Port:             5672,
		ContainerDataDir: "/var/lib/rabbitmq",
		HostDataDirName:  "rabbitmq",
	},
}

func internalServiceContainerName(projectID, bindingID, kind string) string {
	return fmt.Sprintf(
		"lazyops-int-%s-%s-%s",
		normalizeContainerToken(projectID),
		normalizeContainerToken(bindingID),
		normalizeContainerToken(kind),
	)
}

func normalizeContainerToken(input string) string {
	raw := strings.ToLower(strings.TrimSpace(input))
	if raw == "" {
		return "default"
	}
	var builder strings.Builder
	builder.Grow(len(raw))
	lastDash := false
	for _, ch := range raw {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			builder.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(builder.String(), "-")
	if out == "" {
		return "default"
	}
	if len(out) > 40 {
		return out[:40]
	}
	return out
}

func (d *FilesystemDriver) internalServiceContainerExists(ctx context.Context, name string) (bool, error) {
	output, err := d.runDockerCommand(ctx, "ps", "-a", "--filter", "name=^/"+name+"$", "--format", "{{.Names}}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func (d *FilesystemDriver) internalServiceContainerRunning(ctx context.Context, name string) (bool, error) {
	output, err := d.runDockerCommand(ctx, "ps", "--filter", "name=^/"+name+"$", "--format", "{{.Names}}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func (d *FilesystemDriver) removeInternalServiceContainer(ctx context.Context, name string) error {
	if _, err := d.runDockerCommand(ctx, "rm", "-f", name); err != nil {
		return err
	}
	return nil
}

func (d *FilesystemDriver) recreateInternalServiceContainer(ctx context.Context, projectID, bindingID, containerName string, definition internalServiceRuntimeSpec, requestedPort int) error {
	if requestedPort <= 0 {
		requestedPort = definition.Port
	}
	if requestedPort <= 0 {
		return &OperationError{
			Code:      "invalid_internal_service_port",
			Message:   "internal service port must be greater than zero",
			Retryable: false,
		}
	}

	// Remove existing container before recreate to keep config deterministic.
	if _, err := d.runDockerCommand(ctx, "rm", "-f", containerName); err != nil {
		// Ignore "No such container" style errors.
		if !strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return err
		}
	}

	hostDataDir := filepath.Join("/var/lib/lazyops-agent", "internal-services", normalizeContainerToken(projectID), normalizeContainerToken(bindingID), definition.HostDataDirName)
	if err := os.MkdirAll(hostDataDir, 0o755); err != nil {
		return &OperationError{
			Code:      "internal_service_data_dir_failed",
			Message:   fmt.Sprintf("prepare data directory for %s failed", containerName),
			Retryable: false,
			Err:       err,
		}
	}

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"--network", bindingNetworkName(projectID, bindingID),
		"--network-alias", internalServiceNetworkAlias(definition.HostDataDirName),
		"--label", "lazyops.managed=internal-service",
		"--label", "lazyops.project_id=" + projectID,
		"--label", "lazyops.binding_id=" + bindingID,
		"--label", "lazyops.kind=" + definition.HostDataDirName,
	}

	if definition.ContainerDataDir != "" {
		args = append(args, "-v", hostDataDir+":"+definition.ContainerDataDir)
	}
	envVars := definition.Env
	var credentialState internalPostgresCredentialState
	if definition.HostDataDirName == "postgres" {
		var err error
		credentialState, err = d.ensureInternalPostgresCredentialState(projectID, bindingID, requestedPort)
		if err != nil {
			return err
		}
		envVars = map[string]string{
			"POSTGRES_DB":               credentialState.Database,
			"POSTGRES_USER":             credentialState.Username,
			"POSTGRES_PASSWORD":         credentialState.Password,
			"POSTGRES_HOST_AUTH_METHOD": "password",
		}
	}
	for key, value := range envVars {
		args = append(args, "-e", key+"="+value)
	}
	args = append(args, definition.Image)
	args = append(args, definition.Command...)

	if _, err := d.runDockerCommand(ctx, args...); err != nil {
		return err
	}
	if definition.HostDataDirName == "postgres" {
		if err := d.ensureInternalPostgresAuthentication(ctx, containerName, hostDataDir, credentialState); err != nil {
			return err
		}
	}
	return nil
}

func internalServiceNetworkAlias(kind string) string {
	return "lazyops-internal-" + normalizeContainerToken(kind)
}

func (d *FilesystemDriver) ensureInternalPostgresAuthentication(ctx context.Context, containerName, hostDataDir string, credentialState internalPostgresCredentialState) error {
	if strings.TrimSpace(credentialState.Username) == "" || strings.TrimSpace(credentialState.Password) == "" {
		return &OperationError{
			Code:      "internal_postgres_missing_credentials",
			Message:   "internal postgres credentials are missing",
			Retryable: false,
		}
	}
	if err := rewritePostgresHostAuth(filepath.Join(hostDataDir, "pg_hba.conf"), "password"); err != nil {
		return err
	}
	if err := d.waitForInternalPostgres(ctx, containerName); err != nil {
		return err
	}
	passwordSQL := escapePostgresLiteral(credentialState.Password)
	usernameSQL := escapePostgresIdentifier(credentialState.Username)
	if _, err := d.runDockerCommand(
		ctx,
		"exec", "-u", "postgres", containerName,
		"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres",
		"-c", fmt.Sprintf("ALTER ROLE %s WITH PASSWORD '%s';", usernameSQL, passwordSQL),
	); err != nil {
		return &OperationError{
			Code:      "internal_postgres_password_sync_failed",
			Message:   fmt.Sprintf("sync internal postgres password for %s failed", containerName),
			Retryable: true,
			Err:       err,
		}
	}
	if _, err := d.runDockerCommand(
		ctx,
		"exec", "-u", "postgres", containerName,
		"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres",
		"-c", "SELECT pg_reload_conf();",
	); err != nil {
		return &OperationError{
			Code:      "internal_postgres_reload_failed",
			Message:   fmt.Sprintf("reload internal postgres config for %s failed", containerName),
			Retryable: true,
			Err:       err,
		}
	}
	return nil
}

func (d *FilesystemDriver) waitForInternalPostgres(ctx context.Context, containerName string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := d.runDockerCommand(
			ctx,
			"exec", "-u", "postgres", containerName,
			"psql", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres",
			"-tAc", "SELECT 1",
		); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return &OperationError{
				Code:      "internal_postgres_start_timeout",
				Message:   fmt.Sprintf("timed out waiting for internal postgres container %q", containerName),
				Retryable: true,
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (d *FilesystemDriver) ensureBindingNetwork(ctx context.Context, projectID, bindingID string) error {
	networkName := bindingNetworkName(projectID, bindingID)
	if _, err := d.runDockerCommand(ctx, "network", "inspect", networkName); err == nil {
		return nil
	}

	args := []string{
		"network", "create",
		"--driver", "bridge",
		"--label", "lazyops.managed=binding-network",
		"--label", "lazyops.project_id=" + projectID,
		"--label", "lazyops.binding_id=" + bindingID,
		networkName,
	}
	_, err := d.runDockerCommand(ctx, args...)
	return err
}

func (d *FilesystemDriver) ensureInternalPostgresCredentialState(projectID, bindingID string, listenerPort int) (internalPostgresCredentialState, error) {
	return loadOrCreateInternalPostgresCredentialState(d.root, d.stateKey, projectID, bindingID, listenerPort, d.now())
}

func (d *FilesystemDriver) runDockerCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		return text, &OperationError{
			Code:      "docker_command_failed",
			Message:   fmt.Sprintf("docker %s failed: %s", strings.Join(args, " "), text),
			Retryable: true,
			Err:       err,
		}
	}
	return text, nil
}
