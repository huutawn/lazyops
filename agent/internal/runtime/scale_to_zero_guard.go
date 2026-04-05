package runtime

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type ScaleToZeroGuard struct {
	logger           *slog.Logger
	autosleep        *AutosleepManager
	gatewayHold      *GatewayHoldManager
	wakeTimeout      time.Duration
	coldStartTimeout time.Duration
	maxColdStarts    int
	now              func() time.Time

	mu           sync.Mutex
	wakeAttempts map[string]int
	lastWakeAt   map[string]time.Time
	coldStarts   map[string]int
}

func DefaultScaleToZeroGuardConfig() (wakeTimeout, coldStartTimeout time.Duration) {
	return 30 * time.Second, 60 * time.Second
}

func NewScaleToZeroGuard(logger *slog.Logger, autosleep *AutosleepManager, gatewayHold *GatewayHoldManager, wakeTimeout, coldStartTimeout time.Duration) *ScaleToZeroGuard {
	if wakeTimeout <= 0 {
		wakeTimeout = 30 * time.Second
	}
	if coldStartTimeout <= 0 {
		coldStartTimeout = 60 * time.Second
	}

	maxColdStarts := int(coldStartTimeout / (30 * time.Second))
	if maxColdStarts < 3 {
		maxColdStarts = 3
	}

	return &ScaleToZeroGuard{
		logger:           logger,
		autosleep:        autosleep,
		gatewayHold:      gatewayHold,
		wakeTimeout:      wakeTimeout,
		coldStartTimeout: coldStartTimeout,
		maxColdStarts:    maxColdStarts,
		now: func() time.Time {
			return time.Now().UTC()
		},
		wakeAttempts: make(map[string]int),
		lastWakeAt:   make(map[string]time.Time),
		coldStarts:   make(map[string]int),
	}
}

func (g *ScaleToZeroGuard) ValidateSleepPolicy(serviceName string, policy contracts.ScaleToZeroPolicy, runtimeMode contracts.RuntimeMode) error {
	if runtimeMode == contracts.RuntimeModeDistributedK3s {
		return fmt.Errorf("scale-to-zero is not supported in distributed-k3s mode")
	}

	if !policy.Enabled {
		return fmt.Errorf("service %s does not have scale-to-zero enabled", serviceName)
	}

	return nil
}

func (g *ScaleToZeroGuard) CanSleep(serviceName string, policy contracts.ScaleToZeroPolicy, runtimeMode contracts.RuntimeMode) bool {
	if err := g.ValidateSleepPolicy(serviceName, policy, runtimeMode); err != nil {
		return false
	}

	return g.autosleep.CanSleep(serviceName, policy)
}

func (g *ScaleToZeroGuard) SleepService(serviceName, revisionID string, runtimeMode contracts.RuntimeMode) (*ServiceSleepState, error) {
	if runtimeMode == contracts.RuntimeModeDistributedK3s {
		return nil, fmt.Errorf("scale-to-zero is not supported in distributed-k3s mode")
	}

	return g.autosleep.SleepService(serviceName, revisionID)
}

func (g *ScaleToZeroGuard) WakeService(serviceName string, runtimeMode contracts.RuntimeMode) (*ServiceSleepState, error) {
	if runtimeMode == contracts.RuntimeModeDistributedK3s {
		return nil, fmt.Errorf("scale-to-zero is not supported in distributed-k3s mode")
	}

	state, err := g.autosleep.WakeService(serviceName)
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	g.wakeAttempts[serviceName]++
	g.lastWakeAt[serviceName] = g.now()
	g.mu.Unlock()

	return state, nil
}

func (g *ScaleToZeroGuard) MarkActive(serviceName string) {
	g.autosleep.MarkActive(serviceName)

	g.mu.Lock()
	g.coldStarts[serviceName]++
	delete(g.wakeAttempts, serviceName)
	g.mu.Unlock()
}

func (g *ScaleToZeroGuard) CheckWakeTimeout(serviceName string) (bool, int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	lastWake, exists := g.lastWakeAt[serviceName]
	if !exists {
		return false, 0
	}

	if g.now().Sub(lastWake) > g.wakeTimeout {
		attempts := g.wakeAttempts[serviceName]
		return true, attempts
	}

	return false, 0
}

func (g *ScaleToZeroGuard) CheckColdStartTimeout(serviceName string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	count := g.coldStarts[serviceName]
	return count > g.maxColdStarts
}

func (g *ScaleToZeroGuard) GetWakeStats(serviceName string) (wakeAttempts, coldStarts int, lastWake time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.wakeAttempts[serviceName], g.coldStarts[serviceName], g.lastWakeAt[serviceName]
}

func (g *ScaleToZeroGuard) ResetWakeStats(serviceName string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.wakeAttempts, serviceName)
	delete(g.lastWakeAt, serviceName)
	delete(g.coldStarts, serviceName)
}
