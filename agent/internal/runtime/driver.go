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
	RenderGatewayConfig(context.Context, RuntimeContext) (GatewayRenderResult, error)
	StartReleaseCandidate(context.Context, RuntimeContext) (CandidateRecord, error)
	RunHealthGate(context.Context, RuntimeContext) (HealthGateResult, error)
	PromoteRelease(context.Context, RuntimeContext) error
	RollbackRelease(context.Context, RuntimeContext) error
	SleepService(context.Context, RuntimeContext, string) error
	WakeService(context.Context, RuntimeContext, string) error
	GarbageCollectRuntime(context.Context, RuntimeContext) error
}

type RuntimeContext struct {
	Project  ProjectMetadata                    `json:"project"`
	Binding  contracts.DeploymentBindingPayload `json:"binding"`
	Revision contracts.DesiredRevisionPayload   `json:"revision"`
	Services []ServiceRuntimeContext            `json:"services"`
	Rollout  RolloutContext                     `json:"-"`
}

type RolloutContext struct {
	StableRevisionID string
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
	Services  string `json:"services"`
}

type PreparedWorkspace struct {
	Layout            WorkspaceLayout         `json:"layout"`
	ManifestPath      string                  `json:"manifest_path"`
	ServiceManifests  map[string]string       `json:"service_manifests"`
	Artifact          ArtifactMaterialization `json:"artifact"`
	SidecarConfigPath string                  `json:"sidecar_config_path"`
	GatewayConfigPath string                  `json:"gateway_config_path"`
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
	Version             string             `json:"version,omitempty"`
	GeneratedAt         time.Time          `json:"generated_at,omitempty"`
	Provider            string             `json:"provider"`
	PublicServices      []string           `json:"public_services"`
	MagicDomain         string             `json:"magic_domain_provider,omitempty"`
	FallbackMagicDomain string             `json:"fallback_magic_domain_provider,omitempty"`
	HostToken           string             `json:"host_token,omitempty"`
	Routes              []GatewayRoute     `json:"routes,omitempty"`
	Validation          *GatewayHookResult `json:"validation,omitempty"`
	Apply               *GatewayHookResult `json:"apply,omitempty"`
	Reload              *GatewayHookResult `json:"reload,omitempty"`
	Rollback            *GatewayHookResult `json:"rollback,omitempty"`
}

type GatewayRoute struct {
	ServiceName  string `json:"service_name"`
	Port         int    `json:"port"`
	Upstream     string `json:"upstream"`
	PrimaryHost  string `json:"primary_host"`
	FallbackHost string `json:"fallback_host"`
	PrimaryURL   string `json:"primary_url"`
	FallbackURL  string `json:"fallback_url"`
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
	EnabledServices []string                      `json:"enabled_services"`
	Compatibility   contracts.CompatibilityPolicy `json:"compatibility_policy"`
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
	services := make([]ServiceRuntimeContext, 0, len(payload.Revision.Services))
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
		for _, placement := range payload.Revision.PlacementAssignments {
			if placement.ServiceName == service.Name {
				placementCopy := placement
				ctxService.Placement = &placementCopy
				break
			}
		}
		services = append(services, ctxService)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

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
