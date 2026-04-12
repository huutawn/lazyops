package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestHealthCheckPortCandidatesForAppHTTP(t *testing.T) {
	service := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     8080,
			Protocol: "http",
		},
	}

	ports := healthCheckPortCandidates(service, "http")
	if len(ports) != 3 {
		t.Fatalf("expected 3 candidate ports, got %d (%v)", len(ports), ports)
	}
	if ports[0] != 8080 || ports[1] != 3000 || ports[2] != 5000 {
		t.Fatalf("unexpected candidate ports order: %v", ports)
	}
}

func TestHealthCheckPortCandidatesForNonAppService(t *testing.T) {
	service := ServiceRuntimeContext{
		Name: "api",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     8080,
			Protocol: "http",
		},
	}

	ports := healthCheckPortCandidates(service, "http")
	if len(ports) != 1 || ports[0] != 8080 {
		t.Fatalf("expected only configured port for non-app service, got %v", ports)
	}
}

func TestRunServiceHealthCheckUsesFallbackPortForApp(t *testing.T) {
	originalProbe := healthProbeOnce
	defer func() { healthProbeOnce = originalProbe }()

	calls := make([]string, 0, 4)
	healthProbeOnce = func(_ context.Context, _ string, address string, _ string, _ time.Duration) (bool, int, float64, string) {
		calls = append(calls, address)
		if strings.HasSuffix(address, ":3000") {
			return true, 200, 1, "http health check passed with status 200"
		}
		return false, 0, 1, "connection refused"
	}

	service := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:             8080,
			Protocol:         "http",
			Path:             "/",
			FailureThreshold: 1,
		},
	}

	result := runServiceHealthCheck(context.Background(), service, time.Now())
	if !result.Passed {
		t.Fatalf("expected fallback port to pass health check, got %+v", result)
	}
	if !strings.HasSuffix(result.Address, ":3000") {
		t.Fatalf("expected fallback success on port 3000, got address %q", result.Address)
	}
	if len(calls) < 2 {
		t.Fatalf("expected at least two probe attempts (primary + fallback), got %v", calls)
	}
	if !strings.HasSuffix(calls[0], ":8080") {
		t.Fatalf("expected first probe on configured port 8080, got %v", calls)
	}
	seenFallback := false
	for _, call := range calls {
		if strings.HasSuffix(call, ":3000") {
			seenFallback = true
			break
		}
	}
	if !seenFallback {
		t.Fatalf("expected fallback probe on port 3000, got %v", calls)
	}
}
