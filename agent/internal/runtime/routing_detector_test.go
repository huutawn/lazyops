package runtime

import (
	"testing"

	"lazyops-agent/internal/contracts"
)

func TestRoutingDetectorSingleService(t *testing.T) {
	ctx := RuntimeContext{
		Project: ProjectMetadata{
			ProjectID: "prj_123",
			Slug:      "my-app",
		},
		Binding: contracts.DeploymentBindingPayload{
			DomainPolicy: contracts.DomainPolicy{Provider: "sslip.io"},
		},
		Services: []ServiceRuntimeContext{
			{
				Name:   "myproject",
				Public: true,
				HealthCheck: contracts.HealthCheckPayload{Port: 8888},
			},
		},
	}

	detector := NewRoutingDetector(ctx)
	suggestion := detector.Detect()

	if suggestion == nil {
		t.Fatal("expected non-nil suggestion")
	}
	if len(suggestion.Routes) != 1 {
		t.Fatalf("expected 1 route for single service, got %d: %+v", len(suggestion.Routes), suggestion.Routes)
	}
	if suggestion.Routes[0].Path != "/" {
		t.Fatalf("expected path '/', got %q", suggestion.Routes[0].Path)
	}
	if suggestion.Confidence != 1.0 {
		t.Fatalf("expected confidence 1.0 for single service, got %f", suggestion.Confidence)
	}
}

func TestRoutingDetectorFrontendBackend(t *testing.T) {
	ctx := RuntimeContext{
		Project: ProjectMetadata{
			ProjectID: "prj_123",
			Slug:      "my-app",
		},
		Binding: contracts.DeploymentBindingPayload{
			DomainPolicy: contracts.DomainPolicy{Provider: "sslip.io"},
		},
		Services: []ServiceRuntimeContext{
			{
				Name:   "frontend",
				Public: true,
				HealthCheck: contracts.HealthCheckPayload{Port: 3000},
			},
			{
				Name:   "backend",
				Public: false,
				HealthCheck: contracts.HealthCheckPayload{Port: 8000},
			},
		},
	}

	detector := NewRoutingDetector(ctx)
	suggestion := detector.Detect()

	if suggestion == nil {
		t.Fatal("expected non-nil suggestion")
	}
	if !suggestion.FrontendDetected {
		t.Fatal("expected frontend detected")
	}
	if suggestion.APIDetected {
		t.Fatal("did not expect API detected for 'backend' service name")
	}
	if suggestion.SharedDomain == "" {
		t.Fatal("expected shared domain for FE+BE")
	}
	if len(suggestion.Routes) < 2 {
		t.Fatalf("expected at least 2 routes for FE+BE, got %d", len(suggestion.Routes))
	}
}

func TestRoutingDetectorAPI(t *testing.T) {
	ctx := RuntimeContext{
		Project: ProjectMetadata{
			ProjectID: "prj_123",
			Slug:      "my-app",
		},
		Binding: contracts.DeploymentBindingPayload{
			DomainPolicy: contracts.DomainPolicy{Provider: "sslip.io"},
		},
		Services: []ServiceRuntimeContext{
			{
				Name:   "frontend",
				Public: true,
				HealthCheck: contracts.HealthCheckPayload{Port: 3000},
			},
			{
				Name:   "api",
				Public: false,
				HealthCheck: contracts.HealthCheckPayload{Port: 8000},
			},
		},
	}

	detector := NewRoutingDetector(ctx)
	suggestion := detector.Detect()

	if !suggestion.APIDetected {
		t.Fatal("expected API detected")
	}
	// Should have /api route
	found := false
	for _, r := range suggestion.Routes {
		if r.Path == "/api" && r.Service == "api" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected /api → api route, got %+v", suggestion.Routes)
	}
}

func TestRoutingDetectorWebSocket(t *testing.T) {
	ctx := RuntimeContext{
		Project: ProjectMetadata{
			ProjectID: "prj_123",
			Slug:      "my-app",
		},
		Binding: contracts.DeploymentBindingPayload{
			DomainPolicy: contracts.DomainPolicy{Provider: "sslip.io"},
		},
		Services: []ServiceRuntimeContext{
			{
				Name:   "frontend",
				Public: true,
				HealthCheck: contracts.HealthCheckPayload{Port: 3000},
			},
			{
				Name:   "backend",
				Public: false,
				HealthCheck: contracts.HealthCheckPayload{Port: 8000},
			},
			{
				Name:   "websocket",
				Public: false,
				HealthCheck: contracts.HealthCheckPayload{Port: 8080},
			},
		},
	}

	detector := NewRoutingDetector(ctx)
	suggestion := detector.Detect()

	if !suggestion.WebSocketDetected {
		t.Fatal("expected websocket detected")
	}
	// Should have /ws route
	found := false
	for _, r := range suggestion.Routes {
		if r.Path == "/ws" && r.WebSocket {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected /ws → websocket route with WebSocket=true, got %+v", suggestion.Routes)
	}
}

func TestRoutingDetectorToRoutingPolicy(t *testing.T) {
	suggestion := &RoutingSuggestion{
		SharedDomain: "app.test.sslip.io",
		Routes: []SuggestedRoute{
			{Path: "/", Service: "frontend", Port: 3000},
			{Path: "/api", Service: "backend", Port: 8000, WebSocket: true},
		},
	}

	policy := suggestion.ToRoutingPolicy()
	if policy.SharedDomain != "app.test.sslip.io" {
		t.Fatalf("expected shared domain, got %q", policy.SharedDomain)
	}
	if len(policy.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(policy.Routes))
	}
	if !policy.Routes[1].WebSocket {
		t.Fatal("expected second route to have websocket=true")
	}
}

func TestIsFrontendService(t *testing.T) {
	tests := []struct {
		name string
		svc  ServiceRuntimeContext
		want bool
	}{
		{"frontend", ServiceRuntimeContext{Name: "frontend", HealthCheck: contracts.HealthCheckPayload{Port: 3000}}, true},
		{"fe", ServiceRuntimeContext{Name: "fe", HealthCheck: contracts.HealthCheckPayload{Port: 5173}}, true},
		{"web", ServiceRuntimeContext{Name: "web", HealthCheck: contracts.HealthCheckPayload{Port: 80}}, true},
		{"next", ServiceRuntimeContext{Name: "next-app", HealthCheck: contracts.HealthCheckPayload{Port: 3000}}, true},
		{"port 3000", ServiceRuntimeContext{Name: "myapp", HealthCheck: contracts.HealthCheckPayload{Port: 3000}}, true},
		{"port 5173", ServiceRuntimeContext{Name: "myapp", HealthCheck: contracts.HealthCheckPayload{Port: 5173}}, true},
		{"backend", ServiceRuntimeContext{Name: "backend", HealthCheck: contracts.HealthCheckPayload{Port: 8000}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFrontendService(tt.svc)
			if got != tt.want {
				t.Errorf("isFrontendService(%s) = %v, want %v", tt.svc.Name, got, tt.want)
			}
		})
	}
}

func TestIsAPIService(t *testing.T) {
	tests := []struct {
		name string
		svc  ServiceRuntimeContext
		want bool
	}{
		{"api", ServiceRuntimeContext{Name: "api"}, true},
		{"rest", ServiceRuntimeContext{Name: "rest-api"}, true},
		{"graphql", ServiceRuntimeContext{Name: "graphql-service"}, true},
		{"gateway", ServiceRuntimeContext{Name: "api-gateway"}, true},
		{"backend", ServiceRuntimeContext{Name: "backend"}, false},
		{"frontend", ServiceRuntimeContext{Name: "frontend"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAPIService(tt.svc)
			if got != tt.want {
				t.Errorf("isAPIService(%s) = %v, want %v", tt.svc.Name, got, tt.want)
			}
		})
	}
}

func TestIsWebSocketService(t *testing.T) {
	tests := []struct {
		name string
		svc  ServiceRuntimeContext
		want bool
	}{
		{"ws", ServiceRuntimeContext{Name: "ws"}, true},
		{"websocket", ServiceRuntimeContext{Name: "websocket"}, true},
		{"socket", ServiceRuntimeContext{Name: "socket-server"}, true},
		{"realtime", ServiceRuntimeContext{Name: "realtime"}, true},
		{"hub", ServiceRuntimeContext{Name: "chat-hub"}, true},
		{"backend", ServiceRuntimeContext{Name: "backend"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWebSocketService(tt.svc)
			if got != tt.want {
				t.Errorf("isWebSocketService(%s) = %v, want %v", tt.svc.Name, got, tt.want)
			}
		})
	}
}
