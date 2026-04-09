package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
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
	logs      LogStreamStore
	topoNodes TopologyNodeStore
	topoEdges TopologyEdgeStore
	instances InstanceStore
	meshes    MeshNetworkStore
	clusters  ClusterStore
	bindings  DeploymentBindingStore
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
	logs LogStreamStore,
	topoNodes TopologyNodeStore,
	topoEdges TopologyEdgeStore,
	instances InstanceStore,
	meshes MeshNetworkStore,
	clusters ClusterStore,
) *ObservabilityService {
	return &ObservabilityService{
		traces:    traces,
		incidents: incidents,
		logs:      logs,
		topoNodes: topoNodes,
		topoEdges: topoEdges,
		instances: instances,
		meshes:    meshes,
		clusters:  clusters,
	}
}

func (s *ObservabilityService) WithBindingStore(bindings DeploymentBindingStore) *ObservabilityService {
	s.bindings = bindings
	return s
}

type TopologyNodeUpsertCommand struct {
	ProjectID string
	NodeRef   string
	NodeKind  string
	Name      string
	Status    string
	Metadata  string
}

type TopologyEdgeUpsertCommand struct {
	ProjectID string
	SourceID  string
	TargetID  string
	EdgeKind  string
	Protocol  string
	Metadata  string
}

func (s *ObservabilityService) UpsertTopologyNode(ctx context.Context, cmd TopologyNodeUpsertCommand) error {
	if s.topoNodes == nil {
		return nil
	}
	node := &models.TopologyNode{
		ProjectID:    cmd.ProjectID,
		NodeKind:     cmd.NodeKind,
		NodeRef:      cmd.NodeRef,
		Name:         cmd.Name,
		Status:       cmd.Status,
		MetadataJSON: cmd.Metadata,
	}
	return s.topoNodes.Upsert(node)
}

func (s *ObservabilityService) UpsertTopologyEdge(ctx context.Context, cmd TopologyEdgeUpsertCommand) error {
	if s.topoEdges == nil {
		return nil
	}
	edge := &models.TopologyEdge{
		ProjectID:    cmd.ProjectID,
		SourceID:     cmd.SourceID,
		TargetID:     cmd.TargetID,
		EdgeKind:     cmd.EdgeKind,
		Protocol:     cmd.Protocol,
		MetadataJSON: cmd.Metadata,
	}
	return s.topoEdges.Upsert(edge)
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

func (s *ObservabilityService) ListRecentLogs(
	ctx context.Context,
	projectID,
	serviceName,
	level,
	contains string,
	limit int,
) ([]LogLineRecord, error) {
	_ = ctx
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrInvalidInput
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	normalizedLevel, err := validatePreviewLevel(level)
	if err != nil {
		return nil, ErrInvalidInput
	}

	entries, err := s.logs.ListByQuery(models.LogStreamQuery{
		ProjectID:   projectID,
		ServiceName: strings.TrimSpace(serviceName),
		Level:       normalizedLevel,
		Contains:    strings.TrimSpace(contains),
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}

	lines := make([]LogLineRecord, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, LogLineRecord{
			ID:            entry.ID,
			Service:       entry.ServiceName,
			RevisionID:    entry.RevisionID,
			CorrelationID: entry.CorrelationID,
			Timestamp:     entry.OccurredAt,
			Level:         entry.Level,
			Message:       entry.Message,
			Node:          entry.Node,
		})
	}
	return lines, nil
}

type ServiceMetricSummary struct {
	Service      string  `json:"service"`
	CpuP95       float64 `json:"cpu_p95"`
	CpuMax       float64 `json:"cpu_max"`
	CpuMin       float64 `json:"cpu_min"`
	CpuAvg       float64 `json:"cpu_avg"`
	RamP95       float64 `json:"ram_p95"`
	RamMax       float64 `json:"ram_max"`
	RamMin       float64 `json:"ram_min"`
	RamAvg       float64 `json:"ram_avg"`
	RequestCount int64   `json:"request_count"`
	Period       string  `json:"period"`
}

func (s *ObservabilityService) BuildServiceMetricSummary(ctx context.Context, projectID string, limit int) ([]ServiceMetricSummary, error) {
	traces, err := s.ListTracesByProject(ctx, projectID, limit)
	if err != nil {
		return nil, err
	}

	type series struct {
		cpu []float64
		ram []float64
	}
	perService := make(map[string]*series)
	for _, trace := range traces {
		service := strings.TrimSpace(trace.ServiceName)
		if service == "" {
			service = "unknown"
		}
		track := perService[service]
		if track == nil {
			track = &series{
				cpu: make([]float64, 0, 8),
				ram: make([]float64, 0, 8),
			}
			perService[service] = track
		}

		cpuProxy := math.Max(0, math.Min(100, trace.DurationMs/10))
		ramProxy := math.Max(0, float64(trace.SpanCount)*64)
		track.cpu = append(track.cpu, cpuProxy)
		track.ram = append(track.ram, ramProxy)
	}

	metrics := make([]ServiceMetricSummary, 0, len(perService))
	for serviceName, values := range perService {
		cpuStats := summarizeSeries(values.cpu)
		ramStats := summarizeSeries(values.ram)
		metrics = append(metrics, ServiceMetricSummary{
			Service:      serviceName,
			CpuP95:       cpuStats.p95,
			CpuMax:       cpuStats.max,
			CpuMin:       cpuStats.min,
			CpuAvg:       cpuStats.avg,
			RamP95:       ramStats.p95,
			RamMax:       ramStats.max,
			RamMin:       ramStats.min,
			RamAvg:       ramStats.avg,
			RequestCount: int64(len(values.cpu)),
			Period:       "trace_recent",
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].RequestCount == metrics[j].RequestCount {
			return metrics[i].Service < metrics[j].Service
		}
		return metrics[i].RequestCount > metrics[j].RequestCount
	})
	return metrics, nil
}

func (s *ObservabilityService) IngestLogBatch(ctx context.Context, cmd IngestLogBatchCommand) (*LogBatchRecord, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	bindingID := strings.TrimSpace(cmd.BindingID)
	if projectID == "" || bindingID == "" || len(cmd.Entries) == 0 {
		return nil, ErrInvalidInput
	}

	collectedAt := cmd.CollectedAt
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}

	records := make([]models.LogStreamEntry, 0, len(cmd.Entries))
	serviceName := strings.TrimSpace(cmd.ServiceName)
	for _, entry := range cmd.Entries {
		message := normalizeLogMessage(entry.Message, entry.Excerpt)
		if message == "" {
			continue
		}

		labels := cloneStringMap(entry.Labels)
		resolvedService := normalizeLogServiceName(serviceName, entry.Source, labels)
		if resolvedService == "" {
			continue
		}

		labelsJSON, _ := json.Marshal(labels)
		occurredAt := entry.Timestamp
		if occurredAt.IsZero() {
			occurredAt = collectedAt
		}

		correlationID := strings.TrimSpace(cmd.CorrelationID)
		if correlationID == "" {
			correlationID = strings.TrimSpace(entry.CorrelationID)
		}

		records = append(records, models.LogStreamEntry{
			ID:            utils.NewPrefixedID("log"),
			ProjectID:     projectID,
			BindingID:     bindingID,
			RevisionID:    strings.TrimSpace(cmd.RevisionID),
			ServiceName:   resolvedService,
			Source:        normalizeLogSource(entry.Source, resolvedService),
			Level:         normalizeLogLevel(entry.Severity),
			Node:          normalizeLogNode(labels),
			CorrelationID: correlationID,
			Message:       message,
			Excerpt:       strings.TrimSpace(entry.Excerpt),
			LabelsJSON:    string(labelsJSON),
			OccurredAt:    occurredAt.UTC(),
			CollectedAt:   collectedAt.UTC(),
			CreatedAt:     time.Now().UTC(),
		})
	}

	if len(records) == 0 {
		return nil, ErrInvalidInput
	}
	if err := s.logs.CreateBatch(records); err != nil {
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].OccurredAt.Before(records[j].OccurredAt)
	})

	return &LogBatchRecord{
		ProjectID:   projectID,
		BindingID:   bindingID,
		RevisionID:  strings.TrimSpace(cmd.RevisionID),
		ServiceName: records[0].ServiceName,
		EntryCount:  len(records),
		CollectedAt: collectedAt.UTC(),
	}, nil
}

func (s *ObservabilityService) PreviewLogs(ctx context.Context, cmd PreviewLogsCommand) (*LogsStreamPreview, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	serviceName := strings.TrimSpace(cmd.ServiceName)
	if projectID == "" || serviceName == "" {
		return nil, ErrInvalidInput
	}

	limit := cmd.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	level, err := validatePreviewLevel(cmd.Level)
	if err != nil {
		return nil, ErrInvalidInput
	}

	query := models.LogStreamQuery{
		ProjectID:     projectID,
		ServiceName:   serviceName,
		Level:         level,
		Contains:      strings.TrimSpace(cmd.Contains),
		Node:          strings.TrimSpace(cmd.Node),
		CorrelationID: strings.TrimSpace(cmd.CorrelationID),
		Limit:         limit,
	}
	if cursor := strings.TrimSpace(cmd.Cursor); cursor != "" {
		beforeAt, beforeID, err := decodeLogCursor(cursor)
		if err != nil {
			return nil, ErrInvalidInput
		}
		query.BeforeOccurredAt = beforeAt
		query.BeforeID = beforeID
	}

	entries, err := s.logs.ListByQuery(query)
	if err != nil {
		return nil, err
	}

	preview := &LogsStreamPreview{
		Service: serviceName,
		Lines:   make([]LogLineRecord, 0, len(entries)),
	}
	for _, entry := range entries {
		preview.Lines = append(preview.Lines, LogLineRecord{
			Timestamp: entry.OccurredAt,
			Level:     entry.Level,
			Message:   entry.Message,
			Node:      entry.Node,
		})
	}
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		preview.Cursor = encodeLogCursor(last.OccurredAt, last.ID)
	}

	return preview, nil
}

type CorrelatedObservability struct {
	Trace    *TraceRecord    `json:"trace,omitempty"`
	Logs     []LogLineRecord `json:"logs"`
	Topology *TopologyGraph  `json:"topology,omitempty"`
}

func (s *ObservabilityService) GetCorrelatedObservability(ctx context.Context, projectID, correlationID string) (*CorrelatedObservability, error) {
	projectID = strings.TrimSpace(projectID)
	correlationID = strings.TrimSpace(correlationID)
	if projectID == "" || correlationID == "" {
		return nil, ErrInvalidInput
	}

	result := &CorrelatedObservability{
		Logs: make([]LogLineRecord, 0),
	}

	trace, err := s.GetTraceByCorrelationID(ctx, correlationID)
	if err == nil && trace != nil {
		result.Trace = trace
	}

	logQuery := models.LogStreamQuery{
		ProjectID:     projectID,
		CorrelationID: correlationID,
		Limit:         100,
	}
	logEntries, err := s.logs.ListByQuery(logQuery)
	if err == nil {
		for _, entry := range logEntries {
			result.Logs = append(result.Logs, LogLineRecord{
				Timestamp: entry.OccurredAt,
				Level:     entry.Level,
				Message:   entry.Message,
				Node:      entry.Node,
			})
		}
	}

	topology, err := s.BuildTopologyGraph(ctx, projectID)
	if err == nil && topology != nil {
		result.Topology = topology
	}

	return result, nil
}

type TraceLogsQuery struct {
	ProjectID     string
	CorrelationID string
	Limit         int
}

func (s *ObservabilityService) GetTraceLogs(ctx context.Context, cmd TraceLogsQuery) ([]LogLineRecord, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	correlationID := strings.TrimSpace(cmd.CorrelationID)
	if projectID == "" || correlationID == "" {
		return nil, ErrInvalidInput
	}

	limit := cmd.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	trace, err := s.GetTraceByCorrelationID(ctx, correlationID)
	if err != nil {
		return nil, err
	}
	if trace == nil {
		return nil, ErrTraceNotFound
	}

	serviceName := trace.ServiceName
	if serviceName == "" {
		return nil, ErrInvalidInput
	}

	logQuery := models.LogStreamQuery{
		ProjectID:     projectID,
		ServiceName:   serviceName,
		CorrelationID: correlationID,
		Limit:         limit,
	}
	entries, err := s.logs.ListByQuery(logQuery)
	if err != nil {
		return nil, err
	}

	lines := make([]LogLineRecord, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, LogLineRecord{
			Timestamp: entry.OccurredAt,
			Level:     entry.Level,
			Message:   entry.Message,
			Node:      entry.Node,
		})
	}
	return lines, nil
}

type TopologyNodeLogsQuery struct {
	ProjectID string
	NodeRef   string
	Limit     int
}

func (s *ObservabilityService) GetTopologyNodeLogs(ctx context.Context, cmd TopologyNodeLogsQuery) ([]LogLineRecord, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	nodeRef := strings.TrimSpace(cmd.NodeRef)
	if projectID == "" || nodeRef == "" {
		return nil, ErrInvalidInput
	}

	limit := cmd.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	nodes, err := s.topoNodes.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	var node *models.TopologyNode
	for i := range nodes {
		if nodes[i].NodeRef == nodeRef {
			node = &nodes[i]
			break
		}
	}
	if node == nil {
		return nil, fmt.Errorf("topology node %q not found in project %q", nodeRef, projectID)
	}

	logQuery := models.LogStreamQuery{
		ProjectID: projectID,
		Node:      nodeRef,
		Limit:     limit,
	}
	entries, err := s.logs.ListByQuery(logQuery)
	if err != nil {
		return nil, err
	}

	lines := make([]LogLineRecord, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, LogLineRecord{
			Timestamp: entry.OccurredAt,
			Level:     entry.Level,
			Message:   entry.Message,
			Node:      entry.Node,
		})
	}
	return lines, nil
}

type ObservabilityQuery struct {
	ProjectID     string
	CorrelationID string
	NodeID        string
	ServiceName   string
	Level         string
	Contains      string
	Limit         int
}

func (s *ObservabilityService) GetLogsByCorrelationID(ctx context.Context, projectID, correlationID string, limit int) ([]LogLineRecord, error) {
	return s.GetTraceLogs(ctx, TraceLogsQuery{
		ProjectID:     projectID,
		CorrelationID: correlationID,
		Limit:         limit,
	})
}

func (s *ObservabilityService) GetLogsByNode(ctx context.Context, projectID, nodeID string, limit int) ([]LogLineRecord, error) {
	return s.GetTopologyNodeLogs(ctx, TopologyNodeLogsQuery{
		ProjectID: projectID,
		NodeRef:   nodeID,
		Limit:     limit,
	})
}

func (s *ObservabilityService) QueryObservabilityData(ctx context.Context, q ObservabilityQuery) (map[string]any, error) {
	result := map[string]any{}

	if q.CorrelationID != "" {
		logs, err := s.GetLogsByCorrelationID(ctx, q.ProjectID, q.CorrelationID, q.Limit)
		if err == nil {
			result["logs"] = logs
			result["log_count"] = len(logs)
		}

		trace, err := s.GetTraceByCorrelationID(ctx, q.CorrelationID)
		if err == nil && trace != nil {
			result["trace"] = trace
		}
	}

	if q.NodeID != "" {
		logs, err := s.GetLogsByNode(ctx, q.ProjectID, q.NodeID, q.Limit)
		if err == nil {
			result["node_logs"] = logs
			result["node_log_count"] = len(logs)
		}
	}

	if q.ServiceName != "" || q.Level != "" || q.Contains != "" {
		logQuery := models.LogStreamQuery{
			ProjectID:     q.ProjectID,
			ServiceName:   q.ServiceName,
			Level:         q.Level,
			Contains:      q.Contains,
			CorrelationID: q.CorrelationID,
			Node:          q.NodeID,
			Limit:         q.Limit,
		}
		if logQuery.Limit <= 0 {
			logQuery.Limit = 50
		}
		entries, err := s.logs.ListByQuery(logQuery)
		if err == nil {
			lines := make([]LogLineRecord, 0, len(entries))
			for _, e := range entries {
				lines = append(lines, LogLineRecord{
					Timestamp: e.OccurredAt,
					Level:     e.Level,
					Message:   e.Message,
					Node:      e.Node,
				})
			}
			result["logs"] = lines
			result["log_count"] = len(lines)
		}
	}

	topology, err := s.BuildTopologyGraph(ctx, q.ProjectID)
	if err == nil && topology != nil {
		result["topology"] = topology
	}

	return result, nil
}

type ObservabilityQueryCommand struct {
	ProjectID string
	QueryType string
	Selectors map[string]string
	Limit     int
}

func (s *ObservabilityService) QueryObservability(ctx context.Context, cmd ObservabilityQueryCommand) (any, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	if projectID == "" {
		return nil, ErrInvalidInput
	}

	limit := cmd.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	switch cmd.QueryType {
	case "trace_logs":
		correlationID := cmd.Selectors["correlation_id"]
		serviceName := cmd.Selectors["service_name"]
		logQuery := models.LogStreamQuery{
			ProjectID:     projectID,
			ServiceName:   serviceName,
			CorrelationID: correlationID,
			Limit:         limit,
		}
		entries, err := s.logs.ListByQuery(logQuery)
		if err != nil {
			return nil, err
		}
		lines := make([]LogLineRecord, 0, len(entries))
		for _, entry := range entries {
			lines = append(lines, LogLineRecord{
				Timestamp: entry.OccurredAt,
				Level:     entry.Level,
				Message:   entry.Message,
				Node:      entry.Node,
			})
		}
		return lines, nil

	case "node_logs":
		nodeRef := cmd.Selectors["node_ref"]
		logQuery := models.LogStreamQuery{
			ProjectID: projectID,
			Node:      nodeRef,
			Limit:     limit,
		}
		entries, err := s.logs.ListByQuery(logQuery)
		if err != nil {
			return nil, err
		}
		lines := make([]LogLineRecord, 0, len(entries))
		for _, entry := range entries {
			lines = append(lines, LogLineRecord{
				Timestamp: entry.OccurredAt,
				Level:     entry.Level,
				Message:   entry.Message,
				Node:      entry.Node,
			})
		}
		return lines, nil

	case "service_traces":
		serviceName := cmd.Selectors["service_name"]
		traces, err := s.traces.ListByProject(projectID, limit)
		if err != nil {
			return nil, err
		}
		if serviceName != "" {
			filtered := make([]models.TraceSummary, 0)
			for _, t := range traces {
				if t.ServiceName == serviceName {
					filtered = append(filtered, t)
				}
			}
			traces = filtered
		}
		records := make([]*TraceRecord, 0, len(traces))
		for _, t := range traces {
			records = append(records, toTraceRecord(t))
		}
		return records, nil

	case "topology_graph":
		return s.BuildTopologyGraph(ctx, projectID)

	default:
		return nil, fmt.Errorf("unsupported query type %q", cmd.QueryType)
	}
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

func (s *ObservabilityService) BuildTopologyGraphForUser(ctx context.Context, projectID, userID string) (*TopologyGraph, error) {
	nodes, err := s.topoNodes.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	edges, err := s.topoEdges.ListByProject(projectID)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		nodes, err = s.buildNodesFromTargetsForUser(ctx, projectID, userID)
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

	s.createEdgesFromDependencyBindings(projectID)
	s.createEdgesFromMeshPeerState(projectID)

	return s.BuildTopologyGraph(ctx, projectID)
}

func (s *ObservabilityService) createEdgesFromDependencyBindings(projectID string) {
	if s.bindings == nil {
		return
	}

	bindings, err := s.bindings.ListByProject(projectID)
	if err != nil {
		return
	}

	for _, binding := range bindings {
		if binding.TargetKind != "instance" || binding.TargetID == "" {
			continue
		}
		if _, err := s.instances.GetByID(binding.TargetID); err != nil {
			continue
		}

		metadata, _ := json.Marshal(map[string]any{
			"binding_id":   binding.ID,
			"binding_name": binding.Name,
			"target_ref":   binding.TargetRef,
		})

		edge := &models.TopologyEdge{
			ID:           utils.NewPrefixedID("te"),
			ProjectID:    projectID,
			SourceID:     binding.TargetID,
			TargetID:     binding.TargetID,
			EdgeKind:     EdgeKindDependency,
			Protocol:     "http",
			MetadataJSON: string(metadata),
			CreatedAt:    time.Now().UTC(),
		}
		_ = s.topoEdges.Upsert(edge)
	}
}

func (s *ObservabilityService) createEdgesFromMeshPeerState(projectID string) {
	if s.meshes == nil {
		return
	}

	meshes, err := s.meshes.ListByUser("")
	if err != nil {
		return
	}

	for _, mesh := range meshes {
		metadata, _ := json.Marshal(map[string]any{
			"mesh_id":   mesh.ID,
			"mesh_name": mesh.Name,
			"provider":  mesh.Provider,
			"cidr":      mesh.CIDR,
			"status":    mesh.Status,
		})

		edge := &models.TopologyEdge{
			ID:           utils.NewPrefixedID("te"),
			ProjectID:    projectID,
			SourceID:     mesh.ID,
			TargetID:     mesh.ID,
			EdgeKind:     EdgeKindMeshPeer,
			Protocol:     "tcp",
			MetadataJSON: string(metadata),
			CreatedAt:    time.Now().UTC(),
		}
		_ = s.topoEdges.Upsert(edge)
	}
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

func (s *ObservabilityService) buildNodesFromTargetsForUser(ctx context.Context, projectID, userID string) ([]models.TopologyNode, error) {
	nodes := make([]models.TopologyNode, 0)

	instances, err := s.instances.ListByUser(userID)
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

type IngestLogBatchCommand struct {
	ProjectID     string
	BindingID     string
	RevisionID    string
	ServiceName   string
	CorrelationID string
	Entries       []LogBatchEntry
	CollectedAt   time.Time
}

type LogBatchEntry struct {
	Timestamp     time.Time
	Severity      string
	Source        string
	Message       string
	Excerpt       string
	Labels        map[string]string
	CorrelationID string
}

type LogBatchRecord struct {
	ProjectID   string    `json:"project_id"`
	BindingID   string    `json:"binding_id"`
	RevisionID  string    `json:"revision_id,omitempty"`
	ServiceName string    `json:"service_name"`
	EntryCount  int       `json:"entry_count"`
	CollectedAt time.Time `json:"collected_at"`
}

type PreviewLogsCommand struct {
	ProjectID     string
	ServiceName   string
	Level         string
	Contains      string
	Node          string
	CorrelationID string
	Cursor        string
	Limit         int
}

type LogsStreamPreview struct {
	Service string          `json:"service"`
	Cursor  string          `json:"cursor,omitempty"`
	Lines   []LogLineRecord `json:"lines"`
}

type LogLineRecord struct {
	ID            string    `json:"id,omitempty"`
	Service       string    `json:"service,omitempty"`
	RevisionID    string    `json:"revision_id,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	Level         string    `json:"level"`
	Message       string    `json:"message"`
	Node          string    `json:"node,omitempty"`
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

type seriesSummary struct {
	min float64
	max float64
	avg float64
	p95 float64
}

func summarizeSeries(values []float64) seriesSummary {
	if len(values) == 0 {
		return seriesSummary{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	var total float64
	for _, item := range sorted {
		total += item
	}

	index := int(math.Ceil(float64(len(sorted))*0.95)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return seriesSummary{
		min: sorted[0],
		max: sorted[len(sorted)-1],
		avg: total / float64(len(sorted)),
		p95: sorted[index],
	}
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

func normalizeLogLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "critical", "error":
		return "error"
	case "warning", "warn":
		return "warn"
	case "info":
		return "info"
	case "debug":
		return "debug"
	default:
		return "info"
	}
}

func normalizePreviewLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug", "info", "warn", "error":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func validatePreviewLevel(raw string) (string, error) {
	normalized := normalizePreviewLevel(raw)
	if strings.TrimSpace(raw) != "" && normalized == "" {
		return "", fmt.Errorf("invalid log level")
	}
	return normalized, nil
}

func normalizeLogServiceName(batchService, source string, labels map[string]string) string {
	for _, candidate := range []string{
		strings.TrimSpace(batchService),
		strings.TrimSpace(labels["service"]),
		strings.TrimSpace(labels["service_name"]),
		strings.TrimSpace(source),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func normalizeLogSource(source, serviceName string) string {
	if strings.TrimSpace(source) != "" {
		return strings.TrimSpace(source)
	}
	return serviceName
}

func normalizeLogNode(labels map[string]string) string {
	for _, key := range []string{"node", "node_name", "instance", "instance_id"} {
		if value := strings.TrimSpace(labels[key]); value != "" {
			return value
		}
	}
	return ""
}

func normalizeLogMessage(message, excerpt string) string {
	if strings.TrimSpace(message) != "" {
		return strings.TrimSpace(message)
	}
	return strings.TrimSpace(excerpt)
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		out[key] = value
	}
	return out
}

func encodeLogCursor(occurredAt time.Time, id string) string {
	raw := fmt.Sprintf("%d|%s", occurredAt.UTC().UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeLogCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", err
	}
	return time.Unix(0, nanos).UTC(), strings.TrimSpace(parts[1]), nil
}
