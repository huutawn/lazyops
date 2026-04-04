package service

import (
	"context"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

func mustParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func newTestK3sClusterService(
	clusterStore ClusterStore,
	topologyStore TopologyStateStore,
) *K3sClusterService {
	return NewK3sClusterService(clusterStore, topologyStore)
}

func TestK3sClusterServiceValidateClusterSuccess(t *testing.T) {
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "k3s-prod",
		Provider:            "k3s",
		KubeconfigSecretRef: "secret/k3s-prod-kubeconfig",
		Status:              ClusterStatusValidating,
	})

	svc := newTestK3sClusterService(clusterStore, newFakeTopologyStateStore())

	result, err := svc.ValidateCluster(context.Background(), "usr_123", "cls_123")
	if err != nil {
		t.Fatalf("validate cluster: %v", err)
	}

	if result.ClusterID != "cls_123" {
		t.Fatalf("expected cluster id cls_123, got %q", result.ClusterID)
	}
	if result.Provider != "k3s" {
		t.Fatalf("expected provider k3s, got %q", result.Provider)
	}
	if result.Status != ClusterStatusReady {
		t.Fatalf("expected status ready, got %q", result.Status)
	}
	if len(result.Checks) != 3 {
		t.Fatalf("expected 3 validation checks, got %d", len(result.Checks))
	}
	for _, check := range result.Checks {
		if !check.Passed {
			t.Fatalf("expected check %q to pass", check.Name)
		}
	}
}

func TestK3sClusterServiceRejectsNonK3sCluster(t *testing.T) {
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "standalone-srv",
		Provider:            "standalone",
		KubeconfigSecretRef: "secret/something",
		Status:              ClusterStatusValidating,
	})

	svc := newTestK3sClusterService(clusterStore, newFakeTopologyStateStore())

	_, err := svc.ValidateCluster(context.Background(), "usr_123", "cls_123")
	if err == nil {
		t.Fatal("expected error for non-k3s cluster")
	}
}

func TestK3sClusterServiceGetClusterReadiness(t *testing.T) {
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "k3s-prod",
		Provider:            "k3s",
		KubeconfigSecretRef: "secret/k3s-prod-kubeconfig",
		Status:              ClusterStatusReady,
	})

	topologyStore := newFakeTopologyStateStore(
		models.TopologyState{
			ID:           "topo_1",
			InstanceID:   "node_1",
			MeshID:       "cls_123",
			State:        TopologyStateOnline,
			MetadataJSON: `{"role":"worker"}`,
			LastSeenAt:   mustParseTime("2026-04-04T10:00:00Z"),
		},
		models.TopologyState{
			ID:           "topo_2",
			InstanceID:   "node_2",
			MeshID:       "cls_123",
			State:        TopologyStateOnline,
			MetadataJSON: `{"role":"worker"}`,
			LastSeenAt:   mustParseTime("2026-04-04T10:00:00Z"),
		},
	)

	svc := newTestK3sClusterService(clusterStore, topologyStore)

	report, err := svc.GetClusterReadiness(context.Background(), "usr_123", "cls_123")
	if err != nil {
		t.Fatalf("get readiness: %v", err)
	}

	if !report.IsReady {
		t.Fatal("expected cluster to be ready")
	}
	if report.TotalNodes != 2 {
		t.Fatalf("expected 2 total nodes, got %d", report.TotalNodes)
	}
	if report.ReadyNodes != 2 {
		t.Fatalf("expected 2 ready nodes, got %d", report.ReadyNodes)
	}
}

func TestK3sClusterServiceIngestNodeTelemetry(t *testing.T) {
	topologyStore := newFakeTopologyStateStore()

	svc := newTestK3sClusterService(
		newFakeClusterStore(),
		topologyStore,
	)

	record, err := svc.IngestNodeTelemetry(context.Background(), "cls_123", "node_1", NodeTelemetryPayload{
		State:       "busy",
		Health:      "healthy",
		CPUPercent:  45.2,
		MemoryBytes: 4294967296,
		DiskBytes:   10737418240,
		PodCount:    12,
		Metadata:    map[string]any{"role": "worker"},
	})
	if err != nil {
		t.Fatalf("ingest telemetry: %v", err)
	}

	if record.NodeID != "node_1" {
		t.Fatalf("expected node id node_1, got %q", record.NodeID)
	}
	if record.ClusterID != "cls_123" {
		t.Fatalf("expected cluster id cls_123, got %q", record.ClusterID)
	}
	if record.State != NodeAgentStateBusy {
		t.Fatalf("expected state busy, got %q", record.State)
	}
	if record.CPUPercent != 45.2 {
		t.Fatalf("expected cpu 45.2, got %f", record.CPUPercent)
	}
	if record.PodCount != 12 {
		t.Fatalf("expected pod count 12, got %d", record.PodCount)
	}
}

func TestK3sClusterServiceEnforceK3sBoundaryRejectsForbiddenCommands(t *testing.T) {
	svc := newTestK3sClusterService(
		newFakeClusterStore(),
		newFakeTopologyStateStore(),
	)

	forbiddenCommands := []string{
		"docker_run", "docker_stop", "docker_rm",
		"direct_deploy", "process_start", "process_stop",
		"file_deploy", "systemctl_start", "systemctl_stop",
	}

	for _, cmdType := range forbiddenCommands {
		err := svc.EnforceK3sBoundary(cmdType)
		if err == nil {
			t.Fatalf("expected error for forbidden command %q, got nil", cmdType)
		}
	}
}

func TestK3sClusterServiceEnforceK3sBoundaryAllowsValidCommands(t *testing.T) {
	svc := newTestK3sClusterService(
		newFakeClusterStore(),
		newFakeTopologyStateStore(),
	)

	validCommands := []string{
		"reconcile_revision", "render_gateway_config",
		"run_health_gate", "promote_release",
		"report_topology_state", "report_metric_rollup",
	}

	for _, cmdType := range validCommands {
		err := svc.EnforceK3sBoundary(cmdType)
		if err != nil {
			t.Fatalf("expected valid command %q to pass, got error: %v", cmdType, err)
		}
	}
}

func TestNormalizeNodeAgentState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"idle", NodeAgentStateIdle},
		{"busy", NodeAgentStateBusy},
		{"draining", NodeAgentStateDraining},
		{"error", NodeAgentStateError},
		{"unknown", NodeAgentStateIdle},
		{"", NodeAgentStateIdle},
	}

	for _, tt := range tests {
		got := normalizeNodeAgentState(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeNodeAgentState(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTelemetryStateFromNodeState(t *testing.T) {
	tests := []struct {
		state    string
		expected string
	}{
		{NodeAgentStateError, TopologyStateDegraded},
		{NodeAgentStateIdle, TopologyStateOnline},
		{NodeAgentStateBusy, TopologyStateOnline},
		{NodeAgentStateDraining, TopologyStateOffline},
		{"unknown", TopologyStateOffline},
	}

	for _, tt := range tests {
		got := telemetryStateFromNodeState(tt.state)
		if got != tt.expected {
			t.Errorf("telemetryStateFromNodeState(%q) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}
