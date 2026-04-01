package models

import "time"

type Instance struct {
	ID                      string     `json:"id" gorm:"primaryKey;size:64"`
	UserID                  string     `json:"user_id" gorm:"size:64;not null;index;uniqueIndex:idx_instances_user_name"`
	Name                    string     `json:"name" gorm:"size:255;not null;uniqueIndex:idx_instances_user_name"`
	PublicIP                *string    `json:"public_ip" gorm:"type:inet"`
	PrivateIP               *string    `json:"private_ip" gorm:"type:inet"`
	AgentID                 *string    `json:"agent_id" gorm:"size:64;index"`
	Status                  string     `json:"status" gorm:"size:64;not null;default:'pending_enrollment'"`
	LabelsJSON              string     `json:"labels_json" gorm:"type:jsonb;not null;default:'{}'"`
	RuntimeCapabilitiesJSON string     `json:"runtime_capabilities_json" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}
