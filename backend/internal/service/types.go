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

type CreateInstanceCommand struct {
	UserID    string
	Name      string
	PublicIP  string
	PrivateIP string
	Labels    map[string]string
}

type BootstrapTokenIssue struct {
	Token     string
	TokenID   string
	ExpiresAt time.Time
	SingleUse bool
}

type AgentMachineInfo struct {
	Hostname string
	OS       string
	Arch     string
	Kernel   string
	IPs      []string
	Labels   map[string]string
}

type AgentEnrollmentCommand struct {
	BootstrapToken string
	RuntimeMode    string
	AgentKind      string
	Machine        AgentMachineInfo
	Capabilities   map[string]any
}

type AgentEnrollmentResult struct {
	AgentID    string
	AgentToken string
	InstanceID string
	IssuedAt   time.Time
	ExpiresAt  *time.Time
}

type AgentHeartbeatCommand struct {
	UserID           string
	AgentID          string
	InstanceID       string
	SessionID        string
	State            string
	HealthStatus     string
	HealthSummary    string
	RuntimeMode      string
	AgentKind        string
	SentAt           time.Time
	UptimeSeconds    int64
	CapabilityHash   string
	CapabilityUpdate map[string]any
	Capabilities     map[string]any
}

type AgentHeartbeatResult struct {
	AgentID        string
	InstanceID     string
	AgentStatus    string
	InstanceStatus string
	ReceivedAt     time.Time
}

type InstanceSummary struct {
	ID                  string
	TargetKind          string
	Name                string
	PublicIP            *string
	PrivateIP           *string
	AgentID             *string
	Status              string
	Labels              map[string]string
	RuntimeCapabilities map[string]any
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateInstanceResult struct {
	Instance  InstanceSummary
	Bootstrap BootstrapTokenIssue
}

type InstanceListResult struct {
	Items []InstanceSummary
}

type CreateMeshNetworkCommand struct {
	UserID   string
	Name     string
	Provider string
	CIDR     string
}

type MeshNetworkSummary struct {
	ID         string
	TargetKind string
	Name       string
	Provider   string
	CIDR       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type MeshNetworkListResult struct {
	Items []MeshNetworkSummary
}

type CreateClusterCommand struct {
	UserID              string
	Name                string
	Provider            string
	KubeconfigSecretRef string
}

type ClusterSummary struct {
	ID         string
	TargetKind string
	Name       string
	Provider   string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ClusterListResult struct {
	Items []ClusterSummary
}

type CreateDeploymentBindingCommand struct {
	RequesterUserID     string
	RequesterRole       string
	ProjectID           string
	Name                string
	TargetRef           string
	RuntimeMode         string
	TargetKind          string
	TargetID            string
	PlacementPolicy     map[string]any
	DomainPolicy        map[string]any
	CompatibilityPolicy map[string]any
	ScaleToZeroPolicy   map[string]any
}

type DeploymentBindingRecord struct {
	ID                  string
	ProjectID           string
	Name                string
	TargetRef           string
	RuntimeMode         string
	TargetKind          string
	TargetID            string
	PlacementPolicy     map[string]any
	DomainPolicy        map[string]any
	CompatibilityPolicy map[string]any
	ScaleToZeroPolicy   map[string]any
	CreatedAt           time.Time
	UpdatedAt           time.Time
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
