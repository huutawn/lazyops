package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lazyops-server/internal/service"
)

const (
	githubUserInstallationsURL = "https://api.github.com/user/installations?per_page=%d&page=%d"
	githubInstallationReposURL = "https://api.github.com/user/installations/%d/repositories?per_page=%d&page=%d"
	gitHubPageSize             = 100
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
	type githubInstallationPayload struct {
		Installations []struct {
			ID                  int64             `json:"id"`
			Account             githubAccount     `json:"account"`
			RepositorySelection string            `json:"repository_selection"`
			Permissions         map[string]string `json:"permissions"`
			SuspendedAt         *time.Time        `json:"suspended_at"`
		} `json:"installations"`
	}

	snapshots := make([]service.GitHubInstallationSnapshot, 0)
	syncedAt := time.Now().UTC()
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf(githubUserInstallationsURL, gitHubPageSize, page),
			nil,
		)
		if err != nil {
			return nil, err
		}
		p.applyHeaders(req, githubAccessToken)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return nil, githubAPIError("github installations fetch failed", resp)
		}

		var payload githubInstallationPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, err
		}
		hasNextPage := hasGitHubNextPage(resp.Header.Get("Link"))
		resp.Body.Close()

		for _, installation := range payload.Installations {
			repositories, err := p.listRepositories(ctx, githubAccessToken, installation.ID)
			if err != nil {
				return nil, err
			}

			snapshots = append(snapshots, service.GitHubInstallationSnapshot{
				GitHubInstallationID: installation.ID,
				AccountLogin:         strings.TrimSpace(installation.Account.Login),
				AccountType:          strings.TrimSpace(installation.Account.Type),
				InstalledAt:          syncedAt,
				RevokedAt:            installation.SuspendedAt,
				Scope: service.GitHubInstallationScope{
					RepositorySelection: strings.TrimSpace(installation.RepositorySelection),
					Permissions:         installation.Permissions,
					Repositories:        repositories,
				},
			})
		}

		if !hasNextPage {
			break
		}
	}

	return snapshots, nil
}

func (p *AppInstallationsProvider) listRepositories(ctx context.Context, githubAccessToken string, installationID int64) ([]service.GitHubInstallationRepositoryScope, error) {
	type githubRepositoriesPayload struct {
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

	repositories := make([]service.GitHubInstallationRepositoryScope, 0)
	for page := 1; ; page++ {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf(githubInstallationReposURL, installationID, gitHubPageSize, page),
			nil,
		)
		if err != nil {
			return nil, err
		}
		p.applyHeaders(req, githubAccessToken)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return nil, githubAPIError("github installation repositories fetch failed", resp)
		}

		var payload githubRepositoriesPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, err
		}
		hasNextPage := hasGitHubNextPage(resp.Header.Get("Link"))
		resp.Body.Close()

		for _, repository := range payload.Repositories {
			repositories = append(repositories, service.GitHubInstallationRepositoryScope{
				ID:         repository.ID,
				Name:       strings.TrimSpace(repository.Name),
				FullName:   strings.TrimSpace(repository.FullName),
				OwnerLogin: strings.TrimSpace(repository.Owner.Login),
				Private:    repository.Private,
			})
		}

		if !hasNextPage {
			break
		}
	}

	return repositories, nil
}

func (p *AppInstallationsProvider) applyHeaders(req *http.Request, githubAccessToken string) {
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(githubAccessToken))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func hasGitHubNextPage(linkHeader string) bool {
	for _, segment := range strings.Split(linkHeader, ",") {
		entry := strings.TrimSpace(segment)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, `rel="next"`) {
			return true
		}
	}
	return false
}

type githubAccount struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

func githubAPIError(prefix string, resp *http.Response) error {
	if resp == nil {
		return errors.New(prefix)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	_ = resp.Body.Close()
	message := strings.TrimSpace(string(body))
	if message == "" {
		return fmt.Errorf("%s (status=%d)", prefix, resp.StatusCode)
	}
	return fmt.Errorf("%s (status=%d): %s", prefix, resp.StatusCode, message)
}
