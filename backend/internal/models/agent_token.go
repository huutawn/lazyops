package models

import "time"

type AgentToken struct {
	ID          string     `json:"id" gorm:"primaryKey;size:64"`
	UserID      string     `json:"user_id" gorm:"size:64;not null;index"`
	InstanceID  string     `json:"instance_id" gorm:"size:64;not null;index"`
	AgentID     string     `json:"agent_id" gorm:"size:64;not null;index"`
	TokenHash   string     `json:"token_hash" gorm:"size:128;not null;uniqueIndex"`
	TokenPrefix string     `json:"token_prefix" gorm:"size:32;not null"`
	LastUsedAt  *time.Time `json:"last_used_at" gorm:"index"`
	ExpiresAt   *time.Time `json:"expires_at" gorm:"index"`
	RevokedAt   *time.Time `json:"revoked_at" gorm:"index"`
	CreatedAt   time.Time  `json:"created_at"`
}
