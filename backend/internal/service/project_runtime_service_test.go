package service

import (
	"context"
	"testing"

	"lazyops-server/internal/models"
)

func TestProjectRuntimeServiceReturnsEmptyStatesBeforeFirstDeployment(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		NamespaceSlug: "acme-api",
		RuntimeMode:   "distributed-k3s",
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:          "api",
			Path:          "apps/api",
			Kind:          "app",
			Public:        true,
			PlacementMode: servicePlacementModeSharedCluster,
		},
		{
			Name:            "worker",
			Path:            "apps/worker",
			Kind:            "worker",
			PlacementMode:   servicePlacementModePinnedNode,
			PlacementNodeID: "inst_worker",
		},
		{
			Name:                    "db",
			Kind:                    "postgres",
			SourceType:              serviceSourceTypeInternal,
			ConnectionTemplateKey:   "",
			ConnectionTargetService: "",
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}

	projectSvc := NewProjectService(projectStore).WithServiceStore(serviceStore)
	deployments := NewDeploymentService(projectStore, newFakeBlueprintStore(), newFakeDesiredStateRevisionStore(), newFakeDeploymentStore())
	runtimeSvc := NewProjectRuntimeService(projectStore, projectSvc, deployments, nil, nil, nil)

	result, err := runtimeSvc.Get(context.Background(), "usr_123", RoleOperator, "prj_123")
	if err != nil {
		t.Fatalf("project runtime summary: %v", err)
	}
	if result.SyncState != "missing" {
		t.Fatalf("expected missing sync state before first deployment, got %#v", result)
	}
	if result.SyncReason == "" {
		t.Fatalf("expected empty-state sync reason, got %#v", result)
	}
	if len(result.Services) != 3 {
		t.Fatalf("expected runtime summary for every configured service, got %#v", result.Services)
	}

	statusByName := map[string]ProjectRuntimeServiceRecord{}
	for _, item := range result.Services {
		statusByName[item.Name] = item
	}
	if statusByName["api"].RuntimeStatus != "not_deployed" {
		t.Fatalf("expected api to be not_deployed before first rollout, got %#v", statusByName["api"])
	}
	if statusByName["worker"].RuntimeStatus != "not_deployed" {
		t.Fatalf("expected pinned worker to remain not_deployed before first rollout, got %#v", statusByName["worker"])
	}
	if statusByName["db"].RuntimeStatus != "configured" {
		t.Fatalf("expected internal service to remain configured before first rollout, got %#v", statusByName["db"])
	}
}
