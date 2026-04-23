package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/internal/secret"
)

type ProjectEnvService struct {
	projects         ProjectStore
	bundles          ProjectEnvBundleStore
	internalServices ProjectInternalServiceStore
	services         ProjectServiceStore
	routingStore     RoutingPolicyStore
	encryptionKey    string
}

func NewProjectEnvService(projects ProjectStore, bundles ProjectEnvBundleStore, internalServices ProjectInternalServiceStore, encryptionKey string) *ProjectEnvService {
	return &ProjectEnvService{
		projects:         projects,
		bundles:          bundles,
		internalServices: internalServices,
		encryptionKey:    strings.TrimSpace(encryptionKey),
	}
}

func (s *ProjectEnvService) WithServiceStore(services ProjectServiceStore) *ProjectEnvService {
	if s == nil {
		return s
	}
	s.services = services
	return s
}

func (s *ProjectEnvService) WithRoutingStore(store RoutingPolicyStore) *ProjectEnvService {
	if s == nil {
		return s
	}
	s.routingStore = store
	return s
}

func (s *ProjectEnvService) Get(requesterUserID, requesterRole, projectID string) (*ProjectEnvBundleRecord, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}
	return s.getForProject(project.ID)
}

func (s *ProjectEnvService) Upsert(cmd UpsertProjectEnvCommand) (*ProjectEnvBundleRecord, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(strings.ReplaceAll(cmd.Content, "\r\n", "\n"))
	if content == "" {
		return nil, ErrInvalidInput
	}
	if s.encryptionKey == "" {
		return nil, fmt.Errorf("project env encryption is not configured")
	}

	envMap, warnings, err := parseProjectEnvContent(content)
	if err != nil {
		return nil, err
	}
	serialized, err := serializeProjectEnvMap(envMap)
	if err != nil {
		return nil, err
	}
	encrypted, err := secret.Encrypt(serialized, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt project env bundle: %w", err)
	}

	keys := sortedEnvKeys(envMap)
	keysJSON, err := json.Marshal(keys)
	if err != nil {
		return nil, fmt.Errorf("serialize env keys: %w", err)
	}
	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		return nil, fmt.Errorf("serialize env warnings: %w", err)
	}

	bundle := &models.ProjectEnvBundle{
		ProjectID:         project.ID,
		EnvEncrypted:      encrypted,
		EnvFingerprint:    projectEnvFingerprint(serialized),
		KeyNamesJSON:      string(keysJSON),
		ParseWarningsJSON: string(warningsJSON),
		UpdatedBy:         strings.TrimSpace(cmd.RequesterUserID),
	}
	if err := s.bundles.Upsert(bundle); err != nil {
		return nil, err
	}

	return s.getForProject(project.ID)
}

func (s *ProjectEnvService) Delete(requesterUserID, requesterRole, projectID string) (*ProjectEnvBundleRecord, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.bundles.DeleteByProject(project.ID); err != nil {
		return nil, err
	}
	return s.getForProject(project.ID)
}

func (s *ProjectEnvService) LoadRuntimeEnv(projectID string) (map[string]string, error) {
	if s == nil || s.bundles == nil {
		return map[string]string{}, nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return map[string]string{}, ErrInvalidInput
	}
	bundle, err := s.bundles.GetByProject(projectID)
	if err != nil {
		return nil, err
	}
	if bundle == nil || strings.TrimSpace(bundle.EnvEncrypted) == "" {
		return map[string]string{}, nil
	}
	if s.encryptionKey == "" {
		return nil, fmt.Errorf("project env encryption is not configured")
	}
	plaintext, err := secret.Decrypt(bundle.EnvEncrypted, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt project env bundle: %w", err)
	}
	envMap, err := deserializeProjectEnvMap(plaintext)
	if err != nil {
		return nil, fmt.Errorf("decode project env bundle: %w", err)
	}
	if envMap == nil {
		return map[string]string{}, nil
	}
	return envMap, nil
}

func (s *ProjectEnvService) getForProject(projectID string) (*ProjectEnvBundleRecord, error) {
	if s == nil || s.bundles == nil {
		return nil, ErrInvalidInput
	}
	bundle, err := s.bundles.GetByProject(projectID)
	if err != nil {
		return nil, err
	}
	helperSnippets, err := s.loadHelperSnippets(projectID)
	if err != nil {
		return nil, err
	}
	annotateProvisionedHelperPacks(helperSnippets, nil)
	userKeys := []string{}
	record := &ProjectEnvBundleRecord{
		Configured:      bundle != nil,
		Keys:            []string{},
		UserKeys:        []string{},
		ManagedKeys:     collectManagedKeys(helperSnippets),
		ProvisionedKeys: []string{},
		ParseWarnings:   []string{},
		HelperPacks:     helperSnippets,
	}
	if bundle == nil {
		record.ProvisionedKeys = collectProvisionedKeys(helperSnippets, nil)
		return record, nil
	}
	if !bundle.UpdatedAt.IsZero() {
		updatedAt := bundle.UpdatedAt
		record.UpdatedAt = &updatedAt
	}
	record.Fingerprint = strings.TrimSpace(bundle.EnvFingerprint)
	if record.Keys, err = decodeStringArray(bundle.KeyNamesJSON); err != nil {
		return nil, fmt.Errorf("decode env keys: %w", err)
	}
	userKeys = append(userKeys, record.Keys...)
	record.UserKeys = append(record.UserKeys, record.Keys...)
	if record.ParseWarnings, err = decodeStringArray(bundle.ParseWarningsJSON); err != nil {
		return nil, fmt.Errorf("decode env warnings: %w", err)
	}
	annotateProvisionedHelperPacks(helperSnippets, userKeys)
	record.ManagedKeys = collectManagedKeys(helperSnippets)
	record.ProvisionedKeys = collectProvisionedKeys(helperSnippets, userKeys)
	return record, nil
}

func (s *ProjectEnvService) loadHelperSnippets(projectID string) ([]ProjectEnvHelperPack, error) {
	runtimeMode := ""
	if s.projects != nil {
		project, err := s.projects.GetByID(projectID)
		if err != nil {
			return nil, err
		}
		if project != nil {
			runtimeMode = strings.TrimSpace(project.RuntimeMode)
		}
	}
	if s.services != nil {
		items, err := s.services.ListByProject(projectID)
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			runtimeEnv, err := s.LoadRuntimeEnv(projectID)
			if err != nil {
				return nil, err
			}
			records := make([]ProjectServiceRecord, 0, len(items))
			for _, item := range items {
				record, err := ToProjectServiceRecord(item)
				if err != nil {
					return nil, err
				}
				records = append(records, record)
			}
			effectiveRouting, err := s.resolveEffectiveRouting(projectID, records)
			if err != nil {
				return nil, err
			}
			return buildProjectEnvHelperSnippetsFromServiceInventory(runtimeMode, records, runtimeEnv, effectiveRouting), nil
		}
	}
	internalServices := []models.ProjectInternalService{}
	if s.internalServices != nil {
		items, err := s.internalServices.ListByProject(projectID)
		if err != nil {
			return nil, err
		}
		internalServices = items
	}
	return buildProjectEnvHelperSnippets(internalServices), nil
}

func parseProjectEnvContent(content string) (map[string]string, []string, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	out := make(map[string]string)
	warnings := make([]string, 0)
	for index, rawLine := range lines {
		lineNo := index + 1
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		eqIndex := strings.IndexRune(line, '=')
		if eqIndex <= 0 {
			return nil, nil, fmt.Errorf("%w: line %d must be in KEY=VALUE form", ErrInvalidInput, lineNo)
		}
		key := strings.TrimSpace(line[:eqIndex])
		if !isValidProjectEnvKey(key) {
			return nil, nil, fmt.Errorf("%w: line %d has invalid key %q", ErrInvalidInput, lineNo, key)
		}
		value, err := normalizeProjectEnvValue(strings.TrimSpace(line[eqIndex+1:]))
		if err != nil {
			return nil, nil, fmt.Errorf("%w: line %d for key %q: %v", ErrInvalidInput, lineNo, key, err)
		}
		if _, exists := out[key]; exists {
			warnings = append(warnings, fmt.Sprintf("duplicate key %q detected; last value wins", key))
		}
		out[key] = value
	}
	return out, warnings, nil
}

func normalizeProjectEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) >= 2 {
		switch value[0] {
		case '\'', '"':
			if value[len(value)-1] != value[0] {
				return "", fmt.Errorf("unterminated quoted value")
			}
			if value[0] == '\'' {
				return value[1 : len(value)-1], nil
			}
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return "", err
			}
			return unquoted, nil
		}
	}
	return value, nil
}

func isValidProjectEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for index, r := range key {
		if index == 0 {
			if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				return false
			}
			continue
		}
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func serializeProjectEnvMap(envMap map[string]string) (string, error) {
	type entry struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	keys := sortedEnvKeys(envMap)
	entries := make([]entry, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, entry{Key: key, Value: envMap[key]})
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func deserializeProjectEnvMap(raw string) (map[string]string, error) {
	type entry struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var entries []entry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(entries))
	for _, item := range entries {
		out[item.Key] = item.Value
	}
	return out, nil
}

func decodeStringArray(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

func sortedEnvKeys(envMap map[string]string) []string {
	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func projectEnvFingerprint(serialized string) string {
	sum := sha256.Sum256([]byte(serialized))
	return hex.EncodeToString(sum[:])
}

func buildProjectEnvHelperSnippets(services []models.ProjectInternalService) []ProjectEnvHelperPack {
	items := make([]ProjectEnvHelperPack, 0, len(services))
	for _, item := range services {
		host, port := splitProjectEnvHostPort(item.LocalEndpoint)
		if host == "" || port == "" {
			continue
		}
		aliasKey := sanitizeProjectEnvAlias(item.Alias)
		runtimeKeys := []string{aliasKey + "_HOST", aliasKey + "_PORT", aliasKey + "_URL"}
		entry := ProjectEnvHelperPack{
			ServiceKind:     item.Kind,
			Alias:           item.Alias,
			Category:        "service_dependency",
			Audience:        "backend",
			SourceService:   item.Alias,
			PrimaryKey:      aliasKey + "_URL",
			Managed:         false,
			RuntimeInjected: false,
			PlaceholderEnv: map[string]string{
				aliasKey + "_HOST": "${" + aliasKey + "_HOST}",
				aliasKey + "_PORT": "${" + aliasKey + "_PORT}",
				aliasKey + "_URL":  "${" + aliasKey + "_URL}",
			},
			EnvExample: map[string]string{
				aliasKey + "_HOST": "",
				aliasKey + "_PORT": "",
				aliasKey + "_URL":  "",
			},
			LocalExampleEnv: map[string]string{
				aliasKey + "_HOST": host,
				aliasKey + "_PORT": port,
				aliasKey + "_URL":  projectEnvDependencyURL(item.Protocol, host, port),
			},
			RuntimeKeys: runtimeKeys,
			Notes: []string{
				"Local development can keep a local endpoint; production should use the service dependency URL or the managed runtime contract.",
			},
			LanguageSnippets: buildLanguageSnippets(aliasKey+"_URL", aliasKey+"_HOST", "Service Dependency"),
		}
		if strings.EqualFold(item.Kind, "postgres") {
			entry.Category = "database"
			entry.PrimaryKey = "DATABASE_URL"
			entry.PlaceholderEnv = map[string]string{"DATABASE_URL": "${DATABASE_URL}"}
			entry.EnvExample = map[string]string{"DATABASE_URL": ""}
			entry.LocalExampleEnv = map[string]string{"DATABASE_URL": "postgres://postgres:postgres@localhost:5432/app?sslmode=disable"}
			entry.RuntimeKeys = []string{"DATABASE_URL"}
			entry.Notes = []string{
				"Prefer a single DATABASE_URL in application config.",
				"LazyOps will inject the real runtime value for managed database keys during distributed-k3s deploys.",
			}
			entry.LanguageSnippets = buildLanguageSnippets("DATABASE_URL", "", "Database")
		}
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ServiceKind == items[j].ServiceKind {
			return items[i].Alias < items[j].Alias
		}
		return items[i].ServiceKind < items[j].ServiceKind
	})
	return items
}

func buildProjectEnvHelperSnippetsFromServiceInventory(runtimeMode string, services []ProjectServiceRecord, projectEnv map[string]string, effectiveRouting RoutingPolicyRecord) []ProjectEnvHelperPack {
	items := make([]ProjectEnvHelperPack, 0)
	serviceIndex := make(map[string]ProjectServiceRecord, len(services))
	relationalDependents := make(map[string][]string)
	for _, item := range services {
		serviceIndex[item.Name] = item
		if strings.TrimSpace(item.ConnectionTargetService) == "" {
			continue
		}
		targetName := strings.TrimSpace(item.ConnectionTargetService)
		relationalDependents[targetName] = append(relationalDependents[targetName], item.Name)
	}
	for _, item := range services {
		if item.SourceType != serviceSourceTypeInternal || !isRelationalDatabaseKind(item.Kind) {
			continue
		}
		template := coerceConnectionTemplateForKind(item.Kind, item.ConnectionTemplate)
		primaryKey := firstNonEmpty(template["DB_URL"], "DATABASE_URL")
		runtimeValues := buildRelationalConnectionRuntimeValues(item, projectEnv, runtimeMode)
		runtimeEnv := buildRelationalConnectionTemplateEnv(item, projectEnv, runtimeMode)
		runtimeKeys := sortedEnvKeys(runtimeEnv)
		envExample := make(map[string]string, len(template))
		placeholderEnv := make(map[string]string, len(template))
		localExampleEnv := make(map[string]string, len(template))
		for _, slot := range relationalConnectionTemplateSlots {
			envName := strings.TrimSpace(template[slot])
			if envName == "" {
				continue
			}
			envExample[envName] = ""
			placeholderEnv[envName] = "${" + envName + "}"
			localExampleEnv[envName] = localRelationalExampleValue(item.Kind, slot)
		}
		relatedServices := append([]string{item.Name}, relationalDependents[item.Name]...)
		sort.Strings(relatedServices)
		items = append(items, ProjectEnvHelperPack{
			ServiceKind:     item.Kind,
			Alias:           item.Name,
			Category:        "database",
			Audience:        "backend",
			SourceService:   item.Name,
			RelatedServices: relatedServices,
			PrimaryKey:      primaryKey,
			Managed:         true,
			RuntimeInjected: true,
			PlaceholderEnv:  placeholderEnv,
			EnvExample:      envExample,
			LocalExampleEnv: localExampleEnv,
			RuntimeKeys:     runtimeKeys,
			Notes: []string{
				"Prefer a single database URL in code and framework config.",
				"LazyOps injects managed database keys for distributed-k3s; user-defined env values win when already present.",
			},
			LanguageSnippets: buildRelationalLanguageSnippets(primaryKey, template["DB_HOST"], runtimeValues),
		})
	}

	for _, route := range effectiveRouting.Routes {
		service, ok := serviceIndex[route.Service]
		if !ok {
			continue
		}
		publicPath := normalizedPublicPath(route.Path)
		if route.WebSocket {
			items = append(items, ProjectEnvHelperPack{
				ServiceKind:     service.Kind,
				Alias:           route.Service,
				Category:        "websocket",
				Audience:        "frontend-browser",
				SourceService:   route.Service,
				RelatedServices: []string{route.Service},
				PrimaryKey:      "WS_URL",
				PublicPath:      publicPath,
				Managed:         false,
				RuntimeInjected: false,
				PlaceholderEnv:  map[string]string{"WS_URL": "${WS_URL}"},
				EnvExample:      map[string]string{"WS_URL": ""},
				LocalExampleEnv: map[string]string{"WS_URL": localWebSocketExample(service, publicPath)},
				RuntimeKeys:     []string{},
				Notes: []string{
					"Browser clients should connect via the public WebSocket path, not an internal service DNS name.",
					"When using a custom route, update client snippets to match the effective public path shown here.",
				},
				LanguageSnippets: buildLanguageSnippets("WS_URL", "", "WebSocket"),
			})
			continue
		}
		if publicPath == "/" && isFrontendDescriptor(routingDescriptor{Name: service.Name, Kind: service.Kind, RuntimeProfile: service.RuntimeProfile, Public: service.Public}) {
			continue
		}
		items = append(items, ProjectEnvHelperPack{
			ServiceKind:     service.Kind,
			Alias:           route.Service,
			Category:        "browser_api",
			Audience:        "frontend-browser",
			SourceService:   route.Service,
			RelatedServices: []string{route.Service},
			PrimaryKey:      "API_BASE_URL",
			PublicPath:      publicPath,
			Managed:         false,
			RuntimeInjected: false,
			PlaceholderEnv:  map[string]string{"API_BASE_URL": "${API_BASE_URL}"},
			EnvExample:      map[string]string{"API_BASE_URL": ""},
			LocalExampleEnv: map[string]string{"API_BASE_URL": localHTTPExample(service, publicPath)},
			RuntimeKeys:     []string{},
			Notes: []string{
				"Browser code should use the public path below or a same-origin base URL.",
				"Do not point browser code at internal Kubernetes service DNS names.",
			},
			LanguageSnippets: buildLanguageSnippets("API_BASE_URL", "", "Public API"),
		})
		internalBaseURL := internalHTTPURL(service)
		if internalBaseURL != "" {
			items = append(items, ProjectEnvHelperPack{
				ServiceKind:     service.Kind,
				Alias:           route.Service + "-internal",
				Category:        "server_api",
				Audience:        "frontend-server",
				SourceService:   route.Service,
				RelatedServices: []string{route.Service},
				PrimaryKey:      "API_INTERNAL_URL",
				PublicPath:      publicPath,
				Managed:         false,
				RuntimeInjected: false,
				PlaceholderEnv:  map[string]string{"API_INTERNAL_URL": "${API_INTERNAL_URL}"},
				EnvExample:      map[string]string{"API_INTERNAL_URL": ""},
				LocalExampleEnv: map[string]string{"API_INTERNAL_URL": internalBaseURL},
				RuntimeKeys:     []string{},
				Notes: []string{
					"Server-side services can call the internal service URL when they run inside the cluster.",
					"Browser-side code should keep using the public path instead.",
				},
				LanguageSnippets: buildLanguageSnippets("API_INTERNAL_URL", "", "Internal API"),
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			return items[i].Alias < items[j].Alias
		}
		return items[i].Category < items[j].Category
	})
	return items
}

func (s *ProjectEnvService) resolveEffectiveRouting(projectID string, services []ProjectServiceRecord) (RoutingPolicyRecord, error) {
	descriptors := make([]routingDescriptor, 0, len(services))
	serviceIndex := make(map[string]ProjectServiceRecord, len(services))
	for _, item := range services {
		serviceIndex[item.Name] = item
		descriptors = append(descriptors, routingDescriptor{
			Name:           item.Name,
			Kind:           item.Kind,
			RuntimeProfile: item.RuntimeProfile,
			Public:         item.Public,
		})
	}
	suggested := buildDefaultRoutingPolicy("", descriptors)
	if s == nil || s.routingStore == nil || strings.TrimSpace(projectID) == "" {
		return suggested, nil
	}
	policy, err := s.routingStore.GetByProjectID(projectID)
	if err != nil {
		return RoutingPolicyRecord{}, err
	}
	if policy == nil {
		return suggested, nil
	}
	routes, err := parseRoutes(policy.RoutesJSON)
	if err != nil {
		return RoutingPolicyRecord{}, err
	}
	if len(routes) == 0 {
		return suggested, nil
	}
	out := RoutingPolicyRecord{
		SharedDomain: strings.TrimSpace(policy.SharedDomain),
		Routes:       make([]RoutingRouteRecord, 0, len(routes)),
	}
	for _, route := range routes {
		if _, ok := serviceIndex[route.Service]; !ok {
			continue
		}
		record := RoutingRouteRecord{
			Path:        route.Path,
			Service:     route.Service,
			Port:        route.Port,
			WebSocket:   route.WebSocket,
			StripPrefix: route.StripPrefix,
			CreatedAt:   policy.CreatedAt,
		}
		out.Routes = append(out.Routes, normalizeRoutingRoute(record, routingDescriptor{
			Name:           serviceIndex[route.Service].Name,
			Kind:           serviceIndex[route.Service].Kind,
			RuntimeProfile: serviceIndex[route.Service].RuntimeProfile,
			Public:         serviceIndex[route.Service].Public,
		}))
	}
	if len(out.Routes) == 0 {
		return suggested, nil
	}
	return out, nil
}

func collectManagedKeys(packs []ProjectEnvHelperPack) []string {
	keys := make(map[string]struct{})
	for _, pack := range packs {
		if !pack.Managed {
			continue
		}
		for _, key := range pack.RuntimeKeys {
			keys[key] = struct{}{}
		}
	}
	return sortedEnvKeySet(keys)
}

func collectProvisionedKeys(packs []ProjectEnvHelperPack, userKeys []string) []string {
	userKeySet := make(map[string]struct{}, len(userKeys))
	for _, key := range userKeys {
		userKeySet[key] = struct{}{}
	}
	keys := make(map[string]struct{})
	for _, pack := range packs {
		if !pack.Managed {
			continue
		}
		for _, key := range pack.RuntimeKeys {
			if _, overridden := userKeySet[key]; overridden {
				continue
			}
			keys[key] = struct{}{}
		}
	}
	return sortedEnvKeySet(keys)
}

func annotateProvisionedHelperPacks(packs []ProjectEnvHelperPack, userKeys []string) {
	userKeySet := make(map[string]struct{}, len(userKeys))
	for _, key := range userKeys {
		userKeySet[key] = struct{}{}
	}
	for index := range packs {
		provisioned := make([]string, 0, len(packs[index].RuntimeKeys))
		for _, key := range packs[index].RuntimeKeys {
			if _, overridden := userKeySet[key]; overridden {
				continue
			}
			provisioned = append(provisioned, key)
		}
		packs[index].ProvisionedKeys = provisioned
		packs[index].RuntimeInjected = packs[index].Managed && len(provisioned) > 0
	}
}

func sortedEnvKeySet(items map[string]struct{}) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func buildLanguageSnippets(primaryKey, secondaryKey, title string) []ProjectEnvHelperSnippet {
	envTitle := title
	if strings.TrimSpace(envTitle) == "" {
		envTitle = "Runtime Config"
	}
	secondaryComment := ""
	if strings.TrimSpace(secondaryKey) != "" {
		secondaryComment = "\n# Optional fallback key: " + secondaryKey
	}
	return []ProjectEnvHelperSnippet{
		{
			Language:  "nodejs",
			Framework: "native",
			Kind:      "code",
			Title:     envTitle + " / Node.js",
			Content:   "const value = process.env." + primaryKey + " ?? '';\nif (!value) throw new Error('" + primaryKey + " is required');",
		},
		{
			Language:  "python",
			Framework: "native",
			Kind:      "code",
			Title:     envTitle + " / Python",
			Content:   "import os\nvalue = os.getenv('" + primaryKey + "', '')\nif not value:\n    raise RuntimeError('" + primaryKey + " is required')",
		},
		{
			Language:  "java",
			Framework: "spring",
			Kind:      "config_file",
			Title:     envTitle + " / Spring application.yml",
			Content:   "app:\n  value: ${" + primaryKey + ":}" + secondaryComment,
		},
		{
			Language:  "java",
			Framework: "native",
			Kind:      "code",
			Title:     envTitle + " / Java",
			Content:   "String value = System.getenv(\"" + primaryKey + "\");\nif (value == null || value.isBlank()) throw new IllegalStateException(\"" + primaryKey + " is required\");",
		},
		{
			Language:  "csharp",
			Framework: ".net",
			Kind:      "code",
			Title:     envTitle + " / C#",
			Content:   "var value = Environment.GetEnvironmentVariable(\"" + primaryKey + "\") ?? string.Empty;\nif (string.IsNullOrWhiteSpace(value)) throw new InvalidOperationException(\"" + primaryKey + " is required\");",
		},
		{
			Language:  "go",
			Framework: "native",
			Kind:      "code",
			Title:     envTitle + " / Go",
			Content:   "value := os.Getenv(\"" + primaryKey + "\")\nif value == \"\" {\n\tlog.Fatal(\"" + primaryKey + " is required\")\n}",
		},
		{
			Language:  "php",
			Framework: "native",
			Kind:      "code",
			Title:     envTitle + " / PHP",
			Content:   "$value = getenv('" + primaryKey + "') ?: '';\nif ($value === '') {\n    throw new RuntimeException('" + primaryKey + " is required');\n}",
		},
	}
}

func buildRelationalLanguageSnippets(primaryKey, secondaryKey string, runtimeValues map[string]string) []ProjectEnvHelperSnippet {
	snippets := buildLanguageSnippets(primaryKey, secondaryKey, "Database")
	dbNameKey := firstNonEmptyCompiledValue(secondaryKey, "DB_HOST")
	decomposedBlock := fmt.Sprintf(
		"db:\n  url: ${%s:}\n  name: ${DB_NAME:}\n  host: ${%s:}\n  port: ${DB_PORT:}\n  username: ${DB_USERNAME:}\n  password: ${DB_PASSWORD:}",
		primaryKey,
		dbNameKey,
	)
	fallbackCode := fmt.Sprintf(
		"const url = process.env.%s ?? '';\nconst dbConfig = {\n  name: process.env.DB_NAME ?? '',\n  host: process.env.%s ?? '',\n  port: process.env.DB_PORT ?? '',\n  username: process.env.DB_USERNAME ?? '',\n  password: process.env.DB_PASSWORD ?? '',\n};",
		primaryKey,
		dbNameKey,
	)
	snippets = append(snippets,
		ProjectEnvHelperSnippet{
			Language:  "nodejs",
			Framework: "native",
			Kind:      "config_file",
			Title:     "Database / Node.js decomposed fallback",
			Content:   fallbackCode,
		},
		ProjectEnvHelperSnippet{
			Language:  "java",
			Framework: "spring",
			Kind:      "config_file",
			Title:     "Database / Spring decomposed application.yml",
			Content:   decomposedBlock,
		},
		ProjectEnvHelperSnippet{
			Language:  "csharp",
			Framework: ".net",
			Kind:      "config_file",
			Title:     "Database / .NET appsettings fallback",
			Content:   "{\n  \"Database\": {\n    \"Url\": \"${" + primaryKey + "}\",\n    \"Name\": \"${DB_NAME}\",\n    \"Host\": \"${" + dbNameKey + "}\",\n    \"Port\": \"${DB_PORT}\",\n    \"Username\": \"${DB_USERNAME}\",\n    \"Password\": \"${DB_PASSWORD}\"\n  }\n}",
		},
	)
	_ = runtimeValues
	return snippets
}

func localRelationalExampleValue(kind, slot string) string {
	switch strings.ToUpper(strings.TrimSpace(slot)) {
	case "DB_URL":
		if normalizeManagedInternalBridgeKind(kind) == "mysql" {
			return "mysql://mysql:mysql@tcp(localhost:3306)/app"
		}
		return "postgres://postgres:postgres@localhost:5432/app?sslmode=disable"
	case "DB_NAME":
		return "app"
	case "DB_HOST":
		return "localhost"
	case "DB_PORT":
		if normalizeManagedInternalBridgeKind(kind) == "mysql" {
			return "3306"
		}
		return "5432"
	case "DB_USERNAME":
		if normalizeManagedInternalBridgeKind(kind) == "mysql" {
			return "mysql"
		}
		return "postgres"
	case "DB_PASSWORD":
		if normalizeManagedInternalBridgeKind(kind) == "mysql" {
			return "mysql"
		}
		return "postgres"
	default:
		return ""
	}
}

func normalizedPublicPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

func internalHTTPURL(service ProjectServiceRecord) string {
	port := firstPositive(service.ServicePort, service.TargetPort)
	if port <= 0 {
		return ""
	}
	return "http://" + strings.TrimSpace(service.Name) + ":" + strconv.Itoa(port)
}

func localHTTPExample(service ProjectServiceRecord, publicPath string) string {
	port := firstPositive(service.ServicePort, service.TargetPort, 8080)
	return "http://localhost:" + strconv.Itoa(port) + publicPath
}

func localWebSocketExample(service ProjectServiceRecord, publicPath string) string {
	port := firstPositive(service.ServicePort, service.TargetPort, 8080)
	return "ws://localhost:" + strconv.Itoa(port) + publicPath
}

func projectEnvDependencyURL(protocol, host, port string) string {
	scheme := strings.ToLower(strings.TrimSpace(protocol))
	if scheme == "" {
		scheme = "tcp"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

func sanitizeProjectEnvAlias(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return "SERVICE"
	}
	var buf bytes.Buffer
	for _, r := range alias {
		switch {
		case r >= 'a' && r <= 'z':
			buf.WriteRune(r - 32)
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			buf.WriteRune(r)
		default:
			buf.WriteRune('_')
		}
	}
	return buf.String()
}

func splitProjectEnvHostPort(endpoint string) (string, string) {
	value := strings.TrimSpace(endpoint)
	if value == "" {
		return "", ""
	}
	host, port, err := net.SplitHostPort(value)
	if err == nil {
		return strings.TrimSpace(host), strings.TrimSpace(port)
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}
