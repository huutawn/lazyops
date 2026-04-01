package service

import (
	"errors"
	"sort"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

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
