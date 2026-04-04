package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToBuildCallbackCommand(req requestdto.BuildCallbackRequest) service.BuildCallbackCommand {
	return service.BuildCallbackCommand{
		BuildJobID:       req.BuildJobID,
		ProjectID:        req.ProjectID,
		CommitSHA:        req.CommitSHA,
		Status:           req.Status,
		ImageRef:         req.ImageRef,
		ImageDigest:      req.ImageDigest,
		DetectedServices: req.Metadata.DetectedServices,
	}
}

func ToBuildCallbackResponse(result service.BuildCallbackResult) responsedto.BuildCallbackResponse {
	response := responsedto.BuildCallbackResponse{
		BuildJob: toBuildJobResponse(result.BuildJob),
	}
	if result.Revision != nil {
		revisionServices := make([]responsedto.ProjectServiceResponse, 0, len(result.Revision.Services))
		for _, item := range result.Revision.Services {
			revisionServices = append(revisionServices, toBlueprintServiceResponse(item))
		}
		response.Revision = &responsedto.DesiredStateRevisionResponse{
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
		}
	}
	return response
}

func toBuildJobResponse(record service.BuildJobRecord) responsedto.BuildJobResponse {
	return responsedto.BuildJobResponse{
		ID:                record.ID,
		ProjectID:         record.ProjectID,
		ProjectRepoLinkID: record.ProjectRepoLinkID,
		GitHubDeliveryID:  record.GitHubDeliveryID,
		TriggerKind:       record.TriggerKind,
		Status:            record.Status,
		CommitSHA:         record.CommitSHA,
		TrackedBranch:     record.TrackedBranch,
		PullRequestNumber: record.PullRequestNumber,
		RetryCount:        record.RetryCount,
		MaxAttempts:       record.MaxAttempts,
		ArtifactMetadata: responsedto.BuildArtifactMetadataResponse{
			CommitSHA:        record.ArtifactMetadata.CommitSHA,
			ArtifactRef:      record.ArtifactMetadata.ArtifactRef,
			ImageRef:         record.ArtifactMetadata.ImageRef,
			ImageDigest:      record.ArtifactMetadata.ImageDigest,
			DetectedServices: record.ArtifactMetadata.DetectedServices,
		},
		StartedAt:   record.StartedAt,
		CompletedAt: record.CompletedAt,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}
