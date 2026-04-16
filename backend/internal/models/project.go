package models

import "time"

type Project struct {
	ID            string    `json:"id" gorm:"primaryKey;size:64"`
	UserID        string    `json:"user_id" gorm:"size:64;not null;index;uniqueIndex:idx_projects_user_slug"`
	Name          string    `json:"name" gorm:"size:255;not null"`
	Slug          string    `json:"slug" gorm:"size:255;not null;uniqueIndex:idx_projects_user_slug"`
	NamespaceSlug string    `json:"namespace_slug" gorm:"size:63;not null;default:'';index"`
	ClusterID     *string   `json:"cluster_id,omitempty" gorm:"size:64;index"`
	RuntimeMode   string    `json:"runtime_mode" gorm:"size:64;not null;default:'distributed-k3s';index"`
	DefaultBranch string    `json:"default_branch" gorm:"size:255;not null;default:'main'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
