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

type BootstrapTokenProfile struct {
	RuntimeMode string
	AgentKind   string
	TargetRef   string
}

type InstallInstanceAgentSSHCommand struct {
	UserID                  string
	ProjectID               string
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

type InstallBootstrapStageRecord struct {
	ID      string
	State   string
	Message string
}

type InstallInstanceAgentSSHResult struct {
	InstanceID         string
	Bootstrap          BootstrapTokenIssue
	StartedAt          time.Time
	HostKeyFingerprint string
	AttachedProjectID  string
	ClusterID          string
	ClusterName        string
	ClusterStatus      string
	TargetKind         string
	RuntimeMode        string
	Stages             []InstallBootstrapStageRecord
}

type JoinClusterNodeSSHCommand struct {
	UserID                  string
	ClusterID               string
	InstanceID              string
	Host                    string
	Port                    int
	Username                string
	Password                string
	PrivateKey              string
	HostKeyFingerprint      string
	ControlPlaneURL         string
	JoinServerURL           string
	JoinToken               string
	StateDir                string
	ContainerRuntimeRootDir string
}

type JoinClusterNodeSSHResult struct {
	ClusterID           string
	InstanceID          string
	StartedAt           time.Time
	HostKeyFingerprint  string
	NodeName            string
	JoinServerURL       string
	LabeledByControl    bool
	PlacementLabelKey   string
	PlacementLabelValue string
	Stages              []InstallBootstrapStageRecord
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

type UpsertManagedClusterCommand struct {
	UserID              string
	InstanceID          string
	Name                string
	Provider            string
	KubeconfigSecretRef string
	PublicIP            string
	Status              string
	JoinServerURL       string
	JoinToken           string
}

type ClusterSummary struct {
	ID         string
	TargetKind string
	Name       string
	Provider   string
	Status     string
	PublicIP   *string
	InstanceID *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ClusterListResult struct {
	Items []ClusterSummary
}

type ClusterNodeRecord struct {
	ClusterID   string
	InstanceID  string
	Name        string
	Status      string
	K8sNodeName string
	Labels      map[string]string
	LastSeenAt  *time.Time
	IsReady     bool
}

type ClusterNodeListResult struct {
	Items []ClusterNodeRecord
}

type ConnectClusterNodeSSHCommand struct {
	UserID             string
	ClusterID          string
	InstanceName       string
	PublicIP           string
	PrivateIP          string
	Labels             map[string]string
	Host               string
	Port               int
	Username           string
	Password           string
	PrivateKey         string
	HostKeyFingerprint string
	ControlPlaneURL    string
	AgentImage         string
	ContainerName      string
}

type ConnectClusterNodeSSHResult struct {
	ClusterID string
	Instance  InstanceSummary
	Join      JoinClusterNodeSSHResult
}

type PlacementNodeListResult struct {
	ClusterID string
	Items     []ClusterNodeRecord
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
	RoutingPolicy       LazyopsYAMLRoutingPolicy        `json:"routing_policy,omitempty"`
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
	TransparentProxy   bool `json:"transparent_proxy"`
}

type LazyopsYAMLRoutingPolicy struct {
	SharedDomain string             `json:"shared_domain,omitempty"`
	Routes       []LazyopsYAMLRoute `json:"routes,omitempty"`
}

type LazyopsYAMLRoute struct {
	Path            string `json:"path"`
	Service         string `json:"service"`
	Port            int    `json:"port,omitempty"`
	WebSocket       bool   `json:"websocket,omitempty"`
	StripPrefix     bool   `json:"strip_prefix,omitempty"`
	StripPrefixMode string `json:"strip_prefix_mode,omitempty"`
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
	Project              ProjectSummary
	DeploymentBinding    DeploymentBindingRecord
	TargetSummary        InitTargetSummary
	Schema               LazyopsYAMLSchemaSummary
	SuggestedRoutes      []RoutingGuidanceRouteRecord
	EffectivePublicPaths []RoutingGuidanceRouteRecord
	MigrationFindings    []MigrationFindingRecord
}

type BlueprintArtifactMetadata struct {
	CommitSHA        string
	ArtifactRef      string
	ImageRef         string
	ServiceArtifacts []BuildServiceArtifactRecord
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

type ServiceDetectedPortRecord struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
	Exposed  bool   `json:"exposed,omitempty"`
}

type ManifestDocumentRecord struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content"`
}

type K3sManifestBundleRecord struct {
	Namespace    string                   `json:"namespace"`
	CombinedYAML string                   `json:"combined_yaml"`
	RollbackYAML string                   `json:"rollback_yaml,omitempty"`
	Documents    []ManifestDocumentRecord `json:"documents,omitempty"`
	GeneratedAt  time.Time                `json:"generated_at"`
}

type InternalBindingRecord struct {
	ServiceName      string `json:"service_name"`
	Alias            string `json:"alias"`
	TargetService    string `json:"target_service"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Protocol         string `json:"protocol"`
	URL              string `json:"url,omitempty"`
	ConnectionString string `json:"connection_string,omitempty"`
}

type K3sServiceSpecRecord struct {
	Name            string                      `json:"name"`
	Kind            string                      `json:"kind"`
	Namespace       string                      `json:"namespace,omitempty"`
	Path            string                      `json:"path,omitempty"`
	Public          bool                        `json:"public"`
	PlacementMode   string                      `json:"placement_mode,omitempty"`
	PlacementNodeID string                      `json:"placement_node_id,omitempty"`
	RuntimeProfile  string                      `json:"runtime_profile,omitempty"`
	StartHint       string                      `json:"start_hint,omitempty"`
	ImageRef        string                      `json:"image_ref,omitempty"`
	ImageDigest     string                      `json:"image_digest,omitempty"`
	TargetPort      int                         `json:"target_port,omitempty"`
	ServicePort     int                         `json:"service_port,omitempty"`
	Replicas        int                         `json:"replicas,omitempty"`
	Healthcheck     map[string]any              `json:"healthcheck,omitempty"`
	DetectedPorts   []ServiceDetectedPortRecord `json:"detected_ports,omitempty"`
	EnvBundle       map[string]string           `json:"env_bundle,omitempty"`
	PVCSpec         map[string]any              `json:"pvc_spec,omitempty"`
	DeployStrategy  map[string]any              `json:"deploy_strategy,omitempty"`
}

type ProjectServiceDependencyBinding struct {
	TargetService         string
	ConnectionTemplateKey string
	ConnectionTemplate    map[string]string
}

type ProjectServiceRecord struct {
	ID                      string
	ProjectID               string
	Name                    string
	Path                    string
	Kind                    string
	SourceType              string
	Public                  bool
	RuntimeProfile          string
	PlacementMode           string
	PlacementNodeID         string
	Dependencies            []ProjectServiceDependencyBinding
	ConnectionTemplateKey   string
	ConnectionTemplate      map[string]string
	ConnectionTargetService string
	ManagedByLazyops        bool
	StartHint               string
	ImageRef                string
	ImageDigest             string
	DetectedPorts           []ServiceDetectedPortRecord
	TargetPort              int
	ServicePort             int
	Replicas                int
	EnvBundle               map[string]string
	PVCSpec                 map[string]any
	DeployStrategy          map[string]any
	Healthcheck             map[string]any
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type ProjectServiceListResult struct {
	Items []ProjectServiceRecord
}

type ConfigureProjectServicesCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	Items           []ConfigureProjectServiceItem
}

type ProjectServiceActionResult struct {
	Action       string
	ServiceID    string
	ServiceName  string
	Status       string
	TriggerKind  string
	DeploymentID string
	RevisionID   string
	Message      string
}

type ProjectRuntimeLogPreviewRecord struct {
	ID            string
	Source        string
	Level         string
	Message       string
	Timestamp     time.Time
	Node          string
	CorrelationID string
}

type ProjectRuntimeDependencyRecord struct {
	ServiceID        string
	ServiceName      string
	Status           string
	StatusReason     string
	InternalEndpoint string
}

type ProjectRuntimeNodeRecord struct {
	ClusterID   string
	InstanceID  string
	Name        string
	Status      string
	K8sNodeName string
	Labels      map[string]string
	LastSeenAt  *time.Time
	IsReady     bool
}

type ProjectRuntimeServiceRecord struct {
	ServiceID         string
	Name              string
	Kind              string
	SourceType        string
	Public            bool
	RuntimeProfile    string
	RuntimeStatus     string
	RuntimeReason     string
	BuildState        string
	RolloutState      string
	PlacementMode     string
	RequestedNodeID   string
	EffectiveNodeIDs  []string
	ImageRef          string
	ImageDigest       string
	RevisionID        string
	Revision          int
	DeploymentID      string
	PublicURLs        []string
	InternalEndpoints []string
	Dependencies      []ProjectRuntimeDependencyRecord
	RecentLogs        []ProjectRuntimeLogPreviewRecord
}

type ProjectRuntimeSummaryResult struct {
	ProjectID        string
	RuntimeMode      string
	ClusterID        string
	Namespace        string
	LiveRevisionID   string
	LiveRevision     int
	StableRevisionID string
	StableRevision   int
	SyncState        string
	SyncReason       string
	PublicURLs       []string
	PublicURLStatus  string
	PublicURLReason  string
	Nodes            []ProjectRuntimeNodeRecord
	Services         []ProjectRuntimeServiceRecord
}

type ConfigureProjectServiceItem struct {
	Name                    string
	Path                    string
	Kind                    string
	SourceType              string
	Public                  bool
	RuntimeProfile          string
	PlacementMode           string
	PlacementNodeID         string
	Dependencies            []ProjectServiceDependencyBinding
	ConnectionTemplateKey   string
	ConnectionTemplate      map[string]string
	ConnectionTargetService string
	ManagedByLazyops        bool
	StartHint               string
	ImageRef                string
	ImageDigest             string
	TargetPort              int
	ServicePort             int
	Replicas                int
	EnvBundle               map[string]string
	PVCSpec                 map[string]any
	DeployStrategy          map[string]any
	Healthcheck             map[string]any
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
	Name                    string
	Path                    string
	Kind                    string
	SourceType              string
	Public                  bool
	RuntimeProfile          string
	PlacementMode           string
	PlacementNodeID         string
	Dependencies            []ProjectServiceDependencyBinding
	ConnectionTemplateKey   string
	ConnectionTemplate      map[string]string
	ConnectionTargetService string
	ManagedByLazyops        bool
	StartHint               string
	ImageRef                string
	ImageDigest             string
	DetectedPorts           []ServiceDetectedPortRecord
	TargetPort              int
	ServicePort             int
	Replicas                int
	EnvBundle               map[string]string
	PVCSpec                 map[string]any
	DeployStrategy          map[string]any
	Healthcheck             map[string]any
}

type PlacementAssignmentRecord struct {
	ServiceName string
	TargetID    string
	TargetKind  string
	Labels      map[string]string
}

type BlueprintCompiledContractRecord struct {
	ProjectID           string
	ProjectSlug         string
	Namespace           string
	RuntimeMode         string
	Repo                BlueprintRepoStateRecord
	Binding             DeploymentBindingRecord
	Services            []BlueprintServiceContractRecord
	DependencyBindings  []LazyopsYAMLDependencyBinding
	CompatibilityPolicy LazyopsYAMLCompatibilityPolicy
	MagicDomainPolicy   LazyopsYAMLMagicDomainPolicy
	ScaleToZeroPolicy   LazyopsYAMLScaleToZeroPolicy
	RoutingPolicy       LazyopsYAMLRoutingPolicy
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
	Namespace            string
	CommitSHA            string
	ArtifactRef          string
	ImageRef             string
	TriggerKind          string
	RuntimeMode          string
	Services             []BlueprintServiceContractRecord
	ServiceSpecs         []K3sServiceSpecRecord
	DependencyBindings   []LazyopsYAMLDependencyBinding
	InternalBindings     []InternalBindingRecord
	CompatibilityPolicy  LazyopsYAMLCompatibilityPolicy
	MagicDomainPolicy    LazyopsYAMLMagicDomainPolicy
	ScaleToZeroPolicy    LazyopsYAMLScaleToZeroPolicy
	RoutingPolicy        LazyopsYAMLRoutingPolicy
	PlacementAssignments []PlacementAssignmentRecord
	ManifestBundle       K3sManifestBundleRecord
}

type CompileBlueprintResult struct {
	Services             []ProjectServiceRecord
	Blueprint            BlueprintRecord
	DesiredRevisionDraft DesiredStateRevisionDraftRecord
}

type CreateProjectCommand struct {
	UserID           string
	Name             string
	Slug             string
	NamespaceSlug    string
	ClusterID        string
	RuntimeMode      string
	DefaultBranch    string
	Services         []ConfigureProjectServiceItem
	InternalServices []string
}

type ProjectSummary struct {
	ID            string
	Name          string
	Slug          string
	NamespaceSlug string
	ClusterID     string
	RuntimeMode   string
	DefaultBranch string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ConfigureProjectInternalServicesCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	Kinds           []string
}

type ProjectInternalServiceRecord struct {
	ID            string
	ProjectID     string
	Kind          string
	Alias         string
	Protocol      string
	Port          int
	LocalEndpoint string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ProjectInternalServiceListResult struct {
	Items []ProjectInternalServiceRecord
}

type UpsertProjectEnvCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	Content         string
}

type ProjectEnvHelperSnippet struct {
	Language  string
	Framework string
	Kind      string
	Title     string
	Content   string
}

type ProjectEnvBundleRecord struct {
	Configured      bool
	UpdatedAt       *time.Time
	Fingerprint     string
	Keys            []string
	UserKeys        []string
	ManagedKeys     []string
	ProvisionedKeys []string
	ParseWarnings   []string
	HelperPacks     []ProjectEnvHelperPack
}

type ProjectEnvHelperPack struct {
	ServiceKind      string
	Alias            string
	Category         string
	Audience         string
	SourceService    string
	RelatedServices  []string
	PrimaryKey       string
	PublicPath       string
	Managed          bool
	RuntimeInjected  bool
	PlaceholderEnv   map[string]string
	EnvExample       map[string]string
	LocalExampleEnv  map[string]string
	RuntimeKeys      []string
	ProvisionedKeys  []string
	Notes            []string
	LanguageSnippets []ProjectEnvHelperSnippet
}

type ProjectAIPromptServiceSnapshot struct {
	Name           string
	Kind           string
	Role           string
	RuntimeProfile string
	SourceType     string
	Public         bool
	Managed        bool
	WebSocket      bool
	PublicPath     string
	InternalURL    string
}

type ProjectAIPromptSourceSection struct {
	Key         string
	Title       string
	Description string
	ItemCount   int
}

type ProjectAIPromptRecord struct {
	Title                string
	Summary              string
	Prompt               string
	ServiceSnapshot      []ProjectAIPromptServiceSnapshot
	EffectivePublicPaths []RoutingGuidanceRouteRecord
	ManagedKeys          []string
	MigrationFindings    []MigrationFindingRecord
	SourceSections       []ProjectAIPromptSourceSection
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
