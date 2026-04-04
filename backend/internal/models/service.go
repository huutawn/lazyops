package models

import "time"

type Service struct {
	ID              string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID       string    `json:"project_id" gorm:"size:64;not null;index;uniqueIndex:idx_services_project_name"`
	Name            string    `json:"name" gorm:"size:255;not null;uniqueIndex:idx_services_project_name"`
	Path            string    `json:"path" gorm:"size:1024;not null"`
	Public          bool      `json:"public" gorm:"not null;default:false"`
	RuntimeProfile  *string   `json:"runtime_profile,omitempty" gorm:"size:128"`
	HealthcheckJSON string    `json:"healthcheck_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
