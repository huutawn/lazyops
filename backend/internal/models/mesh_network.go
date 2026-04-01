package models

import "time"

type MeshNetwork struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	UserID    string    `json:"user_id" gorm:"size:64;not null;index;uniqueIndex:idx_mesh_networks_user_name"`
	Name      string    `json:"name" gorm:"size:255;not null;uniqueIndex:idx_mesh_networks_user_name"`
	Provider  string    `json:"provider" gorm:"size:64;not null"`
	CIDR      string    `json:"cidr" gorm:"type:cidr;not null"`
	Status    string    `json:"status" gorm:"size:64;not null;default:'provisioning'"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
