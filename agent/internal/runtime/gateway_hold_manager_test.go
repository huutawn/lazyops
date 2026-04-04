package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testGatewayHoldManager() *GatewayHoldManager {
	m := NewGatewayHoldManager(nil, GatewayHoldConfig{
		DefaultHoldTimeout: 1 * time.Second,
		MaxHeldRequests:    5,
	})
	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)
	}
	return m
}

func TestGatewayHoldManagerDefaultConfig(t *testing.T) {
	m := NewGatewayHoldManager(nil, GatewayHoldConfig{})
	if m.cfg.DefaultHoldTimeout != 30*time.Second {
		t.Fatalf("expected default hold timeout 30s, got %s", m.cfg.DefaultHoldTimeout)
	}
	if m.cfg.MaxHeldRequests != 100 {
		t.Fatalf("expected default max held requests 100, got %d", m.cfg.MaxHeldRequests)
	}
}

func TestGatewayHoldManagerHoldRequest(t *testing.T) {
	m := testGatewayHoldManager()
	req, err := m.HoldRequest("api", "req_1", "corr_1", 0)
	if err != nil {
		t.Fatalf("hold request: %v", err)
	}
	if req.RequestID != "req_1" {
		t.Fatalf("expected request_id req_1, got %q", req.RequestID)
	}
	if req.ServiceName != "api" {
		t.Fatalf("expected service name api, got %q", req.ServiceName)
	}
	if req.Status != "held" {
		t.Fatalf("expected status held, got %q", req.Status)
	}
	if req.ExpiresAt.IsZero() {
		t.Fatal("expected expires_at to be set")
	}
}

func TestGatewayHoldManagerHoldRequestCustomTimeout(t *testing.T) {
	m := testGatewayHoldManager()
	req, err := m.HoldRequest("api", "req_1", "corr_1", 5*time.Second)
	if err != nil {
		t.Fatalf("hold request: %v", err)
	}
	if req.ExpiresAt.Sub(req.HeldAt) != 5*time.Second {
		t.Fatalf("expected 5s hold timeout, got %s", req.ExpiresAt.Sub(req.HeldAt))
	}
}

func TestGatewayHoldManagerMaxHeldRequests(t *testing.T) {
	m := testGatewayHoldManager()
	for i := 0; i < 5; i++ {
		_, err := m.HoldRequest("api", "req_1", "corr_1", 0)
		if err != nil {
			t.Fatalf("hold request %d: %v", i, err)
		}
	}
	_, err := m.HoldRequest("api", "req_6", "corr_6", 0)
	if err == nil {
		t.Fatal("expected error when max held requests reached")
	}
}

func TestGatewayHoldManagerResumeRequests(t *testing.T) {
	m := testGatewayHoldManager()
	m.HoldRequest("api", "req_1", "corr_1", 5*time.Second)
	m.HoldRequest("api", "req_2", "corr_2", 5*time.Second)

	resumed := m.ResumeRequests("api")
	if len(resumed) != 2 {
		t.Fatalf("expected 2 resumed requests, got %d", len(resumed))
	}
	for _, req := range resumed {
		if req.Status != "resumed" {
			t.Fatalf("expected status resumed, got %q", req.Status)
		}
	}

	total, active, expired, resumedCount := m.Stats()
	if active != 0 {
		t.Fatalf("expected 0 active after resume, got %d", active)
	}
	if resumedCount != 2 {
		t.Fatalf("expected 2 resumed count, got %d", resumedCount)
	}
	if total != 2 {
		t.Fatalf("expected 2 total, got %d", total)
	}
	if expired != 0 {
		t.Fatalf("expected 0 expired, got %d", expired)
	}
}

func TestGatewayHoldManagerResumeRequestsExpired(t *testing.T) {
	m := testGatewayHoldManager()
	m.HoldRequest("api", "req_1", "corr_1", 1*time.Second)

	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 5, 0, time.UTC)
	}

	resumed := m.ResumeRequests("api")
	if len(resumed) != 0 {
		t.Fatalf("expected 0 resumed (expired), got %d", len(resumed))
	}

	_, _, expired, _ := m.Stats()
	if expired != 1 {
		t.Fatalf("expected 1 expired, got %d", expired)
	}
}

func TestGatewayHoldManagerResumeRequestsEmpty(t *testing.T) {
	m := testGatewayHoldManager()
	resumed := m.ResumeRequests("api")
	if len(resumed) != 0 {
		t.Fatalf("expected 0 resumed, got %d", len(resumed))
	}
}

func TestGatewayHoldManagerCollectExpiredHolds(t *testing.T) {
	m := testGatewayHoldManager()
	m.HoldRequest("api", "req_1", "corr_1", 1*time.Second)
	m.HoldRequest("web", "req_2", "corr_2", 10*time.Second)

	m.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 5, 0, time.UTC)
	}

	expired := m.CollectExpiredHolds()
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired hold, got %d", len(expired))
	}
	if expired[0].RequestID != "req_1" {
		t.Fatalf("expected req_1 expired, got %q", expired[0].RequestID)
	}
	if expired[0].Status != "expired" {
		t.Fatalf("expected status expired, got %q", expired[0].Status)
	}
}

func TestGatewayHoldManagerStats(t *testing.T) {
	m := testGatewayHoldManager()
	m.HoldRequest("api", "req_1", "corr_1", 5*time.Second)
	m.HoldRequest("web", "req_2", "corr_2", 5*time.Second)

	total, active, expired, resumed := m.Stats()
	if total != 2 {
		t.Fatalf("expected 2 total, got %d", total)
	}
	if active != 2 {
		t.Fatalf("expected 2 active, got %d", active)
	}
	if expired != 0 {
		t.Fatalf("expected 0 expired, got %d", expired)
	}
	if resumed != 0 {
		t.Fatalf("expected 0 resumed, got %d", resumed)
	}
}

func TestGatewayHoldManagerPersistHoldState(t *testing.T) {
	m := testGatewayHoldManager()
	m.HoldRequest("api", "req_1", "corr_1", 5*time.Second)

	root := filepath.Join(t.TempDir(), "runtime-root")
	holdPath, err := m.PersistHoldState(root, "prj_123", "bind_123")
	if err != nil {
		t.Fatalf("persist hold state: %v", err)
	}

	if _, err := os.Stat(holdPath); err != nil {
		t.Fatalf("expected hold state file to exist: %v", err)
	}

	var loaded map[string][]*HeldRequest
	raw, err := os.ReadFile(holdPath)
	if err != nil {
		t.Fatalf("read hold state file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode hold state file: %v", err)
	}
	if len(loaded["api"]) != 1 {
		t.Fatalf("expected 1 hold for api, got %d", len(loaded["api"]))
	}
}
