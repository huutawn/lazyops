package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// SidecarProxy is a lightweight reverse proxy that listens on a local endpoint
// and forwards traffic to an upstream target. It supports both TCP and HTTP protocols.
// This is the runtime execution component for localhost_rescue sidecar mode.
type SidecarProxy struct {
	logger   *slog.Logger
	mu       sync.Mutex
	proxies  map[string]*proxyInstance
	metrics  *ProxyMetrics
	now      func() time.Time
}

type proxyInstance struct {
	route    SidecarProxyRoute
	listener net.Listener
	server   *http.Server
	cancel   context.CancelFunc
	started  time.Time
}

// ProxyMetrics tracks per-route latency and request counts.
type ProxyMetrics struct {
	mu       sync.Mutex
	counters map[string]*RouteMetrics
}

// RouteMetrics holds metrics for a single proxy route.
type RouteMetrics struct {
	Alias          string        `json:"alias"`
	TargetService  string        `json:"target_service"`
	Protocol       string        `json:"protocol"`
	RequestCount   int64         `json:"request_count"`
	ErrorCount     int64         `json:"error_count"`
	TotalLatencyMs float64       `json:"total_latency_ms"`
	LastRequestAt  time.Time     `json:"last_request_at,omitempty"`
}

// NewSidecarProxy creates a new proxy manager.
func NewSidecarProxy(logger *slog.Logger) *SidecarProxy {
	return &SidecarProxy{
		logger:  logger,
		proxies: make(map[string]*proxyInstance),
		metrics: &ProxyMetrics{
			counters: make(map[string]*RouteMetrics),
		},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// StartRoute starts a proxy for the given route. If a proxy is already running
// for this route's alias, it is stopped first.
func (p *SidecarProxy) StartRoute(ctx context.Context, route SidecarProxyRoute) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	routeKey := proxyRouteKey(route)

	// Stop existing proxy if running
	if existing, ok := p.proxies[routeKey]; ok {
		p.stopInstanceLocked(existing)
		delete(p.proxies, routeKey)
	}

	switch route.Protocol {
	case "http":
		return p.startHTTPProxy(ctx, route, routeKey)
	case "tcp":
		return p.startTCPProxy(ctx, route, routeKey)
	default:
		return fmt.Errorf("unsupported protocol %q for sidecar proxy route %s", route.Protocol, route.Alias)
	}
}

// StopRoute stops the proxy for the given route alias.
func (p *SidecarProxy) StopRoute(alias string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, instance := range p.proxies {
		if instance.route.Alias == alias {
			p.stopInstanceLocked(instance)
			delete(p.proxies, key)
			return nil
		}
	}
	return nil
}

// StopAll stops all running proxy instances.
func (p *SidecarProxy) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, instance := range p.proxies {
		p.stopInstanceLocked(instance)
		delete(p.proxies, key)
	}
}

// GetMetrics returns a snapshot of all route metrics.
func (p *SidecarProxy) GetMetrics() map[string]RouteMetrics {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	snapshot := make(map[string]RouteMetrics, len(p.metrics.counters))
	for key, m := range p.metrics.counters {
		snapshot[key] = *m
	}
	return snapshot
}

// ActiveRoutes returns the count of actively proxied routes.
func (p *SidecarProxy) ActiveRoutes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.proxies)
}

func (p *SidecarProxy) startHTTPProxy(ctx context.Context, route SidecarProxyRoute, routeKey string) error {
	upstream, err := url.Parse(route.Upstream)
	if err != nil {
		return fmt.Errorf("invalid upstream URL %q: %w", route.Upstream, err)
	}

	listenAddr := fmt.Sprintf("%s:%d", route.ListenerHost, route.ListenerPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s for route %s: %w", listenAddr, route.Alias, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)

	// WebSocket upgrade detection and explicit handling
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Log WebSocket upgrades
		if resp.Request != nil && isWebSocketUpgrade(resp.Request) {
			if p.logger != nil {
				p.logger.Info("websocket upgrade detected",
					"alias", route.Alias,
					"target_service", route.TargetService,
					"upstream", route.Upstream,
					"path", resp.Request.URL.Path,
				)
			}
		}
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		p.recordError(routeKey, route)
		if p.logger != nil {
			p.logger.Warn("sidecar proxy error",
				"alias", route.Alias,
				"target_service", route.TargetService,
				"upstream", route.Upstream,
				"error", proxyErr,
			)
		}
		http.Error(w, "sidecar proxy error", http.StatusBadGateway)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := p.now()

		// Health check endpoint for WebSocket routes
		if r.URL.Path == "/health" || r.URL.Path == "/ws/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","service":"sidecar-proxy","alias":"` + route.Alias + `"}`))
			return
		}

		// Inject correlation ID if configured
		if route.LocalhostRescue {
			if r.Header.Get("X-Correlation-ID") == "" {
				r.Header.Set("X-Correlation-ID", generateCorrelationID())
			}
			r.Header.Set("X-LazyOps-Sidecar", "localhost-rescue")
			r.Header.Set("X-LazyOps-Route", route.Alias)
		}

		proxy.ServeHTTP(w, r)

		latency := time.Since(start)
		p.recordRequest(routeKey, route, latency)
	})

	proxyCtx, cancel := context.WithCancel(ctx)
	server := &http.Server{
		Handler:           handler,
		BaseContext:       func(_ net.Listener) context.Context { return proxyCtx },
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second, // WebSocket connections can stay idle for longer
		WriteTimeout:      0,                 // No write timeout for long-lived WebSocket connections
	}

	p.proxies[routeKey] = &proxyInstance{
		route:    route,
		listener: listener,
		server:   server,
		cancel:   cancel,
		started:  p.now(),
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			if p.logger != nil {
				p.logger.Error("sidecar HTTP proxy exited with error",
					"alias", route.Alias,
					"listen", listenAddr,
					"error", err,
				)
			}
		}
	}()

	if p.logger != nil {
		p.logger.Info("sidecar HTTP proxy started",
			"alias", route.Alias,
			"target_service", route.TargetService,
			"listen", listenAddr,
			"upstream", route.Upstream,
			"forwarding_mode", route.ForwardingMode,
		)
	}

	return nil
}

func (p *SidecarProxy) startTCPProxy(ctx context.Context, route SidecarProxyRoute, routeKey string) error {
	listenAddr := fmt.Sprintf("%s:%d", route.ListenerHost, route.ListenerPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s for TCP route %s: %w", listenAddr, route.Alias, err)
	}

	proxyCtx, cancel := context.WithCancel(ctx)

	p.proxies[routeKey] = &proxyInstance{
		route:    route,
		listener: listener,
		cancel:   cancel,
		started:  p.now(),
	}

	go func() {
		for {
			select {
			case <-proxyCtx.Done():
				return
			default:
			}

			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-proxyCtx.Done():
					return
				default:
					if p.logger != nil {
						p.logger.Warn("sidecar TCP proxy accept error",
							"alias", route.Alias,
							"error", err,
						)
					}
					continue
				}
			}

			go p.handleTCPConnection(proxyCtx, conn, route, routeKey)
		}
	}()

	if p.logger != nil {
		p.logger.Info("sidecar TCP proxy started",
			"alias", route.Alias,
			"target_service", route.TargetService,
			"listen", listenAddr,
			"upstream", route.Upstream,
			"forwarding_mode", route.ForwardingMode,
		)
	}

	return nil
}

func (p *SidecarProxy) handleTCPConnection(ctx context.Context, clientConn net.Conn, route SidecarProxyRoute, routeKey string) {
	defer clientConn.Close()

	start := p.now()

	upstreamAddr := route.Upstream
	// Remove scheme prefix if present (e.g., "tcp://host:port" → "host:port")
	for _, prefix := range []string{"tcp://", "http://", "https://"} {
		upstreamAddr = removePrefix(upstreamAddr, prefix)
	}

	upstreamConn, err := net.DialTimeout("tcp", upstreamAddr, 10*time.Second)
	if err != nil {
		p.recordError(routeKey, route)
		if p.logger != nil {
			p.logger.Warn("sidecar TCP proxy upstream dial failed",
				"alias", route.Alias,
				"upstream", upstreamAddr,
				"error", err,
			)
		}
		return
	}
	defer upstreamConn.Close()

	done := make(chan struct{})

	// Client → Upstream
	go func() {
		_, _ = io.Copy(upstreamConn, clientConn)
		done <- struct{}{}
	}()

	// Upstream → Client
	go func() {
		_, _ = io.Copy(clientConn, upstreamConn)
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	latency := time.Since(start)
	p.recordRequest(routeKey, route, latency)
}

func (p *SidecarProxy) stopInstanceLocked(instance *proxyInstance) {
	if instance.cancel != nil {
		instance.cancel()
	}
	if instance.server != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = instance.server.Shutdown(shutdownCtx)
	}
	if instance.listener != nil {
		_ = instance.listener.Close()
	}
	if p.logger != nil {
		p.logger.Info("sidecar proxy stopped",
			"alias", instance.route.Alias,
			"target_service", instance.route.TargetService,
			"ran_for", time.Since(instance.started).String(),
		)
	}
}

func (p *SidecarProxy) recordRequest(routeKey string, route SidecarProxyRoute, latency time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	m := p.ensureMetricsLocked(routeKey, route)
	m.RequestCount++
	m.TotalLatencyMs += float64(latency.Milliseconds())
	m.LastRequestAt = p.now()
}

func (p *SidecarProxy) recordError(routeKey string, route SidecarProxyRoute) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	m := p.ensureMetricsLocked(routeKey, route)
	m.ErrorCount++
	m.LastRequestAt = p.now()
}

func (p *SidecarProxy) ensureMetricsLocked(routeKey string, route SidecarProxyRoute) *RouteMetrics {
	if m, ok := p.metrics.counters[routeKey]; ok {
		return m
	}
	m := &RouteMetrics{
		Alias:         route.Alias,
		TargetService: route.TargetService,
		Protocol:      route.Protocol,
	}
	p.metrics.counters[routeKey] = m
	return m
}

func proxyRouteKey(route SidecarProxyRoute) string {
	return fmt.Sprintf("%s:%s:%d", route.Alias, route.ListenerHost, route.ListenerPort)
}

func removePrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// generateCorrelationID creates a random hex-encoded correlation ID.
func generateCorrelationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func isWebSocketUpgrade(r *http.Request) bool {
	upgrade := r.Header.Get("Upgrade")
	connection := r.Header.Get("Connection")
	return upgrade != "" && (connection == "Upgrade" || connection == "upgrade")
}
