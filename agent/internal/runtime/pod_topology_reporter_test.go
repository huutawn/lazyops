package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testPodTopologyReporter() *PodTopologyReporter {
	r := NewPodTopologyReporter(nil)
	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC)
	}
	return r
}

func TestPodTopologyReporterReportNode(t *testing.T) {
	r := testPodTopologyReporter()
	r.ReportNode(ClusterNode{
		Name:   "node-1",
		Status: "ready",
		Role:   "master",
	})

	nodes, pods := r.Stats()
	if nodes != 1 {
		t.Fatalf("expected 1 node, got %d", nodes)
	}
	if pods != 0 {
		t.Fatalf("expected 0 pods, got %d", pods)
	}
}

func TestPodTopologyReporterReportPod(t *testing.T) {
	r := testPodTopologyReporter()
	r.ReportPod(ClusterPod{
		Name:      "pod-1",
		Namespace: "default",
		NodeName:  "node-1",
		Status:    "running",
	})

	nodes, pods := r.Stats()
	if nodes != 0 {
		t.Fatalf("expected 0 nodes, got %d", nodes)
	}
	if pods != 1 {
		t.Fatalf("expected 1 pod, got %d", pods)
	}
}

func TestPodTopologyReporterSetNodes(t *testing.T) {
	r := testPodTopologyReporter()
	r.SetNodes([]ClusterNode{
		{Name: "node-1", Status: "ready", Role: "master"},
		{Name: "node-2", Status: "ready", Role: "worker"},
	})

	nodes, _ := r.Stats()
	if nodes != 2 {
		t.Fatalf("expected 2 nodes, got %d", nodes)
	}
}

func TestPodTopologyReporterSetPods(t *testing.T) {
	r := testPodTopologyReporter()
	r.SetPods([]ClusterPod{
		{Name: "pod-1", Namespace: "default", NodeName: "node-1", Status: "running"},
		{Name: "pod-2", Namespace: "default", NodeName: "node-2", Status: "pending"},
	})

	_, pods := r.Stats()
	if pods != 2 {
		t.Fatalf("expected 2 pods, got %d", pods)
	}
}

func TestPodTopologyReporterBuildSummary(t *testing.T) {
	r := testPodTopologyReporter()
	r.ReportNode(ClusterNode{Name: "node-1", Status: "ready", Role: "master"})
	r.ReportPod(ClusterPod{Name: "pod-1", Namespace: "default", NodeName: "node-1", Status: "running"})

	summary := r.BuildSummary()
	if summary.NodeCount != 1 {
		t.Fatalf("expected 1 node in summary, got %d", summary.NodeCount)
	}
	if summary.PodCount != 1 {
		t.Fatalf("expected 1 pod in summary, got %d", summary.PodCount)
	}
	if summary.SnapshotAt.IsZero() {
		t.Fatal("expected snapshot_at to be set")
	}
}

func TestPodTopologyReporterPersistTopology(t *testing.T) {
	r := testPodTopologyReporter()
	r.ReportNode(ClusterNode{Name: "node-1", Status: "ready", Role: "master"})

	summary := r.BuildSummary()
	root := filepath.Join(t.TempDir(), "runtime-root")
	topoPath, err := r.PersistTopology(root, summary)
	if err != nil {
		t.Fatalf("persist topology: %v", err)
	}

	if _, err := os.Stat(topoPath); err != nil {
		t.Fatalf("expected topology file to exist: %v", err)
	}

	var loaded ClusterTopologySummary
	raw, err := os.ReadFile(topoPath)
	if err != nil {
		t.Fatalf("read topology file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode topology file: %v", err)
	}
	if loaded.NodeCount != 1 {
		t.Fatalf("expected 1 node in persisted topology, got %d", loaded.NodeCount)
	}
}
