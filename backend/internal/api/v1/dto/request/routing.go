package request

type RoutingRouteRequest struct {
	Path        string `json:"path" binding:"required"`
	Service     string `json:"service" binding:"required"`
	Port        int    `json:"port,omitempty"`
	WebSocket   bool   `json:"websocket,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
}

type UpdateRoutingPolicyRequest struct {
	SharedDomain string                  `json:"shared_domain,omitempty"`
	Routes       []RoutingRouteRequest   `json:"routes"`
}
