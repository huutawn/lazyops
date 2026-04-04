package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type TopologyReporterConfig struct {
	ReportingInterval time.Duration
}

func DefaultTopologyReporterConfig() TopologyReporterConfig {
	return TopologyReporterConfig{
		ReportingInterval: 60 * time.Second,
	}
}

type TopologyReporter struct {
	logger *slog.Logger
	cfg    TopologyReporterConfig
	now    func() time.Time

	mu    sync.Mutex
	nodes []contracts.TopologyNode
	edges []contracts.TopologyEdge
}

func NewTopologyReporter(logger *slog.Logger, cfg TopologyReporterConfig) *TopologyReporter {
	if cfg.ReportingInterval <= 0 {
		cfg.ReportingInterval = 60 * time.Second
	}

	return &TopologyReporter{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (r *TopologyReporter) SetNodes(nodes []contracts.TopologyNode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes = nodes
}

func (r *TopologyReporter) SetEdges(edges []contracts.TopologyEdge) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edges = edges
}

func (r *TopologyReporter) AddNode(node contracts.TopologyNode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes = append(r.nodes, node)
}

func (r *TopologyReporter) AddEdge(edge contracts.TopologyEdge) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edges = append(r.edges, edge)
}

func (r *TopologyReporter) BuildTopology(projectID string) contracts.TopologyPayload {
	r.mu.Lock()
	defer r.mu.Unlock()

	nodes := make([]contracts.TopologyNode, len(r.nodes))
	copy(nodes, r.nodes)
	edges := make([]contracts.TopologyEdge, len(r.edges))
	copy(edges, r.edges)

	return contracts.TopologyPayload{
		ProjectID:  projectID,
		SnapshotAt: r.now(),
		Nodes:      nodes,
		Edges:      edges,
	}
}

func (r *TopologyReporter) Stats() (nodes, edges int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.nodes), len(r.edges)
}

func (r *TopologyReporter) PersistTopology(workspaceRoot, projectID, bindingID string, topology contracts.TopologyPayload) (string, error) {
	topoDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "topology")
	if err := os.MkdirAll(topoDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create topology directory: %w", err)
	}

	timestamp := r.now().Format("20060102T150405Z")
	topoPath := filepath.Join(topoDir, "snapshot_"+timestamp+".json")

	raw, err := json.MarshalIndent(topology, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal topology: %w", err)
	}

	if err := os.WriteFile(topoPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write topology: %w", err)
	}

	return topoPath, nil
}

type TopologySender interface {
	SendTopology(context.Context, contracts.TopologyPayload) error
}

type ReportTopologyStatePayload struct {
	ProjectID      string                `json:"project_id"`
	BindingID      string                `json:"binding_id"`
	RevisionID     string                `json:"revision_id"`
	RuntimeMode    contracts.RuntimeMode `json:"runtime_mode"`
	WorkspaceRoot  string                `json:"workspace_root"`
	TopologySender TopologySender        `json:"-"`
}

func (r *TopologyReporter) HandleReportTopologyState(ctx context.Context, logger *slog.Logger, payload ReportTopologyStatePayload) error {
	if logger == nil {
		logger = slog.Default()
	}

	topology := r.BuildTopology(payload.ProjectID)

	if len(topology.Nodes) == 0 && len(topology.Edges) == 0 {
		logger.Info("no topology data to report",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
		)
		return nil
	}

	if payload.TopologySender != nil {
		if err := payload.TopologySender.SendTopology(ctx, topology); err != nil {
			logger.Warn("could not send topology to backend",
				"project_id", payload.ProjectID,
				"error", err,
			)
		}
	}

	workspaceRoot := payload.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(
			"/var/lib/lazyops",
			"projects", payload.ProjectID,
			"bindings", payload.BindingID,
			"revisions", payload.RevisionID,
		)
	}

	topoPath, err := r.PersistTopology(workspaceRoot, payload.ProjectID, payload.BindingID, topology)
	if err != nil {
		logger.Warn("could not persist topology",
			"project_id", payload.ProjectID,
			"error", err,
		)
	} else {
		logger.Info("topology snapshot collected",
			"project_id", payload.ProjectID,
			"nodes", len(topology.Nodes),
			"edges", len(topology.Edges),
			"topo_path", topoPath,
		)
	}

	return nil
}
