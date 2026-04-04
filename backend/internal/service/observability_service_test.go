package service

import (
	"context"
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

func newTestObservabilityService(
	traceStore TraceSummaryStore,
	incidentStore RuntimeIncidentStore,
	nodeStore TopologyNodeStore,
	edgeStore TopologyEdgeStore,
	instanceStore InstanceStore,
	meshStore MeshNetworkStore,
	clusterStore ClusterStore,
) *ObservabilityService {
	return NewObservabilityService(traceStore, incidentStore, nodeStore, edgeStore, instanceStore, meshStore, clusterStore)
}

func TestObservabilityServiceIngestTraceSuccess(t *testing.T) {
	traceStore := newFakeTraceSummaryStore()

	svc := newTestObservabilityService(
		traceStore,
		newFakeRuntimeIncidentStore(),
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
