package models

import "time"

type Deployment struct {
	ID          string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID   string     `json:"project_id" gorm:"size:64;not null;index"`
	RevisionID  string     `json:"revision_id" gorm:"size:64;not null;index"`
	Status      string     `json:"status" gorm:"size:64;not null;default:'queued';index"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
