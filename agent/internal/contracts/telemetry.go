package contracts

import "time"

type TraceSummaryPayload struct {
	ProjectID     string            `json:"project_id"`
	CorrelationID string            `json:"correlation_id"`
	StartedAt     time.Time         `json:"started_at"`
	EndedAt       time.Time         `json:"ended_at"`
	Hops          []TraceHopSummary `json:"hops"`
}

type TraceHopSummary struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Protocol    string  `json:"protocol"`
	LatencyMS   float64 `json:"latency_ms"`
	Status      string  `json:"status"`
	LocalSignal bool    `json:"local_signal"`
}

type IncidentPayload struct {
	ProjectID  string         `json:"project_id"`
	RevisionID string         `json:"revision_id,omitempty"`
	Severity   Severity       `json:"severity"`
	Kind       string         `json:"kind"`
	Summary    string         `json:"summary"`
	OccurredAt time.Time      `json:"occurred_at"`
	Details    map[string]any `json:"details,omitempty"`
}

type MetricRollupPayload struct {
	ProjectID   string          `json:"project_id"`
	TargetKind  TargetKind      `json:"target_kind"`
	TargetID    string          `json:"target_id"`
	ServiceName string          `json:"service_name,omitempty"`
	Window      MetricWindow    `json:"window"`
	CPU         MetricAggregate `json:"cpu"`
	RAM         MetricAggregate `json:"ram"`
	Latency     MetricAggregate `json:"latency,omitempty"`
}

type TopologyPayload struct {
	ProjectID  string         `json:"project_id"`
	SnapshotAt time.Time      `json:"snapshot_at"`
	Nodes      []TopologyNode `json:"nodes"`
	Edges      []TopologyEdge `json:"edges"`
}

type TopologyNode struct {
	NodeRef  string         `json:"node_ref"`
	NodeType string         `json:"node_type"`
	Status   string         `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type TopologyEdge struct {
	SourceNodeRef string          `json:"source_node_ref"`
	TargetNodeRef string          `json:"target_node_ref"`
	EdgeType      string          `json:"edge_type"`
	Status        string          `json:"status"`
	Latency       MetricAggregate `json:"latency,omitempty"`
}
