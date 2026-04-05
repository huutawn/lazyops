package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/dispatcher"
)

func testAutosleepManager() *AutosleepManager {
	m := NewAutosleepManager(nil, AutosleepConfig{
		IdleWindow:     1 * time.Second,
		WakeTimeout:    1 * time.Second,
		MaxSleepWindow: 2 * time.Second,
	})
	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)
	}
	return m
}

func TestAutosleepManagerDefaultConfig(t *testing.T) {
	m := NewAutosleepManager(nil, AutosleepConfig{})
	if m.cfg.IdleWindow != 15*time.Minute {
		t.Fatalf("expected default idle window 15m, got %s", m.cfg.IdleWindow)
	}
	if m.cfg.WakeTimeout != 30*time.Second {
		t.Fatalf("expected default wake timeout 30s, got %s", m.cfg.WakeTimeout)
	}
	if m.cfg.MaxSleepWindow != 8*time.Hour {
		t.Fatalf("expected default max sleep window 8h, got %s", m.cfg.MaxSleepWindow)
	}
}

func TestAutosleepManagerCanSleepDisabled(t *testing.T) {
	m := testAutosleepManager()
	can := m.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: false})
	if can {
		t.Fatal("expected cannot sleep when policy disabled")
	}
}

func TestAutosleepManagerCanSleepNoState(t *testing.T) {
	m := testAutosleepManager()
	can := m.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true})
	if can {
		t.Fatal("expected cannot sleep when no state exists")
	}
}

func TestAutosleepManagerCanSleepAlreadySleeping(t *testing.T) {
	m := testAutosleepManager()
	m.SleepService("api", "rev_123")
	can := m.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true})
	if can {
		t.Fatal("expected cannot sleep when already sleeping")
	}
}

func TestAutosleepManagerCanSleepIdleWindowNotMet(t *testing.T) {
	m := testAutosleepManager()
	m.MarkActive("api")
	can := m.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true})
	if can {
		t.Fatal("expected cannot sleep when idle window not met")
	}
}

func TestAutosleepManagerCanSleepIdleWindowMet(t *testing.T) {
	m := testAutosleepManager()
	m.MarkActive("api")
	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 5, 0, time.UTC)
	}
	can := m.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true})
	if !can {
		t.Fatal("expected can sleep when idle window met")
	}
}

func TestAutosleepManagerCanSleepCustomIdleWindow(t *testing.T) {
	m := testAutosleepManager()
	m.MarkActive("api")
	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 3, 0, time.UTC)
	}
	can := m.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true, IdleWindow: "2s"})
	if !can {
		t.Fatal("expected can sleep with custom idle window")
	}
}

func TestAutosleepManagerSleepService(t *testing.T) {
	m := testAutosleepManager()
	state, err := m.SleepService("api", "rev_123")
	if err != nil {
		t.Fatalf("sleep service: %v", err)
	}
	if state.Status != "sleeping" {
		t.Fatalf("expected status sleeping, got %q", state.Status)
	}
	if state.ServiceName != "api" {
		t.Fatalf("expected service name api, got %q", state.ServiceName)
	}
	if state.RevisionID != "rev_123" {
		t.Fatalf("expected revision_id rev_123, got %q", state.RevisionID)
	}
	if state.SleepingAt.IsZero() {
		t.Fatal("expected sleeping_at to be set")
	}
	if state.WakeAt.IsZero() {
		t.Fatal("expected wake_at to be set")
	}
}

func TestAutosleepManagerSleepServiceAlreadySleeping(t *testing.T) {
	m := testAutosleepManager()
	m.SleepService("api", "rev_123")
	_, err := m.SleepService("api", "rev_124")
	if err == nil {
		t.Fatal("expected error when already sleeping")
	}
}

func TestAutosleepManagerWakeService(t *testing.T) {
	m := testAutosleepManager()
	m.SleepService("api", "rev_123")
	state, err := m.WakeService("api")
	if err != nil {
		t.Fatalf("wake service: %v", err)
	}
	if state.Status != "waking" {
		t.Fatalf("expected status waking, got %q", state.Status)
	}
	if state.LastActiveAt.IsZero() {
		t.Fatal("expected last_active_at to be set")
	}
	if !state.SleepingAt.IsZero() {
		t.Fatal("expected sleeping_at to be cleared")
	}
}

func TestAutosleepManagerWakeServiceNotSleeping(t *testing.T) {
	m := testAutosleepManager()
	_, err := m.WakeService("api")
	if err == nil {
		t.Fatal("expected error when service not sleeping")
	}
}

func TestAutosleepManagerWakeServiceExpired(t *testing.T) {
	m := testAutosleepManager()
	m.SleepService("api", "rev_123")
	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 5, 0, time.UTC)
	}
	_, err := m.WakeService("api")
	if err == nil {
		t.Fatal("expected error when sleep window expired")
	}
}

func TestAutosleepManagerMarkActive(t *testing.T) {
	m := testAutosleepManager()
	m.MarkActive("api")
	state, ok := m.GetState("api")
	if !ok {
		t.Fatal("expected state to exist")
	}
	if state.Status != "active" {
		t.Fatalf("expected status active, got %q", state.Status)
	}
}

func TestAutosleepManagerMarkActiveTransitionsWaking(t *testing.T) {
	m := testAutosleepManager()
	m.SleepService("api", "rev_123")
	m.WakeService("api")
	m.MarkActive("api")
	state, _ := m.GetState("api")
	if state.Status != "active" {
		t.Fatalf("expected status active after waking->active, got %q", state.Status)
	}
}

func TestAutosleepManagerGetStateNotFound(t *testing.T) {
	m := testAutosleepManager()
	_, ok := m.GetState("missing")
	if ok {
		t.Fatal("expected no state for missing service")
	}
}

func TestAutosleepManagerCollectExpiredSleepWindows(t *testing.T) {
	m := testAutosleepManager()
	m.SleepService("api", "rev_123")
	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 5, 0, time.UTC)
	}
	expired := m.CollectExpiredSleepWindows()
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired sleep window, got %d", len(expired))
	}
	if expired[0].Status != "expired" {
		t.Fatalf("expected status expired, got %q", expired[0].Status)
	}
}

func TestAutosleepManagerStats(t *testing.T) {
	m := testAutosleepManager()
	m.MarkActive("api")
	m.SleepService("web", "rev_123")
	active, sleeping, expired := m.Stats()
	if active != 1 {
		t.Fatalf("expected 1 active, got %d", active)
	}
	if sleeping != 1 {
		t.Fatalf("expected 1 sleeping, got %d", sleeping)
	}
	if expired != 0 {
		t.Fatalf("expected 0 expired, got %d", expired)
	}
}

func TestAutosleepManagerPersistSleepState(t *testing.T) {
	m := testAutosleepManager()
	m.MarkActive("api")
	m.SleepService("web", "rev_123")

	root := filepath.Join(t.TempDir(), "runtime-root")
	statePath, err := m.PersistSleepState(root, "prj_123", "bind_123")
	if err != nil {
		t.Fatalf("persist sleep state: %v", err)
	}

	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected sleep state file to exist: %v", err)
	}

	var loaded []*ServiceSleepState
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read sleep state file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode sleep state file: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 sleep states in persisted file, got %d", len(loaded))
	}
}

func TestSleepServiceHandlerReturnsDone(t *testing.T) {
	mgr := testAutosleepManager()
	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(mgr)

	payload := SleepServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := service.handleSleepService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSleepService,
		RequestID:     "req_sleep",
		CorrelationID: "corr_sleep",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected handler to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	state, ok := mgr.GetState("api")
	if !ok {
		t.Fatal("expected sleep state to exist")
	}
	if state.Status != "sleeping" {
		t.Fatalf("expected status sleeping, got %q", state.Status)
	}
}

func TestSleepServiceHandlerRejectsBadPayload(t *testing.T) {
	service := NewService(nil, nil, nil)
	result := service.handleSleepService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSleepService,
		RequestID:     "req_sleep_bad",
		CorrelationID: "corr_sleep_bad",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       json.RawMessage(`{}`),
	})
	if result.Error == nil {
		t.Fatal("expected handler to fail with empty payload")
	}
}

func TestSleepServiceHandlerRejectsMissingManager(t *testing.T) {
	service := NewService(nil, nil, nil)
	payload := SleepServicePayload{ServiceName: "api"}
	raw, _ := json.Marshal(payload)
	result := service.handleSleepService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSleepService,
		RequestID:     "req_sleep_no_mgr",
		CorrelationID: "corr_sleep_no_mgr",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected handler to fail without autosleep manager")
	}
	if result.Error.Code != "autosleep_manager_not_configured" {
		t.Fatalf("expected autosleep_manager_not_configured code, got %q", result.Error.Code)
	}
}

func TestWakeServiceHandlerReturnsDone(t *testing.T) {
	mgr := testAutosleepManager()
	mgr.SleepService("api", "rev_123")
	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(mgr)

	payload := WakeServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := service.handleWakeService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandWakeService,
		RequestID:     "req_wake",
		CorrelationID: "corr_wake",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected handler to succeed, got %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	state, ok := mgr.GetState("api")
	if !ok {
		t.Fatal("expected wake state to exist")
	}
	if state.Status != "active" {
		t.Fatalf("expected status active after wake+mark_active, got %q", state.Status)
	}
}

func TestWakeServiceHandlerRetryableWhenNotSleeping(t *testing.T) {
	mgr := testAutosleepManager()
	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(mgr)

	payload := WakeServicePayload{ServiceName: "api"}
	raw, _ := json.Marshal(payload)
	result := service.handleWakeService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandWakeService,
		RequestID:     "req_wake_not_sleeping",
		CorrelationID: "corr_wake_not_sleeping",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected handler to fail when service not sleeping")
	}
	if !result.Error.Retryable {
		t.Fatal("expected wake failure to be retryable")
	}
}

func TestSleepWakeHandlersCanBeRegistered(t *testing.T) {
	registry := dispatcher.NewDefaultRegistry()
	service := NewService(nil, nil, nil)
	service.Register(registry)

	if _, ok := registry.Resolve(contracts.CommandSleepService); !ok {
		t.Fatal("expected runtime service to register sleep_service handler")
	}
	if _, ok := registry.Resolve(contracts.CommandWakeService); !ok {
		t.Fatal("expected runtime service to register wake_service handler")
	}
}

func TestAutosleepManagerBackgroundLoopStartsAndStops(t *testing.T) {
	mgr := NewAutosleepManager(nil, AutosleepConfig{})
	mgr.StartBackgroundLoop()
	mgr.StopBackgroundLoop()
}

func TestAutosleepManagerRegisterPolicy(t *testing.T) {
	mgr := NewAutosleepManager(nil, AutosleepConfig{})
	mgr.RegisterPolicy("web", contracts.ScaleToZeroPolicy{Enabled: true, IdleWindow: "1m"})
	mgr.RegisterPolicy("api", contracts.ScaleToZeroPolicy{Enabled: false})

	mgr.mu.Lock()
	if len(mgr.policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(mgr.policies))
	}
	if !mgr.policies["web"].Enabled {
		t.Fatal("expected web policy to be enabled")
	}
	if mgr.policies["api"].Enabled {
		t.Fatal("expected api policy to be disabled")
	}
	mgr.mu.Unlock()
}

func TestAutosleepManagerBackgroundLoopAutoSleepTrigger(t *testing.T) {
	mgr := NewAutosleepManager(nil, AutosleepConfig{
		IdleWindow:     100 * time.Millisecond,
		MaxSleepWindow: 1 * time.Hour,
	})
	mgr.now = func() time.Time {
		return time.Now().UTC()
	}

	mgr.RegisterPolicy("web", contracts.ScaleToZeroPolicy{Enabled: true, IdleWindow: "100ms"})

	mgr.states["web"] = &ServiceSleepState{
		ServiceName:  "web",
		LastActiveAt: time.Now().UTC().Add(-200 * time.Millisecond),
		Status:       "active",
	}

	mgr.evaluateAndSleep()

	mgr.mu.Lock()
	state := mgr.states["web"]
	if state.Status != "sleeping" {
		t.Fatalf("expected service to be sleeping after evaluate, got %s", state.Status)
	}
	mgr.mu.Unlock()
}
