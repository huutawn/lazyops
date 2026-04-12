package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func makeTestLayout(root string) WorkspaceLayout {
	return WorkspaceLayout{
		Root:     filepath.Join(root, "workspace"),
		Sidecars: filepath.Join(root, "sidecars"),
		Services: filepath.Join(root, "services"),
	}
}

func writeTestManifest(layout WorkspaceLayout) error {
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		return err
	}
	manifest := WorkspaceManifest{
		PreparedAt: time.Now().UTC(),
		Project:    ProjectMetadata{ProjectID: "prj_1"},
		Binding:    contracts.DeploymentBindingPayload{BindingID: "bind_1"},
		Revision:   contracts.DesiredRevisionPayload{RevisionID: "rev_1"},
	}
	return writeJSON(filepath.Join(layout.Root, "workspace.json"), manifest)
}

func writeTestService(layout WorkspaceLayout, name string) error {
	dir := filepath.Join(layout.Services, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, "runtime.json"), map[string]any{"name": name})
}

func TestSidecarManagerRenderSidecarReturnsErrorWhenManifestMissing(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	layout := makeTestLayout(root)
	runtimeCtx := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_1"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_1"},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_1"},
	}

	_, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err == nil {
		t.Fatal("expected error when workspace manifest is missing")
	}
	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.Code != "sidecar_workspace_missing" {
		t.Fatalf("expected sidecar_workspace_missing error, got %s", opErr.Code)
	}
}

func TestSidecarManagerRenderSidecarCreatesFiles(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	layout := makeTestLayout(root)
	if err := writeTestManifest(layout); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestService(layout, "web"); err != nil {
		t.Fatalf("write service: %v", err)
	}

	runtimeCtx := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_1"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_1", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_1"},
		Services: []ServiceRuntimeContext{
			{Name: "web", Path: "apps/web"},
		},
	}

	result, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}

	if result.Version == "" {
		t.Fatal("expected non-empty version")
	}
	if result.PlanPath == "" {
		t.Fatal("expected non-empty plan path")
	}
	if result.ActivationPath == "" {
		t.Fatal("expected non-empty activation path")
	}
	if _, err := os.Stat(result.PlanPath); err != nil {
		t.Fatalf("expected plan file to exist: %v", err)
	}
	if _, err := os.Stat(result.ActivationPath); err != nil {
		t.Fatalf("expected activation file to exist: %v", err)
	}
}

func TestSidecarManagerRenderSidecarWithDependencies(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	layout := makeTestLayout(root)
	if err := writeTestManifest(layout); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestService(layout, "web"); err != nil {
		t.Fatalf("write web: %v", err)
	}
	if err := writeTestService(layout, "api"); err != nil {
		t.Fatalf("write api: %v", err)
	}

	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{ProjectID: "prj_1"},
		Binding: contracts.DeploymentBindingPayload{BindingID: "bind_1", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{
			RevisionID: "rev_1",
			CompatibilityPolicy: contracts.CompatibilityPolicy{
				EnvInjection: true,
			},
		},
		Services: []ServiceRuntimeContext{
			{
				Name: "web",
				Path: "apps/web",
				Dependencies: []contracts.DependencyBindingPayload{
					{
						Alias:         "api",
						TargetService: "api",
						Protocol:      "http",
						LocalEndpoint: "http://localhost:3000",
					},
				},
			},
			{Name: "api", Path: "apps/api"},
		},
	}

	result, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}

	if len(result.Services) != 1 || result.Services[0] != "web" {
		t.Fatalf("expected web service enabled, got %v", result.Services)
	}
	if len(result.Plan.EnabledServices) != 1 {
		t.Fatalf("expected 1 enabled service in plan, got %d", len(result.Plan.EnabledServices))
	}
	if len(result.Plan.Bindings) != 1 {
		t.Fatalf("expected 1 binding in plan, got %d", len(result.Plan.Bindings))
	}
}

func TestSidecarManagerRenderSidecarMissingTargetService(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	layout := makeTestLayout(root)
	if err := writeTestManifest(layout); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestService(layout, "web"); err != nil {
		t.Fatalf("write web: %v", err)
	}

	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{ProjectID: "prj_1"},
		Binding: contracts.DeploymentBindingPayload{BindingID: "bind_1", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{
			RevisionID: "rev_1",
			CompatibilityPolicy: contracts.CompatibilityPolicy{
				EnvInjection: true,
			},
		},
		Services: []ServiceRuntimeContext{
			{
				Name: "web",
				Path: "apps/web",
				Dependencies: []contracts.DependencyBindingPayload{
					{
						Alias:         "db",
						TargetService: "database",
						Protocol:      "tcp",
						LocalEndpoint: "localhost:5432",
					},
				},
			},
		},
	}

	_, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err == nil {
		t.Fatal("expected error for missing target service")
	}
	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.Code != "sidecar_missing_target_service" {
		t.Fatalf("expected sidecar_missing_target_service, got %s", opErr.Code)
	}
}

func TestSidecarManagerRenderSidecarNoCompatibleMode(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	layout := makeTestLayout(root)
	if err := writeTestManifest(layout); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestService(layout, "web"); err != nil {
		t.Fatalf("write web: %v", err)
	}
	if err := writeTestService(layout, "api"); err != nil {
		t.Fatalf("write api: %v", err)
	}

	runtimeCtx := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_1"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_1", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_1"},
		Services: []ServiceRuntimeContext{
			{
				Name: "web",
				Path: "apps/web",
				Dependencies: []contracts.DependencyBindingPayload{
					{
						Alias:         "api",
						TargetService: "api",
						Protocol:      "http",
						LocalEndpoint: "http://localhost:3000",
					},
				},
			},
			{Name: "api", Path: "apps/api"},
		},
	}

	_, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err == nil {
		t.Fatal("expected error when no compatible mode is set")
	}
	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.Code != "sidecar_no_compatible_mode" {
		t.Fatalf("expected sidecar_no_compatible_mode, got %s", opErr.Code)
	}
}

func TestSidecarManagerRenderSidecarWithCustomHooks(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	createCalled := false
	mgr.createHook = func(ctx context.Context, _ RuntimeContext, plan SidecarPlan, paths sidecarRenderPaths) (SidecarActivation, SidecarHookResult, error) {
		createCalled = true
		return SidecarActivation{Version: plan.Version, AppliedAt: time.Now().UTC()}, SidecarHookResult{Name: "create", Status: "custom_created"}, nil
	}

	layout := makeTestLayout(root)
	if err := writeTestManifest(layout); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestService(layout, "web"); err != nil {
		t.Fatalf("write web: %v", err)
	}

	runtimeCtx := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_1"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_1", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_1"},
		Services: []ServiceRuntimeContext{
			{Name: "web", Path: "apps/web"},
		},
	}

	_, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}

	if !createCalled {
		t.Fatal("expected custom create hook to be called")
	}
}

func TestSidecarManagerRenderSidecarInjectsSidecarBlock(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	layout := makeTestLayout(root)
	if err := writeTestManifest(layout); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestService(layout, "web"); err != nil {
		t.Fatalf("write web: %v", err)
	}
	if err := writeTestService(layout, "api"); err != nil {
		t.Fatalf("write api: %v", err)
	}

	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{ProjectID: "prj_1"},
		Binding: contracts.DeploymentBindingPayload{BindingID: "bind_1", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{
			RevisionID: "rev_1",
			CompatibilityPolicy: contracts.CompatibilityPolicy{
				EnvInjection: true,
			},
		},
		Services: []ServiceRuntimeContext{
			{
				Name: "web",
				Path: "apps/web",
				Dependencies: []contracts.DependencyBindingPayload{
					{
						Alias:         "api",
						TargetService: "api",
						Protocol:      "http",
						LocalEndpoint: "http://localhost:3000",
					},
				},
			},
			{Name: "api", Path: "apps/api"},
		},
	}

	_, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}

	runtimePath := filepath.Join(layout.Services, "web", "runtime.json")
	payload, err := os.ReadFile(runtimePath)
	if err != nil {
		t.Fatalf("read runtime: %v", err)
	}

	var current map[string]any
	if err := json.Unmarshal(payload, &current); err != nil {
		t.Fatalf("decode runtime: %v", err)
	}

	sidecarBlock, ok := current["sidecar"].(map[string]any)
	if !ok {
		t.Fatal("expected sidecar block in runtime.json")
	}
	if enabled, _ := sidecarBlock["enabled"].(bool); !enabled {
		t.Fatal("expected sidecar enabled")
	}
}

func TestSidecarManagerRenderSidecarDisablesSidecarForServiceWithoutDeps(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(slog.New(slog.NewTextHandler(io.Discard, nil)), root)

	layout := makeTestLayout(root)
	if err := writeTestManifest(layout); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestService(layout, "web"); err != nil {
		t.Fatalf("write web: %v", err)
	}

	runtimeCtx := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_1"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_1", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_1"},
		Services: []ServiceRuntimeContext{
			{Name: "web", Path: "apps/web"},
		},
	}

	_, err := mgr.RenderSidecars(context.Background(), runtimeCtx, layout)
	if err != nil {
		t.Fatalf("render sidecars: %v", err)
	}

	runtimePath := filepath.Join(layout.Services, "web", "runtime.json")
	payload, err := os.ReadFile(runtimePath)
	if err != nil {
		t.Fatalf("read runtime: %v", err)
	}

	var current map[string]any
	if err := json.Unmarshal(payload, &current); err != nil {
		t.Fatalf("decode runtime: %v", err)
	}

	sidecarBlock, ok := current["sidecar"].(map[string]any)
	if !ok {
		t.Fatal("expected sidecar block in runtime.json")
	}
	if enabled, _ := sidecarBlock["enabled"].(bool); enabled {
		t.Fatal("expected sidecar disabled for service without dependencies")
	}
}

func TestSelectSidecarMode(t *testing.T) {
	tests := []struct {
		name     string
		policy   contracts.CompatibilityPolicy
		expected string
	}{
		{"env_injection", contracts.CompatibilityPolicy{EnvInjection: true}, "env_injection"},
		{"managed_credentials", contracts.CompatibilityPolicy{ManagedCredentials: true}, "managed_credentials"},
		{"localhost_rescue", contracts.CompatibilityPolicy{LocalhostRescue: true}, "localhost_rescue"},
		{"none", contracts.CompatibilityPolicy{}, ""},
		{"precedence_env_over_managed", contracts.CompatibilityPolicy{EnvInjection: true, ManagedCredentials: true}, "env_injection"},
		{"precedence_env_over_rescue", contracts.CompatibilityPolicy{EnvInjection: true, LocalhostRescue: true}, "env_injection"},
		{"precedence_managed_over_rescue", contracts.CompatibilityPolicy{ManagedCredentials: true, LocalhostRescue: true}, "managed_credentials"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectSidecarMode(tt.policy)
			if got != tt.expected {
				t.Errorf("selectSidecarMode(%v) = %q, want %q", tt.policy, got, tt.expected)
			}
		})
	}
}

func TestSidecarPrecedence(t *testing.T) {
	prec := sidecarPrecedence()
	expected := []string{"env_injection", "managed_credentials", "localhost_rescue"}
	if len(prec) != len(expected) {
		t.Fatalf("expected %d precedence items, got %d", len(expected), len(prec))
	}
	for i, p := range expected {
		if prec[i] != p {
			t.Fatalf("expected precedence[%d] = %q, got %q", i, p, prec[i])
		}
	}
}

func TestValidateSelectedSidecarMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		policy    contracts.CompatibilityPolicy
		expectErr bool
		errCode   string
	}{
		{"valid_env_injection", "env_injection", contracts.CompatibilityPolicy{EnvInjection: true}, false, ""},
		{"valid_managed_credentials", "managed_credentials", contracts.CompatibilityPolicy{ManagedCredentials: true}, false, ""},
		{"valid_localhost_rescue", "localhost_rescue", contracts.CompatibilityPolicy{LocalhostRescue: true}, false, ""},
		{"valid_empty", "", contracts.CompatibilityPolicy{}, false, ""},
		{"env_injection_violation", "managed_credentials", contracts.CompatibilityPolicy{EnvInjection: true}, true, "sidecar_precedence_violation"},
		{"managed_violation", "localhost_rescue", contracts.CompatibilityPolicy{ManagedCredentials: true}, true, "sidecar_precedence_violation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSelectedSidecarMode(tt.mode, tt.policy)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Fatalf("expected OperationError, got %T", err)
				}
				if opErr.Code != tt.errCode {
					t.Fatalf("expected error code %s, got %s", tt.errCode, opErr.Code)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestSidecarVersion(t *testing.T) {
	ctx1 := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_1"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_1"},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_1"},
	}
	ctx2 := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_1"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_1"},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_2"},
	}

	v1 := sidecarVersion(ctx1)
	v2 := sidecarVersion(ctx2)

	if v1 == v2 {
		t.Fatal("expected different versions for different revisions")
	}
	if !strings.HasPrefix(v1, "sc_") {
		t.Fatalf("expected version to start with sc_, got %s", v1)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"203.0.113.10", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := isLoopbackHost(tt.host)
			if got != tt.expected {
				t.Errorf("isLoopbackHost(%q) = %v, want %v", tt.host, got, tt.expected)
			}
		})
	}
}

func TestLooksSensitiveEnvKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"LAZYOPS_DEP_API_ENDPOINT", false},
		{"LAZYOPS_DEP_API_TOKEN", true},
		{"LAZYOPS_DEP_API_SECRET", true},
		{"LAZYOPS_DEP_API_PASSWORD", true},
		{"LAZYOPS_DEP_API_PRIVATE_KEY", true},
		{"LAZYOPS_DEP_API_CREDENTIAL", true},
		{"LAZYOPS_DEP_API_PROTOCOL", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := looksSensitiveEnvKey(tt.key)
			if got != tt.expected {
				t.Errorf("looksSensitiveEnvKey(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func TestSanitizeEnvKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"api", "API"},
		{"my-service", "MY_SERVICE"},
		{"db_1", "DB_1"},
		{"  spaces  ", "SPACES"},
		{"special!@#chars", "SPECIAL___CHARS"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeEnvKey(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeEnvKey(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMaskManagedCredentialValue(t *testing.T) {
	tests := []struct {
		value    string
		expected string
	}{
		{"abc", "[MASKED]"},
		{"abcdef", "[MASKED]"},
		{"abcdefg", "abcdef***defg"},
		{"my-secret-handle-1234", "my-sec***1234"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := maskManagedCredentialValue(tt.value)
			if got != tt.expected {
				t.Errorf("maskManagedCredentialValue(%q) = %q, want %q", tt.value, got, tt.expected)
			}
		})
	}
}

func TestParseLocalhostEndpointHTTP(t *testing.T) {
	result, err := parseLocalhostEndpoint("http", "http://localhost:3000")
	if err != nil {
		t.Fatalf("parse endpoint: %v", err)
	}
	if result.Host != "localhost" {
		t.Fatalf("expected host localhost, got %s", result.Host)
	}
	if result.Port != 3000 {
		t.Fatalf("expected port 3000, got %d", result.Port)
	}
	if result.Scheme != "http" {
		t.Fatalf("expected scheme http, got %s", result.Scheme)
	}
}

func TestParseLocalhostEndpointTCP(t *testing.T) {
	result, err := parseLocalhostEndpoint("tcp", "127.0.0.1:5432")
	if err != nil {
		t.Fatalf("parse endpoint: %v", err)
	}
	if result.Host != "127.0.0.1" {
		t.Fatalf("expected host 127.0.0.1, got %s", result.Host)
	}
	if result.Port != 5432 {
		t.Fatalf("expected port 5432, got %d", result.Port)
	}
	if result.Scheme != "tcp" {
		t.Fatalf("expected scheme tcp, got %s", result.Scheme)
	}
}

func TestParseLocalhostEndpointMissing(t *testing.T) {
	_, err := parseLocalhostEndpoint("http", "")
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.Code != "localhost_rescue_missing_endpoint" {
		t.Fatalf("expected localhost_rescue_missing_endpoint, got %s", opErr.Code)
	}
}

func TestParseLocalhostEndpointNonLocalhost(t *testing.T) {
	_, err := parseLocalhostEndpoint("http", "http://192.168.1.1:3000")
	if err == nil {
		t.Fatal("expected error for non-localhost endpoint")
	}
	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.Code != "localhost_rescue_non_local_endpoint" {
		t.Fatalf("expected localhost_rescue_non_local_endpoint, got %s", opErr.Code)
	}
}

func TestSidecarManagerDefaultRemoveCleansUpStaleServices(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	liveConfigRoot := filepath.Join(root, "live", "services")
	if err := os.MkdirAll(filepath.Join(liveConfigRoot, "old-service"), 0o755); err != nil {
		t.Fatalf("mkdir old-service: %v", err)
	}
	if err := writeJSON(filepath.Join(liveConfigRoot, "old-service", "config.json"), map[string]any{}); err != nil {
		t.Fatalf("write old config: %v", err)
	}

	paths := sidecarRenderPaths{
		liveConfigRoot: liveConfigRoot,
		removePath:     filepath.Join(root, "live", "remove.json"),
	}

	plan := SidecarPlan{
		EnabledServices: []string{"new-service"},
	}
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{ProjectID: "prj_1"},
		Binding: contracts.DeploymentBindingPayload{BindingID: "bind_1"},
	}

	result, err := mgr.defaultRemove(context.Background(), runtimeCtx, plan, paths)
	if err != nil {
		t.Fatalf("default remove: %v", err)
	}
	if result.Status != "removed" {
		t.Fatalf("expected removed status, got %s", result.Status)
	}
	if !strings.Contains(result.Message, "1 stale") {
		t.Fatalf("expected message about 1 stale service, got %s", result.Message)
	}

	if _, err := os.Stat(filepath.Join(liveConfigRoot, "old-service")); !os.IsNotExist(err) {
		t.Fatal("expected old-service directory to be removed")
	}
}

func TestSidecarManagerDefaultRestartSkipsWhenSameVersion(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	paths := sidecarRenderPaths{
		restartPath: filepath.Join(root, "live", "restart.json"),
	}

	if err := os.MkdirAll(filepath.Dir(paths.restartPath), 0o755); err != nil {
		t.Fatalf("mkdir restart dir: %v", err)
	}

	plan := SidecarPlan{Version: "sc_v1"}
	previous := &SidecarActivation{Version: "sc_v1", AppliedAt: time.Now().UTC()}
	activation := SidecarActivation{Version: "sc_v1", AppliedAt: time.Now().UTC()}
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{ProjectID: "prj_1"},
		Binding: contracts.DeploymentBindingPayload{BindingID: "bind_1"},
	}

	result, err := mgr.defaultRestart(context.Background(), runtimeCtx, plan, paths, previous, activation)
	if err != nil {
		t.Fatalf("default restart: %v", err)
	}
	if result.Status != "skipped" {
		t.Fatalf("expected skipped status, got %s", result.Status)
	}
}

func TestSidecarManagerDefaultRestartWhenVersionChanges(t *testing.T) {
	root := t.TempDir()
	mgr := NewSidecarManager(nil, root)

	paths := sidecarRenderPaths{
		restartPath: filepath.Join(root, "live", "restart.json"),
	}

	if err := os.MkdirAll(filepath.Dir(paths.restartPath), 0o755); err != nil {
		t.Fatalf("mkdir restart dir: %v", err)
	}

	plan := SidecarPlan{Version: "sc_v2"}
	previous := &SidecarActivation{Version: "sc_v1", AppliedAt: time.Now().UTC()}
	activation := SidecarActivation{Version: "sc_v2", AppliedAt: time.Now().UTC()}
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{ProjectID: "prj_1"},
		Binding: contracts.DeploymentBindingPayload{BindingID: "bind_1"},
	}

	result, err := mgr.defaultRestart(context.Background(), runtimeCtx, plan, paths, previous, activation)
	if err != nil {
		t.Fatalf("default restart: %v", err)
	}
	if result.Status != "restarted" {
		t.Fatalf("expected restarted status, got %s", result.Status)
	}
}

func TestBuildManagedCredentialAuditLog(t *testing.T) {
	plan := SidecarPlan{
		Version: "sc_v1",
		Services: []SidecarServiceConfig{
			{
				ServiceName: "web",
				ManagedCredentialContracts: []SidecarManagedCredentialContract{
					{
						Alias:         "db",
						TargetService: "db",
						Protocol:      "tcp",
						CredentialRef: "managed://prj_1/web/db",
						Values:        map[string]string{"HANDLE": "mcred_abc123"},
						MaskedValues:  map[string]string{"HANDLE": "mcre***c123"},
						SecretSafe:    true,
					},
				},
			},
		},
	}

	audit := buildManagedCredentialAuditLog(plan, time.Now().UTC())

	if audit.PlaintextPersisted {
		t.Fatal("expected plaintext persisted to be false")
	}
	if len(audit.Services) != 1 {
		t.Fatalf("expected 1 service in audit, got %d", len(audit.Services))
	}
	if len(audit.Services["web"]) != 1 {
		t.Fatalf("expected 1 audit record for web, got %d", len(audit.Services["web"]))
	}
}

func TestLooksManagedCredentialPlaintext(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		expected bool
	}{
		{"LAZYOPS_MANAGED_DB_REF", "managed://prj_1/web/db", false},
		{"LAZYOPS_MANAGED_DB_REF", "not-managed://prj_1/web/db", true},
		{"LAZYOPS_MANAGED_DB_HANDLE", "mcred_abc123", false},
		{"LAZYOPS_MANAGED_DB_HANDLE", "not-mcred_abc123", true},
		{"LAZYOPS_MANAGED_DB_PROTOCOL", "http", false},
		{"LAZYOPS_MANAGED_DB_PROTOCOL", "ftp", true},
		{"LAZYOPS_MANAGED_DB_TARGET_SERVICE", "db", false},
		{"LAZYOPS_MANAGED_DB_TARGET_SERVICE", "", true},
		{"LAZYOPS_DEP_DB_TOKEN", "secret-value", true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := looksManagedCredentialPlaintext(tt.key, tt.value)
			if got != tt.expected {
				t.Errorf("looksManagedCredentialPlaintext(%q, %q) = %v, want %v", tt.key, tt.value, got, tt.expected)
			}
		})
	}
}
