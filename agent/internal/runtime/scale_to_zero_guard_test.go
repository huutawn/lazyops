package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/dispatcher"
)

func testScaleToZeroGuard() *ScaleToZeroGuard {
	autosleep := testAutosleepManager()
	gatewayHold := testGatewayHoldManager()
	wakeTimeout, coldStartTimeout := DefaultScaleToZeroGuardConfig()
	g := NewScaleToZeroGuard(nil, autosleep, gatewayHold, wakeTimeout, coldStartTimeout)
	g.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)
	}
	return g
}

func TestScaleToZeroGuardValidateSleepPolicyK3sRejected(t *testing.T) {
	g := testScaleToZeroGuard()
	err := g.ValidateSleepPolicy("api", contracts.ScaleToZeroPolicy{Enabled: true}, contracts.RuntimeModeDistributedK3s)
	if err == nil {
		t.Fatal("expected error for k3s mode")
	}
}

func TestScaleToZeroGuardValidateSleepPolicyDisabled(t *testing.T) {
	g := testScaleToZeroGuard()
	err := g.ValidateSleepPolicy("api", contracts.ScaleToZeroPolicy{Enabled: false}, contracts.RuntimeModeStandalone)
	if err == nil {
		t.Fatal("expected error for disabled policy")
	}
}

func TestScaleToZeroGuardValidateSleepPolicyStandaloneEnabled(t *testing.T) {
	g := testScaleToZeroGuard()
	err := g.ValidateSleepPolicy("api", contracts.ScaleToZeroPolicy{Enabled: true}, contracts.RuntimeModeStandalone)
	if err != nil {
		t.Fatalf("expected no error for standalone with enabled policy, got %v", err)
	}
}

func TestScaleToZeroGuardCanSleep(t *testing.T) {
	g := testScaleToZeroGuard()
	g.autosleep.MarkActive("api")
	g.autosleep.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 5, 0, time.UTC)
	}
	can := g.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true, IdleWindow: "2s"}, contracts.RuntimeModeStandalone)
	if !can {
		t.Fatal("expected can sleep when policy enabled and idle window met")
	}
}

func TestScaleToZeroGuardCanSleepK3sRejected(t *testing.T) {
	g := testScaleToZeroGuard()
	g.autosleep.MarkActive("api")
	can := g.CanSleep("api", contracts.ScaleToZeroPolicy{Enabled: true}, contracts.RuntimeModeDistributedK3s)
	if can {
		t.Fatal("expected cannot sleep for k3s mode")
	}
}

func TestScaleToZeroGuardSleepServiceK3sRejected(t *testing.T) {
	g := testScaleToZeroGuard()
	_, err := g.SleepService("api", "rev_123", contracts.RuntimeModeDistributedK3s)
	if err == nil {
		t.Fatal("expected error for k3s mode")
	}
}

func TestScaleToZeroGuardSleepServiceStandalone(t *testing.T) {
	g := testScaleToZeroGuard()
	state, err := g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	if err != nil {
		t.Fatalf("sleep service: %v", err)
	}
	if state.Status != "sleeping" {
		t.Fatalf("expected status sleeping, got %q", state.Status)
	}
}

func TestScaleToZeroGuardWakeService(t *testing.T) {
	g := testScaleToZeroGuard()
	g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	state, err := g.WakeService("api", contracts.RuntimeModeStandalone)
	if err != nil {
		t.Fatalf("wake service: %v", err)
	}
	if state.Status != "waking" {
		t.Fatalf("expected status waking, got %q", state.Status)
	}
}

func TestScaleToZeroGuardMarkActive(t *testing.T) {
	g := testScaleToZeroGuard()
	g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	g.WakeService("api", contracts.RuntimeModeStandalone)
	g.MarkActive("api")

	state, ok := g.autosleep.GetState("api")
	if !ok {
		t.Fatal("expected state to exist")
	}
	if state.Status != "active" {
		t.Fatalf("expected status active, got %q", state.Status)
	}

	wakeAttempts, coldStarts, _ := g.GetWakeStats("api")
	if wakeAttempts != 0 {
		t.Fatalf("expected 0 wake attempts after mark active, got %d", wakeAttempts)
	}
	if coldStarts != 1 {
		t.Fatalf("expected 1 cold start, got %d", coldStarts)
	}
}

func TestScaleToZeroGuardCheckWakeTimeout(t *testing.T) {
	g := testScaleToZeroGuard()
	g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	g.WakeService("api", contracts.RuntimeModeStandalone)

	g.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 35, 0, time.UTC)
	}

	timedOut, attempts := g.CheckWakeTimeout("api")
	if !timedOut {
		t.Fatal("expected wake timeout")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestScaleToZeroGuardCheckWakeTimeoutNotExceeded(t *testing.T) {
	g := testScaleToZeroGuard()
	g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	g.WakeService("api", contracts.RuntimeModeStandalone)

	timedOut, _ := g.CheckWakeTimeout("api")
	if timedOut {
		t.Fatal("expected no wake timeout yet")
	}
}

func TestScaleToZeroGuardCheckColdStartTimeout(t *testing.T) {
	g := testScaleToZeroGuard()
	for i := 0; i < 4; i++ {
		g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
		g.WakeService("api", contracts.RuntimeModeStandalone)
		g.MarkActive("api")
	}

	isColdStartTimeout := g.CheckColdStartTimeout("api")
	if !isColdStartTimeout {
		t.Fatal("expected cold start timeout after 4 cold starts")
	}
}

func TestScaleToZeroGuardCheckColdStartTimeoutNotExceeded(t *testing.T) {
	g := testScaleToZeroGuard()
	g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	g.WakeService("api", contracts.RuntimeModeStandalone)
	g.MarkActive("api")

	isColdStartTimeout := g.CheckColdStartTimeout("api")
	if isColdStartTimeout {
		t.Fatal("expected no cold start timeout yet")
	}
}

func TestScaleToZeroGuardResetWakeStats(t *testing.T) {
	g := testScaleToZeroGuard()
	g.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	g.WakeService("api", contracts.RuntimeModeStandalone)

	g.ResetWakeStats("api")
	wakeAttempts, coldStarts, lastWake := g.GetWakeStats("api")
	if wakeAttempts != 0 {
		t.Fatalf("expected 0 wake attempts after reset, got %d", wakeAttempts)
	}
	if coldStarts != 0 {
		t.Fatalf("expected 0 cold starts after reset, got %d", coldStarts)
	}
	if !lastWake.IsZero() {
		t.Fatal("expected zero last wake time after reset")
	}
}

func TestSleepServiceHandlerRejectsK3sMode(t *testing.T) {
	guard := testScaleToZeroGuard()
	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(guard.autosleep)
	service.WithScaleToZeroGuard(guard)

	payload := SleepServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeDistributedK3s,
		ServiceName: "api",
		Policy:      contracts.ScaleToZeroPolicy{Enabled: true},
	}
	raw, _ := json.Marshal(payload)

	result := service.handleSleepService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSleepService,
		RequestID:     "req_sleep_k3s",
		CorrelationID: "corr_sleep_k3s",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected handler to reject k3s mode")
	}
	if result.Error.Code != "scale_to_zero_policy_violation" {
		t.Fatalf("expected scale_to_zero_policy_violation code, got %q", result.Error.Code)
	}
}

func TestSleepServiceHandlerRejectsDisabledPolicy(t *testing.T) {
	guard := testScaleToZeroGuard()
	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(guard.autosleep)
	service.WithScaleToZeroGuard(guard)

	payload := SleepServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
		Policy:      contracts.ScaleToZeroPolicy{Enabled: false},
	}
	raw, _ := json.Marshal(payload)

	result := service.handleSleepService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSleepService,
		RequestID:     "req_sleep_disabled",
		CorrelationID: "corr_sleep_disabled",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected handler to reject disabled policy")
	}
	if result.Error.Code != "scale_to_zero_policy_violation" {
		t.Fatalf("expected scale_to_zero_policy_violation code, got %q", result.Error.Code)
	}
}

func TestSleepServiceHandlerRejectsNotEligible(t *testing.T) {
	guard := testScaleToZeroGuard()
	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(guard.autosleep)
	service.WithScaleToZeroGuard(guard)

	payload := SleepServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
		Policy:      contracts.ScaleToZeroPolicy{Enabled: true},
	}
	raw, _ := json.Marshal(payload)

	result := service.handleSleepService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSleepService,
		RequestID:     "req_sleep_not_eligible",
		CorrelationID: "corr_sleep_not_eligible",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected handler to reject not eligible service")
	}
	if result.Error.Code != "service_not_eligible_for_sleep" {
		t.Fatalf("expected service_not_eligible_for_sleep code, got %q", result.Error.Code)
	}
}

func TestSleepServiceHandlerSucceedsWithGuard(t *testing.T) {
	guard := testScaleToZeroGuard()
	guard.autosleep.MarkActive("api")
	guard.autosleep.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 5, 0, time.UTC)
	}
	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(guard.autosleep)
	service.WithScaleToZeroGuard(guard)

	payload := SleepServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
		Policy:      contracts.ScaleToZeroPolicy{Enabled: true, IdleWindow: "2s"},
	}
	raw, _ := json.Marshal(payload)

	result := service.handleSleepService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandSleepService,
		RequestID:     "req_sleep_ok",
		CorrelationID: "corr_sleep_ok",
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
}

func TestWakeServiceHandlerRejectsWakeTimeout(t *testing.T) {
	guard := testScaleToZeroGuard()
	guard.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
	guard.WakeService("api", contracts.RuntimeModeStandalone)

	guard.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 35, 0, time.UTC)
	}

	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(guard.autosleep)
	service.WithScaleToZeroGuard(guard)

	payload := WakeServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
	}
	raw, _ := json.Marshal(payload)

	result := service.handleWakeService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandWakeService,
		RequestID:     "req_wake_timeout",
		CorrelationID: "corr_wake_timeout",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected handler to reject wake timeout")
	}
	if result.Error.Code != "wake_timeout_exceeded" {
		t.Fatalf("expected wake_timeout_exceeded code, got %q", result.Error.Code)
	}
	if result.Error.Retryable {
		t.Fatal("expected wake timeout to be non-retryable")
	}
}

func TestWakeServiceHandlerRejectsColdStartTimeout(t *testing.T) {
	guard := testScaleToZeroGuard()
	for i := 0; i < 4; i++ {
		guard.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)
		guard.WakeService("api", contracts.RuntimeModeStandalone)
		guard.MarkActive("api")
	}

	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(guard.autosleep)
	service.WithScaleToZeroGuard(guard)

	payload := WakeServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
	}
	raw, _ := json.Marshal(payload)

	result := service.handleWakeService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandWakeService,
		RequestID:     "req_wake_cold_start",
		CorrelationID: "corr_wake_cold_start",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error == nil {
		t.Fatal("expected handler to reject cold start timeout")
	}
	if result.Error.Code != "cold_start_timeout_exceeded" {
		t.Fatalf("expected cold_start_timeout_exceeded code, got %q", result.Error.Code)
	}
	if result.Error.Retryable {
		t.Fatal("expected cold start timeout to be non-retryable")
	}
}

func TestWakeServiceHandlerSucceedsWithGuard(t *testing.T) {
	guard := testScaleToZeroGuard()
	guard.SleepService("api", "rev_123", contracts.RuntimeModeStandalone)

	service := NewService(nil, nil, nil)
	service.WithAutosleepManager(guard.autosleep)
	service.WithScaleToZeroGuard(guard)

	payload := WakeServicePayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
		ServiceName: "api",
	}
	raw, _ := json.Marshal(payload)

	result := service.handleWakeService(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandWakeService,
		RequestID:     "req_wake_ok",
		CorrelationID: "corr_wake_ok",
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
}

func TestScaleToZeroGuardCanBeRegistered(t *testing.T) {
	registry := dispatcher.NewDefaultRegistry()
	service := NewService(nil, nil, nil)
	service.Register(registry)

	if _, ok := registry.Resolve(contracts.CommandSleepService); !ok {
		t.Fatal("expected sleep_service handler to be registered")
	}
	if _, ok := registry.Resolve(contracts.CommandWakeService); !ok {
		t.Fatal("expected wake_service handler to be registered")
	}
}
