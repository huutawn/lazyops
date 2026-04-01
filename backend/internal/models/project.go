package models

import "time"

type Project struct {
	ID            string    `json:"id" gorm:"primaryKey;size:64"`
	UserID        string    `json:"user_id" gorm:"size:64;not null;index;uniqueIndex:idx_projects_user_slug"`
	Name          string    `json:"name" gorm:"size:255;not null"`
	Slug          string    `json:"slug" gorm:"size:255;not null;uniqueIndex:idx_projects_user_slug"`
	DefaultBranch string    `json:"default_branch" gorm:"size:255;not null;default:'main'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
