package service

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeTraceSummaryStore struct {
	items []models.TraceSummary
}

func newFakeTraceSummaryStore(items ...models.TraceSummary) *fakeTraceSummaryStore {
	return &fakeTraceSummaryStore{items: items}
}

func (f *fakeTraceSummaryStore) Create(trace *models.TraceSummary) error {
	f.items = append(f.items, *trace)
	return nil
}

func (f *fakeTraceSummaryStore) GetByCorrelationID(correlationID string) (*models.TraceSummary, error) {
	for _, item := range f.items {
		if item.CorrelationID == correlationID {
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakeTraceSummaryStore) ListByProject(projectID string, limit int) ([]models.TraceSummary, error) {
	out := make([]models.TraceSummary, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type fakeTopologyNodeStore struct {
	items []models.TopologyNode
}

func newFakeTopologyNodeStore(items ...models.TopologyNode) *fakeTopologyNodeStore {
	return &fakeTopologyNodeStore{items: items}
}

func (f *fakeTopologyNodeStore) Upsert(node *models.TopologyNode) error {
	for i, item := range f.items {
		if item.ID == node.ID {
			f.items[i] = *node
			return nil
		}
	}
	f.items = append(f.items, *node)
	return nil
}

func (f *fakeTopologyNodeStore) ListByProject(projectID string) ([]models.TopologyNode, error) {
	out := make([]models.TopologyNode, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeTopologyNodeStore) DeleteByProject(projectID string) error {
	out := make([]models.TopologyNode, 0)
	for _, item := range f.items {
		if item.ProjectID != projectID {
			out = append(out, item)
		}
	}
	f.items = out
	return nil
}

type fakeTopologyEdgeStore struct {
	items []models.TopologyEdge
}

func newFakeTopologyEdgeStore(items ...models.TopologyEdge) *fakeTopologyEdgeStore {
	return &fakeTopologyEdgeStore{items: items}
}

func (f *fakeTopologyEdgeStore) Upsert(edge *models.TopologyEdge) error {
	for i, item := range f.items {
		if item.ID == edge.ID {
			f.items[i] = *edge
			return nil
		}
	}
	f.items = append(f.items, *edge)
	return nil
}

func (f *fakeTopologyEdgeStore) ListByProject(projectID string) ([]models.TopologyEdge, error) {
	out := make([]models.TopologyEdge, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeTopologyEdgeStore) DeleteByProject(projectID string) error {
	out := make([]models.TopologyEdge, 0)
	for _, item := range f.items {
		if item.ProjectID != projectID {
			out = append(out, item)
		}
	}
	f.items = out
	return nil
}

type fakeLogStreamStore struct {
	items []models.LogStreamEntry
}

func newFakeLogStreamStore(items ...models.LogStreamEntry) *fakeLogStreamStore {
	return &fakeLogStreamStore{items: items}
}

func (f *fakeLogStreamStore) CreateBatch(entries []models.LogStreamEntry) error {
	f.items = append(f.items, entries...)
	return nil
}

func (f *fakeLogStreamStore) ListByQuery(query models.LogStreamQuery) ([]models.LogStreamEntry, error) {
	out := make([]models.LogStreamEntry, 0, len(f.items))
	for _, item := range f.items {
		if item.ProjectID != query.ProjectID {
			continue
		}
		if query.ServiceName != "" && item.ServiceName != query.ServiceName {
			continue
		}
		if query.Source != "" && item.Source != query.Source {
			continue
		}
		if query.Level != "" && item.Level != query.Level {
			continue
		}
		if query.Node != "" && item.Node != query.Node {
			continue
		}
		if query.Contains != "" && !strings.Contains(strings.ToLower(item.Message), strings.ToLower(query.Contains)) {
			continue
		}
		if !query.BeforeOccurredAt.IsZero() && query.BeforeID != "" {
			if item.OccurredAt.After(query.BeforeOccurredAt) {
				continue
			}
			if item.OccurredAt.Equal(query.BeforeOccurredAt) && item.ID >= query.BeforeID {
				continue
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OccurredAt.Equal(out[j].OccurredAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	if query.Limit > 0 && len(out) > query.Limit {
		out = out[:query.Limit]
	}
	return out, nil
}

func newTestObservabilityService(
	traceStore TraceSummaryStore,
	incidentStore RuntimeIncidentStore,
	logStore LogStreamStore,
	nodeStore TopologyNodeStore,
	edgeStore TopologyEdgeStore,
	instanceStore InstanceStore,
	meshStore MeshNetworkStore,
	clusterStore ClusterStore,
) *ObservabilityService {
	return NewObservabilityService(traceStore, incidentStore, logStore, nodeStore, edgeStore, instanceStore, meshStore, clusterStore)
}

func TestObservabilityServiceIngestTraceSuccess(t *testing.T) {
	traceStore := newFakeTraceSummaryStore()

	svc := newTestObservabilityService(
		traceStore,
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	record, err := svc.IngestTraceSummary(context.Background(), IngestTraceCommand{
		CorrelationID:  "corr_abc123",
		ProjectID:      "prj_123",
		ServiceName:    "api",
		Operation:      "GET /users",
		HTTPMethod:     "GET",
		HTTPStatusCode: 200,
		DurationMs:     45.2,
		Status:         "ok",
		SpanCount:      3,
		Metadata:       map[string]any{"region": "us-east"},
	})
	if err != nil {
		t.Fatalf("ingest trace: %v", err)
	}

	if record.CorrelationID != "corr_abc123" {
		t.Fatalf("expected correlation id corr_abc123, got %q", record.CorrelationID)
	}
	if record.Status != TraceStatusOK {
		t.Fatalf("expected status ok, got %q", record.Status)
	}
	if record.DurationMs != 45.2 {
		t.Fatalf("expected duration 45.2, got %f", record.DurationMs)
	}
	if record.Metadata["region"] != "us-east" {
		t.Fatalf("expected metadata region us-east, got %v", record.Metadata["region"])
	}
}

func TestObservabilityServiceGetTraceByCorrelationID(t *testing.T) {
	traceStore := newFakeTraceSummaryStore(models.TraceSummary{
		ID:             "trc_123",
		CorrelationID:  "corr_abc123",
		ProjectID:      "prj_123",
		ServiceName:    "api",
		Operation:      "GET /users",
		HTTPMethod:     "GET",
		HTTPStatusCode: 200,
		DurationMs:     45.2,
		Status:         TraceStatusOK,
		SpanCount:      3,
		MetadataJSON:   `{"region":"us-east"}`,
		ReceivedAt:     time.Now().UTC(),
	})

	svc := newTestObservabilityService(
		traceStore,
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	record, err := svc.GetTraceByCorrelationID(context.Background(), "corr_abc123")
	if err != nil {
		t.Fatalf("get trace: %v", err)
	}

	if record.ServiceName != "api" {
		t.Fatalf("expected service name api, got %q", record.ServiceName)
	}
	if record.HTTPStatusCode != 200 {
		t.Fatalf("expected status code 200, got %d", record.HTTPStatusCode)
	}
}

func TestObservabilityServiceGetTraceNotFound(t *testing.T) {
	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	_, err := svc.GetTraceByCorrelationID(context.Background(), "corr_missing")
	if err == nil {
		t.Fatal("expected error for missing trace")
	}
}

func TestObservabilityServiceBuildTopologyGraph(t *testing.T) {
	nodeStore := newFakeTopologyNodeStore(
		models.TopologyNode{
			ID:           "tn_1",
			ProjectID:    "prj_123",
			NodeKind:     NodeKindInstance,
			NodeRef:      "inst_1",
			Name:         "edge-sg-1",
			Status:       "online",
			MetadataJSON: `{"region":"sg"}`,
		},
		models.TopologyNode{
			ID:           "tn_2",
			ProjectID:    "prj_123",
			NodeKind:     NodeKindInstance,
			NodeRef:      "inst_2",
			Name:         "edge-us-1",
			Status:       "online",
			MetadataJSON: `{"region":"us"}`,
		},
	)

	edgeStore := newFakeTopologyEdgeStore(
		models.TopologyEdge{
			ID:           "te_1",
			ProjectID:    "prj_123",
			SourceID:     "tn_1",
			TargetID:     "tn_2",
			EdgeKind:     EdgeKindMeshPeer,
			Protocol:     "wireguard",
			MetadataJSON: `{}`,
		},
	)

	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		nodeStore,
		edgeStore,
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	graph, err := svc.BuildTopologyGraph(context.Background(), "prj_123")
	if err != nil {
		t.Fatalf("build topology graph: %v", err)
	}

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
	if graph.Edges[0].EdgeKind != EdgeKindMeshPeer {
		t.Fatalf("expected edge kind mesh_peer, got %q", graph.Edges[0].EdgeKind)
	}
}

func TestObservabilityServiceListIncidentsByProject(t *testing.T) {
	incidentStore := newFakeRuntimeIncidentStore(
		models.RuntimeIncident{
			ID:           "inc_1",
			ProjectID:    "prj_123",
			DeploymentID: "dep_1",
			RevisionID:   "rev_1",
			Kind:         IncidentKindUnhealthyCandidate,
			Severity:     IncidentSeverityCritical,
			Status:       IncidentStatusOpen,
			Summary:      "candidate unhealthy",
			DetailsJSON:  `{"service":"api"}`,
			CreatedAt:    time.Now().UTC(),
		},
	)

	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		incidentStore,
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	incidents, err := svc.ListIncidentsByProject(context.Background(), "prj_123")
	if err != nil {
		t.Fatalf("list incidents: %v", err)
	}

	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Kind != IncidentKindUnhealthyCandidate {
		t.Fatalf("expected kind unhealthy_candidate, got %q", incidents[0].Kind)
	}
}

func TestNormalizeTraceStatus(t *testing.T) {
	tests := []struct {
		raw        string
		httpStatus int
		expected   string
	}{
		{"ok", 200, TraceStatusOK},
		{"error", 500, TraceStatusError},
		{"warning", 400, TraceStatusWarning},
		{"", 200, TraceStatusOK},
		{"", 404, TraceStatusWarning},
		{"", 503, TraceStatusError},
	}

	for _, tt := range tests {
		got := normalizeTraceStatus(tt.raw, tt.httpStatus)
		if got != tt.expected {
			t.Errorf("normalizeTraceStatus(%q, %d) = %q, want %q", tt.raw, tt.httpStatus, got, tt.expected)
		}
	}
}

func TestObservabilityServiceIngestAndPreviewLogs(t *testing.T) {
	logStore := newFakeLogStreamStore()
	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		logStore,
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	collectedAt := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	record, err := svc.IngestLogBatch(context.Background(), IngestLogBatchCommand{
		ProjectID: "prj_123",
		BindingID: "bind_123",
		Entries: []LogBatchEntry{
			{
				Timestamp: collectedAt.Add(-2 * time.Minute),
				Severity:  "warning",
				Source:    "api",
				Message:   "cache warmup slow",
				Labels:    map[string]string{"node": "edge-1"},
			},
			{
				Timestamp: collectedAt.Add(-1 * time.Minute),
				Severity:  "critical",
				Source:    "api",
				Message:   "postgres timeout",
				Labels:    map[string]string{"node": "edge-2"},
			},
		},
		CollectedAt: collectedAt,
	})
	if err != nil {
		t.Fatalf("ingest log batch: %v", err)
	}
	if record.EntryCount != 2 {
		t.Fatalf("expected 2 ingested entries, got %d", record.EntryCount)
	}

	preview, err := svc.PreviewLogs(context.Background(), PreviewLogsCommand{
		ProjectID:   "prj_123",
		ServiceName: "api",
		Level:       "error",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("preview logs: %v", err)
	}
	if len(preview.Lines) != 1 {
		t.Fatalf("expected 1 filtered log line, got %d", len(preview.Lines))
	}
	if preview.Lines[0].Node != "edge-2" {
		t.Fatalf("expected node edge-2, got %q", preview.Lines[0].Node)
	}
	if preview.Cursor == "" {
		t.Fatal("expected non-empty cursor")
	}
}

func TestObservabilityServicePreviewLogsFiltersBySource(t *testing.T) {
	logStore := newFakeLogStreamStore()
	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		logStore,
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	collectedAt := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	_, err := svc.IngestLogBatch(context.Background(), IngestLogBatchCommand{
		ProjectID: "prj_123",
		BindingID: "bind_123",
		Entries: []LogBatchEntry{
			{
				Timestamp: collectedAt.Add(-1 * time.Minute),
				Severity:  "info",
				Source:    "gateway:caddy:access",
				Message:   `{"status":200,"path":"/health"}`,
				Labels:    map[string]string{"service": "app"},
			},
			{
				Timestamp: collectedAt,
				Severity:  "critical",
				Source:    "app:container",
				Message:   "panic: nil pointer dereference",
				Labels:    map[string]string{"service": "app"},
			},
		},
		CollectedAt: collectedAt,
	})
	if err != nil {
		t.Fatalf("ingest log batch: %v", err)
	}

	preview, err := svc.PreviewLogs(context.Background(), PreviewLogsCommand{
		ProjectID:   "prj_123",
		ServiceName: "app",
		Source:      "gateway:caddy:access",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("preview logs: %v", err)
	}
	if len(preview.Lines) != 1 {
		t.Fatalf("expected 1 source-filtered line, got %d", len(preview.Lines))
	}
	if preview.Lines[0].Source != "gateway:caddy:access" {
		t.Fatalf("expected gateway source, got %q", preview.Lines[0].Source)
	}
}

func TestObservabilityServicePreviewLogsRejectsInvalidLevel(t *testing.T) {
	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	_, err := svc.PreviewLogs(context.Background(), PreviewLogsCommand{
		ProjectID:   "prj_123",
		ServiceName: "api",
		Level:       "verbose",
	})
	if err == nil {
		t.Fatal("expected invalid level error")
	}
}

func TestObservabilityServiceIngestMetricRollupAndBuildSummary(t *testing.T) {
	metricStore := newFakeMetricRollupStore()
	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	).WithMetricRollupStore(metricStore)

	inserted, err := svc.IngestMetricRollup(context.Background(), IngestAgentMetricRollupCommand{
		ProjectID:   "prj_123",
		TargetKind:  "instance",
		TargetID:    "inst_123",
		ServiceName: "app",
		Window:      "1m",
		CPU: AgentMetricAggregate{
			P95:   78.5,
			Max:   90,
			Min:   31,
			Avg:   55.4,
			Count: 12,
		},
		RAM: AgentMetricAggregate{
			P95:   512 * 1024 * 1024,
			Max:   640 * 1024 * 1024,
			Min:   256 * 1024 * 1024,
			Avg:   420 * 1024 * 1024,
			Count: 12,
		},
		Disk: AgentMetricAggregate{
			P95:   30 * 1024 * 1024 * 1024,
			Max:   32 * 1024 * 1024 * 1024,
			Min:   28 * 1024 * 1024 * 1024,
			Avg:   29 * 1024 * 1024 * 1024,
			Count: 12,
		},
		NetworkIn: AgentMetricAggregate{
			P95:   1024,
			Max:   2048,
			Min:   256,
			Avg:   768,
			Count: 12,
		},
		NetworkOut: AgentMetricAggregate{
			P95:   900,
			Max:   1800,
			Min:   300,
			Avg:   700,
			Count: 12,
		},
		RequestCount: AgentMetricAggregate{
			P95:   47,
			Max:   47,
			Min:   47,
			Avg:   47,
			Count: 47,
		},
		Latency: AgentMetricAggregate{
			P95:   180,
			Max:   240,
			Min:   75,
			Avg:   120,
			Count: 47,
		},
	})
	if err != nil {
		t.Fatalf("ingest metric rollup: %v", err)
	}
	if inserted < 7 {
		t.Fatalf("expected at least 7 metric records inserted, got %d", inserted)
	}

	items, err := svc.BuildServiceMetricSummary(context.Background(), "prj_123", 20)
	if err != nil {
		t.Fatalf("build metric summary: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 service summary, got %d", len(items))
	}
	if items[0].Service != "app" {
		t.Fatalf("expected app service, got %q", items[0].Service)
	}
	if items[0].CpuP95 <= 0 {
		t.Fatalf("expected cpu_p95 > 0, got %f", items[0].CpuP95)
	}
	if items[0].RamP95 <= 0 {
		t.Fatalf("expected ram_p95 > 0, got %f", items[0].RamP95)
	}
	if items[0].RequestCount <= 0 {
		t.Fatalf("expected request_count > 0, got %d", items[0].RequestCount)
	}
	if items[0].DiskP95Bytes <= 0 {
		t.Fatalf("expected disk_p95_bytes > 0, got %f", items[0].DiskP95Bytes)
	}
	if items[0].NetworkInTotalBytes <= 0 {
		t.Fatalf("expected network_in_total_bytes > 0, got %f", items[0].NetworkInTotalBytes)
	}
	if items[0].Period == "" {
		t.Fatal("expected non-empty period")
	}
}

func TestObservabilityServiceBuildServiceMetricSummaryDoesNotProxyFromTraces(t *testing.T) {
	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	traceStore := svc.traces.(*fakeTraceSummaryStore)
	if err := traceStore.Create(&models.TraceSummary{
		ID:            "trc_123",
		CorrelationID: "corr_123",
		ProjectID:     "prj_123",
		ServiceName:   "app",
		DurationMs:    250,
		SpanCount:     8,
		Status:        TraceStatusOK,
		ReceivedAt:    time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create trace: %v", err)
	}

	items, err := svc.BuildServiceMetricSummary(context.Background(), "prj_123", 20)
	if err != nil {
		t.Fatalf("build metric summary: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no metric summary without rollups, got %d items", len(items))
	}
}

func TestObservabilityServiceBuildMetricDashboardAggregatesSeries(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	metricStore := newFakeMetricRollupStore(
		models.MetricRollup{
			ID:          "met_req_web",
			ProjectID:   "prj_123",
			ServiceName: "web",
			MetricKind:  MetricKindRequestCount,
			WindowStart: now.Add(-3 * time.Minute),
			WindowEnd:   now.Add(-2 * time.Minute),
			Count:       100,
		},
		models.MetricRollup{
			ID:          "met_lat_web",
			ProjectID:   "prj_123",
			ServiceName: "web",
			MetricKind:  MetricKindRequestLatency,
			WindowStart: now.Add(-3 * time.Minute),
			WindowEnd:   now.Add(-2 * time.Minute),
			P95:         220,
			Avg:         180,
			Count:       100,
		},
		models.MetricRollup{
			ID:          "met_cpu_web",
			ProjectID:   "prj_123",
			ServiceName: "web",
			MetricKind:  MetricKindCPU,
			WindowStart: now.Add(-3 * time.Minute),
			WindowEnd:   now.Add(-2 * time.Minute),
			P95:         65,
			Avg:         54,
			Count:       8,
		},
		models.MetricRollup{
			ID:          "met_ram_web",
			ProjectID:   "prj_123",
			ServiceName: "web",
			MetricKind:  MetricKindMemory,
			WindowStart: now.Add(-3 * time.Minute),
			WindowEnd:   now.Add(-2 * time.Minute),
			P95:         512 * 1024 * 1024,
			Avg:         480 * 1024 * 1024,
			Count:       8,
		},
		models.MetricRollup{
			ID:          "met_req_api",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindRequestCount,
			WindowStart: now.Add(-2 * time.Minute),
			WindowEnd:   now.Add(-1 * time.Minute),
			Count:       40,
		},
		models.MetricRollup{
			ID:          "met_lat_api",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindRequestLatency,
			WindowStart: now.Add(-2 * time.Minute),
			WindowEnd:   now.Add(-1 * time.Minute),
			P95:         150,
			Avg:         120,
			Count:       40,
		},
		models.MetricRollup{
			ID:          "met_cpu_api",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindCPU,
			WindowStart: now.Add(-2 * time.Minute),
			WindowEnd:   now.Add(-1 * time.Minute),
			P95:         55,
			Avg:         49,
			Count:       6,
		},
		models.MetricRollup{
			ID:          "met_ram_api",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindMemory,
			WindowStart: now.Add(-2 * time.Minute),
			WindowEnd:   now.Add(-1 * time.Minute),
			P95:         256 * 1024 * 1024,
			Avg:         220 * 1024 * 1024,
			Count:       6,
		},
	)

	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(
			models.RuntimeIncident{
				ID:          "inc_1",
				ProjectID:   "prj_123",
				Status:      IncidentStatusOpen,
				Severity:    IncidentSeverityWarning,
				Summary:     "api latency high",
				DetailsJSON: `{"service":"api"}`,
				CreatedAt:   now,
			},
		),
		newFakeLogStreamStore(
			models.LogStreamEntry{
				ID:          "log_1",
				ProjectID:   "prj_123",
				ServiceName: "api",
				Level:       "error",
				Message:     "timeout",
				OccurredAt:  now,
			},
		),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	).WithMetricRollupStore(metricStore)

	dashboard, err := svc.BuildMetricDashboard(context.Background(), "prj_123", "", "1h", "1m")
	if err != nil {
		t.Fatalf("build metric dashboard: %v", err)
	}

	if dashboard.Window != "1h" || dashboard.Step != "1m" {
		t.Fatalf("unexpected metric range: window=%s step=%s", dashboard.Window, dashboard.Step)
	}
	if dashboard.Summary.RequestTotal != 140 {
		t.Fatalf("expected request_total 140, got %d", dashboard.Summary.RequestTotal)
	}
	if dashboard.Summary.LatencyP95Ms != 150 {
		t.Fatalf("expected latest latency p95 150, got %f", dashboard.Summary.LatencyP95Ms)
	}
	if dashboard.Summary.CpuP95 != 55 {
		t.Fatalf("expected latest cpu p95 55, got %f", dashboard.Summary.CpuP95)
	}
	if dashboard.Summary.RamP95MB != 256 {
		t.Fatalf("expected latest ram p95 256MB, got %f", dashboard.Summary.RamP95MB)
	}
	if dashboard.Summary.OpenIncidents != 1 {
		t.Fatalf("expected 1 open incident, got %d", dashboard.Summary.OpenIncidents)
	}
	if dashboard.Summary.RecentErrors != 1 {
		t.Fatalf("expected 1 recent error, got %d", dashboard.Summary.RecentErrors)
	}
	if len(dashboard.Services) != 2 || dashboard.Services[0] != "api" || dashboard.Services[1] != "web" {
		t.Fatalf("unexpected services: %#v", dashboard.Services)
	}

	var matched MetricTimeSeriesPoint
	target := alignMetricTimestamp(now.Add(-1*time.Minute), time.Minute)
	for _, point := range dashboard.Series {
		if point.Timestamp.Equal(target) {
			matched = point
			break
		}
	}
	if matched.RequestCount != 40 {
		t.Fatalf("expected api bucket request_count 40, got %d", matched.RequestCount)
	}
	if matched.LatencyP95Ms != 150 {
		t.Fatalf("expected api bucket latency 150, got %f", matched.LatencyP95Ms)
	}
}

func TestObservabilityServiceBuildMetricDashboardFiltersByService(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Minute)
	metricStore := newFakeMetricRollupStore(
		models.MetricRollup{
			ID:          "met_api_req",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindRequestCount,
			WindowStart: now.Add(-2 * time.Minute),
			WindowEnd:   now.Add(-1 * time.Minute),
			Count:       25,
		},
		models.MetricRollup{
			ID:          "met_api_lat",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindRequestLatency,
			WindowStart: now.Add(-2 * time.Minute),
			WindowEnd:   now.Add(-1 * time.Minute),
			P95:         80,
			Avg:         60,
			Count:       25,
		},
		models.MetricRollup{
			ID:          "met_web_req",
			ProjectID:   "prj_123",
			ServiceName: "web",
			MetricKind:  MetricKindRequestCount,
			WindowStart: now.Add(-2 * time.Minute),
			WindowEnd:   now.Add(-1 * time.Minute),
			Count:       90,
		},
	)

	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(
			models.RuntimeIncident{
				ID:          "inc_api",
				ProjectID:   "prj_123",
				Status:      IncidentStatusOpen,
				DetailsJSON: `{"service":"api"}`,
				CreatedAt:   now,
			},
			models.RuntimeIncident{
				ID:          "inc_web",
				ProjectID:   "prj_123",
				Status:      IncidentStatusOpen,
				DetailsJSON: `{"service":"web"}`,
				CreatedAt:   now,
			},
		),
		newFakeLogStreamStore(
			models.LogStreamEntry{
				ID:          "log_api",
				ProjectID:   "prj_123",
				ServiceName: "api",
				Level:       "error",
				Message:     "api timeout",
				OccurredAt:  now,
			},
			models.LogStreamEntry{
				ID:          "log_web",
				ProjectID:   "prj_123",
				ServiceName: "web",
				Level:       "error",
				Message:     "web timeout",
				OccurredAt:  now,
			},
		),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	).WithMetricRollupStore(metricStore)

	dashboard, err := svc.BuildMetricDashboard(context.Background(), "prj_123", "api", "1h", "1m")
	if err != nil {
		t.Fatalf("build filtered metric dashboard: %v", err)
	}

	if dashboard.Summary.RequestTotal != 25 {
		t.Fatalf("expected filtered request_total 25, got %d", dashboard.Summary.RequestTotal)
	}
	if dashboard.Summary.OpenIncidents != 1 {
		t.Fatalf("expected filtered open incidents 1, got %d", dashboard.Summary.OpenIncidents)
	}
	if dashboard.Summary.RecentErrors != 1 {
		t.Fatalf("expected filtered recent errors 1, got %d", dashboard.Summary.RecentErrors)
	}
	if len(dashboard.Services) != 1 || dashboard.Services[0] != "api" {
		t.Fatalf("unexpected filtered services: %#v", dashboard.Services)
	}
}
