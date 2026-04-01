package response

import "time"

type AgentEnrollmentResponse struct {
	AgentID    string     `json:"agent_id"`
	AgentToken string     `json:"agent_token"`
	InstanceID string     `json:"instance_id"`
	IssuedAt   time.Time  `json:"issued_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type AgentHeartbeatResponse struct {
	AgentID        string    `json:"agent_id"`
	InstanceID     string    `json:"instance_id"`
	AgentStatus    string    `json:"agent_status"`
	InstanceStatus string    `json:"instance_status"`
	ReceivedAt     time.Time `json:"received_at"`
}
