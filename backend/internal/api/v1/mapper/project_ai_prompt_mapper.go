package mapper

import (
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToProjectAIPromptResponse(record service.ProjectAIPromptRecord) responsedto.ProjectAIPromptResponse {
	services := make([]responsedto.ProjectAIPromptServiceSnapshotResponse, 0, len(record.ServiceSnapshot))
	for _, item := range record.ServiceSnapshot {
		services = append(services, responsedto.ProjectAIPromptServiceSnapshotResponse{
			Name:           item.Name,
			Kind:           item.Kind,
			Role:           item.Role,
			RuntimeProfile: item.RuntimeProfile,
			SourceType:     item.SourceType,
			Public:         item.Public,
			Managed:        item.Managed,
			WebSocket:      item.WebSocket,
			PublicPath:     item.PublicPath,
			InternalURL:    item.InternalURL,
		})
	}

	effectivePaths := make([]responsedto.RoutingGuidanceRouteResponse, 0, len(record.EffectivePublicPaths))
	for _, item := range record.EffectivePublicPaths {
		effectivePaths = append(effectivePaths, responsedto.RoutingGuidanceRouteResponse{
			Path:      item.Path,
			Service:   item.Service,
			Audience:  item.Audience,
			Source:    item.Source,
			WebSocket: item.WebSocket,
		})
	}

	findings := make([]responsedto.MigrationFindingResponse, 0, len(record.MigrationFindings))
	for _, item := range record.MigrationFindings {
		findings = append(findings, responsedto.MigrationFindingResponse{
			Category:         item.Category,
			Severity:         item.Severity,
			ServiceName:      item.ServiceName,
			CurrentValue:     item.CurrentValue,
			RecommendedValue: item.RecommendedValue,
			Message:          item.Message,
		})
	}

	sourceSections := make([]responsedto.ProjectAIPromptSourceSectionResponse, 0, len(record.SourceSections))
	for _, item := range record.SourceSections {
		sourceSections = append(sourceSections, responsedto.ProjectAIPromptSourceSectionResponse{
			Key:         item.Key,
			Title:       item.Title,
			Description: item.Description,
			ItemCount:   item.ItemCount,
		})
	}

	return responsedto.ProjectAIPromptResponse{
		Title:                record.Title,
		Summary:              record.Summary,
		Prompt:               record.Prompt,
		ServiceSnapshot:      services,
		EffectivePublicPaths: effectivePaths,
		ManagedKeys:          append([]string{}, record.ManagedKeys...),
		MigrationFindings:    findings,
		SourceSections:       sourceSections,
	}
}
