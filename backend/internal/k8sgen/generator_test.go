package k8sgen

import (
	"strings"
	"testing"
	"time"
)

func TestGeneratorRendersNamespaceServiceIngressSecretAndPVC(t *testing.T) {
	gen := NewGenerator()
	gen.now = func() time.Time {
		return time.Date(2026, 4, 16, 8, 0, 0, 0, time.UTC)
	}

	bundle, err := gen.Generate(Input{
		Namespace:  "lazyops-prj-123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Services: []ServiceSpec{
			{
				Name:        "api",
				Kind:        "app",
				Namespace:   "lazyops-prj-123",
				Public:      true,
				ImageRef:    "ghcr.io/lazyops/api:rev_123",
				TargetPort:  8080,
				ServicePort: 80,
				Replicas:    2,
				Healthcheck: map[string]any{
					"path": "/health",
					"port": 8080,
				},
				EnvBundle: map[string]string{
					"DATABASE_URL": "postgres://lazyops:secret@db-service:5432/lazyops",
				},
			},
			{
				Name:        "db-service",
				Kind:        "postgres",
				Namespace:   "lazyops-prj-123",
				ImageRef:    "postgres:16-alpine",
				TargetPort:  5432,
				ServicePort: 5432,
				PVCSpec: map[string]any{
					"size": "20Gi",
				},
			},
		},
		PublicDomains: []PublicDomain{
			{
				ServiceName:  "api",
				PrimaryHost:  "api.lazyops-prj-123.203-0-113-10.sslip.io",
				FallbackHost: "api.lazyops-prj-123.203.0.113.10.nip.io",
			},
		},
	})
	if err != nil {
		t.Fatalf("generate manifest bundle: %v", err)
	}

	if bundle.Namespace != "lazyops-prj-123" {
		t.Fatalf("expected namespace lazyops-prj-123, got %q", bundle.Namespace)
	}
	if len(bundle.Documents) != 9 {
		t.Fatalf("expected 9 manifest documents, got %d", len(bundle.Documents))
	}

	combined := bundle.CombinedYAML
	for _, expected := range []string{
		"kind: Namespace",
		"name: lazyops-prj-123",
		"kind: Secret",
		"DATABASE_URL: \"postgres://lazyops:secret@db-service:5432/lazyops\"",
		"POSTGRES_DB: \"app\"",
		"POSTGRES_USER: \"postgres\"",
		"POSTGRES_PASSWORD: \"postgres\"",
		"kind: Service",
		"targetPort: 8080",
		"kind: Deployment",
		"image: ghcr.io/lazyops/api:rev_123",
		"startupProbe:",
		"failureThreshold: 18",
		"kind: Ingress",
		"host: api.lazyops-prj-123.203.0.113.10.nip.io",
		"kind: PersistentVolumeClaim",
		"storage: 20Gi",
		"mountPath: /var/lib/postgresql/data",
	} {
		if !strings.Contains(combined, expected) {
			t.Fatalf("expected manifest bundle to contain %q\nbundle:\n%s", expected, combined)
		}
	}
}

func TestGeneratorRejectsGenericServiceWithoutResolvedPort(t *testing.T) {
	gen := NewGenerator()

	_, err := gen.Generate(Input{
		Namespace: "lazyops-prj-123",
		Services: []ServiceSpec{
			{
				Name:     "api",
				Kind:     "api",
				Public:   true,
				ImageRef: "ghcr.io/lazyops/api:rev_123",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "requires an explicit resolved target_port/service_port") {
		t.Fatalf("expected unresolved port error, got %v", err)
	}
}

func TestGeneratorRejectsGenericServiceWithoutResolvedImage(t *testing.T) {
	gen := NewGenerator()

	_, err := gen.Generate(Input{
		Namespace: "lazyops-prj-123",
		Services: []ServiceSpec{
			{
				Name:        "api",
				Kind:        "api",
				Public:      true,
				TargetPort:  8080,
				ServicePort: 8080,
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "requires an explicit resolved image_ref") {
		t.Fatalf("expected unresolved image error, got %v", err)
	}
}

func TestGeneratorOmitsProbesForGenericServiceWithoutHealthcheck(t *testing.T) {
	gen := NewGenerator()

	bundle, err := gen.Generate(Input{
		Namespace: "lazyops-prj-123",
		Services: []ServiceSpec{
			{
				Name:        "api",
				Kind:        "api",
				Public:      true,
				ImageRef:    "ghcr.io/lazyops/api:rev_123",
				TargetPort:  8080,
				ServicePort: 8080,
			},
		},
		PublicDomains: []PublicDomain{{
			ServiceName:  "api",
			FallbackHost: "api.lazyops-prj-123.203.0.113.10.nip.io",
		}},
	})
	if err != nil {
		t.Fatalf("generate manifest bundle: %v", err)
	}
	if strings.Contains(bundle.CombinedYAML, "startupProbe:") {
		t.Fatalf("expected generic service without explicit healthcheck to omit probes, got:\n%s", bundle.CombinedYAML)
	}
}
