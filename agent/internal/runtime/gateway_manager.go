package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
)

type GatewayManager struct {
	logger       *slog.Logger
	runtimeRoot  string
	now          func() time.Time
	validateHook func(context.Context, GatewayPlan, gatewayRenderPaths) (GatewayHookResult, error)
	applyHook    func(context.Context, GatewayPlan, gatewayRenderPaths) (GatewayActivation, GatewayHookResult, error)
	reloadHook   func(context.Context, GatewayPlan, gatewayRenderPaths, GatewayActivation) (GatewayHookResult, error)
	rollbackHook func(context.Context, GatewayPlan, gatewayRenderPaths, *GatewayActivation, GatewayActivation) (GatewayHookResult, error)
}

type gatewayRenderPaths struct {
	versionRoot     string
	versionPlanPath string
	versionConfig   string
	workspacePlan   string
	workspaceConfig string
	liveRoot        string
	livePlanPath    string
	liveConfigPath  string
	liveActivePath  string
	validatePath    string
	applyPath       string
	reloadPath      string
	rollbackPath    string
}

func NewGatewayManager(logger *slog.Logger, runtimeRoot string) *GatewayManager {
	return &GatewayManager{
		logger:      logger,
		runtimeRoot: runtimeRoot,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (m *GatewayManager) RenderGatewayConfig(ctx context.Context, runtimeCtx RuntimeContext, layout WorkspaceLayout) (GatewayRenderResult, error) {
	if _, err := loadWorkspaceManifest(layout); err != nil {
		return GatewayRenderResult{}, &OperationError{
			Code:      "gateway_workspace_missing",
			Message:   fmt.Sprintf("workspace manifest is missing for revision %q", runtimeCtx.Revision.RevisionID),
			Retryable: true,
			Err:       err,
		}
	}

	version := gatewayVersion(runtimeCtx)
	paths := m.renderPaths(layout, runtimeCtx, version)
	plan, err := m.buildPlan(runtimeCtx, version)
	if err != nil {
		return GatewayRenderResult{}, err
	}
	config := renderCaddyfile(plan)

	for _, dir := range []string{paths.versionRoot, filepath.Dir(paths.workspacePlan), paths.liveRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return GatewayRenderResult{}, err
		}
	}
	if err := os.WriteFile(paths.versionConfig, []byte(config), 0o644); err != nil {
		return GatewayRenderResult{}, err
	}
	if err := os.WriteFile(paths.workspaceConfig, []byte(config), 0o644); err != nil {
		return GatewayRenderResult{}, err
	}
	if err := writeJSON(paths.versionPlanPath, plan); err != nil {
		return GatewayRenderResult{}, err
	}
	if err := writeJSON(paths.workspacePlan, plan); err != nil {
		return GatewayRenderResult{}, err
	}

	previousActive, _ := loadGatewayActivation(paths.liveActivePath)
	previousVersion := ""
	if previousActive != nil {
		previousVersion = previousActive.Version
	}
	renderResult := GatewayRenderResult{
		Version:               version,
		PlanPath:              paths.versionPlanPath,
		ConfigPath:            paths.versionConfig,
		LivePlanPath:          paths.livePlanPath,
		LiveConfigPath:        paths.liveConfigPath,
		ActivationPath:        paths.liveActivePath,
		PreviousActiveVersion: previousVersion,
		PublicURLs:            collectPublicURLs(plan),
		Plan:                  plan,
	}

	validate := m.validateHook
	if validate == nil {
		validate = m.defaultValidate
	}
	validateResult, err := validate(ctx, plan, paths)
	if err != nil {
		return GatewayRenderResult{}, err
	}
	plan.Validation = &validateResult
	if err := writeJSON(paths.versionPlanPath, plan); err != nil {
		return GatewayRenderResult{}, err
	}
	if err := writeJSON(paths.workspacePlan, plan); err != nil {
		return GatewayRenderResult{}, err
	}

	apply := m.applyHook
	if apply == nil {
		apply = m.defaultApply
	}
	activation, applyResult, err := apply(ctx, plan, paths)
	if err != nil {
		rollback := m.rollbackHook
		if rollback == nil {
			rollback = m.defaultRollback
		}
		rollbackResult, rollbackErr := rollback(ctx, plan, paths, previousActive, GatewayActivation{
			Version:    version,
			PlanPath:   paths.versionPlanPath,
			ConfigPath: paths.versionConfig,
			AppliedAt:  m.now(),
		})
		if rollbackErr == nil {
			plan.Rollback = &rollbackResult
			renderResult.RolledBack = true
		}
		if err := writeJSON(paths.versionPlanPath, plan); err == nil {
			_ = writeJSON(paths.workspacePlan, plan)
		}
		details := map[string]any{
			"version":          version,
			"config_path":      paths.versionConfig,
			"live_config_path": paths.liveConfigPath,
		}
		if previousActive != nil {
			details["previous_active_version"] = previousActive.Version
		}
		if rollbackErr != nil {
			details["rollback_error"] = rollbackErr.Error()
		} else {
			details["rollback_path"] = rollbackResult.Path
		}
		return GatewayRenderResult{}, &OperationError{
			Code:      "gateway_apply_failed",
			Message:   fmt.Sprintf("gateway config render validation passed but apply failed: %v", err),
			Retryable: true,
			Details:   details,
			Err:       err,
		}
	}
	plan.Apply = &applyResult
	renderResult.Activation = activation
	if err := writeJSON(paths.versionPlanPath, plan); err != nil {
		return GatewayRenderResult{}, err
	}
	if err := writeJSON(paths.workspacePlan, plan); err != nil {
		return GatewayRenderResult{}, err
	}

	reload := m.reloadHook
	if reload == nil {
		reload = m.defaultReload
	}
	reloadResult, err := reload(ctx, plan, paths, activation)
	if err != nil {
		rollback := m.rollbackHook
		if rollback == nil {
			rollback = m.defaultRollback
		}
		rollbackResult, rollbackErr := rollback(ctx, plan, paths, previousActive, activation)
		if rollbackErr == nil {
			plan.Rollback = &rollbackResult
			renderResult.RolledBack = true
		}
		if plan.Apply != nil {
			plan.Apply.Status = "rolled_back"
		}
		if err := writeJSON(paths.versionPlanPath, plan); err == nil {
			_ = writeJSON(paths.workspacePlan, plan)
		}

		details := map[string]any{
			"version":          version,
			"config_path":      paths.versionConfig,
			"live_config_path": paths.liveConfigPath,
		}
		if previousActive != nil {
			details["previous_active_version"] = previousActive.Version
		}
		if rollbackErr != nil {
			details["rollback_error"] = rollbackErr.Error()
		} else {
			details["rollback_path"] = rollbackResult.Path
		}
		return GatewayRenderResult{}, &OperationError{
			Code:      "gateway_reload_failed",
			Message:   fmt.Sprintf("gateway config rendered but reload failed: %v", err),
			Retryable: true,
			Details:   details,
			Err:       err,
		}
	}

	plan.Reload = &reloadResult
	renderResult.Plan = plan
	if err := writeJSON(paths.versionPlanPath, plan); err != nil {
		return GatewayRenderResult{}, err
	}
	if err := writeJSON(paths.workspacePlan, plan); err != nil {
		return GatewayRenderResult{}, err
	}
	if err := copyFile(paths.versionPlanPath, paths.livePlanPath, 0o644); err != nil {
		return GatewayRenderResult{}, err
	}

	if m.logger != nil {
		m.logger.Info("rendered gateway config",
			"revision_id", runtimeCtx.Revision.RevisionID,
			"version", version,
			"public_routes", len(plan.Routes),
			"live_config_path", paths.liveConfigPath,
		)
	}
	return renderResult, nil
}

func (m *GatewayManager) buildPlan(runtimeCtx RuntimeContext, version string) (GatewayPlan, error) {
	primaryProvider, fallbackProvider := preferredMagicProviders()
	hostToken := gatewayHostToken(runtimeCtx)
	resolver, err := newRuntimeDependencyResolver(m.runtimeRoot, runtimeCtx)
	if err != nil {
		return GatewayPlan{}, err
	}

	routes := make([]GatewayRoute, 0)
	publicServices := make([]string, 0)
	for _, service := range runtimeCtx.Services {
		if !service.Public {
			continue
		}
		publicServices = append(publicServices, service.Name)
		primaryHost := fmt.Sprintf("%s.%s.%s", service.Name, hostToken, primaryProvider)
		fallbackHost := fmt.Sprintf("%s.%s.%s", service.Name, hostToken, fallbackProvider)
		route := resolver.ResolvePublicService(service)
		route.PrimaryHost = primaryHost
		route.FallbackHost = fallbackHost
		route.PrimaryURL = "https://" + primaryHost
		route.FallbackURL = "https://" + fallbackHost
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].ServiceName < routes[j].ServiceName
	})
	sort.Strings(publicServices)

	return GatewayPlan{
		Version:              version,
		GeneratedAt:          m.now(),
		Provider:             "caddy",
		PublicServices:       publicServices,
		MagicDomain:          primaryProvider,
		FallbackMagicDomain:  fallbackProvider,
		HostToken:            hostToken,
		PlacementFingerprint: resolver.PlacementFingerprint(),
		RouteFingerprint:     resolver.RouteFingerprint(),
		InvalidationRules:    resolver.InvalidationRules(),
		Routes:               routes,
		RoutingPolicy:        runtimeCtx.RoutingPolicy(),
		Services:             runtimeCtx.Services,
	}, nil
}

func (m *GatewayManager) renderPaths(layout WorkspaceLayout, runtimeCtx RuntimeContext, version string) gatewayRenderPaths {
	bindingRoot := filepath.Join(
		m.runtimeRoot,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
		"gateway",
	)
	versionRoot := filepath.Join(layout.Gateway, "versions", version)
	liveRoot := filepath.Join(bindingRoot, "live")
	return gatewayRenderPaths{
		versionRoot:     versionRoot,
		versionPlanPath: filepath.Join(versionRoot, "plan.json"),
		versionConfig:   filepath.Join(versionRoot, "Caddyfile"),
		workspacePlan:   filepath.Join(layout.Gateway, "plan.json"),
		workspaceConfig: filepath.Join(layout.Gateway, "Caddyfile"),
		liveRoot:        liveRoot,
		livePlanPath:    filepath.Join(liveRoot, "plan.json"),
		liveConfigPath:  filepath.Join(liveRoot, "Caddyfile"),
		liveActivePath:  filepath.Join(liveRoot, "active.json"),
		validatePath:    filepath.Join(versionRoot, "validate.json"),
		applyPath:       filepath.Join(liveRoot, "apply.json"),
		reloadPath:      filepath.Join(liveRoot, "reload.json"),
		rollbackPath:    filepath.Join(liveRoot, "rollback.json"),
	}
}

func (m *GatewayManager) defaultValidate(_ context.Context, plan GatewayPlan, paths gatewayRenderPaths) (GatewayHookResult, error) {
	if plan.Provider != "caddy" {
		return GatewayHookResult{}, &OperationError{
			Code:      "gateway_invalid_provider",
			Message:   fmt.Sprintf("gateway provider %q is not supported", plan.Provider),
			Retryable: false,
		}
	}
	if plan.MagicDomain != "sslip.io" {
		return GatewayHookResult{}, &OperationError{
			Code:      "gateway_invalid_magic_domain",
			Message:   "gateway config must prefer sslip.io",
			Retryable: false,
		}
	}
	if plan.FallbackMagicDomain != "nip.io" {
		return GatewayHookResult{}, &OperationError{
			Code:      "gateway_invalid_magic_domain_fallback",
			Message:   "gateway config must include nip.io fallback",
			Retryable: false,
		}
	}

	seenHosts := make(map[string]string, len(plan.Routes)*2)
	for _, route := range plan.Routes {
		if strings.TrimSpace(route.ServiceName) == "" {
			return GatewayHookResult{}, &OperationError{
				Code:      "gateway_invalid_route",
				Message:   "gateway route is missing service name",
				Retryable: false,
			}
		}
		if route.Port <= 0 {
			return GatewayHookResult{}, &OperationError{
				Code:      "gateway_invalid_route_port",
				Message:   fmt.Sprintf("gateway route for service %q must have a positive port", route.ServiceName),
				Retryable: false,
			}
		}
		for _, host := range []string{route.PrimaryHost, route.FallbackHost} {
			if other, exists := seenHosts[host]; exists && other != route.ServiceName {
				return GatewayHookResult{}, &OperationError{
					Code:      "gateway_duplicate_host",
					Message:   fmt.Sprintf("gateway host %q is shared by services %q and %q", host, other, route.ServiceName),
					Retryable: false,
				}
			}
			seenHosts[host] = route.ServiceName
		}
	}

	result := GatewayHookResult{
		Name:       "validate",
		Status:     "validated",
		Message:    "gateway config validated",
		Path:       paths.validatePath,
		OccurredAt: m.now(),
	}

	// Run caddy validate if available
	if err := execCaddyValidate(paths.versionConfig); err != nil {
		result.Status = "validation_failed"
		result.Message = fmt.Sprintf("caddy validate failed: %v", err)
		_ = writeJSON(paths.validatePath, result)
		return GatewayHookResult{}, &OperationError{
			Code:      "gateway_caddy_validate_failed",
			Message:   result.Message,
			Retryable: true,
			Err:       err,
		}
	}

	if err := writeJSON(paths.validatePath, result); err != nil {
		return GatewayHookResult{}, err
	}
	return result, nil
}

func (m *GatewayManager) defaultApply(_ context.Context, plan GatewayPlan, paths gatewayRenderPaths) (GatewayActivation, GatewayHookResult, error) {
	if err := copyFile(paths.versionConfig, paths.liveConfigPath, 0o644); err != nil {
		return GatewayActivation{}, GatewayHookResult{}, err
	}
	if err := copyFile(paths.versionPlanPath, paths.livePlanPath, 0o644); err != nil {
		return GatewayActivation{}, GatewayHookResult{}, err
	}

	activation := GatewayActivation{
		Version:    plan.Version,
		PlanPath:   paths.versionPlanPath,
		ConfigPath: paths.versionConfig,
		AppliedAt:  m.now(),
	}
	if err := writeJSON(paths.liveActivePath, activation); err != nil {
		return GatewayActivation{}, GatewayHookResult{}, err
	}

	result := GatewayHookResult{
		Name:       "apply",
		Status:     "applied",
		Message:    "gateway config applied to live location",
		Path:       paths.applyPath,
		OccurredAt: activation.AppliedAt,
	}
	if err := writeJSON(paths.applyPath, result); err != nil {
		return GatewayActivation{}, GatewayHookResult{}, err
	}
	return activation, result, nil
}

func (m *GatewayManager) defaultReload(_ context.Context, plan GatewayPlan, paths gatewayRenderPaths, activation GatewayActivation) (GatewayHookResult, error) {
	// Reload Caddy with the new config
	if err := execCaddyReload(paths.liveConfigPath); err != nil {
		return GatewayHookResult{}, &OperationError{
			Code:      "gateway_caddy_reload_failed",
			Message:   fmt.Sprintf("caddy reload failed: %v", err),
			Retryable: true,
			Err:       err,
		}
	}

	result := GatewayHookResult{
		Name:       "reload",
		Status:     "reloaded",
		Message:    fmt.Sprintf("gateway config version %s reloaded", plan.Version),
		Path:       paths.reloadPath,
		OccurredAt: activation.AppliedAt,
	}
	if err := writeJSON(paths.reloadPath, result); err != nil {
		return GatewayHookResult{}, err
	}
	return result, nil
}

// execCaddyValidate runs 'caddy validate' against the given config.
// If caddy binary is not found, it skips validation (dev mode).
func execCaddyValidate(configPath string) error {
	if _, err := exec.LookPath("caddy"); err != nil {
		slog.Default().Warn("caddy binary not found, skipping validation (dev mode)",
			"config_path", configPath,
		)
		return nil
	}

	cmd := exec.Command("caddy", "validate", "--config", configPath, "--adapter", "caddyfile")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("caddy validate: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// execCaddyReload runs 'caddy reload' to apply a new config.
// If caddy binary is not found, it skips the reload (dev mode).
// If Caddy isn't running, it starts Caddy instead of reloading.
func execCaddyReload(configPath string) error {
	if _, err := exec.LookPath("caddy"); err != nil {
		slog.Default().Warn("caddy binary not found, skipping reload (dev mode)",
			"config_path", configPath,
		)
		return nil
	}

	// Check if Caddy is running by probing the admin API
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:2019/config")
	if err != nil {
		// Caddy isn't running, start it
		slog.Default().Info("caddy not running, starting with config",
			"config_path", configPath,
		)
		cmd := exec.Command("caddy", "start", "--config", configPath, "--adapter", "caddyfile")
		output, startErr := cmd.CombinedOutput()
		if startErr != nil {
			return fmt.Errorf("caddy start: %s: %w", strings.TrimSpace(string(output)), startErr)
		}
		// Wait briefly for Caddy to become available
		time.Sleep(1 * time.Second)
		return nil
	}
	_ = resp.Body.Close()

	// Caddy is running, reload it
	cmd := exec.Command("caddy", "reload", "--config", configPath, "--adapter", "caddyfile")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("caddy reload: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CheckCaddyAvailable returns nil if the caddy binary is available.
func CheckCaddyAvailable() error {
	if _, err := exec.LookPath("caddy"); err != nil {
		return fmt.Errorf("caddy binary not found in PATH: %w", err)
	}
	return nil
}

func (m *GatewayManager) defaultRollback(_ context.Context, _ GatewayPlan, paths gatewayRenderPaths, previous *GatewayActivation, current GatewayActivation) (GatewayHookResult, error) {
	if previous != nil {
		if err := copyFile(previous.ConfigPath, paths.liveConfigPath, 0o644); err != nil {
			return GatewayHookResult{}, err
		}
		if err := copyFile(previous.PlanPath, paths.livePlanPath, 0o644); err != nil {
			return GatewayHookResult{}, err
		}
		if err := writeJSON(paths.liveActivePath, previous); err != nil {
			return GatewayHookResult{}, err
		}
	} else {
		for _, path := range []string{paths.liveConfigPath, paths.livePlanPath, paths.liveActivePath} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return GatewayHookResult{}, err
			}
		}
	}

	result := GatewayHookResult{
		Name:       "rollback",
		Status:     "rolled_back",
		Message:    fmt.Sprintf("gateway config rollback completed after failed apply/reload of version %s", current.Version),
		Path:       paths.rollbackPath,
		OccurredAt: m.now(),
	}
	if err := writeJSON(paths.rollbackPath, result); err != nil {
		return GatewayHookResult{}, err
	}
	return result, nil
}

func renderCaddyfile(plan GatewayPlan) string {
	// If routing policy has routes, generate path-based config
	if len(plan.RoutingPolicy.Routes) > 0 {
		return renderCaddyfileWithPathRouting(plan)
	}

	// Fallback: per-service domains (existing behavior)
	return renderCaddyfilePerService(plan)
}

func renderCaddyfilePerService(plan GatewayPlan) string {
	if len(plan.Routes) == 0 {
		return "{\n  auto_https disable_redirects\n}\n\n# no public services for this revision\n"
	}

	var builder strings.Builder
	builder.WriteString("{\n")
	builder.WriteString("  auto_https disable_redirects\n")
	builder.WriteString("}\n\n")

	for _, route := range plan.Routes {
		builder.WriteString(fmt.Sprintf("https://%s, https://%s {\n", route.PrimaryHost, route.FallbackHost))
		builder.WriteString("  encode zstd gzip\n")

		// WebSocket support: detect /ws paths and add explicit handling
		wsPath := inferWebSocketPath(route.ServiceName)
		if wsPath != "" {
			builder.WriteString(fmt.Sprintf("  @ws path %s*\n", wsPath))
			builder.WriteString("  handle @ws {\n")
			builder.WriteString(fmt.Sprintf("    reverse_proxy %s {\n", route.Upstream))
			builder.WriteString("      transport http {\n")
			builder.WriteString("        keepalive 60s\n")
			builder.WriteString("        keepalive_idle_conns 100\n")
			builder.WriteString("        read_buffer 32k\n")
			builder.WriteString("        write_buffer 32k\n")
			builder.WriteString("        flush_interval -1\n")
			builder.WriteString("      }\n")
			builder.WriteString("      health_uri /health\n")
			builder.WriteString("      health_interval 30s\n")
			builder.WriteString("      health_timeout 10s\n")
			builder.WriteString("    }\n")
			builder.WriteString("  }\n\n")
		}

		builder.WriteString(fmt.Sprintf("  reverse_proxy %s\n", route.Upstream))
		builder.WriteString("}\n\n")
	}
	return builder.String()
}

func renderCaddyfileWithPathRouting(plan GatewayPlan) string {
	if len(plan.Routes) == 0 && plan.RoutingPolicy.SharedDomain == "" {
		return renderCaddyfilePerService(plan)
	}

	var builder strings.Builder
	builder.WriteString("{\n")
	builder.WriteString("  auto_https disable_redirects\n")
	builder.WriteString("}\n\n")

	// Shared domain with path-based routing
	if plan.RoutingPolicy.SharedDomain != "" {
		domain := plan.RoutingPolicy.SharedDomain

		// Add magic domain fallbacks for the first service's domain
		for _, route := range plan.Routes {
			if len(plan.RoutingPolicy.Routes) > 0 && route.ServiceName == plan.RoutingPolicy.Routes[0].Service {
				domain = fmt.Sprintf("%s, https://%s, https://%s",
					domain, route.PrimaryHost, route.FallbackHost)
				break
			}
		}

		builder.WriteString(fmt.Sprintf("%s {\n", domain))
		builder.WriteString("  encode zstd gzip\n\n")

		// WebSocket routes (must be first)
		wsRoutes := filterRoutes(plan.RoutingPolicy.Routes, true)
		for i, route := range wsRoutes {
			svc := findServiceByRouteName(route.Service, plan.Services)
			if svc == nil {
				continue
			}

			matcher := fmt.Sprintf("  @ws%d path %s*", i, route.Path)
			builder.WriteString(matcher + "\n")
			builder.WriteString(fmt.Sprintf("  handle @ws%d {\n", i))
			builder.WriteString(fmt.Sprintf("    reverse_proxy %s:%d {\n", svc.Name, svc.HealthCheck.Port))
			builder.WriteString("      transport http {\n")
			builder.WriteString("        keepalive 60s\n")
			builder.WriteString("        keepalive_idle_conns 100\n")
			builder.WriteString("        read_buffer 32k\n")
			builder.WriteString("        write_buffer 32k\n")
			builder.WriteString("        flush_interval -1\n")
			builder.WriteString("      }\n")
			builder.WriteString("      health_uri /health\n")
			builder.WriteString("      health_interval 30s\n")
			builder.WriteString("      health_timeout 10s\n")
			builder.WriteString("    }\n")
			builder.WriteString("  }\n\n")
		}

		// HTTP routes
		httpRoutes := filterRoutes(plan.RoutingPolicy.Routes, false)
		for _, route := range httpRoutes {
			svc := findServiceByRouteName(route.Service, plan.Services)
			if svc == nil {
				continue
			}

			if route.Path == "/" {
				// Default route (catch-all)
				builder.WriteString("  handle {\n")
				if route.StripPrefix {
					builder.WriteString("    uri strip_prefix /\n")
				}
				builder.WriteString(fmt.Sprintf("    reverse_proxy %s:%d\n", svc.Name, svc.HealthCheck.Port))
				builder.WriteString("  }\n\n")
			} else {
				matcher := fmt.Sprintf("  @%s path %s*", sanitizePathMatcher(route.Path), route.Path)
				builder.WriteString(matcher + "\n")
				builder.WriteString(fmt.Sprintf("  handle @%s {\n", sanitizePathMatcher(route.Path)))
				if route.StripPrefix {
					builder.WriteString(fmt.Sprintf("    uri strip_prefix %s\n", route.Path))
				}
				builder.WriteString(fmt.Sprintf("    reverse_proxy %s:%d\n", svc.Name, svc.HealthCheck.Port))
				builder.WriteString("  }\n\n")
			}
		}

		builder.WriteString("}\n\n")
	}

	// Individual service domains (keep for direct access)
	for _, route := range plan.Routes {
		builder.WriteString(fmt.Sprintf("https://%s, https://%s {\n", route.PrimaryHost, route.FallbackHost))
		builder.WriteString("  encode zstd gzip\n")
		builder.WriteString(fmt.Sprintf("  reverse_proxy %s\n", route.Upstream))
		builder.WriteString("}\n\n")
	}

	return builder.String()
}

func filterRoutes(routes []contracts.RoutePayload, websocket bool) []contracts.RoutePayload {
	var filtered []contracts.RoutePayload
	for _, r := range routes {
		if r.WebSocket == websocket {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func findServiceByRouteName(name string, services []ServiceRuntimeContext) *ServiceRuntimeContext {
	for i := range services {
		if services[i].Name == name {
			return &services[i]
		}
	}
	return nil
}

func sanitizePathMatcher(path string) string {
	// Convert "/api/v1" → "api_v1" for Caddy matcher name
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "-", "_")
	return path
}

// inferWebSocketPath returns a common WebSocket path prefix for a service
// based on service name conventions. Returns empty string if no WebSocket
// path is inferred.
func inferWebSocketPath(serviceName string) string {
	lower := strings.ToLower(serviceName)
	// Common WebSocket path conventions
	if strings.Contains(lower, "realtime") || strings.Contains(lower, "socket") || strings.Contains(lower, "ws") {
		return "/ws"
	}
	// For backend services, common WS paths
	if strings.Contains(lower, "backend") || strings.Contains(lower, "api") || strings.Contains(lower, "server") {
		return "/ws"
	}
	// For gateway/hub services
	if strings.Contains(lower, "gateway") || strings.Contains(lower, "hub") || strings.Contains(lower, "chat") {
		return "/ws"
	}
	return ""
}

func gatewayVersion(runtimeCtx RuntimeContext) string {
	base := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		runtimeCtx.Project.ProjectID,
		runtimeCtx.Binding.BindingID,
		runtimeCtx.Revision.RevisionID,
		runtimeCtx.Runtime.PlacementFingerprint,
		gatewayHostToken(runtimeCtx),
		runtimeCtx.Binding.DomainPolicy.Provider,
	)
	sum := sha256.Sum256([]byte(base))
	return "gw_" + hex.EncodeToString(sum[:8])
}

func gatewayHostToken(runtimeCtx RuntimeContext) string {
	candidates := []string{
		runtimeCtx.Binding.TargetRef,
		runtimeCtx.Binding.TargetID,
		runtimeCtx.Project.Slug,
		runtimeCtx.Project.ProjectID,
	}
	for _, candidate := range candidates {
		token := sanitizeHostToken(candidate)
		if token != "" {
			return token
		}
	}
	return "target"
}

func sanitizeHostToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.':
			builder.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', r == ' ':
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-.")
}

func preferredMagicProviders() (string, string) {
	return "sslip.io", "nip.io"
}

func collectPublicURLs(plan GatewayPlan) []string {
	urls := make([]string, 0, len(plan.Routes)*2)
	for _, route := range plan.Routes {
		urls = append(urls, route.PrimaryURL, route.FallbackURL)
	}
	return urls
}

func loadGatewayActivation(path string) (*GatewayActivation, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var activation GatewayActivation
	if err := json.Unmarshal(payload, &activation); err != nil {
		return nil, err
	}
	return &activation, nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	payload, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, payload, perm)
}
