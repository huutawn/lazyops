package response

import "time"

type ProjectServiceResponse struct {
	ID                      string                      `json:"id"`
	ProjectID               string                      `json:"project_id"`
	Name                    string                      `json:"name"`
	Path                    string                      `json:"path"`
	Kind                    string                      `json:"kind,omitempty"`
	SourceType              string                      `json:"source_type,omitempty"`
	Public                  bool                        `json:"public"`
	RuntimeProfile          string                      `json:"runtime_profile,omitempty"`
	PlacementMode           string                      `json:"placement_mode,omitempty"`
	PlacementNodeID         string                      `json:"placement_node_id,omitempty"`
	ConnectionTemplateKey   string                      `json:"connection_template_key,omitempty"`
	ConnectionTargetService string                      `json:"connection_target_service,omitempty"`
	ManagedByLazyops        bool                        `json:"managed_by_lazyops"`
	StartHint               string                      `json:"start_hint,omitempty"`
	ImageRef                string                      `json:"image_ref,omitempty"`
	ImageDigest             string                      `json:"image_digest,omitempty"`
	DetectedPorts           []BuildDetectedPortResponse `json:"detected_ports,omitempty"`
	TargetPort              int                         `json:"target_port,omitempty"`
	ServicePort             int                         `json:"service_port,omitempty"`
	Replicas                int                         `json:"replicas,omitempty"`
	EnvBundle               map[string]string           `json:"env_bundle,omitempty"`
	PVCSpec                 map[string]any              `json:"pvc_spec,omitempty"`
	DeployStrategy          map[string]any              `json:"deploy_strategy,omitempty"`
	Healthcheck             map[string]any              `json:"healthcheck"`
	CreatedAt               time.Time                   `json:"created_at,omitempty"`
	UpdatedAt               time.Time                   `json:"updated_at,omitempty"`
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
	ProjectSlug         string                            `json:"project_slug,omitempty"`
	Namespace           string                            `json:"namespace,omitempty"`
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
	Namespace            string                        `json:"namespace,omitempty"`
	CommitSHA            string                        `json:"commit_sha"`
	ArtifactRef          string                        `json:"artifact_ref,omitempty"`
	ImageRef             string                        `json:"image_ref,omitempty"`
	TriggerKind          string                        `json:"trigger_kind"`
	RuntimeMode          string                        `json:"runtime_mode"`
	Services             []ProjectServiceResponse      `json:"services"`
	ServiceSpecs         []ProjectServiceResponse      `json:"service_specs,omitempty"`
	DependencyBindings   []map[string]any              `json:"dependency_bindings,omitempty"`
	InternalBindings     []map[string]any              `json:"internal_bindings,omitempty"`
	CompatibilityPolicy  map[string]any                `json:"compatibility_policy"`
	MagicDomainPolicy    map[string]any                `json:"magic_domain_policy"`
	ScaleToZeroPolicy    map[string]any                `json:"scale_to_zero_policy"`
	PlacementAssignments []PlacementAssignmentResponse `json:"placement_assignments,omitempty"`
	ManifestBundle       map[string]any                `json:"manifest_bundle,omitempty"`
}

type CompileBlueprintResponse struct {
	Services             []ProjectServiceResponse     `json:"services"`
	Blueprint            BlueprintResponse            `json:"blueprint"`
	DesiredRevisionDraft DesiredRevisionDraftResponse `json:"desired_revision_draft"`
}
