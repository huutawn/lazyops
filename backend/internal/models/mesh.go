package models

import "time"

type TunnelSession struct {
	ID          string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID   string    `json:"project_id" gorm:"size:64;not null;index"`
	TargetKind  string    `json:"target_kind" gorm:"size:64;not null"`
	TargetID    string    `json:"target_id" gorm:"size:64;not null;index"`
	InstanceID  string    `json:"instance_id" gorm:"size:64;not null;index"`
	SessionType string    `json:"session_type" gorm:"size:64;not null"`
	LocalPort   int       `json:"local_port" gorm:"not null"`
	RemotePort  int       `json:"remote_port" gorm:"not null"`
	Status      string    `json:"status" gorm:"size:64;not null;default:'active';index"`
	Token       string    `json:"token" gorm:"size:255;not null"`
	ExpiresAt   time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TopologyState struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	InstanceID   string    `json:"instance_id" gorm:"size:64;not null;uniqueIndex"`
	MeshID       string    `json:"mesh_id" gorm:"size:64;not null;index"`
	State        string    `json:"state" gorm:"size:64;not null;index"`
	MetadataJSON string    `json:"metadata_json" gorm:"type:jsonb;not null;default:'{}'"`
	LastSeenAt   time.Time `json:"last_seen_at" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
	UpdatedAt    time.Time `json:"updated_at"`
}
