package service

import (
	"errors"
	"strings"
	"testing"

	"lazyops-server/internal/models"
)

func TestInitContractServiceValidateLazyopsYAMLSuccess(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:               "bind_123",
		ProjectID:        "prj_123",
		Name:             "Production Binding",
		TargetRef:        "prod-main",
		RuntimeMode:      "standalone",
		TargetKind:       "instance",
		TargetID:         "inst_123",
		DomainPolicyJSON: `{"magic_domain_provider":"sslip.io"}`,
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:     "inst_123",
		UserID: "usr_123",
		Name:   "edge-sg-1",
		Status: "online",
	})
	service := NewInitContractService(projectStore, bindingStore, instanceStore, newFakeMeshNetworkStore(), newFakeClusterStore())

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"standalone",
		"deployment_binding":{"target_ref":"prod-main"},
		"services":[{"name":"api","path":"apps/api","healthcheck":{"path":"/healthz","port":8080}}],
		"compatibility_policy":{"env_injection":true,"managed_credentials":true,"localhost_rescue":true},
		"magic_domain_policy":{"enabled":true,"provider":"sslip.io"}
	}`)

	result, err := service.ValidateLazyopsYAML(ValidateLazyopsYAMLCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		RawDocument:     raw,
	})
	if err != nil {
		t.Fatalf("validate lazyops yaml: %v", err)
	}

	if result.Project.ID != "prj_123" {
		t.Fatalf("expected project id prj_123, got %q", result.Project.ID)
	}
	if result.DeploymentBinding.TargetRef != "prod-main" {
		t.Fatalf("expected target_ref prod-main, got %q", result.DeploymentBinding.TargetRef)
	}
	if result.TargetSummary.Kind != "instance" || result.TargetSummary.RuntimeMode != "standalone" {
		t.Fatalf("unexpected target summary %#v", result.TargetSummary)
	}
	if len(result.Schema.ForbiddenFieldNames) == 0 {
		t.Fatal("expected forbidden field names in schema summary")
	}
}

func TestInitContractServiceRejectsUnknownTargetRef(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	service := NewInitContractService(projectStore, newFakeDeploymentBindingStore(), newFakeInstanceStore(), newFakeMeshNetworkStore(), newFakeClusterStore())

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"standalone",
		"deployment_binding":{"target_ref":"prod-main"},
		"services":[{"name":"api","path":"apps/api"}],
		"compatibility_policy":{"env_injection":true}
	}`)

	_, err := service.ValidateLazyopsYAML(ValidateLazyopsYAMLCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		RawDocument:     raw,
	})
	if !errors.Is(err, ErrUnknownTargetRef) {
		t.Fatalf("expected ErrUnknownTargetRef, got %v", err)
	}
}

func TestInitContractServiceRejectsInvalidDependencyMapping(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production Binding",
		TargetRef:   "prod-main",
		RuntimeMode: "distributed-mesh",
		TargetKind:  "mesh",
		TargetID:    "mesh_123",
	})
	meshStore := newFakeMeshNetworkStore(&models.MeshNetwork{
		ID:       "mesh_123",
		UserID:   "usr_123",
		Name:     "mesh-prod",
		Provider: "wireguard",
		Status:   "online",
	})
	service := NewInitContractService(projectStore, bindingStore, newFakeInstanceStore(), meshStore, newFakeClusterStore())

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"distributed-mesh",
		"deployment_binding":{"target_ref":"prod-main"},
		"services":[{"name":"api","path":"apps/api"}],
		"dependency_bindings":[{"service":"worker","alias":"db","target_service":"app-db","protocol":"tcp","local_endpoint":"localhost:5432"}],
		"compatibility_policy":{"managed_credentials":true}
	}`)

	_, err := service.ValidateLazyopsYAML(ValidateLazyopsYAMLCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		RawDocument:     raw,
	})
	if !errors.Is(err, ErrInvalidDependencyMapping) {
		t.Fatalf("expected ErrInvalidDependencyMapping, got %v", err)
	}
}

func TestInitContractServiceRejectsSecretBearingConfig(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production Binding",
		TargetRef:   "prod-main",
		RuntimeMode: "standalone",
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:     "inst_123",
		UserID: "usr_123",
		Name:   "edge-sg-1",
		Status: "online",
	})
	service := NewInitContractService(projectStore, bindingStore, instanceStore, newFakeMeshNetworkStore(), newFakeClusterStore())

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"standalone",
		"deployment_binding":{"target_ref":"prod-main"},
		"services":[{"name":"api","path":"apps/api","start_hint":"ghp_abcdef"}],
		"compatibility_policy":{"env_injection":true}
	}`)

	_, err := service.ValidateLazyopsYAML(ValidateLazyopsYAMLCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		RawDocument:     raw,
	})
	if !errors.Is(err, ErrSecretBearingConfig) {
		t.Fatalf("expected ErrSecretBearingConfig, got %v", err)
	}
	if !strings.Contains(err.Error(), "start_hint") {
		t.Fatalf("expected start_hint path in error, got %v", err)
	}
}
