package service

import (
	"strings"
	"testing"

	"lazyops-server/internal/models"
)

func TestProjectAIPromptServiceBuildsProjectWidePrompt(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:          "prj_123",
		UserID:      "usr_123",
		Name:        "Acme Platform",
		Slug:        "acme-platform",
		RuntimeMode: "distributed-k3s",
	})
	projectEnvStore := newFakeProjectEnvBundleStore()
	internalServiceStore := newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:        "frontend",
			Kind:        "frontend",
			Public:      true,
			Path:        "apps/frontend",
			TargetPort:  3000,
			ServicePort: 3000,
		},
		{
			Name:        "backend",
			Kind:        "api",
			Public:      true,
			Path:        "apps/backend",
			TargetPort:  8080,
			ServicePort: 8080,
		},
		{
			Name:       "db",
			Kind:       "postgres",
			SourceType: serviceSourceTypeInternal,
			ConnectionTemplate: map[string]string{
				"DB_URL":      "DATABASE_URL",
				"DB_NAME":     "DATABASE_NAME",
				"DB_HOST":     "PGHOST",
				"DB_PORT":     "PGPORT",
				"DB_USERNAME": "PGUSER",
				"DB_PASSWORD": "PGPASSWORD",
			},
			EnvBundle: map[string]string{
				"POSTGRES_DB":       "app",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "supersecret",
			},
		},
		{
			Name:        "socket",
			Kind:        "ws",
			Public:      true,
			Path:        "apps/socket",
			TargetPort:  3001,
			ServicePort: 3001,
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}

	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed services: %v", err)
	}

	envService := NewProjectEnvService(projectStore, projectEnvStore, internalServiceStore, "backend-secret-key").
		WithServiceStore(serviceStore).
		WithRoutingStore(newFakeRoutingPolicyRepo())
	if _, err := envService.Upsert(UpsertProjectEnvCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleViewer,
		ProjectID:       "prj_123",
		Content:         "DATABASE_URL=postgres://postgres:postgres@localhost:5432/app\nNEXT_PUBLIC_API_BASE=http://localhost:8080\nNEXT_PUBLIC_WS_URL=ws://localhost:3001\n",
	}); err != nil {
		t.Fatalf("upsert env bundle: %v", err)
	}

	routingRepo := newFakeRoutingPolicyRepo()
	routingService := NewRoutingService(routingRepo, serviceStore)
	promptService := NewProjectAIPromptService(projectStore, serviceStore, envService, routingService)

	record, err := promptService.Get("usr_123", RoleViewer, "prj_123")
	if err != nil {
		t.Fatalf("get ai prompt: %v", err)
	}

	if !strings.Contains(record.Prompt, "frontend -> /") {
		t.Fatalf("expected frontend root route in prompt, got %q", record.Prompt)
	}
	if !strings.Contains(record.Prompt, "backend -> /api") {
		t.Fatalf("expected api route in prompt, got %q", record.Prompt)
	}
	if !strings.Contains(record.Prompt, "socket -> /ws") {
		t.Fatalf("expected websocket route in prompt, got %q", record.Prompt)
	}
	if !strings.Contains(record.Prompt, "${DATABASE_URL}") {
		t.Fatalf("expected managed placeholder in prompt, got %q", record.Prompt)
	}
	if len(record.ServiceSnapshot) != 4 {
		t.Fatalf("expected four service snapshots, got %#v", record.ServiceSnapshot)
	}
	if len(record.MigrationFindings) < 3 {
		t.Fatalf("expected localhost findings, got %#v", record.MigrationFindings)
	}
}

func TestProjectAIPromptServicePrefersExplicitAPIKindOverFrontendHeuristic(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:          "prj_456",
		UserID:      "usr_123",
		Name:        "BBB",
		Slug:        "bbb",
		RuntimeMode: "distributed-k3s",
	})
	projectEnvStore := newFakeProjectEnvBundleStore()
	internalServiceStore := newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_456", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:        "be",
			Kind:        "api",
			Public:      true,
			Path:        "apps/be",
			TargetPort:  8080,
			ServicePort: 8080,
		},
		{
			Name:        "fe",
			Kind:        "web",
			Public:      true,
			Path:        "apps/fe",
			TargetPort:  3000,
			ServicePort: 3000,
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}

	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_456", serviceModels); err != nil {
		t.Fatalf("seed services: %v", err)
	}

	routesJSON, err := serializeRoutes([]models.RoutingRoute{
		{Path: "/", Service: "be"},
		{Path: "/fe", Service: "fe"},
	})
	if err != nil {
		t.Fatalf("serialize routes: %v", err)
	}
	routingRepo := newFakeRoutingPolicyRepo()
	routingRepo.policies["prj_456"] = &models.RoutingPolicy{
		ProjectID:  "prj_456",
		RoutesJSON: routesJSON,
	}

	envService := NewProjectEnvService(projectStore, projectEnvStore, internalServiceStore, "backend-secret-key").
		WithServiceStore(serviceStore).
		WithRoutingStore(routingRepo)
	promptService := NewProjectAIPromptService(projectStore, serviceStore, envService, NewRoutingService(routingRepo, serviceStore))

	record, err := promptService.Get("usr_123", RoleViewer, "prj_456")
	if err != nil {
		t.Fatalf("get ai prompt: %v", err)
	}

	snapshotByName := make(map[string]ProjectAIPromptServiceSnapshot, len(record.ServiceSnapshot))
	for _, item := range record.ServiceSnapshot {
		snapshotByName[item.Name] = item
	}
	if snapshotByName["be"].Role != "api" {
		t.Fatalf("expected explicit api kind to win for be, got %#v", snapshotByName["be"])
	}
	if snapshotByName["fe"].Role != "frontend" {
		t.Fatalf("expected fe to stay frontend, got %#v", snapshotByName["fe"])
	}
	if !strings.Contains(record.Prompt, "- be | role=api | kind=api") {
		t.Fatalf("expected prompt to expose be as api, got %q", record.Prompt)
	}
}
