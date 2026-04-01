package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToAgentEnrollmentCommand(req requestdto.EnrollAgentRequest) service.AgentEnrollmentCommand {
	return service.AgentEnrollmentCommand{
		BootstrapToken: req.BootstrapToken,
		RuntimeMode:    req.RuntimeMode,
		AgentKind:      req.AgentKind,
		Machine: service.AgentMachineInfo{
			Hostname: req.Machine.Hostname,
			OS:       req.Machine.OS,
			Arch:     req.Machine.Arch,
			Kernel:   req.Machine.Kernel,
			IPs:      req.Machine.IPs,
			Labels:   req.Machine.Labels,
		},
		Capabilities: req.Capabilities,
	}
}

func ToAgentEnrollmentResponse(result service.AgentEnrollmentResult) responsedto.AgentEnrollmentResponse {
	return responsedto.AgentEnrollmentResponse{
		AgentID:    result.AgentID,
		AgentToken: result.AgentToken,
		InstanceID: result.InstanceID,
		IssuedAt:   result.IssuedAt,
		ExpiresAt:  result.ExpiresAt,
	}
}

func ToAgentHeartbeatCommand(req requestdto.AgentHeartbeatRequest) service.AgentHeartbeatCommand {
	return service.AgentHeartbeatCommand{
		AgentID:          req.AgentID,
		SessionID:        req.SessionID,
		State:            req.State,
		HealthStatus:     req.HealthStatus,
		HealthSummary:    req.HealthSummary,
		RuntimeMode:      req.RuntimeMode,
		AgentKind:        req.AgentKind,
		SentAt:           req.SentAt,
		UptimeSeconds:    req.UptimeSeconds,
		CapabilityHash:   req.CapabilityHash,
		CapabilityUpdate: req.CapabilityUpdate,
		Capabilities:     req.Capabilities,
	}
}

func ToAgentHeartbeatResponse(result service.AgentHeartbeatResult) responsedto.AgentHeartbeatResponse {
	return responsedto.AgentHeartbeatResponse{
		AgentID:        result.AgentID,
		InstanceID:     result.InstanceID,
		AgentStatus:    result.AgentStatus,
		InstanceStatus: result.InstanceStatus,
		ReceivedAt:     result.ReceivedAt,
	}
}
