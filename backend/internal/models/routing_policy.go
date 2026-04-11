package models

import "time"

// RoutingPolicy stores the path-based routing configuration for a project.
// Each project has exactly one RoutingPolicy record.
type RoutingPolicy struct {
	ID           string    `json:"id" gorm:"primaryKey;size:64"`
	ProjectID    string    `json:"project_id" gorm:"size:64;not null;uniqueIndex:idx_routing_policies_project"`
	SharedDomain string    `json:"shared_domain" gorm:"size:512"`
	RoutesJSON   string    `json:"routes_json" gorm:"type:text;not null"` // JSON array of route records
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RoutingRoute is a single route entry within a RoutingPolicy.
type RoutingRoute struct {
	Path        string `json:"path"`
	Service     string `json:"service"`
	Port        int    `json:"port,omitempty"`
	WebSocket   bool   `json:"websocket,omitempty"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
}
