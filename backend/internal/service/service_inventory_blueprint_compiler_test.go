package service

import (
	"testing"

	"lazyops-server/internal/models"
)

func TestServiceInventoryBlueprintCompilerInjectsPostgresTemplatePerService(t *testing.T) {
	project := models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		NamespaceSlug: "acme-api",
		RuntimeMode:   "distributed-k3s",
	}
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:                    "api",
			Path:                    "apps/api",
			Kind:                    "app",
			Public:                  true,
			ConnectionTemplateKey:   "postgres.basic",
			ConnectionTargetService: "db",
			EnvBundle: map[string]string{
				"APP_ENV": "production",
			},
		},
		{
			Name:       "db",
			Kind:       "postgres",
			SourceType: serviceSourceTypeInternal,
			EnvBundle: map[string]string{
				"POSTGRES_DB":       "app",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "supersecret",
			},
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto Primary",
		TargetRef:               "auto-primary",
		RuntimeMode:             "distributed-k3s",
		TargetKind:              "cluster",
		TargetID:                "clu_123",
		CompatibilityPolicyJSON: `{"env_injection":true}`,
		PlacementPolicyJSON:     `{}`,
		DomainPolicyJSON:        `{}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	blueprintStore := newFakeBlueprintStore()
	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, serviceStore, blueprintStore)

	result, err := compiler.Compile(project, ServiceInventoryBlueprintCompileInput{
		TriggerKind: "manual",
	})
	if err != nil {
		t.Fatalf("compile hidden blueprint snapshot: %v", err)
	}
	if result.Blueprint.SourceKind != hiddenServiceInventoryBlueprintSourceKind {
		t.Fatalf("expected hidden source kind, got %q", result.Blueprint.SourceKind)
	}
	if len(result.Blueprint.Compiled.DependencyBindings) != 1 {
		t.Fatalf("expected one generated postgres dependency binding, got %#v", result.Blueprint.Compiled.DependencyBindings)
	}

	var apiSvc *BlueprintServiceContractRecord
	for index := range result.Blueprint.Compiled.Services {
		if result.Blueprint.Compiled.Services[index].Name == "api" {
			apiSvc = &result.Blueprint.Compiled.Services[index]
			break
		}
	}
	if apiSvc == nil {
		t.Fatalf("expected api service in compiled services, got %#v", result.Blueprint.Compiled.Services)
	}
	if apiSvc.EnvBundle["DB_HOST"] != "db" {
		t.Fatalf("expected DB_HOST=db, got %#v", apiSvc.EnvBundle)
	}
	if apiSvc.EnvBundle["DB_PORT"] != "5432" {
		t.Fatalf("expected DB_PORT=5432, got %#v", apiSvc.EnvBundle)
	}
	if apiSvc.EnvBundle["DB_USERNAME"] != "postgres" || apiSvc.EnvBundle["DB_PASSWORD"] != "supersecret" {
		t.Fatalf("expected postgres credentials to be injected, got %#v", apiSvc.EnvBundle)
	}
	if apiSvc.EnvBundle["DB_URL"] != "postgres://postgres:supersecret@db:5432/app" {
		t.Fatalf("expected DB_URL to use internal dns host, got %#v", apiSvc.EnvBundle)
	}
}
