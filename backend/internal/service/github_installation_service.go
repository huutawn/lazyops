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
	if strings.TrimSpace(cmd.UserID) == "" || strings.TrimSpace(cmd.GitHubAccessToken) == "" {
		return nil, ErrInvalidInput
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

	installations, err := s.installations.ListByUser(cmd.UserID)
	if err != nil {
		return nil, err
	}

	records := make([]GitHubInstallationRecord, 0, len(installations))
	for _, installation := range installations {
		scope := GitHubInstallationScope{}
		if strings.TrimSpace(installation.ScopeJSON) != "" {
			if err := json.Unmarshal([]byte(installation.ScopeJSON), &scope); err != nil {
				return nil, err
			}
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

	return &GitHubInstallationSyncResult{Items: records}, nil
}
