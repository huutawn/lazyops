package service

import "time"

const (
	AssistantSessionStatusActive = "active"

	AssistantMessageRoleUser      = "user"
	AssistantMessageRoleAssistant = "assistant"

	AssistantMessageKindChat                = "chat"
	AssistantMessageKindPlan                = "plan"
	AssistantMessageKindConfirmationRequest = "confirmation_request"
	AssistantMessageKindExecutionResult     = "execution_result"

	AssistantActionTypeDeployRef = "deploy_ref"

	AssistantIntentDeployRef        = "deploy_ref"
	AssistantIntentDeploymentStatus = "deployment_status"
	AssistantIntentQueryLogs        = "query_logs"
	AssistantIntentExplainIncident  = "explain_incident"
	AssistantIntentQueryTopology    = "query_topology"
	AssistantIntentReviewSystem     = "review_system"
	AssistantIntentQueryMetrics     = "query_metrics"
	AssistantIntentActivityTable    = "activity_table"
	AssistantIntentUnknown          = "unknown"

	AssistantPlanStatusDraft                = "draft"
	AssistantPlanStatusAwaitingConfirmation = "awaiting_confirmation"
	AssistantPlanStatusApproved             = "approved"
	AssistantPlanStatusExecuting            = "executing"
	AssistantPlanStatusCompleted            = "completed"
	AssistantPlanStatusFailed               = "failed"
	AssistantPlanStatusCancelled            = "cancelled"

	AssistantUIStateChat                 = "chat"
	AssistantUIStatePlanning             = "planning"
	AssistantUIStateAwaitingConfirmation = "awaiting_confirmation"
	AssistantUIStateExecuting            = "executing"
	AssistantUIStateCompleted            = "completed"
	AssistantUIStateFailed               = "failed"
)

type AssistantPlannerInput struct {
	UserID    string
	Role      string
	ProjectID string
	Content   string
	History   []AssistantMessageRecord
}

type AssistantPlannedIntent struct {
	Intent            string   `json:"intent"`
	Confidence        float64  `json:"confidence"`
	ProjectID         string   `json:"project_id,omitempty"`
	SourceRef         string   `json:"source_ref,omitempty"`
	RepoFullName      string   `json:"repo_full_name,omitempty"`
	TargetEnvironment string   `json:"target_environment,omitempty"`
	BindingHint       string   `json:"binding_hint,omitempty"`
	ServiceName       string   `json:"service_name,omitempty"`
	Window            string   `json:"window,omitempty"`
	Limit             int      `json:"limit,omitempty"`
	RequiresMutation  bool     `json:"requires_mutation"`
	MissingInputs     []string `json:"missing_inputs,omitempty"`
	Reason            string   `json:"reason,omitempty"`
}

type AssistantSessionRecord struct {
	ID            string    `json:"id"`
	ProjectID     *string   `json:"project_id,omitempty"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	LastMessageAt time.Time `json:"last_message_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AssistantMessageRecord struct {
	ID          string         `json:"id"`
	SessionID   string         `json:"session_id"`
	Role        string         `json:"role"`
	Kind        string         `json:"kind"`
	Content     string         `json:"content"`
	ContentData map[string]any `json:"content_data,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type AssistantMissingInput struct {
	Field   string `json:"field"`
	Prompt  string `json:"prompt"`
	Example string `json:"example,omitempty"`
}

type AssistantPlanRecord struct {
	ID                   string                  `json:"id"`
	ActionType           string                  `json:"action_type"`
	Status               string                  `json:"status"`
	Summary              string                  `json:"summary"`
	RiskLevel            string                  `json:"risk_level"`
	RequiresConfirmation bool                    `json:"requires_confirmation"`
	MissingInputs        []AssistantMissingInput `json:"missing_inputs,omitempty"`
	Plan                 map[string]any          `json:"plan"`
	ExpiresAt            *time.Time              `json:"expires_at,omitempty"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

type AssistantExecutionRecord struct {
	Status        string `json:"status"`
	DeploymentID  string `json:"deployment_id,omitempty"`
	RevisionID    string `json:"revision_id,omitempty"`
	BuildJobID    string `json:"build_job_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type AssistantConversationRecord struct {
	Session     AssistantSessionRecord    `json:"session"`
	Messages    []AssistantMessageRecord  `json:"messages"`
	PendingPlan *AssistantPlanRecord      `json:"pending_plan,omitempty"`
	UIState     string                    `json:"ui_state"`
	Execution   *AssistantExecutionRecord `json:"execution_result,omitempty"`
}

type CreateAssistantSessionCommand struct {
	UserID    string
	ProjectID string
	Title     string
}

type AssistantMessageCommand struct {
	UserID    string
	Role      string
	SessionID string
	ProjectID string
	Content   string
}

type ConfirmAssistantPlanCommand struct {
	UserID string
	Role   string
	PlanID string
}

type CancelAssistantPlanCommand struct {
	UserID string
	Role   string
	PlanID string
}

type DeployIntentPlan struct {
	ProjectID            string   `json:"project_id"`
	RepoFullName         string   `json:"repo_full_name,omitempty"`
	SourceRef            string   `json:"source_ref"`
	TargetEnvironment    string   `json:"target_environment"`
	BindingHint          string   `json:"binding_hint,omitempty"`
	DeploymentBindingID  string   `json:"deployment_binding_id,omitempty"`
	InternalServices     []string `json:"internal_services,omitempty"`
	ExecutionStrategy    string   `json:"execution_strategy"`
	MissingInputs        []string `json:"missing_inputs,omitempty"`
	Summary              string   `json:"summary"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	RiskLevel            string   `json:"risk_level"`
}
