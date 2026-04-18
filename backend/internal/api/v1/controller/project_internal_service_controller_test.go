package controller

import (
	"testing"
	"time"

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
	})

	if len(result.Items) != 1 {
		t.Fatalf("expected one internal service, got %#v", result.Items)
	}
	if result.Items[0].Kind != "postgres" || result.Items[0].Alias != "postgres" {
		t.Fatalf("expected postgres internal service alias, got %#v", result.Items[0])
	}
	if result.Items[0].LocalEndpoint != "localhost:5432" {
		t.Fatalf("expected compatibility localhost endpoint, got %#v", result.Items[0])
	}
}
