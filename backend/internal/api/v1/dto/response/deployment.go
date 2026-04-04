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
