package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PodTopologyReporter struct {
	logger *slog.Logger
	now    func() time.Time

	mu    sync.Mutex
	nodes []ClusterNode
	pods  []ClusterPod
}

type ClusterNode struct {
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	Role         string            `json:"role"`
	Labels       map[string]string `json:"labels,omitempty"`
	LastReported time.Time         `json:"last_reported"`
}

type ClusterPod struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	NodeName     string            `json:"node_name"`
	Status       string            `json:"status"`
	RestartCount int               `json:"restart_count"`
	Labels       map[string]string `json:"labels,omitempty"`
	LastReported time.Time         `json:"last_reported"`
}

func NewPodTopologyReporter(logger *slog.Logger) *PodTopologyReporter {
	return &PodTopologyReporter{
		logger: logger,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (r *PodTopologyReporter) ReportNode(node ClusterNode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	node.LastReported = r.now()
	r.nodes = append(r.nodes, node)
}

func (r *PodTopologyReporter) ReportPod(pod ClusterPod) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pod.LastReported = r.now()
	r.pods = append(r.pods, pod)
}

func (r *PodTopologyReporter) SetNodes(nodes []ClusterNode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for i := range nodes {
		nodes[i].LastReported = now
	}
	r.nodes = nodes
}

func (r *PodTopologyReporter) SetPods(pods []ClusterPod) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for i := range pods {
		pods[i].LastReported = now
	}
	r.pods = pods
}

type ClusterTopologySummary struct {
	Nodes      []ClusterNode `json:"nodes"`
	Pods       []ClusterPod  `json:"pods"`
	SnapshotAt time.Time     `json:"snapshot_at"`
	NodeCount  int           `json:"node_count"`
	PodCount   int           `json:"pod_count"`
}

func (r *PodTopologyReporter) BuildSummary() ClusterTopologySummary {
	r.mu.Lock()
	defer r.mu.Unlock()

	nodes := make([]ClusterNode, len(r.nodes))
	copy(nodes, r.nodes)
	pods := make([]ClusterPod, len(r.pods))
	copy(pods, r.pods)

	return ClusterTopologySummary{
		Nodes:      nodes,
		Pods:       pods,
		SnapshotAt: r.now(),
		NodeCount:  len(nodes),
		PodCount:   len(pods),
	}
}

func (r *PodTopologyReporter) Stats() (nodes, pods int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.nodes), len(r.pods)
}

func (r *PodTopologyReporter) PersistTopology(workspaceRoot string, summary ClusterTopologySummary) (string, error) {
	topoDir := filepath.Join(workspaceRoot, "node-agent", "cluster-topology")
	if err := os.MkdirAll(topoDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create cluster topology directory: %w", err)
	}

	timestamp := r.now().Format("20060102T150405Z")
	topoPath := filepath.Join(topoDir, "summary_"+timestamp+".json")

	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal cluster topology: %w", err)
	}

	if err := os.WriteFile(topoPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write cluster topology: %w", err)
	}

	return topoPath, nil
}
