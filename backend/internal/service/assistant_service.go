package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

type AssistantSessionStore interface {
	Create(item *models.AssistantSession) error
	GetByIDForUser(sessionID, userID string) (*models.AssistantSession, error)
	ListByUser(userID string) ([]models.AssistantSession, error)
	Update(item *models.AssistantSession) error
	Touch(sessionID string, at time.Time) error
}

type AssistantMessageStore interface {
	Create(item *models.AssistantMessage) error
	ListBySession(sessionID string) ([]models.AssistantMessage, error)
}

type AssistantActionPlanStore interface {
	Create(item *models.AssistantActionPlan) error
	GetByID(planID string) (*models.AssistantActionPlan, error)
	GetLatestPendingBySession(sessionID string) (*models.AssistantActionPlan, error)
	Update(item *models.AssistantActionPlan) error
}

type AssistantAuditEventStore interface {
	Create(item *models.AssistantAuditEvent) error
}

type AssistantService struct {
	sessions       AssistantSessionStore
	messages       AssistantMessageStore
	plans          AssistantActionPlanStore
	audit          AssistantAuditEventStore
	projects       ProjectStore
	repoLinks      ProjectRepoLinkStore
	envBundles     ProjectEnvBundleStore
	internalSvcs   ProjectInternalServiceStore
	deploySvc      *DeploymentService
	runtimeSvc     *ProjectRuntimeService
	observability  *ObservabilityService
	errorKnowledge *ErrorKnowledgeService
	deployments    *BootstrapOrchestrator
	bindingRepo    DeploymentBindingStore
	planner        AssistantIntentPlanner
	explanations   *IncidentExplanationService
}

func NewAssistantService(
	sessions AssistantSessionStore,
	messages AssistantMessageStore,
	plans AssistantActionPlanStore,
	audit AssistantAuditEventStore,
	projects ProjectStore,
	repoLinks ProjectRepoLinkStore,
	envBundles ProjectEnvBundleStore,
	internalSvcs ProjectInternalServiceStore,
	deploySvc *DeploymentService,
	runtimeSvc *ProjectRuntimeService,
	observability *ObservabilityService,
	errorKnowledge *ErrorKnowledgeService,
	deployments *BootstrapOrchestrator,
	bindingRepo DeploymentBindingStore,
) *AssistantService {
	return &AssistantService{
		sessions:       sessions,
		messages:       messages,
		plans:          plans,
		audit:          audit,
		projects:       projects,
		repoLinks:      repoLinks,
		envBundles:     envBundles,
		internalSvcs:   internalSvcs,
		deploySvc:      deploySvc,
		runtimeSvc:     runtimeSvc,
		observability:  observability,
		errorKnowledge: errorKnowledge,
		deployments:    deployments,
		bindingRepo:    bindingRepo,
		planner:        NewHeuristicAssistantIntentPlanner(),
	}
}

func (s *AssistantService) WithPlanner(planner AssistantIntentPlanner) *AssistantService {
	if s == nil {
		return s
	}
	if planner != nil {
		s.planner = planner
	}
	return s
}

func (s *AssistantService) WithIncidentExplanationService(explanations *IncidentExplanationService) *AssistantService {
	if s == nil {
		return s
	}
	s.explanations = explanations
	return s
}

func (s *AssistantService) CreateSession(cmd CreateAssistantSessionCommand) (*AssistantSessionRecord, error) {
	if s == nil || s.sessions == nil {
		return nil, ErrInvalidInput
	}
	userID := strings.TrimSpace(cmd.UserID)
	if userID == "" {
		return nil, ErrInvalidInput
	}
	var projectIDPtr *string
	if projectID := strings.TrimSpace(cmd.ProjectID); projectID != "" {
		projectIDPtr = &projectID
	}
	now := time.Now().UTC()
	item := &models.AssistantSession{
		ID:            utils.NewPrefixedID("asst_sess"),
		UserID:        userID,
		ProjectID:     projectIDPtr,
		Title:         strings.TrimSpace(cmd.Title),
		Status:        AssistantSessionStatusActive,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if item.Title == "" {
		item.Title = "New assistant session"
	}
	if err := s.sessions.Create(item); err != nil {
		return nil, err
	}
	record := toAssistantSessionRecord(*item)
	return &record, nil
}

func (s *AssistantService) ListSessions(userID string) ([]AssistantSessionRecord, error) {
	if s == nil || s.sessions == nil || strings.TrimSpace(userID) == "" {
		return nil, ErrInvalidInput
	}
	items, err := s.sessions.ListByUser(strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	out := make([]AssistantSessionRecord, 0, len(items))
	for _, item := range items {
		out = append(out, toAssistantSessionRecord(item))
	}
	return out, nil
}

func (s *AssistantService) GetSession(userID, sessionID string) (*AssistantConversationRecord, error) {
	if s == nil || s.sessions == nil || s.messages == nil || s.plans == nil {
		return nil, ErrInvalidInput
	}
	session, err := s.sessions.GetByIDForUser(strings.TrimSpace(sessionID), strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrProjectNotFound
	}
	return s.buildConversation(session)
}

func (s *AssistantService) PostMessage(cmd AssistantMessageCommand) (*AssistantConversationRecord, error) {
	if s == nil || s.sessions == nil || s.messages == nil || s.plans == nil {
		return nil, ErrInvalidInput
	}
	content := strings.TrimSpace(cmd.Content)
	if strings.TrimSpace(cmd.UserID) == "" || strings.TrimSpace(cmd.SessionID) == "" || content == "" {
		return nil, ErrInvalidInput
	}
	session, err := s.sessions.GetByIDForUser(strings.TrimSpace(cmd.SessionID), strings.TrimSpace(cmd.UserID))
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrProjectNotFound
	}
	now := time.Now().UTC()
	if err := s.messages.Create(&models.AssistantMessage{
		ID:          utils.NewPrefixedID("asst_msg"),
		SessionID:   session.ID,
		Role:        AssistantMessageRoleUser,
		Kind:        AssistantMessageKindChat,
		Content:     content,
		ContentJSON: "{}",
		CreatedAt:   now,
	}); err != nil {
		return nil, err
	}
	_ = s.sessions.Touch(session.ID, now)
	_ = s.auditEvent(session.ID, nil, "message.received", map[string]any{"content": content})

	responseMessage, messageKind, uiState, contentData, plan, execution, err := s.handleMessage(session, strings.TrimSpace(cmd.Role), content)
	if err != nil {
		return nil, err
	}
	contentEnvelope := map[string]any{}
	for key, value := range contentData {
		contentEnvelope[key] = value
	}
	if plan != nil {
		contentEnvelope["plan"] = plan
	}
	contentJSON := "{}"
	if len(contentEnvelope) > 0 {
		if raw, marshalErr := json.Marshal(contentEnvelope); marshalErr == nil {
			contentJSON = string(raw)
		}
	}
	if err := s.messages.Create(&models.AssistantMessage{
		ID:          utils.NewPrefixedID("asst_msg"),
		SessionID:   session.ID,
		Role:        AssistantMessageRoleAssistant,
		Kind:        messageKind,
		Content:     responseMessage,
		ContentJSON: contentJSON,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	conversation, err := s.buildConversation(session)
	if err != nil {
		return nil, err
	}
	conversation.UIState = uiState
	conversation.Execution = execution
	if plan != nil {
		conversation.PendingPlan = plan
	}
	return conversation, nil
}

func (s *AssistantService) ConfirmPlan(cmd ConfirmAssistantPlanCommand) (*AssistantConversationRecord, error) {
	if s == nil || s.plans == nil {
		return nil, ErrInvalidInput
	}
	plan, err := s.plans.GetByID(strings.TrimSpace(cmd.PlanID))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, ErrProjectNotFound
	}
	session, err := s.sessions.GetByIDForUser(plan.SessionID, strings.TrimSpace(cmd.UserID))
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrProjectAccessDenied
	}
	if cmd.Role != RoleAdmin && cmd.Role != RoleOperator {
		return nil, ErrProjectAccessDenied
	}
	if plan.Status != AssistantPlanStatusAwaitingConfirmation {
		return nil, ErrInvalidInput
	}
	if plan.ExpiresAt != nil && time.Now().UTC().After(*plan.ExpiresAt) {
		plan.Status = AssistantPlanStatusFailed
		_ = s.plans.Update(plan)
		return nil, ErrInvalidInput
	}
	now := time.Now().UTC()
	plan.Status = AssistantPlanStatusApproved
	plan.ConfirmedBy = &cmd.UserID
	plan.ConfirmedAt = &now
	plan.ConfirmationMethod = "click_confirm"
	if err := s.plans.Update(plan); err != nil {
		return nil, err
	}
	_, execution, execErr := s.executePlan(session, plan)
	if execErr != nil {
		return nil, execErr
	}
	if err := s.messages.Create(&models.AssistantMessage{
		ID:          utils.NewPrefixedID("asst_msg"),
		SessionID:   session.ID,
		Role:        AssistantMessageRoleAssistant,
		Kind:        AssistantMessageKindExecutionResult,
		Content:     buildExecutionMessage(execution),
		ContentJSON: "{}",
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	conversation, err := s.buildConversation(session)
	if err != nil {
		return nil, err
	}
	conversation.Execution = execution
	conversation.UIState = executionState(execution)
	conversation.PendingPlan = nil
	return conversation, nil
}

func (s *AssistantService) CancelPlan(cmd CancelAssistantPlanCommand) (*AssistantConversationRecord, error) {
	plan, err := s.plans.GetByID(strings.TrimSpace(cmd.PlanID))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, ErrProjectNotFound
	}
	session, err := s.sessions.GetByIDForUser(plan.SessionID, strings.TrimSpace(cmd.UserID))
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrProjectAccessDenied
	}
	plan.Status = AssistantPlanStatusCancelled
	if err := s.plans.Update(plan); err != nil {
		return nil, err
	}
	if err := s.messages.Create(&models.AssistantMessage{
		ID:          utils.NewPrefixedID("asst_msg"),
		SessionID:   session.ID,
		Role:        AssistantMessageRoleAssistant,
		Kind:        AssistantMessageKindChat,
		Content:     "Deployment plan cancelled.",
		ContentJSON: "{}",
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	conversation, err := s.buildConversation(session)
	if err != nil {
		return nil, err
	}
	conversation.PendingPlan = nil
	conversation.UIState = AssistantUIStateChat
	return conversation, nil
}

func (s *AssistantService) handleMessage(session *models.AssistantSession, role, content string) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	projectID := ""
	if session.ProjectID != nil {
		projectID = strings.TrimSpace(*session.ProjectID)
	}
	if projectID == "" {
		return "Tell me which project to work on first, then I can review runtime state or prepare a deploy plan.", AssistantMessageKindChat, AssistantUIStatePlanning, nil, nil, nil, nil
	}
	planner := s.planner
	if planner == nil {
		planner = NewHeuristicAssistantIntentPlanner()
	}
	history, _ := s.messages.ListBySession(session.ID)
	historyRecords := make([]AssistantMessageRecord, 0, len(history))
	for _, item := range history {
		historyRecords = append(historyRecords, toAssistantMessageRecord(item))
	}
	planned, err := planner.Plan(context.Background(), AssistantPlannerInput{UserID: session.UserID, Role: role, ProjectID: projectID, Content: content, History: historyRecords})
	if err != nil {
		return "I could not safely classify that request. Try asking for deploy status, metrics, activity, logs, topology, or a deploy plan.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
	planned = guardAssistantIntent(projectID, content, planned)
	return s.dispatchIntent(session, role, projectID, content, planned)
}

func (s *AssistantService) dispatchIntent(session *models.AssistantSession, role, projectID, content string, planned *AssistantPlannedIntent) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	switch planned.Intent {
	case AssistantIntentDeployRef:
		return s.handleDeployIntent(session, role, projectID, content, planned)
	case AssistantIntentDeploymentStatus:
		return s.handleDeploymentStatus(session, role, projectID)
	case AssistantIntentReviewSystem:
		return s.handleSystemEvaluation(session, role, projectID, planned)
	case AssistantIntentQueryLogs:
		return s.handleQueryLogs(projectID, planned.ServiceName)
	case AssistantIntentExplainIncident:
		return s.handleExplainIncident(projectID)
	case AssistantIntentQueryTopology:
		return s.handleTopology(session, projectID)
	case AssistantIntentQueryMetrics:
		return s.handleMetricsDashboard(projectID, planned)
	case AssistantIntentActivityTable:
		return s.handleActivityTable(session, role, projectID, planned)
	default:
		return "I can help with deployments, metrics, activity, runtime evaluation, logs, incidents, and topology for this project.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
}

func (s *AssistantService) handleDeployIntent(session *models.AssistantSession, role, projectID, content string, planned *AssistantPlannedIntent) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	plan, followUp := buildDeployIntentPlan(projectID, content)
	if planned != nil {
		if planned.SourceRef != "" {
			plan.SourceRef = planned.SourceRef
		}
		if planned.RepoFullName != "" {
			plan.RepoFullName = planned.RepoFullName
		}
		if planned.TargetEnvironment != "" {
			plan.TargetEnvironment = planned.TargetEnvironment
		}
		if planned.BindingHint != "" {
			plan.BindingHint = planned.BindingHint
		}
		plan.MissingInputs = nil
		if plan.TargetEnvironment == "" {
			plan.MissingInputs = appendMissingInput(plan.MissingInputs, "target_environment")
		}
		if plan.SourceRef == "" {
			plan.MissingInputs = appendMissingInput(plan.MissingInputs, "source_ref")
		}
		if plan.TargetEnvironment == "production" {
			plan.RequiresConfirmation = true
			plan.RiskLevel = "high"
		}
	}
	plan, validationPrompt, validationMissing, err := s.validateDeployIntentPlan(session, role, plan)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	if validationPrompt != "" {
		followUp = validationPrompt
	}
	for _, field := range validationMissing {
		plan.MissingInputs = appendMissingInput(plan.MissingInputs, field)
	}
	if len(plan.MissingInputs) > 0 {
		missing := make([]AssistantMissingInput, 0, len(plan.MissingInputs))
		for _, field := range plan.MissingInputs {
			missing = append(missing, AssistantMissingInput{Field: field, Prompt: followUp})
		}
		record, err := s.createPlan(session.ID, plan, AssistantPlanStatusDraft, missing)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		return followUp, AssistantMessageKindPlan, AssistantUIStatePlanning, nil, record, nil, nil
	}
	status := AssistantPlanStatusApproved
	messageKind := AssistantMessageKindExecutionResult
	uiState := AssistantUIStateExecuting
	if plan.RequiresConfirmation {
		status = AssistantPlanStatusAwaitingConfirmation
		messageKind = AssistantMessageKindConfirmationRequest
		uiState = AssistantUIStateAwaitingConfirmation
	}
	record, err := s.createPlan(session.ID, plan, status, nil)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	if plan.RequiresConfirmation {
		return "I prepared a production deploy plan. Review the target and confirm when ready.", messageKind, uiState, nil, record, nil, nil
	}
	updatedPlan, execution, err := s.executePlan(session, nil)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	if updatedPlan != nil {
		record = updatedPlan
	}
	return buildExecutionMessage(execution), messageKind, executionState(execution), nil, record, execution, nil
}

func (s *AssistantService) handleDeploymentStatus(session *models.AssistantSession, role, projectID string) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	if s.deploySvc == nil {
		return "Deployment status is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
	items, err := s.deploySvc.List(session.UserID, role, projectID)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	if len(items) == 0 {
		return "This project does not have any deployments yet.", AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "deployment_status", "deployments": []any{}}, nil, nil, nil
	}
	latest := items[0]
	message := fmt.Sprintf("Latest deployment %s is %s with build state %s.", latest.ID, firstRuntimeNonEmpty(latest.RolloutState, "unknown"), firstRuntimeNonEmpty(latest.BuildState, "unknown"))
	return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "deployment_status", "deployment": latest, "project_id": projectID}, nil, nil, nil
}

func (s *AssistantService) handleQueryLogs(projectID, serviceName string) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	if s.observability == nil {
		return "Logs are not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
	logs, err := s.observability.ListRecentLogs(context.Background(), projectID, serviceName, "", "", "", 10)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	if len(logs) == 0 {
		return "I did not find recent logs matching that query.", AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "logs", "logs": []any{}}, nil, nil, nil
	}
	message := fmt.Sprintf("I found %d recent log lines%s. Latest: [%s] %s", len(logs), assistantServiceSuffix(serviceName), logs[0].Level, logs[0].Message)
	contentData := map[string]any{"card_type": "logs", "logs": logs, "project_id": projectID}
	if s.errorKnowledge != nil {
		similar, simErr := s.errorKnowledge.FindSimilar(projectID, serviceName, logs[0].Message, 3)
		if simErr == nil && len(similar) > 0 {
			message += fmt.Sprintf(" I also found %d similar historical error knowledge documents.", len(similar))
			contentData["historical_matches"] = toErrorKnowledgePreviewList(similar)
		}
	}
	return message, AssistantMessageKindChat, AssistantUIStateChat, contentData, nil, nil, nil
}

func (s *AssistantService) handleExplainIncident(projectID string) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	if s.explanations != nil {
		explanation, err := s.explanations.Explain(context.Background(), ExplainIncidentCommand{ProjectID: projectID})
		if err == nil && explanation != nil {
			message := firstRuntimeNonEmpty(explanation.Summary, explanation.LikelyCause)
			return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "incident_explanation_engine", "project_id": projectID, "explanation": explanation}, nil, nil, nil
		}
	}
	if s.observability == nil {
		return "Incident explanation is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
	incidents, err := s.observability.ListIncidentsByProject(context.Background(), projectID)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	if len(incidents) == 0 {
		return "There are no recorded incidents for this project right now.", AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "incident_explanation", "incidents": []any{}}, nil, nil, nil
	}
	latest := incidents[0]
	message := fmt.Sprintf("Latest incident %s is %s with severity %s. Summary: %s", latest.ID, latest.Kind, latest.Severity, latest.Summary)
	contentData := map[string]any{"card_type": "incident_explanation", "incident": latest, "project_id": projectID}
	if s.errorKnowledge != nil {
		similar, simErr := s.errorKnowledge.FindSimilar(projectID, "", latest.Summary, 3)
		if simErr == nil && len(similar) > 0 {
			message += fmt.Sprintf(" I found %d similar historical error knowledge documents that may help explain this incident.", len(similar))
			contentData["historical_matches"] = toErrorKnowledgePreviewList(similar)
		}
	}
	return message, AssistantMessageKindChat, AssistantUIStateChat, contentData, nil, nil, nil
}

func (s *AssistantService) handleTopology(session *models.AssistantSession, projectID string) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	if s.observability == nil {
		return "Topology data is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
	graph, err := s.observability.BuildTopologyGraphForUser(context.Background(), projectID, session.UserID)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	message := fmt.Sprintf("Topology currently has %d nodes and %d edges.", len(graph.Nodes), len(graph.Edges))
	return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "topology", "topology": graph, "project_id": projectID}, nil, nil, nil
}

func (s *AssistantService) handleReadOnlyMessage(session *models.AssistantSession, role, projectID, content string) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	query := strings.ToLower(strings.TrimSpace(content))
	switch {
	case strings.Contains(query, "deployment") || strings.Contains(query, "deploy status") || strings.Contains(query, "rollout"):
		if s.deploySvc == nil {
			return "Deployment status is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
		}
		items, err := s.deploySvc.List(session.UserID, role, projectID)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		if len(items) == 0 {
			return "This project does not have any deployments yet.", AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "deployment_status", "deployments": []any{}}, nil, nil, nil
		}
		latest := items[0]
		message := fmt.Sprintf("Latest deployment %s is %s with build state %s.", latest.ID, firstRuntimeNonEmpty(latest.RolloutState, "unknown"), firstRuntimeNonEmpty(latest.BuildState, "unknown"))
		return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "deployment_status", "deployment": latest, "project_id": projectID}, nil, nil, nil
	case strings.Contains(query, "review") || strings.Contains(query, "system") || strings.Contains(query, "runtime"):
		if s.runtimeSvc == nil || s.observability == nil {
			return "System review is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
		}
		runtime, err := s.runtimeSvc.Get(context.Background(), session.UserID, role, projectID)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		incidents, err := s.observability.ListIncidentsByProject(context.Background(), projectID)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		degraded := 0
		for _, item := range runtime.Services {
			if strings.EqualFold(strings.TrimSpace(item.RuntimeStatus), "degraded") || strings.EqualFold(strings.TrimSpace(item.RuntimeStatus), "pending") {
				degraded++
			}
		}
		message := fmt.Sprintf("Runtime review: %d services, %d nodes, %d degraded or pending services, and %d open incidents.", len(runtime.Services), len(runtime.Nodes), degraded, len(incidents))
		return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "runtime_review", "runtime": runtime, "incident_count": len(incidents), "degraded_count": degraded, "project_id": projectID}, nil, nil, nil
	case strings.Contains(query, "log"):
		if s.observability == nil {
			return "Logs are not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
		}
		serviceName := extractAssistantServiceName(query)
		logs, err := s.observability.ListRecentLogs(context.Background(), projectID, serviceName, "", "", "", 10)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		if len(logs) == 0 {
			return "I did not find recent logs matching that query.", AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "logs", "logs": []any{}}, nil, nil, nil
		}
		message := fmt.Sprintf("I found %d recent log lines%s. Latest: [%s] %s", len(logs), assistantServiceSuffix(serviceName), logs[0].Level, logs[0].Message)
		contentData := map[string]any{}
		contentData["card_type"] = "logs"
		contentData["logs"] = logs
		contentData["project_id"] = projectID
		if s.errorKnowledge != nil {
			similar, simErr := s.errorKnowledge.FindSimilar(projectID, serviceName, logs[0].Message, 3)
			if simErr == nil && len(similar) > 0 {
				message += fmt.Sprintf(" I also found %d similar historical error knowledge documents.", len(similar))
				contentData["historical_matches"] = toErrorKnowledgePreviewList(similar)
			}
		}
		return message, AssistantMessageKindChat, AssistantUIStateChat, contentData, nil, nil, nil
	case strings.Contains(query, "incident") || strings.Contains(query, "error"):
		if s.observability == nil {
			return "Incident explanation is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
		}
		incidents, err := s.observability.ListIncidentsByProject(context.Background(), projectID)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		if len(incidents) == 0 {
			return "There are no recorded incidents for this project right now.", AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "incident_explanation", "incidents": []any{}}, nil, nil, nil
		}
		latest := incidents[0]
		message := fmt.Sprintf("Latest incident %s is %s with severity %s. Summary: %s", latest.ID, latest.Kind, latest.Severity, latest.Summary)
		contentData := map[string]any{}
		contentData["card_type"] = "incident_explanation"
		contentData["incident"] = latest
		contentData["project_id"] = projectID
		if s.errorKnowledge != nil {
			similar, simErr := s.errorKnowledge.FindSimilar(projectID, "", latest.Summary, 3)
			if simErr == nil && len(similar) > 0 {
				message += fmt.Sprintf(" I found %d similar historical error knowledge documents that may help explain this incident.", len(similar))
				contentData["historical_matches"] = toErrorKnowledgePreviewList(similar)
			}
		}
		return message, AssistantMessageKindChat, AssistantUIStateChat, contentData, nil, nil, nil
	case strings.Contains(query, "topology"):
		if s.observability == nil {
			return "Topology data is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
		}
		graph, err := s.observability.BuildTopologyGraphForUser(context.Background(), projectID, session.UserID)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		message := fmt.Sprintf("Topology currently has %d nodes and %d edges.", len(graph.Nodes), len(graph.Edges))
		return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "topology", "topology": graph, "project_id": projectID}, nil, nil, nil
	default:
		return "I can help with deployment status, runtime review, logs, incidents, and topology. Ask me about one of those for this project.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
}

func isDeployPrompt(content string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(content)), "deploy")
}

func extractAssistantServiceName(content string) string {
	servicePattern := regexp.MustCompile(`(?i)service\s+([a-zA-Z0-9._-]+)`)
	if matches := servicePattern.FindStringSubmatch(strings.TrimSpace(content)); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func assistantServiceSuffix(serviceName string) string {
	if strings.TrimSpace(serviceName) == "" {
		return ""
	}
	return " for service " + serviceName
}

func toErrorKnowledgePreviewList(items []models.ErrorKnowledgeDocument) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":             item.ID,
			"project_id":     item.ProjectID,
			"service_name":   item.ServiceName,
			"severity":       item.Severity,
			"title":          item.Title,
			"body":           item.Body,
			"correlation_id": item.CorrelationID,
			"revision_id":    item.RevisionID,
			"last_seen_at":   item.LastSeenAt,
		})
	}
	return out
}

func (s *AssistantService) createPlan(sessionID string, plan DeployIntentPlan, status string, missing []AssistantMissingInput) (*AssistantPlanRecord, error) {
	now := time.Now().UTC()
	planMap := map[string]any{
		"project_id":            plan.ProjectID,
		"repo_full_name":        plan.RepoFullName,
		"source_ref":            plan.SourceRef,
		"target_environment":    plan.TargetEnvironment,
		"binding_hint":          plan.BindingHint,
		"deployment_binding_id": plan.DeploymentBindingID,
		"internal_services":     plan.InternalServices,
		"execution_strategy":    plan.ExecutionStrategy,
		"summary":               plan.Summary,
		"requires_confirmation": plan.RequiresConfirmation,
		"risk_level":            plan.RiskLevel,
	}
	raw, err := json.Marshal(planMap)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(raw)
	var expiresAt *time.Time
	if plan.RequiresConfirmation {
		exp := now.Add(15 * time.Minute)
		expiresAt = &exp
	}
	item := &models.AssistantActionPlan{
		ID:                   utils.NewPrefixedID("asst_plan"),
		SessionID:            sessionID,
		ProjectID:            plan.ProjectID,
		ActionType:           AssistantActionTypeDeployRef,
		Status:               status,
		PlanJSON:             string(raw),
		PlanHash:             hex.EncodeToString(hash[:]),
		RiskLevel:            firstRuntimeNonEmpty(plan.RiskLevel, "low"),
		RequiresConfirmation: plan.RequiresConfirmation,
		ExpiresAt:            expiresAt,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.plans.Create(item); err != nil {
		return nil, err
	}
	record := toAssistantPlanRecord(*item, missing)
	record.Summary = plan.Summary
	record.Plan = planMap
	_ = s.auditEvent(sessionID, &item.ID, "plan.created", map[string]any{"status": status, "plan": planMap})
	return &record, nil
}

func (s *AssistantService) executePlan(session *models.AssistantSession, explicitPlan *models.AssistantActionPlan) (*AssistantPlanRecord, *AssistantExecutionRecord, error) {
	var plan *models.AssistantActionPlan
	var err error
	if explicitPlan != nil {
		plan = explicitPlan
	} else {
		plan, err = s.plans.GetLatestPendingBySession(session.ID)
		if err != nil {
			return nil, nil, err
		}
	}
	if plan == nil {
		return nil, nil, ErrInvalidInput
	}
	plan.Status = AssistantPlanStatusExecuting
	plan.UpdatedAt = time.Now().UTC()
	if err := s.plans.Update(plan); err != nil {
		return nil, nil, err
	}
	var payload DeployIntentPlan
	if err := json.Unmarshal([]byte(plan.PlanJSON), &payload); err != nil {
		return nil, nil, err
	}
	if validatedPayload, prompt, missing, err := s.validateDeployIntentPlan(session, RoleOperator, payload); err != nil {
		return nil, nil, err
	} else if prompt != "" || len(missing) > 0 {
		execution := &AssistantExecutionRecord{Status: "failed", Reason: firstRuntimeNonEmpty(prompt, "deploy plan is missing required inputs")}
		plan.Status = AssistantPlanStatusFailed
		plan.UpdatedAt = time.Now().UTC()
		_ = s.plans.Update(plan)
		return toPlanRecordFromJSON(plan), execution, nil
	} else {
		payload = validatedPayload
	}
	result, execErr := s.deployments.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: session.UserID,
		RequesterRole:   RoleOperator,
		ProjectID:       payload.ProjectID,
		SourceRef:       payload.SourceRef,
		TriggerKind:     "assistant_deploy",
	})
	execution := &AssistantExecutionRecord{}
	if execErr != nil {
		plan.Status = AssistantPlanStatusFailed
		plan.UpdatedAt = time.Now().UTC()
		_ = s.plans.Update(plan)
		execution.Status = "failed"
		execution.Reason = execErr.Error()
		_ = s.auditEvent(session.ID, &plan.ID, "plan.execution_failed", map[string]any{"error": execErr.Error()})
		return toPlanRecordFromJSON(plan), execution, nil
	}
	plan.Status = AssistantPlanStatusCompleted
	plan.UpdatedAt = time.Now().UTC()
	if err := s.plans.Update(plan); err != nil {
		return nil, nil, err
	}
	execution.Status = "completed"
	execution.DeploymentID = result.DeploymentID
	execution.RevisionID = result.RevisionID
	execution.BuildJobID = result.BuildJobID
	execution.CorrelationID = result.CorrelationID
	execution.AgentID = result.AgentID
	execution.Reason = result.RolloutReason
	_ = s.auditEvent(session.ID, &plan.ID, "plan.executed", map[string]any{"deployment_id": result.DeploymentID, "build_job_id": result.BuildJobID})
	return toPlanRecordFromJSON(plan), execution, nil
}

func (s *AssistantService) buildConversation(session *models.AssistantSession) (*AssistantConversationRecord, error) {
	messageModels, err := s.messages.ListBySession(session.ID)
	if err != nil {
		return nil, err
	}
	messages := make([]AssistantMessageRecord, 0, len(messageModels))
	for _, item := range messageModels {
		messages = append(messages, toAssistantMessageRecord(item))
	}
	planModel, err := s.plans.GetLatestPendingBySession(session.ID)
	if err != nil {
		return nil, err
	}
	var pendingPlan *AssistantPlanRecord
	uiState := AssistantUIStateChat
	if planModel != nil {
		pendingPlan = toPlanRecordFromJSON(planModel)
		if pendingPlan != nil {
			switch pendingPlan.Status {
			case AssistantPlanStatusDraft:
				uiState = AssistantUIStatePlanning
			case AssistantPlanStatusAwaitingConfirmation:
				uiState = AssistantUIStateAwaitingConfirmation
			case AssistantPlanStatusExecuting:
				uiState = AssistantUIStateExecuting
			case AssistantPlanStatusCompleted:
				uiState = AssistantUIStateCompleted
			case AssistantPlanStatusFailed:
				uiState = AssistantUIStateFailed
			}
		}
	}
	return &AssistantConversationRecord{
		Session:     toAssistantSessionRecord(*session),
		Messages:    messages,
		PendingPlan: pendingPlan,
		UIState:     uiState,
	}, nil
}

func (s *AssistantService) auditEvent(sessionID string, planID *string, eventType string, payload map[string]any) error {
	if s == nil || s.audit == nil {
		return nil
	}
	raw, _ := json.Marshal(payload)
	return s.audit.Create(&models.AssistantAuditEvent{
		ID:           utils.NewPrefixedID("asst_evt"),
		SessionID:    sessionID,
		ActionPlanID: planID,
		EventType:    eventType,
		PayloadJSON:  string(raw),
		CreatedAt:    time.Now().UTC(),
	})
}

func buildDeployIntentPlan(projectID, content string) (DeployIntentPlan, string) {
	normalized := strings.ToLower(strings.TrimSpace(content))
	plan := DeployIntentPlan{
		ProjectID:         projectID,
		ExecutionStrategy: "one_click_deploy",
		RiskLevel:         "medium",
	}
	if !strings.Contains(normalized, "deploy") {
		return plan, "I can review runtime, logs, topology, or deploy a ref. Try asking me to deploy a branch, tag, commit, or PR."
	}
	if strings.Contains(normalized, "production") || strings.Contains(normalized, "prod") {
		plan.TargetEnvironment = "production"
		plan.RequiresConfirmation = true
		plan.RiskLevel = "high"
	} else if strings.Contains(normalized, "staging") {
		plan.TargetEnvironment = "staging"
	} else if strings.Contains(normalized, "preview") {
		plan.TargetEnvironment = "preview"
	} else {
		plan.MissingInputs = append(plan.MissingInputs, "target_environment")
	}
	plan.SourceRef = extractSourceRef(content)
	plan.RepoFullName = extractRepoFullName(content)
	plan.BindingHint = extractBindingHint(content)
	if plan.SourceRef == "" {
		plan.MissingInputs = append(plan.MissingInputs, "source_ref")
	}
	if plan.TargetEnvironment == "production" {
		plan.Summary = fmt.Sprintf("Deploy %s to production using the current project binding.", firstRuntimeNonEmpty(plan.SourceRef, "the requested ref"))
	} else {
		plan.Summary = fmt.Sprintf("Deploy %s to %s using the current project binding.", firstRuntimeNonEmpty(plan.SourceRef, "the requested ref"), firstRuntimeNonEmpty(plan.TargetEnvironment, "the selected environment"))
	}
	if len(plan.MissingInputs) > 0 {
		return plan, buildMissingInputPrompt(plan.MissingInputs)
	}
	return plan, ""
}

func extractSourceRef(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	githubPRURLPattern := regexp.MustCompile(`(?i)https?://github\.com/[^/]+/[^/]+/pull/(\d+)`)
	if matches := githubPRURLPattern.FindStringSubmatch(trimmed); len(matches) == 2 && strings.TrimSpace(matches[1]) != "" {
		return "refs/pull/" + strings.TrimSpace(matches[1]) + "/head"
	}
	prRefPattern := regexp.MustCompile(`(?i)\bpr\s*#?(\d+)\b|#(\d+)\b`)
	if matches := prRefPattern.FindStringSubmatch(trimmed); len(matches) > 0 {
		number := firstRuntimeNonEmpty(matches[1], matches[2])
		if number != "" {
			return "refs/pull/" + number + "/head"
		}
	}
	fields := strings.Fields(trimmed)
	for i, field := range fields {
		lower := strings.ToLower(strings.Trim(field, ",.?!"))
		if lower == "pr" || lower == "branch" || lower == "tag" || lower == "commit" || lower == "ref" {
			if i+1 < len(fields) {
				raw := strings.Trim(fields[i+1], ",.?!")
				switch lower {
				case "branch":
					return "refs/heads/" + strings.TrimPrefix(strings.TrimPrefix(raw, "refs/heads/"), "/")
				case "tag":
					return "refs/tags/" + strings.TrimPrefix(strings.TrimPrefix(raw, "refs/tags/"), "/")
				default:
					return raw
				}
			}
		}
		if strings.HasPrefix(lower, "refs/heads/") || strings.HasPrefix(lower, "refs/tags/") {
			return strings.Trim(field, ",.?!")
		}
	}
	return ""
}

func (s *AssistantService) validateDeployIntentPlan(session *models.AssistantSession, role string, plan DeployIntentPlan) (DeployIntentPlan, string, []string, error) {
	missing := make([]string, 0)
	project, err := resolveProjectForAccess(s.projects, session.UserID, role, plan.ProjectID)
	if err != nil {
		return plan, "", missing, err
	}
	if strings.TrimSpace(plan.ProjectID) == "" {
		missing = appendMissingInput(missing, "project_id")
		return plan, "Tell me which project to use for this deploy.", missing, nil
	}
	if project == nil {
		return plan, "", missing, ErrProjectNotFound
	}
	if s.bindingRepo == nil {
		missing = appendMissingInput(missing, "deployment_binding")
		return plan, "I cannot validate the deploy target yet because deployment binding data is unavailable.", missing, nil
	}
	bindings, err := s.bindingRepo.ListByProject(plan.ProjectID)
	if err != nil {
		return plan, "", missing, err
	}
	if len(bindings) == 0 {
		missing = appendMissingInput(missing, "deployment_binding")
		return plan, "I cannot deploy this project yet because it does not have a deployment binding. Connect a target first.", missing, nil
	}
	if strings.TrimSpace(plan.BindingHint) != "" {
		bindingID := resolveBindingByHint(bindings, plan.BindingHint)
		if bindingID == "" {
			missing = appendMissingInput(missing, "deployment_binding")
			return plan, fmt.Sprintf("I could not find a deployment binding matching %q in this project. Tell me the exact binding name or target_ref.", plan.BindingHint), missing, nil
		}
		plan.DeploymentBindingID = bindingID
	}
	if strings.TrimSpace(plan.DeploymentBindingID) == "" {
		bindingID, ambiguous := resolveBindingForEnvironment(bindings, plan.TargetEnvironment)
		if ambiguous {
			missing = appendMissingInput(missing, "deployment_binding")
			return plan, fmt.Sprintf("I found more than one %s deployment binding for this project. Tell me which binding or target_ref to use.", plan.TargetEnvironment), missing, nil
		}
		plan.DeploymentBindingID = bindingID
		if strings.TrimSpace(plan.DeploymentBindingID) == "" {
			plan.DeploymentBindingID = bindings[0].ID
		}
	}
	if s.repoLinks != nil {
		link, err := s.repoLinks.GetByProjectID(plan.ProjectID)
		if err != nil {
			return plan, "", missing, err
		}
		if strings.TrimSpace(plan.RepoFullName) != "" {
			if link == nil {
				missing = appendMissingInput(missing, "repo_link")
				return plan, "I need a linked GitHub repository before I can verify the repo you mentioned in this deploy request.", missing, nil
			}
			linkedRepo := strings.TrimSpace(link.RepoOwner + "/" + link.RepoName)
			if !strings.EqualFold(strings.TrimSpace(plan.RepoFullName), linkedRepo) {
				missing = appendMissingInput(missing, "repo_match")
				return plan, fmt.Sprintf("The prompt references repo %q, but this project is linked to %q. Confirm the right project or repo first.", plan.RepoFullName, linkedRepo), missing, nil
			}
		}
		if strings.HasPrefix(plan.SourceRef, "refs/pull/") && link == nil {
			missing = appendMissingInput(missing, "repo_link")
			return plan, "I need a linked GitHub repository before I can deploy a pull request for this project.", missing, nil
		}
	}
	if s.internalSvcs != nil && s.envBundles != nil {
		internalItems, err := s.internalSvcs.ListByProject(plan.ProjectID)
		if err != nil {
			return plan, "", missing, err
		}
		if len(internalItems) > 0 {
			bundle, err := s.envBundles.GetByProject(plan.ProjectID)
			if err != nil {
				return plan, "", missing, err
			}
			if bundle == nil || strings.TrimSpace(bundle.EnvEncrypted) == "" {
				missing = appendMissingInput(missing, "project_env")
				return plan, "This project uses internal services, so I need the project env bundle configured before I can approve the deploy plan.", missing, nil
			}
			requiredKeys, err := s.requiredEnvKeysForProject(plan.ProjectID)
			if err != nil {
				missing = appendMissingInput(missing, "project_env_keys")
				return plan, "I could not derive the required managed env keys for this project yet. Re-check the service inventory and env helpers first.", missing, nil
			}
			if len(requiredKeys) > 0 {
				record, err := s.loadEnvReadiness(plan.ProjectID)
				if err != nil {
					missing = appendMissingInput(missing, "project_env_keys")
					return plan, "I could not verify the project env keys for this deploy. Re-save the env bundle and try again.", missing, nil
				}
				missingKeys := diffRequiredEnvKeys(requiredKeys, record.ProvisionedKeys)
				if len(missingKeys) > 0 {
					missing = appendMissingInput(missing, "project_env_keys")
					return plan, fmt.Sprintf("I need the project env bundle to provide these managed keys before deploy: %s.", strings.Join(missingKeys, ", ")), missing, nil
				}
			}
		}
	}
	if strings.TrimSpace(plan.TargetEnvironment) == "preview" && strings.TrimSpace(plan.DeploymentBindingID) == "" {
		missing = appendMissingInput(missing, "deployment_binding")
		return plan, "I could not map preview to a deployment binding automatically. Tell me which target binding should be used.", missing, nil
	}
	if strings.TrimSpace(plan.SourceRef) != "" {
		plan.SourceRef = normalizeAssistantSourceRef(plan.SourceRef)
	}
	return plan, "", missing, nil
}

func resolveBindingForEnvironment(bindings []models.DeploymentBinding, environment string) (string, bool) {
	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" {
		return "", false
	}
	matches := make([]string, 0)
	for _, binding := range bindings {
		if strings.EqualFold(strings.TrimSpace(binding.TargetEnvironment), environment) {
			matches = append(matches, binding.ID)
		}
	}
	if len(matches) == 1 {
		return matches[0], false
	}
	if len(matches) > 1 {
		return "", true
	}
	for _, binding := range bindings {
		targetRef := strings.ToLower(strings.TrimSpace(binding.TargetRef))
		name := strings.ToLower(strings.TrimSpace(binding.Name))
		switch environment {
		case "production":
			if strings.Contains(targetRef, "prod") || strings.Contains(name, "prod") || strings.Contains(targetRef, "production") || strings.Contains(name, "production") {
				matches = append(matches, binding.ID)
			}
		case "staging":
			if strings.Contains(targetRef, "staging") || strings.Contains(name, "staging") || strings.Contains(targetRef, "stage") || strings.Contains(name, "stage") {
				matches = append(matches, binding.ID)
			}
		case "preview":
			if strings.Contains(targetRef, "preview") || strings.Contains(name, "preview") {
				matches = append(matches, binding.ID)
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], false
	}
	if len(matches) > 1 {
		return "", true
	}
	return "", false
}

func extractRepoFullName(content string) string {
	githubRepoURLPattern := regexp.MustCompile(`(?i)https?://github\.com/([^/]+)/([^/#?]+)`)
	if matches := githubRepoURLPattern.FindStringSubmatch(strings.TrimSpace(content)); len(matches) == 3 {
		return strings.TrimSpace(matches[1] + "/" + strings.TrimSuffix(matches[2], ".git"))
	}
	githubTreeURLPattern := regexp.MustCompile(`(?i)https?://github\.com/([^/]+)/([^/]+)/tree/[^\s?#]+`)
	if matches := githubTreeURLPattern.FindStringSubmatch(strings.TrimSpace(content)); len(matches) == 3 {
		return strings.TrimSpace(matches[1] + "/" + strings.TrimSuffix(matches[2], ".git"))
	}
	githubCommitURLPattern := regexp.MustCompile(`(?i)https?://github\.com/([^/]+)/([^/]+)/commit/([a-f0-9]{7,40})`)
	if matches := githubCommitURLPattern.FindStringSubmatch(strings.TrimSpace(content)); len(matches) == 4 {
		return strings.TrimSpace(matches[1] + "/" + strings.TrimSuffix(matches[2], ".git"))
	}
	for _, field := range strings.Fields(strings.TrimSpace(content)) {
		trimmed := strings.Trim(field, ",.?!()[]{}")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.Contains(trimmed, "://") {
			return trimmed
		}
	}
	return ""
}

func extractBindingHint(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	targetRefPattern := regexp.MustCompile(`(?i)\btarget[_ -]?ref\s+([a-zA-Z0-9._/-]+)`)
	if matches := targetRefPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	bindingPattern := regexp.MustCompile(`(?i)\bbinding\s+([a-zA-Z0-9._/-]+)`)
	if matches := bindingPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func resolveBindingByHint(bindings []models.DeploymentBinding, hint string) string {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return ""
	}
	for _, binding := range bindings {
		if strings.EqualFold(strings.TrimSpace(binding.ID), hint) || strings.EqualFold(strings.TrimSpace(binding.TargetRef), hint) || strings.EqualFold(strings.TrimSpace(binding.Name), hint) {
			return binding.ID
		}
	}
	return ""
}

func (s *AssistantService) loadEnvReadiness(projectID string) (*ProjectEnvBundleRecord, error) {
	if s == nil || s.projects == nil || s.envBundles == nil || s.internalSvcs == nil {
		return nil, ErrInvalidInput
	}
	envService := NewProjectEnvService(s.projects, s.envBundles, s.internalSvcs, "")
	return envService.getForProject(projectID)
}

func (s *AssistantService) requiredEnvKeysForProject(projectID string) ([]string, error) {
	record, err := s.loadEnvReadiness(projectID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	keys := make([]string, 0)
	for _, pack := range record.HelperPacks {
		if !pack.Managed {
			continue
		}
		for key := range pack.PlaceholderEnv {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		keys = append(keys, requiredEnvKeysFromInternalServicesModelFallback(s.internalSvcs, projectID)...)
	}
	return uniqueSortedStrings(keys), nil
}

func diffRequiredEnvKeys(required, provisioned []string) []string {
	provisionedSet := make(map[string]struct{}, len(provisioned))
	for _, key := range provisioned {
		provisionedSet[key] = struct{}{}
	}
	missing := make([]string, 0)
	for _, key := range required {
		if _, ok := provisionedSet[key]; !ok {
			missing = append(missing, key)
		}
	}
	return uniqueSortedStrings(missing)
}

func requiredEnvKeysFromInternalServices(items []models.ProjectInternalService) []string {
	keys := make([]string, 0)
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Kind)) {
		case "postgres", "mysql", "mongodb":
			keys = append(keys, "DATABASE_URL")
		case "redis":
			keys = append(keys, "REDIS_URL")
		case "rabbitmq":
			keys = append(keys, "RABBITMQ_URL")
		}
	}
	return uniqueSortedStrings(keys)
}

func requiredEnvKeysFromInternalServicesModelFallback(store ProjectInternalServiceStore, projectID string) []string {
	if store == nil {
		return nil
	}
	items, err := store.ListByProject(projectID)
	if err != nil {
		return nil
	}
	return requiredEnvKeysFromInternalServices(items)
}

func uniqueSortedStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	unique := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	slices.Sort(unique)
	return unique
}

func normalizeAssistantSourceRef(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "refs/") {
		return trimmed
	}
	if matched, _ := regexp.MatchString(`^[a-fA-F0-9]{7,40}$`, trimmed); matched {
		return trimmed
	}
	return "refs/heads/" + trimmed
}

func appendMissingInput(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func buildMissingInputPrompt(fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 && fields[0] == "target_environment" {
		return "Which environment should I deploy to, for example staging or production?"
	}
	if len(fields) == 1 && fields[0] == "source_ref" {
		return "Which ref should I deploy, for example a branch name, tag, commit SHA, or PR number?"
	}
	return "I need two details before I can prepare the deploy: the source ref and the target environment."
}

func buildExecutionMessage(execution *AssistantExecutionRecord) string {
	if execution == nil {
		return "Execution started."
	}
	if execution.Status == "failed" {
		return fmt.Sprintf("Deploy failed to start: %s", firstRuntimeNonEmpty(execution.Reason, "unknown error"))
	}
	if execution.DeploymentID != "" {
		return fmt.Sprintf("Deployment %s created. I also tracked revision %s.", execution.DeploymentID, execution.RevisionID)
	}
	if execution.BuildJobID != "" {
		return fmt.Sprintf("Build job %s was queued before deploy execution.", execution.BuildJobID)
	}
	return "Deploy execution completed."
}

func executionState(execution *AssistantExecutionRecord) string {
	if execution == nil {
		return AssistantUIStateExecuting
	}
	if execution.Status == "failed" {
		return AssistantUIStateFailed
	}
	return AssistantUIStateCompleted
}

func toAssistantSessionRecord(item models.AssistantSession) AssistantSessionRecord {
	return AssistantSessionRecord{
		ID:            item.ID,
		ProjectID:     item.ProjectID,
		Title:         item.Title,
		Status:        item.Status,
		LastMessageAt: item.LastMessageAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func toAssistantMessageRecord(item models.AssistantMessage) AssistantMessageRecord {
	record := AssistantMessageRecord{
		ID:        item.ID,
		SessionID: item.SessionID,
		Role:      item.Role,
		Kind:      item.Kind,
		Content:   item.Content,
		CreatedAt: item.CreatedAt,
	}
	if strings.TrimSpace(item.ContentJSON) != "" && item.ContentJSON != "{}" {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(item.ContentJSON), &decoded); err == nil {
			record.ContentData = decoded
		}
	}
	return record
}

func toAssistantPlanRecord(item models.AssistantActionPlan, missing []AssistantMissingInput) AssistantPlanRecord {
	record := AssistantPlanRecord{
		ID:                   item.ID,
		ActionType:           item.ActionType,
		Status:               item.Status,
		RiskLevel:            item.RiskLevel,
		RequiresConfirmation: item.RequiresConfirmation,
		MissingInputs:        missing,
		Plan:                 map[string]any{},
		ExpiresAt:            item.ExpiresAt,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
	if err := json.Unmarshal([]byte(item.PlanJSON), &record.Plan); err == nil {
		if summary, ok := record.Plan["summary"].(string); ok {
			record.Summary = summary
		}
	}
	return record
}

func toPlanRecordFromJSON(item *models.AssistantActionPlan) *AssistantPlanRecord {
	if item == nil {
		return nil
	}
	record := toAssistantPlanRecord(*item, nil)
	return &record
}
