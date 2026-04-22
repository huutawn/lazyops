package service

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"lazyops-server/internal/models"
)

type ProjectAIPromptService struct {
	projects ProjectStore
	services ProjectServiceStore
	envs     *ProjectEnvService
	routing  *RoutingService
}

func NewProjectAIPromptService(projects ProjectStore, services ProjectServiceStore, envs *ProjectEnvService, routing *RoutingService) *ProjectAIPromptService {
	return &ProjectAIPromptService{
		projects: projects,
		services: services,
		envs:     envs,
		routing:  routing,
	}
}

func (s *ProjectAIPromptService) Get(requesterUserID, requesterRole, projectID string) (*ProjectAIPromptRecord, error) {
	if s == nil || s.projects == nil || s.services == nil || s.envs == nil || s.routing == nil {
		return nil, ErrInvalidInput
	}

	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}

	envRecord, err := s.envs.getForProject(project.ID)
	if err != nil {
		return nil, err
	}
	runtimeEnv, err := s.envs.LoadRuntimeEnv(project.ID)
	if err != nil {
		return nil, err
	}

	serviceModels, err := s.services.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}
	serviceRecords := make([]ProjectServiceRecord, 0, len(serviceModels))
	for _, item := range serviceModels {
		record, convErr := ToProjectServiceRecord(item)
		if convErr != nil {
			return nil, convErr
		}
		serviceRecords = append(serviceRecords, record)
	}

	routingResult, err := s.routing.GetRouting(requesterUserID, requesterRole, project.ID)
	if err != nil {
		return nil, err
	}

	serviceSnapshot := buildProjectAIPromptServiceSnapshot(serviceRecords, routingResult.EffectivePublicPaths)
	migrationFindings := buildProjectAIPromptMigrationFindings(runtimeEnv, envRecord.HelperPacks, routingResult)
	sourceSections := buildProjectAIPromptSourceSections(serviceSnapshot, envRecord.ManagedKeys, routingResult.EffectivePublicPaths, migrationFindings)
	prompt := composeProjectAIPrompt(*project, envRecord.HelperPacks, serviceSnapshot, routingResult, migrationFindings)
	summary := fmt.Sprintf(
		"Project-wide prompt with %d services, %d managed keys, %d effective public paths, and %d migration findings.",
		len(serviceSnapshot),
		len(envRecord.ManagedKeys),
		len(routingResult.EffectivePublicPaths),
		len(migrationFindings),
	)

	return &ProjectAIPromptRecord{
		Title:                fmt.Sprintf("AI migration prompt for %s", firstRuntimeNonEmpty(strings.TrimSpace(project.Name), strings.TrimSpace(project.Slug), project.ID)),
		Summary:              summary,
		Prompt:               prompt,
		ServiceSnapshot:      serviceSnapshot,
		EffectivePublicPaths: append([]RoutingGuidanceRouteRecord{}, routingResult.EffectivePublicPaths...),
		ManagedKeys:          append([]string{}, envRecord.ManagedKeys...),
		MigrationFindings:    migrationFindings,
		SourceSections:       sourceSections,
	}, nil
}

func buildProjectAIPromptSourceSections(serviceSnapshot []ProjectAIPromptServiceSnapshot, managedKeys []string, effectivePaths []RoutingGuidanceRouteRecord, findings []MigrationFindingRecord) []ProjectAIPromptSourceSection {
	return []ProjectAIPromptSourceSection{
		{
			Key:         "services",
			Title:       "Services",
			Description: "Project services included in the prompt context.",
			ItemCount:   len(serviceSnapshot),
		},
		{
			Key:         "managed_keys",
			Title:       "Managed Keys",
			Description: "Safe placeholder env keys the AI should prefer.",
			ItemCount:   len(managedKeys),
		},
		{
			Key:         "effective_public_paths",
			Title:       "Effective Paths",
			Description: "Browser/API/WS public routes the AI must respect.",
			ItemCount:   len(effectivePaths),
		},
		{
			Key:         "migration_findings",
			Title:       "Migration Findings",
			Description: "Localhost or custom-path issues the AI should address.",
			ItemCount:   len(findings),
		},
	}
}

func buildProjectAIPromptServiceSnapshot(services []ProjectServiceRecord, effectivePaths []RoutingGuidanceRouteRecord) []ProjectAIPromptServiceSnapshot {
	pathByService := make(map[string]RoutingGuidanceRouteRecord, len(effectivePaths))
	for _, route := range effectivePaths {
		key := route.Service + fmt.Sprintf("|%t", route.WebSocket)
		pathByService[key] = route
	}

	items := make([]ProjectAIPromptServiceSnapshot, 0, len(services))
	for _, item := range services {
		descriptor := routingDescriptor{
			Name:           item.Name,
			Kind:           item.Kind,
			RuntimeProfile: item.RuntimeProfile,
			Public:         item.Public,
		}
		publicRoute, hasPublicRoute := pathByService[item.Name+"|false"]
		wsRoute, hasWSRoute := pathByService[item.Name+"|true"]

		role := "service"
		switch {
		case isDatabaseProjectService(item):
			role = "database"
		case hasWSRoute:
			role = "websocket"
		case hasPublicRoute && normalizedPublicPath(publicRoute.Path) == "/" && isFrontendDescriptor(descriptor):
			role = "frontend-root"
		case isFrontendDescriptor(descriptor):
			role = "frontend"
		case isAPIDescriptor(descriptor) || isBackendDescriptor(descriptor):
			role = "api"
		}

		internalURL := ""
		if !isDatabaseProjectService(item) {
			internalURL = internalHTTPURL(item)
		}
		publicPath := ""
		if hasWSRoute {
			publicPath = normalizedPublicPath(wsRoute.Path)
		} else if hasPublicRoute {
			publicPath = normalizedPublicPath(publicRoute.Path)
		}

		items = append(items, ProjectAIPromptServiceSnapshot{
			Name:           item.Name,
			Kind:           item.Kind,
			Role:           role,
			RuntimeProfile: item.RuntimeProfile,
			SourceType:     item.SourceType,
			Public:         item.Public,
			Managed:        item.ManagedByLazyops,
			WebSocket:      hasWSRoute,
			PublicPath:     publicPath,
			InternalURL:    internalURL,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func buildProjectAIPromptMigrationFindings(runtimeEnv map[string]string, helperPacks []ProjectEnvHelperPack, routingResult *ProjectRoutingResult) []MigrationFindingRecord {
	findings := make([]MigrationFindingRecord, 0)
	if routingResult == nil {
		return findings
	}

	apiPublicPath := firstPromptPublicPath(routingResult.EffectivePublicPaths, "browser-api", false)
	if apiPublicPath == "" {
		apiPublicPath = "/api"
	}
	wsPublicPath := firstPromptPublicPath(routingResult.EffectivePublicPaths, "websocket", true)
	if wsPublicPath == "" {
		wsPublicPath = "/ws"
	}
	databasePlaceholder := firstPromptPlaceholder(helperPacks, "database")
	if databasePlaceholder == "" {
		databasePlaceholder = "${DATABASE_URL}"
	}
	internalAPIPlaceholder := firstPromptPlaceholder(helperPacks, "server_api")
	internalAPIExample := firstPromptLocalExample(helperPacks, "server_api")

	envKeys := make([]string, 0, len(runtimeEnv))
	for key := range runtimeEnv {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)

	for _, key := range envKeys {
		value := strings.TrimSpace(runtimeEnv[key])
		if !containsLocalReference(value) {
			continue
		}
		category := classifyPromptEnvFinding(key, value)
		recommended := ""
		switch category {
		case "database_internal":
			recommended = databasePlaceholder
		case "websocket":
			recommended = wsPublicPath
		case "browser_api":
			recommended = apiPublicPath
		default:
			recommended = firstRuntimeNonEmpty(internalAPIPlaceholder, internalAPIExample, "${API_INTERNAL_URL}")
		}

		findings = append(findings, MigrationFindingRecord{
			Category:         category,
			Severity:         "warning",
			ServiceName:      "",
			CurrentValue:     fmt.Sprintf("%s -> %s", key, summarizeLocalReference(value)),
			RecommendedValue: recommended,
			Message:          fmt.Sprintf("Env key %q still points at a local-only address. Replace it with a managed placeholder or the cluster/public route shown above.", key),
		})
	}

	seenWarnings := make(map[string]struct{}, len(routingResult.Warnings))
	for _, warning := range routingResult.Warnings {
		text := strings.TrimSpace(warning)
		if text == "" {
			continue
		}
		if _, exists := seenWarnings[text]; exists {
			continue
		}
		seenWarnings[text] = struct{}{}
		findings = append(findings, MigrationFindingRecord{
			Category: "routing",
			Severity: "info",
			Message:  text,
		})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity < findings[j].Severity
		}
		if findings[i].Category != findings[j].Category {
			return findings[i].Category < findings[j].Category
		}
		return findings[i].Message < findings[j].Message
	})
	return findings
}

func composeProjectAIPrompt(project models.Project, helperPacks []ProjectEnvHelperPack, serviceSnapshot []ProjectAIPromptServiceSnapshot, routingResult *ProjectRoutingResult, findings []MigrationFindingRecord) string {
	builder := &strings.Builder{}
	runtimeMode := firstRuntimeNonEmpty(strings.TrimSpace(project.RuntimeMode), "distributed-k3s")
	sharedDomain := ""
	if routingResult != nil {
		sharedDomain = strings.TrimSpace(routingResult.RoutingPolicy.SharedDomain)
	}

	builder.WriteString(fmt.Sprintf("You are helping migrate this project from localhost-style config to LazyOps %s runtime.\n\n", runtimeMode))
	builder.WriteString("Context:\n")
	builder.WriteString(fmt.Sprintf("- Project: %s\n", firstRuntimeNonEmpty(strings.TrimSpace(project.Name), strings.TrimSpace(project.Slug), project.ID)))
	builder.WriteString(fmt.Sprintf("- Runtime mode: %s\n", runtimeMode))
	if sharedDomain != "" {
		builder.WriteString(fmt.Sprintf("- Shared domain: %s\n", sharedDomain))
	} else {
		builder.WriteString("- Shared domain: none configured yet\n")
	}

	builder.WriteString("\nService inventory:\n")
	if len(serviceSnapshot) == 0 {
		builder.WriteString("- No services were detected.\n")
	} else {
		for _, item := range serviceSnapshot {
			line := fmt.Sprintf("- %s | role=%s | kind=%s | runtime=%s | source=%s | public=%t", item.Name, item.Role, item.Kind, firstRuntimeNonEmpty(item.RuntimeProfile, "unknown"), firstRuntimeNonEmpty(item.SourceType, "repo"), item.Public)
			if item.Managed {
				line += " | managed=true"
			}
			if item.WebSocket {
				line += " | websocket=true"
			}
			if item.PublicPath != "" {
				line += " | public_path=" + item.PublicPath
			}
			if item.InternalURL != "" {
				line += " | internal_url=" + item.InternalURL
			}
			builder.WriteString(line + "\n")
		}
	}

	builder.WriteString("\nManaged env guidance:\n")
	if len(helperPacks) == 0 {
		builder.WriteString("- No helper packs are available.\n")
	} else {
		for _, pack := range helperPacks {
			builder.WriteString(fmt.Sprintf(
				"- %s | source_service=%s | audience=%s | primary_key=%s | placeholders=%s",
				pack.Category,
				firstRuntimeNonEmpty(pack.SourceService, pack.Alias),
				firstRuntimeNonEmpty(pack.Audience, "unknown"),
				firstRuntimeNonEmpty(pack.PrimaryKey, "n/a"),
				promptEnvMapSummary(pack.PlaceholderEnv),
			))
			if pack.PublicPath != "" {
				builder.WriteString(" | public_path=" + pack.PublicPath)
			}
			if pack.Managed {
				builder.WriteString(" | managed=true")
			}
			if pack.RuntimeInjected {
				builder.WriteString(" | runtime_injected=true")
			}
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\nEffective public paths:\n")
	if routingResult == nil || len(routingResult.EffectivePublicPaths) == 0 {
		builder.WriteString("- No effective public paths are configured yet.\n")
	} else {
		for _, route := range routingResult.EffectivePublicPaths {
			line := fmt.Sprintf("- %s -> %s", route.Service, normalizedPublicPath(route.Path))
			if route.WebSocket {
				line += " (websocket)"
			}
			if route.Audience != "" {
				line += " | audience=" + route.Audience
			}
			builder.WriteString(line + "\n")
		}
	}

	builder.WriteString("\nMigration findings:\n")
	if len(findings) == 0 {
		builder.WriteString("- No localhost or custom-path migration findings were detected from the current project metadata.\n")
	} else {
		for _, finding := range findings {
			line := fmt.Sprintf("- [%s] %s", strings.ToUpper(firstRuntimeNonEmpty(finding.Severity, "info")), finding.Message)
			if finding.CurrentValue != "" {
				line += " | current=" + finding.CurrentValue
			}
			if finding.RecommendedValue != "" {
				line += " | recommended=" + finding.RecommendedValue
			}
			builder.WriteString(line + "\n")
		}
	}

	builder.WriteString("\nRules you must follow:\n")
	builder.WriteString("- Internal service-to-service calls must use Kubernetes service DNS or cluster-safe internal URLs, never localhost.\n")
	builder.WriteString("- Browser-side HTTP calls must use the effective public paths shown above, not internal DNS names.\n")
	builder.WriteString("- Browser-side WebSocket calls must use the effective public WebSocket path shown above.\n")
	builder.WriteString("- Managed env keys must stay placeholders like ${DATABASE_URL}; do not invent or fill real secret values.\n")
	builder.WriteString("- Do not expose internal databases or other internal-only services publicly.\n")
	builder.WriteString("- Respect custom public paths already configured here. If frontend and backend both need root semantics, propose /api or a subdomain instead of keeping two services on /.\n")

	builder.WriteString("\nTask:\n")
	builder.WriteString("- Identify the likely files or config surfaces that need changes. If the exact filenames are unknown, say which files are likely and state that as an assumption.\n")
	builder.WriteString("- Propose concrete code, env, and config changes to replace localhost with managed placeholders, internal service DNS, or effective public paths.\n")
	builder.WriteString("- Prefer DATABASE_URL, API_BASE_URL, API_INTERNAL_URL, and WS_URL style variables when they fit the context.\n")
	builder.WriteString("- Separate server-side internal calls from browser-side public calls.\n")
	builder.WriteString("- Return the answer in this format: Summary, Files to change, Env/config changes, Code snippets, Notes.\n")

	return builder.String()
}

func isDatabaseProjectService(item ProjectServiceRecord) bool {
	lowerKind := strings.ToLower(strings.TrimSpace(item.Kind))
	lowerRuntime := strings.ToLower(strings.TrimSpace(item.RuntimeProfile))
	switch lowerKind {
	case "postgres", "mysql", "redis", "rabbitmq", "internal-db", "database":
		return true
	}
	return strings.Contains(lowerRuntime, "db")
}

func firstPromptPublicPath(items []RoutingGuidanceRouteRecord, audience string, websocket bool) string {
	for _, item := range items {
		if item.WebSocket != websocket {
			continue
		}
		if audience != "" && item.Audience != audience {
			continue
		}
		return normalizedPublicPath(item.Path)
	}
	for _, item := range items {
		if item.WebSocket == websocket {
			return normalizedPublicPath(item.Path)
		}
	}
	return ""
}

func firstPromptPlaceholder(packs []ProjectEnvHelperPack, category string) string {
	for _, pack := range packs {
		if pack.Category != category {
			continue
		}
		key := strings.TrimSpace(pack.PrimaryKey)
		if key == "" {
			continue
		}
		return "${" + key + "}"
	}
	return ""
}

func firstPromptLocalExample(packs []ProjectEnvHelperPack, category string) string {
	for _, pack := range packs {
		if pack.Category != category {
			continue
		}
		if value, ok := pack.LocalExampleEnv[pack.PrimaryKey]; ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
		for _, value := range pack.LocalExampleEnv {
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func promptEnvMapSummary(items map[string]string) string {
	if len(items) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, items[key]))
	}
	return strings.Join(parts, ", ")
}

func containsLocalReference(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") || strings.Contains(lower, "[::1]") || strings.Contains(lower, "::1")
}

func summarizeLocalReference(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		host := parsed.Hostname()
		switch {
		case host == "localhost":
			return parsed.Scheme + "://localhost"
		case host == "127.0.0.1":
			return parsed.Scheme + "://127.0.0.1"
		case host == "::1":
			return parsed.Scheme + "://[::1]"
		}
	}
	switch {
	case strings.Contains(lower, "ws://localhost"):
		return "ws://localhost"
	case strings.Contains(lower, "wss://localhost"):
		return "wss://localhost"
	case strings.Contains(lower, "http://localhost"):
		return "http://localhost"
	case strings.Contains(lower, "https://localhost"):
		return "https://localhost"
	case strings.Contains(lower, "127.0.0.1"):
		return "127.0.0.1"
	case strings.Contains(lower, "::1"):
		return "::1"
	default:
		return "localhost"
	}
}

func classifyPromptEnvFinding(key, value string) string {
	keyLower := strings.ToLower(strings.TrimSpace(key))
	valueLower := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(keyLower, "ws") || strings.HasPrefix(valueLower, "ws://") || strings.HasPrefix(valueLower, "wss://"):
		return "websocket"
	case strings.Contains(keyLower, "db") || strings.Contains(keyLower, "database") || strings.Contains(keyLower, "pg") ||
		strings.HasPrefix(valueLower, "postgres://") || strings.HasPrefix(valueLower, "mysql://") || strings.HasPrefix(valueLower, "redis://"):
		return "database_internal"
	case strings.HasPrefix(keyLower, "next_public_") || strings.HasPrefix(keyLower, "vite_") || strings.HasPrefix(keyLower, "react_app_") || strings.HasPrefix(keyLower, "nuxt_public_") || strings.HasPrefix(keyLower, "public_"):
		return "browser_api"
	default:
		return "server_http_internal"
	}
}
