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

func TestInitContractServiceBuildsMigrationFindingsAndSuggestedRoutes(t *testing.T) {
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
		"services":[
			{"name":"frontend","path":"apps/web","public":true},
			{"name":"api","path":"apps/api","public":true},
			{"name":"db","path":"apps/db"}
		],
		"dependency_bindings":[
			{"service":"api","alias":"db","target_service":"db","protocol":"tcp","local_endpoint":"localhost:5432"}
		],
		"compatibility_policy":{"env_injection":true,"managed_credentials":true,"localhost_rescue":true},
		"routing_policy":{"routes":[{"path":"/backend","service":"api"}]}
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
	if len(result.SuggestedRoutes) == 0 {
		t.Fatalf("expected suggested routes, got %#v", result)
	}
	if len(result.MigrationFindings) == 0 {
		t.Fatalf("expected migration findings, got %#v", result)
	}
	if result.MigrationFindings[0].CurrentValue != "localhost:5432" {
		t.Fatalf("expected localhost finding, got %#v", result.MigrationFindings[0])
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

func TestInitContractServiceRejectsK3sIncompatibleServiceName(t *testing.T) {
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
		Name:        "Cluster Binding",
		TargetRef:   "cluster-prod",
		RuntimeMode: "distributed-k3s",
		TargetKind:  "cluster",
		TargetID:    "clu_123",
	})
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:     "clu_123",
		UserID: "usr_123",
		Name:   "cluster-prod",
		Status: "online",
	})
	service := NewInitContractService(projectStore, bindingStore, newFakeInstanceStore(), newFakeMeshNetworkStore(), clusterStore)

	raw := []byte(`{
		"project_slug":"acme-api",
		"runtime_mode":"distributed-k3s",
		"deployment_binding":{"target_ref":"cluster-prod"},
		"services":[{"name":"api_v1","path":"apps/api"}],
		"compatibility_policy":{"env_injection":true}
	}`)

	_, err := service.ValidateLazyopsYAML(ValidateLazyopsYAMLCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		RawDocument:     raw,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "kubernetes-compatible") {
		t.Fatalf("expected kubernetes-compatible validation error, got %v", err)
	}
}
