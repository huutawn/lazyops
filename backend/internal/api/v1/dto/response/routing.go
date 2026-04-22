package response

type RoutingRouteResponse struct {
	Path        string `json:"path"`
	Service     string `json:"service"`
	Port        int    `json:"port,omitempty"`
	WebSocket   bool   `json:"websocket,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
}

type RoutingPolicyResponse struct {
	SharedDomain string                 `json:"shared_domain,omitempty"`
	Routes       []RoutingRouteResponse `json:"routes"`
}

type RoutingGuidanceRouteResponse struct {
	Path      string `json:"path"`
	Service   string `json:"service"`
	Audience  string `json:"audience,omitempty"`
	Source    string `json:"source,omitempty"`
	WebSocket bool   `json:"websocket,omitempty"`
}

type ProjectRoutingResponse struct {
	RoutingPolicy        RoutingPolicyResponse          `json:"routing_policy"`
	AvailableServices    []string                       `json:"available_services"`
	SuggestedRoutes      []RoutingGuidanceRouteResponse `json:"suggested_routes"`
	EffectivePublicPaths []RoutingGuidanceRouteResponse `json:"effective_public_paths"`
	Warnings             []string                       `json:"warnings"`
}
