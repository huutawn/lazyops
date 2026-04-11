package service

import "time"

type RoutingRouteRecord struct {
	Path        string    `json:"path"`
	Service     string    `json:"service"`
	Port        int       `json:"port,omitempty"`
	WebSocket   bool      `json:"websocket,omitempty"`
	StripPrefix bool      `json:"strip_prefix,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type RoutingPolicyRecord struct {
	SharedDomain string                 `json:"shared_domain,omitempty"`
	Routes       []RoutingRouteRecord   `json:"routes"`
}

type ProjectRoutingResult struct {
	RoutingPolicy     RoutingPolicyRecord `json:"routing_policy"`
	AvailableServices []string            `json:"available_services"`
}

type UpdateRoutingCommand struct {
	UserID       string
	Role         string
	ProjectID    string
	SharedDomain string
	Routes       []RoutingRouteRecord
}

func (c *UpdateRoutingCommand) IsValid() bool {
	return c.ProjectID != "" && c.UserID != ""
}
