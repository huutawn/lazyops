package runtime

import (
	"fmt"
	"strings"

	"lazyops-agent/internal/contracts"
)

// RoutingSuggestion holds a suggested routing configuration
// based on automatic detection of service patterns.
type RoutingSuggestion struct {
	SharedDomain       string           `json:"shared_domain"`
	Routes             []SuggestedRoute `json:"routes"`
	Confidence         float64          `json:"confidence"`  // 0.0-1.0
	Reason             string           `json:"reason"`
	WebSocketDetected  bool             `json:"websocket_detected"`
	APIDetected        bool             `json:"api_detected"`
	FrontendDetected   bool             `json:"frontend_detected"`
}

// SuggestedRoute is a suggested route for the Caddy config
type SuggestedRoute struct {
	Path        string `json:"path"`
	Service     string `json:"service"`
	Port        int    `json:"port"`
	WebSocket   bool   `json:"websocket"`
	StripPrefix bool   `json:"strip_prefix"`
	Confidence  float64 `json:"confidence"`
	Reason      string `json:"reason"`
}

// RoutingDetector analyzes services and suggests routing configuration
// based on service name conventions, port patterns, and labels.
type RoutingDetector struct {
	runtimeCtx RuntimeContext
}

// NewRoutingDetector creates a new routing detector
func NewRoutingDetector(runtimeCtx RuntimeContext) *RoutingDetector {
	return &RoutingDetector{runtimeCtx: runtimeCtx}
}

// Detect analyzes services and returns routing suggestions
func (d *RoutingDetector) Detect() *RoutingSuggestion {
	suggestion := &RoutingSuggestion{
		Routes: []SuggestedRoute{},
	}

	// Detect service roles
	hasFrontend := false
	hasBackend := false
	hasAPI := false
	hasWebSocket := false
	frontendService := ""
	backendService := ""
	apiService := ""
	webSocketService := ""
	frontendPort := 0
	backendPort := 0
	apiPort := 0
	webSocketPort := 0

	for _, svc := range d.runtimeCtx.Services {
		// Detect frontend
		if isFrontendService(svc) {
			hasFrontend = true
			frontendService = svc.Name
			frontendPort = svc.HealthCheck.Port
			suggestion.FrontendDetected = true
		}

		// Detect backend/API
		if isBackendService(svc) {
			hasBackend = true
			backendService = svc.Name
			backendPort = svc.HealthCheck.Port
		}

		if isAPIService(svc) {
			hasAPI = true
			apiService = svc.Name
			apiPort = svc.HealthCheck.Port
			suggestion.APIDetected = true
		}

		// Detect WebSocket
		if isWebSocketService(svc) {
			hasWebSocket = true
			webSocketService = svc.Name
			webSocketPort = svc.HealthCheck.Port
			suggestion.WebSocketDetected = true
		}
	}

	// Build routing suggestions based on detected patterns
	if hasFrontend && (hasBackend || hasAPI) {
		// Classic FE + BE architecture
		suggestion.SharedDomain = fmt.Sprintf("app.%s.%s",
			d.runtimeCtx.Project.Slug,
			resolveMagicDomain(d.runtimeCtx.Binding.DomainPolicy.Provider))

		// WebSocket route (first, if detected)
		if hasWebSocket {
			suggestion.Routes = append(suggestion.Routes, SuggestedRoute{
				Path:       "/ws",
				Service:    webSocketService,
				Port:       webSocketPort,
				WebSocket:  true,
				Confidence: 0.8,
				Reason:     fmt.Sprintf("Service %q detected as WebSocket service by name convention", webSocketService),
			})
		}

		// API route
		if hasAPI {
			suggestion.Routes = append(suggestion.Routes, SuggestedRoute{
				Path:        "/api",
				Service:     apiService,
				Port:        apiPort,
				WebSocket:   false,
				StripPrefix: false,
				Confidence:  0.9,
				Reason:      fmt.Sprintf("Service %q detected as API service, routing /api to it", apiService),
			})
		} else if hasBackend {
			suggestion.Routes = append(suggestion.Routes, SuggestedRoute{
				Path:        "/api",
				Service:     backendService,
				Port:        backendPort,
				WebSocket:   false,
				StripPrefix: false,
				Confidence:  0.7,
				Reason:      fmt.Sprintf("Service %q detected as backend service, routing /api to it", backendService),
			})
		}

		// Frontend as catch-all
		if hasFrontend {
			suggestion.Routes = append(suggestion.Routes, SuggestedRoute{
				Path:        "/",
				Service:     frontendService,
				Port:        frontendPort,
				WebSocket:   false,
				StripPrefix: false,
				Confidence:  0.9,
				Reason:      fmt.Sprintf("Service %q detected as frontend, serving as root", frontendService),
			})
		}

		suggestion.Confidence = 0.85
		suggestion.Reason = "Detected classic FE+BE architecture with path-based routing"
	} else if len(d.runtimeCtx.Services) == 1 {
		// Single service
		svc := d.runtimeCtx.Services[0]
		suggestion.SharedDomain = fmt.Sprintf("%s.%s.%s",
			svc.Name,
			d.runtimeCtx.Project.Slug,
			resolveMagicDomain(d.runtimeCtx.Binding.DomainPolicy.Provider))
		suggestion.Routes = []SuggestedRoute{
			{
				Path:       "/",
				Service:    svc.Name,
				Port:       svc.HealthCheck.Port,
				Confidence: 1.0,
				Reason:     fmt.Sprintf("Single service %q, all traffic goes to it", svc.Name),
			},
		}
		suggestion.Confidence = 1.0
		suggestion.Reason = fmt.Sprintf("Single service deployment: %s", svc.Name)
	} else {
		// Multiple services, no clear FE+BE pattern
		// Suggest per-service routing based on conventions
		for _, svc := range d.runtimeCtx.Services {
			if !svc.Public {
				continue
			}

			lower := strings.ToLower(svc.Name)
			suggestedPath := "/"
			isWS := false

			if strings.Contains(lower, "api") || strings.Contains(lower, "server") {
				suggestedPath = "/api"
			}
			if strings.Contains(lower, "ws") || strings.Contains(lower, "socket") || strings.Contains(lower, "realtime") {
				suggestedPath = "/ws"
				isWS = true
			}
			if strings.Contains(lower, "admin") || strings.Contains(lower, "dashboard") {
				suggestedPath = "/admin"
			}

			suggestion.Routes = append(suggestion.Routes, SuggestedRoute{
				Path:       suggestedPath,
				Service:    svc.Name,
				Port:       svc.HealthCheck.Port,
				WebSocket:  isWS,
				Confidence: 0.5,
				Reason:     fmt.Sprintf("Convention-based detection for service %q", svc.Name),
			})
		}
		suggestion.Confidence = 0.5
		suggestion.Reason = "Multiple services detected, suggesting routes based on name conventions"
	}

	return suggestion
}

// ToRoutingPolicy converts the suggestion to an actual RoutingPolicyPayload
func (s *RoutingSuggestion) ToRoutingPolicy() contracts.RoutingPolicyPayload {
	policy := contracts.RoutingPolicyPayload{
		SharedDomain: s.SharedDomain,
	}

	for _, route := range s.Routes {
		policy.Routes = append(policy.Routes, contracts.RoutePayload{
			Path:        route.Path,
			Service:     route.Service,
			Port:        route.Port,
			WebSocket:   route.WebSocket,
			StripPrefix: route.StripPrefix,
		})
	}

	return policy
}

// Service role detection helpers

func isFrontendService(svc ServiceRuntimeContext) bool {
	lower := strings.ToLower(svc.Name)

	// Name-based detection
	frontendNames := []string{"frontend", "fe", "web", "ui", "app", "next", "nuxt", "react", "vue", "angular", "svelte"}
	for _, name := range frontendNames {
		if strings.Contains(lower, name) {
			return true
		}
	}

	// Port-based detection (common frontend ports)
	frontendPorts := []int{3000, 3001, 5173, 4200, 8080, 80, 443}
	for _, port := range frontendPorts {
		if svc.HealthCheck.Port == port {
			return true
		}
	}

	// Label-based detection
	if labels := svc.Labels; labels != nil {
		if role, ok := labels["lazyops.role"]; ok {
			if strings.ToLower(role) == "frontend" || strings.ToLower(role) == "web" {
				return true
			}
		}
	}

	return false
}

func isBackendService(svc ServiceRuntimeContext) bool {
	lower := strings.ToLower(svc.Name)

	backendNames := []string{"backend", "be", "server", "app", "service", "api-server"}
	for _, name := range backendNames {
		if strings.Contains(lower, name) {
			return true
		}
	}

	backendPorts := []int{8000, 8001, 8080, 9000, 5000, 3001, 4000}
	for _, port := range backendPorts {
		if svc.HealthCheck.Port == port {
			return true
		}
	}

	if labels := svc.Labels; labels != nil {
		if role, ok := labels["lazyops.role"]; ok {
			if strings.ToLower(role) == "backend" || strings.ToLower(role) == "server" {
				return true
			}
		}
	}

	return false
}

func isAPIService(svc ServiceRuntimeContext) bool {
	lower := strings.ToLower(svc.Name)

	apiNames := []string{"api", "rest", "graphql", "grpc", "gateway"}
	for _, name := range apiNames {
		if strings.Contains(lower, name) {
			return true
		}
	}

	return false
}

func isWebSocketService(svc ServiceRuntimeContext) bool {
	lower := strings.ToLower(svc.Name)

	wsNames := []string{"ws", "websocket", "socket", "realtime", "live", "streaming", "hub", "chat"}
	for _, name := range wsNames {
		if strings.Contains(lower, name) {
			return true
		}
	}

	if labels := svc.Labels; labels != nil {
		if role, ok := labels["lazyops.role"]; ok {
			if strings.ToLower(role) == "websocket" || strings.ToLower(role) == "realtime" {
				return true
			}
		}
	}

	return false
}

func resolveMagicDomain(provider string) string {
	if provider == "" {
		return "sslip.io"
	}
	switch strings.ToLower(provider) {
	case "sslip.io":
		return "sslip.io"
	case "nip.io":
		return "nip.io"
	default:
		return "sslip.io"
	}
}
