package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testTopologyReporter() *TopologyReporter {
	r := NewTopologyReporter(nil, TopologyReporterConfig{
		ReportingInterval: 1 * time.Second,
	})
	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	}
	return r
}

func TestTopologyReporterDefaultConfig(t *testing.T) {
	r := NewTopologyReporter(nil, TopologyReporterConfig{})
	if r.cfg.ReportingInterval != 60*time.Second {
		t.Fatalf("expected default interval 60s, got %s", r.cfg.ReportingInterval)
	}
}

func TestTopologyReporterAddNode(t *testing.T) {
	r := testTopologyReporter()
	r.AddNode(contracts.TopologyNode{
		NodeRef:  "inst_123",
		NodeType: "instance",
		Status:   "online",
	})

	nodes, edges := r.Stats()
	if nodes != 1 {
		t.Fatalf("expected 1 node, got %d", nodes)
	}
	if edges != 0 {
		t.Fatalf("expected 0 edges, got %d", edges)
	}
}

func TestTopologyReporterAddEdge(t *testing.T) {
	r := testTopologyReporter()
	r.AddEdge(contracts.TopologyEdge{
		SourceNodeRef: "inst_123",
		TargetNodeRef: "inst_456",
		EdgeType:      "mesh",
		Status:        "active",
	})

	nodes, edges := r.Stats()
	if nodes != 0 {
		t.Fatalf("expected 0 nodes, got %d", nodes)
	}
	if edges != 1 {
		t.Fatalf("expected 1 edge, got %d", edges)
	}
}

func TestTopologyReporterSetNodes(t *testing.T) {
	r := testTopologyReporter()
	r.SetNodes([]contracts.TopologyNode{
		{NodeRef: "inst_123", NodeType: "instance", Status: "online"},
		{NodeRef: "inst_456", NodeType: "instance", Status: "degraded"},
	})

	nodes, _ := r.Stats()
	if nodes != 2 {
		t.Fatalf("expected 2 nodes, got %d", nodes)
	}
}

func TestTopologyReporterBuildTopology(t *testing.T) {
	r := testTopologyReporter()
	r.AddNode(contracts.TopologyNode{
		NodeRef:  "inst_123",
		NodeType: "instance",
		Status:   "online",
	})
	r.AddEdge(contracts.TopologyEdge{
		SourceNodeRef: "inst_123",
		TargetNodeRef: "mesh_remote",
		EdgeType:      "mesh",
		Status:        "active",
	})

	topology := r.BuildTopology("prj_123")

	if topology.ProjectID != "prj_123" {
		t.Fatalf("expected project_id prj_123, got %q", topology.ProjectID)
	}
	if topology.SnapshotAt.IsZero() {
		t.Fatal("expected snapshot_at to be set")
	}
	if len(topology.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(topology.Nodes))
	}
	if len(topology.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(topology.Edges))
	}
}

func TestTopologyReporterPersistTopology(t *testing.T) {
	r := testTopologyReporter()
	topology := r.BuildTopology("prj_123")

	root := filepath.Join(t.TempDir(), "runtime-root")
	topoPath, err := r.PersistTopology(root, "prj_123", "bind_123", topology)
	if err != nil {
		t.Fatalf("persist topology: %v", err)
	}

	if _, err := os.Stat(topoPath); err != nil {
		t.Fatalf("expected topology file to exist: %v", err)
	}

	var loaded contracts.TopologyPayload
	raw, err := os.ReadFile(topoPath)
	if err != nil {
		t.Fatalf("read topology file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode topology file: %v", err)
	}
	if loaded.ProjectID != "prj_123" {
		t.Fatalf("expected project_id prj_123, got %q", loaded.ProjectID)
	}
}

func TestTopologyReporterHandleReportTopologyState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	r := testTopologyReporter()
	r.AddNode(contracts.TopologyNode{
		NodeRef:  "inst_123",
		NodeType: "instance",
		Status:   "online",
	})

	err := r.HandleReportTopologyState(context.Background(), nil, ReportTopologyStatePayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: root,
	})
	if err != nil {
		t.Fatalf("handle report topology state: %v", err)
	}

	topoPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "topology", "snapshot_20260404T120000Z.json")
	if _, err := os.Stat(topoPath); err != nil {
		t.Fatalf("expected topology file at %s: %v", topoPath, err)
	}
}

func TestTopologyReporterHandleReportTopologyStateEmpty(t *testing.T) {
	r := testTopologyReporter()

	err := r.HandleReportTopologyState(context.Background(), nil, ReportTopologyStatePayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("handle report topology state: %v", err)
	}
}

type fakeTopologySender struct {
	mu      sync.Mutex
	sent    []contracts.TopologyPayload
	sendErr error
}

func (f *fakeTopologySender) SendTopology(_ context.Context, payload contracts.TopologyPayload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, payload)
	return f.sendErr
}

func TestTopologyReporterHandleReportTopologyStateWithSender(t *testing.T) {
	r := testTopologyReporter()
	r.AddNode(contracts.TopologyNode{
		NodeRef:  "inst_123",
		NodeType: "instance",
		Status:   "online",
	})

	sender := &fakeTopologySender{}
	err := r.HandleReportTopologyState(context.Background(), nil, ReportTopologyStatePayload{
		ProjectID:      "prj_123",
		BindingID:      "bind_123",
		RevisionID:     "rev_123",
		RuntimeMode:    contracts.RuntimeModeStandalone,
		WorkspaceRoot:  filepath.Join(t.TempDir(), "runtime-root"),
		TopologySender: sender,
	})
	if err != nil {
		t.Fatalf("handle report topology state: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 topology sent, got %d", len(sender.sent))
	}
	if len(sender.sent[0].Nodes) != 1 {
		t.Fatalf("expected 1 node in sent topology, got %d", len(sender.sent[0].Nodes))
	}
}
