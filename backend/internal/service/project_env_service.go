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
	record := &ProjectEnvBundleRecord{
		Configured:     bundle != nil,
		Keys:           []string{},
		ParseWarnings:  []string{},
		HelperSnippets: helperSnippets,
	}
	if bundle == nil {
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
	if record.ParseWarnings, err = decodeStringArray(bundle.ParseWarningsJSON); err != nil {
		return nil, fmt.Errorf("decode env warnings: %w", err)
	}
	return record, nil
}

func (s *ProjectEnvService) loadHelperSnippets(projectID string) ([]ProjectEnvHelperSnippet, error) {
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
			return buildProjectEnvHelperSnippetsFromServiceInventory(runtimeMode, records, runtimeEnv), nil
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

func buildProjectEnvHelperSnippets(services []models.ProjectInternalService) []ProjectEnvHelperSnippet {
	items := make([]ProjectEnvHelperSnippet, 0, len(services))
	for _, item := range services {
		host, port := splitProjectEnvHostPort(item.LocalEndpoint)
		if host == "" || port == "" {
			continue
		}
		aliasKey := sanitizeProjectEnvAlias(item.Alias)
		entry := ProjectEnvHelperSnippet{
			ServiceKind: item.Kind,
			Alias:       item.Alias,
			Env: map[string]string{
				aliasKey + "_HOST": host,
				aliasKey + "_PORT": port,
				aliasKey + "_URL":  projectEnvDependencyURL(item.Protocol, host, port),
			},
		}
		if strings.EqualFold(item.Kind, "postgres") {
			entry.Env["DB_HOST"] = host
			entry.Env["DB_PORT"] = port
			entry.Env["DB_NAME"] = "app"
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

func buildProjectEnvHelperSnippetsFromServiceInventory(runtimeMode string, services []ProjectServiceRecord, projectEnv map[string]string) []ProjectEnvHelperSnippet {
	items := make([]ProjectEnvHelperSnippet, 0)
	for _, item := range services {
		if item.SourceType != serviceSourceTypeInternal || !strings.EqualFold(strings.TrimSpace(item.Kind), "postgres") {
			continue
		}
		items = append(items, ProjectEnvHelperSnippet{
			ServiceKind: item.Kind,
			Alias:       item.Name,
			Env:         buildPostgresConnectionTemplateEnv(item, projectEnv, runtimeMode),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ServiceKind == items[j].ServiceKind {
			return items[i].Alias < items[j].Alias
		}
		return items[i].ServiceKind < items[j].ServiceKind
	})
	return items
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
