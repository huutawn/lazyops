package runtime

import (
	"net"
	"testing"

	"lazyops-agent/internal/contracts"
)

func TestShouldSkipServiceHealthCheckSkipsAppWhenListenerMissing(t *testing.T) {
	svc := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     65530,
			Protocol: "http",
			Path:     "/",
		},
	}

	skip, reason := shouldSkipServiceHealthCheck(svc)
	if !skip {
		t.Fatal("expected app health check to be skipped when listener is missing")
	}
	if reason == "" {
		t.Fatal("expected skip reason to be populated")
	}
}

func TestShouldSkipServiceHealthCheckDoesNotSkipAppWhenListenerExists(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	svc := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     port,
			Protocol: "http",
			Path:     "/",
		},
	}

	skip, reason := shouldSkipServiceHealthCheck(svc)
	if skip {
		t.Fatalf("expected app health check not to be skipped, reason=%q", reason)
	}
}

func TestShouldSkipServiceHealthCheckNeverSkipsNonAppServices(t *testing.T) {
	svc := ServiceRuntimeContext{
		Name: "api",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     65530,
			Protocol: "http",
			Path:     "/health",
		},
	}

	skip, _ := shouldSkipServiceHealthCheck(svc)
	if skip {
		t.Fatal("expected non-app service health check not to be skipped")
	}
}
