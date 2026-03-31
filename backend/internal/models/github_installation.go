package models

import "time"

type GitHubInstallation struct {
	ID                   string     `json:"id" gorm:"primaryKey;size:64"`
	UserID               string     `json:"user_id" gorm:"size:64;not null;uniqueIndex:idx_github_installation_user_installation"`
	GitHubInstallationID int64      `json:"github_installation_id" gorm:"column:github_installation_id;not null;uniqueIndex:idx_github_installation_user_installation"`
	AccountLogin         string     `json:"account_login" gorm:"size:255;index;not null"`
	AccountType          string     `json:"account_type" gorm:"size:64;not null"`
	ScopeJSON            string     `json:"scope_json" gorm:"type:jsonb;not null;default:'{}'"`
	InstalledAt          time.Time  `json:"installed_at"`
	RevokedAt            *time.Time `json:"revoked_at" gorm:"index"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}
