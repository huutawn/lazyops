package models

import "time"

type ProjectInternalService struct {
	ID            string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID     string    `json:"project_id" gorm:"size:64;not null;index;uniqueIndex:idx_project_internal_services_project_kind"`
	Kind          string    `json:"kind" gorm:"size:64;not null;uniqueIndex:idx_project_internal_services_project_kind"`
	Alias         string    `json:"alias" gorm:"size:128;not null"`
	Protocol      string    `json:"protocol" gorm:"size:16;not null;default:tcp"`
	Port          int       `json:"port" gorm:"not null"`
	LocalEndpoint string    `json:"local_endpoint" gorm:"size:255;not null"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
