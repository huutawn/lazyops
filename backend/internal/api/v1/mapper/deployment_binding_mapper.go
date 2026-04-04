package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateDeploymentBindingCommand(userID, role, projectID string, req requestdto.CreateDeploymentBindingRequest) service.CreateDeploymentBindingCommand {
	return service.CreateDeploymentBindingCommand{
		RequesterUserID:     userID,
		RequesterRole:       role,
		ProjectID:           projectID,
		Name:                req.Name,
		TargetRef:           req.TargetRef,
		RuntimeMode:         req.RuntimeMode,
		TargetKind:          req.TargetKind,
		TargetID:            req.TargetID,
		PlacementPolicy:     req.PlacementPolicy,
		DomainPolicy:        req.DomainPolicy,
		CompatibilityPolicy: req.CompatibilityPolicy,
		ScaleToZeroPolicy:   req.ScaleToZeroPolicy,
	}
}

func ToDeploymentBindingResponse(record service.DeploymentBindingRecord) responsedto.DeploymentBindingResponse {
	return responsedto.DeploymentBindingResponse{
		ID:                  record.ID,
		ProjectID:           record.ProjectID,
		Name:                record.Name,
		TargetRef:           record.TargetRef,
		RuntimeMode:         record.RuntimeMode,
		TargetKind:          record.TargetKind,
		TargetID:            record.TargetID,
		PlacementPolicy:     record.PlacementPolicy,
		DomainPolicy:        record.DomainPolicy,
		CompatibilityPolicy: record.CompatibilityPolicy,
		ScaleToZeroPolicy:   record.ScaleToZeroPolicy,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
	}
}

func ToDeploymentBindingListResponse(result service.DeploymentBindingListResult) responsedto.DeploymentBindingListResponse {
	items := make([]responsedto.DeploymentBindingResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, ToDeploymentBindingResponse(item))
	}

	return responsedto.DeploymentBindingListResponse{Items: items}
}
