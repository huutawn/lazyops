package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"lazyops-server/internal/models"
	backendruntime "lazyops-server/internal/runtime"
)

const clusterPlacementLabelKey = "lazyops.io/instance-id"

type ClusterNodeService struct {
	projects   ProjectStore
	bindings   DeploymentBindingStore
	clusters   ClusterStore
	instances  *InstanceService
	topology   TopologyStateStore
	sshInstall *InstanceSSHInstallService
	control    *ControlService
}

func NewClusterNodeService(
	projects ProjectStore,
	bindings DeploymentBindingStore,
	clusters ClusterStore,
	instances *InstanceService,
	topology TopologyStateStore,
	sshInstall *InstanceSSHInstallService,
	control *ControlService,
) *ClusterNodeService {
	return &ClusterNodeService{
		projects:   projects,
		bindings:   bindings,
		clusters:   clusters,
		instances:  instances,
		topology:   topology,
		sshInstall: sshInstall,
		control:    control,
	}
}

func (s *ClusterNodeService) ConnectNodeViaSSH(ctx context.Context, cmd ConnectClusterNodeSSHCommand) (*ConnectClusterNodeSSHResult, error) {
	if s == nil || s.instances == nil || s.clusters == nil || s.sshInstall == nil {
		return nil, ErrInvalidInput
	}

	userID := strings.TrimSpace(cmd.UserID)
	clusterID := strings.TrimSpace(cmd.ClusterID)
	if userID == "" || clusterID == "" {
		return nil, ErrInvalidInput
	}

	cluster, err := s.clusters.GetByIDForUser(userID, clusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, ErrTargetNotFound
	}
	if cluster.InstanceID == nil || strings.TrimSpace(*cluster.InstanceID) == "" {
		return nil, ErrClusterNotReady
	}
	meta := DecodeManagedClusterMetadata(*cluster)
	if strings.TrimSpace(meta.JoinServerURL) == "" || strings.TrimSpace(meta.JoinToken) == "" {
		return nil, ErrClusterNotReady
	}

	instanceName := strings.TrimSpace(cmd.InstanceName)
	if instanceName == "" {
		instanceName = "srv-" + normalizeBindingTargetRef(strings.TrimSpace(cmd.Host))
	}
	instanceSummary, _, err := s.resolveOrCreateSSHInstance(userID, instanceName, CreateInstanceCommand{
		UserID:    userID,
		Name:      instanceName,
		PublicIP:  cmd.PublicIP,
		PrivateIP: cmd.PrivateIP,
		Labels:    cmd.Labels,
	})
	if err != nil {
		return nil, err
	}

	joinResult, err := s.sshInstall.JoinClusterNode(ctx, JoinClusterNodeSSHCommand{
		UserID:             userID,
		ClusterID:          cluster.ID,
		InstanceID:         instanceSummary.ID,
		Host:               cmd.Host,
		Port:               cmd.Port,
		Username:           cmd.Username,
		Password:           cmd.Password,
		PrivateKey:         cmd.PrivateKey,
		HostKeyFingerprint: cmd.HostKeyFingerprint,
		ControlPlaneURL:    cmd.ControlPlaneURL,
		JoinServerURL:      meta.JoinServerURL,
		JoinToken:          meta.JoinToken,
	})
	if err != nil {
		return nil, err
	}

	if err := s.labelJoinedNode(ctx, *cluster, joinResult.NodeName, instanceSummary.ID); err != nil {
		return nil, err
	}
	joinResult.LabeledByControl = true
	joinResult.PlacementLabelKey = clusterPlacementLabelKey
	joinResult.PlacementLabelValue = instanceSummary.ID
	joinResult.Stages = buildJoinClusterNodeStages(joinClusterNodeMetadata{
		NodeName:        joinResult.NodeName,
		AgentJoined:     true,
		K3sAgentReady:   true,
		RuntimePrepared: true,
	}, true)

	if s.topology != nil {
		state := buildClusterTopologyState(instanceSummary.ID, cluster.ID, joinResult.NodeName, "cluster_join_ssh")
		state.MetadataJSON = marshalOrEmpty(map[string]any{
			"k8s_node_name": joinResult.NodeName,
			"label_key":     clusterPlacementLabelKey,
			"label_value":   instanceSummary.ID,
			"provisioned_by": "cluster_join_ssh",
		})
		if err := s.topology.Upsert(state); err != nil {
			return nil, err
		}
	}

	return &ConnectClusterNodeSSHResult{
		ClusterID: cluster.ID,
		Instance:  instanceSummary,
		Join:      *joinResult,
	}, nil
}

func (s *ClusterNodeService) ListClusterNodes(userID, clusterID string) (*ClusterNodeListResult, error) {
	if s == nil || s.clusters == nil || s.instances == nil {
		return nil, ErrInvalidInput
	}
	cluster, err := s.clusters.GetByIDForUser(strings.TrimSpace(userID), strings.TrimSpace(clusterID))
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, ErrTargetNotFound
	}

	nodes := make(map[string]ClusterNodeRecord)
	if s.topology != nil {
		states, err := s.topology.ListActiveByMesh(cluster.ID)
		if err != nil {
			return nil, err
		}
		for _, state := range states {
			instance, getErr := s.instances.instances.GetByID(state.InstanceID)
			if getErr != nil || instance == nil {
				continue
			}
			record, convErr := toClusterNodeRecord(cluster.ID, state, *instance)
			if convErr != nil {
				return nil, convErr
			}
			nodes[record.InstanceID] = record
		}
	}
	if cluster.InstanceID != nil && strings.TrimSpace(*cluster.InstanceID) != "" {
		if _, ok := nodes[strings.TrimSpace(*cluster.InstanceID)]; !ok {
			instance, err := s.instances.instances.GetByID(strings.TrimSpace(*cluster.InstanceID))
			if err != nil {
				return nil, err
			}
			if instance != nil {
				summary, convErr := ToInstanceSummary(*instance)
				if convErr != nil {
					return nil, convErr
				}
				nodes[summary.ID] = ClusterNodeRecord{
					ClusterID:   cluster.ID,
					InstanceID:  summary.ID,
					Name:        summary.Name,
					Status:      summary.Status,
					K8sNodeName: "",
					Labels:      summary.Labels,
					LastSeenAt:  nil,
					IsReady:     normalizeClusterStatus(cluster.Status) == ClusterStatusReady,
				}
			}
		}
	}

	items := make([]ClusterNodeRecord, 0, len(nodes))
	for _, item := range nodes {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsReady != items[j].IsReady {
			return items[i].IsReady
		}
		return items[i].Name < items[j].Name
	})
	return &ClusterNodeListResult{Items: items}, nil
}

func (s *ClusterNodeService) ListPlacementNodes(userID, role, projectID string) (*PlacementNodeListResult, error) {
	project, err := resolveProjectForAccess(s.projects, userID, role, projectID)
	if err != nil {
		return nil, err
	}
	clusterID := ""
	if project.ClusterID != nil {
		clusterID = strings.TrimSpace(*project.ClusterID)
	}
	if clusterID == "" && s.bindings != nil {
		binding, bindErr := s.resolvePrimaryClusterBinding(project.ID)
		if bindErr != nil {
			return nil, bindErr
		}
		if binding != nil && strings.TrimSpace(binding.TargetKind) == "cluster" {
			clusterID = strings.TrimSpace(binding.TargetID)
		}
	}
	if clusterID == "" {
		return &PlacementNodeListResult{Items: []ClusterNodeRecord{}}, nil
	}
	nodes, err := s.ListClusterNodes(project.UserID, clusterID)
	if err != nil {
		return nil, err
	}
	ready := make([]ClusterNodeRecord, 0, len(nodes.Items))
	for _, item := range nodes.Items {
		if item.IsReady {
			ready = append(ready, item)
		}
	}
	return &PlacementNodeListResult{
		ClusterID: clusterID,
		Items:     ready,
	}, nil
}

func (s *ClusterNodeService) resolveOrCreateSSHInstance(userID, instanceName string, createCmd CreateInstanceCommand) (InstanceSummary, bool, error) {
	createResult, err := s.instances.Create(createCmd)
	if err == nil {
		return createResult.Instance, true, nil
	}
	if !errors.Is(err, ErrInstanceNameExists) {
		return InstanceSummary{}, false, err
	}
	instances, listErr := s.instances.List(userID)
	if listErr != nil {
		return InstanceSummary{}, false, listErr
	}
	for _, item := range instances.Items {
		if strings.TrimSpace(item.Name) == strings.TrimSpace(instanceName) {
			return item, false, nil
		}
	}
	return InstanceSummary{}, false, err
}

func (s *ClusterNodeService) resolvePrimaryClusterBinding(projectID string) (*models.DeploymentBinding, error) {
	autoBinding, err := s.bindings.GetByTargetRefForProject(projectID, "auto-primary")
	if err != nil {
		return nil, err
	}
	if autoBinding != nil && strings.TrimSpace(autoBinding.TargetKind) == "cluster" {
		return autoBinding, nil
	}
	items, err := s.bindings.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.TargetKind) == "cluster" {
			binding := item
			return &binding, nil
		}
	}
	return nil, nil
}

func (s *ClusterNodeService) labelJoinedNode(ctx context.Context, cluster models.Cluster, nodeName, instanceID string) error {
	if s.control == nil || s.instances == nil || cluster.InstanceID == nil || strings.TrimSpace(*cluster.InstanceID) == "" {
		return nil
	}
	controlPlane, err := s.instances.instances.GetByID(strings.TrimSpace(*cluster.InstanceID))
	if err != nil {
		return err
	}
	if controlPlane == nil || controlPlane.AgentID == nil || strings.TrimSpace(*controlPlane.AgentID) == "" {
		return ErrRolloutAgentUnavailable
	}
	dispatchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	result, err := s.control.DispatchCommand(dispatchCtx, strings.TrimSpace(*controlPlane.AgentID), backendruntime.AgentCommand{
		Type:      backendruntime.CommandTypeLabelK3sNode,
		ProjectID: "",
		Source:    "cluster_node_service",
		Payload: map[string]any{
			"node_name":   strings.TrimSpace(nodeName),
			"label_key":   clusterPlacementLabelKey,
			"label_value": strings.TrimSpace(instanceID),
		},
	})
	if err != nil {
		return err
	}
	tracked, err := s.control.WaitForCommand(dispatchCtx, result.RequestID)
	if err != nil {
		return err
	}
	if tracked.State != CommandStateDone {
		if tracked.Error != "" {
			return fmt.Errorf("label joined node: %s", tracked.Error)
		}
		return fmt.Errorf("label joined node command %s ended in state %s", tracked.RequestID, tracked.State)
	}
	return nil
}

func toClusterNodeRecord(clusterID string, state models.TopologyState, instance models.Instance) (ClusterNodeRecord, error) {
	summary, err := ToInstanceSummary(instance)
	if err != nil {
		return ClusterNodeRecord{}, err
	}
	metadata := map[string]any{}
	if strings.TrimSpace(state.MetadataJSON) != "" && strings.TrimSpace(state.MetadataJSON) != "{}" {
		if err := json.Unmarshal([]byte(state.MetadataJSON), &metadata); err != nil {
			return ClusterNodeRecord{}, err
		}
	}
	lastSeen := state.LastSeenAt
	return ClusterNodeRecord{
		ClusterID:   clusterID,
		InstanceID:  summary.ID,
		Name:        summary.Name,
		Status:      state.State,
		K8sNodeName: stringFromAny(metadata["k8s_node_name"]),
		Labels:      summary.Labels,
		LastSeenAt:  &lastSeen,
		IsReady:     state.State == TopologyStateOnline,
	}, nil
}
