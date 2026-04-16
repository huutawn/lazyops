package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateProjectCommand(userID string, req requestdto.CreateProjectRequest) service.CreateProjectCommand {
	return service.CreateProjectCommand{
		UserID:           userID,
		Name:             req.Name,
		Slug:             req.Slug,
		NamespaceSlug:    req.NamespaceSlug,
		ClusterID:        req.ClusterID,
		RuntimeMode:      req.RuntimeMode,
		DefaultBranch:    req.DefaultBranch,
		InternalServices: req.InternalServices,
	}
}

func ToConfigureProjectServicesCommand(userID, role, projectID string, req requestdto.ConfigureProjectServicesRequest) service.ConfigureProjectServicesCommand {
	items := make([]service.ConfigureProjectServiceItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, service.ConfigureProjectServiceItem{
			Name:           item.Name,
			Path:           item.Path,
			Kind:           item.Kind,
			Public:         item.Public,
			RuntimeProfile: item.RuntimeProfile,
			StartHint:      item.StartHint,
			ImageRef:       item.ImageRef,
			ImageDigest:    item.ImageDigest,
			TargetPort:     item.TargetPort,
			ServicePort:    item.ServicePort,
			Replicas:       item.Replicas,
			EnvBundle:      item.EnvBundle,
			PVCSpec:        item.PVCSpec,
			DeployStrategy: item.DeployStrategy,
			Healthcheck:    item.Healthcheck,
		})
	}

	return service.ConfigureProjectServicesCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		Items:           items,
	}
}

func ToConfigureProjectInternalServicesCommand(userID, role, projectID string, req requestdto.ConfigureProjectInternalServicesRequest) service.ConfigureProjectInternalServicesCommand {
	return service.ConfigureProjectInternalServicesCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		Kinds:           req.Kinds,
	}
}

func ToUpsertProjectEnvCommand(userID, role, projectID string, req requestdto.UpsertProjectEnvRequest) service.UpsertProjectEnvCommand {
	return service.UpsertProjectEnvCommand{
		RequesterUserID: userID,
		RequesterRole:   role,
		ProjectID:       projectID,
		Content:         req.Content,
	}
}

func ToCreateProjectRepoLinkCommand(userID, role, projectID string, req requestdto.LinkProjectRepoRequest) service.CreateProjectRepoLinkCommand {
	return service.CreateProjectRepoLinkCommand{
		RequesterUserID:      userID,
		RequesterRole:        role,
		ProjectID:            projectID,
		GitHubInstallationID: req.GitHubInstallationID,
		GitHubRepoID:         req.GitHubRepoID,
		TrackedBranch:        req.TrackedBranch,
		PreviewEnabled:       req.PreviewEnabled,
	}
}

func ToProjectSummaryResponse(summary service.ProjectSummary) responsedto.ProjectSummaryResponse {
	return responsedto.ProjectSummaryResponse{
		ID:            summary.ID,
		Name:          summary.Name,
		Slug:          summary.Slug,
		NamespaceSlug: summary.NamespaceSlug,
		ClusterID:     summary.ClusterID,
		RuntimeMode:   summary.RuntimeMode,
		DefaultBranch: summary.DefaultBranch,
		CreatedAt:     summary.CreatedAt,
		UpdatedAt:     summary.UpdatedAt,
	}
}

func ToProjectListResponse(items []service.ProjectSummary) responsedto.ProjectListResponse {
	responseItems := make([]responsedto.ProjectSummaryResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, ToProjectSummaryResponse(item))
	}

	return responsedto.ProjectListResponse{Items: responseItems}
}

func ToProjectRepoLinkResponse(record service.ProjectRepoLinkRecord) responsedto.ProjectRepoLinkResponse {
	return responsedto.ProjectRepoLinkResponse{
		ID:                   record.ID,
		ProjectID:            record.ProjectID,
		GitHubInstallationID: record.GitHubInstallationID,
		GitHubRepoID:         record.GitHubRepoID,
		RepoOwner:            record.RepoOwner,
		RepoName:             record.RepoName,
		RepoFullName:         record.RepoFullName,
		TrackedBranch:        record.TrackedBranch,
		PreviewEnabled:       record.PreviewEnabled,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
	}
}

func ToProjectInternalServiceResponse(record service.ProjectInternalServiceRecord) responsedto.ProjectInternalServiceResponse {
	return responsedto.ProjectInternalServiceResponse{
		ID:            record.ID,
		ProjectID:     record.ProjectID,
		Kind:          record.Kind,
		Alias:         record.Alias,
		Protocol:      record.Protocol,
		Port:          record.Port,
		LocalEndpoint: record.LocalEndpoint,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}

func ToProjectInternalServiceListResponse(result service.ProjectInternalServiceListResult) responsedto.ProjectInternalServiceListResponse {
	items := make([]responsedto.ProjectInternalServiceResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, ToProjectInternalServiceResponse(item))
	}

	return responsedto.ProjectInternalServiceListResponse{Items: items}
}

func ToProjectServiceListResponse(result service.ProjectServiceListResult) responsedto.ProjectServiceListResponse {
	items := make([]responsedto.ProjectServiceResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, responsedto.ProjectServiceResponse{
			ID:             item.ID,
			ProjectID:      item.ProjectID,
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
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
		})
	}

	return responsedto.ProjectServiceListResponse{Items: items}
}

func ToProjectEnvBundleResponse(record service.ProjectEnvBundleRecord) responsedto.ProjectEnvBundleResponse {
	helperSnippets := make([]responsedto.ProjectEnvHelperSnippetResponse, 0, len(record.HelperSnippets))
	for _, item := range record.HelperSnippets {
		helperSnippets = append(helperSnippets, responsedto.ProjectEnvHelperSnippetResponse{
			ServiceKind: item.ServiceKind,
			Alias:       item.Alias,
			Env:         item.Env,
		})
	}

	return responsedto.ProjectEnvBundleResponse{
		Configured:     record.Configured,
		UpdatedAt:      record.UpdatedAt,
		Fingerprint:    record.Fingerprint,
		Keys:           record.Keys,
		ParseWarnings:  record.ParseWarnings,
		HelperSnippets: helperSnippets,
	}
}
