package mapper

import (
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToValidateLazyopsYAMLResponse(result service.ValidateLazyopsYAMLResult) responsedto.ValidateLazyopsYAMLResponse {
	suggested := make([]responsedto.RoutingGuidanceRouteResponse, 0, len(result.SuggestedRoutes))
	for _, route := range result.SuggestedRoutes {
		suggested = append(suggested, responsedto.RoutingGuidanceRouteResponse{
			Path:      route.Path,
			Service:   route.Service,
			Audience:  route.Audience,
			Source:    route.Source,
			WebSocket: route.WebSocket,
		})
	}
	effective := make([]responsedto.RoutingGuidanceRouteResponse, 0, len(result.EffectivePublicPaths))
	for _, route := range result.EffectivePublicPaths {
		effective = append(effective, responsedto.RoutingGuidanceRouteResponse{
			Path:      route.Path,
			Service:   route.Service,
			Audience:  route.Audience,
			Source:    route.Source,
			WebSocket: route.WebSocket,
		})
	}
	findings := make([]responsedto.MigrationFindingResponse, 0, len(result.MigrationFindings))
	for _, finding := range result.MigrationFindings {
		findings = append(findings, responsedto.MigrationFindingResponse{
			Category:         finding.Category,
			Severity:         finding.Severity,
			ServiceName:      finding.ServiceName,
			CurrentValue:     finding.CurrentValue,
			RecommendedValue: finding.RecommendedValue,
			Message:          finding.Message,
		})
	}
	return responsedto.ValidateLazyopsYAMLResponse{
		Project:           ToProjectSummaryResponse(result.Project),
		DeploymentBinding: ToDeploymentBindingResponse(result.DeploymentBinding),
		TargetSummary: responsedto.InitTargetSummaryResponse{
			ID:          result.TargetSummary.ID,
			Name:        result.TargetSummary.Name,
			Kind:        result.TargetSummary.Kind,
			Status:      result.TargetSummary.Status,
			RuntimeMode: result.TargetSummary.RuntimeMode,
		},
		Schema: responsedto.LazyopsYAMLSchemaResponse{
			AllowedDependencyProtocols:  result.Schema.AllowedDependencyProtocols,
			AllowedMagicDomainProviders: result.Schema.AllowedMagicDomainProviders,
			ForbiddenFieldNames:         result.Schema.ForbiddenFieldNames,
		},
		SuggestedRoutes:      suggested,
		EffectivePublicPaths: effective,
		MigrationFindings:    findings,
	}
}
