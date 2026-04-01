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
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	FullName   string `json:"full_name"`
	OwnerLogin string `json:"owner_login"`
	Private    bool   `json:"private"`
}

type GitHubInstallationScope struct {
	RepositorySelection string                              `json:"repository_selection"`
	Permissions         map[string]string                   `json:"permissions"`
	Repositories        []GitHubInstallationRepositoryScope `json:"repositories"`
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

type GitHubRepositoryRecord struct {
	GitHubInstallationID     int64
	InstallationAccountLogin string
	InstallationAccountType  string
	GitHubRepoID             int64
	RepoOwner                string
	RepoName                 string
	FullName                 string
	Private                  bool
	Permissions              map[string]string
}

type GitHubRepositoryListResult struct {
	Items []GitHubRepositoryRecord
}

type GitHubWebhookNormalizedEventRecord struct {
	TriggerKind          string
	Action               string
	ProjectID            string
	ProjectRepoLinkID    string
	GitHubInstallationID int64
	GitHubRepoID         int64
	RepoOwner            string
	RepoName             string
	RepoFullName         string
	TrackedBranch        string
	CommitSHA            string
	PullRequestNumber    int
	PreviewEnabled       bool
	ShouldEnqueueBuild   bool
	ShouldDestroyPreview bool
}

type CreateProjectCommand struct {
	UserID        string
	Name          string
	Slug          string
	DefaultBranch string
}

type ProjectSummary struct {
	ID            string
	Name          string
	Slug          string
	DefaultBranch string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateProjectRepoLinkCommand struct {
	RequesterUserID      string
	RequesterRole        string
	ProjectID            string
	GitHubInstallationID int64
	GitHubRepoID         int64
	TrackedBranch        string
	PreviewEnabled       bool
}

type WebhookRouteLookupCommand struct {
	GitHubInstallationID int64
	GitHubRepoID         int64
	TrackedBranch        string
}

type ProjectRepoLinkRecord struct {
	ID                         string
	ProjectID                  string
	GitHubInstallationRecordID string
	GitHubInstallationID       int64
	GitHubRepoID               int64
	RepoOwner                  string
	RepoName                   string
	RepoFullName               string
	TrackedBranch              string
	PreviewEnabled             bool
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
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
