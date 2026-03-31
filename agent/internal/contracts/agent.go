package contracts

import "time"

type EnrollAgentRequest struct {
	BootstrapToken string                  `json:"bootstrap_token"`
	RuntimeMode    RuntimeMode             `json:"runtime_mode"`
	AgentKind      AgentKind               `json:"agent_kind"`
	Machine        MachineInfo             `json:"machine"`
	Capabilities   CapabilityReportPayload `json:"capabilities"`
}

type EnrollAgentResponse struct {
	AgentID    string    `json:"agent_id"`
	AgentToken string    `json:"agent_token"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

type HeartbeatPayload struct {
	AgentID       string                  `json:"agent_id"`
	SessionID     string                  `json:"session_id"`
	State         AgentState              `json:"state"`
	RuntimeMode   RuntimeMode             `json:"runtime_mode"`
	AgentKind     AgentKind               `json:"agent_kind"`
	SentAt        time.Time               `json:"sent_at"`
	UptimeSeconds int64                   `json:"uptime_seconds"`
	Capabilities  CapabilityReportPayload `json:"capabilities,omitempty"`
}

type CapabilityReportPayload struct {
	AgentKind              AgentKind                `json:"agent_kind"`
	RuntimeMode            RuntimeMode              `json:"runtime_mode"`
	ControlChannel         ControlChannelCapability `json:"control_channel"`
	Gateway                GatewayCapability        `json:"gateway"`
	Sidecar                SidecarCapability        `json:"sidecar"`
	Mesh                   MeshCapability           `json:"mesh"`
	Telemetry              TelemetryCapability      `json:"telemetry"`
	Node                   NodeCapability           `json:"node"`
	PerformanceTargets     PerformanceTargets       `json:"performance_targets"`
	AdditionalCapabilities map[string]bool          `json:"additional_capabilities,omitempty"`
}

type ControlChannelCapability struct {
	WebSocketPath string `json:"websocket_path"`
	OutboundOnly  bool   `json:"outbound_only"`
	Reconnectable bool   `json:"reconnectable"`
}

type GatewayCapability struct {
	Enabled      bool     `json:"enabled"`
	Provider     string   `json:"provider"`
	MagicDomains []string `json:"magic_domains,omitempty"`
	HTTPSManaged bool     `json:"https_managed"`
}

type SidecarCapability struct {
	Enabled                   bool     `json:"enabled"`
	Precedence                []string `json:"precedence"`
	SupportsHTTP              bool     `json:"supports_http"`
	SupportsTCP               bool     `json:"supports_tcp"`
	SupportsLocalhostRescue   bool     `json:"supports_localhost_rescue"`
	SupportsManagedCredential bool     `json:"supports_managed_credentials"`
}

type MeshCapability struct {
	Enabled                  bool           `json:"enabled"`
	DefaultProvider          MeshProvider   `json:"default_provider,omitempty"`
	SupportedProviders       []MeshProvider `json:"supported_providers,omitempty"`
	DeterministicPeerCleanup bool           `json:"deterministic_peer_cleanup"`
}

type TelemetryCapability struct {
	LogCollection     bool `json:"log_collection"`
	MetricRollup      bool `json:"metric_rollup"`
	TraceSummary      bool `json:"trace_summary"`
	TopologyReporting bool `json:"topology_reporting"`
	IncidentReporting bool `json:"incident_reporting"`
	TunnelRelay       bool `json:"tunnel_relay"`
}

type NodeCapability struct {
	K3sDetection        bool `json:"k3s_detection"`
	DaemonSetBootstrap  bool `json:"daemonset_bootstrap"`
	ContainerLogTailing bool `json:"container_log_tailing"`
	NodeMetrics         bool `json:"node_metrics"`
	PodTopology         bool `json:"pod_topology"`
}

type PerformanceTargets struct {
	IdleRAMMB       int     `json:"idle_ram_mb"`
	IdleCPUPercent  float64 `json:"idle_cpu_percent"`
	BufferPooling   bool    `json:"buffer_pooling"`
	LowAllocHotPath bool    `json:"low_alloc_hot_path"`
}
