package response

import "time"

type ProjectSummaryResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ProjectListResponse struct {
	Items []ProjectSummaryResponse `json:"items"`
}

type ProjectRepoLinkResponse struct {
	ID                   string    `json:"id"`
	ProjectID            string    `json:"project_id"`
	GitHubInstallationID int64     `json:"github_installation_id"`
	GitHubRepoID         int64     `json:"github_repo_id"`
	RepoOwner            string    `json:"repo_owner"`
	RepoName             string    `json:"repo_name"`
	RepoFullName         string    `json:"repo_full_name"`
	TrackedBranch        string    `json:"tracked_branch"`
	PreviewEnabled       bool      `json:"preview_enabled"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
