package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/api/response"
	"lazyops-server/internal/service"
)

type ObservabilityController struct {
	projects      service.ProjectStore
	observability *service.ObservabilityService
}

func NewObservabilityController(projects service.ProjectStore, observability *service.ObservabilityService) *ObservabilityController {
	return &ObservabilityController{
		projects:      projects,
		observability: observability,
	}
}

func (ctl *ObservabilityController) GetTrace(c *gin.Context) {
	claims := middleware.MustClaims(c)
	record, err := ctl.observability.GetTraceByCorrelationID(c.Request.Context(), c.Param("correlation_id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load trace", "invalid_input", err.Error())
		case errors.Is(err, service.ErrTraceNotFound):
			response.Error(c, http.StatusNotFound, "failed to load trace", "trace_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load trace", "internal_error", err.Error())
		}
		return
	}

	if _, err := resolveProjectForClaims(ctl.projects, claims, record.ProjectID); err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load trace", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load trace", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load trace", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "trace loaded", gin.H{
		"correlation_id":   record.CorrelationID,
		"service_path":     buildTraceServicePath(*record),
		"node_hops":        buildTraceNodeHops(*record),
		"latency_hotspot":  buildTraceLatencyHotspot(*record),
		"total_latency_ms": int(math.Round(record.DurationMs)),
	})
}

func (ctl *ObservabilityController) GetTopology(c *gin.Context) {
	claims := middleware.MustClaims(c)
	project, err := resolveProjectForClaims(ctl.projects, claims, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load topology", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load topology", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load topology", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load topology", "internal_error", err.Error())
		}
		return
	}

	graph, err := ctl.observability.BuildTopologyGraphForUser(c.Request.Context(), project.ID, project.UserID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to load topology", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "topology loaded", graph)
}

func (ctl *ObservabilityController) StreamLogs(c *gin.Context) {
	claims := middleware.MustClaims(c)
	project, err := resolveProjectForClaims(ctl.projects, claims, c.Query("project"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load logs", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to load logs", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to load logs", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load logs", "internal_error", err.Error())
		}
		return
	}

	serviceName := strings.TrimSpace(c.Query("service"))
	if serviceName == "" {
		response.Error(c, http.StatusBadRequest, "failed to load logs", "missing_service", "service query parameter is required")
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsedLimit, convErr := strconv.Atoi(rawLimit)
		if convErr != nil || parsedLimit <= 0 {
			response.Error(c, http.StatusBadRequest, "failed to load logs", "invalid_limit", "limit query parameter must be a positive integer")
			return
		}
		limit = parsedLimit
	}

	preview, err := ctl.observability.PreviewLogs(c.Request.Context(), service.PreviewLogsCommand{
		ProjectID:   project.ID,
		ServiceName: serviceName,
		Level:       c.Query("level"),
		Contains:    c.Query("contains"),
		Node:        c.Query("node"),
		Cursor:      c.Query("cursor"),
		Limit:       limit,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load logs", "invalid_logs_query", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load logs", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "logs loaded", preview)
}

func (ctl *ObservabilityController) GetCorrelatedObservability(c *gin.Context) {
	claims := middleware.MustClaims(c)
	correlationID := strings.TrimSpace(c.Query("correlation_id"))
	if correlationID == "" {
		response.Error(c, http.StatusBadRequest, "correlation_id is required", "missing_correlation_id", nil)
		return
	}

	project, err := resolveProjectForClaims(ctl.projects, claims, c.Query("project"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to correlate", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "failed to correlate", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "failed to correlate", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to correlate", "internal_error", err.Error())
		}
		return
	}

	result, err := ctl.observability.GetCorrelatedObservability(c.Request.Context(), project.ID, correlationID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to correlate", "invalid_input", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to correlate", "internal_error", err.Error())
		}
		return
	}

	response.JSON(c, http.StatusOK, "correlated observability loaded", result)
}

func (ctl *ObservabilityController) GetTraceLogs(c *gin.Context) {
	claims := middleware.MustClaims(c)
	traceID := strings.TrimSpace(c.Param("correlation_id"))
	if traceID == "" {
		response.Error(c, http.StatusBadRequest, "correlation_id is required", "missing_correlation_id", nil)
		return
	}

	trace, err := ctl.observability.GetTraceByCorrelationID(c.Request.Context(), traceID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTraceNotFound):
			response.Error(c, http.StatusNotFound, "trace not found", "trace_not_found", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load trace", "internal_error", err.Error())
		}
		return
	}

	project, err := resolveProjectForClaims(ctl.projects, claims, trace.ProjectID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "project not found", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "project access denied", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load logs", "internal_error", err.Error())
		}
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := ctl.observability.GetTraceLogs(c.Request.Context(), service.TraceLogsQuery{
		ProjectID:     project.ID,
		CorrelationID: traceID,
		Limit:         limit,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to load trace logs", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "trace logs loaded", gin.H{
		"correlation_id": traceID,
		"logs":           logs,
		"total":          len(logs),
	})
}

func (ctl *ObservabilityController) GetTopologyNodeLogs(c *gin.Context) {
	claims := middleware.MustClaims(c)
	nodeID := strings.TrimSpace(c.Param("node_ref"))
	if nodeID == "" {
		response.Error(c, http.StatusBadRequest, "node is required", "missing_node", nil)
		return
	}

	project, err := resolveProjectForClaims(ctl.projects, claims, c.Param("project"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to load node logs", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "project not found", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "project access denied", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to load node logs", "internal_error", err.Error())
		}
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := ctl.observability.GetTopologyNodeLogs(c.Request.Context(), service.TopologyNodeLogsQuery{
		ProjectID: project.ID,
		NodeRef:   nodeID,
		Limit:     limit,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to load node logs", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "node logs loaded", gin.H{
		"node":  nodeID,
		"logs":  logs,
		"total": len(logs),
	})
}

func (ctl *ObservabilityController) QueryObservability(c *gin.Context) {
	claims := middleware.MustClaims(c)

	var req struct {
		ProjectID     string `json:"project_id" binding:"required"`
		CorrelationID string `json:"correlation_id"`
		NodeID        string `json:"node_id"`
		ServiceName   string `json:"service_name"`
		Level         string `json:"level"`
		Contains      string `json:"contains"`
		Limit         int    `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid query payload", "invalid_input", err.Error())
		return
	}

	if req.ProjectID == "" {
		response.Error(c, http.StatusBadRequest, "project_id is required", "missing_project_id", nil)
		return
	}

	project, err := resolveProjectForClaims(ctl.projects, claims, req.ProjectID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, "failed to query", "invalid_input", err.Error())
		case errors.Is(err, service.ErrProjectNotFound):
			response.Error(c, http.StatusNotFound, "project not found", "project_not_found", err.Error())
		case errors.Is(err, service.ErrProjectAccessDenied):
			response.Error(c, http.StatusForbidden, "project access denied", "project_access_denied", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "failed to query", "internal_error", err.Error())
		}
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	result, err := ctl.observability.QueryObservabilityData(c.Request.Context(), service.ObservabilityQuery{
		ProjectID:     project.ID,
		CorrelationID: req.CorrelationID,
		NodeID:        req.NodeID,
		ServiceName:   req.ServiceName,
		Level:         req.Level,
		Contains:      req.Contains,
		Limit:         limit,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to query observability", "internal_error", err.Error())
		return
	}

	response.JSON(c, http.StatusOK, "observability query completed", result)
}

func buildTraceServicePath(record service.TraceRecord) []string {
	path := make([]string, 0, 4)

	if hops, ok := record.Metadata["hops"].([]any); ok {
		for _, hop := range hops {
			hopMap, ok := hop.(map[string]any)
			if !ok {
				continue
			}
			from := strings.TrimSpace(stringValue(hopMap["from"]))
			to := strings.TrimSpace(stringValue(hopMap["to"]))
			if from != "" {
				path = appendIfMissing(path, from)
			}
			if to != "" {
				path = appendIfMissing(path, to)
			}
		}
	}

	if len(path) == 0 && strings.TrimSpace(record.ServiceName) != "" {
		path = append(path, record.ServiceName)
	}
	if len(path) == 0 && strings.TrimSpace(record.Operation) != "" {
		path = append(path, record.Operation)
	}
	if len(path) == 0 {
		path = append(path, "unknown")
	}

	return path
}

func buildTraceNodeHops(record service.TraceRecord) []string {
	hops := make([]string, 0, 4)

	if rawNodeHops, ok := record.Metadata["node_hops"].([]any); ok {
		for _, hop := range rawNodeHops {
			value := strings.TrimSpace(stringValue(hop))
			if value != "" {
				hops = append(hops, value)
			}
		}
	}

	if len(hops) == 0 {
		if rawHops, ok := record.Metadata["hops"].([]any); ok {
			for _, rawHop := range rawHops {
				hopMap, ok := rawHop.(map[string]any)
				if !ok {
					continue
				}
				from := strings.TrimSpace(stringValue(hopMap["from"]))
				to := strings.TrimSpace(stringValue(hopMap["to"]))
				switch {
				case from != "" && to != "":
					hops = append(hops, from+" -> "+to)
				case to != "":
					hops = append(hops, to)
				case from != "":
					hops = append(hops, from)
				}
			}
		}
	}

	return hops
}

func buildTraceLatencyHotspot(record service.TraceRecord) string {
	if value := strings.TrimSpace(stringValue(record.Metadata["latency_hotspot"])); value != "" {
		return value
	}
	if strings.TrimSpace(record.ErrorSummary) != "" {
		return record.ErrorSummary
	}
	if strings.TrimSpace(record.ServiceName) != "" && record.DurationMs > 0 {
		return record.ServiceName
	}
	return ""
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func appendIfMissing(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
