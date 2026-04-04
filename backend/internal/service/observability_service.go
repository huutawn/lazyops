package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	TraceStatusOK      = "ok"
	TraceStatusError   = "error"
	TraceStatusWarning = "warning"

	NodeKindInstance = "instance"
	NodeKindMesh     = "mesh_network"
	NodeKindCluster  = "cluster"
	NodeKindService  = "service"

	EdgeKindDependency = "dependency"
	EdgeKindMeshPeer   = "mesh_peer"
	EdgeKindRouting    = "routing"
)

var (
	ErrTraceNotFound = errors.New("trace not found")
)

type ObservabilityService struct {
	traces    TraceSummaryStore
	incidents RuntimeIncidentStore
	topoNodes TopologyNodeStore
	topoEdges TopologyEdgeStore
	instances InstanceStore
	meshes    MeshNetworkStore
	clusters  ClusterStore
}

type TraceSummaryStore interface {
	Create(trace *models.TraceSummary) error
	GetByCorrelationID(correlationID string) (*models.TraceSummary, error)
	ListByProject(projectID string, limit int) ([]models.TraceSummary, error)
}

type TopologyNodeStore interface {
	Upsert(node *models.TopologyNode) error
	ListByProject(projectID string) ([]models.TopologyNode, error)
	DeleteByProject(projectID string) error
}

type TopologyEdgeStore interface {
	Upsert(edge *models.TopologyEdge) error
	ListByProject(projectID string) ([]models.TopologyEdge, error)
	DeleteByProject(projectID string) error
}

func NewObservabilityService(
	traces TraceSummaryStore,
	incidents RuntimeIncidentStore,
	topoNodes TopologyNodeStore,
	topoEdges TopologyEdgeStore,
	instances InstanceStore,
	meshes MeshNetworkStore,
	clusters ClusterStore,
) *ObservabilityService {
	return &ObservabilityService{
		traces:    traces,
		incidents: incidents,
		topoNodes: topoNodes,
		topoEdges: topoEdges,
		instances: instances,
		meshes:    meshes,
		clusters:  clusters,
	}
}

func (s *ObservabilityService) IngestTraceSummary(ctx context.Context, cmd IngestTraceCommand) (*TraceRecord, error) {
	correlationID := strings.TrimSpace(cmd.CorrelationID)
	if correlationID == "" {
		return nil, ErrInvalidInput
	}

	status := normalizeTraceStatus(cmd.Status, cmd.HTTPStatusCode)

	metadata := cmd.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["x_correlation_id"] = correlationID
	metadataJSON, _ := json.Marshal(metadata)

	trace := &models.TraceSummary{
		ID:             utils.NewPrefixedID("trc"),
		CorrelationID:  correlationID,
		ProjectID:      strings.TrimSpace(cmd.ProjectID),
		ServiceName:    strings.TrimSpace(cmd.ServiceName),
		Operation:      strings.TrimSpace(cmd.Operation),
		HTTPMethod:     strings.TrimSpace(cmd.HTTPMethod),
		HTTPStatusCode: cmd.HTTPStatusCode,
		DurationMs:     cmd.DurationMs,
		Status:         status,
		ErrorSummary:   strings.TrimSpace(cmd.ErrorSummary),
		SpanCount:      cmd.SpanCount,
		MetadataJSON:   string(metadataJSON),
		ReceivedAt:     time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.traces.Create(trace); err != nil {
		return nil, err
	}

	return toTraceRecord(*trace), nil
}

func (s *ObservabilityService) GetTraceByCorrelationID(ctx context.Context, correlationID string) (*TraceRecord, error) {
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return nil, ErrInvalidInput
	}

	trace, err := s.traces.GetByCorrelationID(correlationID)
	if err != nil {
		return nil, err
	}
	if trace == nil {
		return nil, ErrTraceNotFound
	}

	return toTraceRecord(*trace), nil
}

func (s *ObservabilityService) ListTracesByProject(ctx context.Context, projectID string, limit int) ([]TraceRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	traces, err := s.traces.ListByProject(projectID, limit)
	if err != nil {
		return nil, err
	}

	out := make([]TraceRecord, len(traces))
	for i, trace := range traces {
		out[i] = *toTraceRecord(trace)
	}
	return out, nil
}

func (s *ObservabilityService) ListIncidentsByProject(ctx context.Context, projectID string) ([]IncidentRecord, error) {
	items, err := s.incidents.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	out := make([]IncidentRecord, len(items))
	for i, item := range items {
		r := toIncidentRecord(item)
		out[i] = *r
	}
	return out, nil
}

func (s *ObservabilityService) BuildTopologyGraph(ctx context.Context, projectID string) (*TopologyGraph, error) {
	nodes, err := s.topoNodes.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	edges, err := s.topoEdges.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		nodes, err = s.buildNodesFromTargets(ctx, projectID)
		if err != nil {
			return nil, err
		}
	}

	graph := &TopologyGraph{
		ProjectID: projectID,
		Nodes:     make([]TopologyNodeRecord, len(nodes)),
		Edges:     make([]TopologyEdgeRecord, len(edges)),
	}

	for i, node := range nodes {
		graph.Nodes[i] = toTopologyNodeRecord(node)
	}
	for i, edge := range edges {
		graph.Edges[i] = toTopologyEdgeRecord(edge)
	}

	return graph, nil
}

func (s *ObservabilityService) RefreshTopologyGraph(ctx context.Context, projectID string) (*TopologyGraph, error) {
	_ = s.topoNodes.DeleteByProject(projectID)
	_ = s.topoEdges.DeleteByProject(projectID)

	instances, err := s.instances.ListByUser("")
	if err != nil {
		return nil, err
	}

	for _, inst := range instances {
		node := &models.TopologyNode{
			ID:           utils.NewPrefixedID("tn"),
			ProjectID:    projectID,
			NodeKind:     NodeKindInstance,
			NodeRef:      inst.ID,
			Name:         inst.Name,
			Status:       normalizeTopologyNodeStatus(inst.Status),
			MetadataJSON: inst.LabelsJSON,
			UpdatedAt:    time.Now().UTC(),
			CreatedAt:    time.Now().UTC(),
		}
		_ = s.topoNodes.Upsert(node)
	}

	return s.BuildTopologyGraph(ctx, projectID)
}

func (s *ObservabilityService) buildNodesFromTargets(ctx context.Context, projectID string) ([]models.TopologyNode, error) {
	nodes := make([]models.TopologyNode, 0)

	instances, err := s.instances.ListByUser("")
	if err != nil {
		return nil, err
	}
	for _, inst := range instances {
		nodes = append(nodes, models.TopologyNode{
			ID:           utils.NewPrefixedID("tn"),
			ProjectID:    projectID,
			NodeKind:     NodeKindInstance,
			NodeRef:      inst.ID,
			Name:         inst.Name,
			Status:       normalizeTopologyNodeStatus(inst.Status),
			MetadataJSON: inst.LabelsJSON,
			UpdatedAt:    time.Now().UTC(),
			CreatedAt:    time.Now().UTC(),
		})
	}

	return nodes, nil
}

type IngestTraceCommand struct {
	CorrelationID  string
	ProjectID      string
	ServiceName    string
	Operation      string
	HTTPMethod     string
	HTTPStatusCode int
	DurationMs     float64
	Status         string
	ErrorSummary   string
	SpanCount      int
	Metadata       map[string]any
}

type TraceRecord struct {
	ID             string         `json:"id"`
	CorrelationID  string         `json:"correlation_id"`
	ProjectID      string         `json:"project_id"`
	ServiceName    string         `json:"service_name"`
	Operation      string         `json:"operation"`
	HTTPMethod     string         `json:"http_method"`
	HTTPStatusCode int            `json:"http_status_code"`
	DurationMs     float64        `json:"duration_ms"`
	Status         string         `json:"status"`
	ErrorSummary   string         `json:"error_summary,omitempty"`
	SpanCount      int            `json:"span_count"`
	Metadata       map[string]any `json:"metadata"`
	ReceivedAt     time.Time      `json:"received_at"`
}

type TopologyGraph struct {
	ProjectID string               `json:"project_id"`
	Nodes     []TopologyNodeRecord `json:"nodes"`
	Edges     []TopologyEdgeRecord `json:"edges"`
}

type TopologyNodeRecord struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	NodeKind  string         `json:"node_kind"`
	NodeRef   string         `json:"node_ref"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type TopologyEdgeRecord struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	SourceID  string         `json:"source_id"`
	TargetID  string         `json:"target_id"`
	EdgeKind  string         `json:"edge_kind"`
	Protocol  string         `json:"protocol"`
	Metadata  map[string]any `json:"metadata"`
}

func toTraceRecord(item models.TraceSummary) *TraceRecord {
	var metadata map[string]any
	if item.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(item.MetadataJSON), &metadata)
	}
	return &TraceRecord{
		ID:             item.ID,
		CorrelationID:  item.CorrelationID,
		ProjectID:      item.ProjectID,
		ServiceName:    item.ServiceName,
		Operation:      item.Operation,
		HTTPMethod:     item.HTTPMethod,
		HTTPStatusCode: item.HTTPStatusCode,
		DurationMs:     item.DurationMs,
		Status:         item.Status,
		ErrorSummary:   item.ErrorSummary,
		SpanCount:      item.SpanCount,
		Metadata:       metadata,
		ReceivedAt:     item.ReceivedAt,
	}
}

func toTopologyNodeRecord(item models.TopologyNode) TopologyNodeRecord {
	var metadata map[string]any
	if item.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(item.MetadataJSON), &metadata)
	}
	return TopologyNodeRecord{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		NodeKind:  item.NodeKind,
		NodeRef:   item.NodeRef,
		Name:      item.Name,
		Status:    item.Status,
		Metadata:  metadata,
		UpdatedAt: item.UpdatedAt,
	}
}

func toTopologyEdgeRecord(item models.TopologyEdge) TopologyEdgeRecord {
	var metadata map[string]any
	if item.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(item.MetadataJSON), &metadata)
	}
	return TopologyEdgeRecord{
		ID:        item.ID,
		ProjectID: item.ProjectID,
		SourceID:  item.SourceID,
		TargetID:  item.TargetID,
		EdgeKind:  item.EdgeKind,
		Protocol:  item.Protocol,
		Metadata:  metadata,
	}
}

func normalizeTraceStatus(raw string, httpStatus int) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status != "" {
		switch status {
		case "ok", "error", "warning":
			return status
		}
	}
	if httpStatus >= 500 {
		return TraceStatusError
	}
	if httpStatus >= 400 {
		return TraceStatusWarning
	}
	return TraceStatusOK
}

func normalizeTopologyNodeStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "online":
		return "online"
	case "busy":
		return "online"
	case "error":
		return "degraded"
	case "offline":
		return "offline"
	default:
		return "unknown"
	}
}
