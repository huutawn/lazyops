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
