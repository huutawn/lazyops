package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testClusterIncidentReporter() *ClusterIncidentReporter {
	r := NewClusterIncidentReporter(nil)
	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 15, 0, 0, 0, time.UTC)
	}
	return r
}

func TestClusterIncidentReporterReport(t *testing.T) {
	r := testClusterIncidentReporter()
	r.Report(ClusterIncident{
		Kind:     "unhealthy_node",
		Severity: contracts.SeverityWarning,
		Summary:  "node node-1 is unhealthy",
		NodeName: "node-1",
	})

	total, suppressed, pending := r.Stats()
	if total != 1 {
		t.Fatalf("expected 1 total, got %d", total)
	}
	if suppressed != 0 {
		t.Fatalf("expected 0 suppressed, got %d", suppressed)
	}
	if pending != 1 {
		t.Fatalf("expected 1 pending, got %d", pending)
	}
}

func TestClusterIncidentReporterCooldown(t *testing.T) {
	r := testClusterIncidentReporter()
	r.Report(ClusterIncident{
		Kind:     "unhealthy_node",
		Severity: contracts.SeverityWarning,
		Summary:  "node node-1 is unhealthy",
		NodeName: "node-1",
	})
	r.Report(ClusterIncident{
		Kind:     "unhealthy_node",
		Severity: contracts.SeverityWarning,
		Summary:  "node node-1 is unhealthy",
		NodeName: "node-1",
	})

	total, suppressed, pending := r.Stats()
	if total != 2 {
		t.Fatalf("expected 2 total, got %d", total)
	}
	if suppressed != 1 {
		t.Fatalf("expected 1 suppressed due to cooldown, got %d", suppressed)
	}
	if pending != 1 {
		t.Fatalf("expected 1 pending, got %d", pending)
	}
}

func TestClusterIncidentReporterReportUnhealthyNode(t *testing.T) {
	r := testClusterIncidentReporter()
	r.ReportUnhealthyNode("node-1", map[string]any{"reason": "not_ready"})

	incidents := r.CollectIncidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Kind != "unhealthy_node" {
		t.Fatalf("expected kind unhealthy_node, got %q", incidents[0].Kind)
	}
	if incidents[0].NodeName != "node-1" {
		t.Fatalf("expected node_name node-1, got %q", incidents[0].NodeName)
	}
}

func TestClusterIncidentReporterReportPodCrashLoop(t *testing.T) {
	r := testClusterIncidentReporter()
	r.ReportPodCrashLoop("pod-1", "default", 5)

	incidents := r.CollectIncidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Kind != "pod_crash_loop" {
		t.Fatalf("expected kind pod_crash_loop, got %q", incidents[0].Kind)
	}
	if incidents[0].Severity != contracts.SeverityCritical {
		t.Fatalf("expected critical severity, got %q", incidents[0].Severity)
	}
}

func TestClusterIncidentReporterReportMeshIssue(t *testing.T) {
	r := testClusterIncidentReporter()
	r.ReportMeshIssue("mesh peer unreachable", map[string]any{"peer": "node-2"})

	incidents := r.CollectIncidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Kind != "mesh_issue" {
		t.Fatalf("expected kind mesh_issue, got %q", incidents[0].Kind)
	}
}

func TestClusterIncidentReporterReportTunnelIssue(t *testing.T) {
	r := testClusterIncidentReporter()
	r.ReportTunnelIssue("tunnel disconnected", map[string]any{"tunnel": "tun-1"})

	incidents := r.CollectIncidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Kind != "tunnel_issue" {
		t.Fatalf("expected kind tunnel_issue, got %q", incidents[0].Kind)
	}
}

func TestClusterIncidentReporterCollectIncidents(t *testing.T) {
	r := testClusterIncidentReporter()
	r.ReportUnhealthyNode("node-1", nil)
	r.ReportPodCrashLoop("pod-1", "default", 3)

	incidents := r.CollectIncidents()
	if len(incidents) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(incidents))
	}

	_, _, pending := r.Stats()
	if pending != 0 {
		t.Fatalf("expected 0 pending after collection, got %d", pending)
	}
}

func TestClusterIncidentReporterCollectIncidentsEmpty(t *testing.T) {
	r := testClusterIncidentReporter()
	incidents := r.CollectIncidents()
	if len(incidents) != 0 {
		t.Fatalf("expected 0 incidents, got %d", len(incidents))
	}
}

func TestClusterIncidentReporterPersistIncidents(t *testing.T) {
	r := testClusterIncidentReporter()
	r.ReportUnhealthyNode("node-1", nil)
	incidents := r.CollectIncidents()

	root := filepath.Join(t.TempDir(), "runtime-root")
	incidentPath, err := r.PersistIncidents(root, incidents)
	if err != nil {
		t.Fatalf("persist incidents: %v", err)
	}

	if _, err := os.Stat(incidentPath); err != nil {
		t.Fatalf("expected incident file to exist: %v", err)
	}

	var loaded []ClusterIncident
	raw, err := os.ReadFile(incidentPath)
	if err != nil {
		t.Fatalf("read incident file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode incident file: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 incident in persisted file, got %d", len(loaded))
	}
}

func TestClusterIncidentReporterPersistIncidentsEmpty(t *testing.T) {
	r := testClusterIncidentReporter()
	root := filepath.Join(t.TempDir(), "runtime-root")
	incidentPath, err := r.PersistIncidents(root, nil)
	if err != nil {
		t.Fatalf("persist incidents: %v", err)
	}
	if incidentPath != "" {
		t.Fatal("expected empty path for empty incidents")
	}
}
