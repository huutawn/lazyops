package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/service"
)

func TestFilterUnifiedInternalServicesReturnsInternalSubset(t *testing.T) {
	now := time.Date(2026, 4, 18, 6, 0, 0, 0, time.UTC)
	result := filterUnifiedInternalServices(&service.ProjectServiceListResult{
		Items: []service.ProjectServiceRecord{
			{
				ID:            "svc_api",
				ProjectID:     "prj_1",
				Name:          "api",
				Kind:          "backend",
				SourceType:    "repo",
				TargetPort:    8080,
				ServicePort:   8080,
				PlacementMode: "shared_cluster",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			{
				ID:            "svc_postgres",
				ProjectID:     "prj_1",
				Name:          "lazyops-internal-postgres",
				Path:          ".lazyops/internal/postgres",
				Kind:          "postgres",
				SourceType:    "internal",
				TargetPort:    5432,
				ServicePort:   5432,
				PlacementMode: "shared_cluster",
				Healthcheck: map[string]any{
					"protocol": "tcp",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}, "distributed-k3s")

	if len(result.Items) != 1 {
		t.Fatalf("expected one internal service, got %#v", result.Items)
	}
	if result.Items[0].Kind != "postgres" || result.Items[0].Alias != "postgres" {
		t.Fatalf("expected postgres internal service alias, got %#v", result.Items[0])
	}
	if result.Items[0].LocalEndpoint != "lazyops-internal-postgres:5432" {
		t.Fatalf("expected distributed-k3s compatibility endpoint to use service dns, got %#v", result.Items[0])
	}
}

func TestFilterUnifiedInternalServicesKeepsLocalhostForStandalone(t *testing.T) {
	now := time.Date(2026, 4, 18, 6, 0, 0, 0, time.UTC)
	result := filterUnifiedInternalServices(&service.ProjectServiceListResult{
		Items: []service.ProjectServiceRecord{
			{
				ID:          "svc_postgres",
				ProjectID:   "prj_1",
				Name:        "db",
				Kind:        "postgres",
				SourceType:  "internal",
				TargetPort:  5432,
				ServicePort: 5432,
				Healthcheck: map[string]any{
					"protocol": "tcp",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}, "standalone")

	if len(result.Items) != 1 {
		t.Fatalf("expected one internal service, got %#v", result.Items)
	}
	if result.Items[0].LocalEndpoint != "localhost:5432" {
		t.Fatalf("expected standalone compatibility endpoint to keep localhost, got %#v", result.Items[0])
	}
}

func TestProjectInternalServiceControllerConfigureReturnsServiceFirstConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPut, "/projects/prj_123/internal-services", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	ctx.Request = request

	controller := NewProjectInternalServiceController(nil, nil)
	controller.Configure(ctx)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d", recorder.Code)
	}
	if !responseBodyContains(recorder.Body.String(), "service_first_required") {
		t.Fatalf("expected compatibility error code in body, got %s", recorder.Body.String())
	}
}

func responseBodyContains(body, needle string) bool {
	return strings.Contains(body, needle)
}
