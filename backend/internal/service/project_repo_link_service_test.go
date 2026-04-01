package service

import (
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeProjectRepoLinkStore struct {
	byProjectID map[string]*models.ProjectRepoLink
	byRouteKey  map[string]*models.ProjectRepoLink
	upsertErr   error
	getErr      error
	lookupErr   error
}

func newFakeProjectRepoLinkStore(links ...*models.ProjectRepoLink) *fakeProjectRepoLinkStore {
	store := &fakeProjectRepoLinkStore{
		byProjectID: make(map[string]*models.ProjectRepoLink),
		byRouteKey:  make(map[string]*models.ProjectRepoLink),
	}

	for _, link := range links {
		store.put(link)
	}

	return store
}

func (f *fakeProjectRepoLinkStore) Upsert(link *models.ProjectRepoLink) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}

	cloned := *link
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	cloned.UpdatedAt = now
	f.put(&cloned)
	return nil
}

func (f *fakeProjectRepoLinkStore) GetByProjectID(projectID string) (*models.ProjectRepoLink, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if link, ok := f.byProjectID[projectID]; ok {
		return link, nil
	}

	return nil, nil
}

func (f *fakeProjectRepoLinkStore) GetByRepoBranch(githubInstallationID string, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if link, ok := f.byRouteKey[routeKey(githubInstallationID, githubRepoID, trackedBranch)]; ok {
		return link, nil
	}

	return nil, nil
}

func (f *fakeProjectRepoLinkStore) LookupWebhookRoute(githubInstallationID int64, githubRepoID int64, trackedBranch string) (*models.ProjectRepoLink, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}

	items := make([]*models.ProjectRepoLink, 0)
	for _, link := range f.byProjectID {
		if link.GitHubRepoID == githubRepoID && link.TrackedBranch == trackedBranch {
			items = append(items, link)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProjectID < items[j].ProjectID
	})
	for _, link := range items {
		if link.GitHubInstallationID == installationRecordIDForExternal(githubInstallationID) {
			return link, nil
		}
	}

	return nil, nil
}

func (f *fakeProjectRepoLinkStore) put(link *models.ProjectRepoLink) {
	f.byProjectID[link.ProjectID] = link
	f.byRouteKey[routeKey(link.GitHubInstallationID, link.GitHubRepoID, link.TrackedBranch)] = link
}

func routeKey(githubInstallationID string, githubRepoID int64, trackedBranch string) string {
	return fmt.Sprintf("%s|%d|%s", githubInstallationID, githubRepoID, trackedBranch)
}

func installationRecordIDForExternal(githubInstallationID int64) string {
	switch githubInstallationID {
	case 100:
		return "ghi_alpha"
	case 200:
		return "ghi_beta"
	default:
		return ""
	}
}

func TestProjectRepoLinkServiceSuccess(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	installStore := newFakeGitHubInstallationStore(&models.GitHubInstallation{
		ID:                   "ghi_alpha",
		UserID:               "usr_123",
		GitHubInstallationID: 100,
		AccountLogin:         "lazyops",
		AccountType:          "Organization",
		ScopeJSON:            `{"repository_selection":"selected","permissions":{"contents":"read"},"repositories":[{"id":42,"name":"backend","full_name":"lazyops/backend","owner_login":"lazyops","private":true}]}`,
		InstalledAt:          time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
	})
	repoLinkStore := newFakeProjectRepoLinkStore()
	service := NewProjectRepoLinkService(projectStore, installStore, repoLinkStore)

	result, err := service.LinkRepository(CreateProjectRepoLinkCommand{
		RequesterUserID:      "usr_123",
		RequesterRole:        RoleViewer,
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		TrackedBranch:        "refs/heads/main",
		PreviewEnabled:       true,
	})
	if err != nil {
		t.Fatalf("link repository: %v", err)
	}
	if result.ProjectID != "prj_123" {
		t.Fatalf("expected project id prj_123, got %q", result.ProjectID)
	}
	if result.TrackedBranch != "main" {
		t.Fatalf("expected normalized tracked branch main, got %q", result.TrackedBranch)
	}
	if result.RepoFullName != "lazyops/backend" {
		t.Fatalf("expected repo full name lazyops/backend, got %q", result.RepoFullName)
	}
	if !result.PreviewEnabled {
		t.Fatal("expected preview enabled to persist")
	}
}

func TestProjectRepoLinkServiceRejectsInaccessibleRepo(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	installStore := newFakeGitHubInstallationStore(&models.GitHubInstallation{
		ID:                   "ghi_alpha",
		UserID:               "usr_123",
		GitHubInstallationID: 100,
		AccountLogin:         "lazyops",
		AccountType:          "Organization",
		ScopeJSON:            `{"repository_selection":"selected","permissions":{"contents":"read"},"repositories":[{"id":42,"name":"backend","full_name":"lazyops/backend","owner_login":"lazyops","private":true}]}`,
		InstalledAt:          time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
	})
	service := NewProjectRepoLinkService(projectStore, installStore, newFakeProjectRepoLinkStore())

	_, err := service.LinkRepository(CreateProjectRepoLinkCommand{
		RequesterUserID:      "usr_123",
		RequesterRole:        RoleViewer,
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         99,
		TrackedBranch:        "main",
	})
	if !errors.Is(err, ErrRepoNotAccessible) {
		t.Fatalf("expected ErrRepoNotAccessible, got %v", err)
	}
}

func TestProjectRepoLinkServiceRejectsInvalidBranch(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	installStore := newFakeGitHubInstallationStore(&models.GitHubInstallation{
		ID:                   "ghi_alpha",
		UserID:               "usr_123",
		GitHubInstallationID: 100,
		AccountLogin:         "lazyops",
		AccountType:          "Organization",
		ScopeJSON:            `{"repository_selection":"selected","permissions":{"contents":"read"},"repositories":[{"id":42,"name":"backend","full_name":"lazyops/backend","owner_login":"lazyops","private":true}]}`,
		InstalledAt:          time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
	})
	service := NewProjectRepoLinkService(projectStore, installStore, newFakeProjectRepoLinkStore())

	_, err := service.LinkRepository(CreateProjectRepoLinkCommand{
		RequesterUserID:      "usr_123",
		RequesterRole:        RoleViewer,
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		TrackedBranch:        "feature bad",
	})
	if !errors.Is(err, ErrInvalidTrackedBranch) {
		t.Fatalf("expected ErrInvalidTrackedBranch, got %v", err)
	}
}

func TestProjectRepoLinkServiceRejectsOwnershipMismatch(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_owner",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	installStore := newFakeGitHubInstallationStore()
	service := NewProjectRepoLinkService(projectStore, installStore, newFakeProjectRepoLinkStore())

	_, err := service.LinkRepository(CreateProjectRepoLinkCommand{
		RequesterUserID:      "usr_other",
		RequesterRole:        RoleViewer,
		ProjectID:            "prj_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		TrackedBranch:        "main",
	})
	if !errors.Is(err, ErrProjectAccessDenied) {
		t.Fatalf("expected ErrProjectAccessDenied, got %v", err)
	}
}

func TestProjectRepoLinkServiceLookupWebhookRoute(t *testing.T) {
	service := NewProjectRepoLinkService(nil, nil, newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
		PreviewEnabled:       true,
	}))

	result, err := service.LookupWebhookRoute(WebhookRouteLookupCommand{
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		TrackedBranch:        "refs/heads/main",
	})
	if err != nil {
		t.Fatalf("lookup webhook route: %v", err)
	}
	if result.ProjectID != "prj_123" {
		t.Fatalf("expected project id prj_123, got %q", result.ProjectID)
	}
	if result.RepoFullName != "lazyops/backend" {
		t.Fatalf("expected full name lazyops/backend, got %q", result.RepoFullName)
	}
}
