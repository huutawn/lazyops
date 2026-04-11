package response

type RoutingRouteResponse struct {
	Path        string `json:"path"`
	Service     string `json:"service"`
	Port        int    `json:"port,omitempty"`
	WebSocket   bool   `json:"websocket,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
}

type RoutingPolicyResponse struct {
	SharedDomain      string                   `json:"shared_domain,omitempty"`
	Routes            []RoutingRouteResponse   `json:"routes"`
}

type ProjectRoutingResponse struct {
	RoutingPolicy      RoutingPolicyResponse `json:"routing_policy"`
	AvailableServices  []string              `json:"available_services"`
}
