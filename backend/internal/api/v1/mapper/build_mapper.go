package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToBuildCallbackCommand(req requestdto.BuildCallbackRequest) service.BuildCallbackCommand {
	var suggestedHealthcheck *service.BuildSuggestedHealthcheckRecord
	if req.Metadata.SuggestedHealthcheck != nil {
		suggestedHealthcheck = &service.BuildSuggestedHealthcheckRecord{
			Path: req.Metadata.SuggestedHealthcheck.Path,
			Port: req.Metadata.SuggestedHealthcheck.Port,
		}
	}
	detectedPorts := make([]service.ServiceDetectedPortRecord, 0, len(req.Metadata.DetectedPorts))
	for _, item := range req.Metadata.DetectedPorts {
		detectedPorts = append(detectedPorts, service.ServiceDetectedPortRecord{
			Port:     item.Port,
			Protocol: item.Protocol,
			Name:     item.Name,
			Exposed:  item.Exposed,
		})
	}

	return service.BuildCallbackCommand{
		BuildJobID:              req.BuildJobID,
		ProjectID:               req.ProjectID,
		CommitSHA:               req.CommitSHA,
		Status:                  req.Status,
		ImageRef:                req.ImageRef,
		ImageDigest:             req.ImageDigest,
		DetectedServices:        req.Metadata.DetectedServices,
		DetectedPorts:           detectedPorts,
		PortDetectionSource:     req.Metadata.PortDetectionSource,
		PortDetectionConfidence: req.Metadata.PortDetectionConfidence,
		SuggestedTargetPort:     req.Metadata.SuggestedTargetPort,
		DetectedFramework:       req.Metadata.DetectedFramework,
		SuggestedHealthcheck:    suggestedHealthcheck,
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
		revisionSpecs := make([]responsedto.ProjectServiceResponse, 0, len(result.Revision.ServiceSpecs))
		for _, item := range result.Revision.ServiceSpecs {
			revisionSpecs = append(revisionSpecs, responsedto.ProjectServiceResponse{
				Name:           item.Name,
				Path:           item.Path,
				Kind:           item.Kind,
				Public:         item.Public,
				RuntimeProfile: item.RuntimeProfile,
				StartHint:      item.StartHint,
				ImageRef:       item.ImageRef,
				ImageDigest:    item.ImageDigest,
				DetectedPorts:  toBuildDetectedPortResponses(item.DetectedPorts),
				TargetPort:     item.TargetPort,
				ServicePort:    item.ServicePort,
				Replicas:       item.Replicas,
				EnvBundle:      item.EnvBundle,
				PVCSpec:        item.PVCSpec,
				DeployStrategy: item.DeployStrategy,
				Healthcheck:    item.Healthcheck,
			})
		}
		response.Revision = &responsedto.DesiredStateRevisionResponse{
			ID:                   result.Revision.ID,
			ProjectID:            result.Revision.ProjectID,
			BlueprintID:          result.Revision.BlueprintID,
			DeploymentBindingID:  result.Revision.DeploymentBindingID,
			Namespace:            result.Revision.Namespace,
			CommitSHA:            result.Revision.CommitSHA,
			ArtifactRef:          result.Revision.ArtifactRef,
			ImageRef:             result.Revision.ImageRef,
			TriggerKind:          result.Revision.TriggerKind,
			Status:               result.Revision.Status,
			RuntimeMode:          result.Revision.RuntimeMode,
			Services:             revisionServices,
			ServiceSpecs:         revisionSpecs,
			DependencyBindings:   toDependencyBindingMaps(result.Revision.DependencyBindings),
			InternalBindings:     toInternalBindingMaps(result.Revision.InternalBindings),
			CompatibilityPolicy:  toCompatibilityPolicyMap(result.Revision.CompatibilityPolicy),
			MagicDomainPolicy:    toMagicDomainPolicyMap(result.Revision.MagicDomainPolicy),
			ScaleToZeroPolicy:    toScaleToZeroPolicyMap(result.Revision.ScaleToZeroPolicy),
			PlacementAssignments: toPlacementAssignmentResponses(result.Revision.PlacementAssignments),
			ManifestBundle:       toManifestBundleMap(result.Revision.ManifestBundle),
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
			CommitSHA:               record.ArtifactMetadata.CommitSHA,
			ArtifactRef:             record.ArtifactMetadata.ArtifactRef,
			ImageRef:                record.ArtifactMetadata.ImageRef,
			ImageDigest:             record.ArtifactMetadata.ImageDigest,
			AppliedServices:         record.ArtifactMetadata.AppliedServices,
			DetectedServices:        record.ArtifactMetadata.DetectedServices,
			DetectedPorts:           toBuildDetectedPortResponses(record.ArtifactMetadata.DetectedPorts),
			PortDetectionSource:     record.ArtifactMetadata.PortDetectionSource,
			PortDetectionConfidence: record.ArtifactMetadata.PortDetectionConfidence,
			SuggestedTargetPort:     record.ArtifactMetadata.SuggestedTargetPort,
			DetectedFramework:       record.ArtifactMetadata.DetectedFramework,
			SuggestedHealthcheck: toBuildSuggestedHealthcheckResponse(
				record.ArtifactMetadata.SuggestedHealthcheck,
			),
		},
		StartedAt:   record.StartedAt,
		CompletedAt: record.CompletedAt,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func toBuildDetectedPortResponses(items []service.ServiceDetectedPortRecord) []responsedto.BuildDetectedPortResponse {
	if len(items) == 0 {
		return nil
	}
	out := make([]responsedto.BuildDetectedPortResponse, 0, len(items))
	for _, item := range items {
		out = append(out, responsedto.BuildDetectedPortResponse{
			Port:     item.Port,
			Protocol: item.Protocol,
			Name:     item.Name,
			Exposed:  item.Exposed,
		})
	}
	return out
}

func toBuildSuggestedHealthcheckResponse(item *service.BuildSuggestedHealthcheckRecord) *responsedto.BuildSuggestedHealthcheckResponse {
	if item == nil {
		return nil
	}
	return &responsedto.BuildSuggestedHealthcheckResponse{
		Path: item.Path,
		Port: item.Port,
	}
}
