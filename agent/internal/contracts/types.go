package contracts

import "time"

type RuntimeMode string

const (
	RuntimeModeStandalone      RuntimeMode = "standalone"
	RuntimeModeDistributedMesh RuntimeMode = "distributed-mesh"
	RuntimeModeDistributedK3s  RuntimeMode = "distributed-k3s"
)

type AgentKind string

const (
	AgentKindInstance AgentKind = "instance_agent"
	AgentKindNode     AgentKind = "node_agent"
)

type AgentState string

const (
	AgentStateBootstrap    AgentState = "bootstrap"
	AgentStateConnected    AgentState = "connected"
	AgentStateReconciling  AgentState = "reconciling"
	AgentStateDegraded     AgentState = "degraded"
	AgentStateSleeping     AgentState = "sleeping"
	AgentStateReporting    AgentState = "reporting"
	AgentStateDisconnected AgentState = "disconnected"
)

type EnvelopeSource string

const (
	EnvelopeSourceBackend EnvelopeSource = "backend"
	EnvelopeSourceAgent   EnvelopeSource = "agent"
	EnvelopeSourceGateway EnvelopeSource = "gateway"
	EnvelopeSourceSidecar EnvelopeSource = "sidecar"
)

type TargetKind string

const (
	TargetKindInstance TargetKind = "instance"
	TargetKindMesh     TargetKind = "mesh"
	TargetKindCluster  TargetKind = "cluster"
)

type MetricWindow string

const (
	MetricWindow1Min  MetricWindow = "1m"
	MetricWindow5Min  MetricWindow = "5m"
	MetricWindow15Min MetricWindow = "15m"
	MetricWindow1Hour MetricWindow = "1h"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type MachineInfo struct {
	Hostname string            `json:"hostname"`
	OS       string            `json:"os"`
	Arch     string            `json:"arch"`
	Kernel   string            `json:"kernel,omitempty"`
	IPs      []string          `json:"ips,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type MeshProvider string

const (
	MeshProviderWireGuard MeshProvider = "wireguard"
	MeshProviderTailscale MeshProvider = "tailscale"
)

type MetricAggregate struct {
	P95   float64 `json:"p95"`
	Max   float64 `json:"max"`
	Min   float64 `json:"min"`
	Avg   float64 `json:"avg"`
	Count int64   `json:"count"`
}

type Metadata struct {
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}
