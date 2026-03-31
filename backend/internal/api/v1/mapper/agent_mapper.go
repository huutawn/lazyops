package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateAgentCommand(req requestdto.CreateAgentRequest) service.CreateAgentCommand {
	return service.CreateAgentCommand{
		AgentID: req.AgentID,
		Name:    req.Name,
		Status:  req.Status,
	}
}

func ToUpdateAgentStatusCommand(agentID string, req requestdto.UpdateAgentStatusRequest) service.UpdateAgentStatusCommand {
	return service.UpdateAgentStatusCommand{
		AgentID: agentID,
		Name:    req.Name,
		Status:  req.Status,
		Source:  req.Source,
	}
}

func ToAgentStatusWSCommand(req requestdto.AgentStatusWSMessage, source string) service.UpdateAgentStatusCommand {
	return service.UpdateAgentStatusCommand{
		AgentID: req.AgentID,
		Name:    req.Name,
		Status:  req.Status,
		Source:  source,
	}
}

func ToAgentResponse(agent service.AgentRecord) responsedto.AgentResponse {
	return responsedto.AgentResponse{
		ID:         agent.ID,
		AgentID:    agent.AgentID,
		Name:       agent.Name,
		Status:     agent.Status,
		LastSeenAt: agent.LastSeenAt,
		UpdatedAt:  agent.UpdatedAt,
	}
}

func ToAgentListResponse(agents []service.AgentRecord) responsedto.AgentListResponse {
	items := make([]responsedto.AgentResponse, 0, len(agents))
	for _, agent := range agents {
		items = append(items, ToAgentResponse(agent))
	}

	return responsedto.AgentListResponse{Items: items}
}

func ToAgentRealtimeEventResponse(event service.AgentRealtimeEvent) responsedto.AgentRealtimeEventResponse {
	return responsedto.AgentRealtimeEventResponse{
		Type:    event.Type,
		Payload: ToAgentResponse(event.Payload),
		Meta: responsedto.RealtimeMetaResponse{
			Source: event.Meta.Source,
			At:     event.Meta.At,
		},
	}
}
