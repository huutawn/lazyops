package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateProjectCommand(userID string, req requestdto.CreateProjectRequest) service.CreateProjectCommand {
	services := make([]service.ConfigureProjectServiceItem, 0, len(req.Services))
	for _, item := range req.Services {
		services = append(services, service.ConfigureProjectServiceItem{
			Name:                    item.Name,
			Path:                    item.Path,
			Kind:                    item.Kind,
			SourceType:              item.SourceType,
			Public:                  item.Public,
			RuntimeProfile:          item.RuntimeProfile,
			PlacementMode:           item.PlacementMode,
			PlacementNodeID:         item.PlacementNodeID,
			Dependencies:            toProjectServiceDependencyBindings(item.Dependencies),
			ConnectionTemplateKey:   item.ConnectionTemplateKey,
			ConnectionTemplate:      item.ConnectionTemplate,
			ConnectionTargetService: item.ConnectionTargetService,
			ManagedByLazyops:        item.ManagedByLazyops,
			StartHint:               item.StartHint,
			ImageRef:                item.ImageRef,
			ImageDigest:             item.ImageDigest,
			TargetPort:              item.TargetPort,
			ServicePort:             item.ServicePort,
			Replicas:                item.Replicas,
			EnvBundle:               item.EnvBundle,
			PVCSpec:                 item.PVCSpec,
			DeployStrategy:          item.DeployStrategy,
			Healthcheck:             item.Healthcheck,
		})
	}

	return service.CreateProjectCommand{
		UserID:           userID,
		Name:             req.Name,
		Slug:             req.Slug,
		NamespaceSlug:    req.NamespaceSlug,
		ClusterID:        req.ClusterID,
		RuntimeMode:      req.RuntimeMode,
		DefaultBranch:    req.DefaultBranch,
		Services:         services,
		InternalServices: req.InternalServices,
	}
}

func ToProjectServiceActionResponse(result service.ProjectServiceActionResult) responsedto.ProjectServiceActionResponse {
	return responsedto.ProjectServiceActionResponse{
		Action:       result.Action,
		ServiceID:    result.ServiceID,
		ServiceName:  result.ServiceName,
		Status:       result.Status,
		TriggerKind:  result.TriggerKind,
		DeploymentID: result.DeploymentID,
		RevisionID:   result.RevisionID,
		Message:      result.Message,
	}
}

func ToConfigureProjectServicesCommand(userID, role, projectID string, req requestdto.ConfigureProjectServicesRequest) service.ConfigureProjectServicesCommand {
	items := make([]service.ConfigureProjectServiceItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, service.ConfigureProjectServiceItem{
			Name:                    item.Name,
			Path:                    item.Path,
			Kind:                    item.Kind,
			SourceType:              item.SourceType,
			Public:                  item.Public,
			RuntimeProfile:          item.RuntimeProfile,
			PlacementMode:           item.PlacementMode,
			PlacementNodeID:         item.PlacementNodeID,
			Dependencies:            toProjectServiceDependencyBindings(item.Dependencies),
			ConnectionTemplateKey:   item.ConnectionTemplateKey,
			ConnectionTemplate:      item.ConnectionTemplate,
			ConnectionTargetService: item.ConnectionTargetService,
			ManagedByLazyops:        item.ManagedByLazyops,
			StartHint:               item.StartHint,
			ImageRef:                item.ImageRef,
			ImageDigest:             item.ImageDigest,
			TargetPort:              item.TargetPort,
			ServicePort:             item.ServicePort,
			Replicas:                item.Replicas,
			EnvBundle:               item.EnvBundle,
			PVCSpec:                 item.PVCSpec,
			DeployStrategy:          item.DeployStrategy,
			Healthcheck:             item.Healthcheck,
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
	items := make([]responsedto.ProjectInventoryServiceResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, responsedto.ProjectInventoryServiceResponse{
			ID:                      item.ID,
			ProjectID:               item.ProjectID,
			Name:                    item.Name,
			Path:                    item.Path,
			Kind:                    item.Kind,
			SourceType:              item.SourceType,
			Public:                  item.Public,
			RuntimeProfile:          item.RuntimeProfile,
			PlacementMode:           item.PlacementMode,
			PlacementNodeID:         item.PlacementNodeID,
			Dependencies:            toProjectServiceDependencyBindingResponses(item.Dependencies),
			ConnectionTemplateKey:   item.ConnectionTemplateKey,
			ConnectionTemplate:      item.ConnectionTemplate,
			ConnectionTargetService: item.ConnectionTargetService,
			ManagedByLazyops:        item.ManagedByLazyops,
			StartHint:               item.StartHint,
			ImageRef:                item.ImageRef,
			ImageDigest:             item.ImageDigest,
			DetectedPorts:           toBuildDetectedPortResponses(item.DetectedPorts),
			TargetPort:              item.TargetPort,
			ServicePort:             item.ServicePort,
			Replicas:                item.Replicas,
			EnvBundle:               item.EnvBundle,
			PVCSpec:                 item.PVCSpec,
			DeployStrategy:          item.DeployStrategy,
			Healthcheck:             item.Healthcheck,
			CreatedAt:               item.CreatedAt,
			UpdatedAt:               item.UpdatedAt,
		})
	}

	return responsedto.ProjectServiceListResponse{Items: items}
}

func toProjectServiceDependencyBindings(items []requestdto.ProjectServiceDependencyBindingRequest) []service.ProjectServiceDependencyBinding {
	if len(items) == 0 {
		return nil
	}
	out := make([]service.ProjectServiceDependencyBinding, 0, len(items))
	for _, item := range items {
		out = append(out, service.ProjectServiceDependencyBinding{
			TargetService:         item.TargetService,
			ConnectionTemplateKey: item.ConnectionTemplateKey,
			ConnectionTemplate:    item.ConnectionTemplate,
		})
	}
	return out
}

func toProjectServiceDependencyBindingResponses(items []service.ProjectServiceDependencyBinding) []responsedto.ProjectServiceDependencyBindingResponse {
	if len(items) == 0 {
		return nil
	}
	out := make([]responsedto.ProjectServiceDependencyBindingResponse, 0, len(items))
	for _, item := range items {
		out = append(out, responsedto.ProjectServiceDependencyBindingResponse{
			TargetService:         item.TargetService,
			ConnectionTemplateKey: item.ConnectionTemplateKey,
			ConnectionTemplate:    item.ConnectionTemplate,
		})
	}
	return out
}

func ToPlacementNodeListResponse(result service.PlacementNodeListResult) responsedto.PlacementNodeListResponse {
	items := make([]responsedto.ClusterNodeResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, responsedto.ClusterNodeResponse{
			ClusterID:   item.ClusterID,
			InstanceID:  item.InstanceID,
			Name:        item.Name,
			Status:      item.Status,
			K8sNodeName: item.K8sNodeName,
			Labels:      item.Labels,
			LastSeenAt:  item.LastSeenAt,
			IsReady:     item.IsReady,
		})
	}
	return responsedto.PlacementNodeListResponse{
		ClusterID: result.ClusterID,
		Items:     items,
	}
}

func ToProjectRuntimeSummaryResponse(result service.ProjectRuntimeSummaryResult) responsedto.ProjectRuntimeSummaryResponse {
	nodes := make([]responsedto.ProjectRuntimeNodeResponse, 0, len(result.Nodes))
	for _, item := range result.Nodes {
		nodes = append(nodes, responsedto.ProjectRuntimeNodeResponse{
			ClusterID:   item.ClusterID,
			InstanceID:  item.InstanceID,
			Name:        item.Name,
			Status:      item.Status,
			K8sNodeName: item.K8sNodeName,
			Labels:      item.Labels,
			LastSeenAt:  item.LastSeenAt,
			IsReady:     item.IsReady,
		})
	}

	services := make([]responsedto.ProjectRuntimeServiceResponse, 0, len(result.Services))
	for _, item := range result.Services {
		dependencies := make([]responsedto.ProjectRuntimeDependencyResponse, 0, len(item.Dependencies))
		for _, dep := range item.Dependencies {
			dependencies = append(dependencies, responsedto.ProjectRuntimeDependencyResponse{
				ServiceID:        dep.ServiceID,
				ServiceName:      dep.ServiceName,
				Status:           dep.Status,
				StatusReason:     dep.StatusReason,
				InternalEndpoint: dep.InternalEndpoint,
			})
		}

		recentLogs := make([]responsedto.ProjectRuntimeLogPreviewResponse, 0, len(item.RecentLogs))
		for _, log := range item.RecentLogs {
			recentLogs = append(recentLogs, responsedto.ProjectRuntimeLogPreviewResponse{
				ID:            log.ID,
				Source:        log.Source,
				Level:         log.Level,
				Message:       log.Message,
				Timestamp:     log.Timestamp,
				Node:          log.Node,
				CorrelationID: log.CorrelationID,
			})
		}

		services = append(services, responsedto.ProjectRuntimeServiceResponse{
			ServiceID:         item.ServiceID,
			Name:              item.Name,
			Kind:              item.Kind,
			SourceType:        item.SourceType,
			Public:            item.Public,
			RuntimeProfile:    item.RuntimeProfile,
			RuntimeStatus:     item.RuntimeStatus,
			RuntimeReason:     item.RuntimeReason,
			BuildState:        item.BuildState,
			RolloutState:      item.RolloutState,
			PlacementMode:     item.PlacementMode,
			RequestedNodeID:   item.RequestedNodeID,
			EffectiveNodeIDs:  append([]string{}, item.EffectiveNodeIDs...),
			ImageRef:          item.ImageRef,
			ImageDigest:       item.ImageDigest,
			RevisionID:        item.RevisionID,
			Revision:          item.Revision,
			DeploymentID:      item.DeploymentID,
			PublicURLs:        append([]string{}, item.PublicURLs...),
			InternalEndpoints: append([]string{}, item.InternalEndpoints...),
			Dependencies:      dependencies,
			RecentLogs:        recentLogs,
		})
	}

	return responsedto.ProjectRuntimeSummaryResponse{
		ProjectID:        result.ProjectID,
		RuntimeMode:      result.RuntimeMode,
		ClusterID:        result.ClusterID,
		Namespace:        result.Namespace,
		LiveRevisionID:   result.LiveRevisionID,
		LiveRevision:     result.LiveRevision,
		StableRevisionID: result.StableRevisionID,
		StableRevision:   result.StableRevision,
		SyncState:        result.SyncState,
		SyncReason:       result.SyncReason,
		PublicURLs:       append([]string{}, result.PublicURLs...),
		PublicURLStatus:  result.PublicURLStatus,
		PublicURLReason:  result.PublicURLReason,
		Nodes:            nodes,
		Services:         services,
	}
}

func ToProjectEnvBundleResponse(record service.ProjectEnvBundleRecord) responsedto.ProjectEnvBundleResponse {
	helperPacks := make([]responsedto.ProjectEnvHelperPackResponse, 0, len(record.HelperPacks))
	for _, item := range record.HelperPacks {
		snippets := make([]responsedto.ProjectEnvHelperSnippetResponse, 0, len(item.LanguageSnippets))
		for _, snippet := range item.LanguageSnippets {
			snippets = append(snippets, responsedto.ProjectEnvHelperSnippetResponse{
				Language:  snippet.Language,
				Framework: snippet.Framework,
				Kind:      snippet.Kind,
				Title:     snippet.Title,
				Content:   snippet.Content,
			})
		}
		helperPacks = append(helperPacks, responsedto.ProjectEnvHelperPackResponse{
			ServiceKind:      item.ServiceKind,
			Alias:            item.Alias,
			Category:         item.Category,
			Audience:         item.Audience,
			SourceService:    item.SourceService,
			RelatedServices:  append([]string{}, item.RelatedServices...),
			PrimaryKey:       item.PrimaryKey,
			PublicPath:       item.PublicPath,
			Managed:          item.Managed,
			RuntimeInjected:  item.RuntimeInjected,
			PlaceholderEnv:   item.PlaceholderEnv,
			EnvExample:       item.EnvExample,
			LocalExampleEnv:  item.LocalExampleEnv,
			RuntimeKeys:      append([]string{}, item.RuntimeKeys...),
			ProvisionedKeys:  append([]string{}, item.ProvisionedKeys...),
			Notes:            append([]string{}, item.Notes...),
			LanguageSnippets: snippets,
		})
	}

	return responsedto.ProjectEnvBundleResponse{
		Configured:      record.Configured,
		UpdatedAt:       record.UpdatedAt,
		Fingerprint:     record.Fingerprint,
		Keys:            append([]string{}, record.Keys...),
		UserKeys:        append([]string{}, record.UserKeys...),
		ManagedKeys:     append([]string{}, record.ManagedKeys...),
		ProvisionedKeys: append([]string{}, record.ProvisionedKeys...),
		ParseWarnings:   append([]string{}, record.ParseWarnings...),
		HelperPacks:     helperPacks,
	}
}
