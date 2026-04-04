package runtime

import (
	"fmt"
	"log/slog"
	"sync"

	"lazyops-agent/internal/contracts"
)

type NodeAgentGuard struct {
	logger   *slog.Logger
	detector *K3sEnvironmentDetector
	mode     contracts.RuntimeMode

	mu sync.Mutex
}

func NewNodeAgentGuard(logger *slog.Logger, detector *K3sEnvironmentDetector, mode contracts.RuntimeMode) *NodeAgentGuard {
	return &NodeAgentGuard{
		logger:   logger,
		detector: detector,
		mode:     mode,
	}
}

func (g *NodeAgentGuard) IsNodeAgentMode() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.mode == contracts.RuntimeModeDistributedK3s
}

func (g *NodeAgentGuard) AssertTelemetryOnly(operation string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.mode == contracts.RuntimeModeDistributedK3s {
		return fmt.Errorf("node_agent is telemetry/protection only and must not perform %s", operation)
	}

	return nil
}

func (g *NodeAgentGuard) AssertNotNodeAgent(operation string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.mode == contracts.RuntimeModeDistributedK3s {
		return fmt.Errorf("operation %s is not allowed in node_agent mode", operation)
	}

	return nil
}

func (g *NodeAgentGuard) AllowedOperations() []string {
	return []string{
		"container_log_tailing",
		"node_metrics_collection",
		"pod_topology_reporting",
		"cluster_incident_reporting",
		"health_gate_reporting",
	}
}

func (g *NodeAgentGuard) BlockedOperations() []string {
	return []string{
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
}
