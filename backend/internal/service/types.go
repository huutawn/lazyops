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

type InstallInstanceAgentSSHCommand struct {
	UserID                  string
	InstanceID              string
	Host                    string
	Port                    int
	Username                string
	Password                string
	PrivateKey              string
	HostKeyFingerprint      string
	ControlPlaneURL         string
	RuntimeMode             string
	AgentKind               string
	AgentImage              string
	ContainerName           string
	StateDir                string
	ContainerRuntimeRootDir string
}

type InstallInstanceAgentSSHResult struct {
	InstanceID         string
	Bootstrap          BootstrapTokenIssue
	StartedAt          time.Time
	HostKeyFingerprint string
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

type DeploymentBindingListResult struct {
	Items []DeploymentBindingRecord
}

type ValidateLazyopsYAMLCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	RawDocument     []byte
}

type LazyopsYAMLDocument struct {
	ProjectSlug         string                          `json:"project_slug"`
	RuntimeMode         string                          `json:"runtime_mode"`
	DeploymentBinding   LazyopsYAMLDeploymentBindingRef `json:"deployment_binding"`
	Services            []LazyopsYAMLService            `json:"services"`
	DependencyBindings  []LazyopsYAMLDependencyBinding  `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy LazyopsYAMLCompatibilityPolicy  `json:"compatibility_policy"`
	MagicDomainPolicy   LazyopsYAMLMagicDomainPolicy    `json:"magic_domain_policy,omitempty"`
	PreviewPolicy       LazyopsYAMLPreviewPolicy        `json:"preview_policy,omitempty"`
	ScaleToZeroPolicy   LazyopsYAMLScaleToZeroPolicy    `json:"scale_to_zero_policy,omitempty"`
}

type LazyopsYAMLDeploymentBindingRef struct {
	TargetRef string `json:"target_ref"`
}

type LazyopsYAMLService struct {
	Name        string                        `json:"name"`
	Path        string                        `json:"path"`
	StartHint   string                        `json:"start_hint,omitempty"`
	Public      bool                          `json:"public,omitempty"`
	Healthcheck LazyopsYAMLServiceHealthcheck `json:"healthcheck,omitempty"`
}

type LazyopsYAMLServiceHealthcheck struct {
	Path string `json:"path,omitempty"`
	Port int    `json:"port,omitempty"`
}

type LazyopsYAMLDependencyBinding struct {
	Service       string `json:"service"`
	Alias         string `json:"alias"`
	TargetService string `json:"target_service"`
	Protocol      string `json:"protocol"`
	LocalEndpoint string `json:"local_endpoint,omitempty"`
}

type LazyopsYAMLCompatibilityPolicy struct {
	EnvInjection       bool `json:"env_injection"`
	ManagedCredentials bool `json:"managed_credentials"`
	LocalhostRescue    bool `json:"localhost_rescue"`
}

type LazyopsYAMLMagicDomainPolicy struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type LazyopsYAMLPreviewPolicy struct {
	Enabled bool `json:"enabled,omitempty"`
}

type LazyopsYAMLScaleToZeroPolicy struct {
	Enabled            bool   `json:"enabled"`
	IdleWindow         string `json:"idle_window,omitempty"`
	GatewayHoldTimeout string `json:"gateway_hold_timeout,omitempty"`
}

type InitTargetSummary struct {
	ID          string
	Name        string
	Kind        string
	Status      string
	RuntimeMode string
}

type LazyopsYAMLSchemaSummary struct {
	AllowedDependencyProtocols  []string
	AllowedMagicDomainProviders []string
	ForbiddenFieldNames         []string
}

type ValidateLazyopsYAMLResult struct {
	Project           ProjectSummary
	DeploymentBinding DeploymentBindingRecord
	TargetSummary     InitTargetSummary
	Schema            LazyopsYAMLSchemaSummary
}

type BlueprintArtifactMetadata struct {
	CommitSHA   string
	ArtifactRef string
	ImageRef    string
}

type CompileBlueprintCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	SourceRef       string
	TriggerKind     string
	Artifact        BlueprintArtifactMetadata
	LazyopsYAMLRaw  []byte
}

type ProjectServiceRecord struct {
	ID             string
	ProjectID      string
	Name           string
	Path           string
	Public         bool
	RuntimeProfile string
	Healthcheck    map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type BlueprintRepoStateRecord struct {
	ProjectRepoLinkID string
	RepoOwner         string
	RepoName          string
	RepoFullName      string
	TrackedBranch     string
	PreviewEnabled    bool
}

type BlueprintServiceContractRecord struct {
	Name           string
	Path           string
	Public         bool
	RuntimeProfile string
	StartHint      string
	Healthcheck    map[string]any
}

type PlacementAssignmentRecord struct {
	ServiceName string
	TargetID    string
	TargetKind  string
	Labels      map[string]string
}

type BlueprintCompiledContractRecord struct {
	ProjectID           string
	RuntimeMode         string
	Repo                BlueprintRepoStateRecord
	Binding             DeploymentBindingRecord
	Services            []BlueprintServiceContractRecord
	DependencyBindings  []LazyopsYAMLDependencyBinding
	CompatibilityPolicy LazyopsYAMLCompatibilityPolicy
	MagicDomainPolicy   LazyopsYAMLMagicDomainPolicy
	ScaleToZeroPolicy   LazyopsYAMLScaleToZeroPolicy
	ArtifactMetadata    BlueprintArtifactMetadata
}

type BlueprintRecord struct {
	ID         string
	ProjectID  string
	SourceKind string
	SourceRef  string
	Compiled   BlueprintCompiledContractRecord
	CreatedAt  time.Time
}

type DesiredStateRevisionDraftRecord struct {
	RevisionID           string
	ProjectID            string
	BlueprintID          string
	DeploymentBindingID  string
	CommitSHA            string
	ArtifactRef          string
	ImageRef             string
	TriggerKind          string
	RuntimeMode          string
	Services             []BlueprintServiceContractRecord
	DependencyBindings   []LazyopsYAMLDependencyBinding
	CompatibilityPolicy  LazyopsYAMLCompatibilityPolicy
	MagicDomainPolicy    LazyopsYAMLMagicDomainPolicy
	ScaleToZeroPolicy    LazyopsYAMLScaleToZeroPolicy
	PlacementAssignments []PlacementAssignmentRecord
}

type CompileBlueprintResult struct {
	Services             []ProjectServiceRecord
	Blueprint            BlueprintRecord
	DesiredRevisionDraft DesiredStateRevisionDraftRecord
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
