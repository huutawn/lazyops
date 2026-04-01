package response

import "time"

type InstanceSummaryResponse struct {
	ID                  string            `json:"id"`
	TargetKind          string            `json:"target_kind"`
	Name                string            `json:"name"`
	PublicIP            *string           `json:"public_ip,omitempty"`
	PrivateIP           *string           `json:"private_ip,omitempty"`
	AgentID             *string           `json:"agent_id,omitempty"`
	Status              string            `json:"status"`
	Labels              map[string]string `json:"labels"`
	RuntimeCapabilities map[string]any    `json:"runtime_capabilities"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type BootstrapTokenIssueResponse struct {
	Token     string    `json:"token"`
	TokenID   string    `json:"token_id"`
	ExpiresAt time.Time `json:"expires_at"`
	SingleUse bool      `json:"single_use"`
}

type CreateInstanceResponse struct {
	Instance  InstanceSummaryResponse     `json:"instance"`
	Bootstrap BootstrapTokenIssueResponse `json:"bootstrap"`
}

type InstanceListResponse struct {
	Items []InstanceSummaryResponse `json:"items"`
}
