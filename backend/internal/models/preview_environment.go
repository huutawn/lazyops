package models

import "time"

type PreviewEnvironment struct {
	ID                string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID         string     `json:"project_id" gorm:"size:64;not null;index"`
	ProjectRepoLinkID string     `json:"project_repo_link_id" gorm:"size:64;not null;index"`
	PRNumber          int        `json:"pr_number" gorm:"not null;index"`
	PRTitle           string     `json:"pr_title" gorm:"size:512"`
	PRAuthor          string     `json:"pr_author" gorm:"size:255"`
	CommitSHA         string     `json:"commit_sha" gorm:"size:255;not null"`
	Branch            string     `json:"branch" gorm:"size:255;not null"`
	Status            string     `json:"status" gorm:"size:64;not null;default:'provisioning';index"`
	DomainJSON        string     `json:"domain_json" gorm:"type:jsonb;not null;default:'[]'"`
	RevisionID        string     `json:"revision_id" gorm:"size:64;index"`
	DeploymentID      string     `json:"deployment_id" gorm:"size:64;index"`
	DestroyReason     string     `json:"destroy_reason" gorm:"size:512"`
	DestroyedAt       *time.Time `json:"destroyed_at"`
	CreatedAt         time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
