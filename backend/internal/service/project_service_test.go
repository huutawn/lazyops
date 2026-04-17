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
	existing := f.items[projectID]
	cloned := make([]models.Service, 0, len(existing)+len(items))
	for _, item := range existing {
		if strings.HasPrefix(item.Path, reservedManagedInternalServicePathPrefix) {
			cloned = append(cloned, item)
		}
	}
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
	for index := range result.Items {
		switch result.Items[index].Name {
		case "api":
			apiSvc = &result.Items[index]
		case "web":
			webSvc = &result.Items[index]
		}
	}
	if apiSvc == nil || webSvc == nil {
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
