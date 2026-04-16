package contracts

type DesiredRevisionPayload struct {
	RevisionID           string                     `json:"revision_id"`
	ProjectID            string                     `json:"project_id"`
	ProjectSlug          string                     `json:"project_slug,omitempty"`
	Namespace            string                     `json:"namespace,omitempty"`
	BlueprintID          string                     `json:"blueprint_id"`
	DeploymentBindingID  string                     `json:"deployment_binding_id"`
	CommitSHA            string                     `json:"commit_sha"`
	ArtifactRef          string                     `json:"artifact_ref,omitempty"`
	ImageRef             string                     `json:"image_ref,omitempty"`
	TriggerKind          string                     `json:"trigger_kind"`
	RuntimeMode          RuntimeMode                `json:"runtime_mode"`
	Services             []ServicePayload           `json:"services"`
	ServiceSpecs         []K3sServiceSpecPayload    `json:"service_specs,omitempty"`
	DependencyBindings   []DependencyBindingPayload `json:"dependency_bindings,omitempty"`
	InternalBindings     []InternalBindingPayload   `json:"internal_bindings,omitempty"`
	CompatibilityPolicy  CompatibilityPolicy        `json:"compatibility_policy"`
	MagicDomainPolicy    MagicDomainPolicy          `json:"magic_domain_policy"`
	ScaleToZeroPolicy    ScaleToZeroPolicy          `json:"scale_to_zero_policy"`
	RoutingPolicy        RoutingPolicyPayload       `json:"routing_policy,omitempty"`
	ManifestBundle       ManifestBundlePayload      `json:"manifest_bundle,omitempty"`
	PublicDomains        []PublicDomainPayload      `json:"public_domains,omitempty"`
	PlacementAssignments []PlacementAssignment      `json:"placement_assignments,omitempty"`
}

type PublicDomainPayload struct {
	ServiceName  string `json:"service_name"`
	PrimaryHost  string `json:"primary_host"`
	FallbackHost string `json:"fallback_host"`
	PrimaryURL   string `json:"primary_url"`
	FallbackURL  string `json:"fallback_url"`
}

type ProjectMetadataPayload struct {
	ProjectID string            `json:"project_id"`
	Name      string            `json:"name,omitempty"`
	Slug      string            `json:"slug,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type PrepareReleaseWorkspacePayload struct {
	Project    ProjectMetadataPayload   `json:"project"`
	Binding    DeploymentBindingPayload `json:"binding"`
	Revision   DesiredRevisionPayload   `json:"revision"`
	ProjectEnv map[string]string        `json:"project_env,omitempty"`
}

type DeploymentBindingPayload struct {
	BindingID           string              `json:"binding_id"`
	ProjectID           string              `json:"project_id"`
	Name                string              `json:"name"`
	TargetRef           string              `json:"target_ref"`
	RuntimeMode         RuntimeMode         `json:"runtime_mode"`
	TargetKind          TargetKind          `json:"target_kind"`
	TargetID            string              `json:"target_id"`
	PlacementPolicy     PlacementPolicy     `json:"placement_policy"`
	DomainPolicy        DomainPolicy        `json:"domain_policy"`
	CompatibilityPolicy CompatibilityPolicy `json:"compatibility_policy"`
	ScaleToZeroPolicy   ScaleToZeroPolicy   `json:"scale_to_zero_policy"`
}

type PlacementPolicy struct {
	Strategy string            `json:"strategy"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type DomainPolicy struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
}

type CompatibilityPolicy struct {
	EnvInjection       bool `json:"env_injection"`
	ManagedCredentials bool `json:"managed_credentials"`
	LocalhostRescue    bool `json:"localhost_rescue"`
	TransparentProxy   bool `json:"transparent_proxy"`
}

type RoutingPolicyPayload struct {
	SharedDomain string         `json:"shared_domain,omitempty"`
	Routes       []RoutePayload `json:"routes,omitempty"`
}

type RoutePayload struct {
	Path        string `json:"path"`
	Service     string `json:"service"`
	Port        int    `json:"port,omitempty"`
	WebSocket   bool   `json:"websocket,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
}

type MagicDomainPolicy struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
}

type ScaleToZeroPolicy struct {
	Enabled            bool   `json:"enabled"`
	IdleWindow         string `json:"idle_window,omitempty"`
	GatewayHoldTimeout string `json:"gateway_hold_timeout,omitempty"`
}

type ServicePayload struct {
	Name           string                `json:"name"`
	Path           string                `json:"path"`
	Kind           string                `json:"kind,omitempty"`
	Public         bool                  `json:"public"`
	RuntimeProfile string                `json:"runtime_profile,omitempty"`
	StartHint      string                `json:"start_hint,omitempty"`
	ImageRef       string                `json:"image_ref,omitempty"`
	ImageDigest    string                `json:"image_digest,omitempty"`
	DetectedPorts  []DetectedPortPayload `json:"detected_ports,omitempty"`
	TargetPort     int                   `json:"target_port,omitempty"`
	ServicePort    int                   `json:"service_port,omitempty"`
	Replicas       int                   `json:"replicas,omitempty"`
	EnvBundle      map[string]string     `json:"env_bundle,omitempty"`
	PVCSpec        map[string]any        `json:"pvc_spec,omitempty"`
	DeployStrategy map[string]any        `json:"deploy_strategy,omitempty"`
	HealthCheck    HealthCheckPayload    `json:"healthcheck"`
	Labels         map[string]string     `json:"labels,omitempty"`
}

type DetectedPortPayload struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
	Exposed  bool   `json:"exposed,omitempty"`
}

type K3sServiceSpecPayload struct {
	Name           string                `json:"name"`
	Kind           string                `json:"kind,omitempty"`
	Namespace      string                `json:"namespace,omitempty"`
	Path           string                `json:"path,omitempty"`
	Public         bool                  `json:"public"`
	RuntimeProfile string                `json:"runtime_profile,omitempty"`
	StartHint      string                `json:"start_hint,omitempty"`
	ImageRef       string                `json:"image_ref,omitempty"`
	ImageDigest    string                `json:"image_digest,omitempty"`
	DetectedPorts  []DetectedPortPayload `json:"detected_ports,omitempty"`
	TargetPort     int                   `json:"target_port,omitempty"`
	ServicePort    int                   `json:"service_port,omitempty"`
	Replicas       int                   `json:"replicas,omitempty"`
	EnvBundle      map[string]string     `json:"env_bundle,omitempty"`
	PVCSpec        map[string]any        `json:"pvc_spec,omitempty"`
	DeployStrategy map[string]any        `json:"deploy_strategy,omitempty"`
	HealthCheck    HealthCheckPayload    `json:"healthcheck"`
}

type InternalBindingPayload struct {
	ServiceName      string `json:"service_name"`
	Alias            string `json:"alias"`
	TargetService    string `json:"target_service"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Protocol         string `json:"protocol"`
	URL              string `json:"url,omitempty"`
	ConnectionString string `json:"connection_string,omitempty"`
}

type ManifestDocumentPayload struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content"`
}

type ManifestBundlePayload struct {
	Namespace    string                    `json:"namespace,omitempty"`
	CombinedYAML string                    `json:"combined_yaml,omitempty"`
	RollbackYAML string                    `json:"rollback_yaml,omitempty"`
	Documents    []ManifestDocumentPayload `json:"documents,omitempty"`
}

type HealthCheckPayload struct {
	Path               string `json:"path,omitempty"`
	Port               int    `json:"port"`
	Protocol           string `json:"protocol"`
	Timeout            string `json:"timeout,omitempty"`
	SuccessThreshold   int    `json:"success_threshold,omitempty"`
	FailureThreshold   int    `json:"failure_threshold,omitempty"`
	StartupGracePeriod string `json:"startup_grace_period,omitempty"`
}

type DependencyBindingPayload struct {
	Service       string `json:"service"`
	Alias         string `json:"alias"`
	TargetService string `json:"target_service"`
	Protocol      string `json:"protocol"`
	LocalEndpoint string `json:"local_endpoint"`
}

type PlacementAssignment struct {
	ServiceName string            `json:"service_name"`
	TargetID    string            `json:"target_id"`
	TargetKind  TargetKind        `json:"target_kind"`
	Labels      map[string]string `json:"labels,omitempty"`
}
