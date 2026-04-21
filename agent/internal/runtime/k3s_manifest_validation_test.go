package runtime

import (
	"strings"
	"testing"

	"lazyops-agent/internal/contracts"
)

func TestValidateK3sManifestPreflightRejectsInvalidServiceName(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{Namespace: "lazyops-test"},
		Revision: contracts.DesiredRevisionPayload{
			ServiceSpecs: []contracts.K3sServiceSpecPayload{
				{Name: "API_BAD"},
			},
		},
	}

	_, err := validateK3sManifestPreflight(runtimeCtx)
	if err == nil || !strings.Contains(err.Error(), "must not contain underscores") {
		t.Fatalf("expected invalid service name error, got %v", err)
	}
}

func TestValidateK3sManifestPreflightRejectsMissingSecretAndPVCReferences(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{Namespace: "lazyops-test"},
		Revision: contracts.DesiredRevisionPayload{
			ServiceSpecs: []contracts.K3sServiceSpecPayload{
				{
					Name:      "db",
					Kind:      "postgres",
					EnvBundle: map[string]string{"POSTGRES_PASSWORD": "secret"},
					PVCSpec:   map[string]any{"size": "5Gi"},
				},
			},
			ManifestBundle: contracts.ManifestBundlePayload{
				Documents: []contracts.ManifestDocumentPayload{
					{Kind: "Namespace", Name: "lazyops-test"},
					{Kind: "Service", Name: "db", Content: "kind: Service\nmetadata:\n  name: db\n"},
					{Kind: "Deployment", Name: "db", Content: "kind: Deployment\nmetadata:\n  name: db\n"},
				},
			},
		},
	}

	_, err := validateK3sManifestPreflight(runtimeCtx)
	if err == nil || !strings.Contains(err.Error(), "missing Secret document") {
		t.Fatalf("expected missing secret validation error, got %v", err)
	}
}

func TestValidateK3sManifestPreflightWarnsOnDependencyOrder(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{Namespace: "lazyops-test"},
		Revision: contracts.DesiredRevisionPayload{
			ServiceSpecs: []contracts.K3sServiceSpecPayload{
				{Name: "api"},
				{Name: "db", Kind: "postgres"},
			},
			InternalBindings: []contracts.InternalBindingPayload{
				{
					ServiceName:   "api",
					TargetService: "db",
				},
			},
			ManifestBundle: contracts.ManifestBundlePayload{
				Documents: []contracts.ManifestDocumentPayload{
					{Kind: "Namespace", Name: "lazyops-test"},
					{Kind: "Service", Name: "api", Content: "kind: Service\nmetadata:\n  name: api\n"},
					{Kind: "Deployment", Name: "api", Content: "kind: Deployment\nmetadata:\n  name: api\n"},
					{Kind: "Service", Name: "db", Content: "kind: Service\nmetadata:\n  name: db\n"},
					{Kind: "Deployment", Name: "db", Content: "kind: Deployment\nmetadata:\n  name: db\nclaimName: db-data\n"},
					{Kind: "PersistentVolumeClaim", Name: "db-data", Content: "kind: PersistentVolumeClaim\nmetadata:\n  name: db-data\n"},
				},
			},
		},
	}

	result, err := validateK3sManifestPreflight(runtimeCtx)
	if err != nil {
		t.Fatalf("expected dependency-order warning, got error %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected dependency-order warning, got none")
	}
	if !strings.Contains(strings.Join(result.Warnings, " "), "dependency order warning") {
		t.Fatalf("expected dependency order warning, got %#v", result.Warnings)
	}
}

func TestValidateK3sManifestPreflightRejectsMissingPVCClaimReference(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{Namespace: "lazyops-test"},
		Revision: contracts.DesiredRevisionPayload{
			ServiceSpecs: []contracts.K3sServiceSpecPayload{
				{
					Name:    "db",
					Kind:    "postgres",
					PVCSpec: map[string]any{"size": "5Gi"},
				},
			},
			ManifestBundle: contracts.ManifestBundlePayload{
				Documents: []contracts.ManifestDocumentPayload{
					{Kind: "Namespace", Name: "lazyops-test"},
					{Kind: "Service", Name: "db", Content: "kind: Service\nmetadata:\n  name: db\n"},
					{Kind: "PersistentVolumeClaim", Name: "db-data", Content: "kind: PersistentVolumeClaim\nmetadata:\n  name: db-data\n"},
					{Kind: "Deployment", Name: "db", Content: "kind: Deployment\nmetadata:\n  name: db\n"},
				},
			},
		},
	}

	_, err := validateK3sManifestPreflight(runtimeCtx)
	if err == nil || !strings.Contains(err.Error(), "does not reference expected pvc claim") {
		t.Fatalf("expected missing pvc claim validation error, got %v", err)
	}
}

func TestValidateK3sManifestPreflightRejectsGenericServiceWithoutResolvedPort(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{Namespace: "lazyops-test"},
		Revision: contracts.DesiredRevisionPayload{
			ServiceSpecs: []contracts.K3sServiceSpecPayload{
				{
					Name:           "api",
					Kind:           "api",
					RuntimeProfile: "service",
					Public:         true,
					ImageRef:       "ghcr.io/lazyops/api:abc123",
				},
			},
		},
	}

	_, err := validateK3sManifestPreflight(runtimeCtx)
	if err == nil || !strings.Contains(err.Error(), "requires a resolved target_port") {
		t.Fatalf("expected unresolved target port validation error, got %v", err)
	}
}

func TestValidateK3sManifestPreflightRejectsHealthcheckPortMismatch(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{Namespace: "lazyops-test"},
		Revision: contracts.DesiredRevisionPayload{
			ServiceSpecs: []contracts.K3sServiceSpecPayload{
				{
					Name:           "api",
					Kind:           "api",
					RuntimeProfile: "service",
					Public:         true,
					ImageRef:       "ghcr.io/lazyops/api:abc123",
					TargetPort:     3000,
					ServicePort:    3000,
					HealthCheck: contracts.HealthCheckPayload{
						Path: "/healthz",
						Port: 8080,
					},
				},
			},
		},
	}

	_, err := validateK3sManifestPreflight(runtimeCtx)
	if err == nil || !strings.Contains(err.Error(), "conflicts with resolved target_port 3000") {
		t.Fatalf("expected healthcheck mismatch validation error, got %v", err)
	}
}
