package response

import "time"

type AgentResponse struct {
	ID         string     `json:"id"`
	AgentID    string     `json:"agent_id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	LastSeenAt *time.Time `json:"last_seen_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type AgentListResponse struct {
	Items []AgentResponse `json:"items"`
}

type RealtimeMetaResponse struct {
	Source string    `json:"source"`
	At     time.Time `json:"at"`
}

type AgentRealtimeEventResponse struct {
	Type    string               `json:"type"`
	Payload AgentResponse        `json:"payload"`
	Meta    RealtimeMetaResponse `json:"meta"`
}
