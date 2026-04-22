package service

import (
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeProjectEnvBundleStore struct {
	byProject map[string]*models.ProjectEnvBundle
}

func newFakeProjectEnvBundleStore(items ...*models.ProjectEnvBundle) *fakeProjectEnvBundleStore {
	store := &fakeProjectEnvBundleStore{byProject: make(map[string]*models.ProjectEnvBundle)}
	for _, item := range items {
		clone := *item
		store.byProject[item.ProjectID] = &clone
	}
	return store
}

func (f *fakeProjectEnvBundleStore) GetByProject(projectID string) (*models.ProjectEnvBundle, error) {
	if item, ok := f.byProject[projectID]; ok {
		clone := *item
		return &clone, nil
	}
	return nil, nil
}

func (f *fakeProjectEnvBundleStore) Upsert(bundle *models.ProjectEnvBundle) error {
	now := time.Now().UTC()
	clone := *bundle
	if clone.CreatedAt.IsZero() {
		clone.CreatedAt = now
	}
	clone.UpdatedAt = now
	f.byProject[bundle.ProjectID] = &clone
	return nil
}

func (f *fakeProjectEnvBundleStore) DeleteByProject(projectID string) error {
	delete(f.byProject, projectID)
	return nil
}

type fakeProjectInternalServiceStore struct {
	items map[string][]models.ProjectInternalService
}

func newFakeProjectInternalServiceStore(items map[string][]models.ProjectInternalService) *fakeProjectInternalServiceStore {
	return &fakeProjectInternalServiceStore{items: items}
}

func (f *fakeProjectInternalServiceStore) ReplaceForProject(projectID string, items []models.ProjectInternalService) error {
	cloned := make([]models.ProjectInternalService, len(items))
	copy(cloned, items)
	f.items[projectID] = cloned
	return nil
}

func (f *fakeProjectInternalServiceStore) ListByProject(projectID string) ([]models.ProjectInternalService, error) {
	items := f.items[projectID]
	cloned := make([]models.ProjectInternalService, len(items))
	copy(cloned, items)
	return cloned, nil
}

func TestProjectEnvServiceUpsertAndLoadRuntimeEnv(t *testing.T) {
	projects := newFakeProjectStore(&models.Project{ID: "prj_123", UserID: "usr_123", Slug: "demo"})
	bundles := newFakeProjectEnvBundleStore()
	internalServices := newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{
		"prj_123": {{ProjectID: "prj_123", Kind: "postgres", Alias: "postgres", Protocol: "tcp", LocalEndpoint: "localhost:5432"}},
	})
	service := NewProjectEnvService(projects, bundles, internalServices, "backend-secret-key")

	record, err := service.Upsert(UpsertProjectEnvCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Content:         "APP_ENV=prod\nTOKEN=abc123\n",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !record.Configured {
		t.Fatal("expected configured=true")
	}
	if len(record.Keys) != 2 || record.Keys[0] != "APP_ENV" || record.Keys[1] != "TOKEN" {
		t.Fatalf("unexpected keys %#v", record.Keys)
	}
	if record.UpdatedAt == nil || record.UpdatedAt.IsZero() {
		t.Fatal("expected updated_at to be populated")
	}
	if len(record.HelperPacks) != 1 {
		t.Fatalf("expected one helper pack, got %#v", record.HelperPacks)
	}
	if record.HelperPacks[0].LocalExampleEnv["DATABASE_URL"] == "" {
		t.Fatalf("expected safe local example, got %#v", record.HelperPacks[0])
	}

	envMap, err := service.LoadRuntimeEnv("prj_123")
	if err != nil {
		t.Fatalf("load runtime env: %v", err)
	}
	if envMap["APP_ENV"] != "prod" || envMap["TOKEN"] != "abc123" {
		t.Fatalf("unexpected runtime env %#v", envMap)
	}
}

func TestProjectEnvServiceParsesWarningsAndQuotes(t *testing.T) {
	envMap, warnings, err := parseProjectEnvContent("# comment\nexport APP_ENV=prod\nTOKEN=first\nTOKEN=\"second\"\n")
	if err != nil {
		t.Fatalf("parse env: %v", err)
	}
	if envMap["APP_ENV"] != "prod" || envMap["TOKEN"] != "second" {
		t.Fatalf("unexpected env map %#v", envMap)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %#v", warnings)
	}
}

func TestProjectEnvServiceRejectsInvalidLine(t *testing.T) {
	_, _, err := parseProjectEnvContent("BROKEN_LINE")
	if err == nil {
		t.Fatal("expected invalid env line to fail")
	}
}

func TestProjectEnvServiceBuildsPostgresHelpersFromUnifiedServiceInventory(t *testing.T) {
	projects := newFakeProjectStore(&models.Project{
		ID:          "prj_123",
		UserID:      "usr_123",
		Slug:        "demo",
		RuntimeMode: "distributed-k3s",
	})
	bundles := newFakeProjectEnvBundleStore()
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{
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
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}

	service := NewProjectEnvService(projects, bundles, newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{}), "backend-secret-key").
		WithServiceStore(serviceStore)
	record, err := service.Get("usr_123", RoleOperator, "prj_123")
	if err != nil {
		t.Fatalf("get project env helpers: %v", err)
	}
	if len(record.HelperPacks) != 1 {
		t.Fatalf("expected one postgres helper pack, got %#v", record.HelperPacks)
	}
	pack := record.HelperPacks[0]
	if pack.PrimaryKey != "DATABASE_URL" {
		t.Fatalf("expected primary key DATABASE_URL, got %#v", pack)
	}
	if pack.EnvExample["DATABASE_URL"] != "" {
		t.Fatalf("expected .env example to stay blank, got %#v", pack.EnvExample)
	}
	if pack.PlaceholderEnv["DATABASE_URL"] != "${DATABASE_URL}" {
		t.Fatalf("expected placeholder env token, got %#v", pack.PlaceholderEnv)
	}
	if !containsManagedKey(record.ManagedKeys, "DATABASE_URL") {
		t.Fatalf("expected managed keys to include DATABASE_URL, got %#v", record.ManagedKeys)
	}
	if len(record.ProvisionedKeys) == 0 {
		t.Fatalf("expected provisioned keys for managed defaults, got %#v", record)
	}
}

func containsManagedKey(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
