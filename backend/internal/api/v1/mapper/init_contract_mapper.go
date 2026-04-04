package mapper

import (
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToValidateLazyopsYAMLResponse(result service.ValidateLazyopsYAMLResult) responsedto.ValidateLazyopsYAMLResponse {
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
	}
}
