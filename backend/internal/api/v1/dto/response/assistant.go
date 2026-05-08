package response

import "time"

type AssistantSessionResponse struct {
	ID            string     `json:"id"`
	ProjectID     *string    `json:"project_id,omitempty"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	LastMessageAt time.Time  `json:"last_message_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type AssistantMessageResponse struct {
	ID          string         `json:"id"`
	SessionID   string         `json:"session_id"`
	Role        string         `json:"role"`
	Kind        string         `json:"kind"`
	Content     string         `json:"content"`
	ContentData map[string]any `json:"content_data,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type AssistantMissingInputResponse struct {
	Field   string `json:"field"`
	Prompt  string `json:"prompt"`
	Example string `json:"example,omitempty"`
}

type AssistantActionPlanResponse struct {
	ID                   string                           `json:"id"`
	ActionType           string                           `json:"action_type"`
	Status               string                           `json:"status"`
	Summary              string                           `json:"summary"`
	RiskLevel            string                           `json:"risk_level"`
	RequiresConfirmation bool                             `json:"requires_confirmation"`
	MissingInputs        []AssistantMissingInputResponse  `json:"missing_inputs,omitempty"`
	Plan                 map[string]any                   `json:"plan"`
	ExpiresAt            *time.Time                       `json:"expires_at,omitempty"`
	CreatedAt            time.Time                        `json:"created_at"`
	UpdatedAt            time.Time                        `json:"updated_at"`
}

type AssistantExecutionResponse struct {
	Status        string `json:"status"`
	DeploymentID  string `json:"deployment_id,omitempty"`
	RevisionID    string `json:"revision_id,omitempty"`
	BuildJobID    string `json:"build_job_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type AssistantConversationResponse struct {
	Session         AssistantSessionResponse       `json:"session"`
	Messages        []AssistantMessageResponse     `json:"messages"`
	PendingPlan     *AssistantActionPlanResponse   `json:"pending_plan,omitempty"`
	UIState         string                         `json:"ui_state"`
	ExecutionResult *AssistantExecutionResponse    `json:"execution_result,omitempty"`
}
