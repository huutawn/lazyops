package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
)

type SidecarManager struct {
	logger        *slog.Logger
	runtimeRoot   string
	now           func() time.Time
	createHook    func(context.Context, SidecarPlan, sidecarRenderPaths) (SidecarActivation, SidecarHookResult, error)
	reconcileHook func(context.Context, SidecarPlan, sidecarRenderPaths, SidecarActivation) (SidecarHookResult, error)
	restartHook   func(context.Context, SidecarPlan, sidecarRenderPaths, *SidecarActivation, SidecarActivation) (SidecarHookResult, error)
	removeHook    func(context.Context, SidecarPlan, sidecarRenderPaths) (SidecarHookResult, error)
}

type sidecarRenderPaths struct {
	versionRoot       string
	versionPlanPath   string
	versionConfigPath string
	workspacePlan     string
	workspaceConfig   string
	injectionsRoot    string
	liveRoot          string
	livePlanPath      string
	liveConfigRoot    string
	liveActivation    string
	metadataCachePath string
	createPath        string
	reconcilePath     string
	restartPath       string
	removePath        string
}

func NewSidecarManager(logger *slog.Logger, runtimeRoot string) *SidecarManager {
	return &SidecarManager{
		logger:      logger,
		runtimeRoot: runtimeRoot,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (m *SidecarManager) RenderSidecars(ctx context.Context, runtimeCtx RuntimeContext, layout WorkspaceLayout) (SidecarRenderResult, error) {
	if _, err := loadWorkspaceManifest(layout); err != nil {
		return SidecarRenderResult{}, &OperationError{
			Code:      "sidecar_workspace_missing",
			Message:   fmt.Sprintf("workspace manifest is missing for revision %q", runtimeCtx.Revision.RevisionID),
			Retryable: true,
			Err:       err,
		}
	}

	version := sidecarVersion(runtimeCtx)
	paths := m.renderPaths(layout, runtimeCtx, version)
	plan, metadataCache, err := m.buildPlan(runtimeCtx, version, paths)
	if err != nil {
		return SidecarRenderResult{}, err
	}

	for _, dir := range []string{
		paths.versionRoot,
		filepath.Dir(paths.workspacePlan),
		paths.injectionsRoot,
		paths.liveConfigRoot,
		filepath.Dir(paths.metadataCachePath),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return SidecarRenderResult{}, err
		}
	}

	if err := writeJSON(paths.versionPlanPath, plan); err != nil {
		return SidecarRenderResult{}, err
	}
	if err := writeJSON(paths.versionConfigPath, map[string]any{
		"version":        version,
		"precedence":     plan.Precedence,
		"bindings":       plan.Bindings,
		"service_config": plan.Services,
	}); err != nil {
		return SidecarRenderResult{}, err
	}
	if err := writeJSON(paths.workspacePlan, plan); err != nil {
		return SidecarRenderResult{}, err
	}
	if err := writeJSON(paths.workspaceConfig, map[string]any{
		"version":        version,
		"precedence":     plan.Precedence,
		"bindings":       plan.Bindings,
		"service_config": plan.Services,
	}); err != nil {
		return SidecarRenderResult{}, err
	}

	if err := m.injectRuntimeSidecars(layout, runtimeCtx, plan, paths, &metadataCache); err != nil {
		return SidecarRenderResult{}, err
	}
	if err := writeJSON(paths.metadataCachePath, metadataCache); err != nil {
		return SidecarRenderResult{}, err
	}

	create := m.createHook
	if create == nil {
		create = m.defaultCreate
	}
	previousActivation, _ := loadSidecarActivation(paths.liveActivation)
	activation, createResult, err := create(ctx, plan, paths)
	if err != nil {
		return SidecarRenderResult{}, err
	}
	plan.Create = &createResult

	reconcile := m.reconcileHook
	if reconcile == nil {
		reconcile = m.defaultReconcile
	}
	reconcileResult, err := reconcile(ctx, plan, paths, activation)
	if err != nil {
		return SidecarRenderResult{}, err
	}
	plan.Reconcile = &reconcileResult

	restart := m.restartHook
	if restart == nil {
		restart = m.defaultRestart
	}
	restartResult, err := restart(ctx, plan, paths, previousActivation, activation)
	if err != nil {
		return SidecarRenderResult{}, err
	}
	plan.Restart = &restartResult

	remove := m.removeHook
	if remove == nil {
		remove = m.defaultRemove
	}
	removeResult, err := remove(ctx, plan, paths)
	if err != nil {
		return SidecarRenderResult{}, err
	}
	plan.Remove = &removeResult

	if err := writeJSON(paths.versionPlanPath, plan); err != nil {
		return SidecarRenderResult{}, err
	}
	if err := writeJSON(paths.workspacePlan, plan); err != nil {
		return SidecarRenderResult{}, err
	}
	if err := copyFile(paths.versionPlanPath, paths.livePlanPath, 0o644); err != nil {
		return SidecarRenderResult{}, err
	}

	if m.logger != nil {
		m.logger.Info("rendered sidecar config",
			"revision_id", runtimeCtx.Revision.RevisionID,
			"version", version,
			"services", len(plan.EnabledServices),
		)
	}

	return SidecarRenderResult{
		Version:           version,
		PlanPath:          paths.versionPlanPath,
		ConfigPath:        paths.versionConfigPath,
		LivePlanPath:      paths.livePlanPath,
		LiveConfigRoot:    paths.liveConfigRoot,
		ActivationPath:    paths.liveActivation,
		MetadataCachePath: paths.metadataCachePath,
		Services:          append([]string(nil), plan.EnabledServices...),
		Plan:              plan,
		Activation:        activation,
	}, nil
}

func (m *SidecarManager) buildPlan(runtimeCtx RuntimeContext, version string, paths sidecarRenderPaths) (SidecarPlan, SidecarMetadataCache, error) {
	serviceIndex := make(map[string]ServiceRuntimeContext, len(runtimeCtx.Services))
	for _, service := range runtimeCtx.Services {
		serviceIndex[service.Name] = service
	}

	selectedMode := selectSidecarMode(runtimeCtx.Revision.CompatibilityPolicy)
	plan := SidecarPlan{
		Version:           version,
		GeneratedAt:       m.now(),
		Compatibility:     runtimeCtx.Revision.CompatibilityPolicy,
		Precedence:        sidecarPrecedence(),
		MetadataCachePath: paths.metadataCachePath,
	}
	metadataCache := SidecarMetadataCache{
		Version:   version,
		UpdatedAt: m.now(),
		Services:  make(map[string]SidecarServiceMetadata),
	}

	for _, service := range runtimeCtx.Services {
		if len(service.Dependencies) == 0 {
			continue
		}
		if selectedMode == "" {
			return SidecarPlan{}, SidecarMetadataCache{}, &OperationError{
				Code:      "sidecar_no_compatible_mode",
				Message:   fmt.Sprintf("service %q has dependency bindings but no sidecar compatibility mode is enabled", service.Name),
				Retryable: false,
			}
		}

		config := SidecarServiceConfig{
			ServiceName:            service.Name,
			SelectedMode:           selectedMode,
			Env:                    make(map[string]string),
			ManagedCredentials:     make(map[string]string),
			CorrelationPropagation: true,
			LatencyMeasurement:     true,
		}
		protocols := make([]string, 0, len(service.Dependencies))
		for _, dependency := range service.Dependencies {
			if _, ok := serviceIndex[dependency.TargetService]; !ok {
				return SidecarPlan{}, SidecarMetadataCache{}, &OperationError{
					Code:      "sidecar_missing_target_service",
					Message:   fmt.Sprintf("dependency alias %q for service %q points to missing target service %q", dependency.Alias, service.Name, dependency.TargetService),
					Retryable: false,
				}
			}
			if dependency.Protocol != "http" && dependency.Protocol != "tcp" {
				return SidecarPlan{}, SidecarMetadataCache{}, &OperationError{
					Code:      "sidecar_unsupported_protocol",
					Message:   fmt.Sprintf("dependency alias %q for service %q uses unsupported protocol %q", dependency.Alias, service.Name, dependency.Protocol),
					Retryable: false,
				}
			}

			plan.Bindings = append(plan.Bindings, SidecarBinding{
				ServiceName:   service.Name,
				Alias:         dependency.Alias,
				TargetService: dependency.TargetService,
				Protocol:      dependency.Protocol,
				LocalEndpoint: dependency.LocalEndpoint,
			})
			config.DependencyAliases = append(config.DependencyAliases, dependency.Alias)
			protocols = append(protocols, dependency.Protocol)

			envKeyPrefix := "LAZYOPS_DEP_" + sanitizeEnvKey(nonEmptyAlias(dependency.Alias, dependency.TargetService))
			endpoint := derivedDependencyEndpoint(dependency)
			switch selectedMode {
			case "env_injection":
				config.Env[envKeyPrefix+"_ENDPOINT"] = endpoint
				config.Env[envKeyPrefix+"_PROTOCOL"] = dependency.Protocol
				config.Env[envKeyPrefix+"_TARGET_SERVICE"] = dependency.TargetService
			case "managed_credentials":
				config.ManagedCredentials["LAZYOPS_MANAGED_"+sanitizeEnvKey(nonEmptyAlias(dependency.Alias, dependency.TargetService))+"_REF"] =
					fmt.Sprintf("managed://%s/%s/%s", runtimeCtx.Project.ProjectID, service.Name, dependency.Alias)
			case "localhost_rescue":
				config.ProxyRoutes = append(config.ProxyRoutes, SidecarProxyRoute{
					Alias:           dependency.Alias,
					TargetService:   dependency.TargetService,
					Protocol:        dependency.Protocol,
					LocalEndpoint:   dependency.LocalEndpoint,
					Upstream:        derivedProxyUpstream(dependency),
					LocalhostRescue: true,
				})
			}
		}

		sort.Strings(config.DependencyAliases)
		sort.Strings(protocols)
		plan.EnabledServices = append(plan.EnabledServices, service.Name)
		plan.Services = append(plan.Services, config)
		metadataCache.Services[service.Name] = SidecarServiceMetadata{
			SelectedMode:           selectedMode,
			DependencyAliases:      append([]string(nil), config.DependencyAliases...),
			Protocols:              protocols,
			CorrelationPropagation: true,
			LatencyMeasurement:     true,
		}
	}

	sort.Strings(plan.EnabledServices)
	sort.Slice(plan.Bindings, func(i, j int) bool {
		if plan.Bindings[i].ServiceName == plan.Bindings[j].ServiceName {
			return plan.Bindings[i].Alias < plan.Bindings[j].Alias
		}
		return plan.Bindings[i].ServiceName < plan.Bindings[j].ServiceName
	})
	sort.Slice(plan.Services, func(i, j int) bool {
		return plan.Services[i].ServiceName < plan.Services[j].ServiceName
	})

	return plan, metadataCache, nil
}

func (m *SidecarManager) injectRuntimeSidecars(layout WorkspaceLayout, runtimeCtx RuntimeContext, plan SidecarPlan, paths sidecarRenderPaths, metadataCache *SidecarMetadataCache) error {
	serviceConfig := make(map[string]SidecarServiceConfig, len(plan.Services))
	for _, config := range plan.Services {
		serviceConfig[config.ServiceName] = config
	}

	for _, service := range runtimeCtx.Services {
		runtimePath := filepath.Join(layout.Services, service.Name, "runtime.json")
		payload, err := os.ReadFile(runtimePath)
		if err != nil {
			return err
		}

		var current map[string]any
		if err := json.Unmarshal(payload, &current); err != nil {
			return err
		}

		sidecarBlock := map[string]any{
			"enabled":    false,
			"precedence": plan.Precedence,
		}
		injectionPath := filepath.Join(paths.injectionsRoot, service.Name+".json")
		if config, ok := serviceConfig[service.Name]; ok {
			sidecarBlock = map[string]any{
				"enabled":                 true,
				"selected_mode":           config.SelectedMode,
				"precedence":              plan.Precedence,
				"dependency_aliases":      config.DependencyAliases,
				"env":                     config.Env,
				"managed_credentials":     config.ManagedCredentials,
				"proxy_routes":            config.ProxyRoutes,
				"correlation_propagation": config.CorrelationPropagation,
				"latency_measurement":     config.LatencyMeasurement,
			}
			if metadataCache != nil {
				meta := metadataCache.Services[service.Name]
				meta.ConfigPath = injectionPath
				meta.RuntimePath = runtimePath
				metadataCache.Services[service.Name] = meta
			}
		}

		current["sidecar"] = sidecarBlock
		if err := writeJSON(runtimePath, current); err != nil {
			return err
		}
		if err := writeJSON(injectionPath, sidecarBlock); err != nil {
			return err
		}
	}
	return nil
}

func (m *SidecarManager) renderPaths(layout WorkspaceLayout, runtimeCtx RuntimeContext, version string) sidecarRenderPaths {
	bindingRoot := filepath.Join(
		m.runtimeRoot,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
		"sidecars",
	)
	versionRoot := filepath.Join(layout.Sidecars, "versions", version)
	liveRoot := filepath.Join(bindingRoot, "live")
	return sidecarRenderPaths{
		versionRoot:       versionRoot,
		versionPlanPath:   filepath.Join(versionRoot, "plan.json"),
		versionConfigPath: filepath.Join(versionRoot, "config.json"),
		workspacePlan:     filepath.Join(layout.Sidecars, "plan.json"),
		workspaceConfig:   filepath.Join(layout.Sidecars, "config.json"),
		injectionsRoot:    filepath.Join(layout.Sidecars, "injections"),
		liveRoot:          liveRoot,
		livePlanPath:      filepath.Join(liveRoot, "plan.json"),
		liveConfigRoot:    filepath.Join(liveRoot, "services"),
		liveActivation:    filepath.Join(liveRoot, "activation.json"),
		metadataCachePath: filepath.Join(m.runtimeRoot, "cache", "sidecars", runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, "metadata.json"),
		createPath:        filepath.Join(liveRoot, "create.json"),
		reconcilePath:     filepath.Join(liveRoot, "reconcile.json"),
		restartPath:       filepath.Join(liveRoot, "restart.json"),
		removePath:        filepath.Join(liveRoot, "remove.json"),
	}
}

func (m *SidecarManager) defaultCreate(_ context.Context, plan SidecarPlan, paths sidecarRenderPaths) (SidecarActivation, SidecarHookResult, error) {
	if err := copyFile(paths.versionConfigPath, filepath.Join(paths.liveRoot, "config.json"), 0o644); err != nil {
		return SidecarActivation{}, SidecarHookResult{}, err
	}
	if err := copyFile(paths.versionPlanPath, paths.livePlanPath, 0o644); err != nil {
		return SidecarActivation{}, SidecarHookResult{}, err
	}
	for _, service := range plan.Services {
		serviceDir := filepath.Join(paths.liveConfigRoot, service.ServiceName)
		if err := os.MkdirAll(serviceDir, 0o755); err != nil {
			return SidecarActivation{}, SidecarHookResult{}, err
		}
		if err := writeJSON(filepath.Join(serviceDir, "config.json"), service); err != nil {
			return SidecarActivation{}, SidecarHookResult{}, err
		}
	}

	activation := SidecarActivation{
		Version:    plan.Version,
		PlanPath:   paths.versionPlanPath,
		ConfigPath: paths.versionConfigPath,
		AppliedAt:  m.now(),
	}
	if err := writeJSON(paths.liveActivation, activation); err != nil {
		return SidecarActivation{}, SidecarHookResult{}, err
	}

	result := SidecarHookResult{
		Name:       "create",
		Status:     "created",
		Message:    fmt.Sprintf("sidecar config created for %d services", len(plan.EnabledServices)),
		Path:       paths.createPath,
		OccurredAt: activation.AppliedAt,
	}
	if err := writeJSON(paths.createPath, result); err != nil {
		return SidecarActivation{}, SidecarHookResult{}, err
	}
	return activation, result, nil
}

func (m *SidecarManager) defaultReconcile(_ context.Context, plan SidecarPlan, paths sidecarRenderPaths, activation SidecarActivation) (SidecarHookResult, error) {
	if err := copyFile(paths.versionPlanPath, paths.livePlanPath, 0o644); err != nil {
		return SidecarHookResult{}, err
	}
	result := SidecarHookResult{
		Name:       "reconcile",
		Status:     "reconciled",
		Message:    fmt.Sprintf("sidecar live state reconciled to version %s", plan.Version),
		Path:       paths.reconcilePath,
		OccurredAt: activation.AppliedAt,
	}
	if err := writeJSON(paths.reconcilePath, result); err != nil {
		return SidecarHookResult{}, err
	}
	return result, nil
}

func (m *SidecarManager) defaultRestart(_ context.Context, plan SidecarPlan, paths sidecarRenderPaths, previous *SidecarActivation, activation SidecarActivation) (SidecarHookResult, error) {
	status := "skipped"
	message := "sidecar restart not required"
	if previous != nil && previous.Version != "" && previous.Version != plan.Version {
		status = "restarted"
		message = fmt.Sprintf("sidecar runtime restarted from %s to %s", previous.Version, plan.Version)
	}

	result := SidecarHookResult{
		Name:       "restart",
		Status:     status,
		Message:    message,
		Path:       paths.restartPath,
		OccurredAt: activation.AppliedAt,
	}
	if err := writeJSON(paths.restartPath, result); err != nil {
		return SidecarHookResult{}, err
	}
	return result, nil
}

func (m *SidecarManager) defaultRemove(_ context.Context, plan SidecarPlan, paths sidecarRenderPaths) (SidecarHookResult, error) {
	current := make(map[string]struct{}, len(plan.EnabledServices))
	for _, serviceName := range plan.EnabledServices {
		current[serviceName] = struct{}{}
	}

	removed := 0
	entries, err := os.ReadDir(paths.liveConfigRoot)
	if err != nil && !os.IsNotExist(err) {
		return SidecarHookResult{}, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := current[entry.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(paths.liveConfigRoot, entry.Name())); err != nil {
			return SidecarHookResult{}, err
		}
		removed++
	}

	result := SidecarHookResult{
		Name:       "remove",
		Status:     "removed",
		Message:    fmt.Sprintf("removed %d stale sidecar service configs", removed),
		Path:       paths.removePath,
		OccurredAt: m.now(),
	}
	if err := writeJSON(paths.removePath, result); err != nil {
		return SidecarHookResult{}, err
	}
	return result, nil
}

func loadSidecarActivation(path string) (*SidecarActivation, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var activation SidecarActivation
	if err := json.Unmarshal(payload, &activation); err != nil {
		return nil, err
	}
	return &activation, nil
}

func sidecarVersion(runtimeCtx RuntimeContext) string {
	parts := []string{
		runtimeCtx.Project.ProjectID,
		runtimeCtx.Binding.BindingID,
		runtimeCtx.Revision.RevisionID,
		fmt.Sprintf("%t|%t|%t",
			runtimeCtx.Revision.CompatibilityPolicy.EnvInjection,
			runtimeCtx.Revision.CompatibilityPolicy.ManagedCredentials,
			runtimeCtx.Revision.CompatibilityPolicy.LocalhostRescue,
		),
	}
	for _, service := range runtimeCtx.Services {
		for _, dependency := range service.Dependencies {
			parts = append(parts, fmt.Sprintf("%s|%s|%s|%s|%s", service.Name, dependency.Alias, dependency.TargetService, dependency.Protocol, dependency.LocalEndpoint))
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "||")))
	return "sc_" + hex.EncodeToString(sum[:8])
}

func sidecarPrecedence() []string {
	return []string{"env_injection", "managed_credentials", "localhost_rescue"}
}

func selectSidecarMode(policy contracts.CompatibilityPolicy) string {
	switch {
	case policy.EnvInjection:
		return "env_injection"
	case policy.ManagedCredentials:
		return "managed_credentials"
	case policy.LocalhostRescue:
		return "localhost_rescue"
	default:
		return ""
	}
}

func derivedDependencyEndpoint(binding contracts.DependencyBindingPayload) string {
	if strings.TrimSpace(binding.LocalEndpoint) != "" {
		return binding.LocalEndpoint
	}
	switch binding.Protocol {
	case "http":
		return fmt.Sprintf("http://%s.service.lazyops.internal", binding.TargetService)
	default:
		return fmt.Sprintf("%s.service.lazyops.internal", binding.TargetService)
	}
}

func derivedProxyUpstream(binding contracts.DependencyBindingPayload) string {
	switch binding.Protocol {
	case "http":
		return fmt.Sprintf("http://%s.service.lazyops.internal", binding.TargetService)
	default:
		return fmt.Sprintf("%s.service.lazyops.internal", binding.TargetService)
	}
}

func sanitizeEnvKey(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func nonEmptyAlias(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
