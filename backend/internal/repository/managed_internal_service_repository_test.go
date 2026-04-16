package repository

import (
	"strings"
	"testing"

	"lazyops-server/internal/models"
)

func TestManagedInternalServiceToServiceMapsDefaults(t *testing.T) {
	service := managedInternalServiceToService(models.ProjectInternalService{
		ID:            "insvc_123",
		ProjectID:     "prj_123",
		Kind:          "postgres",
		Alias:         "postgres",
		Protocol:      "tcp",
		Port:          5432,
		LocalEndpoint: "localhost:5432",
	})

	if service.Name != "lazyops-internal-postgres" {
		t.Fatalf("expected managed service name lazyops-internal-postgres, got %q", service.Name)
	}
	if service.Path != ".lazyops/internal/postgres" {
		t.Fatalf("expected internal path .lazyops/internal/postgres, got %q", service.Path)
	}
	if service.ImageRef != "postgres:16-alpine" {
		t.Fatalf("expected postgres image, got %q", service.ImageRef)
	}
	if service.ServicePort != 5432 || service.TargetPort != 5432 {
		t.Fatalf("expected service/target port 5432, got %d/%d", service.ServicePort, service.TargetPort)
	}
	if !strings.Contains(service.PVCSpecJSON, "5Gi") {
		t.Fatalf("expected pvc spec to request storage, got %q", service.PVCSpecJSON)
	}
}

func TestManagedServiceToInternalServiceKeepsLegacyShape(t *testing.T) {
	internal := managedServiceToInternalService(models.Service{
		ID:              "svc_123",
		ProjectID:       "prj_123",
		Name:            "lazyops-internal-redis",
		Path:            ".lazyops/internal/redis",
		Kind:            "redis",
		ServicePort:     6379,
		TargetPort:      6379,
		HealthcheckJSON: `{"protocol":"tcp","port":6379}`,
	})

	if internal.Kind != "redis" || internal.Alias != "redis" {
		t.Fatalf("expected redis legacy record, got %#v", internal)
	}
	if internal.Port != 6379 {
		t.Fatalf("expected legacy port 6379, got %d", internal.Port)
	}
	if internal.LocalEndpoint != "localhost:6379" {
		t.Fatalf("expected localhost endpoint, got %q", internal.LocalEndpoint)
	}
}
