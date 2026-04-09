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

type GitHubRepositoryResponse struct {
	GitHubInstallationID     int64             `json:"github_installation_id"`
	InstallationAccountLogin string            `json:"installation_account_login"`
	InstallationAccountType  string            `json:"installation_account_type"`
	GitHubRepoID             int64             `json:"github_repo_id"`
	RepoOwner                string            `json:"repo_owner"`
	RepoName                 string            `json:"repo_name"`
	FullName                 string            `json:"full_name"`
	Private                  bool              `json:"private"`
	Permissions              map[string]string `json:"permissions"`
}

type GitHubRepositoryListResponse struct {
	Items []GitHubRepositoryResponse `json:"items"`
}

type GitHubAppConfigResponse struct {
	Name        string `json:"name"`
	InstallURL  string `json:"install_url"`
	WebhookURL  string `json:"webhook_url"`
	CallbackURL string `json:"callback_url"`
	Enabled     bool   `json:"enabled"`
}

type GitHubWebhookNormalizedEventResponse struct {
	TriggerKind          string `json:"trigger_kind"`
	Action               string `json:"action,omitempty"`
	ProjectID            string `json:"project_id,omitempty"`
	ProjectRepoLinkID    string `json:"project_repo_link_id,omitempty"`
	GitHubInstallationID int64  `json:"github_installation_id"`
	GitHubRepoID         int64  `json:"github_repo_id"`
	RepoOwner            string `json:"repo_owner,omitempty"`
	RepoName             string `json:"repo_name,omitempty"`
	RepoFullName         string `json:"repo_full_name,omitempty"`
	TrackedBranch        string `json:"tracked_branch,omitempty"`
	CommitSHA            string `json:"commit_sha,omitempty"`
	PullRequestNumber    int    `json:"pull_request_number,omitempty"`
	PreviewEnabled       bool   `json:"preview_enabled"`
	ShouldEnqueueBuild   bool   `json:"should_enqueue_build"`
	ShouldDestroyPreview bool   `json:"should_destroy_preview"`
}

type GitHubWebhookResponse struct {
	DeliveryID    string                               `json:"delivery_id"`
	EventType     string                               `json:"event_type"`
	Status        string                               `json:"status"`
	IgnoredReason string                               `json:"ignored_reason,omitempty"`
	Event         GitHubWebhookNormalizedEventResponse `json:"event"`
}
