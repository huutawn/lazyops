package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateInstanceCommand(userID string, req requestdto.CreateInstanceRequest) service.CreateInstanceCommand {
	return service.CreateInstanceCommand{
		UserID:    userID,
		Name:      req.Name,
		PublicIP:  req.PublicIP,
		PrivateIP: req.PrivateIP,
		Labels:    req.Labels,
	}
}

func ToInstallInstanceAgentSSHCommand(userID, instanceID string, req requestdto.InstallInstanceAgentSSHRequest) service.InstallInstanceAgentSSHCommand {
	return service.InstallInstanceAgentSSHCommand{
		UserID:             userID,
		ProjectID:          req.ProjectID,
		InstanceID:         instanceID,
		Host:               req.Host,
		Port:               req.Port,
		Username:           req.Username,
		Password:           req.Password,
		PrivateKey:         req.PrivateKey,
		HostKeyFingerprint: req.HostKeyFingerprint,
		ControlPlaneURL:    req.ControlPlaneURL,
		RuntimeMode:        req.RuntimeMode,
		AgentKind:          req.AgentKind,
		AgentImage:         req.AgentImage,
		ContainerName:      req.ContainerName,
	}
}

func ToCreateInstanceResponse(result service.CreateInstanceResult) responsedto.CreateInstanceResponse {
	return responsedto.CreateInstanceResponse{
		Instance:  ToInstanceSummaryResponse(result.Instance),
		Bootstrap: ToBootstrapTokenIssueResponse(result.Bootstrap),
	}
}

func ToInstallInstanceAgentSSHResponse(result service.InstallInstanceAgentSSHResult) responsedto.InstallInstanceAgentSSHResponse {
	return responsedto.InstallInstanceAgentSSHResponse{
		InstanceID:         result.InstanceID,
		Bootstrap:          ToBootstrapTokenIssueResponse(result.Bootstrap),
		StartedAt:          result.StartedAt,
		HostKeyFingerprint: result.HostKeyFingerprint,
		AttachedProjectID:  result.AttachedProjectID,
	}
}

func ToInstanceSummaryResponse(summary service.InstanceSummary) responsedto.InstanceSummaryResponse {
	labels := make(map[string]string, len(summary.Labels))
	for key, value := range summary.Labels {
		labels[key] = value
	}

	return responsedto.InstanceSummaryResponse{
		ID:                  summary.ID,
		TargetKind:          summary.TargetKind,
		Name:                summary.Name,
		PublicIP:            summary.PublicIP,
		PrivateIP:           summary.PrivateIP,
		AgentID:             summary.AgentID,
		Status:              summary.Status,
		Labels:              labels,
		RuntimeCapabilities: summary.RuntimeCapabilities,
		CreatedAt:           summary.CreatedAt,
		UpdatedAt:           summary.UpdatedAt,
	}
}

func ToBootstrapTokenIssueResponse(issue service.BootstrapTokenIssue) responsedto.BootstrapTokenIssueResponse {
	return responsedto.BootstrapTokenIssueResponse{
		Token:     issue.Token,
		TokenID:   issue.TokenID,
		ExpiresAt: issue.ExpiresAt,
		SingleUse: issue.SingleUse,
	}
}

func ToInstanceListResponse(result service.InstanceListResult) responsedto.InstanceListResponse {
	items := make([]responsedto.InstanceSummaryResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, ToInstanceSummaryResponse(item))
	}

	return responsedto.InstanceListResponse{Items: items}
}
