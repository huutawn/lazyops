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

type AutosleepConfig struct {
	IdleWindow     time.Duration
	WakeTimeout    time.Duration
	MaxSleepWindow time.Duration
}

func DefaultAutosleepConfig() AutosleepConfig {
	return AutosleepConfig{
		IdleWindow:     15 * time.Minute,
		WakeTimeout:    30 * time.Second,
		MaxSleepWindow: 8 * time.Hour,
	}
}

type ServiceSleepState struct {
	ServiceName  string    `json:"service_name"`
	RevisionID   string    `json:"revision_id"`
	LastActiveAt time.Time `json:"last_active_at"`
	SleepingAt   time.Time `json:"sleeping_at,omitempty"`
	WakeAt       time.Time `json:"wake_at,omitempty"`
	Status       string    `json:"status"`
}

type AutosleepManager struct {
	logger *slog.Logger
	cfg    AutosleepConfig
	now    func() time.Time

	mu     sync.Mutex
	states map[string]*ServiceSleepState
}

func NewAutosleepManager(logger *slog.Logger, cfg AutosleepConfig) *AutosleepManager {
	if cfg.IdleWindow <= 0 {
		cfg.IdleWindow = 15 * time.Minute
	}
	if cfg.WakeTimeout <= 0 {
		cfg.WakeTimeout = 30 * time.Second
	}
	if cfg.MaxSleepWindow <= 0 {
		cfg.MaxSleepWindow = 8 * time.Hour
	}

	return &AutosleepManager{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		states: make(map[string]*ServiceSleepState),
	}
}

func (m *AutosleepManager) CanSleep(serviceName string, policy contracts.ScaleToZeroPolicy) bool {
	if !policy.Enabled {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[serviceName]
	if !exists {
		return false
	}

	if state.Status == "sleeping" {
		return false
	}

	idleDuration := m.now().Sub(state.LastActiveAt)
	idleWindow := m.cfg.IdleWindow
	if policy.IdleWindow != "" {
		if parsed, err := time.ParseDuration(policy.IdleWindow); err == nil {
			idleWindow = parsed
		}
	}

	return idleDuration >= idleWindow
}

func (m *AutosleepManager) SleepService(serviceName, revisionID string) (*ServiceSleepState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()

	state, exists := m.states[serviceName]
	if !exists {
		state = &ServiceSleepState{
			ServiceName:  serviceName,
			RevisionID:   revisionID,
			LastActiveAt: now,
		}
		m.states[serviceName] = state
	}

	if state.Status == "sleeping" {
		return state, fmt.Errorf("service %s is already sleeping", serviceName)
	}

	state.SleepingAt = now
	state.Status = "sleeping"
	state.WakeAt = now.Add(m.cfg.MaxSleepWindow)

	return state, nil
}

func (m *AutosleepManager) WakeService(serviceName string) (*ServiceSleepState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s has no sleep state", serviceName)
	}

	if state.Status != "sleeping" {
		return state, fmt.Errorf("service %s is not sleeping (status: %s)", serviceName, state.Status)
	}

	now := m.now()
	if !state.WakeAt.IsZero() && now.After(state.WakeAt) {
		return nil, fmt.Errorf("service %s sleep window expired at %s", serviceName, state.WakeAt.Format(time.RFC3339))
	}

	state.Status = "waking"
	state.LastActiveAt = now
	state.SleepingAt = time.Time{}

	return state, nil
}

func (m *AutosleepManager) MarkActive(serviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[serviceName]
	if !exists {
		state = &ServiceSleepState{
			ServiceName: serviceName,
			Status:      "active",
		}
		m.states[serviceName] = state
	}

	state.LastActiveAt = m.now()
	if state.Status == "waking" {
		state.Status = "active"
	}
}

func (m *AutosleepManager) GetState(serviceName string) (*ServiceSleepState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[serviceName]
	if !exists {
		return nil, false
	}

	return state, true
}

func (m *AutosleepManager) CollectExpiredSleepWindows() []*ServiceSleepState {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	var expired []*ServiceSleepState

	for name, state := range m.states {
		if state.Status == "sleeping" && !state.WakeAt.IsZero() && now.After(state.WakeAt) {
			state.Status = "expired"
			expired = append(expired, state)
			delete(m.states, name)
		}
	}

	return expired
}

func (m *AutosleepManager) Stats() (active, sleeping, expired int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, state := range m.states {
		switch state.Status {
		case "active", "waking":
			active++
		case "sleeping":
			sleeping++
		case "expired":
			expired++
		}
	}

	return active, sleeping, expired
}

func (m *AutosleepManager) PersistSleepState(workspaceRoot, projectID, bindingID string) (string, error) {
	sleepDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "autosleep")
	if err := os.MkdirAll(sleepDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create autosleep directory: %w", err)
	}

	timestamp := m.now().Format("20060102T150405Z")
	statePath := filepath.Join(sleepDir, "state_"+timestamp+".json")

	m.mu.Lock()
	states := make([]*ServiceSleepState, 0, len(m.states))
	for _, state := range m.states {
		states = append(states, state)
	}
	m.mu.Unlock()

	raw, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal sleep states: %w", err)
	}

	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write sleep states: %w", err)
	}

	return statePath, nil
}
