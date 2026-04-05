package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/models"
	"lazyops-server/internal/service"
)

type controllerProjectStore struct {
	projects []*models.Project
}

func (s *controllerProjectStore) Create(project *models.Project) error {
	s.projects = append(s.projects, project)
	return nil
}

func (s *controllerProjectStore) ListByUser(userID string) ([]models.Project, error) {
	out := make([]models.Project, 0)
	for _, project := range s.projects {
		if userID == "" || project.UserID == userID {
			out = append(out, *project)
		}
	}
	return out, nil
}

func (s *controllerProjectStore) GetBySlugForUser(userID, slug string) (*models.Project, error) {
	for _, project := range s.projects {
		if project.UserID == userID && project.Slug == slug {
			return project, nil
		}
	}
	return nil, nil
}

func (s *controllerProjectStore) GetByIDForUser(userID, projectID string) (*models.Project, error) {
	for _, project := range s.projects {
		if project.UserID == userID && project.ID == projectID {
			return project, nil
		}
	}
	return nil, nil
}

func (s *controllerProjectStore) GetByID(projectID string) (*models.Project, error) {
	for _, project := range s.projects {
		if project.ID == projectID {
			return project, nil
		}
	}
	return nil, nil
}

func TestObservabilityControllerStreamLogsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	projects := &controllerProjectStore{
		projects: []*models.Project{{
			ID:     "prj_123",
			UserID: "usr_123",
			Name:   "Acme",
			Slug:   "acme",
		}},
	}

	observability := service.NewObservabilityService(
		nil,
		nil,
		&controllerLogStore{items: []models.LogStreamEntry{
			models.LogStreamEntry{
				ID:          "log_2",
				ProjectID:   "prj_123",
				BindingID:   "bind_123",
				ServiceName: "api",
				Level:       "error",
				Node:        "edge-ap-2",
				Message:     "postgres timeout",
				OccurredAt:  time.Date(2026, 4, 4, 10, 1, 0, 0, time.UTC),
				CollectedAt: time.Date(2026, 4, 4, 10, 2, 0, 0, time.UTC),
			},
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	controller := NewObservabilityController(projects, observability)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/ws/logs/stream?project=prj_123&service=api&level=error&limit=5", nil)
	ctx.Request = req
	ctx.Set("auth.claims", &service.Claims{UserID: "usr_123", Role: service.RoleOperator, AuthKind: service.AuthKindCLIPAT})

	controller.StreamLogs(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Service string `json:"service"`
			Cursor  string `json:"cursor"`
			Lines   []struct {
				Level   string `json:"level"`
				Message string `json:"message"`
				Node    string `json:"node"`
			} `json:"lines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Success || payload.Data.Service != "api" {
		t.Fatalf("unexpected payload: %s", rec.Body.String())
	}
	if len(payload.Data.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(payload.Data.Lines))
	}
	if payload.Data.Lines[0].Node != "edge-ap-2" {
		t.Fatalf("expected node edge-ap-2, got %q", payload.Data.Lines[0].Node)
	}
}

func TestObservabilityControllerStreamLogsRejectsMissingService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewObservabilityController(&controllerProjectStore{
		projects: []*models.Project{{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"}},
	}, service.NewObservabilityService(
		nil,
		nil,
		&controllerLogStore{},
		nil,
		nil,
		nil,
		nil,
		nil,
	))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ws/logs/stream?project=prj_123", nil)
	ctx.Set("auth.claims", &service.Claims{UserID: "usr_123", Role: service.RoleOperator, AuthKind: service.AuthKindCLIPAT})

	controller.StreamLogs(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"missing_service"`) {
		t.Fatalf("expected missing_service error, got %s", rec.Body.String())
	}
}

type controllerLogStore struct {
	items []models.LogStreamEntry
}

func (s *controllerLogStore) CreateBatch(entries []models.LogStreamEntry) error {
	s.items = append(s.items, entries...)
	return nil
}

func (s *controllerLogStore) ListByQuery(query models.LogStreamQuery) ([]models.LogStreamEntry, error) {
	out := make([]models.LogStreamEntry, 0)
	for _, item := range s.items {
		if item.ProjectID != query.ProjectID || item.ServiceName != query.ServiceName {
			continue
		}
		if query.Level != "" && item.Level != query.Level {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

type controllerTraceStore struct {
	items []models.TraceSummary
}

func (s *controllerTraceStore) Create(trace *models.TraceSummary) error {
	s.items = append(s.items, *trace)
	return nil
}

func (s *controllerTraceStore) GetByCorrelationID(correlationID string) (*models.TraceSummary, error) {
	for _, item := range s.items {
		if item.CorrelationID == correlationID {
			return &item, nil
		}
	}
	return nil, nil
}

func (s *controllerTraceStore) ListByProject(projectID string, limit int) ([]models.TraceSummary, error) {
	out := make([]models.TraceSummary, 0)
	for _, item := range s.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type controllerTopologyNodeStore struct {
	items []models.TopologyNode
}

func (s *controllerTopologyNodeStore) Upsert(node *models.TopologyNode) error {
	s.items = append(s.items, *node)
	return nil
}

func (s *controllerTopologyNodeStore) ListByProject(projectID string) ([]models.TopologyNode, error) {
	out := make([]models.TopologyNode, 0)
	for _, item := range s.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *controllerTopologyNodeStore) DeleteByProject(projectID string) error {
	return nil
}

type controllerTopologyEdgeStore struct {
	items []models.TopologyEdge
}

func (s *controllerTopologyEdgeStore) Upsert(edge *models.TopologyEdge) error {
	s.items = append(s.items, *edge)
	return nil
}

func (s *controllerTopologyEdgeStore) ListByProject(projectID string) ([]models.TopologyEdge, error) {
	out := make([]models.TopologyEdge, 0)
	for _, item := range s.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *controllerTopologyEdgeStore) DeleteByProject(projectID string) error {
	return nil
}

func TestObservabilityControllerGetTraceNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewObservabilityController(&controllerProjectStore{}, service.NewObservabilityService(
		&controllerTraceStore{},
		nil,
		&controllerLogStore{},
		nil,
		nil,
		nil,
		nil,
		nil,
	))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "correlation_id", Value: "corr_missing"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/traces/corr_missing", nil)
	ctx.Set("auth.claims", &service.Claims{UserID: "usr_123", Role: service.RoleOperator, AuthKind: service.AuthKindCLIPAT})

	controller.GetTrace(ctx)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"trace_not_found"`) {
		t.Fatalf("expected trace_not_found, got %s", rec.Body.String())
	}
}

func TestObservabilityControllerGetTraceRejectsCrossProjectAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewObservabilityController(&controllerProjectStore{
		projects: []*models.Project{{ID: "prj_123", UserID: "usr_owner", Name: "Acme", Slug: "acme"}},
	}, service.NewObservabilityService(
		&controllerTraceStore{items: []models.TraceSummary{{
			ID:            "trc_123",
			CorrelationID: "corr_123",
			ProjectID:     "prj_123",
			ServiceName:   "api",
			Operation:     "GET /health",
			DurationMs:    12,
			Status:        service.TraceStatusOK,
		}}},
		nil,
		&controllerLogStore{},
		nil,
		nil,
		nil,
		nil,
		nil,
	))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "correlation_id", Value: "corr_123"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/traces/corr_123", nil)
	ctx.Set("auth.claims", &service.Claims{UserID: "usr_other", Role: service.RoleOperator, AuthKind: service.AuthKindCLIPAT})

	controller.GetTrace(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"project_access_denied"`) {
		t.Fatalf("expected project_access_denied, got %s", rec.Body.String())
	}
}

func TestObservabilityControllerGetTopologyRejectsCrossProjectAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewObservabilityController(&controllerProjectStore{
		projects: []*models.Project{{ID: "prj_123", UserID: "usr_owner", Name: "Acme", Slug: "acme"}},
	}, service.NewObservabilityService(
		nil,
		nil,
		&controllerLogStore{},
		&controllerTopologyNodeStore{items: []models.TopologyNode{{
			ID:        "tn_123",
			ProjectID: "prj_123",
			NodeKind:  service.NodeKindInstance,
			NodeRef:   "inst_123",
			Name:      "edge-1",
			Status:    "online",
		}}},
		&controllerTopologyEdgeStore{},
		nil,
		nil,
		nil,
	))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "id", Value: "prj_123"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/projects/prj_123/topology", nil)
	ctx.Set("auth.claims", &service.Claims{UserID: "usr_other", Role: service.RoleOperator, AuthKind: service.AuthKindCLIPAT})

	controller.GetTopology(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"project_access_denied"`) {
		t.Fatalf("expected project_access_denied, got %s", rec.Body.String())
	}
}

func TestObservabilityControllerStreamLogsRejectsInvalidCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewObservabilityController(&controllerProjectStore{
		projects: []*models.Project{{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"}},
	}, service.NewObservabilityService(
		nil,
		nil,
		&controllerLogStore{},
		nil,
		nil,
		nil,
		nil,
		nil,
	))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ws/logs/stream?project=prj_123&service=api&cursor=not-base64!", nil)
	ctx.Set("auth.claims", &service.Claims{UserID: "usr_123", Role: service.RoleOperator, AuthKind: service.AuthKindCLIPAT})

	controller.StreamLogs(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"invalid_logs_query"`) {
		t.Fatalf("expected invalid_logs_query, got %s", rec.Body.String())
	}
}

func TestObservabilityControllerGetTracePreservesCorrelationIDHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewObservabilityController(&controllerProjectStore{
		projects: []*models.Project{{ID: "prj_123", UserID: "usr_123", Name: "Acme", Slug: "acme"}},
	}, service.NewObservabilityService(
		&controllerTraceStore{items: []models.TraceSummary{{
			ID:            "trc_123",
			CorrelationID: "corr_trace_payload",
			ProjectID:     "prj_123",
			ServiceName:   "api",
			Operation:     "GET /health",
			DurationMs:    12,
			Status:        service.TraceStatusOK,
			MetadataJSON:  `{"x_correlation_id":"corr_trace_payload"}`,
		}}},
		nil,
		&controllerLogStore{},
		nil,
		nil,
		nil,
		nil,
		nil,
	))

	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/api/v1/traces/:correlation_id", func(c *gin.Context) {
		c.Set("auth.claims", &service.Claims{UserID: "usr_123", Role: service.RoleOperator, AuthKind: service.AuthKindCLIPAT})
		controller.GetTrace(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/traces/corr_trace_payload", nil)
	req.Header.Set("X-Correlation-ID", "corr_header_123")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		CorrelationID string `json:"correlation_id"`
		Data          struct {
			CorrelationID string `json:"correlation_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.CorrelationID != "corr_header_123" {
		t.Fatalf("expected response envelope correlation id corr_header_123, got %q", payload.CorrelationID)
	}
	if payload.Data.CorrelationID != "corr_trace_payload" {
		t.Fatalf("expected trace data correlation id corr_trace_payload, got %q", payload.Data.CorrelationID)
	}
}
