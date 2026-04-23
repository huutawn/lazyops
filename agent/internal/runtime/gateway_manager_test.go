package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lazyops-agent/internal/contracts"
)

func TestGatewayManagerResolveAutoStripPrefixRoutesPrefersNonStrippedBackendWhenProbeRequiresPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	manager := NewGatewayManager(nil, t.TempDir())
	policy := contracts.RoutingPolicyPayload{
		Routes: []contracts.RoutePayload{
			{
				Path:            "/api",
				Service:         "api",
				StripPrefix:     true,
				StripPrefixMode: contracts.RouteStripPrefixModeAuto,
			},
		},
	}
	routes := []GatewayRoute{{ServiceName: "api", Upstream: strings.TrimPrefix(server.URL, "http://")}}
	services := []ServiceRuntimeContext{
		{
			Name: "api",
			HealthCheck: contracts.HealthCheckPayload{
				Protocol: "http",
				Path:     "/api/health",
			},
		},
	}

	resolved := manager.resolveAutoStripPrefixRoutes(context.Background(), policy, routes, services)
	if len(resolved.Routes) != 1 {
		t.Fatalf("expected one route, got %#v", resolved.Routes)
	}
	if resolved.Routes[0].StripPrefix {
		t.Fatalf("expected auto strip-prefix probe to disable stripping, got %#v", resolved.Routes[0])
	}
}

func TestGatewayManagerResolveAutoStripPrefixRoutesKeepsStrippedBackendWhenProbeMatchesInternalPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	manager := NewGatewayManager(nil, t.TempDir())
	policy := contracts.RoutingPolicyPayload{
		Routes: []contracts.RoutePayload{
			{
				Path:            "/api",
				Service:         "api",
				StripPrefix:     true,
				StripPrefixMode: contracts.RouteStripPrefixModeAuto,
			},
		},
	}
	routes := []GatewayRoute{{ServiceName: "api", Upstream: strings.TrimPrefix(server.URL, "http://")}}
	services := []ServiceRuntimeContext{
		{
			Name: "api",
			HealthCheck: contracts.HealthCheckPayload{
				Protocol: "http",
				Path:     "/health",
			},
		},
	}

	resolved := manager.resolveAutoStripPrefixRoutes(context.Background(), policy, routes, services)
	if len(resolved.Routes) != 1 {
		t.Fatalf("expected one route, got %#v", resolved.Routes)
	}
	if !resolved.Routes[0].StripPrefix {
		t.Fatalf("expected auto strip-prefix probe to keep stripping, got %#v", resolved.Routes[0])
	}
}
