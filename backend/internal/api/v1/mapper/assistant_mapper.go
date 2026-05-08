package mapper

import (
	requestdto "lazyops-server/internal/api/v1/dto/request"
	responsedto "lazyops-server/internal/api/v1/dto/response"
	"lazyops-server/internal/service"
)

func ToCreateAssistantSessionCommand(userID string, req requestdto.CreateAssistantSessionRequest) service.CreateAssistantSessionCommand {
	return service.CreateAssistantSessionCommand{
		UserID:    userID,
		ProjectID: req.ProjectID,
		Title:     req.Title,
	}
}

func ToAssistantMessageCommand(userID, role, sessionID string, req requestdto.PostAssistantMessageRequest) service.AssistantMessageCommand {
	return service.AssistantMessageCommand{
		UserID:    userID,
		Role:      role,
		SessionID: sessionID,
		ProjectID: req.ProjectID,
		Content:   req.Content,
	}
}

func ToAssistantSessionResponse(record service.AssistantSessionRecord) responsedto.AssistantSessionResponse {
	return responsedto.AssistantSessionResponse(record)
}

func ToAssistantConversationResponse(record service.AssistantConversationRecord) responsedto.AssistantConversationResponse {
	messages := make([]responsedto.AssistantMessageResponse, 0, len(record.Messages))
	for _, item := range record.Messages {
		messages = append(messages, responsedto.AssistantMessageResponse(item))
	}
	var plan *responsedto.AssistantActionPlanResponse
	if record.PendingPlan != nil {
		missing := make([]responsedto.AssistantMissingInputResponse, 0, len(record.PendingPlan.MissingInputs))
		for _, item := range record.PendingPlan.MissingInputs {
			missing = append(missing, responsedto.AssistantMissingInputResponse(item))
		}
		plan = &responsedto.AssistantActionPlanResponse{
			ID:                   record.PendingPlan.ID,
			ActionType:           record.PendingPlan.ActionType,
			Status:               record.PendingPlan.Status,
			Summary:              record.PendingPlan.Summary,
			RiskLevel:            record.PendingPlan.RiskLevel,
			RequiresConfirmation: record.PendingPlan.RequiresConfirmation,
			MissingInputs:        missing,
			Plan:                 record.PendingPlan.Plan,
			ExpiresAt:            record.PendingPlan.ExpiresAt,
			CreatedAt:            record.PendingPlan.CreatedAt,
			UpdatedAt:            record.PendingPlan.UpdatedAt,
		}
	}
	var execution *responsedto.AssistantExecutionResponse
	if record.Execution != nil {
		value := responsedto.AssistantExecutionResponse(*record.Execution)
		execution = &value
	}
	return responsedto.AssistantConversationResponse{
		Session:         ToAssistantSessionResponse(record.Session),
		Messages:        messages,
		PendingPlan:     plan,
		UIState:         record.UIState,
		ExecutionResult: execution,
	}
}
