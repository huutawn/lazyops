package response

type InitTargetSummaryResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	RuntimeMode string `json:"runtime_mode"`
}

type LazyopsYAMLSchemaResponse struct {
	AllowedDependencyProtocols  []string `json:"allowed_dependency_protocols"`
	AllowedMagicDomainProviders []string `json:"allowed_magic_domain_providers"`
	ForbiddenFieldNames         []string `json:"forbidden_field_names"`
}

type MigrationFindingResponse struct {
	Category         string `json:"category"`
	Severity         string `json:"severity"`
	ServiceName      string `json:"service_name,omitempty"`
	CurrentValue     string `json:"current_value,omitempty"`
	RecommendedValue string `json:"recommended_value,omitempty"`
	Message          string `json:"message"`
}

type ValidateLazyopsYAMLResponse struct {
	Project              ProjectSummaryResponse         `json:"project"`
	DeploymentBinding    DeploymentBindingResponse      `json:"deployment_binding"`
	TargetSummary        InitTargetSummaryResponse      `json:"target_summary"`
	Schema               LazyopsYAMLSchemaResponse      `json:"schema"`
	SuggestedRoutes      []RoutingGuidanceRouteResponse `json:"suggested_routes"`
	EffectivePublicPaths []RoutingGuidanceRouteResponse `json:"effective_public_paths"`
	MigrationFindings    []MigrationFindingResponse     `json:"migration_findings"`
}
