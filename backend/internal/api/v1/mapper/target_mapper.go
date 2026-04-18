package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateMeshNetworkCommand(userID string, req requestdto.CreateMeshNetworkRequest) service.CreateMeshNetworkCommand {
	return service.CreateMeshNetworkCommand{
		UserID:   userID,
		Name:     req.Name,
		Provider: req.Provider,
		CIDR:     req.CIDR,
	}
}

func ToMeshNetworkSummaryResponse(summary service.MeshNetworkSummary) responsedto.MeshNetworkSummaryResponse {
	return responsedto.MeshNetworkSummaryResponse{
		ID:         summary.ID,
		TargetKind: summary.TargetKind,
		Name:       summary.Name,
		Provider:   summary.Provider,
		CIDR:       summary.CIDR,
		Status:     summary.Status,
		CreatedAt:  summary.CreatedAt,
		UpdatedAt:  summary.UpdatedAt,
	}
}

func ToMeshNetworkListResponse(result service.MeshNetworkListResult) responsedto.MeshNetworkListResponse {
	items := make([]responsedto.MeshNetworkSummaryResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, ToMeshNetworkSummaryResponse(item))
	}

	return responsedto.MeshNetworkListResponse{Items: items}
}

func ToCreateClusterCommand(userID string, req requestdto.CreateClusterRequest) service.CreateClusterCommand {
	return service.CreateClusterCommand{
		UserID:              userID,
		Name:                req.Name,
		Provider:            req.Provider,
		KubeconfigSecretRef: req.KubeconfigSecretRef,
	}
}

func ToClusterSummaryResponse(summary service.ClusterSummary) responsedto.ClusterSummaryResponse {
	return responsedto.ClusterSummaryResponse{
		ID:         summary.ID,
		TargetKind: summary.TargetKind,
		Name:       summary.Name,
		Provider:   summary.Provider,
		Status:     summary.Status,
		PublicIP:   summary.PublicIP,
		InstanceID: summary.InstanceID,
		CreatedAt:  summary.CreatedAt,
		UpdatedAt:  summary.UpdatedAt,
	}
}

func ToClusterListResponse(result service.ClusterListResult) responsedto.ClusterListResponse {
	items := make([]responsedto.ClusterSummaryResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, ToClusterSummaryResponse(item))
	}

	return responsedto.ClusterListResponse{Items: items}
}

func ToConnectClusterNodeSSHCommand(userID, clusterID string, req requestdto.ConnectClusterNodeSSHRequest) service.ConnectClusterNodeSSHCommand {
	return service.ConnectClusterNodeSSHCommand{
		UserID:             userID,
		ClusterID:          clusterID,
		InstanceName:       req.InstanceName,
		PublicIP:           req.PublicIP,
		PrivateIP:          req.PrivateIP,
		Labels:             req.Labels,
		Host:               req.SSHHost,
		Port:               req.SSHPort,
		Username:           req.SSHUsername,
		Password:           req.SSHPassword,
		PrivateKey:         req.SSHPrivateKey,
		HostKeyFingerprint: req.SSHHostKeyFingerprint,
		ControlPlaneURL:    req.ControlPlaneURL,
		AgentImage:         req.AgentImage,
		ContainerName:      req.ContainerName,
	}
}

func ToClusterNodeResponse(record service.ClusterNodeRecord) responsedto.ClusterNodeResponse {
	return responsedto.ClusterNodeResponse{
		ClusterID:   record.ClusterID,
		InstanceID:  record.InstanceID,
		Name:        record.Name,
		Status:      record.Status,
		K8sNodeName: record.K8sNodeName,
		Labels:      record.Labels,
		LastSeenAt:  record.LastSeenAt,
		IsReady:     record.IsReady,
	}
}

func ToClusterNodeListResponse(result service.ClusterNodeListResult) responsedto.ClusterNodeListResponse {
	items := make([]responsedto.ClusterNodeResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, ToClusterNodeResponse(item))
	}
	return responsedto.ClusterNodeListResponse{Items: items}
}

func ToConnectClusterNodeSSHResponse(result service.ConnectClusterNodeSSHResult) responsedto.ConnectClusterNodeSSHResponse {
	stages := make([]responsedto.BootstrapStepResponse, 0, len(result.Join.Stages))
	for _, stage := range result.Join.Stages {
		stages = append(stages, responsedto.BootstrapStepResponse{
			ID:      stage.ID,
			State:   stage.State,
			Summary: stage.Message,
			Actions: []responsedto.BootstrapStepActionResponse{},
		})
	}
	response := responsedto.ConnectClusterNodeSSHResponse{
		ClusterID: result.ClusterID,
		Instance:  ToInstanceSummaryResponse(result.Instance),
	}
	response.Join.ClusterID = result.Join.ClusterID
	response.Join.InstanceID = result.Join.InstanceID
	response.Join.StartedAt = result.Join.StartedAt
	response.Join.HostKeyFingerprint = result.Join.HostKeyFingerprint
	response.Join.NodeName = result.Join.NodeName
	response.Join.JoinServerURL = result.Join.JoinServerURL
	response.Join.LabeledByControl = result.Join.LabeledByControl
	response.Join.PlacementLabelKey = result.Join.PlacementLabelKey
	response.Join.PlacementLabelValue = result.Join.PlacementLabelValue
	response.Join.Stages = stages
	return response
}
