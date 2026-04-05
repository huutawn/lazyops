package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type GatewayHoldConfig struct {
	DefaultHoldTimeout time.Duration
	MaxHeldRequests    int
}

func DefaultGatewayHoldConfig() GatewayHoldConfig {
	return GatewayHoldConfig{
		DefaultHoldTimeout: 30 * time.Second,
		MaxHeldRequests:    100,
	}
}

type HeldRequest struct {
	RequestID     string    `json:"request_id"`
	ServiceName   string    `json:"service_name"`
	CorrelationID string    `json:"correlation_id"`
	HeldAt        time.Time `json:"held_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Status        string    `json:"status"`
}

type GatewayHoldManager struct {
	logger *slog.Logger
	cfg    GatewayHoldConfig
	now    func() time.Time

	mu      sync.Mutex
	holds   map[string][]*HeldRequest
	total   int
	expired int
	resumed int

	stopCh chan struct{}
	doneCh chan struct{}
}

func NewGatewayHoldManager(logger *slog.Logger, cfg GatewayHoldConfig) *GatewayHoldManager {
	if cfg.DefaultHoldTimeout <= 0 {
		cfg.DefaultHoldTimeout = 30 * time.Second
	}
	if cfg.MaxHeldRequests <= 0 {
		cfg.MaxHeldRequests = 100
	}

	return &GatewayHoldManager{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		holds: make(map[string][]*HeldRequest),
	}
}

func (m *GatewayHoldManager) HoldRequest(serviceName, requestID, correlationID string, holdTimeout time.Duration) (*HeldRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.total++

	if holdTimeout <= 0 {
		holdTimeout = m.cfg.DefaultHoldTimeout
	}

	existing := m.holds[serviceName]
	if len(existing) >= m.cfg.MaxHeldRequests {
		return nil, fmt.Errorf("max held requests reached for service %s (%d)", serviceName, m.cfg.MaxHeldRequests)
	}

	now := m.now()
	req := &HeldRequest{
		RequestID:     requestID,
		ServiceName:   serviceName,
		CorrelationID: correlationID,
		HeldAt:        now,
		ExpiresAt:     now.Add(holdTimeout),
		Status:        "held",
	}

	m.holds[serviceName] = append(m.holds[serviceName], req)
	return req, nil
}

func (m *GatewayHoldManager) ResumeRequests(serviceName string) []*HeldRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	requests := m.holds[serviceName]
	if len(requests) == 0 {
		return nil
	}

	now := m.now()
	var resumable []*HeldRequest

	for _, req := range requests {
		if now.Before(req.ExpiresAt) {
			req.Status = "resumed"
			resumable = append(resumable, req)
			m.resumed++
		} else {
			req.Status = "expired"
			m.expired++
		}
	}

	delete(m.holds, serviceName)
	return resumable
}

func (m *GatewayHoldManager) CollectExpiredHolds() []*HeldRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	var expired []*HeldRequest

	for serviceName, requests := range m.holds {
		var remaining []*HeldRequest
		for _, req := range requests {
			if now.After(req.ExpiresAt) {
				req.Status = "expired"
				expired = append(expired, req)
				m.expired++
			} else {
				remaining = append(remaining, req)
			}
		}
		if len(remaining) == 0 {
			delete(m.holds, serviceName)
		} else {
			m.holds[serviceName] = remaining
		}
	}

	return expired
}

func (m *GatewayHoldManager) Stats() (total, active, expired, resumed int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	activeCount := 0
	for _, requests := range m.holds {
		activeCount += len(requests)
	}

	return m.total, activeCount, m.expired, m.resumed
}

func (m *GatewayHoldManager) PersistHoldState(workspaceRoot, projectID, bindingID string) (string, error) {
	holdDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "gateway-holds")
	if err := os.MkdirAll(holdDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create gateway hold directory: %w", err)
	}

	timestamp := m.now().Format("20060102T150405Z")
	holdPath := filepath.Join(holdDir, "state_"+timestamp+".json")

	m.mu.Lock()
	allHolds := make(map[string][]*HeldRequest)
	for serviceName, requests := range m.holds {
		allHolds[serviceName] = requests
	}
	m.mu.Unlock()

	raw, err := json.MarshalIndent(allHolds, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal hold states: %w", err)
	}

	if err := os.WriteFile(holdPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write hold states: %w", err)
	}

	return holdPath, nil
}

func (m *GatewayHoldManager) StartBackgroundLoop() {
	m.stopCh = make(chan struct{})
	m.doneCh = make(chan struct{})
	go m.backgroundLoop()
}

func (m *GatewayHoldManager) StopBackgroundLoop() {
	if m.stopCh == nil {
		return
	}
	close(m.stopCh)
	<-m.doneCh
}

func (m *GatewayHoldManager) backgroundLoop() {
	defer close(m.doneCh)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			if m.logger != nil {
				m.logger.Info("gateway hold manager background loop stopped")
			}
			return
		case <-ticker.C:
			expired := m.CollectExpiredHolds()
			for _, req := range expired {
				if m.logger != nil {
					m.logger.Warn("gateway hold expired",
						"service", req.ServiceName,
						"request_id", req.RequestID,
						"held_at", req.HeldAt.Format(time.RFC3339),
					)
				}
			}
		}
	}
}

func (m *GatewayHoldManager) HoldTimeoutFromPolicy(policy contracts.ScaleToZeroPolicy) time.Duration {
	if policy.GatewayHoldTimeout != "" {
		if d, err := time.ParseDuration(policy.GatewayHoldTimeout); err == nil && d > 0 {
			return d
		}
	}
	return m.cfg.DefaultHoldTimeout
}
