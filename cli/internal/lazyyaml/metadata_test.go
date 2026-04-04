package lazyyaml

import (
	"os"
	"path/filepath"
	"testing"

	"lazyops-cli/internal/initplan"
)

func TestReadLinkMetadataReadsDeployContractFields(t *testing.T) {
	repoRoot := t.TempDir()
	payload := "project_slug: acme-shop\nruntime_mode: distributed-k3s\n\ndeployment_binding:\n  target_ref: prod-k3s-ap\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "lazyops.yaml"), []byte(payload), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	metadata, err := ReadLinkMetadata(repoRoot)
	if err != nil {
		t.Fatalf("ReadLinkMetadata() error = %v", err)
	}

	if metadata.ProjectSlug != "acme-shop" {
		t.Fatalf("expected project slug acme-shop, got %q", metadata.ProjectSlug)
	}
	if metadata.RuntimeMode != initplan.RuntimeModeDistributedK3s {
		t.Fatalf("expected distributed-k3s runtime mode, got %q", metadata.RuntimeMode)
	}
	if metadata.TargetRef != "prod-k3s-ap" {
		t.Fatalf("expected target_ref prod-k3s-ap, got %q", metadata.TargetRef)
	}
}

func TestReadLinkMetadataRejectsMissingTargetRef(t *testing.T) {
	repoRoot := t.TempDir()
	payload := "project_slug: acme-shop\nruntime_mode: standalone\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "lazyops.yaml"), []byte(payload), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := ReadLinkMetadata(repoRoot); err == nil {
		t.Fatal("expected missing target_ref error, got nil")
	}
}

func TestReadDoctorMetadataReadsServicesAndDependencies(t *testing.T) {
	repoRoot := t.TempDir()
	payload := "" +
		"project_slug: acme-shop\n" +
		"runtime_mode: distributed-mesh\n\n" +
		"deployment_binding:\n" +
		"  target_ref: prod-mesh-ap\n\n" +
		"services:\n" +
		"  - name: api\n" +
		"    path: apps/api\n" +
		"  - name: web\n" +
		"    path: apps/web\n\n" +
		"dependency_bindings:\n" +
		"  - service: web\n" +
		"    alias: api\n" +
		"    target_service: api\n" +
		"    protocol: http\n" +
		"    local_endpoint: 127.0.0.1:8080\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "lazyops.yaml"), []byte(payload), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	metadata, err := ReadDoctorMetadata(repoRoot)
	if err != nil {
		t.Fatalf("ReadDoctorMetadata() error = %v", err)
	}

	if err := metadata.ValidateDoctorContract(); err != nil {
		t.Fatalf("ValidateDoctorContract() error = %v", err)
	}
	if err := metadata.ValidateDependencyDeclarations(); err != nil {
		t.Fatalf("ValidateDependencyDeclarations() error = %v", err)
	}
	if len(metadata.Services) != 2 {
		t.Fatalf("expected two services, got %+v", metadata.Services)
	}
	if len(metadata.DependencyBindings) != 1 {
		t.Fatalf("expected one dependency binding, got %+v", metadata.DependencyBindings)
	}
	if metadata.DependencyBindings[0].Service != "web" {
		t.Fatalf("expected dependency service web, got %+v", metadata.DependencyBindings[0])
	}
}

func TestDoctorMetadataValidateDependencyDeclarationsRejectsUnknownService(t *testing.T) {
	metadata := DoctorMetadata{
		ProjectSlug: "acme-shop",
		RuntimeMode: initplan.RuntimeModeDistributedMesh,
		TargetRef:   "prod-mesh-ap",
		Services: []DoctorService{
			{Name: "api", Path: "apps/api"},
		},
		DependencyBindings: []DoctorDependencyBinding{
			{
				Service:       "web",
				Alias:         "api",
				TargetService: "api",
				Protocol:      "http",
			},
		},
	}

	if err := metadata.ValidateDependencyDeclarations(); err == nil {
		t.Fatal("expected dependency declaration validation error, got nil")
	}
}
