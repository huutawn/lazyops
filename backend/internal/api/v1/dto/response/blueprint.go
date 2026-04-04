package response

import "time"

type ProjectServiceResponse struct {
	ID             string         `json:"id"`
	ProjectID      string         `json:"project_id"`
	Name           string         `json:"name"`
	Path           string         `json:"path"`
	Public         bool           `json:"public"`
	RuntimeProfile string         `json:"runtime_profile,omitempty"`
	StartHint      string         `json:"start_hint,omitempty"`
	Healthcheck    map[string]any `json:"healthcheck"`
	CreatedAt      time.Time      `json:"created_at,omitempty"`
	UpdatedAt      time.Time      `json:"updated_at,omitempty"`
}

type BlueprintArtifactMetadataResponse struct {
	CommitSHA   string `json:"commit_sha"`
	ArtifactRef string `json:"artifact_ref,omitempty"`
	ImageRef    string `json:"image_ref,omitempty"`
}

type BlueprintRepoStateResponse struct {
	ProjectRepoLinkID string `json:"project_repo_link_id"`
	RepoOwner         string `json:"repo_owner"`
	RepoName          string `json:"repo_name"`
	RepoFullName      string `json:"repo_full_name"`
	TrackedBranch     string `json:"tracked_branch"`
	PreviewEnabled    bool   `json:"preview_enabled"`
}

type PlacementAssignmentResponse struct {
	ServiceName string            `json:"service_name"`
	TargetID    string            `json:"target_id"`
	TargetKind  string            `json:"target_kind"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type BlueprintCompiledResponse struct {
	ProjectID           string                            `json:"project_id"`
	RuntimeMode         string                            `json:"runtime_mode"`
	Repo                BlueprintRepoStateResponse        `json:"repo"`
	Binding             DeploymentBindingResponse         `json:"binding"`
	Services            []ProjectServiceResponse          `json:"services"`
	DependencyBindings  []map[string]any                  `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy map[string]any                    `json:"compatibility_policy"`
	MagicDomainPolicy   map[string]any                    `json:"magic_domain_policy"`
	ScaleToZeroPolicy   map[string]any                    `json:"scale_to_zero_policy"`
	ArtifactMetadata    BlueprintArtifactMetadataResponse `json:"artifact_metadata"`
}

type BlueprintResponse struct {
	ID         string                    `json:"id"`
	ProjectID  string                    `json:"project_id"`
	SourceKind string                    `json:"source_kind"`
	SourceRef  string                    `json:"source_ref"`
	Compiled   BlueprintCompiledResponse `json:"compiled"`
	CreatedAt  time.Time                 `json:"created_at"`
}

type DesiredRevisionDraftResponse struct {
	RevisionID           string                        `json:"revision_id"`
	ProjectID            string                        `json:"project_id"`
	BlueprintID          string                        `json:"blueprint_id"`
	DeploymentBindingID  string                        `json:"deployment_binding_id"`
	CommitSHA            string                        `json:"commit_sha"`
	ArtifactRef          string                        `json:"artifact_ref,omitempty"`
	ImageRef             string                        `json:"image_ref,omitempty"`
	TriggerKind          string                        `json:"trigger_kind"`
	RuntimeMode          string                        `json:"runtime_mode"`
	Services             []ProjectServiceResponse      `json:"services"`
	DependencyBindings   []map[string]any              `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy  map[string]any                `json:"compatibility_policy"`
	MagicDomainPolicy    map[string]any                `json:"magic_domain_policy"`
	ScaleToZeroPolicy    map[string]any                `json:"scale_to_zero_policy"`
	PlacementAssignments []PlacementAssignmentResponse `json:"placement_assignments,omitempty"`
}

type CompileBlueprintResponse struct {
	Services             []ProjectServiceResponse     `json:"services"`
	Blueprint            BlueprintResponse            `json:"blueprint"`
	DesiredRevisionDraft DesiredRevisionDraftResponse `json:"desired_revision_draft"`
}
