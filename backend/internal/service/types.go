package service

import "time"

type RegisterCommand struct {
	Name     string
	Email    string
	Password string
}

type LoginCommand struct {
	Email    string
	Password string
}

type CLILoginCommand struct {
	AuthFlow   string
	Email      string
	Password   string
	DeviceName string
}

type RevokePATCommand struct {
	UserID  string
	TokenID string
}

type UserProfile struct {
	ID          string
	DisplayName string
	Email       string
	Role        string
	Status      string
	LastLoginAt *time.Time
}

type AuthResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   time.Duration
	User        UserProfile
}

type CLIAuthResult struct {
	Token     string
	TokenType string
	TokenID   string
	ExpiresAt *time.Time
	User      UserProfile
}

type PATRevokeResult struct {
	TokenID string
	Revoked bool
}

type SyncGitHubInstallationsCommand struct {
	UserID            string
	GitHubAccessToken string
}

type GitHubInstallationRepositoryScope struct {
	ID         int64
	Name       string
	FullName   string
	OwnerLogin string
	Private    bool
}

type GitHubInstallationScope struct {
	RepositorySelection string
	Permissions         map[string]string
	Repositories        []GitHubInstallationRepositoryScope
}

type GitHubInstallationRecord struct {
	ID                   string
	GitHubInstallationID int64
	AccountLogin         string
	AccountType          string
	InstalledAt          time.Time
	RevokedAt            *time.Time
	Status               string
	Scope                GitHubInstallationScope
}

type GitHubInstallationSyncResult struct {
	Items []GitHubInstallationRecord
}

type CreateAgentCommand struct {
	UserID  string
	AgentID string
	Name    string
	Status  string
}

type UpdateAgentStatusCommand struct {
	UserID  string
	AgentID string
	Name    string
	Status  string
	Source  string
	At      time.Time
}

type AgentRecord struct {
	ID         string
	UserID     string
	AgentID    string
	Name       string
	Status     string
	LastSeenAt *time.Time
	UpdatedAt  time.Time
}

type RealtimeMeta struct {
	Source string
	At     time.Time
}

type AgentRealtimeEvent struct {
	Type    string
	Payload AgentRecord
	Meta    RealtimeMeta
}
