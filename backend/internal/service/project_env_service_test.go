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
	if pack.PlaceholderEnv["DATABASE_NAME"] != "${DATABASE_NAME}" || pack.PlaceholderEnv["PGHOST"] != "${PGHOST}" {
		t.Fatalf("expected decomposed placeholders to be present, got %#v", pack.PlaceholderEnv)
	}
	if !containsManagedKey(record.ManagedKeys, "PGHOST") || !containsManagedKey(record.ManagedKeys, "PGPASSWORD") {
		t.Fatalf("expected decomposed managed keys, got %#v", record.ManagedKeys)
	}
	if !containsManagedKey(record.ManagedKeys, "DATABASE_URL") {
		t.Fatalf("expected managed keys to include DATABASE_URL, got %#v", record.ManagedKeys)
	}
	if len(record.ProvisionedKeys) == 0 {
		t.Fatalf("expected provisioned keys for managed defaults, got %#v", record)
	}
	if len(pack.RelatedServices) != 1 || pack.RelatedServices[0] != "db" {
		t.Fatalf("expected db helper to keep related service ownership, got %#v", pack.RelatedServices)
	}
}

func TestProjectEnvServiceBuildsMySQLHelpersFromUnifiedServiceInventory(t *testing.T) {
	projects := newFakeProjectStore(&models.Project{
		ID:          "prj_mysql",
		UserID:      "usr_123",
		Slug:        "demo",
		RuntimeMode: "distributed-k3s",
	})
	bundles := newFakeProjectEnvBundleStore()
	serviceModels, err := buildConfiguredProjectServiceModels("prj_mysql", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:                    "api",
			Path:                    "apps/api",
			Kind:                    "api",
			Public:                  true,
			ConnectionTemplateKey:   "mysql.basic",
			ConnectionTargetService: "mysql",
		},
		{
			Name:       "mysql",
			Kind:       "mysql",
			SourceType: serviceSourceTypeInternal,
			ConnectionTemplate: map[string]string{
				"DB_URL":      "DATABASE_URL",
				"DB_NAME":     "DB_NAME",
				"DB_HOST":     "DB_HOST",
				"DB_PORT":     "DB_PORT",
				"DB_USERNAME": "DB_USERNAME",
				"DB_PASSWORD": "DB_PASSWORD",
			},
			EnvBundle: map[string]string{
				"MYSQL_DATABASE": "app",
				"MYSQL_USER":     "mysql",
				"MYSQL_PASSWORD": "supersecret",
			},
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_mysql", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}

	service := NewProjectEnvService(projects, bundles, newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{}), "backend-secret-key").
		WithServiceStore(serviceStore)
	record, err := service.Get("usr_123", RoleOperator, "prj_mysql")
	if err != nil {
		t.Fatalf("get project env helpers: %v", err)
	}
	if len(record.HelperPacks) == 0 {
		t.Fatalf("expected helper packs for mysql inventory, got %#v", record)
	}
	var mysqlPack *ProjectEnvHelperPack
	for index := range record.HelperPacks {
		if record.HelperPacks[index].SourceService == "mysql" && record.HelperPacks[index].Category == "database" {
			mysqlPack = &record.HelperPacks[index]
			break
		}
	}
	if mysqlPack == nil {
		t.Fatalf("expected mysql helper pack, got %#v", record.HelperPacks)
	}
	if mysqlPack.LocalExampleEnv["DATABASE_URL"] != "mysql://mysql:mysql@tcp(localhost:3306)/app" {
		t.Fatalf("expected mysql local url example, got %#v", mysqlPack.LocalExampleEnv)
	}
	if !containsManagedKey(record.ManagedKeys, "DB_HOST") || !containsManagedKey(record.ManagedKeys, "DATABASE_URL") {
		t.Fatalf("expected mysql managed keys to include url and decomposed keys, got %#v", record.ManagedKeys)
	}
	if len(mysqlPack.RelatedServices) != 2 || mysqlPack.RelatedServices[0] != "api" || mysqlPack.RelatedServices[1] != "mysql" {
		t.Fatalf("expected mysql helper to include dependent service ownership, got %#v", mysqlPack.RelatedServices)
	}
}

func TestProjectEnvServiceBuildsMongoHelpersIncludingMongoURI(t *testing.T) {
	projects := newFakeProjectStore(&models.Project{
		ID:          "prj_mongo",
		UserID:      "usr_123",
		Slug:        "demo",
		RuntimeMode: "distributed-k3s",
	})
	bundles := newFakeProjectEnvBundleStore()
	serviceModels, err := buildConfiguredProjectServiceModels("prj_mongo", "distributed-k3s", []ConfigureProjectServiceItem{
		{
			Name:                    "api",
			Path:                    "apps/api",
			Kind:                    "api",
			Public:                  true,
			ConnectionTargetService: "mongodb",
		},
		{
			Name:       "mongodb",
			Kind:       "mongodb",
			SourceType: serviceSourceTypeInternal,
			EnvBundle: map[string]string{
				"MONGO_INITDB_DATABASE": "tamsang",
			},
		},
	})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_mongo", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}

	service := NewProjectEnvService(projects, bundles, newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{}), "backend-secret-key").
		WithServiceStore(serviceStore)
	record, err := service.Get("usr_123", RoleOperator, "prj_mongo")
	if err != nil {
		t.Fatalf("get project env helpers: %v", err)
	}
	if len(record.HelperPacks) == 0 {
		t.Fatalf("expected helper packs for mongo inventory, got %#v", record)
	}

	var mongoPack *ProjectEnvHelperPack
	for index := range record.HelperPacks {
		if record.HelperPacks[index].SourceService == "mongodb" && record.HelperPacks[index].Category == "database" {
			mongoPack = &record.HelperPacks[index]
			break
		}
	}
	if mongoPack == nil {
		t.Fatalf("expected mongo helper pack, got %#v", record.HelperPacks)
	}
	if mongoPack.PrimaryKey != "MONGODB_URI" {
		t.Fatalf("expected primary key MONGODB_URI, got %#v", mongoPack)
	}
	if mongoPack.LocalExampleEnv["MONGODB_URI"] != "mongodb://localhost:27017/tamsang" {
		t.Fatalf("expected mongodb uri local example, got %#v", mongoPack.LocalExampleEnv)
	}
	if mongoPack.LocalExampleEnv["MONGODB_URL"] != "mongodb://localhost:27017/tamsang" {
		t.Fatalf("expected mongodb url local example, got %#v", mongoPack.LocalExampleEnv)
	}
	if !containsManagedKey(record.ManagedKeys, "MONGODB_URI") || !containsManagedKey(record.ManagedKeys, "MONGODB_URL") {
		t.Fatalf("expected managed keys to include mongodb uri aliases, got %#v", record.ManagedKeys)
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
