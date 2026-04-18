package service

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeProjectServiceStoreForProjectSvc struct {
	items map[string][]models.Service
	err   error
}

func newFakeProjectServiceStoreForProjectSvc(items map[string][]models.Service) *fakeProjectServiceStoreForProjectSvc {
	return &fakeProjectServiceStoreForProjectSvc{items: items}
}

func (f *fakeProjectServiceStoreForProjectSvc) ReplaceForProject(projectID string, items []models.Service) error {
	cloned := make([]models.Service, 0, len(items))
	cloned = append(cloned, items...)
	if f.items == nil {
		f.items = make(map[string][]models.Service)
	}
	f.items[projectID] = cloned
	return f.err
}

func (f *fakeProjectServiceStoreForProjectSvc) ListByProject(projectID string) ([]models.Service, error) {
	if f.err != nil {
		return nil, f.err
	}
	items := f.items[projectID]
	cloned := make([]models.Service, len(items))
	copy(cloned, items)
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].Name < cloned[j].Name
	})
	return cloned, nil
}

type fakeProjectInternalServiceStoreForProjectSvc struct {
	items map[string][]models.ProjectInternalService
	err   error
}

func newFakeProjectInternalServiceStoreForProjectSvc(items map[string][]models.ProjectInternalService) *fakeProjectInternalServiceStoreForProjectSvc {
	return &fakeProjectInternalServiceStoreForProjectSvc{items: items}
}

func (f *fakeProjectInternalServiceStoreForProjectSvc) ReplaceForProject(projectID string, items []models.ProjectInternalService) error {
	if f.err != nil {
		return f.err
	}
	cloned := make([]models.ProjectInternalService, len(items))
	copy(cloned, items)
	if f.items == nil {
		f.items = make(map[string][]models.ProjectInternalService)
	}
	f.items[projectID] = cloned
	return nil
}

func (f *fakeProjectInternalServiceStoreForProjectSvc) ListByProject(projectID string) ([]models.ProjectInternalService, error) {
	if f.err != nil {
		return nil, f.err
	}
	items := f.items[projectID]
	cloned := make([]models.ProjectInternalService, len(items))
	copy(cloned, items)
	sort.Slice(cloned, func(i, j int) bool {
		if cloned[i].Kind != cloned[j].Kind {
			return cloned[i].Kind < cloned[j].Kind
		}
		return cloned[i].Alias < cloned[j].Alias
	})
	return cloned, nil
}

type fakeProjectStore struct {
	byID       map[string]*models.Project
	byUserSlug map[string]*models.Project
	createErr  error
	listErr    error
	getErr     error
}

func newFakeProjectStore(projects ...*models.Project) *fakeProjectStore {
	store := &fakeProjectStore{
		byID:       make(map[string]*models.Project),
		byUserSlug: make(map[string]*models.Project),
	}

	for _, project := range projects {
		cloned := *project
		store.byID[project.ID] = &cloned
		store.byUserSlug[project.UserID+":"+project.Slug] = &cloned
	}

	return store
}

func (f *fakeProjectStore) Create(project *models.Project) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *project
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	f.byID[cloned.ID] = &cloned
	f.byUserSlug[cloned.UserID+":"+cloned.Slug] = &cloned
	return nil
}

func (f *fakeProjectStore) ListByUser(userID string) ([]models.Project, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	items := make([]models.Project, 0)
	for _, project := range f.byID {
		if project.UserID == userID {
			items = append(items, *project)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Slug < items[j].Slug
	})

	return items, nil
}

func (f *fakeProjectStore) GetBySlugForUser(userID, slug string) (*models.Project, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if project, ok := f.byUserSlug[userID+":"+slug]; ok {
		return project, nil
	}

	return nil, nil
}

func (f *fakeProjectStore) GetByIDForUser(userID, projectID string) (*models.Project, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	project, ok := f.byID[projectID]
	if !ok || project.UserID != userID {
		return nil, nil
	}

	return project, nil
}

func (f *fakeProjectStore) GetByID(projectID string) (*models.Project, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	project, ok := f.byID[projectID]
	if !ok {
		return nil, nil
	}

	return project, nil
}

func TestProjectServiceCreateDefaultsSlugAndBranch(t *testing.T) {
	store := newFakeProjectStore()
	service := NewProjectService(store)

	result, err := service.Create(CreateProjectCommand{
		UserID: "usr_123",
		Name:   "  Acme   Shop API  ",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if result.ID == "" || result.ID[:4] != "prj_" {
		t.Fatalf("expected prefixed project id, got %q", result.ID)
	}
	if result.Name != "Acme Shop API" {
		t.Fatalf("expected normalized name, got %q", result.Name)
	}
	if result.Slug != "acme-shop-api" {
		t.Fatalf("expected slug to be generated, got %q", result.Slug)
	}
	if result.DefaultBranch != "main" {
		t.Fatalf("expected default branch main, got %q", result.DefaultBranch)
	}
}

func TestProjectServiceRejectsDuplicateSlugPerUser(t *testing.T) {
	store := newFakeProjectStore(&models.Project{
		ID:            "prj_existing",
		UserID:        "usr_123",
		Name:          "Acme Shop",
		Slug:          "acme-shop",
		DefaultBranch: "main",
	})
	service := NewProjectService(store)

	_, err := service.Create(CreateProjectCommand{
		UserID: "usr_123",
		Name:   "Acme Shop",
	})
	if !errors.Is(err, ErrProjectSlugExists) {
		t.Fatalf("expected ErrProjectSlugExists, got %v", err)
	}

	result, err := service.Create(CreateProjectCommand{
		UserID: "usr_other",
		Name:   "Acme Shop",
	})
	if err != nil {
		t.Fatalf("create project for different user: %v", err)
	}
	if result.Slug != "acme-shop" {
		t.Fatalf("expected same normalized slug for different user, got %q", result.Slug)
	}
}

func TestProjectServiceListScopesProjectsToOwner(t *testing.T) {
	store := newFakeProjectStore(
		&models.Project{
			ID:            "prj_1",
			UserID:        "usr_123",
			Name:          "API",
			Slug:          "api",
			DefaultBranch: "main",
			CreatedAt:     time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC),
		},
		&models.Project{
			ID:            "prj_2",
			UserID:        "usr_other",
			Name:          "Web",
			Slug:          "web",
			DefaultBranch: "main",
			CreatedAt:     time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
		},
	)
	service := NewProjectService(store)

	items, err := service.List("usr_123")
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one project, got %d", len(items))
	}
	if items[0].ID != "prj_1" {
		t.Fatalf("expected owner-scoped project, got %q", items[0].ID)
	}
}

func TestProjectServiceListServicesReturnsPersistedServices(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	serviceStore := newFakeProjectServiceStoreForProjectSvc(map[string][]models.Service{
		"prj_1": {
			{
				ID:              "svc_api",
				ProjectID:       "prj_1",
				Name:            "api",
				Path:            "apps/api",
				Kind:            "app",
				TargetPort:      8080,
				ServicePort:     80,
				HealthcheckJSON: `{"path":"/healthz","port":8080,"protocol":"http"}`,
			},
		},
	})

	service := NewProjectService(projectStore).WithServiceStore(serviceStore)
	result, err := service.ListServices("usr_123", RoleAdmin, "prj_1")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one service, got %d", len(result.Items))
	}
	if result.Items[0].Name != "api" || result.Items[0].TargetPort != 8080 || result.Items[0].ServicePort != 80 {
		t.Fatalf("unexpected service record %#v", result.Items[0])
	}
	if result.Items[0].SourceType != serviceSourceTypeRepo {
		t.Fatalf("expected repo source type, got %#v", result.Items[0])
	}
	if result.Items[0].PlacementMode != servicePlacementModeSharedCluster {
		t.Fatalf("expected shared_cluster placement, got %#v", result.Items[0])
	}
	if result.Items[0].ManagedByLazyops {
		t.Fatalf("expected repo service to stay unmanaged, got %#v", result.Items[0])
	}
}

func TestProjectServiceListServicesBridgesLegacyInternalServices(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	serviceStore := newFakeProjectServiceStoreForProjectSvc(map[string][]models.Service{
		"prj_1": {
			{
				ID:          "svc_api",
				ProjectID:   "prj_1",
				Name:        "api",
				Path:        "apps/api",
				Kind:        "app",
				TargetPort:  8080,
				ServicePort: 8080,
			},
		},
	})
	internalStore := newFakeProjectInternalServiceStoreForProjectSvc(map[string][]models.ProjectInternalService{
		"prj_1": {
			{
				ID:            "insvc_postgres",
				ProjectID:     "prj_1",
				Kind:          "postgres",
				Alias:         "postgres",
				Protocol:      "tcp",
				Port:          5432,
				LocalEndpoint: "localhost:5432",
			},
		},
	})

	svc := NewProjectService(projectStore, internalStore).WithServiceStore(serviceStore)
	result, err := svc.ListServices("usr_123", RoleAdmin, "prj_1")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected repo and bridged internal services, got %d", len(result.Items))
	}
	if result.Items[0].Name != "api" || result.Items[0].SourceType != serviceSourceTypeRepo {
		t.Fatalf("expected repo service first, got %#v", result.Items[0])
	}
	internal := result.Items[1]
	if internal.SourceType != serviceSourceTypeInternal || !internal.ManagedByLazyops {
		t.Fatalf("expected bridged internal metadata, got %#v", internal)
	}
	if internal.Name != "lazyops-internal-postgres" || internal.Path != ".lazyops/internal/postgres" {
		t.Fatalf("expected bridged internal identity, got %#v", internal)
	}
	if internal.TargetPort != 5432 || internal.ServicePort != 5432 {
		t.Fatalf("expected bridged internal ports, got %#v", internal)
	}
}

func TestProjectServiceListServicesDedupesManagedInternalMirror(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	serviceStore := newFakeProjectServiceStoreForProjectSvc(map[string][]models.Service{
		"prj_1": {
			{
				ID:          "svc_api",
				ProjectID:   "prj_1",
				Name:        "api",
				Path:        "apps/api",
				Kind:        "app",
				TargetPort:  8080,
				ServicePort: 8080,
			},
			{
				ID:          "svc_postgres",
				ProjectID:   "prj_1",
				Name:        "lazyops-internal-postgres",
				Path:        ".lazyops/internal/postgres",
				Kind:        "postgres",
				TargetPort:  5432,
				ServicePort: 5432,
			},
		},
	})
	internalStore := newFakeProjectInternalServiceStoreForProjectSvc(map[string][]models.ProjectInternalService{
		"prj_1": {
			{
				ID:            "insvc_postgres",
				ProjectID:     "prj_1",
				Kind:          "postgres",
				Alias:         "postgres",
				Protocol:      "tcp",
				Port:          5432,
				LocalEndpoint: "localhost:5432",
			},
		},
	})

	svc := NewProjectService(projectStore, internalStore).WithServiceStore(serviceStore)
	result, err := svc.ListServices("usr_123", RoleAdmin, "prj_1")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected managed internal service to avoid duplication, got %#v", result.Items)
	}
	internalCount := 0
	for _, item := range result.Items {
		if item.SourceType == serviceSourceTypeInternal {
			internalCount++
		}
	}
	if internalCount != 1 {
		t.Fatalf("expected a single internal service in unified inventory, got %#v", result.Items)
	}
}

func TestProjectServiceConfigureServicesPersistsServiceCatalog(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	serviceStore := newFakeProjectServiceStoreForProjectSvc(map[string][]models.Service{
		"prj_1": {
			{
				ID:        "svc_internal",
				ProjectID: "prj_1",
				Name:      "lazyops-internal-postgres",
				Path:      ".lazyops/internal/postgres",
				Kind:      "postgres",
			},
		},
	})

	service := NewProjectService(projectStore).WithServiceStore(serviceStore)
	result, err := service.ConfigureServices(ConfigureProjectServicesCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleAdmin,
		ProjectID:       "prj_1",
		Items: []ConfigureProjectServiceItem{
			{
				Name:       "api",
				Path:       "apps/api",
				Kind:       "backend",
				Public:     false,
				TargetPort: 8080,
				Replicas:   2,
				Healthcheck: map[string]any{
					"path": "/ready",
					"port": 8080,
				},
			},
			{
				Name:       "web",
				Path:       "apps/web",
				Kind:       "frontend",
				Public:     true,
				TargetPort: 3000,
			},
		},
	})
	if err != nil {
		t.Fatalf("configure services: %v", err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 services including preserved internal, got %d", len(result.Items))
	}

	var apiSvc *ProjectServiceRecord
	var webSvc *ProjectServiceRecord
	var internalSvc *ProjectServiceRecord
	for index := range result.Items {
		switch result.Items[index].Name {
		case "api":
			apiSvc = &result.Items[index]
		case "web":
			webSvc = &result.Items[index]
		case "lazyops-internal-postgres":
			internalSvc = &result.Items[index]
		}
	}
	if apiSvc == nil || webSvc == nil || internalSvc == nil {
		t.Fatalf("expected api and web services in result, got %#v", result.Items)
	}
	if apiSvc.Kind != "backend" || apiSvc.Replicas != 2 || apiSvc.TargetPort != 8080 || apiSvc.ServicePort != 8080 {
		t.Fatalf("unexpected api service record %#v", apiSvc)
	}
	if extractHealthcheckPort(apiSvc.Healthcheck) != 8080 {
		t.Fatalf("expected api healthcheck port 8080, got %#v", apiSvc.Healthcheck)
	}
	if webSvc.RuntimeProfile != "web" || !webSvc.Public {
		t.Fatalf("expected web service to infer runtime profile web, got %#v", webSvc)
	}
	if internalSvc.SourceType != serviceSourceTypeInternal || !internalSvc.ManagedByLazyops {
		t.Fatalf("expected preserved internal service metadata, got %#v", internalSvc)
	}
}

func TestProjectServiceConfigureServicesRejectsReservedInternalPath(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	service := NewProjectService(projectStore).WithServiceStore(newFakeProjectServiceStoreForProjectSvc(nil))

	_, err := service.ConfigureServices(ConfigureProjectServicesCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleAdmin,
		ProjectID:       "prj_1",
		Items: []ConfigureProjectServiceItem{
			{Name: "db", Path: ".lazyops/internal/postgres"},
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestProjectServiceConfigureServicesSupportsInternalPostgresCatalogItem(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	serviceStore := newFakeProjectServiceStoreForProjectSvc(nil)
	service := NewProjectService(projectStore).WithServiceStore(serviceStore)

	result, err := service.ConfigureServices(ConfigureProjectServicesCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleAdmin,
		ProjectID:       "prj_1",
		Items: []ConfigureProjectServiceItem{
			{
				Name:       "db",
				Kind:       "postgres",
				SourceType: serviceSourceTypeInternal,
			},
		},
	})
	if err != nil {
		t.Fatalf("configure internal postgres service: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one internal service, got %#v", result.Items)
	}
	item := result.Items[0]
	if item.Path != ".lazyops/internal/postgres/db" {
		t.Fatalf("expected normalized internal postgres path, got %q", item.Path)
	}
	if item.ImageRef != "postgres:16-alpine" {
		t.Fatalf("expected default postgres image, got %q", item.ImageRef)
	}
	if item.TargetPort != 5432 || item.ServicePort != 5432 {
		t.Fatalf("expected postgres ports 5432, got %#v", item)
	}
	if item.RuntimeProfile != "internal-db" || !item.ManagedByLazyops {
		t.Fatalf("expected managed internal-db profile, got %#v", item)
	}
	if extractHealthcheckPort(item.Healthcheck) != 5432 {
		t.Fatalf("expected tcp healthcheck on 5432, got %#v", item.Healthcheck)
	}
}

func TestProjectServiceConfigureServicesRejectsK3sIncompatibleServiceName(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "distributed-k3s",
		DefaultBranch: "main",
	})
	service := NewProjectService(projectStore).WithServiceStore(newFakeProjectServiceStoreForProjectSvc(nil))

	_, err := service.ConfigureServices(ConfigureProjectServicesCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleAdmin,
		ProjectID:       "prj_1",
		Items: []ConfigureProjectServiceItem{
			{Name: "api_v1", Path: "apps/api"},
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "kubernetes-compatible") {
		t.Fatalf("expected kubernetes-compatible validation error, got %v", err)
	}
}

func TestProjectServiceConfigureServicesAllowsLegacyStandaloneLogicalNames(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_1",
		UserID:        "usr_123",
		Name:          "API",
		Slug:          "api",
		NamespaceSlug: "api",
		RuntimeMode:   "standalone",
		DefaultBranch: "main",
	})
	serviceStore := newFakeProjectServiceStoreForProjectSvc(nil)
	service := NewProjectService(projectStore).WithServiceStore(serviceStore)

	result, err := service.ConfigureServices(ConfigureProjectServicesCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleAdmin,
		ProjectID:       "prj_1",
		Items: []ConfigureProjectServiceItem{
			{Name: "api_v1", Path: "apps/api"},
		},
	})
	if err != nil {
		t.Fatalf("expected standalone service name to pass, got %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Name != "api_v1" {
		t.Fatalf("unexpected configured services: %#v", result.Items)
	}
}
