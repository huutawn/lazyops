package response

import "time"

type GitHubInstallationRepositoryScopeResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	FullName   string `json:"full_name"`
	OwnerLogin string `json:"owner_login"`
	Private    bool   `json:"private"`
}

type GitHubInstallationScopeResponse struct {
	RepositorySelection string                                      `json:"repository_selection"`
	Permissions         map[string]string                           `json:"permissions"`
	Repositories        []GitHubInstallationRepositoryScopeResponse `json:"repositories"`
}

type GitHubInstallationResponse struct {
	ID                   string                          `json:"id"`
	GitHubInstallationID int64                           `json:"github_installation_id"`
	AccountLogin         string                          `json:"account_login"`
	AccountType          string                          `json:"account_type"`
	InstalledAt          time.Time                       `json:"installed_at"`
	RevokedAt            *time.Time                      `json:"revoked_at,omitempty"`
	Status               string                          `json:"status"`
	Scope                GitHubInstallationScopeResponse `json:"scope"`
}

type GitHubInstallationSyncResponse struct {
	Items []GitHubInstallationResponse `json:"items"`
}
