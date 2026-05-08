package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type IncidentExplanationService struct {
	observability  *ObservabilityService
	errorKnowledge *ErrorKnowledgeService
	deployments    *DeploymentService
}

type ExplainIncidentCommand struct {
	ProjectID     string
	IncidentID    string
	CorrelationID string
	ServiceName   string
	QueryText     string
}

type IncidentExplanationRecord struct {
	Incident          *IncidentRecord           `json:"incident,omitempty"`
	Summary           string                    `json:"summary"`
	LikelyCause       string                    `json:"likely_cause"`
	Confidence        string                    `json:"confidence"`
	Timeline          []IncidentTimelineItem    `json:"timeline"`
	Correlations      IncidentCorrelationRecord `json:"correlations"`
	Citations         []IncidentCitation        `json:"citations"`
	Recommendations   []IncidentRecommendation  `json:"recommendations"`
	HistoricalMatches []map[string]any          `json:"historical_matches,omitempty"`
}

type IncidentTimelineItem struct {
	Timestamp time.Time `json:"timestamp"`
	Kind      string    `json:"kind"`
	Severity  string    `json:"severity,omitempty"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail,omitempty"`
	SourceID  string    `json:"source_id,omitempty"`
}

type IncidentCorrelationRecord struct {
	CorrelationID string `json:"correlation_id,omitempty"`
	TraceStatus   string `json:"trace_status,omitempty"`
	TraceError    string `json:"trace_error,omitempty"`
	LogCount      int    `json:"log_count"`
	TopologyNodes int    `json:"topology_nodes"`
	TopologyEdges int    `json:"topology_edges"`
}

type IncidentCitation struct {
	ID        string     `json:"id"`
	Source    string     `json:"source"`
	Title     string     `json:"title"`
	Excerpt   string     `json:"excerpt"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
}

type IncidentRecommendation struct {
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

func NewIncidentExplanationService(observability *ObservabilityService, errorKnowledge *ErrorKnowledgeService, deployments *DeploymentService) *IncidentExplanationService {
	return &IncidentExplanationService{observability: observability, errorKnowledge: errorKnowledge, deployments: deployments}
}

func (s *IncidentExplanationService) Explain(ctx context.Context, cmd ExplainIncidentCommand) (*IncidentExplanationRecord, error) {
	projectID := strings.TrimSpace(cmd.ProjectID)
	if s == nil || s.observability == nil || projectID == "" {
		return nil, ErrInvalidInput
	}
	incidents, err := s.observability.ListIncidentsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	incident := selectIncidentForExplanation(incidents, cmd.IncidentID)
	if incident == nil && strings.TrimSpace(cmd.QueryText) == "" {
		return nil, ErrProjectNotFound
	}
	serviceName := strings.TrimSpace(cmd.ServiceName)
	correlationID := strings.TrimSpace(cmd.CorrelationID)
	queryText := strings.TrimSpace(cmd.QueryText)
	if incident != nil {
		if queryText == "" {
			queryText = incident.Summary
		}
		if correlationID == "" {
			correlationID = stringFromIncidentDetails(incident.Details, "correlation_id")
		}
		if serviceName == "" {
			serviceName = stringFromIncidentDetails(incident.Details, "service_name")
		}
	}
	if serviceName == "" {
		serviceName = extractAssistantServiceName(strings.ToLower(queryText))
	}
	var correlated *CorrelatedObservability
	if correlationID != "" {
		correlated, _ = s.observability.GetCorrelatedObservability(ctx, projectID, correlationID)
	}
	recentLogs, _ := s.observability.ListRecentLogs(ctx, projectID, serviceName, "", "error", "", 25)
	historical := make([]map[string]any, 0)
	if s.errorKnowledge != nil && queryText != "" {
		matches, matchErr := s.errorKnowledge.FindSimilar(projectID, serviceName, queryText, 5)
		if matchErr == nil {
			historical = toErrorKnowledgePreviewList(matches)
		}
	}
	timeline, citations := buildIncidentTimelineAndCitations(incident, correlated, recentLogs, historical)
	likelyCause, confidence := deriveIncidentLikelyCause(queryText, correlated, recentLogs, historical)
	recommendations := deriveIncidentRecommendations(likelyCause, serviceName, incident, correlated, recentLogs, historical)
	correlations := IncidentCorrelationRecord{CorrelationID: correlationID}
	if correlated != nil {
		correlations.LogCount = len(correlated.Logs)
		if correlated.Trace != nil {
			correlations.TraceStatus = correlated.Trace.Status
			correlations.TraceError = correlated.Trace.ErrorSummary
		}
		if correlated.Topology != nil {
			correlations.TopologyNodes = len(correlated.Topology.Nodes)
			correlations.TopologyEdges = len(correlated.Topology.Edges)
		}
	}
	summary := buildIncidentExplanationSummary(incident, likelyCause, len(citations), len(historical))
	return &IncidentExplanationRecord{Incident: incident, Summary: summary, LikelyCause: likelyCause, Confidence: confidence, Timeline: timeline, Correlations: correlations, Citations: citations, Recommendations: recommendations, HistoricalMatches: historical}, nil
}

func selectIncidentForExplanation(items []IncidentRecord, incidentID string) *IncidentRecord {
	incidentID = strings.TrimSpace(incidentID)
	for _, item := range items {
		if incidentID != "" && item.ID == incidentID {
			copy := item
			return &copy
		}
	}
	for _, item := range items {
		if item.Status == IncidentStatusOpen || strings.EqualFold(item.Status, "investigating") {
			copy := item
			return &copy
		}
	}
	if len(items) > 0 {
		copy := items[0]
		return &copy
	}
	return nil
}

func stringFromIncidentDetails(details map[string]any, key string) string {
	if details == nil {
		return ""
	}
	if value, ok := details[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func buildIncidentTimelineAndCitations(incident *IncidentRecord, correlated *CorrelatedObservability, recentLogs []LogLineRecord, historical []map[string]any) ([]IncidentTimelineItem, []IncidentCitation) {
	timeline := make([]IncidentTimelineItem, 0)
	citations := make([]IncidentCitation, 0)
	if incident != nil {
		timeline = append(timeline, IncidentTimelineItem{Timestamp: incident.CreatedAt, Kind: "incident", Severity: incident.Severity, Title: "Incident opened", Detail: incident.Summary, SourceID: incident.ID})
		at := incident.CreatedAt
		citations = append(citations, IncidentCitation{ID: incident.ID, Source: "incident", Title: incident.Kind, Excerpt: incident.Summary, Timestamp: &at})
	}
	if correlated != nil && correlated.Trace != nil {
		at := correlated.Trace.ReceivedAt
		timeline = append(timeline, IncidentTimelineItem{Timestamp: correlated.Trace.ReceivedAt, Kind: "trace", Severity: correlated.Trace.Status, Title: correlated.Trace.Operation, Detail: correlated.Trace.ErrorSummary, SourceID: correlated.Trace.CorrelationID})
		citations = append(citations, IncidentCitation{ID: correlated.Trace.CorrelationID, Source: "trace", Title: correlated.Trace.Operation, Excerpt: correlated.Trace.ErrorSummary, Timestamp: &at})
	}
	logs := recentLogs
	if correlated != nil && len(correlated.Logs) > 0 {
		logs = correlated.Logs
	}
	for i, log := range logs {
		if i >= 5 {
			break
		}
		at := log.Timestamp
		timeline = append(timeline, IncidentTimelineItem{Timestamp: log.Timestamp, Kind: "log", Severity: log.Level, Title: firstRuntimeNonEmpty(log.Service, "runtime log"), Detail: log.Message, SourceID: log.ID})
		citations = append(citations, IncidentCitation{ID: firstRuntimeNonEmpty(log.ID, fmt.Sprintf("log-%d", i)), Source: "log", Title: firstRuntimeNonEmpty(log.Service, "log"), Excerpt: log.Message, Timestamp: &at})
	}
	for i, item := range historical {
		if i >= 3 {
			break
		}
		id := fmt.Sprint(item["id"])
		body := fmt.Sprint(item["body"])
		citations = append(citations, IncidentCitation{ID: id, Source: "error_knowledge", Title: fmt.Sprint(item["title"]), Excerpt: body})
	}
	sort.Slice(timeline, func(i, j int) bool { return timeline[i].Timestamp.Before(timeline[j].Timestamp) })
	return timeline, citations
}

func deriveIncidentLikelyCause(queryText string, correlated *CorrelatedObservability, recentLogs []LogLineRecord, historical []map[string]any) (string, string) {
	text := strings.ToLower(queryText)
	if correlated != nil && correlated.Trace != nil {
		text += " " + strings.ToLower(correlated.Trace.ErrorSummary)
	}
	for _, log := range recentLogs {
		text += " " + strings.ToLower(log.Message)
	}
	switch {
	case containsAny(text, "oom", "out of memory", "killed"):
		return "Memory pressure or OOM termination is the most likely cause.", "high"
	case containsAny(text, "connection refused", "refused", "dial tcp"):
		return "A dependency or upstream service appears unreachable or refusing connections.", "high"
	case containsAny(text, "timeout", "deadline exceeded"):
		return "Requests are timing out, likely due to dependency latency or overloaded runtime.", "medium"
	case containsAny(text, "panic", "exception", "stack trace"):
		return "Application runtime exception is the most likely cause.", "high"
	case containsAny(text, "crash", "crashloop"):
		return "The service appears to be crash-looping or repeatedly failing during startup.", "high"
	case containsAny(text, "health gate", "unhealthy"):
		return "A rollout health gate or candidate readiness check failed.", "medium"
	case len(historical) > 0:
		return "This resembles previously indexed error knowledge for the same project or service.", "medium"
	default:
		return "The available evidence points to a runtime error, but more correlated logs or traces are needed for a precise cause.", "low"
	}
}

func deriveIncidentRecommendations(cause, serviceName string, incident *IncidentRecord, correlated *CorrelatedObservability, recentLogs []LogLineRecord, historical []map[string]any) []IncidentRecommendation {
	recommendations := make([]IncidentRecommendation, 0)
	lower := strings.ToLower(cause)
	if containsAny(lower, "memory", "oom") {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "high", Action: "Inspect memory usage and container memory limits", Reason: "OOM-like errors usually require resource or traffic analysis."})
	}
	if containsAny(lower, "dependency", "unreachable", "refusing") {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "high", Action: "Check dependent service health and topology links", Reason: "Connection failures often originate outside the failing service."})
	}
	if containsAny(lower, "timeout", "latency") {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "medium", Action: "Inspect p95 latency and recent trace errors", Reason: "Timeouts need latency and trace correlation before remediation."})
	}
	if containsAny(lower, "exception", "runtime") {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "high", Action: "Inspect the latest stack excerpt and recent code changes", Reason: "Runtime exceptions are usually application-level regressions."})
	}
	if incident != nil && incident.DeploymentID != "" {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "medium", Action: "Review the linked deployment before considering rollback", Reason: "The incident is tied to a deployment context."})
	}
	if serviceName != "" {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "medium", Action: "Open logs filtered to service " + serviceName, Reason: "Service-scoped logs reduce noise during triage."})
	}
	if len(historical) > 0 {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "low", Action: "Compare with historical error knowledge matches", Reason: "Similar prior errors may contain known resolution clues."})
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, IncidentRecommendation{Priority: "medium", Action: "Inspect correlated logs, traces, and latest deployment", Reason: "The current evidence is insufficient for an automated fix recommendation."})
	}
	return recommendations
}

func buildIncidentExplanationSummary(incident *IncidentRecord, likelyCause string, citations, matches int) string {
	if incident != nil {
		return fmt.Sprintf("Incident %s is %s/%s. %s Evidence includes %d citations and %d historical matches.", incident.ID, firstRuntimeNonEmpty(incident.Severity, "unknown"), firstRuntimeNonEmpty(incident.Status, "unknown"), likelyCause, citations, matches)
	}
	return fmt.Sprintf("No incident record was found, but related evidence was analyzed. %s Evidence includes %d citations and %d historical matches.", likelyCause, citations, matches)
}
