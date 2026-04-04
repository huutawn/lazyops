package models

import "time"

type ScaleToZeroState struct {
	ID                string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID         string     `json:"project_id" gorm:"size:64;not null;index;uniqueIndex:idx_scale_to_zero_project_service"`
	ServiceName       string     `json:"service_name" gorm:"size:255;not null;uniqueIndex:idx_scale_to_zero_project_service"`
	State             string     `json:"state" gorm:"size:64;not null;default:'active'"`
	Enabled           bool       `json:"enabled" gorm:"not null;default:false"`
	WakeTimeoutMs     int        `json:"wake_timeout_ms" gorm:"not null;default:30000"`
	LastStateChangeAt time.Time  `json:"last_state_change_at"`
	WakeTimeoutAt     *time.Time `json:"wake_timeout_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
