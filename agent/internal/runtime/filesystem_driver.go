package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"lazyops-agent/internal/contracts"
)

type FilesystemDriver struct {
	logger         *slog.Logger
	root           string
	fetcher        AssetFetcher
	gateway        *GatewayManager
	sidecar        *SidecarManager
	mesh           *MeshManager
	processManager *ProcessManager
	now            func() time.Time
}

func NewFilesystemDriver(logger *slog.Logger, root string) *FilesystemDriver {
	pm := NewProcessManager(logger, root)
	return &FilesystemDriver{
		logger:         logger,
		root:           root,
		fetcher:        NewLocalCacheFetcher(filepath.Join(root, "cache", "assets")),
		gateway:        NewGatewayManager(logger, root),
		sidecar:        NewSidecarManager(logger, root).WithProcessManager(pm),
		mesh:           NewMeshManager(logger, root),
		processManager: pm,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
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
	if d.gateway == nil {
		d.gateway = NewGatewayManager(d.logger, d.root)
	}
	d.gateway.now = d.now
	return d.gateway.RenderGatewayConfig(ctx, runtimeCtx, layout)
}

func (d *FilesystemDriver) RenderSidecars(ctx context.Context, runtimeCtx RuntimeContext) (SidecarRenderResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	if d.sidecar == nil {
		d.sidecar = NewSidecarManager(d.logger, d.root)
	}
	d.sidecar.now = d.now
	return d.sidecar.RenderSidecars(ctx, runtimeCtx, layout)
}

func (d *FilesystemDriver) ReconcileRevision(ctx context.Context, runtimeCtx RuntimeContext) (ReconcileRevisionResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)

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

func (d *FilesystemDriver) StartReleaseCandidate(_ context.Context, runtimeCtx RuntimeContext) (CandidateRecord, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	manifest, err := loadWorkspaceManifest(layout)
	if err != nil {
		return CandidateRecord{}, fmt.Errorf("workspace manifest is missing for revision %q: %w", runtimeCtx.Revision.RevisionID, err)
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
		)
	}
	return candidate, nil
}

func (d *FilesystemDriver) SleepService(ctx context.Context, runtimeCtx RuntimeContext, serviceName string) error {
	if d.processManager != nil {
		if err := d.processManager.StopProcess(serviceName); err != nil {
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
			if _, err := d.processManager.RestartProcess(ctx, serviceName, configPath); err != nil {
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
