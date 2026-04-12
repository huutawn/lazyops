package models

import "time"

type BuildJob struct {
	ID                   string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID            string     `json:"project_id" gorm:"size:64;not null;index"`
	ProjectRepoLinkID    string     `json:"project_repo_link_id" gorm:"size:64;not null;index"`
	GitHubDeliveryID     string     `json:"github_delivery_id" gorm:"column:github_delivery_id;size:255;not null;index"`
	GitHubInstallationID int64      `json:"github_installation_id" gorm:"column:github_installation_id;not null;index"`
	GitHubRepoID         int64      `json:"github_repo_id" gorm:"column:github_repo_id;not null;index"`
	RepoFullName         string     `json:"repo_full_name" gorm:"size:512;not null"`
	TriggerKind          string     `json:"trigger_kind" gorm:"size:128;not null;index"`
	Status               string     `json:"status" gorm:"size:64;not null;default:'queued';index"`
	CommitSHA            string     `json:"commit_sha" gorm:"size:255;not null;index"`
	TrackedBranch        string     `json:"tracked_branch" gorm:"size:255;not null;index"`
	PullRequestNumber    int        `json:"pull_request_number" gorm:"not null;default:0"`
	RetryCount           int        `json:"retry_count" gorm:"not null;default:0"`
	MaxAttempts          int        `json:"max_attempts" gorm:"not null;default:3"`
	WorkerInputJSON      string     `json:"worker_input_json" gorm:"type:jsonb;not null"`
	ArtifactMetadataJSON string     `json:"artifact_metadata_json" gorm:"type:jsonb;not null"`
	StartedAt            *time.Time `json:"started_at"`
	CompletedAt          *time.Time `json:"completed_at"`
	CreatedAt            time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
