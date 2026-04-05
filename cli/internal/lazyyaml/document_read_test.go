package lazyyaml

import (
	"os"
	"path/filepath"
	"testing"

	"lazyops-cli/internal/initplan"
)

func TestReadDocumentReadsFullDeployContract(t *testing.T) {
	repoRoot := t.TempDir()
	payload := "" +
		"project_slug: acme-shop\n" +
		"runtime_mode: standalone\n\n" +
		"deployment_binding:\n" +
		"  target_ref: prod-solo-1\n\n" +
		"services:\n" +
		"  - name: api\n" +
		"    path: apps/api\n" +
		"    start_hint: go run ./cmd/server\n" +
		"    healthcheck:\n" +
		"      path: /healthz\n" +
		"      port: 8080\n\n" +
		"compatibility_policy:\n" +
		"  env_injection: true\n" +
		"  managed_credentials: true\n" +
		"  localhost_rescue: true\n\n" +
		"magic_domain_policy:\n" +
		"  enabled: true\n" +
		"  provider: sslip.io\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "lazyops.yaml"), []byte(payload), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	document, err := ReadDocument(repoRoot)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if document.ProjectSlug != "acme-shop" {
		t.Fatalf("expected project slug acme-shop, got %q", document.ProjectSlug)
	}
	if document.RuntimeMode != initplan.RuntimeModeStandalone {
		t.Fatalf("expected standalone runtime mode, got %q", document.RuntimeMode)
	}
	if len(document.Services) != 1 {
		t.Fatalf("expected one service, got %+v", document.Services)
	}
	if document.Services[0].Healthcheck.Port != 8080 {
		t.Fatalf("expected parsed healthcheck port 8080, got %+v", document.Services[0].Healthcheck)
	}
	if !document.CompatibilityPolicy.EnvInjection || !document.CompatibilityPolicy.ManagedCredentials || !document.CompatibilityPolicy.LocalhostRescue {
		t.Fatalf("expected compatibility policy to be parsed, got %+v", document.CompatibilityPolicy)
	}
}

func TestParseDocumentRejectsForbiddenRawFields(t *testing.T) {
	payload := []byte("" +
		"project_slug: acme-shop\n" +
		"runtime_mode: standalone\n\n" +
		"deployment_binding:\n" +
		"  target_ref: prod-solo-1\n\n" +
		"services:\n" +
		"  - name: api\n" +
		"    path: apps/api\n\n" +
		"compatibility_policy:\n" +
		"  env_injection: true\n" +
		"  managed_credentials: true\n" +
		"  localhost_rescue: true\n\n" +
		"token: secret://demo\n")

	if _, err := ParseDocument(payload); err == nil {
		t.Fatal("expected forbidden raw field validation error, got nil")
	}
}
