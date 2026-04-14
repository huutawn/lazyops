package models

import "time"

type ProjectEnvBundle struct {
	ID                string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID         string    `json:"project_id" gorm:"size:64;not null;uniqueIndex:idx_project_env_bundles_project"`
	EnvEncrypted      string    `json:"env_encrypted" gorm:"type:text;not null"`
	EnvFingerprint    string    `json:"env_fingerprint" gorm:"size:128;not null"`
	KeyNamesJSON      string    `json:"key_names_json" gorm:"type:jsonb;not null;default:'[]'"`
	ParseWarningsJSON string    `json:"parse_warnings_json" gorm:"type:jsonb;not null;default:'[]'"`
	UpdatedBy         string    `json:"updated_by" gorm:"size:64;not null"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
