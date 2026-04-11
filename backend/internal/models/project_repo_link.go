package models

import "time"

type ProjectRepoLink struct {
	ID                   string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID            string    `json:"project_id" gorm:"size:64;not null;uniqueIndex"`
	GitHubInstallationID string    `json:"github_installation_id" gorm:"column:github_installation_id;size:64;not null;index:idx_project_repo_links_installation_repo;uniqueIndex:idx_project_repo_links_route,priority:1"`
	GitHubRepoID         int64     `json:"github_repo_id" gorm:"column:github_repo_id;not null;index:idx_project_repo_links_installation_repo;uniqueIndex:idx_project_repo_links_route,priority:2"`
	RepoOwner            string    `json:"repo_owner" gorm:"size:255;not null"`
	RepoName             string    `json:"repo_name" gorm:"size:255;not null"`
	TrackedBranch        string    `json:"tracked_branch" gorm:"size:255;not null;uniqueIndex:idx_project_repo_links_route,priority:3"`
	PreviewEnabled       bool      `json:"preview_enabled" gorm:"not null;default:false"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
