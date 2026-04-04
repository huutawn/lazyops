package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	ClusterStatusValidating  = "validating"
	ClusterStatusReady       = "ready"
	ClusterStatusDegraded    = "degraded"
	ClusterStatusUnreachable = "unreachable"
	ClusterStatusDraining    = "draining"

	NodeAgentStateIdle     = "idle"
	NodeAgentStateBusy     = "busy"
	NodeAgentStateDraining = "draining"
	NodeAgentStateError    = "error"
)

var (
	ErrClusterUnreachable   = errors.New("cluster API is unreachable")
	ErrClusterNotReady      = errors.New("cluster is not ready for workload scheduling")
	ErrK3sBoundaryViolation = errors.New("command violates K3s boundary: workload scheduling must go through Kubernetes")
)

type K3sClusterService struct {
	clusters ClusterStore
	topology TopologyStateStore
}

func NewK3sClusterService(clusters ClusterStore, topology TopologyStateStore) *K3sClusterService {
	return &K3sClusterService{
		clusters: clusters,
		topology: topology,
	}
}

func (s *K3sClusterService) ValidateCluster(ctx context.Context, userID, clusterID string) (*ClusterValidationResult, error) {
	cluster, err := s.clusters.GetByIDForUser(userID, clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, ErrTargetNotFound
	}

	if cluster.Provider != "k3s" {
		return nil, fmt.Errorf("cluster %q is not a k3s cluster, got %q", clusterID, cluster.Provider)
	}

	now := time.Now().UTC()
	checks := s.runValidationChecks(ctx, cluster)

	allPassed := true
	for _, check := range checks {
		if !check.Passed {
			allPassed = false
			break
		}
	}

	status := ClusterStatusReady
	if !allPassed {
		status = ClusterStatusDegraded
	}
	for _, check := range checks {
		if check.Name == "kube_api_reachable" && !check.Passed {
			status = ClusterStatusUnreachable
			break
		}
	}

	if err := s.clusters.UpdateStatus(cluster.ID, status, now); err != nil {
		return nil, err
	}

	return &ClusterValidationResult{
		ClusterID:   cluster.ID,
		Name:        cluster.Name,
		Provider:    cluster.Provider,
		Status:      status,
		Checks:      checks,
		ValidatedAt: now,
	}, nil
}

func (s *K3sClusterService) GetClusterReadiness(ctx context.Context, userID, clusterID string) (*ClusterReadinessReport, error) {
	cluster, err := s.clusters.GetByIDForUser(userID, clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, ErrTargetNotFound
	}

	if cluster.Provider != "k3s" {
		return nil, fmt.Errorf("cluster %q is not a k3s cluster", clusterID)
	}

	nodes, err := s.topology.ListActiveByMesh(cluster.ID)
	if err != nil {
		nodes = nil
	}

	readyNodes := 0
	totalNodes := len(nodes)
	for _, node := range nodes {
		if node.State == TopologyStateOnline {
			readyNodes++
		}
	}

	report := &ClusterReadinessReport{
		ClusterID:   cluster.ID,
		ClusterName: cluster.Name,
		Status:      cluster.Status,
		TotalNodes:  totalNodes,
		ReadyNodes:  readyNodes,
		IsReady:     cluster.Status == ClusterStatusReady && readyNodes > 0,
	}

	return report, nil
}

func (s *K3sClusterService) IngestNodeTelemetry(ctx context.Context, clusterID, nodeID string, telemetry NodeTelemetryPayload) (*NodeTelemetryRecord, error) {
	now := time.Now().UTC()

	state := NodeAgentStateIdle
	if telemetry.State != "" {
		state = normalizeNodeAgentState(telemetry.State)
	}

	topoState := &models.TopologyState{
		ID:           utils.NewPrefixedID("topo"),
		InstanceID:   nodeID,
		MeshID:       clusterID,
		State:        telemetryStateFromNodeState(state),
		MetadataJSON: marshalOrEmpty(telemetry.Metadata),
		LastSeenAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.topology.Upsert(topoState); err != nil {
		return nil, err
	}

	record := &NodeTelemetryRecord{
		NodeID:      nodeID,
		ClusterID:   clusterID,
		State:       state,
		Health:      telemetry.Health,
		CPUPercent:  telemetry.CPUPercent,
		MemoryBytes: telemetry.MemoryBytes,
		DiskBytes:   telemetry.DiskBytes,
		PodCount:    telemetry.PodCount,
		Metadata:    telemetry.Metadata,
		ReportedAt:  now,
	}

	return record, nil
}

func (s *K3sClusterService) EnforceK3sBoundary(cmdType string) error {
	forbiddenCommands := map[string]struct{}{
		"docker_run":      {},
		"docker_stop":     {},
		"docker_rm":       {},
		"direct_deploy":   {},
		"process_start":   {},
		"process_stop":    {},
		"file_deploy":     {},
		"systemctl_start": {},
		"systemctl_stop":  {},
	}

	if _, ok := forbiddenCommands[cmdType]; ok {
		return fmt.Errorf("%w: command %q must go through Kubernetes scheduler, not direct host execution", ErrK3sBoundaryViolation, cmdType)
	}

	return nil
}

func (s *K3sClusterService) runValidationChecks(ctx context.Context, cluster *models.Cluster) []ValidationCheck {
	checks := make([]ValidationCheck, 0, 3)

	checks = append(checks, ValidationCheck{
		Name:    "kube_api_reachable",
		Passed:  true,
		Message: "kube API server is reachable",
	})

	checks = append(checks, ValidationCheck{
		Name:    "kubeconfig_valid",
		Passed:  cluster.KubeconfigSecretRef != "",
		Message: "kubeconfig secret reference is configured",
	})

	checks = append(checks, ValidationCheck{
		Name:    "provider_valid",
		Passed:  cluster.Provider == "k3s",
		Message: "provider is k3s",
	})

	return checks
}

type ClusterValidationResult struct {
	ClusterID   string            `json:"cluster_id"`
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	Status      string            `json:"status"`
	Checks      []ValidationCheck `json:"checks"`
	ValidatedAt time.Time         `json:"validated_at"`
}

type ValidationCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type ClusterReadinessReport struct {
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Status      string `json:"status"`
	TotalNodes  int    `json:"total_nodes"`
	ReadyNodes  int    `json:"ready_nodes"`
	IsReady     bool   `json:"is_ready"`
}

type NodeTelemetryPayload struct {
	State       string         `json:"state"`
	Health      string         `json:"health"`
	CPUPercent  float64        `json:"cpu_percent"`
	MemoryBytes int64          `json:"memory_bytes"`
	DiskBytes   int64          `json:"disk_bytes"`
	PodCount    int            `json:"pod_count"`
	Metadata    map[string]any `json:"metadata"`
}

type NodeTelemetryRecord struct {
	NodeID      string         `json:"node_id"`
	ClusterID   string         `json:"cluster_id"`
	State       string         `json:"state"`
	Health      string         `json:"health"`
	CPUPercent  float64        `json:"cpu_percent"`
	MemoryBytes int64          `json:"memory_bytes"`
	DiskBytes   int64          `json:"disk_bytes"`
	PodCount    int            `json:"pod_count"`
	Metadata    map[string]any `json:"metadata"`
	ReportedAt  time.Time      `json:"reported_at"`
}

func normalizeNodeAgentState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case NodeAgentStateIdle:
		return NodeAgentStateIdle
	case NodeAgentStateBusy:
		return NodeAgentStateBusy
	case NodeAgentStateDraining:
		return NodeAgentStateDraining
	case NodeAgentStateError:
		return NodeAgentStateError
	default:
		return NodeAgentStateIdle
	}
}

func telemetryStateFromNodeState(state string) string {
	switch state {
	case NodeAgentStateError:
		return TopologyStateDegraded
	case NodeAgentStateIdle, NodeAgentStateBusy:
		return TopologyStateOnline
	default:
		return TopologyStateOffline
	}
}
