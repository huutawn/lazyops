package models

import "time"

type PersonalAccessToken struct {
	ID          string     `json:"id" gorm:"primaryKey;size:64"`
	UserID      string     `json:"user_id" gorm:"size:64;index;not null"`
	Name        string     `json:"name" gorm:"size:255;not null"`
	TokenHash   string     `json:"-" gorm:"size:128;uniqueIndex;not null"`
	TokenPrefix string     `json:"token_prefix" gorm:"size:32;not null"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at" gorm:"index"`
	RevokedAt   *time.Time `json:"revoked_at" gorm:"index"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
