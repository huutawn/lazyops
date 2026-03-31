package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeGitHubInstallationStore struct {
	byInstallationID map[int64]*models.GitHubInstallation
	upsertErr        error
	listErr          error
	getErr           error
	revokeErr        error
	revokedIDs       []int64
}

type fakeGitHubInstallationsProvider struct {
	snapshots []GitHubInstallationSnapshot
	err       error
}

func newFakeGitHubInstallationStore(installations ...*models.GitHubInstallation) *fakeGitHubInstallationStore {
	store := &fakeGitHubInstallationStore{
		byInstallationID: make(map[int64]*models.GitHubInstallation),
	}
	for _, installation := range installations {
		store.byInstallationID[installation.GitHubInstallationID] = installation
	}
	return store
}

func (f *fakeGitHubInstallationStore) Upsert(installation *models.GitHubInstallation) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}

	cloned := *installation
	f.byInstallationID[installation.GitHubInstallationID] = &cloned
	return nil
}

func (f *fakeGitHubInstallationStore) ListByUser(userID string) ([]models.GitHubInstallation, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	items := make([]models.GitHubInstallation, 0, len(f.byInstallationID))
	for _, installation := range f.byInstallationID {
		if installation.UserID == userID {
			items = append(items, *installation)
		}
	}
	return items, nil
}

func (f *fakeGitHubInstallationStore) GetByInstallationIDForUser(userID string, installationID int64) (*models.GitHubInstallation, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	installation, ok := f.byInstallationID[installationID]
	if !ok || installation.UserID != userID {
		return nil, nil
	}
	return installation, nil
}

func (f *fakeGitHubInstallationStore) RevokeMissing(userID string, activeInstallationIDs []int64, at time.Time) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}

	active := make(map[int64]struct{}, len(activeInstallationIDs))
	for _, installationID := range activeInstallationIDs {
		active[installationID] = struct{}{}
	}

	for installationID, installation := range f.byInstallationID {
		if installation.UserID != userID || installation.RevokedAt != nil {
			continue
		}
		if _, ok := active[installationID]; ok {
			continue
		}
		installation.RevokedAt = &at
		f.revokedIDs = append(f.revokedIDs, installationID)
	}

	return nil
}

func (f *fakeGitHubInstallationsProvider) ListInstallations(ctx context.Context, githubAccessToken string) ([]GitHubInstallationSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshots, nil
}

func TestGitHubInstallationServiceSyncSuccess(t *testing.T) {
	identityStore := newFakeOAuthIdentityStore(&models.OAuthIdentity{
		ID:              "oid_test",
		UserID:          "usr_test",
		Provider:        GitHubOAuthProviderName,
		ProviderSubject: "github-subject",
		Email:           "jane@example.com",
	})
	installStore := newFakeGitHubInstallationStore()
	provider := &fakeGitHubInstallationsProvider{
		snapshots: []GitHubInstallationSnapshot{
			{
				GitHubInstallationID: 123,
				AccountLogin:         "lazyops",
				AccountType:          "Organization",
				InstalledAt:          time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC),
				Scope: GitHubInstallationScope{
					RepositorySelection: "selected",
					Permissions: map[string]string{
						"contents": "read",
					},
					Repositories: []GitHubInstallationRepositoryScope{
						{
							ID:         1,
							Name:       "backend",
							FullName:   "lazyops/backend",
							OwnerLogin: "lazyops",
							Private:    true,
						},
					},
				},
			},
		},
	}
	service := NewGitHubInstallationService(identityStore, installStore, provider)

	result, err := service.SyncInstallations(context.Background(), SyncGitHubInstallationsCommand{
		UserID:            "usr_test",
		GitHubAccessToken: "github-user-token",
	})
	if err != nil {
		t.Fatalf("sync installations: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one installation, got %d", len(result.Items))
	}
	if result.Items[0].Status != "active" {
		t.Fatalf("expected active status, got %q", result.Items[0].Status)
	}
	if len(result.Items[0].Scope.Repositories) != 1 || result.Items[0].Scope.Repositories[0].FullName != "lazyops/backend" {
		t.Fatalf("expected repository scope to be persisted, got %+v", result.Items[0].Scope.Repositories)
	}
}

func TestGitHubInstallationServiceMissingGitHubIdentity(t *testing.T) {
	identityStore := newFakeOAuthIdentityStore()
	installStore := newFakeGitHubInstallationStore()
	provider := &fakeGitHubInstallationsProvider{}
	service := NewGitHubInstallationService(identityStore, installStore, provider)

	_, err := service.SyncInstallations(context.Background(), SyncGitHubInstallationsCommand{
		UserID:            "usr_test",
		GitHubAccessToken: "github-user-token",
	})
	if !errors.Is(err, ErrGitHubIdentityRequired) {
		t.Fatalf("expected ErrGitHubIdentityRequired, got %v", err)
	}
}

func TestGitHubInstallationServiceRevokesMissingInstallation(t *testing.T) {
	identityStore := newFakeOAuthIdentityStore(&models.OAuthIdentity{
		ID:              "oid_test",
		UserID:          "usr_test",
		Provider:        GitHubOAuthProviderName,
		ProviderSubject: "github-subject",
		Email:           "jane@example.com",
	})
	existingInstalledAt := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	installStore := newFakeGitHubInstallationStore(
		&models.GitHubInstallation{
			ID:                   "ghi_old",
			UserID:               "usr_test",
			GitHubInstallationID: 100,
			AccountLogin:         "old-org",
			AccountType:          "Organization",
			ScopeJSON:            "{}",
			InstalledAt:          existingInstalledAt,
		},
		&models.GitHubInstallation{
			ID:                   "ghi_new",
			UserID:               "usr_test",
			GitHubInstallationID: 200,
			AccountLogin:         "new-org",
			AccountType:          "Organization",
			ScopeJSON:            "{}",
			InstalledAt:          existingInstalledAt,
		},
	)
	provider := &fakeGitHubInstallationsProvider{
		snapshots: []GitHubInstallationSnapshot{
			{
				GitHubInstallationID: 200,
				AccountLogin:         "new-org",
				AccountType:          "Organization",
				InstalledAt:          time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC),
				Scope:                GitHubInstallationScope{RepositorySelection: "all", Permissions: map[string]string{}, Repositories: nil},
			},
		},
	}
	service := NewGitHubInstallationService(identityStore, installStore, provider)

	result, err := service.SyncInstallations(context.Background(), SyncGitHubInstallationsCommand{
		UserID:            "usr_test",
		GitHubAccessToken: "github-user-token",
	})
	if err != nil {
		t.Fatalf("sync installations: %v", err)
	}

	oldInstallation := installStore.byInstallationID[100]
	if oldInstallation.RevokedAt == nil {
		t.Fatal("expected missing installation to be marked revoked")
	}

	var revokedSeen bool
	for _, item := range result.Items {
		if item.GitHubInstallationID == 100 {
			revokedSeen = true
			if item.Status != "revoked" {
				t.Fatalf("expected revoked status, got %q", item.Status)
			}
		}
	}
	if !revokedSeen {
		t.Fatal("expected revoked installation to be returned in sync result")
	}
}

func TestGitHubInstallationServiceProviderError(t *testing.T) {
	identityStore := newFakeOAuthIdentityStore(&models.OAuthIdentity{
		ID:              "oid_test",
		UserID:          "usr_test",
		Provider:        GitHubOAuthProviderName,
		ProviderSubject: "github-subject",
		Email:           "jane@example.com",
	})
	installStore := newFakeGitHubInstallationStore()
	provider := &fakeGitHubInstallationsProvider{
		err: errors.New("github api unavailable"),
	}
	service := NewGitHubInstallationService(identityStore, installStore, provider)

	_, err := service.SyncInstallations(context.Background(), SyncGitHubInstallationsCommand{
		UserID:            "usr_test",
		GitHubAccessToken: "github-user-token",
	})
	if !errors.Is(err, ErrGitHubProviderError) {
		t.Fatalf("expected ErrGitHubProviderError, got %v", err)
	}
}
