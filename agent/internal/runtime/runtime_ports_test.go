package runtime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestAssignRuntimePortsKeepsDeclaredPortWhenAvailable(t *testing.T) {
	driver := NewFilesystemDriver(nil, t.TempDir())
	declaredPort := freeTCPPort(t)

	runtimeCtx := withRuntimeServices(baseStandaloneRuntimeContext(), []ServiceRuntimeContext{
		{
			Name:        "app",
			Public:      true,
			HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: declaredPort},
		},
	})

	updated, err := driver.assignRuntimePorts(runtimeCtx)
	if err != nil {
		t.Fatalf("assign runtime ports: %v", err)
	}
	if got := updated.Services[0].RuntimePort; got != declaredPort {
		t.Fatalf("expected runtime port %d, got %d", declaredPort, got)
	}
}

func TestAssignRuntimePortsAllocatesAlternatePortForSecondService(t *testing.T) {
	driver := NewFilesystemDriver(nil, t.TempDir())
	declaredPort := freeTCPPort(t)

	runtimeCtx := withRuntimeServices(baseStandaloneRuntimeContext(), []ServiceRuntimeContext{
		{
			Name:        "app",
			Public:      true,
			HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: declaredPort},
		},
		{
			Name:        "web",
			Public:      true,
			HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: declaredPort},
		},
	})

	updated, err := driver.assignRuntimePorts(runtimeCtx)
	if err != nil {
		t.Fatalf("assign runtime ports: %v", err)
	}
	if got := updated.Services[0].RuntimePort; got != declaredPort {
		t.Fatalf("expected first service runtime port %d, got %d", declaredPort, got)
	}
	if got := updated.Services[1].RuntimePort; got <= 0 || got == declaredPort {
		t.Fatalf("expected alternate runtime port for second service, got %d", got)
	}
}

func TestAssignRuntimePortsAvoidsDependencyLocalListenerCollision(t *testing.T) {
	driver := NewFilesystemDriver(nil, t.TempDir())
	declaredPort := freeTCPPort(t)

	runtimeCtx := withRuntimeServices(baseStandaloneRuntimeContext(), []ServiceRuntimeContext{
		{
			Name:   "app",
			Public: true,
			HealthCheck: contracts.HealthCheckPayload{
				Protocol: "http",
				Path:     "/",
				Port:     declaredPort,
			},
			Dependencies: []contracts.DependencyBindingPayload{
				{
					Alias:         "api",
					TargetService: "api",
					Protocol:      "http",
					LocalEndpoint: fmt.Sprintf("http://localhost:%d", declaredPort),
				},
			},
		},
		{
			Name:        "api",
			Public:      false,
			RuntimePort: freeTCPPort(t),
			HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: 8080},
		},
	})

	updated, err := driver.assignRuntimePorts(runtimeCtx)
	if err != nil {
		t.Fatalf("assign runtime ports: %v", err)
	}
	if got := updated.Services[0].RuntimePort; got == declaredPort || got <= 0 {
		t.Fatalf("expected alternate runtime port when dependency listener uses %d, got %d", declaredPort, got)
	}
}

func TestHydrateRuntimeContextFromWorkspaceUsesPersistedRuntimePort(t *testing.T) {
	driver := NewFilesystemDriver(nil, t.TempDir())
	runtimeCtx := withRuntimeServices(baseStandaloneRuntimeContext(), []ServiceRuntimeContext{
		{
			Name:        "app",
			Public:      true,
			HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: 8080},
		},
	})

	layout := workspaceLayout(driver.root, runtimeCtx)
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("mkdir workspace root: %v", err)
	}
	if err := writeJSON(layout.Root+"/workspace.json", WorkspaceManifest{
		Revision: runtimeCtx.Revision,
		Services: []ServiceRuntimeContext{
			{
				Name:        "app",
				Public:      true,
				RuntimePort: 31234,
				HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: 8080},
			},
		},
	}); err != nil {
		t.Fatalf("write workspace manifest: %v", err)
	}

	hydrated := driver.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)
	if got := hydrated.Services[0].RuntimePort; got != 31234 {
		t.Fatalf("expected hydrated runtime port 31234, got %d", got)
	}
}

func TestResolvePublicServiceUsesRuntimePort(t *testing.T) {
	runtimeCtx := withRuntimeServices(baseStandaloneRuntimeContext(), []ServiceRuntimeContext{
		{
			Name:        "app",
			Public:      true,
			RuntimePort: 32123,
			HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: 8080},
			Placement: &contracts.PlacementAssignment{
				ServiceName: "app",
				TargetID:    "inst_123",
				TargetKind:  contracts.TargetKindInstance,
			},
		},
	})

	resolver, err := newRuntimeDependencyResolver("", runtimeCtx)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	route := resolver.ResolvePublicService(runtimeCtx.Services[0])
	if route.Port != 32123 {
		t.Fatalf("expected route port 32123, got %d", route.Port)
	}
	if route.Upstream != "127.0.0.1:32123" {
		t.Fatalf("expected upstream 127.0.0.1:32123, got %q", route.Upstream)
	}
}

func TestResolveDependencyUsesServiceAliasUpstreamForStandalone(t *testing.T) {
	runtimeCtx := withRuntimeServices(baseStandaloneRuntimeContext(), []ServiceRuntimeContext{
		{
			Name: "web",
			Dependencies: []contracts.DependencyBindingPayload{
				{
					Alias:         "api",
					TargetService: "api",
					Protocol:      "http",
					LocalEndpoint: "http://localhost:8080",
				},
			},
		},
		{
			Name:        "api",
			RuntimePort: 32123,
			HealthCheck: contracts.HealthCheckPayload{Protocol: "http", Path: "/", Port: 8080},
		},
	})

	resolver, err := newRuntimeDependencyResolver("", runtimeCtx)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	resolved := resolver.ResolveDependency(runtimeCtx.Services[0], runtimeCtx.Services[0].Dependencies[0])
	if resolved.ResolvedUpstream != "http://api:32123" {
		t.Fatalf("expected standalone upstream to use bridge alias, got %q", resolved.ResolvedUpstream)
	}
	if resolved.ResolvedEndpoint != "http://api:32123" {
		t.Fatalf("expected standalone endpoint to use bridge alias, got %q", resolved.ResolvedEndpoint)
	}
}

func TestRunServiceHealthCheckUsesRuntimePort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host, portStr, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	if host == "" || portStr == "" {
		t.Fatalf("invalid server address %q", server.Listener.Addr().String())
	}
	var runtimePort int
	if _, err := fmt.Sscanf(portStr, "%d", &runtimePort); err != nil {
		t.Fatalf("parse runtime port: %v", err)
	}

	service := ServiceRuntimeContext{
		Name:        "app",
		RuntimePort: runtimePort,
		HealthCheck: contracts.HealthCheckPayload{
			Protocol: "http",
			Path:     "/",
			Port:     8080,
		},
	}

	result := runServiceHealthCheck(context.Background(), service, time.Now().UTC())
	if !result.Passed {
		t.Fatalf("expected health check to pass, got message: %s", result.Message)
	}
	if result.Address != net.JoinHostPort("127.0.0.1", portStr) {
		t.Fatalf("expected address to use runtime port %d, got %q", runtimePort, result.Address)
	}
}

func TestRunServiceHealthCheckFailureMentionsDeclaredAndRuntimePorts(t *testing.T) {
	runtimePort := freeTCPPort(t)
	service := ServiceRuntimeContext{
		Name:        "app",
		RuntimePort: runtimePort,
		HealthCheck: contracts.HealthCheckPayload{
			Protocol: "http",
			Path:     "/",
			Port:     8080,
		},
	}

	result := runServiceHealthCheck(context.Background(), service, time.Now().UTC())
	if result.Passed {
		t.Fatalf("expected failing health check when no listener exists, got %#v", result)
	}

	expected := fmt.Sprintf("declared healthcheck port %d differs from runtime port %d", 8080, runtimePort)
	if !strings.Contains(result.Message, expected) {
		t.Fatalf("expected failure message to contain %q, got %q", expected, result.Message)
	}
}

func baseStandaloneRuntimeContext() RuntimeContext {
	return RuntimeContext{
		Project: ProjectMetadata{
			ProjectID: "prj_123",
			Slug:      "lazy-app",
		},
		Binding: contracts.DeploymentBindingPayload{
			BindingID:   "bind_123",
			ProjectID:   "prj_123",
			RuntimeMode: contracts.RuntimeModeStandalone,
			TargetKind:  contracts.TargetKindInstance,
			TargetID:    "inst_123",
			TargetRef:   "auto-primary",
		},
		Revision: contracts.DesiredRevisionPayload{
			RevisionID:  "rev_123",
			ProjectID:   "prj_123",
			RuntimeMode: contracts.RuntimeModeStandalone,
		},
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ephemeral port: %v", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address type %T", listener.Addr())
	}
	return addr.Port
}
