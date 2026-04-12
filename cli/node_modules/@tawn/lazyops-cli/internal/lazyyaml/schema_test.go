package lazyyaml

import (
	"strings"
	"testing"

	"lazyops-cli/internal/initplan"
)

func TestDocumentValidateAcceptsLockedSchema(t *testing.T) {
	doc := Document{
		ProjectSlug: "acme-shop",
		RuntimeMode: initplan.RuntimeModeDistributedMesh,
		DeploymentBinding: DeploymentBindingRef{
			TargetRef: "prod-ap",
		},
		Services: []Service{
			{
				Name:      "web",
				Path:      "apps/web",
				StartHint: "npm run start",
				Public:    true,
				Healthcheck: Healthcheck{
					Path: "/health",
					Port: 3000,
				},
			},
			{
				Name:      "api",
				Path:      "apps/api",
				StartHint: "go run ./cmd/server",
				Healthcheck: Healthcheck{
					Path: "/healthz",
					Port: 8080,
				},
			},
		},
		DependencyBindings: []DependencyBinding{
			{
				Service:       "api",
				Alias:         "postgres",
				TargetService: "app-db",
				Protocol:      "tcp",
				LocalEndpoint: "localhost:5432",
			},
			{
				Service:       "web",
				Alias:         "api",
				TargetService: "api",
				Protocol:      "http",
				LocalEndpoint: "127.0.0.1:8080",
			},
		},
		CompatibilityPolicy: CompatibilityPolicy{
			EnvInjection:       true,
			ManagedCredentials: true,
			LocalhostRescue:    true,
		},
		MagicDomainPolicy: MagicDomainPolicy{
			Enabled:  true,
			Provider: "sslip.io",
		},
		PreviewPolicy: PreviewPolicy{
			Enabled: true,
		},
		ScaleToZeroPolicy: ScaleToZeroPolicy{
			Enabled: false,
		},
	}

	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDocumentValidateRejectsForbiddenRawInfraData(t *testing.T) {
	doc := Document{
		ProjectSlug: "acme-shop",
		RuntimeMode: initplan.RuntimeModeStandalone,
		DeploymentBinding: DeploymentBindingRef{
			TargetRef: "203.0.113.10",
		},
		Services: []Service{
			{Name: "api", Path: "apps/api"},
		},
		CompatibilityPolicy: CompatibilityPolicy{
			EnvInjection: true,
		},
	}

	err := doc.Validate()
	if err == nil {
		t.Fatal("expected forbidden raw infra data error, got nil")
	}
	if !strings.Contains(err.Error(), "target_ref") && !strings.Contains(err.Error(), "logical") {
		t.Fatalf("expected logical target_ref error, got %v", err)
	}
}

func TestDocumentValidateRejectsRemoteDependencyEndpoint(t *testing.T) {
	doc := Document{
		ProjectSlug: "acme-shop",
		RuntimeMode: initplan.RuntimeModeDistributedMesh,
		DeploymentBinding: DeploymentBindingRef{
			TargetRef: "prod-ap",
		},
		Services: []Service{
			{Name: "api", Path: "apps/api"},
		},
		DependencyBindings: []DependencyBinding{
			{
				Service:       "api",
				Alias:         "postgres",
				TargetService: "app-db",
				Protocol:      "tcp",
				LocalEndpoint: "203.0.113.10:5432",
			},
		},
		CompatibilityPolicy: CompatibilityPolicy{
			ManagedCredentials: true,
		},
	}

	err := doc.Validate()
	if err == nil {
		t.Fatal("expected remote local_endpoint rejection, got nil")
	}
	if !strings.Contains(err.Error(), "must stay local") {
		t.Fatalf("expected local endpoint error, got %v", err)
	}
}

func TestDocumentValidateRejectsInvalidMagicDomainProvider(t *testing.T) {
	doc := Document{
		ProjectSlug: "acme-shop",
		RuntimeMode: initplan.RuntimeModeStandalone,
		DeploymentBinding: DeploymentBindingRef{
			TargetRef: "prod-solo-1",
		},
		Services: []Service{
			{Name: "api", Path: "."},
		},
		CompatibilityPolicy: CompatibilityPolicy{
			LocalhostRescue: true,
		},
		MagicDomainPolicy: MagicDomainPolicy{
			Enabled:  true,
			Provider: "example.com",
		},
	}

	err := doc.Validate()
	if err == nil {
		t.Fatal("expected invalid magic domain provider error, got nil")
	}
	if !strings.Contains(err.Error(), "sslip.io") || !strings.Contains(err.Error(), "nip.io") {
		t.Fatalf("expected magic domain provider guidance, got %v", err)
	}
}

func TestValidateSchemaTypeLockRejectsForbiddenFieldNames(t *testing.T) {
	type badSchema struct {
		Token string `json:"token,omitempty"`
	}

	err := ValidateSchemaTypeLock(badSchema{})
	if err == nil {
		t.Fatal("expected schema lock violation, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") || !strings.Contains(err.Error(), "token") {
		t.Fatalf("expected token schema lock violation, got %v", err)
	}
}

func TestForbiddenFieldNamesIncludesConcreteInfraAndSecrets(t *testing.T) {
	names := strings.Join(ForbiddenFieldNames(), ",")
	for _, required := range []string{"token", "kubeconfig", "target_id", "public_ip", "deploy_command"} {
		if !strings.Contains(names, required) {
			t.Fatalf("expected forbidden field list to include %q, got %q", required, names)
		}
	}
}
