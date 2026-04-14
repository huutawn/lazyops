package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestBuildContainerEnvVarsMergesHelperUserAndReservedKeys(t *testing.T) {
	vars := buildContainerEnvVars(40620, []contracts.DependencyBindingPayload{{
		Alias:         "postgres",
		Protocol:      "tcp",
		LocalEndpoint: "localhost:5432",
	}}, map[string]string{
		"APP_ENV":      "prod",
		"DB_HOST":      "db.example.internal",
		"PORT":         "9999",
		"LAZYOPS_FAKE": "blocked",
	})

	got := make(map[string]string, len(vars))
	for _, item := range vars {
		parts := strings.SplitN(item, "=", 2)
		got[parts[0]] = parts[1]
	}

	if got["APP_ENV"] != "prod" {
		t.Fatalf("expected APP_ENV=prod, got %#v", got)
	}
	if got["DB_HOST"] != "db.example.internal" {
		t.Fatalf("expected DB_HOST from user env, got %#v", got)
	}
	if got["DB_PORT"] != "5432" {
		t.Fatalf("expected DB_PORT helper injection, got %#v", got)
	}
	if got["PORT"] != "40620" {
		t.Fatalf("expected reserved PORT to win, got %#v", got)
	}
	if _, ok := got["LAZYOPS_FAKE"]; ok {
		t.Fatalf("expected reserved LAZYOPS_* key to be blocked, got %#v", got)
	}
}

func TestHydrateRuntimeContextFromWorkspaceRestoresProjectEnv(t *testing.T) {
	root := t.TempDir()
	driver := NewFilesystemDriver(nil, root)
	runtimeCtx := RuntimeContext{
		Project:  ProjectMetadata{ProjectID: "prj_123"},
		Binding:  contracts.DeploymentBindingPayload{BindingID: "bind_123", RuntimeMode: contracts.RuntimeModeStandalone},
		Revision: contracts.DesiredRevisionPayload{RevisionID: "rev_123", RuntimeMode: contracts.RuntimeModeStandalone},
	}
	layout := workspaceLayout(root, runtimeCtx)
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := writeJSON(filepath.Join(layout.Root, "workspace.json"), WorkspaceManifest{
		PreparedAt: time.Now().UTC(),
		Project:    runtimeCtx.Project,
		ProjectEnv: map[string]string{"APP_ENV": "prod"},
		Binding:    runtimeCtx.Binding,
		Revision:   runtimeCtx.Revision,
		Services:   []ServiceRuntimeContext{{Name: "app", HealthCheck: contracts.HealthCheckPayload{Port: 8080, Protocol: "http"}}},
	}); err != nil {
		t.Fatalf("write workspace manifest: %v", err)
	}

	hydrated := driver.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)
	if hydrated.ProjectEnv["APP_ENV"] != "prod" {
		t.Fatalf("expected project env to hydrate from workspace, got %#v", hydrated.ProjectEnv)
	}
}
