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
		ServiceIDs:      append([]string{}, req.ServiceIDs...),
	}
}

func ToCreateDeploymentResponse(result service.CreateDeploymentResult) responsedto.CreateDeploymentResponse {
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
			SourceType:     "repo",
			Public:         item.Public,
			RuntimeProfile: item.RuntimeProfile,
			PlacementMode:  item.PlacementMode,
			PlacementNodeID: item.PlacementNodeID,
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

	return responsedto.CreateDeploymentResponse{
		Revision: responsedto.DesiredStateRevisionResponse{
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
	serviceSpecs := make([]responsedto.ProjectServiceResponse, 0, len(record.ServiceSpecs))
	for _, item := range record.ServiceSpecs {
		serviceSpecs = append(serviceSpecs, responsedto.ProjectServiceResponse{
			Name:           item.Name,
			Path:           item.Path,
			Kind:           item.Kind,
			SourceType:     "repo",
			Public:         item.Public,
			RuntimeProfile: item.RuntimeProfile,
			PlacementMode:  item.PlacementMode,
			PlacementNodeID: item.PlacementNodeID,
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
		Namespace:            record.Namespace,
		RuntimeMode:          record.RuntimeMode,
		Services:             services,
		ServiceSpecs:         serviceSpecs,
		PlacementAssignments: toPlacementAssignmentResponses(record.PlacementAssignments),
		PublicURLs:           append([]string{}, record.PublicURLs...),
		PublicURLReason:      record.PublicURLReason,
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

	var incidentSummary *responsedto.DeploymentIncidentSummaryResponse
	if record.IncidentSummary != nil {
		var action *responsedto.DeploymentFixActionResponse
		if record.IncidentSummary.PrimaryAction != nil {
			action = &responsedto.DeploymentFixActionResponse{
				ID:     record.IncidentSummary.PrimaryAction.ID,
				Label:  record.IncidentSummary.PrimaryAction.Label,
				Href:   record.IncidentSummary.PrimaryAction.Href,
				Method: record.IncidentSummary.PrimaryAction.Method,
			}
		}
		incidentSummary = &responsedto.DeploymentIncidentSummaryResponse{
			State:         record.IncidentSummary.State,
			Headline:      record.IncidentSummary.Headline,
			Reason:        record.IncidentSummary.Reason,
			Recommended:   record.IncidentSummary.Recommended,
			IncidentID:    record.IncidentSummary.IncidentID,
			IncidentKind:  record.IncidentSummary.IncidentKind,
			IncidentLevel: record.IncidentSummary.IncidentLevel,
			PrimaryAction: action,
		}
	}

	return responsedto.DeploymentDetailResponse{
		DeploymentOverviewResponse: ToDeploymentOverviewResponse(record.DeploymentOverviewRecord),
		Timeline:                   timeline,
		CanRollback:                record.CanRollback,
		CanPromote:                 record.CanPromote,
		CanCancel:                  record.CanCancel,
		SafetyPolicy: responsedto.DeploymentSafetyPolicyResponse{
			AutoRollbackEnabled: record.SafetyPolicy.AutoRollbackEnabled,
			Triggers:            record.SafetyPolicy.Triggers,
			Description:         record.SafetyPolicy.Description,
		},
		IncidentSummary: incidentSummary,
	}
}

func ToDeploymentListResponse(items []service.DeploymentOverviewRecord) responsedto.DeploymentListResponse {
	out := make([]responsedto.DeploymentOverviewResponse, 0, len(items))
	for _, item := range items {
		out = append(out, ToDeploymentOverviewResponse(item))
	}
	return responsedto.DeploymentListResponse{Items: out}
}

func toInternalBindingMaps(items []service.InternalBindingRecord) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"service_name":      item.ServiceName,
			"alias":             item.Alias,
			"target_service":    item.TargetService,
			"host":              item.Host,
			"port":              item.Port,
			"protocol":          item.Protocol,
			"url":               item.URL,
			"connection_string": item.ConnectionString,
		})
	}
	return out
}

func toManifestBundleMap(item service.K3sManifestBundleRecord) map[string]any {
	if item.Namespace == "" && item.CombinedYAML == "" && len(item.Documents) == 0 {
		return nil
	}
	docs := make([]map[string]any, 0, len(item.Documents))
	for _, doc := range item.Documents {
		docs = append(docs, map[string]any{
			"name":    doc.Name,
			"kind":    doc.Kind,
			"path":    doc.Path,
			"content": doc.Content,
		})
	}
	return map[string]any{
		"namespace":     item.Namespace,
		"combined_yaml": item.CombinedYAML,
		"rollback_yaml": item.RollbackYAML,
		"generated_at":  item.GeneratedAt,
		"documents":     docs,
	}
}
