package service

import "strings"

const assistantPlannerMinConfidence = 0.55

func guardAssistantIntent(sessionProjectID, content string, intent *AssistantPlannedIntent) *AssistantPlannedIntent {
	projectID := strings.TrimSpace(sessionProjectID)
	if intent == nil {
		return &AssistantPlannedIntent{Intent: AssistantIntentUnknown, Confidence: 0.0, ProjectID: projectID, Reason: "planner returned no intent"}
	}
	guarded := *intent
	guarded.Intent = strings.TrimSpace(guarded.Intent)
	guarded.ProjectID = projectID
	guarded.Window = normalizeAssistantIntentWindow(guarded.Window)
	guarded.Limit = normalizeAssistantIntentLimit(guarded.Limit)
	guarded.ServiceName = strings.TrimSpace(guarded.ServiceName)
	guarded.SourceRef = strings.TrimSpace(guarded.SourceRef)
	guarded.TargetEnvironment = strings.ToLower(strings.TrimSpace(guarded.TargetEnvironment))
	guarded.RepoFullName = strings.TrimSpace(guarded.RepoFullName)
	guarded.BindingHint = strings.TrimSpace(guarded.BindingHint)

	if isUnsafeAssistantPrompt(content) {
		return &AssistantPlannedIntent{Intent: AssistantIntentUnknown, Confidence: 0.0, ProjectID: projectID, Window: guarded.Window, Limit: guarded.Limit, Reason: "request is outside the allowed assistant tool surface"}
	}
	if !isAllowedAssistantIntent(guarded.Intent) || guarded.Confidence < assistantPlannerMinConfidence {
		guarded.Intent = AssistantIntentUnknown
		guarded.RequiresMutation = false
		if guarded.Reason == "" {
			guarded.Reason = "I could not classify that request safely."
		}
		return &guarded
	}
	if guarded.Intent != AssistantIntentDeployRef && guarded.RequiresMutation {
		guarded.Intent = AssistantIntentUnknown
		guarded.RequiresMutation = false
		guarded.Reason = "Only typed deploy plans can mutate infrastructure in this assistant phase."
		return &guarded
	}
	if guarded.Intent == AssistantIntentDeployRef {
		guarded.RequiresMutation = true
		if guarded.TargetEnvironment == "prod" {
			guarded.TargetEnvironment = "production"
		}
	}
	return &guarded
}

func isAllowedAssistantIntent(intent string) bool {
	switch intent {
	case AssistantIntentDeployRef,
		AssistantIntentDeploymentStatus,
		AssistantIntentQueryLogs,
		AssistantIntentExplainIncident,
		AssistantIntentQueryTopology,
		AssistantIntentReviewSystem,
		AssistantIntentQueryMetrics,
		AssistantIntentActivityTable,
		AssistantIntentUnknown:
		return true
	default:
		return false
	}
}

func normalizeAssistantIntentWindow(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1h", "6h", "24h", "7d", "30d":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "24h"
	}
}

func normalizeAssistantIntentLimit(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 100 {
		return 100
	}
	return value
}

func isUnsafeAssistantPrompt(content string) bool {
	query := strings.ToLower(strings.TrimSpace(content))
	unsafeTokens := []string{
		"ignore previous instructions",
		"ignore all previous",
		"dump secret",
		"show secret",
		"read secret",
		"print env",
		"run shell",
		"execute shell",
		"arbitrary sql",
		"drop table",
		"agent command",
		"restart service",
		"rollback production",
		"scale service",
		"mutate routing",
	}
	for _, token := range unsafeTokens {
		if strings.Contains(query, token) {
			return true
		}
	}
	return false
}
