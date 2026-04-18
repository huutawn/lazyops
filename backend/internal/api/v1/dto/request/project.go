package request

type CreateProjectRequest struct {
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	NamespaceSlug    string   `json:"namespace_slug,omitempty"`
	ClusterID        string   `json:"cluster_id,omitempty"`
	RuntimeMode      string   `json:"runtime_mode,omitempty"`
	DefaultBranch    string   `json:"default_branch"`
	InternalServices []string `json:"internal_services,omitempty"`
}

type ProjectServiceRequest struct {
	Name                    string            `json:"name"`
	Path                    string            `json:"path"`
	Kind                    string            `json:"kind,omitempty"`
	SourceType              string            `json:"source_type,omitempty"`
	Public                  bool              `json:"public"`
	RuntimeProfile          string            `json:"runtime_profile,omitempty"`
	PlacementMode           string            `json:"placement_mode,omitempty"`
	PlacementNodeID         string            `json:"placement_node_id,omitempty"`
	ConnectionTemplateKey   string            `json:"connection_template_key,omitempty"`
	ConnectionTargetService string            `json:"connection_target_service,omitempty"`
	ManagedByLazyops        bool              `json:"managed_by_lazyops,omitempty"`
	StartHint               string            `json:"start_hint,omitempty"`
	ImageRef                string            `json:"image_ref,omitempty"`
	ImageDigest             string            `json:"image_digest,omitempty"`
	TargetPort              int               `json:"target_port,omitempty"`
	ServicePort             int               `json:"service_port,omitempty"`
	Replicas                int               `json:"replicas,omitempty"`
	EnvBundle               map[string]string `json:"env_bundle,omitempty"`
	PVCSpec                 map[string]any    `json:"pvc_spec,omitempty"`
	DeployStrategy          map[string]any    `json:"deploy_strategy,omitempty"`
	Healthcheck             map[string]any    `json:"healthcheck,omitempty"`
}

type ConfigureProjectServicesRequest struct {
	Items []ProjectServiceRequest `json:"items"`
}

type ProjectServiceActionRequest struct {
	Action string `json:"action"`
}

type LinkProjectRepoRequest struct {
	GitHubInstallationID int64  `json:"github_installation_id"`
	GitHubRepoID         int64  `json:"github_repo_id"`
	TrackedBranch        string `json:"tracked_branch"`
	PreviewEnabled       bool   `json:"preview_enabled"`
}
