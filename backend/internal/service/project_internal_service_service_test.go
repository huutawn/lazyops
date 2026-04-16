package service

import (
	"testing"

	"lazyops-server/internal/models"
)

func TestBuildInternalServicesDependencyBindingsClearsLocalEndpointForK3s(t *testing.T) {
	services, bindings := buildInternalServicesDependencyBindings("distributed-k3s", []LazyopsYAMLService{
		{Name: "api", Path: "apps/api"},
	}, []models.ProjectInternalService{
		{Kind: "postgres", Alias: "postgres", Protocol: "tcp", Port: 5432, LocalEndpoint: "localhost:5432"},
	})

	if len(services) != 2 {
		t.Fatalf("expected synthetic internal service to be appended, got %d services", len(services))
	}
	if len(bindings) != 1 {
		t.Fatalf("expected one dependency binding, got %d", len(bindings))
	}
	if bindings[0].TargetService != "lazyops-internal-postgres" {
		t.Fatalf("expected target service lazyops-internal-postgres, got %q", bindings[0].TargetService)
	}
	if bindings[0].LocalEndpoint != "" {
		t.Fatalf("expected distributed-k3s binding to omit local endpoint, got %q", bindings[0].LocalEndpoint)
	}
}

func TestBuildInternalServicesDependencyBindingsKeepsLocalEndpointForStandalone(t *testing.T) {
	_, bindings := buildInternalServicesDependencyBindings("standalone", []LazyopsYAMLService{
		{Name: "api", Path: "apps/api"},
	}, []models.ProjectInternalService{
		{Kind: "postgres", Alias: "postgres", Protocol: "tcp", Port: 5432, LocalEndpoint: "localhost:5432"},
	})

	if len(bindings) != 1 {
		t.Fatalf("expected one dependency binding, got %d", len(bindings))
	}
	if bindings[0].LocalEndpoint != "localhost:5432" {
		t.Fatalf("expected standalone binding to keep localhost endpoint, got %q", bindings[0].LocalEndpoint)
	}
}
