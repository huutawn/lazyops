package controller

import (
	"errors"
	"math"
	"net/http"
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
