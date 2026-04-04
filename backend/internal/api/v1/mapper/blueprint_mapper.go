package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCompileBlueprintCommand(userID, role, projectID string, req requestdto.CompileBlueprintRequest) service.CompileBlueprintCommand {
	return service.CompileBlueprintCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		SourceRef:       req.SourceRef,
		TriggerKind:     req.TriggerKind,
		Artifact: service.BlueprintArtifactMetadata{
			CommitSHA:   req.Artifact.CommitSHA,
			ArtifactRef: req.Artifact.ArtifactRef,
			ImageRef:    req.Artifact.ImageRef,
		},
		LazyopsYAMLRaw: req.LazyopsYAML,
	}
}

func ToCompileBlueprintResponse(result service.CompileBlueprintResult) responsedto.CompileBlueprintResponse {
	services := make([]responsedto.ProjectServiceResponse, 0, len(result.Services))
	for _, item := range result.Services {
		services = append(services, toProjectServiceResponse(item))
	}

	compiledServices := make([]responsedto.ProjectServiceResponse, 0, len(result.Blueprint.Compiled.Services))
	for _, item := range result.Blueprint.Compiled.Services {
		compiledServices = append(compiledServices, toBlueprintServiceResponse(item))
	}

	draftServices := make([]responsedto.ProjectServiceResponse, 0, len(result.DesiredRevisionDraft.Services))
	for _, item := range result.DesiredRevisionDraft.Services {
		draftServices = append(draftServices, toBlueprintServiceResponse(item))
	}

	return responsedto.CompileBlueprintResponse{
		Services: services,
		Blueprint: responsedto.BlueprintResponse{
			ID:         result.Blueprint.ID,
			ProjectID:  result.Blueprint.ProjectID,
			SourceKind: result.Blueprint.SourceKind,
			SourceRef:  result.Blueprint.SourceRef,
			Compiled: responsedto.BlueprintCompiledResponse{
				ProjectID:           result.Blueprint.Compiled.ProjectID,
				RuntimeMode:         result.Blueprint.Compiled.RuntimeMode,
				Repo:                toBlueprintRepoStateResponse(result.Blueprint.Compiled.Repo),
				Binding:             ToDeploymentBindingResponse(result.Blueprint.Compiled.Binding),
				Services:            compiledServices,
				DependencyBindings:  toDependencyBindingMaps(result.Blueprint.Compiled.DependencyBindings),
				CompatibilityPolicy: toCompatibilityPolicyMap(result.Blueprint.Compiled.CompatibilityPolicy),
				MagicDomainPolicy:   toMagicDomainPolicyMap(result.Blueprint.Compiled.MagicDomainPolicy),
				ScaleToZeroPolicy:   toScaleToZeroPolicyMap(result.Blueprint.Compiled.ScaleToZeroPolicy),
				ArtifactMetadata: responsedto.BlueprintArtifactMetadataResponse{
					CommitSHA:   result.Blueprint.Compiled.ArtifactMetadata.CommitSHA,
					ArtifactRef: result.Blueprint.Compiled.ArtifactMetadata.ArtifactRef,
					ImageRef:    result.Blueprint.Compiled.ArtifactMetadata.ImageRef,
				},
			},
			CreatedAt: result.Blueprint.CreatedAt,
		},
		DesiredRevisionDraft: responsedto.DesiredRevisionDraftResponse{
			RevisionID:           result.DesiredRevisionDraft.RevisionID,
			ProjectID:            result.DesiredRevisionDraft.ProjectID,
			BlueprintID:          result.DesiredRevisionDraft.BlueprintID,
			DeploymentBindingID:  result.DesiredRevisionDraft.DeploymentBindingID,
			CommitSHA:            result.DesiredRevisionDraft.CommitSHA,
			ArtifactRef:          result.DesiredRevisionDraft.ArtifactRef,
			ImageRef:             result.DesiredRevisionDraft.ImageRef,
			TriggerKind:          result.DesiredRevisionDraft.TriggerKind,
			RuntimeMode:          result.DesiredRevisionDraft.RuntimeMode,
			Services:             draftServices,
			DependencyBindings:   toDependencyBindingMaps(result.DesiredRevisionDraft.DependencyBindings),
			CompatibilityPolicy:  toCompatibilityPolicyMap(result.DesiredRevisionDraft.CompatibilityPolicy),
			MagicDomainPolicy:    toMagicDomainPolicyMap(result.DesiredRevisionDraft.MagicDomainPolicy),
			ScaleToZeroPolicy:    toScaleToZeroPolicyMap(result.DesiredRevisionDraft.ScaleToZeroPolicy),
			PlacementAssignments: toPlacementAssignmentResponses(result.DesiredRevisionDraft.PlacementAssignments),
		},
	}
}

func toProjectServiceResponse(item service.ProjectServiceRecord) responsedto.ProjectServiceResponse {
	return responsedto.ProjectServiceResponse{
		ID:             item.ID,
		ProjectID:      item.ProjectID,
		Name:           item.Name,
		Path:           item.Path,
		Public:         item.Public,
		RuntimeProfile: item.RuntimeProfile,
		Healthcheck:    item.Healthcheck,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func toBlueprintServiceResponse(item service.BlueprintServiceContractRecord) responsedto.ProjectServiceResponse {
	return responsedto.ProjectServiceResponse{
		Name:           item.Name,
		Path:           item.Path,
		Public:         item.Public,
		RuntimeProfile: item.RuntimeProfile,
		StartHint:      item.StartHint,
		Healthcheck:    item.Healthcheck,
	}
}

func toBlueprintRepoStateResponse(item service.BlueprintRepoStateRecord) responsedto.BlueprintRepoStateResponse {
	return responsedto.BlueprintRepoStateResponse{
		ProjectRepoLinkID: item.ProjectRepoLinkID,
		RepoOwner:         item.RepoOwner,
		RepoName:          item.RepoName,
		RepoFullName:      item.RepoFullName,
		TrackedBranch:     item.TrackedBranch,
		PreviewEnabled:    item.PreviewEnabled,
	}
}

func toDependencyBindingMaps(items []service.LazyopsYAMLDependencyBinding) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"service":        item.Service,
			"alias":          item.Alias,
			"target_service": item.TargetService,
			"protocol":       item.Protocol,
			"local_endpoint": item.LocalEndpoint,
		})
	}
	return out
}

func toCompatibilityPolicyMap(item service.LazyopsYAMLCompatibilityPolicy) map[string]any {
	return map[string]any{
		"env_injection":       item.EnvInjection,
		"managed_credentials": item.ManagedCredentials,
		"localhost_rescue":    item.LocalhostRescue,
	}
}

func toMagicDomainPolicyMap(item service.LazyopsYAMLMagicDomainPolicy) map[string]any {
	return map[string]any{
		"enabled":  item.Enabled,
		"provider": item.Provider,
	}
}

func toScaleToZeroPolicyMap(item service.LazyopsYAMLScaleToZeroPolicy) map[string]any {
	return map[string]any{
		"enabled": item.Enabled,
	}
}

func toPlacementAssignmentResponses(items []service.PlacementAssignmentRecord) []responsedto.PlacementAssignmentResponse {
	out := make([]responsedto.PlacementAssignmentResponse, 0, len(items))
	for _, item := range items {
		out = append(out, responsedto.PlacementAssignmentResponse{
			ServiceName: item.ServiceName,
			TargetID:    item.TargetID,
			TargetKind:  item.TargetKind,
			Labels:      item.Labels,
		})
	}
	return out
}
