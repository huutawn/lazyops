package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const proactiveAlertSuppressionWindow = 15 * time.Minute

type ProactiveAlertService struct {
	states       ProactiveAlertStateStore
	incidents    RuntimeIncidentStore
	projects     ProjectStore
	explanations *IncidentExplanationService
	operatorHub  OperatorEventBroadcaster
}

type ProactiveAlertRecord struct {
	ProjectID     string                     `json:"project_id"`
	ServiceName   string                     `json:"service_name"`
	Severity      string                     `json:"severity"`
	Fingerprint   string                     `json:"fingerprint"`
	CorrelationID string                     `json:"correlation_id,omitempty"`
	IncidentID    string                     `json:"incident_id,omitempty"`
	Message       string                     `json:"message"`
	Suppressed    bool                       `json:"suppressed"`
	Explanation   *IncidentExplanationRecord `json:"explanation,omitempty"`
}

func NewProactiveAlertService(states ProactiveAlertStateStore, incidents RuntimeIncidentStore, projects ProjectStore, explanations *IncidentExplanationService, operatorHub OperatorEventBroadcaster) *ProactiveAlertService {
	return &ProactiveAlertService{states: states, incidents: incidents, projects: projects, explanations: explanations, operatorHub: operatorHub}
}

func (s *ProactiveAlertService) ProcessLogBatch(ctx context.Context, records []models.LogStreamEntry) ([]ProactiveAlertRecord, error) {
	if s == nil || s.states == nil || s.incidents == nil || len(records) == 0 {
		return nil, nil
	}
	alerts := make([]ProactiveAlertRecord, 0)
	seen := make(map[string]models.LogStreamEntry)
	for _, record := range records {
		if !shouldIndexErrorKnowledge(record) {
			continue
		}
		fingerprint := proactiveAlertFingerprint(record.ProjectID, record.ServiceName, record.Message)
		key := record.ProjectID + "|" + record.ServiceName + "|" + fingerprint
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = record
	}
	for _, record := range seen {
		alert, err := s.processLogRecord(ctx, record)
		if err != nil {
			return alerts, err
		}
		alerts = append(alerts, *alert)
	}
	return alerts, nil
}

func (s *ProactiveAlertService) processLogRecord(ctx context.Context, record models.LogStreamEntry) (*ProactiveAlertRecord, error) {
	now := time.Now().UTC()
	severity := proactiveAlertSeverity(record)
	fingerprint := proactiveAlertFingerprint(record.ProjectID, record.ServiceName, record.Message)
	state, err := s.states.GetByFingerprint(record.ProjectID, record.ServiceName, fingerprint)
	if err != nil {
		return nil, err
	}
	suppressed := false
	if state != nil && state.SuppressedUntil != nil && now.Before(*state.SuppressedUntil) && !proactiveAlertEscalates(state.LastSeverity, severity) {
		suppressed = true
	}
	if state == nil {
		state = &models.ProactiveAlertState{ID: utils.NewPrefixedID("pa"), ProjectID: record.ProjectID, ServiceName: record.ServiceName, Fingerprint: fingerprint, FirstSeenAt: record.OccurredAt, CreatedAt: now}
	}
	state.Count++
	state.LastSeenAt = record.OccurredAt
	state.LastSeverity = severity
	state.UpdatedAt = now
	suppressUntil := now.Add(proactiveAlertSuppressionWindow)
	state.SuppressedUntil = &suppressUntil
	alert := &ProactiveAlertRecord{ProjectID: record.ProjectID, ServiceName: record.ServiceName, Severity: severity, Fingerprint: fingerprint, CorrelationID: record.CorrelationID, Suppressed: suppressed}
	if suppressed {
		alert.Message = fmt.Sprintf("Repeated %s incident in service %s is suppressed until %s.", severity, firstRuntimeNonEmpty(record.ServiceName, "runtime"), suppressUntil.Format(time.RFC3339))
		return alert, s.states.Upsert(state)
	}
	incident, err := s.createIncident(record, severity, fingerprint)
	if err != nil {
		return nil, err
	}
	state.LastIncidentID = incident.ID
	if err := s.states.Upsert(state); err != nil {
		return nil, err
	}
	alert.IncidentID = incident.ID
	alert.Message = fmt.Sprintf("Assistant detected a %s incident in service %s: %s", severity, firstRuntimeNonEmpty(record.ServiceName, "runtime"), record.Message)
	if s.explanations != nil {
		explanation, explainErr := s.explanations.Explain(ctx, ExplainIncidentCommand{ProjectID: record.ProjectID, IncidentID: incident.ID, CorrelationID: record.CorrelationID, ServiceName: record.ServiceName, QueryText: record.Message})
		if explainErr == nil {
			alert.Explanation = explanation
		}
	}
	_ = s.broadcastAlert(alert)
	return alert, nil
}

func (s *ProactiveAlertService) createIncident(record models.LogStreamEntry, severity, fingerprint string) (*models.RuntimeIncident, error) {
	details, _ := json.Marshal(map[string]any{"source": "log_ingest", "fingerprint": fingerprint, "correlation_id": record.CorrelationID, "service_name": record.ServiceName, "sample_log_ids": []string{record.ID}, "sample_messages": []string{record.Message}})
	incident := &models.RuntimeIncident{ID: utils.NewPrefixedID("inc"), ProjectID: record.ProjectID, DeploymentID: "", RevisionID: record.RevisionID, Kind: proactiveAlertIncidentKind(record), Severity: severity, Status: IncidentStatusOpen, Summary: record.Message, DetailsJSON: string(details), TriggeredBy: "proactive_alert", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := s.incidents.Create(incident); err != nil {
		return nil, err
	}
	return incident, nil
}

func (s *ProactiveAlertService) broadcastAlert(alert *ProactiveAlertRecord) error {
	if s == nil || s.operatorHub == nil || alert == nil || alert.Suppressed {
		return nil
	}
	payload := map[string]any{"project_id": alert.ProjectID, "incident_id": alert.IncidentID, "service_name": alert.ServiceName, "severity": alert.Severity, "summary": alert.Message, "message": alert.Message, "correlation_id": alert.CorrelationID, "suppressed": alert.Suppressed, "explanation": alert.Explanation}
	if s.projects != nil {
		project, err := s.projects.GetByID(alert.ProjectID)
		if err == nil && project != nil && strings.TrimSpace(project.UserID) != "" {
			return s.operatorHub.BroadcastEventToUser(project.UserID, "assistant.incident_detected", payload)
		}
	}
	return s.operatorHub.BroadcastEvent("assistant.incident_detected", payload)
}

func proactiveAlertFingerprint(projectID, serviceName, message string) string {
	normalized := normalizeKnowledgeText(message)
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(projectID + "|" + serviceName + "|" + normalized))))
	return hex.EncodeToString(sum[:])
}

func proactiveAlertSeverity(record models.LogStreamEntry) string {
	level := strings.ToLower(strings.TrimSpace(record.Level))
	message := strings.ToLower(strings.TrimSpace(record.Message))
	if level == "critical" || level == "fatal" || containsAny(message, "panic", "oom", "out of memory", "crashloop") {
		return IncidentSeverityCritical
	}
	if level == "error" || containsAny(message, "timeout", "refused", "failed", "exception", "health gate failed") {
		return IncidentSeverityWarning
	}
	return IncidentSeverityInfo
}

func proactiveAlertEscalates(previous, next string) bool {
	return proactiveAlertSeverityRank(next) > proactiveAlertSeverityRank(previous)
}

func proactiveAlertSeverityRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case IncidentSeverityCritical:
		return 3
	case IncidentSeverityWarning:
		return 2
	case IncidentSeverityInfo:
		return 1
	default:
		return 0
	}
}

func proactiveAlertIncidentKind(record models.LogStreamEntry) string {
	message := strings.ToLower(strings.TrimSpace(record.Message))
	switch {
	case containsAny(message, "crash", "crashloop"):
		return IncidentKindCrashLoop
	case containsAny(message, "health gate", "unhealthy"):
		return IncidentKindHealthGateTimeout
	default:
		return "runtime_error"
	}
}
