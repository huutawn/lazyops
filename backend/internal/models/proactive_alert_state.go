package models

import "time"

type ProactiveAlertState struct {
	ID              string     `json:"id" gorm:"primaryKey;size:64"`
	ProjectID       string     `json:"project_id" gorm:"size:64;not null;index:idx_proactive_alert_fingerprint,priority:1"`
	ServiceName     string     `json:"service_name" gorm:"size:255;not null;index:idx_proactive_alert_fingerprint,priority:2"`
	Fingerprint     string     `json:"fingerprint" gorm:"size:255;not null;index:idx_proactive_alert_fingerprint,priority:3"`
	LastIncidentID  string     `json:"last_incident_id" gorm:"size:64;not null;default:''"`
	LastSeverity    string     `json:"last_severity" gorm:"size:64;not null;default:''"`
	Count           int        `json:"count" gorm:"not null;default:0"`
	FirstSeenAt     time.Time  `json:"first_seen_at" gorm:"not null;index"`
	LastSeenAt      time.Time  `json:"last_seen_at" gorm:"not null;index"`
	SuppressedUntil *time.Time `json:"suppressed_until,omitempty" gorm:"index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
