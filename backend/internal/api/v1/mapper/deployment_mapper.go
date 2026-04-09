package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateDeploymentCommand(userID, role, projectID string, req requestdto.CreateDeploymentRequest) service.CreateDeploymentCommand {
	return service.CreateDeploymentCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		BlueprintID:     req.BlueprintID,
		TriggerKind:     req.TriggerKind,
	}
}

func ToCreateDeploymentResponse(result service.CreateDeploymentResult) responsedto.CreateDeploymentResponse {
	revisionServices := make([]responsedto.ProjectServiceResponse, 0, len(result.Revision.Services))
	for _, item := range result.Revision.Services {
		revisionServices = append(revisionServices, toBlueprintServiceResponse(item))
	}

	return responsedto.CreateDeploymentResponse{
		Revision: responsedto.DesiredStateRevisionResponse{
			ID:                   result.Revision.ID,
			ProjectID:            result.Revision.ProjectID,
			BlueprintID:          result.Revision.BlueprintID,
			DeploymentBindingID:  result.Revision.DeploymentBindingID,
			CommitSHA:            result.Revision.CommitSHA,
			ArtifactRef:          result.Revision.ArtifactRef,
			ImageRef:             result.Revision.ImageRef,
			TriggerKind:          result.Revision.TriggerKind,
			Status:               result.Revision.Status,
			RuntimeMode:          result.Revision.RuntimeMode,
			Services:             revisionServices,
			DependencyBindings:   toDependencyBindingMaps(result.Revision.DependencyBindings),
			CompatibilityPolicy:  toCompatibilityPolicyMap(result.Revision.CompatibilityPolicy),
			MagicDomainPolicy:    toMagicDomainPolicyMap(result.Revision.MagicDomainPolicy),
			ScaleToZeroPolicy:    toScaleToZeroPolicyMap(result.Revision.ScaleToZeroPolicy),
			PlacementAssignments: toPlacementAssignmentResponses(result.Revision.PlacementAssignments),
			CreatedAt:            result.Revision.CreatedAt,
			UpdatedAt:            result.Revision.UpdatedAt,
		},
		Deployment: responsedto.DeploymentResponse{
			ID:          result.Deployment.ID,
			ProjectID:   result.Deployment.ProjectID,
			RevisionID:  result.Deployment.RevisionID,
			Status:      result.Deployment.Status,
			StartedAt:   result.Deployment.StartedAt,
			CompletedAt: result.Deployment.CompletedAt,
			CreatedAt:   result.Deployment.CreatedAt,
			UpdatedAt:   result.Deployment.UpdatedAt,
		},
	}
}

func ToDeploymentOverviewResponse(record service.DeploymentOverviewRecord) responsedto.DeploymentOverviewResponse {
	services := make([]responsedto.ProjectServiceResponse, 0, len(record.Services))
	for _, item := range record.Services {
		services = append(services, toBlueprintServiceResponse(item))
	}

	return responsedto.DeploymentOverviewResponse{
		ID:                   record.ID,
		ProjectID:            record.ProjectID,
		RevisionID:           record.RevisionID,
		Revision:             record.Revision,
		CommitSHA:            record.CommitSHA,
		ArtifactRef:          record.ArtifactRef,
		ImageRef:             record.ImageRef,
		TriggerKind:          record.TriggerKind,
		BuildState:           record.BuildState,
		RolloutState:         record.RolloutState,
		Promoted:             record.Promoted,
		TriggeredBy:          record.TriggeredBy,
		RuntimeMode:          record.RuntimeMode,
		Services:             services,
		PlacementAssignments: toPlacementAssignmentResponses(record.PlacementAssignments),
		StartedAt:            record.StartedAt,
		CompletedAt:          record.CompletedAt,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
	}
}

func ToDeploymentDetailResponse(record service.DeploymentDetailRecord) responsedto.DeploymentDetailResponse {
	timeline := make([]responsedto.DeploymentTimelineEventResponse, 0, len(record.Timeline))
	for _, item := range record.Timeline {
		timeline = append(timeline, responsedto.DeploymentTimelineEventResponse{
			Timestamp:   item.Timestamp,
			State:       item.State,
			Label:       item.Label,
			Description: item.Description,
		})
	}

	return responsedto.DeploymentDetailResponse{
		DeploymentOverviewResponse: ToDeploymentOverviewResponse(record.DeploymentOverviewRecord),
		Timeline:                   timeline,
		CanRollback:                record.CanRollback,
		CanPromote:                 record.CanPromote,
		CanCancel:                  record.CanCancel,
	}
}

func ToDeploymentListResponse(items []service.DeploymentOverviewRecord) responsedto.DeploymentListResponse {
	out := make([]responsedto.DeploymentOverviewResponse, 0, len(items))
	for _, item := range items {
		out = append(out, ToDeploymentOverviewResponse(item))
	}
	return responsedto.DeploymentListResponse{Items: out}
}
