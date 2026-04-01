package request

import "time"

type AgentMachineInfoRequest struct {
	Hostname string            `json:"hostname"`
	OS       string            `json:"os"`
	Arch     string            `json:"arch"`
	Kernel   string            `json:"kernel"`
	IPs      []string          `json:"ips"`
	Labels   map[string]string `json:"labels"`
}

type EnrollAgentRequest struct {
	BootstrapToken string                  `json:"bootstrap_token"`
	RuntimeMode    string                  `json:"runtime_mode"`
	AgentKind      string                  `json:"agent_kind"`
	Machine        AgentMachineInfoRequest `json:"machine"`
	Capabilities   map[string]any          `json:"capabilities"`
}

type AgentHeartbeatRequest struct {
	AgentID          string         `json:"agent_id"`
	SessionID        string         `json:"session_id"`
	State            string         `json:"state"`
	HealthStatus     string         `json:"health_status"`
	HealthSummary    string         `json:"health_summary"`
	RuntimeMode      string         `json:"runtime_mode"`
	AgentKind        string         `json:"agent_kind"`
	SentAt           time.Time      `json:"sent_at"`
	UptimeSeconds    int64          `json:"uptime_seconds"`
	CapabilityHash   string         `json:"capability_hash"`
	CapabilityUpdate map[string]any `json:"capability_update"`
	Capabilities     map[string]any `json:"capabilities"`
}
