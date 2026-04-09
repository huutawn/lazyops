package response

import "time"

type DesiredStateRevisionResponse struct {
	ID                   string                        `json:"id"`
	ProjectID            string                        `json:"project_id"`
	BlueprintID          string                        `json:"blueprint_id"`
	DeploymentBindingID  string                        `json:"deployment_binding_id"`
	CommitSHA            string                        `json:"commit_sha"`
	ArtifactRef          string                        `json:"artifact_ref,omitempty"`
	ImageRef             string                        `json:"image_ref,omitempty"`
	TriggerKind          string                        `json:"trigger_kind"`
	Status               string                        `json:"status"`
	RuntimeMode          string                        `json:"runtime_mode"`
	Services             []ProjectServiceResponse      `json:"services"`
	DependencyBindings   []map[string]any              `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy  map[string]any                `json:"compatibility_policy"`
	MagicDomainPolicy    map[string]any                `json:"magic_domain_policy"`
	ScaleToZeroPolicy    map[string]any                `json:"scale_to_zero_policy"`
	PlacementAssignments []PlacementAssignmentResponse `json:"placement_assignments,omitempty"`
	CreatedAt            time.Time                     `json:"created_at"`
	UpdatedAt            time.Time                     `json:"updated_at"`
}

type DeploymentResponse struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	RevisionID  string     `json:"revision_id"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateDeploymentResponse struct {
	Revision   DesiredStateRevisionResponse `json:"revision"`
	Deployment DeploymentResponse           `json:"deployment"`
}

type DeploymentTimelineEventResponse struct {
	Timestamp   time.Time `json:"timestamp"`
	State       string    `json:"state"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
}

type DeploymentOverviewResponse struct {
	ID                   string                        `json:"id"`
	ProjectID            string                        `json:"project_id"`
	RevisionID           string                        `json:"revision_id"`
	Revision             int                           `json:"revision"`
	CommitSHA            string                        `json:"commit_sha"`
	ArtifactRef          string                        `json:"artifact_ref,omitempty"`
	ImageRef             string                        `json:"image_ref,omitempty"`
	TriggerKind          string                        `json:"trigger_kind"`
	BuildState           string                        `json:"build_state"`
	RolloutState         string                        `json:"rollout_state"`
	Promoted             bool                          `json:"promoted"`
	TriggeredBy          string                        `json:"triggered_by"`
	RuntimeMode          string                        `json:"runtime_mode"`
	Services             []ProjectServiceResponse      `json:"services"`
	PlacementAssignments []PlacementAssignmentResponse `json:"placement_assignments"`
	StartedAt            *time.Time                    `json:"started_at,omitempty"`
	CompletedAt          *time.Time                    `json:"completed_at,omitempty"`
	CreatedAt            time.Time                     `json:"created_at"`
	UpdatedAt            time.Time                     `json:"updated_at"`
}

type DeploymentDetailResponse struct {
	DeploymentOverviewResponse
	Timeline        []DeploymentTimelineEventResponse  `json:"timeline"`
	CanRollback     bool                               `json:"can_rollback"`
	CanPromote      bool                               `json:"can_promote"`
	CanCancel       bool                               `json:"can_cancel"`
	SafetyPolicy    DeploymentSafetyPolicyResponse     `json:"safety_policy"`
	IncidentSummary *DeploymentIncidentSummaryResponse `json:"incident_summary,omitempty"`
}

type DeploymentSafetyPolicyResponse struct {
	AutoRollbackEnabled bool     `json:"auto_rollback_enabled"`
	Triggers            []string `json:"triggers"`
	Description         string   `json:"description"`
}

type DeploymentFixActionResponse struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Href   string `json:"href"`
	Method string `json:"method"`
}

type DeploymentIncidentSummaryResponse struct {
	State         string                       `json:"state"`
	Headline      string                       `json:"headline"`
	Reason        string                       `json:"reason"`
	Recommended   string                       `json:"recommended"`
	IncidentID    string                       `json:"incident_id,omitempty"`
	IncidentKind  string                       `json:"incident_kind,omitempty"`
	IncidentLevel string                       `json:"incident_level,omitempty"`
	PrimaryAction *DeploymentFixActionResponse `json:"primary_action,omitempty"`
}

type DeploymentListResponse struct {
	Items []DeploymentOverviewResponse `json:"items"`
}
