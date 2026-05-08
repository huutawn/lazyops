package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"lazyops-server/internal/models"
)

type assistantActivityItem struct {
	Time   time.Time `json:"time"`
	Kind   string    `json:"kind"`
	Status string    `json:"status"`
	Title  string    `json:"title"`
	Ref    string    `json:"ref,omitempty"`
}

func (s *AssistantService) handleMetricsDashboard(projectID string, planned *AssistantPlannedIntent) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	if s.observability == nil {
		return "Metrics are not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
	window := "24h"
	serviceName := ""
	if planned != nil {
		window = planned.Window
		serviceName = planned.ServiceName
	}
	dashboard, err := s.observability.BuildMetricDashboard(context.Background(), projectID, serviceName, window, "")
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	message := fmt.Sprintf("Metrics for %s show %d requests, p95 latency %.0fms, CPU p95 %.2f, RAM p95 %.0fMB, %d open incidents, and %d recent errors.", firstRuntimeNonEmpty(window, "24h"), dashboard.Summary.RequestTotal, dashboard.Summary.LatencyP95Ms, dashboard.Summary.CpuP95, dashboard.Summary.RamP95MB, dashboard.Summary.OpenIncidents, dashboard.Summary.RecentErrors)
	return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "metrics_dashboard", "project_id": projectID, "metrics": dashboard}, nil, nil, nil
}

func (s *AssistantService) handleActivityTable(session *models.AssistantSession, role, projectID string, planned *AssistantPlannedIntent) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	limit := 20
	if planned != nil {
		limit = planned.Limit
	}
	items := make([]assistantActivityItem, 0)
	if s.deploySvc != nil {
		deployments, err := s.deploySvc.List(session.UserID, role, projectID)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		for _, item := range deployments {
			items = append(items, assistantActivityItem{Time: item.CreatedAt, Kind: "deployment", Status: item.RolloutState, Title: fmt.Sprintf("Deployment %s %s", item.ID, firstRuntimeNonEmpty(item.RolloutState, "updated")), Ref: item.RevisionID})
		}
	}
	if s.observability != nil {
		incidents, err := s.observability.ListIncidentsByProject(context.Background(), projectID)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		for _, item := range incidents {
			items = append(items, assistantActivityItem{Time: item.CreatedAt, Kind: "incident", Status: item.Status, Title: fmt.Sprintf("%s incident: %s", firstRuntimeNonEmpty(item.Severity, "runtime"), firstRuntimeNonEmpty(item.Summary, item.Kind)), Ref: item.DeploymentID})
		}
		logs, err := s.observability.ListRecentLogs(context.Background(), projectID, "", "", "error", "", limit)
		if err != nil {
			return "", "", "", nil, nil, nil, err
		}
		for _, log := range logs {
			items = append(items, assistantActivityItem{Time: log.Timestamp, Kind: "error_log", Status: log.Level, Title: fmt.Sprintf("[%s] %s", firstRuntimeNonEmpty(log.Service, "service"), log.Message), Ref: log.CorrelationID})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Time.After(items[j].Time) })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	message := fmt.Sprintf("I found %d recent activity items for this project.", len(items))
	return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "activity_table", "project_id": projectID, "items": items}, nil, nil, nil
}

func (s *AssistantService) handleSystemEvaluation(session *models.AssistantSession, role, projectID string, planned *AssistantPlannedIntent) (string, string, string, map[string]any, *AssistantPlanRecord, *AssistantExecutionRecord, error) {
	if s.runtimeSvc == nil || s.observability == nil {
		return "System evaluation is not available right now.", AssistantMessageKindChat, AssistantUIStateChat, nil, nil, nil, nil
	}
	window := "24h"
	if planned != nil {
		window = planned.Window
	}
	runtime, err := s.runtimeSvc.Get(context.Background(), session.UserID, role, projectID)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	incidents, err := s.observability.ListIncidentsByProject(context.Background(), projectID)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	errorLogs, err := s.observability.ListRecentLogs(context.Background(), projectID, "", "", "error", "", 100)
	if err != nil {
		return "", "", "", nil, nil, nil, err
	}
	dashboard, _ := s.observability.BuildMetricDashboard(context.Background(), projectID, "", window, "")
	deployFailures := 0
	if s.deploySvc != nil {
		deployments, listErr := s.deploySvc.List(session.UserID, role, projectID)
		if listErr != nil {
			return "", "", "", nil, nil, nil, listErr
		}
		for _, deployment := range deployments {
			if strings.EqualFold(deployment.RolloutState, DeploymentStatusFailed) || strings.EqualFold(deployment.BuildState, RevisionStatusFailed) {
				deployFailures++
			}
		}
	}
	degraded := 0
	for _, item := range runtime.Services {
		if strings.EqualFold(strings.TrimSpace(item.RuntimeStatus), "degraded") || strings.EqualFold(strings.TrimSpace(item.RuntimeStatus), "pending") {
			degraded++
		}
	}
	openIncidents := 0
	criticalOpen := 0
	for _, incident := range incidents {
		if incident.Status == IncidentStatusOpen || strings.EqualFold(incident.Status, "investigating") {
			openIncidents++
			if strings.EqualFold(incident.Severity, "critical") {
				criticalOpen++
			}
		}
	}
	score := 100 - minInt(degraded*15, 30) - minInt(openIncidents*10, 30) - minInt(len(errorLogs)/5*5, 20) - minInt(deployFailures*10, 20)
	if criticalOpen > 0 {
		score -= 20
	}
	if dashboard != nil {
		if dashboard.Summary.CpuP95 > 0.85 {
			score -= 10
		}
		if dashboard.Summary.LatencyP95Ms > 1000 {
			score -= 10
		}
	}
	if score < 0 {
		score = 0
	}
	findings := make([]string, 0)
	if degraded > 0 {
		findings = append(findings, fmt.Sprintf("%d services are degraded or pending", degraded))
	}
	if openIncidents > 0 {
		findings = append(findings, fmt.Sprintf("%d incidents are open or investigating", openIncidents))
	}
	if len(errorLogs) > 0 {
		findings = append(findings, fmt.Sprintf("%d recent error logs were found", len(errorLogs)))
	}
	if deployFailures > 0 {
		findings = append(findings, fmt.Sprintf("%d deployment failures are present in recent deployment history", deployFailures))
	}
	if len(findings) == 0 {
		findings = append(findings, "No degraded services, open incidents, or recent error logs were found")
	}
	actions := []string{"Inspect recent logs", "Review deployment history"}
	if degraded > 0 || openIncidents > 0 {
		actions = append([]string{"Open observability and inspect affected services"}, actions...)
	}
	grade := assistantEvaluationGrade(score)
	signals := map[string]any{"services": len(runtime.Services), "nodes": len(runtime.Nodes), "degraded_services": degraded, "open_incidents": openIncidents, "recent_errors": len(errorLogs), "deployment_failures": deployFailures}
	if dashboard != nil {
		signals["request_total"] = dashboard.Summary.RequestTotal
		signals["latency_p95_ms"] = dashboard.Summary.LatencyP95Ms
		signals["cpu_p95"] = dashboard.Summary.CpuP95
		signals["ram_p95_mb"] = dashboard.Summary.RamP95MB
	}
	evaluation := map[string]any{"score": score, "grade": grade, "window": window, "signals": signals, "findings": findings, "recommended_actions": actions}
	message := fmt.Sprintf("System evaluation score is %d/100 (%s). %s", score, grade, findings[0])
	return message, AssistantMessageKindChat, AssistantUIStateChat, map[string]any{"card_type": "system_evaluation", "project_id": projectID, "evaluation": evaluation}, nil, nil, nil
}

func assistantEvaluationGrade(score int) string {
	switch {
	case score >= 90:
		return "healthy"
	case score >= 70:
		return "healthy_with_warnings"
	case score >= 50:
		return "degraded"
	default:
		return "critical"
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
