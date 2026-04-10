package request

type CreateProjectRequest struct {
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	DefaultBranch    string   `json:"default_branch"`
	InternalServices []string `json:"internal_services,omitempty"`
}

type LinkProjectRepoRequest struct {
	GitHubInstallationID int64  `json:"github_installation_id"`
	GitHubRepoID         int64  `json:"github_repo_id"`
	TrackedBranch        string `json:"tracked_branch"`
	PreviewEnabled       bool   `json:"preview_enabled"`
}
