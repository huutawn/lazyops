package models

import "time"

type User struct {
	ID           string     `json:"id" gorm:"primaryKey;size:64"`
	DisplayName  string     `json:"display_name" gorm:"column:display_name;size:255;not null"`
	Email        string     `json:"email" gorm:"size:255;uniqueIndex;not null"`
	PasswordHash string     `json:"-" gorm:"size:255"`
	Role         string     `json:"role" gorm:"size:32;not null;default:'viewer'"`
	Status       string     `json:"status" gorm:"size:32;not null;default:'active'"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
