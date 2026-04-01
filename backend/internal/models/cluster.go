package models

import "time"

type Cluster struct {
	ID                  string    `json:"id" gorm:"primaryKey;size:64"`
	UserID              string    `json:"user_id" gorm:"size:64;not null;index;uniqueIndex:idx_clusters_user_name"`
	Name                string    `json:"name" gorm:"size:255;not null;uniqueIndex:idx_clusters_user_name"`
	Provider            string    `json:"provider" gorm:"size:64;not null"`
	KubeconfigSecretRef string    `json:"kubeconfig_secret_ref" gorm:"size:255;not null"`
	Status              string    `json:"status" gorm:"size:64;not null;default:'validating'"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
