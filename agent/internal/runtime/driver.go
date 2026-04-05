package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/state"
)

var ErrNotImplemented = fmt.Errorf("runtime driver operation is not implemented yet")

type Driver interface {
	PrepareReleaseWorkspace(context.Context, RuntimeContext) (PreparedWorkspace, error)
	ReconcileRevision(context.Context, RuntimeContext) (ReconcileRevisionResult, error)
	RenderGatewayConfig(context.Context, RuntimeContext) (GatewayRenderResult, error)
	RenderSidecars(context.Context, RuntimeContext) (SidecarRenderResult, error)
	StartReleaseCandidate(context.Context, RuntimeContext) (CandidateRecord, error)
	RunHealthGate(context.Context, RuntimeContext) (HealthGateResult, error)
	PromoteRelease(context.Context, RuntimeContext) (PromoteReleaseResult, error)
	RollbackRelease(context.Context, RuntimeContext) (RollbackReleaseResult, error)
	SleepService(context.Context, RuntimeContext, string) error
	WakeService(context.Context, RuntimeContext, string) error
	GarbageCollectRuntime(context.Context, RuntimeContext) (GarbageCollectRuntimeResult, error)
}

type RuntimeContext struct {
	Project  ProjectMetadata                    `json:"project"`
	Binding  contracts.DeploymentBindingPayload `json:"binding"`
	Revision contracts.DesiredRevisionPayload   `json:"revision"`
	Services []ServiceRuntimeContext            `json:"services"`
	Rollout  RolloutContext                     `json:"-"`
	Runtime  RuntimeDependencyContext           `json:"-"`
}

type RolloutContext struct {
	StableRevisionID         string
	CurrentRevisionID        string
	PreviousStableRevisionID string
	PendingRevisionID        string
	CandidateRevisionID      string
	DrainingRevisionID       string
}

type RuntimeDependencyContext struct {
	PlacementByService   map[string]contracts.PlacementAssignment `json:"-"`
	ServiceByName        map[string]ServiceRuntimeContext         `json:"-"`
	PlacementFingerprint string                                   `json:"-"`
}

type ProjectMetadata struct {
	ProjectID string            `json:"project_id"`
	Name      string            `json:"name,omitempty"`
	Slug      string            `json:"slug,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type ServiceRuntimeContext struct {
	Name           string                               `json:"name"`
	Path           string                               `json:"path"`
	Public         bool                                 `json:"public"`
	RuntimeProfile string                               `json:"runtime_profile,omitempty"`
	StartHint      string                               `json:"start_hint,omitempty"`
	Labels         map[string]string                    `json:"labels,omitempty"`
	HealthCheck    contracts.HealthCheckPayload         `json:"healthcheck"`
	Dependencies   []contracts.DependencyBindingPayload `json:"dependencies,omitempty"`
	Placement      *contracts.PlacementAssignment       `json:"placement,omitempty"`
}

type WorkspaceLayout struct {
	Root      string `json:"root"`
	Artifacts string `json:"artifacts"`
	Config    string `json:"config"`
	Sidecars  string `json:"sidecars"`
	Gateway   string `json:"gateway"`
	Mesh      string `json:"mesh"`
	Services  string `json:"services"`
}

type PreparedWorkspace struct {
	Layout            WorkspaceLayout         `json:"layout"`
	ManifestPath      string                  `json:"manifest_path"`
	ServiceManifests  map[string]string       `json:"service_manifests"`
	Artifact          ArtifactMaterialization `json:"artifact"`
	SidecarConfigPath string                  `json:"sidecar_config_path"`
	GatewayConfigPath string                  `json:"gateway_config_path"`
	MeshStatePath     string                  `json:"mesh_state_path,omitempty"`
	ServiceCachePath  string                  `json:"service_cache_path,omitempty"`
}

type WorkspaceManifest struct {
	PreparedAt   time.Time                          `json:"prepared_at"`
	Project      ProjectMetadata                    `json:"project"`
	Binding      contracts.DeploymentBindingPayload `json:"binding"`
	Revision     contracts.DesiredRevisionPayload   `json:"revision"`
	Services     []ServiceRuntimeContext            `json:"services"`
	Layout       WorkspaceLayout                    `json:"layout"`
	ArtifactPlan ArtifactPlan                       `json:"artifact_plan"`
	GatewayPlan  GatewayPlan                        `json:"gateway_plan"`
	SidecarPlan  SidecarPlan                        `json:"sidecar_plan"`
	MeshSnapshot *MeshFoundationSnapshot            `json:"mesh_snapshot,omitempty"`
}

type ArtifactPlan struct {
	Status        string `json:"status"`
	RevisionID    string `json:"revision_id"`
	ArtifactRef   string `json:"artifact_ref,omitempty"`
	ImageRef      string `json:"image_ref,omitempty"`
	CacheKey      string `json:"cache_key,omitempty"`
	CachePath     string `json:"cache_path,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty"`
}

type GatewayPlan struct {
	Version              string             `json:"version,omitempty"`
	GeneratedAt          time.Time          `json:"generated_at,omitempty"`
	Provider             string             `json:"provider"`
	PublicServices       []string           `json:"public_services"`
	MagicDomain          string             `json:"magic_domain_provider,omitempty"`
	FallbackMagicDomain  string             `json:"fallback_magic_domain_provider,omitempty"`
	HostToken            string             `json:"host_token,omitempty"`
	PlacementFingerprint string             `json:"placement_fingerprint,omitempty"`
	RouteFingerprint     string             `json:"route_fingerprint,omitempty"`
	InvalidationRules    []string           `json:"invalidation_rules,omitempty"`
	Routes               []GatewayRoute     `json:"routes,omitempty"`
	Validation           *GatewayHookResult `json:"validation,omitempty"`
	Apply                *GatewayHookResult `json:"apply,omitempty"`
	Reload               *GatewayHookResult `json:"reload,omitempty"`
	Rollback             *GatewayHookResult `json:"rollback,omitempty"`
}

type GatewayRoute struct {
	ServiceName           string                 `json:"service_name"`
	Port                  int                    `json:"port"`
	Upstream              string                 `json:"upstream"`
	PrimaryHost           string                 `json:"primary_host"`
	FallbackHost          string                 `json:"fallback_host"`
	PrimaryURL            string                 `json:"primary_url"`
	FallbackURL           string                 `json:"fallback_url"`
	RouteScope            string                 `json:"route_scope,omitempty"`
	ResolutionStatus      string                 `json:"resolution_status,omitempty"`
	PlacementPeerRef      string                 `json:"placement_peer_ref,omitempty"`
	Provider              contracts.MeshProvider `json:"provider,omitempty"`
	PublicFallbackBlocked bool                   `json:"public_fallback_blocked,omitempty"`
	InvalidationReasons   []string               `json:"invalidation_reasons,omitempty"`
	ResolutionReason      string                 `json:"resolution_reason,omitempty"`
}

type GatewayHookResult struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	Path       string    `json:"path,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type GatewayActivation struct {
	Version    string    `json:"version"`
	PlanPath   string    `json:"plan_path"`
	ConfigPath string    `json:"config_path"`
	AppliedAt  time.Time `json:"applied_at"`
}

type GatewayRenderResult struct {
	Version               string            `json:"version"`
	PlanPath              string            `json:"plan_path"`
	ConfigPath            string            `json:"config_path"`
	LivePlanPath          string            `json:"live_plan_path"`
	LiveConfigPath        string            `json:"live_config_path"`
	ActivationPath        string            `json:"activation_path"`
	PreviousActiveVersion string            `json:"previous_active_version,omitempty"`
	PublicURLs            []string          `json:"public_urls,omitempty"`
	Plan                  GatewayPlan       `json:"plan"`
	Activation            GatewayActivation `json:"activation"`
	RolledBack            bool              `json:"rolled_back,omitempty"`
}

type SidecarPlan struct {
	Version              string                        `json:"version,omitempty"`
	GeneratedAt          time.Time                     `json:"generated_at,omitempty"`
	EnabledServices      []string                      `json:"enabled_services"`
	Compatibility        contracts.CompatibilityPolicy `json:"compatibility_policy"`
	Precedence           []string                      `json:"precedence,omitempty"`
	PlacementFingerprint string                        `json:"placement_fingerprint,omitempty"`
	RouteFingerprint     string                        `json:"route_fingerprint,omitempty"`
	InvalidationRules    []string                      `json:"invalidation_rules,omitempty"`
	Bindings             []SidecarBinding              `json:"bindings,omitempty"`
	Services             []SidecarServiceConfig        `json:"services,omitempty"`
	MetadataCachePath    string                        `json:"metadata_cache_path,omitempty"`
	Create               *SidecarHookResult            `json:"create,omitempty"`
	Reconcile            *SidecarHookResult            `json:"reconcile,omitempty"`
	Restart              *SidecarHookResult            `json:"restart,omitempty"`
	Remove               *SidecarHookResult            `json:"remove,omitempty"`
}

type SidecarBinding struct {
	ServiceName           string                 `json:"service_name"`
	Alias                 string                 `json:"alias"`
	TargetService         string                 `json:"target_service"`
	Protocol              string                 `json:"protocol"`
	LocalEndpoint         string                 `json:"local_endpoint,omitempty"`
	RouteScope            string                 `json:"route_scope,omitempty"`
	ResolutionStatus      string                 `json:"resolution_status,omitempty"`
	PlacementPeerRef      string                 `json:"placement_peer_ref,omitempty"`
	ResolvedEndpoint      string                 `json:"resolved_endpoint,omitempty"`
	ResolvedUpstream      string                 `json:"resolved_upstream,omitempty"`
	Provider              contracts.MeshProvider `json:"provider,omitempty"`
	PublicFallbackBlocked bool                   `json:"public_fallback_blocked,omitempty"`
	InvalidationReasons   []string               `json:"invalidation_reasons,omitempty"`
	ResolutionReason      string                 `json:"resolution_reason,omitempty"`
}

type SidecarServiceConfig struct {
	ServiceName                string                             `json:"service_name"`
	SelectedMode               string                             `json:"selected_mode"`
	DependencyAliases          []string                           `json:"dependency_aliases,omitempty"`
	Env                        map[string]string                  `json:"env,omitempty"`
	EnvContracts               []SidecarEnvContract               `json:"env_contracts,omitempty"`
	ManagedCredentials         map[string]string                  `json:"managed_credentials,omitempty"`
	ManagedCredentialContracts []SidecarManagedCredentialContract `json:"managed_credential_contracts,omitempty"`
	LocalhostRescueContracts   []SidecarLocalhostRescueContract   `json:"localhost_rescue_contracts,omitempty"`
	ProxyRoutes                []SidecarProxyRoute                `json:"proxy_routes,omitempty"`
	Resolutions                []DependencyResolutionView         `json:"resolutions,omitempty"`
	CorrelationPropagation     bool                               `json:"correlation_propagation"`
	LatencyMeasurement         bool                               `json:"latency_measurement"`
}

type SidecarProxyRoute struct {
	Alias                 string                 `json:"alias"`
	TargetService         string                 `json:"target_service"`
	Protocol              string                 `json:"protocol"`
	LocalEndpoint         string                 `json:"local_endpoint,omitempty"`
	ListenerHost          string                 `json:"listener_host,omitempty"`
	ListenerPort          int                    `json:"listener_port,omitempty"`
	ListenerScheme        string                 `json:"listener_scheme,omitempty"`
	ForwardingMode        string                 `json:"forwarding_mode,omitempty"`
	Upstream              string                 `json:"upstream"`
	RouteScope            string                 `json:"route_scope,omitempty"`
	ResolutionStatus      string                 `json:"resolution_status,omitempty"`
	PlacementPeerRef      string                 `json:"placement_peer_ref,omitempty"`
	Provider              contracts.MeshProvider `json:"provider,omitempty"`
	PublicFallbackBlocked bool                   `json:"public_fallback_blocked,omitempty"`
	InvalidationReasons   []string               `json:"invalidation_reasons,omitempty"`
	ResolutionReason      string                 `json:"resolution_reason,omitempty"`
	FallbackClass         string                 `json:"fallback_class,omitempty"`
	FallbackReason        string                 `json:"fallback_reason,omitempty"`
	MeshHealthRequired    bool                   `json:"mesh_health_required,omitempty"`
	NetworkNamespace      bool                   `json:"network_namespace_intercept,omitempty"`
	LocalhostRescue       bool                   `json:"localhost_rescue"`
}

type SidecarHookResult struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	Path       string    `json:"path,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type SidecarActivation struct {
	Version    string    `json:"version"`
	PlanPath   string    `json:"plan_path"`
	ConfigPath string    `json:"config_path"`
	AppliedAt  time.Time `json:"applied_at"`
}

type SidecarMetadataCache struct {
	Version              string                            `json:"version"`
	UpdatedAt            time.Time                         `json:"updated_at"`
	PlacementFingerprint string                            `json:"placement_fingerprint,omitempty"`
	RouteFingerprint     string                            `json:"route_fingerprint,omitempty"`
	InvalidationRules    []string                          `json:"invalidation_rules,omitempty"`
	Services             map[string]SidecarServiceMetadata `json:"services"`
}

type SidecarServiceMetadata struct {
	SelectedMode               string                             `json:"selected_mode"`
	DependencyAliases          []string                           `json:"dependency_aliases,omitempty"`
	Protocols                  []string                           `json:"protocols,omitempty"`
	EnvContracts               []SidecarEnvContract               `json:"env_contracts,omitempty"`
	ManagedCredentialContracts []SidecarManagedCredentialContract `json:"managed_credential_contracts,omitempty"`
	LocalhostRescueContracts   []SidecarLocalhostRescueContract   `json:"localhost_rescue_contracts,omitempty"`
	Resolutions                []DependencyResolutionView         `json:"resolutions,omitempty"`
	CacheInvalidationRules     []string                           `json:"cache_invalidation_rules,omitempty"`
	ConfigPath                 string                             `json:"config_path,omitempty"`
	RuntimePath                string                             `json:"runtime_path,omitempty"`
	ManagedCredentialAuditPath string                             `json:"managed_credential_audit_path,omitempty"`
	CorrelationPropagation     bool                               `json:"correlation_propagation"`
	LatencyMeasurement         bool                               `json:"latency_measurement"`
}

type SidecarEnvContract struct {
	Alias               string            `json:"alias"`
	TargetService       string            `json:"target_service"`
	Protocol            string            `json:"protocol"`
	RequiredKeys        []string          `json:"required_keys,omitempty"`
	Values              map[string]string `json:"values"`
	RouteScope          string            `json:"route_scope,omitempty"`
	ResolutionStatus    string            `json:"resolution_status,omitempty"`
	PlacementPeerRef    string            `json:"placement_peer_ref,omitempty"`
	InvalidationReasons []string          `json:"invalidation_reasons,omitempty"`
	ResolutionReason    string            `json:"resolution_reason,omitempty"`
	SecretSafe          bool              `json:"secret_safe"`
}

type SidecarManagedCredentialContract struct {
	Alias                  string            `json:"alias"`
	TargetService          string            `json:"target_service"`
	Protocol               string            `json:"protocol"`
	CredentialRef          string            `json:"credential_ref"`
	RequiredKeys           []string          `json:"required_keys,omitempty"`
	Values                 map[string]string `json:"values"`
	MaskedValues           map[string]string `json:"masked_values,omitempty"`
	ValueFingerprints      map[string]string `json:"value_fingerprints,omitempty"`
	RouteScope             string            `json:"route_scope,omitempty"`
	ResolutionStatus       string            `json:"resolution_status,omitempty"`
	PlacementPeerRef       string            `json:"placement_peer_ref,omitempty"`
	InvalidationReasons    []string          `json:"invalidation_reasons,omitempty"`
	ResolutionReason       string            `json:"resolution_reason,omitempty"`
	SecretSafe             bool              `json:"secret_safe"`
	LocalhostRescueSkipped bool              `json:"localhost_rescue_skipped"`
}

type SidecarLocalhostRescueContract struct {
	Alias                     string                 `json:"alias"`
	TargetService             string                 `json:"target_service"`
	Protocol                  string                 `json:"protocol"`
	ListenerEndpoint          string                 `json:"listener_endpoint"`
	ListenerHost              string                 `json:"listener_host"`
	ListenerPort              int                    `json:"listener_port"`
	ListenerScheme            string                 `json:"listener_scheme,omitempty"`
	ForwardingMode            string                 `json:"forwarding_mode"`
	Upstream                  string                 `json:"upstream"`
	RouteScope                string                 `json:"route_scope,omitempty"`
	ResolutionStatus          string                 `json:"resolution_status,omitempty"`
	PlacementPeerRef          string                 `json:"placement_peer_ref,omitempty"`
	Provider                  contracts.MeshProvider `json:"provider,omitempty"`
	PublicFallbackBlocked     bool                   `json:"public_fallback_blocked,omitempty"`
	InvalidationReasons       []string               `json:"invalidation_reasons,omitempty"`
	ResolutionReason          string                 `json:"resolution_reason,omitempty"`
	FallbackClass             string                 `json:"fallback_class"`
	FallbackReason            string                 `json:"fallback_reason,omitempty"`
	MeshHealthRequired        bool                   `json:"mesh_health_required,omitempty"`
	NetworkNamespaceIntercept bool                   `json:"network_namespace_intercept"`
}

type ManagedCredentialAuditLog struct {
	Version              string                                    `json:"version"`
	UpdatedAt            time.Time                                 `json:"updated_at"`
	PlaintextPersisted   bool                                      `json:"plaintext_persisted"`
	LoggerRedactionScope []string                                  `json:"logger_redaction_scope,omitempty"`
	Services             map[string][]ManagedCredentialAuditRecord `json:"services,omitempty"`
}

type ManagedCredentialAuditRecord struct {
	ServiceName            string            `json:"service_name"`
	Alias                  string            `json:"alias"`
	TargetService          string            `json:"target_service"`
	Protocol               string            `json:"protocol"`
	CredentialRef          string            `json:"credential_ref"`
	MaskedValues           map[string]string `json:"masked_values,omitempty"`
	ValueFingerprints      map[string]string `json:"value_fingerprints,omitempty"`
	RouteScope             string            `json:"route_scope,omitempty"`
	ResolutionStatus       string            `json:"resolution_status,omitempty"`
	PlacementPeerRef       string            `json:"placement_peer_ref,omitempty"`
	PlaintextPersisted     bool              `json:"plaintext_persisted"`
	SecretSafe             bool              `json:"secret_safe"`
	LocalhostRescueSkipped bool              `json:"localhost_rescue_skipped"`
	AuditedAt              time.Time         `json:"audited_at"`
}

type DependencyResolutionView struct {
	Alias                 string                 `json:"alias"`
	TargetService         string                 `json:"target_service"`
	Protocol              string                 `json:"protocol"`
	RouteScope            string                 `json:"route_scope"`
	ResolutionStatus      string                 `json:"resolution_status"`
	ResolvedEndpoint      string                 `json:"resolved_endpoint,omitempty"`
	ResolvedUpstream      string                 `json:"resolved_upstream,omitempty"`
	PlacementTargetID     string                 `json:"placement_target_id,omitempty"`
	PlacementTargetKind   contracts.TargetKind   `json:"placement_target_kind,omitempty"`
	PlacementPeerRef      string                 `json:"placement_peer_ref,omitempty"`
	Provider              contracts.MeshProvider `json:"provider,omitempty"`
	PublicFallbackBlocked bool                   `json:"public_fallback_blocked,omitempty"`
	InvalidationReasons   []string               `json:"invalidation_reasons,omitempty"`
	Reason                string                 `json:"reason,omitempty"`
}

type SidecarRenderResult struct {
	Version                    string            `json:"version"`
	PlanPath                   string            `json:"plan_path"`
	ConfigPath                 string            `json:"config_path"`
	LivePlanPath               string            `json:"live_plan_path"`
	LiveConfigRoot             string            `json:"live_config_root"`
	ActivationPath             string            `json:"activation_path"`
	MetadataCachePath          string            `json:"metadata_cache_path"`
	ManagedCredentialAuditPath string            `json:"managed_credential_audit_path,omitempty"`
	Services                   []string          `json:"services,omitempty"`
	Plan                       SidecarPlan       `json:"plan"`
	Activation                 SidecarActivation `json:"activation"`
}

type ArtifactMaterialization struct {
	Status        string    `json:"status"`
	ArtifactRef   string    `json:"artifact_ref,omitempty"`
	ImageRef      string    `json:"image_ref,omitempty"`
	CacheKey      string    `json:"cache_key,omitempty"`
	CachePath     string    `json:"cache_path,omitempty"`
	WorkspacePath string    `json:"workspace_path,omitempty"`
	ResolvedAt    time.Time `json:"resolved_at"`
}

type CandidateState string

const (
	CandidateStatePrepared   CandidateState = "prepared"
	CandidateStateStarting   CandidateState = "starting"
	CandidateStateHealthy    CandidateState = "healthy"
	CandidateStateUnhealthy  CandidateState = "unhealthy"
	CandidateStatePromotable CandidateState = "promotable"
	CandidateStateFailed     CandidateState = "failed"
)

type CandidateTransition struct {
	From       CandidateState `json:"from,omitempty"`
	To         CandidateState `json:"to"`
	Reason     string         `json:"reason,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
}

type HealthGatePolicyAction string

const (
	HealthGatePolicyPromoteCandidate HealthGatePolicyAction = "promote_candidate"
	HealthGatePolicyStopRollout      HealthGatePolicyAction = "stop_rollout"
	HealthGatePolicyRollbackRelease  HealthGatePolicyAction = "rollback_release"
)

type ServiceHealthResult struct {
	ServiceName string    `json:"service_name"`
	Protocol    string    `json:"protocol"`
	Address     string    `json:"address,omitempty"`
	Path        string    `json:"path,omitempty"`
	Attempts    int       `json:"attempts"`
	Successes   int       `json:"successes"`
	Failures    int       `json:"failures"`
	Passed      bool      `json:"passed"`
	StatusCode  int       `json:"status_code,omitempty"`
	LatencyMS   float64   `json:"latency_ms,omitempty"`
	Message     string    `json:"message,omitempty"`
	CheckedAt   time.Time `json:"checked_at"`
}

type RolloutSummary struct {
	RevisionID        string                 `json:"revision_id"`
	CandidateState    CandidateState         `json:"candidate_state"`
	PolicyAction      HealthGatePolicyAction `json:"policy_action"`
	Summary           string                 `json:"summary"`
	HealthyServices   int                    `json:"healthy_services"`
	UnhealthyServices int                    `json:"unhealthy_services"`
	CheckedAt         time.Time              `json:"checked_at"`
}

type CandidateHealthSnapshot struct {
	CheckedAt          time.Time                  `json:"checked_at"`
	CandidateState     CandidateState             `json:"candidate_state"`
	Promotable         bool                       `json:"promotable"`
	PolicyAction       HealthGatePolicyAction     `json:"policy_action"`
	Summary            string                     `json:"summary"`
	Services           []ServiceHealthResult      `json:"services"`
	Incident           *contracts.IncidentPayload `json:"incident,omitempty"`
	IncidentSuppressed bool                       `json:"incident_suppressed,omitempty"`
}

type CandidateRecord struct {
	RevisionID       string                     `json:"revision_id"`
	WorkspaceRoot    string                     `json:"workspace_root"`
	State            CandidateState             `json:"state"`
	StartedAt        time.Time                  `json:"started_at"`
	ManifestPath     string                     `json:"manifest_path"`
	LastTransitionAt time.Time                  `json:"last_transition_at,omitempty"`
	History          []CandidateTransition      `json:"history,omitempty"`
	HealthGate       *CandidateHealthSnapshot   `json:"health_gate,omitempty"`
	RolloutSummary   *RolloutSummary            `json:"rollout_summary,omitempty"`
	LatestIncident   *contracts.IncidentPayload `json:"latest_incident,omitempty"`
	LastIncidentKey  string                     `json:"last_incident_key,omitempty"`
	LastIncidentAt   time.Time                  `json:"last_incident_at,omitempty"`
}

type AssetFetcher interface {
	FetchRevisionAssets(context.Context, RuntimeContext, WorkspaceLayout) (ArtifactMaterialization, error)
}

type HealthGateResult struct {
	RevisionID         string                     `json:"revision_id"`
	CandidateState     CandidateState             `json:"candidate_state"`
	Promotable         bool                       `json:"promotable"`
	PolicyAction       HealthGatePolicyAction     `json:"policy_action"`
	Summary            string                     `json:"summary"`
	CheckedAt          time.Time                  `json:"checked_at"`
	Services           []ServiceHealthResult      `json:"services"`
	Incident           *contracts.IncidentPayload `json:"incident,omitempty"`
	IncidentSuppressed bool                       `json:"incident_suppressed,omitempty"`
	ReportPath         string                     `json:"report_path,omitempty"`
	RolloutSummaryPath string                     `json:"rollout_summary_path,omitempty"`
}

type DeploymentEvent struct {
	Type       string         `json:"type"`
	RevisionID string         `json:"revision_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Summary    string         `json:"summary"`
	Details    map[string]any `json:"details,omitempty"`
}

type LatencySignal struct {
	ServiceName string  `json:"service_name"`
	Protocol    string  `json:"protocol"`
	LatencyMS   float64 `json:"latency_ms"`
	Status      string  `json:"status"`
}

type DrainPlan struct {
	PreviousRevisionID string    `json:"previous_revision_id,omitempty"`
	PromotedRevisionID string    `json:"promoted_revision_id"`
	Status             string    `json:"status"`
	ZeroDowntime       bool      `json:"zero_downtime"`
	CleanupPolicy      string    `json:"cleanup_policy"`
	StartedAt          time.Time `json:"started_at"`
}

type TrafficShiftRecord struct {
	ActiveRevisionID   string    `json:"active_revision_id"`
	PreviousRevisionID string    `json:"previous_revision_id,omitempty"`
	StableRevisionID   string    `json:"stable_revision_id,omitempty"`
	GatewayVersion     string    `json:"gateway_version,omitempty"`
	SidecarVersion     string    `json:"sidecar_version,omitempty"`
	ZeroDowntime       bool      `json:"zero_downtime"`
	RollbackReady      bool      `json:"rollback_ready"`
	ShiftedAt          time.Time `json:"shifted_at"`
	DrainPlanPath      string    `json:"drain_plan_path,omitempty"`
}

type PromotionSummary struct {
	ProjectID                string            `json:"project_id"`
	BindingID                string            `json:"binding_id"`
	RevisionID               string            `json:"revision_id"`
	PreviousStableRevisionID string            `json:"previous_stable_revision_id,omitempty"`
	ZeroDowntime             bool              `json:"zero_downtime"`
	RollbackReady            bool              `json:"rollback_ready"`
	DrainStatus              string            `json:"drain_status"`
	GatewayVersion           string            `json:"gateway_version,omitempty"`
	SidecarVersion           string            `json:"sidecar_version,omitempty"`
	PublicURLs               []string          `json:"public_urls,omitempty"`
	LatencySignals           []LatencySignal   `json:"latency_signals,omitempty"`
	Events                   []DeploymentEvent `json:"events,omitempty"`
	Summary                  string            `json:"summary"`
	PromotedAt               time.Time         `json:"promoted_at"`
}

type PromoteReleaseResult struct {
	RevisionID               string             `json:"revision_id"`
	PreviousStableRevisionID string             `json:"previous_stable_revision_id,omitempty"`
	ZeroDowntime             bool               `json:"zero_downtime"`
	RollbackReady            bool               `json:"rollback_ready"`
	GatewayVersion           string             `json:"gateway_version,omitempty"`
	SidecarVersion           string             `json:"sidecar_version,omitempty"`
	TrafficPath              string             `json:"traffic_path"`
	DrainPlanPath            string             `json:"drain_plan_path"`
	SummaryPath              string             `json:"summary_path"`
	EventsPath               string             `json:"events_path"`
	Summary                  PromotionSummary   `json:"summary_payload"`
	Traffic                  TrafficShiftRecord `json:"traffic"`
	DrainPlan                DrainPlan          `json:"drain_plan"`
	Events                   []DeploymentEvent  `json:"events"`
}

type RollbackDrainPlan struct {
	FailedRevisionID   string    `json:"failed_revision_id"`
	RestoredRevisionID string    `json:"restored_revision_id"`
	Status             string    `json:"status"`
	ZeroDowntime       bool      `json:"zero_downtime"`
	CleanupPolicy      string    `json:"cleanup_policy"`
	StartedAt          time.Time `json:"started_at"`
}

type RollbackSummary struct {
	ProjectID          string                     `json:"project_id"`
	BindingID          string                     `json:"binding_id"`
	FailedRevisionID   string                     `json:"failed_revision_id"`
	RestoredRevisionID string                     `json:"restored_revision_id"`
	ZeroDowntime       bool                       `json:"zero_downtime"`
	GatewayVersion     string                     `json:"gateway_version,omitempty"`
	SidecarVersion     string                     `json:"sidecar_version,omitempty"`
	PublicURLs         []string                   `json:"public_urls,omitempty"`
	Incident           *contracts.IncidentPayload `json:"incident,omitempty"`
	Events             []DeploymentEvent          `json:"events,omitempty"`
	Summary            string                     `json:"summary"`
	RolledBackAt       time.Time                  `json:"rolled_back_at"`
}

type RollbackReleaseResult struct {
	FailedRevisionID   string                     `json:"failed_revision_id"`
	RestoredRevisionID string                     `json:"restored_revision_id"`
	TrafficPath        string                     `json:"traffic_path"`
	EventsPath         string                     `json:"events_path"`
	SummaryPath        string                     `json:"summary_path"`
	IncidentPath       string                     `json:"incident_path"`
	DrainPlanPath      string                     `json:"drain_plan_path"`
	RollbackPath       string                     `json:"rollback_path"`
	Summary            RollbackSummary            `json:"summary_payload"`
	Traffic            TrafficShiftRecord         `json:"traffic"`
	Incident           *contracts.IncidentPayload `json:"incident,omitempty"`
	DrainPlan          RollbackDrainPlan          `json:"drain_plan"`
	Events             []DeploymentEvent          `json:"events"`
}

type GarbageCollectRuntimeResult struct {
	ProjectID              string    `json:"project_id"`
	BindingID              string    `json:"binding_id"`
	ProtectedRevisionIDs   []string  `json:"protected_revision_ids,omitempty"`
	RemovedRevisionRoots   []string  `json:"removed_revision_roots,omitempty"`
	RemovedGatewayVersions []string  `json:"removed_gateway_versions,omitempty"`
	RemovedSidecarVersions []string  `json:"removed_sidecar_versions,omitempty"`
	Summary                string    `json:"summary"`
	ReportPath             string    `json:"report_path"`
	CollectedAt            time.Time `json:"collected_at"`
}

type ReconcileRevisionResult struct {
	RevisionID   string    `json:"revision_id"`
	AppliedSteps []string  `json:"applied_steps"`
	Summary      string    `json:"summary"`
	CompletedAt  time.Time `json:"completed_at"`
}

type OperationError struct {
	Code      string
	Message   string
	Retryable bool
	Details   map[string]any
	Err       error
}

func (e *OperationError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "runtime operation failed"
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ContextFromPreparePayload(payload contracts.PrepareReleaseWorkspacePayload) (RuntimeContext, error) {
	if strings.TrimSpace(payload.Project.ProjectID) == "" {
		return RuntimeContext{}, fmt.Errorf("project.project_id is required")
	}
	if strings.TrimSpace(payload.Binding.BindingID) == "" {
		return RuntimeContext{}, fmt.Errorf("binding.binding_id is required")
	}
	if strings.TrimSpace(payload.Revision.RevisionID) == "" {
		return RuntimeContext{}, fmt.Errorf("revision.revision_id is required")
	}
	if payload.Binding.ProjectID != "" && payload.Binding.ProjectID != payload.Project.ProjectID {
		return RuntimeContext{}, fmt.Errorf("binding project ID does not match project metadata")
	}
	if payload.Revision.ProjectID != "" && payload.Revision.ProjectID != payload.Project.ProjectID {
		return RuntimeContext{}, fmt.Errorf("revision project ID does not match project metadata")
	}
	if payload.Binding.RuntimeMode != payload.Revision.RuntimeMode {
		return RuntimeContext{}, fmt.Errorf("binding runtime mode does not match revision runtime mode")
	}
	switch payload.Binding.RuntimeMode {
	case contracts.RuntimeModeStandalone, contracts.RuntimeModeDistributedMesh:
	default:
		return RuntimeContext{}, fmt.Errorf("runtime driver does not support runtime mode %q", payload.Binding.RuntimeMode)
	}
	if len(payload.Revision.Services) == 0 {
		return RuntimeContext{}, fmt.Errorf("revision must include at least one service")
	}

	serviceNames := make(map[string]struct{}, len(payload.Revision.Services))
	placementByService := make(map[string]contracts.PlacementAssignment, len(payload.Revision.PlacementAssignments))
	services := make([]ServiceRuntimeContext, 0, len(payload.Revision.Services))
	for _, placement := range payload.Revision.PlacementAssignments {
		placementByService[placement.ServiceName] = placement
	}
	for _, service := range payload.Revision.Services {
		if strings.TrimSpace(service.Name) == "" {
			return RuntimeContext{}, fmt.Errorf("service name is required")
		}
		if _, exists := serviceNames[service.Name]; exists {
			return RuntimeContext{}, fmt.Errorf("duplicate service name %q", service.Name)
		}
		serviceNames[service.Name] = struct{}{}

		ctxService := ServiceRuntimeContext{
			Name:           service.Name,
			Path:           service.Path,
			Public:         service.Public,
			RuntimeProfile: service.RuntimeProfile,
			StartHint:      service.StartHint,
			Labels:         service.Labels,
			HealthCheck:    service.HealthCheck,
		}

		for _, dependency := range payload.Revision.DependencyBindings {
			if dependency.Service == service.Name {
				ctxService.Dependencies = append(ctxService.Dependencies, dependency)
			}
		}
		if placement, ok := placementByService[service.Name]; ok {
			placementCopy := placement
			ctxService.Placement = &placementCopy
		}
		services = append(services, ctxService)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	serviceByName := make(map[string]ServiceRuntimeContext, len(services))
	for _, service := range services {
		serviceByName[service.Name] = service
	}

	return RuntimeContext{
		Project: ProjectMetadata{
			ProjectID: payload.Project.ProjectID,
			Name:      payload.Project.Name,
			Slug:      payload.Project.Slug,
			Labels:    payload.Project.Labels,
		},
		Binding:  payload.Binding,
		Revision: payload.Revision,
		Services: services,
		Runtime: RuntimeDependencyContext{
			PlacementByService:   placementByService,
			ServiceByName:        serviceByName,
			PlacementFingerprint: placementFingerprint(payload.Binding, services, placementByService),
		},
	}, nil
}

func workspaceLayout(root string, runtimeCtx RuntimeContext) WorkspaceLayout {
	workspaceRoot := filepath.Join(
		root,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
		"revisions",
		runtimeCtx.Revision.RevisionID,
	)
	return WorkspaceLayout{
		Root:      workspaceRoot,
		Artifacts: filepath.Join(workspaceRoot, "artifacts"),
		Config:    filepath.Join(workspaceRoot, "config"),
		Sidecars:  filepath.Join(workspaceRoot, "sidecars"),
		Gateway:   filepath.Join(workspaceRoot, "gateway"),
		Mesh:      filepath.Join(workspaceRoot, "mesh"),
		Services:  filepath.Join(workspaceRoot, "services"),
	}
}

func capabilityNoContainerLeak(manifest WorkspaceManifest) error {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(string(raw)), "container") {
		return fmt.Errorf("runtime workspace manifest must not leak container terminology")
	}
	return nil
}

func recordPendingRevision(local *state.AgentLocalState, revisionID string, now time.Time) {
	local.RevisionCache.PendingRevisionID = revisionID
	local.RevisionCache.UpdatedAt = now
}
