package models

import "time"

type BootstrapToken struct {
	ID                  string     `json:"id" gorm:"primaryKey;size:64"`
	UserID              string     `json:"user_id" gorm:"size:64;not null;index"`
	InstanceID          string     `json:"instance_id" gorm:"size:64;not null;index"`
	TokenHash           string     `json:"token_hash" gorm:"size:128;not null;uniqueIndex"`
	ExpectedRuntimeMode string     `json:"expected_runtime_mode,omitempty" gorm:"size:64"`
	ExpectedAgentKind   string     `json:"expected_agent_kind,omitempty" gorm:"size:64"`
	ExpectedTargetRef   string     `json:"expected_target_ref,omitempty" gorm:"size:255"`
	ExpiresAt           time.Time  `json:"expires_at" gorm:"not null;index"`
	UsedAt              *time.Time `json:"used_at" gorm:"index"`
	CreatedAt           time.Time  `json:"created_at"`
}
