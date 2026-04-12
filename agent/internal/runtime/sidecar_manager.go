package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/state"
)

type SidecarManager struct {
	logger         *slog.Logger
	runtimeRoot    string
	now            func() time.Time
	processManager *ProcessManager
	dnsServer      *DNSServer
	createHook     func(context.Context, RuntimeContext, SidecarPlan, sidecarRenderPaths) (SidecarActivation, SidecarHookResult, error)
	reconcileHook  func(context.Context, RuntimeContext, SidecarPlan, sidecarRenderPaths, SidecarActivation) (SidecarHookResult, error)
	restartHook    func(context.Context, RuntimeContext, SidecarPlan, sidecarRenderPaths, *SidecarActivation, SidecarActivation) (SidecarHookResult, error)
	removeHook     func(context.Context, RuntimeContext, SidecarPlan, sidecarRenderPaths) (SidecarHookResult, error)
}

type sidecarRenderPaths struct {
	versionRoot                string
	versionPlanPath            string
	versionConfigPath          string
	workspacePlan              string
	workspaceConfig            string
	injectionsRoot             string
	liveRoot                   string
	livePlanPath               string
	liveConfigRoot             string
	liveActivation             string
	metadataCachePath          string
	managedCredentialAuditPath string
	createPath                 string
	reconcilePath              string
	restartPath                string
	removePath                 string
}

func NewSidecarManager(logger *slog.Logger, runtimeRoot string) *SidecarManager {
	return &SidecarManager{
		logger:         logger,
		runtimeRoot:    runtimeRoot,
		processManager: NewProcessManager(logger, runtimeRoot),
		dnsServer:      NewDNSServer(logger, ""),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (m *SidecarManager) WithProcessManager(pm *ProcessManager) *SidecarManager {
	m.processManager = pm
	return m
}

func (m *SidecarManager) WithDNSServer(dns *DNSServer) *SidecarManager {
	m.dnsServer = dns
	return m
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
	credentialAudit := buildManagedCredentialAuditLog(plan, m.now())
	if err := writeJSON(paths.metadataCachePath, metadataCache); err != nil {
		return SidecarRenderResult{}, err
	}
	if err := writeJSON(paths.managedCredentialAuditPath, credentialAudit); err != nil {
		return SidecarRenderResult{}, err
	}

	create := m.createHook
	if create == nil {
		create = m.defaultCreate
	}
	previousActivation, _ := loadSidecarActivation(paths.liveActivation)
	activation, createResult, err := create(ctx, runtimeCtx, plan, paths)
	if err != nil {
		return SidecarRenderResult{}, err
	}
	plan.Create = &createResult

	reconcile := m.reconcileHook
	if reconcile == nil {
		reconcile = m.defaultReconcile
	}
	reconcileResult, err := reconcile(ctx, runtimeCtx, plan, paths, activation)
	if err != nil {
		return SidecarRenderResult{}, err
	}
	plan.Reconcile = &reconcileResult

	restart := m.restartHook
	if restart == nil {
		restart = m.defaultRestart
	}
	restartResult, err := restart(ctx, runtimeCtx, plan, paths, previousActivation, activation)
	if err != nil {
		return SidecarRenderResult{}, err
	}
	plan.Restart = &restartResult

	remove := m.removeHook
	if remove == nil {
		remove = m.defaultRemove
	}
	removeResult, err := remove(ctx, runtimeCtx, plan, paths)
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
		Version:                    version,
		PlanPath:                   paths.versionPlanPath,
		ConfigPath:                 paths.versionConfigPath,
		LivePlanPath:               paths.livePlanPath,
		LiveConfigRoot:             paths.liveConfigRoot,
		ActivationPath:             paths.liveActivation,
		MetadataCachePath:          paths.metadataCachePath,
		ManagedCredentialAuditPath: paths.managedCredentialAuditPath,
		Services:                   append([]string(nil), plan.EnabledServices...),
		Plan:                       plan,
		Activation:                 activation,
	}, nil
}

func (m *SidecarManager) buildPlan(runtimeCtx RuntimeContext, version string, paths sidecarRenderPaths) (SidecarPlan, SidecarMetadataCache, error) {
	serviceIndex := make(map[string]ServiceRuntimeContext, len(runtimeCtx.Services))
	for _, service := range runtimeCtx.Services {
		serviceIndex[service.Name] = service
	}
	resolver, err := newRuntimeDependencyResolver(m.runtimeRoot, runtimeCtx)
	if err != nil {
		return SidecarPlan{}, SidecarMetadataCache{}, err
	}

	selectedMode := selectSidecarMode(runtimeCtx.Revision.CompatibilityPolicy)
	if err := validateSelectedSidecarMode(selectedMode, runtimeCtx.Revision.CompatibilityPolicy); err != nil {
		return SidecarPlan{}, SidecarMetadataCache{}, err
	}
	plan := SidecarPlan{
		Version:              version,
		GeneratedAt:          m.now(),
		Compatibility:        runtimeCtx.Revision.CompatibilityPolicy,
		Precedence:           sidecarPrecedence(),
		PlacementFingerprint: resolver.PlacementFingerprint(),
		RouteFingerprint:     resolver.RouteFingerprint(),
		InvalidationRules:    resolver.InvalidationRules(),
		MetadataCachePath:    paths.metadataCachePath,
	}
	metadataCache := SidecarMetadataCache{
		Version:              version,
		UpdatedAt:            m.now(),
		PlacementFingerprint: resolver.PlacementFingerprint(),
		RouteFingerprint:     resolver.RouteFingerprint(),
		InvalidationRules:    resolver.InvalidationRules(),
		Services:             make(map[string]SidecarServiceMetadata),
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

			resolution := resolver.ResolveDependency(service, dependency)
			plan.Bindings = append(plan.Bindings, SidecarBinding{
				ServiceName:           service.Name,
				Alias:                 dependency.Alias,
				TargetService:         dependency.TargetService,
				Protocol:              dependency.Protocol,
				LocalEndpoint:         dependency.LocalEndpoint,
				RouteScope:            resolution.RouteScope,
				ResolutionStatus:      resolution.ResolutionStatus,
				PlacementPeerRef:      resolution.PlacementPeerRef,
				ResolvedEndpoint:      resolution.ResolvedEndpoint,
				ResolvedUpstream:      resolution.ResolvedUpstream,
				Provider:              resolution.Provider,
				PublicFallbackBlocked: resolution.PublicFallbackBlocked,
				InvalidationReasons:   append([]string(nil), resolution.InvalidationReasons...),
				ResolutionReason:      resolution.Reason,
			})
			config.DependencyAliases = append(config.DependencyAliases, dependency.Alias)
			protocols = append(protocols, dependency.Protocol)
			config.Resolutions = append(config.Resolutions, resolution)

			envKeyPrefix := "LAZYOPS_DEP_" + sanitizeEnvKey(nonEmptyAlias(dependency.Alias, dependency.TargetService))
			switch selectedMode {
			case "env_injection":
				envContract := buildSidecarEnvContract(envKeyPrefix, dependency, resolution)
				if err := validateSidecarEnvContract(service.Name, envContract, serviceIndex); err != nil {
					return SidecarPlan{}, SidecarMetadataCache{}, err
				}
				config.EnvContracts = append(config.EnvContracts, envContract)
				for key, value := range envContract.Values {
					config.Env[key] = value
				}
			case "managed_credentials":
				contract := buildManagedCredentialContract(runtimeCtx, service.Name, envKeyPrefix, dependency, resolution)
				if err := validateManagedCredentialContract(service.Name, contract, serviceIndex); err != nil {
					return SidecarPlan{}, SidecarMetadataCache{}, err
				}
				config.ManagedCredentialContracts = append(config.ManagedCredentialContracts, contract)
				for key, value := range contract.Values {
					config.ManagedCredentials[key] = value
				}
			case "localhost_rescue":
				contract, route, err := buildLocalhostRescueContract(service.Name, dependency, resolution)
				if err != nil {
					return SidecarPlan{}, SidecarMetadataCache{}, err
				}
				if err := validateLocalhostRescueContract(service.Name, contract, serviceIndex); err != nil {
					return SidecarPlan{}, SidecarMetadataCache{}, err
				}
				config.LocalhostRescueContracts = append(config.LocalhostRescueContracts, contract)
				config.ProxyRoutes = append(config.ProxyRoutes, route)
			case "transparent_proxy":
				contract, route, err := buildTransparentProxyContract(service.Name, dependency, resolution)
				if err != nil {
					return SidecarPlan{}, SidecarMetadataCache{}, err
				}
				config.TransparentProxyContracts = append(config.TransparentProxyContracts, contract)
				config.ProxyRoutes = append(config.ProxyRoutes, route)
			}
		}

		sort.Slice(config.EnvContracts, func(i, j int) bool {
			if config.EnvContracts[i].Alias == config.EnvContracts[j].Alias {
				return config.EnvContracts[i].TargetService < config.EnvContracts[j].TargetService
			}
			return config.EnvContracts[i].Alias < config.EnvContracts[j].Alias
		})
		sort.Slice(config.ManagedCredentialContracts, func(i, j int) bool {
			if config.ManagedCredentialContracts[i].Alias == config.ManagedCredentialContracts[j].Alias {
				return config.ManagedCredentialContracts[i].TargetService < config.ManagedCredentialContracts[j].TargetService
			}
			return config.ManagedCredentialContracts[i].Alias < config.ManagedCredentialContracts[j].Alias
		})
		sort.Slice(config.LocalhostRescueContracts, func(i, j int) bool {
			if config.LocalhostRescueContracts[i].Alias == config.LocalhostRescueContracts[j].Alias {
				return config.LocalhostRescueContracts[i].TargetService < config.LocalhostRescueContracts[j].TargetService
			}
			return config.LocalhostRescueContracts[i].Alias < config.LocalhostRescueContracts[j].Alias
		})
		sort.Slice(config.ProxyRoutes, func(i, j int) bool {
			if config.ProxyRoutes[i].Alias == config.ProxyRoutes[j].Alias {
				return config.ProxyRoutes[i].TargetService < config.ProxyRoutes[j].TargetService
			}
			return config.ProxyRoutes[i].Alias < config.ProxyRoutes[j].Alias
		})
		sort.Slice(config.Resolutions, func(i, j int) bool {
			if config.Resolutions[i].Alias == config.Resolutions[j].Alias {
				return config.Resolutions[i].TargetService < config.Resolutions[j].TargetService
			}
			return config.Resolutions[i].Alias < config.Resolutions[j].Alias
		})
		sort.Strings(config.DependencyAliases)
		sort.Strings(protocols)

		// Inject LAZYOPS_SERVICE_* env vars for service discovery (Issue 2.3)
		for _, dep := range service.Dependencies {
			envKeyPrefix := "LAZYOPS_SERVICE_" + sanitizeEnvKey(nonEmptyAlias(dep.Alias, dep.TargetService))
			scheme := "http"
			if dep.Protocol == "grpc" || dep.Protocol == "https" {
				scheme = "https"
			}

			// Resolve target service info
			targetSvc, targetOK := serviceIndex[dep.TargetService]
			if targetOK {
				// Register with DNS if available
				if m.dnsServer != nil {
					m.dnsServer.RegisterService(ServiceRecord{
						ServiceName: dep.TargetService,
						ProjectID:   runtimeCtx.Project.ProjectID,
						Host:        "127.0.0.1",
						Port:        targetSvc.HealthCheck.Port,
						Protocol:    dep.Protocol,
					})
				}

				// Use DNS hostname for service discovery
				dnsHostname := fmt.Sprintf("%s.%s.%s", dep.TargetService, runtimeCtx.Project.ProjectID, "lazyops.internal")
				config.Env[envKeyPrefix+"_HOST"] = dnsHostname
				config.Env[envKeyPrefix+"_PORT"] = fmt.Sprintf("%d", targetSvc.HealthCheck.Port)
				config.Env[envKeyPrefix+"_URL"] = fmt.Sprintf("%s://%s.%s.%s:%d",
					scheme, dep.TargetService, runtimeCtx.Project.ProjectID, "lazyops.internal", targetSvc.HealthCheck.Port)
			} else {
				// Fallback: use derived endpoint with DNS-resolvable hostname
				derivedEndpoint := derivedDependencyEndpoint(dep, runtimeCtx.Project.ProjectID)
				if derivedEndpoint != "" {
					config.Env[envKeyPrefix+"_HOST"] = derivedEndpoint
				}
			}
		}

		plan.EnabledServices = append(plan.EnabledServices, service.Name)
		plan.Services = append(plan.Services, config)
		metadataCache.Services[service.Name] = SidecarServiceMetadata{
			SelectedMode:               selectedMode,
			DependencyAliases:          append([]string(nil), config.DependencyAliases...),
			Protocols:                  protocols,
			EnvContracts:               append([]SidecarEnvContract(nil), config.EnvContracts...),
			ManagedCredentialContracts: append([]SidecarManagedCredentialContract(nil), config.ManagedCredentialContracts...),
			LocalhostRescueContracts:   append([]SidecarLocalhostRescueContract(nil), config.LocalhostRescueContracts...),
			TransparentProxyContracts:  append([]TransparentProxyContract(nil), config.TransparentProxyContracts...),
			Resolutions:                append([]DependencyResolutionView(nil), config.Resolutions...),
			CacheInvalidationRules:     append([]string(nil), resolver.InvalidationRules()...),
			CorrelationPropagation:     true,
			LatencyMeasurement:         true,
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
			cacheInvalidationRules := []string(nil)
			if metadataCache != nil {
				cacheInvalidationRules = append(cacheInvalidationRules, metadataCache.Services[service.Name].CacheInvalidationRules...)
			}
			sidecarBlock = map[string]any{
				"enabled":                       true,
				"selected_mode":                 config.SelectedMode,
				"precedence":                    plan.Precedence,
				"dependency_aliases":            config.DependencyAliases,
				"env":                           config.Env,
				"env_contracts":                 config.EnvContracts,
				"managed_credentials":           config.ManagedCredentials,
				"managed_credential_contracts":  config.ManagedCredentialContracts,
				"managed_credential_audit_path": paths.managedCredentialAuditPath,
				"localhost_rescue_contracts":    config.LocalhostRescueContracts,
				"transparent_proxy_contracts":   config.TransparentProxyContracts,
				"proxy_routes":                  config.ProxyRoutes,
				"resolutions":                   config.Resolutions,
				"correlation_propagation":       config.CorrelationPropagation,
				"latency_measurement":           config.LatencyMeasurement,
				"cache_invalidation_rules":      cacheInvalidationRules,
			}
			if metadataCache != nil {
				meta := metadataCache.Services[service.Name]
				meta.ConfigPath = injectionPath
				meta.RuntimePath = runtimePath
				meta.ManagedCredentialAuditPath = paths.managedCredentialAuditPath
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
		versionRoot:                versionRoot,
		versionPlanPath:            filepath.Join(versionRoot, "plan.json"),
		versionConfigPath:          filepath.Join(versionRoot, "config.json"),
		workspacePlan:              filepath.Join(layout.Sidecars, "plan.json"),
		workspaceConfig:            filepath.Join(layout.Sidecars, "config.json"),
		injectionsRoot:             filepath.Join(layout.Sidecars, "injections"),
		liveRoot:                   liveRoot,
		livePlanPath:               filepath.Join(liveRoot, "plan.json"),
		liveConfigRoot:             filepath.Join(liveRoot, "services"),
		liveActivation:             filepath.Join(liveRoot, "activation.json"),
		metadataCachePath:          filepath.Join(m.runtimeRoot, "cache", "sidecars", runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, "metadata.json"),
		managedCredentialAuditPath: filepath.Join(m.runtimeRoot, "cache", "sidecars", runtimeCtx.Project.ProjectID, runtimeCtx.Binding.BindingID, "managed-credentials-audit.json"),
		createPath:                 filepath.Join(liveRoot, "create.json"),
		reconcilePath:              filepath.Join(liveRoot, "reconcile.json"),
		restartPath:                filepath.Join(liveRoot, "restart.json"),
		removePath:                 filepath.Join(liveRoot, "remove.json"),
	}
}

func (m *SidecarManager) defaultCreate(ctx context.Context, runtimeCtx RuntimeContext, plan SidecarPlan, paths sidecarRenderPaths) (SidecarActivation, SidecarHookResult, error) {
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

		if m.processManager != nil {
			configPath := filepath.Join(serviceDir, "config.json")
			processName := sidecarProcessKey(runtimeCtx, service.ServiceName)
			if _, err := m.processManager.StartProcess(ctx, processName, configPath); err != nil {
				if m.logger != nil {
					m.logger.Warn("sidecar process start failed",
						"service", service.ServiceName,
						"error", err.Error(),
					)
				}
			}
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

func (m *SidecarManager) defaultReconcile(_ context.Context, _ RuntimeContext, plan SidecarPlan, paths sidecarRenderPaths, activation SidecarActivation) (SidecarHookResult, error) {
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

func (m *SidecarManager) defaultRestart(ctx context.Context, runtimeCtx RuntimeContext, plan SidecarPlan, paths sidecarRenderPaths, previous *SidecarActivation, activation SidecarActivation) (SidecarHookResult, error) {
	status := "skipped"
	message := "sidecar restart not required"

	if m.processManager != nil {
		for _, service := range plan.Services {
			configPath := filepath.Join(paths.liveConfigRoot, service.ServiceName, "config.json")
			processName := sidecarProcessKey(runtimeCtx, service.ServiceName)
			if _, err := m.processManager.RestartProcess(ctx, processName, configPath); err != nil {
				if m.logger != nil {
					m.logger.Warn("sidecar process restart failed",
						"service", service.ServiceName,
						"error", err.Error(),
					)
				}
			}
		}
	}

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

func (m *SidecarManager) defaultRemove(ctx context.Context, runtimeCtx RuntimeContext, plan SidecarPlan, paths sidecarRenderPaths) (SidecarHookResult, error) {
	current := make(map[string]struct{}, len(plan.EnabledServices))
	for _, serviceName := range plan.EnabledServices {
		current[serviceName] = struct{}{}
	}

	if m.processManager != nil {
		for serviceName := range current {
			_ = m.processManager.StopProcess(sidecarProcessKey(runtimeCtx, serviceName))
		}
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
		if m.processManager != nil {
			_ = m.processManager.StopProcess(sidecarProcessKey(runtimeCtx, entry.Name()))
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
		runtimeCtx.Runtime.PlacementFingerprint,
		fmt.Sprintf("%t|%t|%t|%t",
			runtimeCtx.Revision.CompatibilityPolicy.EnvInjection,
			runtimeCtx.Revision.CompatibilityPolicy.ManagedCredentials,
			runtimeCtx.Revision.CompatibilityPolicy.LocalhostRescue,
			runtimeCtx.Revision.CompatibilityPolicy.TransparentProxy,
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
	case policy.TransparentProxy:
		return "transparent_proxy"
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

func validateSelectedSidecarMode(selectedMode string, policy contracts.CompatibilityPolicy) error {
	if policy.EnvInjection && selectedMode != "env_injection" {
		return &OperationError{
			Code:      "sidecar_precedence_violation",
			Message:   "env injection must remain the first precedence layer when enabled",
			Retryable: false,
		}
	}
	if !policy.EnvInjection && policy.ManagedCredentials && selectedMode != "managed_credentials" {
		return &OperationError{
			Code:      "sidecar_precedence_violation",
			Message:   "managed credential injection must remain ahead of localhost rescue when env injection is disabled",
			Retryable: false,
		}
	}
	return nil
}

type parsedLocalhostEndpoint struct {
	Normalized string
	Host       string
	Port       int
	Scheme     string
}

func buildSidecarEnvContract(envKeyPrefix string, dependency contracts.DependencyBindingPayload, resolution DependencyResolutionView) SidecarEnvContract {
	values := map[string]string{
		envKeyPrefix + "_ENDPOINT":       resolution.ResolvedEndpoint,
		envKeyPrefix + "_PROTOCOL":       dependency.Protocol,
		envKeyPrefix + "_TARGET_SERVICE": dependency.TargetService,
		envKeyPrefix + "_ROUTE_SCOPE":    resolution.RouteScope,
		envKeyPrefix + "_STATUS":         resolution.ResolutionStatus,
	}
	if resolution.PlacementPeerRef != "" {
		values[envKeyPrefix+"_PLACEMENT_PEER"] = resolution.PlacementPeerRef
	}
	if resolution.Provider != "" {
		values[envKeyPrefix+"_PROVIDER"] = string(resolution.Provider)
	}
	if len(resolution.InvalidationReasons) > 0 {
		values[envKeyPrefix+"_INVALIDATION_REASONS"] = strings.Join(resolution.InvalidationReasons, ",")
	}

	return SidecarEnvContract{
		Alias:               dependency.Alias,
		TargetService:       dependency.TargetService,
		Protocol:            dependency.Protocol,
		RequiredKeys:        sidecarEnvRequiredKeys(envKeyPrefix),
		Values:              values,
		RouteScope:          resolution.RouteScope,
		ResolutionStatus:    resolution.ResolutionStatus,
		PlacementPeerRef:    resolution.PlacementPeerRef,
		InvalidationReasons: append([]string(nil), resolution.InvalidationReasons...),
		ResolutionReason:    resolution.Reason,
		SecretSafe:          true,
	}
}

func validateSidecarEnvContract(serviceName string, contract SidecarEnvContract, serviceIndex map[string]ServiceRuntimeContext) error {
	if _, ok := serviceIndex[contract.TargetService]; !ok {
		return &OperationError{
			Code:      "sidecar_env_contract_missing_target",
			Message:   fmt.Sprintf("env injection contract for service %q references missing target service %q", serviceName, contract.TargetService),
			Retryable: false,
		}
	}
	for _, key := range contract.RequiredKeys {
		value := strings.TrimSpace(contract.Values[key])
		if value == "" {
			return &OperationError{
				Code:      "sidecar_env_contract_missing_key",
				Message:   fmt.Sprintf("env injection contract for service %q is missing required key %q", serviceName, key),
				Retryable: false,
			}
		}
		if looksSensitiveEnvKey(key) {
			return &OperationError{
				Code:      "sidecar_env_contract_secret_key",
				Message:   fmt.Sprintf("env injection contract for service %q contains forbidden sensitive key %q", serviceName, key),
				Retryable: false,
			}
		}
	}
	for key := range contract.Values {
		if looksSensitiveEnvKey(key) {
			return &OperationError{
				Code:      "sidecar_env_contract_secret_key",
				Message:   fmt.Sprintf("env injection contract for service %q contains forbidden sensitive key %q", serviceName, key),
				Retryable: false,
			}
		}
	}
	if !contract.SecretSafe {
		return &OperationError{
			Code:      "sidecar_env_contract_not_secret_safe",
			Message:   fmt.Sprintf("env injection contract for service %q is marked unsafe for secret exposure", serviceName),
			Retryable: false,
		}
	}
	return nil
}

func buildLocalhostRescueContract(serviceName string, dependency contracts.DependencyBindingPayload, resolution DependencyResolutionView) (SidecarLocalhostRescueContract, SidecarProxyRoute, error) {
	listener, err := parseLocalhostEndpoint(dependency.Protocol, dependency.LocalEndpoint)
	if err != nil {
		return SidecarLocalhostRescueContract{}, SidecarProxyRoute{}, err
	}
	forwardingMode := localhostRescueForwardingMode(resolution)
	fallbackClass, fallbackReason, meshHealthRequired := localhostRescueFallback(resolution)
	contract := SidecarLocalhostRescueContract{
		Alias:                     dependency.Alias,
		TargetService:             dependency.TargetService,
		Protocol:                  dependency.Protocol,
		ListenerEndpoint:          listener.Normalized,
		ListenerHost:              listener.Host,
		ListenerPort:              listener.Port,
		ListenerScheme:            listener.Scheme,
		ForwardingMode:            forwardingMode,
		Upstream:                  resolution.ResolvedUpstream,
		RouteScope:                resolution.RouteScope,
		ResolutionStatus:          resolution.ResolutionStatus,
		PlacementPeerRef:          resolution.PlacementPeerRef,
		Provider:                  resolution.Provider,
		PublicFallbackBlocked:     resolution.PublicFallbackBlocked,
		InvalidationReasons:       append([]string(nil), resolution.InvalidationReasons...),
		ResolutionReason:          resolution.Reason,
		FallbackClass:             fallbackClass,
		FallbackReason:            fallbackReason,
		MeshHealthRequired:        meshHealthRequired,
		NetworkNamespaceIntercept: true,
	}
	route := SidecarProxyRoute{
		Alias:                 dependency.Alias,
		TargetService:         dependency.TargetService,
		Protocol:              dependency.Protocol,
		LocalEndpoint:         contract.ListenerEndpoint,
		ListenerHost:          contract.ListenerHost,
		ListenerPort:          contract.ListenerPort,
		ListenerScheme:        contract.ListenerScheme,
		ForwardingMode:        contract.ForwardingMode,
		Upstream:              contract.Upstream,
		RouteScope:            contract.RouteScope,
		ResolutionStatus:      contract.ResolutionStatus,
		PlacementPeerRef:      contract.PlacementPeerRef,
		Provider:              contract.Provider,
		PublicFallbackBlocked: contract.PublicFallbackBlocked,
		InvalidationReasons:   append([]string(nil), contract.InvalidationReasons...),
		ResolutionReason:      contract.ResolutionReason,
		FallbackClass:         contract.FallbackClass,
		FallbackReason:        contract.FallbackReason,
		MeshHealthRequired:    contract.MeshHealthRequired,
		NetworkNamespace:      contract.NetworkNamespaceIntercept,
		LocalhostRescue:       true,
	}
	if strings.TrimSpace(route.Upstream) == "" {
		return SidecarLocalhostRescueContract{}, SidecarProxyRoute{}, &OperationError{
			Code:      "localhost_rescue_missing_upstream",
			Message:   fmt.Sprintf("localhost rescue for service %q dependency %q is missing an upstream target", serviceName, dependency.Alias),
			Retryable: false,
		}
	}
	return contract, route, nil
}

func validateLocalhostRescueContract(serviceName string, contract SidecarLocalhostRescueContract, serviceIndex map[string]ServiceRuntimeContext) error {
	if _, ok := serviceIndex[contract.TargetService]; !ok {
		return &OperationError{
			Code:      "localhost_rescue_missing_target",
			Message:   fmt.Sprintf("localhost rescue contract for service %q references missing target service %q", serviceName, contract.TargetService),
			Retryable: false,
		}
	}
	if contract.Protocol != "http" && contract.Protocol != "tcp" {
		return &OperationError{
			Code:      "localhost_rescue_unsupported_protocol",
			Message:   fmt.Sprintf("localhost rescue for service %q uses unsupported protocol %q", serviceName, contract.Protocol),
			Retryable: false,
		}
	}
	if strings.TrimSpace(contract.ListenerEndpoint) == "" || strings.TrimSpace(contract.ListenerHost) == "" || contract.ListenerPort <= 0 {
		return &OperationError{
			Code:      "localhost_rescue_invalid_listener",
			Message:   fmt.Sprintf("localhost rescue for service %q has an invalid localhost listener", serviceName),
			Retryable: false,
		}
	}
	if !isLoopbackHost(contract.ListenerHost) {
		return &OperationError{
			Code:      "localhost_rescue_non_local_endpoint",
			Message:   fmt.Sprintf("localhost rescue for service %q must bind to localhost, got %q", serviceName, contract.ListenerHost),
			Retryable: false,
		}
	}
	if contract.ForwardingMode != "local_target" && contract.ForwardingMode != "mesh_target" {
		return &OperationError{
			Code:      "localhost_rescue_invalid_forwarding_mode",
			Message:   fmt.Sprintf("localhost rescue for service %q has unsupported forwarding mode %q", serviceName, contract.ForwardingMode),
			Retryable: false,
		}
	}
	if strings.TrimSpace(contract.Upstream) == "" {
		return &OperationError{
			Code:      "localhost_rescue_missing_upstream",
			Message:   fmt.Sprintf("localhost rescue for service %q is missing an upstream target", serviceName),
			Retryable: false,
		}
	}
	if contract.ForwardingMode == "mesh_target" && !contract.MeshHealthRequired {
		return &OperationError{
			Code:      "localhost_rescue_mesh_health_required",
			Message:   fmt.Sprintf("localhost rescue for service %q must require mesh health for remote forwarding", serviceName),
			Retryable: false,
		}
	}
	if contract.ForwardingMode == "mesh_target" && contract.FallbackClass == "network_down" && contract.ResolutionStatus == "verified" {
		return &OperationError{
			Code:      "localhost_rescue_invalid_fallback_class",
			Message:   fmt.Sprintf("localhost rescue for service %q cannot classify a verified mesh route as network_down", serviceName),
			Retryable: false,
		}
	}
	if contract.ForwardingMode == "local_target" && contract.FallbackClass != "service_down" {
		return &OperationError{
			Code:      "localhost_rescue_invalid_fallback_class",
			Message:   fmt.Sprintf("localhost rescue for service %q must classify local forwarding failures as service_down", serviceName),
			Retryable: false,
		}
	}
	if !contract.NetworkNamespaceIntercept {
		return &OperationError{
			Code:      "localhost_rescue_namespace_required",
			Message:   fmt.Sprintf("localhost rescue for service %q must stay inside the same network namespace", serviceName),
			Retryable: false,
		}
	}
	return nil
}

// TransparentProxyContract types and functions for issue 2.2

func buildTransparentProxyContract(serviceName string, dependency contracts.DependencyBindingPayload, resolution DependencyResolutionView) (TransparentProxyContract, SidecarProxyRoute, error) {
	// Parse original port from dependency
	originalPort, err := parsePortFromEndpoint(dependency.LocalEndpoint)
	if err != nil {
		return TransparentProxyContract{}, SidecarProxyRoute{}, fmt.Errorf("failed to parse original port for %s: %w", dependency.Alias, err)
	}

	// Generate proxy port (in sidecar range: 19000-19999)
	proxyPort := generateProxyPort(originalPort)

	// Build upstream target
	upstream := resolution.ResolvedUpstream
	if upstream == "" {
		upstream = fmt.Sprintf("http://127.0.0.1:%d", originalPort)
	}

	contract := TransparentProxyContract{
		Alias:         dependency.Alias,
		TargetService: dependency.TargetService,
		Protocol:      dependency.Protocol,
		OriginalPort:  originalPort,
		ProxyPort:     proxyPort,
		Upstream:      upstream,
	}

	route := SidecarProxyRoute{
		Alias:            dependency.Alias,
		TargetService:    dependency.TargetService,
		Protocol:         dependency.Protocol,
		ListenerHost:     "127.0.0.1",
		ListenerPort:     proxyPort,
		Upstream:         upstream,
		ForwardingMode:   "transparent",
		OriginalPort:     originalPort,
		NetworkNamespace: true,
		LocalhostRescue:  false,
	}

	if strings.TrimSpace(route.Upstream) == "" {
		return TransparentProxyContract{}, SidecarProxyRoute{}, &OperationError{
			Code:      "transparent_proxy_missing_upstream",
			Message:   fmt.Sprintf("transparent proxy for service %q dependency %q is missing an upstream target", serviceName, dependency.Alias),
			Retryable: false,
		}
	}

	return contract, route, nil
}

func parsePortFromEndpoint(endpoint string) (int, error) {
	// Try to parse port from endpoint like "http://127.0.0.1:5432" or "tcp://127.0.0.1:3306"
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return 0, err
	}
	_, portStr, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portStr)
}

func generateProxyPort(originalPort int) int {
	// Map common ports to proxy range
	// 5432 → 19001, 3306 → 19002, 6379 → 19003, 8000 → 19004, 3000 → 19005
	base := 19000
	switch originalPort {
	case 5432:
		return base + 1
	case 3306:
		return base + 2
	case 6379:
		return base + 3
	case 5672:
		return base + 4
	case 8000:
		return base + 5
	case 3000:
		return base + 6
	case 8080:
		return base + 7
	case 9000:
		return base + 8
	default:
		return base + (originalPort % 1000)
	}
}

func buildManagedCredentialContract(runtimeCtx RuntimeContext, serviceName, envKeyPrefix string, dependency contracts.DependencyBindingPayload, resolution DependencyResolutionView) SidecarManagedCredentialContract {
	credentialRef := fmt.Sprintf("managed://%s/%s/%s", runtimeCtx.Project.ProjectID, serviceName, dependency.Alias)
	handle := "mcred_" + state.Fingerprint(strings.Join([]string{
		runtimeCtx.Project.ProjectID,
		runtimeCtx.Binding.BindingID,
		runtimeCtx.Revision.RevisionID,
		serviceName,
		dependency.Alias,
		dependency.TargetService,
		resolution.RouteScope,
		resolution.ResolutionStatus,
	}, "|"))
	values := map[string]string{
		"LAZYOPS_MANAGED_" + sanitizeEnvKey(nonEmptyAlias(dependency.Alias, dependency.TargetService)) + "_REF":            credentialRef,
		"LAZYOPS_MANAGED_" + sanitizeEnvKey(nonEmptyAlias(dependency.Alias, dependency.TargetService)) + "_HANDLE":         handle,
		"LAZYOPS_MANAGED_" + sanitizeEnvKey(nonEmptyAlias(dependency.Alias, dependency.TargetService)) + "_PROTOCOL":       dependency.Protocol,
		"LAZYOPS_MANAGED_" + sanitizeEnvKey(nonEmptyAlias(dependency.Alias, dependency.TargetService)) + "_TARGET_SERVICE": dependency.TargetService,
	}

	return SidecarManagedCredentialContract{
		Alias:                  dependency.Alias,
		TargetService:          dependency.TargetService,
		Protocol:               dependency.Protocol,
		CredentialRef:          credentialRef,
		RequiredKeys:           managedCredentialRequiredKeys(envKeyPrefix),
		Values:                 values,
		MaskedValues:           managedCredentialMaskedValues(values),
		ValueFingerprints:      managedCredentialFingerprints(values),
		RouteScope:             resolution.RouteScope,
		ResolutionStatus:       resolution.ResolutionStatus,
		PlacementPeerRef:       resolution.PlacementPeerRef,
		InvalidationReasons:    append([]string(nil), resolution.InvalidationReasons...),
		ResolutionReason:       resolution.Reason,
		SecretSafe:             true,
		LocalhostRescueSkipped: true,
	}
}

func validateManagedCredentialContract(serviceName string, contract SidecarManagedCredentialContract, serviceIndex map[string]ServiceRuntimeContext) error {
	if _, ok := serviceIndex[contract.TargetService]; !ok {
		return &OperationError{
			Code:      "managed_credential_missing_target",
			Message:   fmt.Sprintf("managed credential contract for service %q references missing target service %q", serviceName, contract.TargetService),
			Retryable: false,
		}
	}
	for _, key := range contract.RequiredKeys {
		value := strings.TrimSpace(contract.Values[key])
		if value == "" {
			return &OperationError{
				Code:      "managed_credential_missing_key",
				Message:   fmt.Sprintf("managed credential contract for service %q is missing required key %q", serviceName, key),
				Retryable: false,
			}
		}
	}
	for key, value := range contract.Values {
		if looksManagedCredentialPlaintext(key, value) {
			return &OperationError{
				Code:      "managed_credential_plaintext_forbidden",
				Message:   fmt.Sprintf("managed credential contract for service %q contains forbidden plaintext value for key %q", serviceName, key),
				Retryable: false,
			}
		}
	}
	if !contract.SecretSafe {
		return &OperationError{
			Code:      "managed_credential_not_secret_safe",
			Message:   fmt.Sprintf("managed credential contract for service %q is marked unsafe for secret exposure", serviceName),
			Retryable: false,
		}
	}
	if !contract.LocalhostRescueSkipped {
		return &OperationError{
			Code:      "managed_credential_precedence_violation",
			Message:   fmt.Sprintf("managed credential contract for service %q must skip localhost rescue once credential injection succeeds", serviceName),
			Retryable: false,
		}
	}
	return nil
}

func sidecarEnvRequiredKeys(envKeyPrefix string) []string {
	return []string{
		envKeyPrefix + "_ENDPOINT",
		envKeyPrefix + "_PROTOCOL",
		envKeyPrefix + "_TARGET_SERVICE",
		envKeyPrefix + "_ROUTE_SCOPE",
		envKeyPrefix + "_STATUS",
	}
}

func looksSensitiveEnvKey(key string) bool {
	upperKey := strings.ToUpper(strings.TrimSpace(key))
	for _, marker := range []string{"TOKEN", "SECRET", "PASSWORD", "PRIVATE_KEY", "CREDENTIAL"} {
		if strings.Contains(upperKey, marker) {
			return true
		}
	}
	return false
}

func managedCredentialRequiredKeys(envKeyPrefix string) []string {
	base := strings.TrimPrefix(envKeyPrefix, "LAZYOPS_DEP_")
	return []string{
		"LAZYOPS_MANAGED_" + base + "_REF",
		"LAZYOPS_MANAGED_" + base + "_HANDLE",
		"LAZYOPS_MANAGED_" + base + "_PROTOCOL",
		"LAZYOPS_MANAGED_" + base + "_TARGET_SERVICE",
	}
}

func managedCredentialMaskedValues(values map[string]string) map[string]string {
	masked := make(map[string]string, len(values))
	for key, value := range values {
		switch {
		case strings.HasSuffix(key, "_HANDLE"):
			masked[key] = maskManagedCredentialValue(value)
		default:
			masked[key] = value
		}
	}
	return masked
}

func managedCredentialFingerprints(values map[string]string) map[string]string {
	fingerprints := make(map[string]string, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		fingerprints[key] = state.Fingerprint(value)
	}
	return fingerprints
}

func maskManagedCredentialValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 6 {
		return "[MASKED]"
	}
	return value[:6] + "***" + value[len(value)-4:]
}

func looksManagedCredentialPlaintext(key, value string) bool {
	upperKey := strings.ToUpper(strings.TrimSpace(key))
	trimmed := strings.TrimSpace(value)
	switch {
	case strings.HasSuffix(upperKey, "_REF"):
		return !strings.HasPrefix(trimmed, "managed://")
	case strings.HasSuffix(upperKey, "_HANDLE"):
		return !strings.HasPrefix(trimmed, "mcred_")
	case strings.HasSuffix(upperKey, "_PROTOCOL"):
		return trimmed != "http" && trimmed != "tcp"
	case strings.HasSuffix(upperKey, "_TARGET_SERVICE"):
		return trimmed == ""
	default:
		return looksSensitiveEnvKey(key)
	}
}

func buildManagedCredentialAuditLog(plan SidecarPlan, auditedAt time.Time) ManagedCredentialAuditLog {
	audit := ManagedCredentialAuditLog{
		Version:            plan.Version,
		UpdatedAt:          auditedAt,
		PlaintextPersisted: false,
		LoggerRedactionScope: []string{
			"managed_credentials",
			"managed_credential_contracts",
			"credential_ref",
			"credential_handle",
		},
		Services: make(map[string][]ManagedCredentialAuditRecord),
	}

	for _, service := range plan.Services {
		if len(service.ManagedCredentialContracts) == 0 {
			continue
		}
		records := make([]ManagedCredentialAuditRecord, 0, len(service.ManagedCredentialContracts))
		for _, contract := range service.ManagedCredentialContracts {
			records = append(records, ManagedCredentialAuditRecord{
				ServiceName:            service.ServiceName,
				Alias:                  contract.Alias,
				TargetService:          contract.TargetService,
				Protocol:               contract.Protocol,
				CredentialRef:          contract.CredentialRef,
				MaskedValues:           cloneStringMap(contract.MaskedValues),
				ValueFingerprints:      cloneStringMap(contract.ValueFingerprints),
				RouteScope:             contract.RouteScope,
				ResolutionStatus:       contract.ResolutionStatus,
				PlacementPeerRef:       contract.PlacementPeerRef,
				PlaintextPersisted:     false,
				SecretSafe:             contract.SecretSafe,
				LocalhostRescueSkipped: contract.LocalhostRescueSkipped,
				AuditedAt:              auditedAt,
			})
		}
		audit.Services[service.ServiceName] = records
	}

	return audit
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func parseLocalhostEndpoint(protocol, endpoint string) (parsedLocalhostEndpoint, error) {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return parsedLocalhostEndpoint{}, &OperationError{
			Code:      "localhost_rescue_missing_endpoint",
			Message:   "localhost rescue requires an explicit local endpoint",
			Retryable: false,
		}
	}

	switch protocol {
	case "http":
		candidate := trimmed
		if !strings.Contains(candidate, "://") {
			candidate = "http://" + candidate
		}
		parsed, err := url.Parse(candidate)
		if err != nil {
			return parsedLocalhostEndpoint{}, &OperationError{
				Code:      "localhost_rescue_invalid_endpoint",
				Message:   fmt.Sprintf("invalid localhost rescue endpoint %q: %v", endpoint, err),
				Retryable: false,
			}
		}
		host := parsed.Hostname()
		if !isLoopbackHost(host) {
			return parsedLocalhostEndpoint{}, &OperationError{
				Code:      "localhost_rescue_non_local_endpoint",
				Message:   fmt.Sprintf("localhost rescue endpoint %q must use localhost or loopback, got %q", endpoint, host),
				Retryable: false,
			}
		}
		if parsed.Scheme != "http" {
			return parsedLocalhostEndpoint{}, &OperationError{
				Code:      "localhost_rescue_unsupported_scheme",
				Message:   fmt.Sprintf("localhost rescue endpoint %q uses unsupported scheme %q", endpoint, parsed.Scheme),
				Retryable: false,
			}
		}
		port, err := strconv.Atoi(parsed.Port())
		if err != nil || port <= 0 {
			return parsedLocalhostEndpoint{}, &OperationError{
				Code:      "localhost_rescue_missing_port",
				Message:   fmt.Sprintf("localhost rescue endpoint %q must include an explicit port", endpoint),
				Retryable: false,
			}
		}
		return parsedLocalhostEndpoint{
			Normalized: "http://" + net.JoinHostPort(host, strconv.Itoa(port)),
			Host:       host,
			Port:       port,
			Scheme:     "http",
		}, nil
	case "tcp":
		hostPort := trimmed
		if strings.Contains(hostPort, "://") {
			parsed, err := url.Parse(hostPort)
			if err != nil {
				return parsedLocalhostEndpoint{}, &OperationError{
					Code:      "localhost_rescue_invalid_endpoint",
					Message:   fmt.Sprintf("invalid localhost rescue endpoint %q: %v", endpoint, err),
					Retryable: false,
				}
			}
			if parsed.Scheme != "tcp" {
				return parsedLocalhostEndpoint{}, &OperationError{
					Code:      "localhost_rescue_unsupported_scheme",
					Message:   fmt.Sprintf("localhost rescue endpoint %q uses unsupported scheme %q", endpoint, parsed.Scheme),
					Retryable: false,
				}
			}
			hostPort = parsed.Host
		}
		host, portText, err := net.SplitHostPort(hostPort)
		if err != nil {
			return parsedLocalhostEndpoint{}, &OperationError{
				Code:      "localhost_rescue_invalid_endpoint",
				Message:   fmt.Sprintf("invalid localhost rescue endpoint %q: %v", endpoint, err),
				Retryable: false,
			}
		}
		if !isLoopbackHost(host) {
			return parsedLocalhostEndpoint{}, &OperationError{
				Code:      "localhost_rescue_non_local_endpoint",
				Message:   fmt.Sprintf("localhost rescue endpoint %q must use localhost or loopback, got %q", endpoint, host),
				Retryable: false,
			}
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port <= 0 {
			return parsedLocalhostEndpoint{}, &OperationError{
				Code:      "localhost_rescue_missing_port",
				Message:   fmt.Sprintf("localhost rescue endpoint %q must include an explicit port", endpoint),
				Retryable: false,
			}
		}
		return parsedLocalhostEndpoint{
			Normalized: net.JoinHostPort(host, strconv.Itoa(port)),
			Host:       host,
			Port:       port,
			Scheme:     "tcp",
		}, nil
	default:
		return parsedLocalhostEndpoint{}, &OperationError{
			Code:      "localhost_rescue_unsupported_protocol",
			Message:   fmt.Sprintf("localhost rescue does not support protocol %q", protocol),
			Retryable: false,
		}
	}
}

func localhostRescueForwardingMode(resolution DependencyResolutionView) string {
	if resolution.RouteScope == "mesh_private" {
		return "mesh_target"
	}
	return "local_target"
}

func localhostRescueFallback(resolution DependencyResolutionView) (fallbackClass, fallbackReason string, meshHealthRequired bool) {
	if resolution.RouteScope == "mesh_private" {
		switch resolution.ResolutionStatus {
		case "planned", "degraded", "blocked":
			return "network_down", fmt.Sprintf("mesh route is %s, so localhost rescue must surface a network_down condition", resolution.ResolutionStatus), true
		default:
			return "service_down", "mesh route is healthy enough for forwarding; remaining failures should be treated as upstream service_down", true
		}
	}
	return "service_down", "target is local to this placement; remaining failures should be treated as service_down", false
}

func isLoopbackHost(host string) bool {
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	if trimmed == "" {
		return false
	}
	switch strings.ToLower(trimmed) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

// derivedDependencyEndpoint returns a DNS-resolvable hostname for a dependency binding.
// Uses the format <service>.<project>.lazyops.internal which is resolved by the
// embedded DNS server (Issue 2.3 fix).
func derivedDependencyEndpoint(binding contracts.DependencyBindingPayload, projectID string) string {
	if strings.TrimSpace(binding.LocalEndpoint) != "" {
		return binding.LocalEndpoint
	}
	// Use DNS-resolvable hostname format
	serviceHost := fmt.Sprintf("%s.%s.lazyops.internal", binding.TargetService, projectID)
	switch binding.Protocol {
	case "http":
		return "http://" + serviceHost
	default:
		return serviceHost
	}
}

// derivedProxyUpstream returns a DNS-resolvable upstream for proxy routing.
func derivedProxyUpstream(binding contracts.DependencyBindingPayload, projectID string) string {
	serviceHost := fmt.Sprintf("%s.%s.lazyops.internal", binding.TargetService, projectID)
	switch binding.Protocol {
	case "http":
		return "http://" + serviceHost
	default:
		return serviceHost
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
