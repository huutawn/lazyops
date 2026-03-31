package models

import "time"

type OAuthIdentity struct {
	ID              string     `json:"id" gorm:"primaryKey;size:64"`
	UserID          string     `json:"user_id" gorm:"size:64;index;not null"`
	Provider        string     `json:"provider" gorm:"size:32;not null;uniqueIndex:idx_oauth_identity_provider_subject"`
	ProviderSubject string     `json:"provider_subject" gorm:"size:255;not null;uniqueIndex:idx_oauth_identity_provider_subject"`
	Email           string     `json:"email" gorm:"size:255"`
	AvatarURL       string     `json:"avatar_url" gorm:"size:1024"`
	RevokedAt       *time.Time `json:"revoked_at" gorm:"index"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
