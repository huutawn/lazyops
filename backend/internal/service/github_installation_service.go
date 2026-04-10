package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var (
	ErrGitHubIdentityRequired = errors.New("github identity required")
	ErrGitHubProviderError    = errors.New("github provider error")
)

type GitHubInstallationSnapshot struct {
	GitHubInstallationID int64
	AccountLogin         string
	AccountType          string
	InstalledAt          time.Time
	RevokedAt            *time.Time
	Scope                GitHubInstallationScope
}

type GitHubInstallationsProvider interface {
	ListInstallations(ctx context.Context, githubAccessToken string) ([]GitHubInstallationSnapshot, error)
}

type GitHubInstallationService struct {
	identities    OAuthIdentityStore
	installations GitHubInstallationStore
	provider      GitHubInstallationsProvider
}

func NewGitHubInstallationService(
	identities OAuthIdentityStore,
	installations GitHubInstallationStore,
	provider GitHubInstallationsProvider,
) *GitHubInstallationService {
	return &GitHubInstallationService{
		identities:    identities,
		installations: installations,
		provider:      provider,
	}
}

func (s *GitHubInstallationService) SyncInstallations(ctx context.Context, cmd SyncGitHubInstallationsCommand) (*GitHubInstallationSyncResult, error) {
	if strings.TrimSpace(cmd.UserID) == "" {
		return nil, ErrInvalidInput
	}

	// CLI link flow can request a cached installation snapshot without passing
	// a fresh GitHub user token on every command execution.
	if strings.TrimSpace(cmd.GitHubAccessToken) == "" {
		records, err := s.listInstallationRecords(cmd.UserID)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			identity, identityErr := s.identities.GetByUserProvider(cmd.UserID, GitHubOAuthProviderName)
			if identityErr != nil {
				return nil, identityErr
			}
			if identity == nil || identity.RevokedAt != nil {
				return nil, ErrGitHubIdentityRequired
			}
		}
		return &GitHubInstallationSyncResult{Items: records}, nil
	}

	identity, err := s.identities.GetByUserProvider(cmd.UserID, GitHubOAuthProviderName)
	if err != nil {
		return nil, err
	}
	if identity == nil || identity.RevokedAt != nil {
		return nil, ErrGitHubIdentityRequired
	}

	snapshots, err := s.provider.ListInstallations(ctx, cmd.GitHubAccessToken)
	if err != nil {
		return nil, ErrGitHubProviderError
	}

	activeInstallationIDs := make([]int64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		scopeJSON, err := json.Marshal(snapshot.Scope)
		if err != nil {
			return nil, err
		}

		activeInstallationIDs = append(activeInstallationIDs, snapshot.GitHubInstallationID)
		installation := &models.GitHubInstallation{
			ID:                   utils.NewPrefixedID("ghi"),
			UserID:               cmd.UserID,
			GitHubInstallationID: snapshot.GitHubInstallationID,
			AccountLogin:         snapshot.AccountLogin,
			AccountType:          snapshot.AccountType,
			ScopeJSON:            string(scopeJSON),
			InstalledAt:          snapshot.InstalledAt.UTC(),
			RevokedAt:            snapshot.RevokedAt,
		}

		existing, err := s.installations.GetByInstallationIDForUser(cmd.UserID, snapshot.GitHubInstallationID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			installation.ID = existing.ID
		}

		if err := s.installations.Upsert(installation); err != nil {
			return nil, err
		}
	}

	if err := s.installations.RevokeMissing(cmd.UserID, activeInstallationIDs, time.Now().UTC()); err != nil {
		return nil, err
	}

	records, err := s.listInstallationRecords(cmd.UserID)
	if err != nil {
		return nil, err
	}
	return &GitHubInstallationSyncResult{Items: records}, nil
}

func (s *GitHubInstallationService) ListRepos(userID string) (*GitHubRepositoryListResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}

	records, err := s.listInstallationRecords(userID)
	if err != nil {
		return nil, err
	}

	items := make([]GitHubRepositoryRecord, 0)
	for _, installation := range records {
		if installation.Status != "active" {
			continue
		}

		for _, repository := range installation.Scope.Repositories {
			items = append(items, GitHubRepositoryRecord{
				GitHubInstallationID:     installation.GitHubInstallationID,
				InstallationAccountLogin: installation.AccountLogin,
				InstallationAccountType:  installation.AccountType,
				GitHubRepoID:             repository.ID,
				RepoOwner:                repository.OwnerLogin,
				RepoName:                 repository.Name,
				FullName:                 repository.FullName,
				Private:                  repository.Private,
				Permissions:              clonePermissions(installation.Scope.Permissions),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].FullName != items[j].FullName {
			return items[i].FullName < items[j].FullName
		}
		if items[i].GitHubInstallationID != items[j].GitHubInstallationID {
			return items[i].GitHubInstallationID < items[j].GitHubInstallationID
		}
		return items[i].GitHubRepoID < items[j].GitHubRepoID
	})

	return &GitHubRepositoryListResult{Items: items}, nil
}

func (s *GitHubInstallationService) listInstallationRecords(userID string) ([]GitHubInstallationRecord, error) {
	installations, err := s.installations.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	records := make([]GitHubInstallationRecord, 0, len(installations))
	for _, installation := range installations {
		scope, err := parseGitHubInstallationScope(installation.ScopeJSON)
		if err != nil {
			return nil, err
		}

		status := "active"
		if installation.RevokedAt != nil {
			status = "revoked"
		}

		records = append(records, GitHubInstallationRecord{
			ID:                   installation.ID,
			GitHubInstallationID: installation.GitHubInstallationID,
			AccountLogin:         installation.AccountLogin,
			AccountType:          installation.AccountType,
			InstalledAt:          installation.InstalledAt,
			RevokedAt:            installation.RevokedAt,
			Status:               status,
			Scope:                scope,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		if (records[i].RevokedAt == nil) != (records[j].RevokedAt == nil) {
			return records[i].RevokedAt == nil
		}
		if records[i].AccountLogin != records[j].AccountLogin {
			return records[i].AccountLogin < records[j].AccountLogin
		}
		return records[i].GitHubInstallationID < records[j].GitHubInstallationID
	})

	return records, nil
}

func parseGitHubInstallationScope(scopeJSON string) (GitHubInstallationScope, error) {
	scope := GitHubInstallationScope{}
	if strings.TrimSpace(scopeJSON) == "" {
		return scope, nil
	}
	if err := json.Unmarshal([]byte(scopeJSON), &scope); err != nil {
		return GitHubInstallationScope{}, err
	}

	return scope, nil
}

func clonePermissions(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}

	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}
