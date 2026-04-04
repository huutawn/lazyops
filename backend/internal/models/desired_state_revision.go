package models

import "time"

type DesiredStateRevision struct {
	ID                   string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID            string    `json:"project_id" gorm:"size:64;not null;index"`
	BlueprintID          string    `json:"blueprint_id" gorm:"size:64;not null;index"`
	DeploymentBindingID  string    `json:"deployment_binding_id" gorm:"size:64;not null;index"`
	CommitSHA            string    `json:"commit_sha" gorm:"size:255;not null;index"`
	TriggerKind          string    `json:"trigger_kind" gorm:"size:128;not null"`
	Status               string    `json:"status" gorm:"size:64;not null;default:'queued';index"`
	CompiledRevisionJSON string    `json:"compiled_revision_json" gorm:"type:jsonb;not null"`
	CreatedAt            time.Time `json:"created_at" gorm:"index"`
	UpdatedAt            time.Time `json:"updated_at"`
}
