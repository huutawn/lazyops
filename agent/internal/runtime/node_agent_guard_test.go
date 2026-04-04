package runtime

import (
	"testing"

	"lazyops-agent/internal/contracts"
)

func testNodeAgentGuard() *NodeAgentGuard {
	return NewNodeAgentGuard(nil, nil, contracts.RuntimeModeDistributedK3s)
}

func TestNodeAgentGuardIsNodeAgentMode(t *testing.T) {
	g := testNodeAgentGuard()
	if !g.IsNodeAgentMode() {
		t.Fatal("expected node agent mode")
	}
}

func TestNodeAgentGuardNotNodeAgentMode(t *testing.T) {
	g := NewNodeAgentGuard(nil, nil, contracts.RuntimeModeStandalone)
	if g.IsNodeAgentMode() {
		t.Fatal("expected not node agent mode")
	}
}

func TestNodeAgentGuardAssertTelemetryOnly(t *testing.T) {
	g := NewNodeAgentGuard(nil, nil, contracts.RuntimeModeStandalone)
	err := g.AssertTelemetryOnly("container_log_tailing")
	if err != nil {
		t.Fatalf("expected no error in standalone mode, got %v", err)
	}
}

func TestNodeAgentGuardAssertTelemetryOnlyRejected(t *testing.T) {
	g := testNodeAgentGuard()
	err := g.AssertTelemetryOnly("prepare_release_workspace")
	if err == nil {
		t.Fatal("expected error in node agent mode")
	}
}

func TestNodeAgentGuardAssertNotNodeAgent(t *testing.T) {
	g := NewNodeAgentGuard(nil, nil, contracts.RuntimeModeStandalone)
	err := g.AssertNotNodeAgent("prepare_release_workspace")
	if err != nil {
		t.Fatalf("expected no error in standalone mode, got %v", err)
	}
}

func TestNodeAgentGuardAssertNotNodeAgentRejected(t *testing.T) {
	g := testNodeAgentGuard()
	err := g.AssertNotNodeAgent("promote_release")
	if err == nil {
		t.Fatal("expected error in node agent mode")
	}
}

func TestNodeAgentGuardAllowedOperations(t *testing.T) {
	g := testNodeAgentGuard()
	allowed := g.AllowedOperations()
	if len(allowed) == 0 {
		t.Fatal("expected non-empty allowed operations")
	}
	expected := []string{
		"container_log_tailing",
		"node_metrics_collection",
		"pod_topology_reporting",
		"cluster_incident_reporting",
		"health_gate_reporting",
	}
	for _, op := range expected {
		found := false
		for _, a := range allowed {
			if a == op {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in allowed operations", op)
		}
	}
}

func TestNodeAgentGuardBlockedOperations(t *testing.T) {
	g := testNodeAgentGuard()
	blocked := g.BlockedOperations()
	if len(blocked) == 0 {
		t.Fatal("expected non-empty blocked operations")
	}
	expected := []string{
		"prepare_release_workspace",
		"start_release_candidate",
		"promote_release",
		"rollback_release",
		"render_gateway_config",
		"render_sidecars",
		"run_health_gate",
		"sleep_service",
		"wake_service",
		"scale_to_zero",
	}
	for _, op := range expected {
		found := false
		for _, b := range blocked {
			if b == op {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in blocked operations", op)
		}
	}
}
