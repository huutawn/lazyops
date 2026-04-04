package models

import "time"

type Blueprint struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string    `json:"project_id" gorm:"size:64;not null;index;index:idx_blueprints_project_source_created,priority:1"`
	SourceKind   string    `json:"source_kind" gorm:"size:64;not null;index:idx_blueprints_project_source_created,priority:2"`
	SourceRef    string    `json:"source_ref" gorm:"size:1024;not null"`
	CompiledJSON string    `json:"compiled_json" gorm:"type:jsonb;not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"index:idx_blueprints_project_source_created,priority:3,sort:desc"`
}
