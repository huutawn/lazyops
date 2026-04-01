package lazyyaml

import (
	"strings"
	"testing"

	"lazyops-cli/internal/initplan"
)

func TestGenerateStandaloneYAMLUsesSpecDefaults(t *testing.T) {
	plan := testPlan(initplan.RuntimeModeStandalone, "prod-solo-1", "standalone", "api", "apps/api")

	rendered, err := Generate(plan, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	output := string(rendered)
	for _, expected := range []string{
		"project_slug: acme-shop",
		"runtime_mode: standalone",
		"target_ref: prod-solo-1",
		"name: api",
		"path: apps/api",
		"env_injection: true",
		"managed_credentials: true",
		"localhost_rescue: true",
		"enabled: true\n  provider: sslip.io",
		"scale_to_zero_policy:\n  enabled: false",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected generated YAML to contain %q, got %q", expected, output)
		}
	}
}

func TestGenerateDistributedMeshYAMLIncludesDependencyBindings(t *testing.T) {
	plan := testPlan(initplan.RuntimeModeDistributedMesh, "prod-ap", "distributed-mesh", "web", "apps/web", "api", "apps/api")
	plan.DependencyBindings = []initplan.DependencyBindingDraft{
		{
			Service:       "web",
			Alias:         "api",
			TargetService: "api",
			Protocol:      "http",
			LocalEndpoint: "localhost:8080",
		},
	}

	rendered, err := Generate(plan, GenerateOptions{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	output := string(rendered)
	for _, expected := range []string{
		"runtime_mode: distributed-mesh",
		"target_ref: prod-ap",
		"dependency_bindings:",
		"service: web",
		"alias: api",
		"target_service: api",
		"protocol: http",
		"local_endpoint: 'localhost:8080'",
		"preview_policy:\n  enabled: true",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected generated YAML to contain %q, got %q", expected, output)
		}
	}
}

func TestGenerateDistributedK3sYAMLAllowsNipIOOverrideAndScaleToZeroOptIn(t *testing.T) {
	plan := testPlan(initplan.RuntimeModeDistributedK3s, "prod-k3s-ap", "distributed-k3s", "api", "services/api")

	rendered, err := Generate(plan, GenerateOptions{
		MagicDomainProvider: "nip.io",
		ScaleToZeroEnabled:  boolPtr(true),
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	output := string(rendered)
	for _, expected := range []string{
		"runtime_mode: distributed-k3s",
		"target_ref: prod-k3s-ap",
		"provider: nip.io",
		"scale_to_zero_policy:\n  enabled: true",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected generated YAML to contain %q, got %q", expected, output)
		}
	}
}

func TestBuildDocumentRejectsIncompleteInitPlan(t *testing.T) {
	_, err := BuildDocument(initplan.InitPlan{
		RepoRoot: "/tmp/repo",
		Layout:   "single-service",
	}, GenerateOptions{})
	if err == nil {
		t.Fatal("expected incomplete init plan error, got nil")
	}
	if !strings.Contains(err.Error(), "selected project") {
		t.Fatalf("expected selected project requirement, got %v", err)
	}
}

func testPlan(mode initplan.RuntimeMode, targetRef string, bindingMode string, serviceName string, servicePath string, extraService ...string) initplan.InitPlan {
	services := []initplan.ServiceCandidate{
		{
			Name:      serviceName,
			Path:      servicePath,
			StartHint: "go run ./cmd/server",
			Healthcheck: initplan.HealthcheckHint{
				Path: "/healthz",
				Port: 8080,
			},
		},
	}
	if len(extraService) == 2 {
		services = append(services, initplan.ServiceCandidate{
			Name: extraService[0],
			Path: extraService[1],
		})
	}

	return initplan.InitPlan{
		RepoRoot: "/tmp/repo",
		Layout:   "monorepo",
		SelectedProject: &initplan.ProjectSummary{
			ID:   "prj_demo",
			Name: "Acme Shop",
			Slug: "acme-shop",
		},
		RuntimeMode: mode,
		SelectedBinding: &initplan.BindingSummary{
			ID:          "bind_demo",
			Name:        "binding-demo",
			TargetRef:   targetRef,
			RuntimeMode: initplan.RuntimeMode(bindingMode),
			TargetKind:  "mesh",
			TargetID:    "target_demo",
		},
		Services:            services,
		CompatibilityPolicy: initplan.DefaultCompatibilityPolicyDraft(),
	}
}
