package runtime

import (
	"net"
	"testing"

	"lazyops-agent/internal/contracts"
)

func TestShouldSkipServiceHealthCheckSkipsAppWhenListenerMissing(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Revision: contracts.DesiredRevisionPayload{
			TriggerKind: "one_click_deploy",
			CommitSHA:   "autogen-20260412T041534Z",
		},
	}
	svc := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     65530,
			Protocol: "http",
			Path:     "/",
		},
	}

	skip, reason := shouldSkipServiceHealthCheck(runtimeCtx, svc)
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
	runtimeCtx := RuntimeContext{
		Revision: contracts.DesiredRevisionPayload{
			TriggerKind: "one_click_deploy",
			CommitSHA:   "autogen-20260412T041534Z",
		},
	}
	svc := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     port,
			Protocol: "http",
			Path:     "/",
		},
	}

	skip, reason := shouldSkipServiceHealthCheck(runtimeCtx, svc)
	if skip {
		t.Fatalf("expected app health check not to be skipped, reason=%q", reason)
	}
}

func TestShouldSkipServiceHealthCheckNeverSkipsNonAppServices(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Revision: contracts.DesiredRevisionPayload{
			TriggerKind: "one_click_deploy",
			CommitSHA:   "autogen-20260412T041534Z",
		},
	}
	svc := ServiceRuntimeContext{
		Name: "api",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     65530,
			Protocol: "http",
			Path:     "/health",
		},
	}

	skip, _ := shouldSkipServiceHealthCheck(runtimeCtx, svc)
	if skip {
		t.Fatal("expected non-app service health check not to be skipped")
	}
}

func TestShouldSkipServiceHealthCheckNeverSkipsPushDeployments(t *testing.T) {
	runtimeCtx := RuntimeContext{
		Revision: contracts.DesiredRevisionPayload{
			TriggerKind: "push",
			CommitSHA:   "68ff3d250031bd89819d73c005c957c3ee860c95",
		},
	}
	svc := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     65530,
			Protocol: "http",
			Path:     "/",
		},
	}

	skip, _ := shouldSkipServiceHealthCheck(runtimeCtx, svc)
	if skip {
		t.Fatal("expected push deployment app health check not to be skipped")
	}
}

func TestSoftenFailedAppHealthCheckPassesForOneClickAutogenWhenListenerReachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	runtimeCtx := RuntimeContext{
		Revision: contracts.DesiredRevisionPayload{
			TriggerKind: "one_click_deploy",
			CommitSHA:   "autogen-20260412T041534Z",
		},
	}
	service := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     port,
			Protocol: "http",
			Path:     "/health",
		},
	}
	initial := ServiceHealthResult{
		ServiceName: "app",
		Protocol:    "http",
		Address:     net.JoinHostPort("127.0.0.1", "0"),
		Passed:      false,
		Failures:    1,
		Message:     "http health check returned status 404",
	}

	result := softenFailedAppHealthCheck(runtimeCtx, service, initial)
	if !result.Passed {
		t.Fatal("expected app health result to be soft-passed")
	}
	if result.Failures != 0 {
		t.Fatalf("expected failures reset to 0, got %d", result.Failures)
	}
}

func TestSoftenFailedAppHealthCheckDoesNotPassForNonAutogenRevision(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	runtimeCtx := RuntimeContext{
		Revision: contracts.DesiredRevisionPayload{
			TriggerKind: "github_push",
			CommitSHA:   "9f3e2d1c",
		},
	}
	service := ServiceRuntimeContext{
		Name: "app",
		HealthCheck: contracts.HealthCheckPayload{
			Port:     port,
			Protocol: "http",
			Path:     "/health",
		},
	}
	initial := ServiceHealthResult{
		ServiceName: "app",
		Protocol:    "http",
		Passed:      false,
		Failures:    1,
		Message:     "http health check returned status 404",
	}

	result := softenFailedAppHealthCheck(runtimeCtx, service, initial)
	if result.Passed {
		t.Fatal("expected non-autogen revision not to be soft-passed")
	}
}
