package contracts

type DesiredRevisionPayload struct {
	RevisionID           string                     `json:"revision_id"`
	ProjectID            string                     `json:"project_id"`
	BlueprintID          string                     `json:"blueprint_id"`
	DeploymentBindingID  string                     `json:"deployment_binding_id"`
	CommitSHA            string                     `json:"commit_sha"`
	ArtifactRef          string                     `json:"artifact_ref,omitempty"`
	ImageRef             string                     `json:"image_ref,omitempty"`
	TriggerKind          string                     `json:"trigger_kind"`
	RuntimeMode          RuntimeMode                `json:"runtime_mode"`
	Services             []ServicePayload           `json:"services"`
	DependencyBindings   []DependencyBindingPayload `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy  CompatibilityPolicy        `json:"compatibility_policy"`
	MagicDomainPolicy    MagicDomainPolicy          `json:"magic_domain_policy"`
	ScaleToZeroPolicy    ScaleToZeroPolicy          `json:"scale_to_zero_policy"`
	RoutingPolicy        RoutingPolicyPayload       `json:"routing_policy,omitempty"`
	PlacementAssignments []PlacementAssignment      `json:"placement_assignments,omitempty"`
}

type ProjectMetadataPayload struct {
	ProjectID string            `json:"project_id"`
	Name      string            `json:"name,omitempty"`
	Slug      string            `json:"slug,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type PrepareReleaseWorkspacePayload struct {
	Project  ProjectMetadataPayload   `json:"project"`
	Binding  DeploymentBindingPayload `json:"binding"`
	Revision DesiredRevisionPayload   `json:"revision"`
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
	Name           string             `json:"name"`
	Path           string             `json:"path"`
	Public         bool               `json:"public"`
	RuntimeProfile string             `json:"runtime_profile,omitempty"`
	StartHint      string             `json:"start_hint,omitempty"`
	HealthCheck    HealthCheckPayload `json:"healthcheck"`
	Labels         map[string]string  `json:"labels,omitempty"`
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
