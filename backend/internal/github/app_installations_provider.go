package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lazyops-server/internal/service"
)

const (
	githubUserInstallationsURL = "https://api.github.com/user/installations"
	githubInstallationReposURL = "https://api.github.com/user/installations/%d/repositories?per_page=100"
)

type AppInstallationsProvider struct {
	httpClient *http.Client
}

func NewAppInstallationsProvider(httpClient *http.Client) *AppInstallationsProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &AppInstallationsProvider{httpClient: httpClient}
}

func (p *AppInstallationsProvider) ListInstallations(ctx context.Context, githubAccessToken string) ([]service.GitHubInstallationSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserInstallationsURL, nil)
	if err != nil {
		return nil, err
	}
	p.applyHeaders(req, githubAccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("github installations fetch failed")
	}

	var payload struct {
		Installations []struct {
			ID                  int64             `json:"id"`
			Account             githubAccount     `json:"account"`
			RepositorySelection string            `json:"repository_selection"`
			Permissions         map[string]string `json:"permissions"`
			SuspendedAt         *time.Time        `json:"suspended_at"`
		} `json:"installations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	snapshots := make([]service.GitHubInstallationSnapshot, 0, len(payload.Installations))
	for _, installation := range payload.Installations {
		repositories, err := p.listRepositories(ctx, githubAccessToken, installation.ID)
		if err != nil {
			return nil, err
		}

		snapshots = append(snapshots, service.GitHubInstallationSnapshot{
			GitHubInstallationID: installation.ID,
			AccountLogin:         strings.TrimSpace(installation.Account.Login),
			AccountType:          strings.TrimSpace(installation.Account.Type),
			InstalledAt:          time.Now().UTC(),
			RevokedAt:            installation.SuspendedAt,
			Scope: service.GitHubInstallationScope{
				RepositorySelection: strings.TrimSpace(installation.RepositorySelection),
				Permissions:         installation.Permissions,
				Repositories:        repositories,
			},
		})
	}

	return snapshots, nil
}

func (p *AppInstallationsProvider) listRepositories(ctx context.Context, githubAccessToken string, installationID int64) ([]service.GitHubInstallationRepositoryScope, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(githubInstallationReposURL, installationID), nil)
	if err != nil {
		return nil, err
	}
	p.applyHeaders(req, githubAccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("github installation repositories fetch failed")
	}

	var payload struct {
		Repositories []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Private  bool   `json:"private"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	repositories := make([]service.GitHubInstallationRepositoryScope, 0, len(payload.Repositories))
	for _, repository := range payload.Repositories {
		repositories = append(repositories, service.GitHubInstallationRepositoryScope{
			ID:         repository.ID,
			Name:       strings.TrimSpace(repository.Name),
			FullName:   strings.TrimSpace(repository.FullName),
			OwnerLogin: strings.TrimSpace(repository.Owner.Login),
			Private:    repository.Private,
		})
	}

	return repositories, nil
}

func (p *AppInstallationsProvider) applyHeaders(req *http.Request, githubAccessToken string) {
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(githubAccessToken))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

type githubAccount struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}
