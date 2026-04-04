package models

import "time"

type RuntimeIncident struct {
	ID           string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string     `json:"project_id" gorm:"size:64;not null;index"`
	DeploymentID string     `json:"deployment_id" gorm:"size:64;not null;index"`
	RevisionID   string     `json:"revision_id" gorm:"size:64;not null;index"`
	Kind         string     `json:"kind" gorm:"size:128;not null;index"`
	Severity     string     `json:"severity" gorm:"size:64;not null"`
	Status       string     `json:"status" gorm:"size:64;not null;default:'open';index"`
	Summary      string     `json:"summary" gorm:"size:1024;not null"`
	DetailsJSON  string     `json:"details_json" gorm:"type:jsonb;not null"`
	TriggeredBy  string     `json:"triggered_by" gorm:"size:255"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
