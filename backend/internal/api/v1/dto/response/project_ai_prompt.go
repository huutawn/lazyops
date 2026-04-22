package response

type ProjectAIPromptServiceSnapshotResponse struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Role           string `json:"role"`
	RuntimeProfile string `json:"runtime_profile,omitempty"`
	SourceType     string `json:"source_type,omitempty"`
	Public         bool   `json:"public"`
	Managed        bool   `json:"managed"`
	WebSocket      bool   `json:"websocket,omitempty"`
	PublicPath     string `json:"public_path,omitempty"`
	InternalURL    string `json:"internal_url,omitempty"`
}

type ProjectAIPromptSourceSectionResponse struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ItemCount   int    `json:"item_count"`
}

type ProjectAIPromptResponse struct {
	Title                string                                   `json:"title"`
	Summary              string                                   `json:"summary"`
	Prompt               string                                   `json:"prompt"`
	ServiceSnapshot      []ProjectAIPromptServiceSnapshotResponse `json:"service_snapshot"`
	EffectivePublicPaths []RoutingGuidanceRouteResponse           `json:"effective_public_paths"`
	ManagedKeys          []string                                 `json:"managed_keys"`
	MigrationFindings    []MigrationFindingResponse               `json:"migration_findings"`
	SourceSections       []ProjectAIPromptSourceSectionResponse   `json:"source_sections"`
}
