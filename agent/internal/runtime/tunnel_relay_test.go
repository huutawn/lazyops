package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testTunnelRelay() *TunnelRelay {
	r := NewTunnelRelay(nil, TunnelRelayConfig{
		MaxActiveTunnels: 5,
		SessionTimeout:   1 * time.Second,
	})
	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	}
	return r
}

func TestTunnelRelayDefaultConfig(t *testing.T) {
	r := NewTunnelRelay(nil, TunnelRelayConfig{})
	if r.cfg.MaxActiveTunnels != 5 {
		t.Fatalf("expected default max tunnels 5, got %d", r.cfg.MaxActiveTunnels)
	}
	if r.cfg.SessionTimeout != 30*time.Minute {
		t.Fatalf("expected default timeout 30m, got %s", r.cfg.SessionTimeout)
	}
}

func TestTunnelRelayCreateSession(t *testing.T) {
	r := testTunnelRelay()
	session, err := r.CreateSession("tunnel_1", "prj_123", "bind_123", "rev_123", "api", 8080)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.TunnelID != "tunnel_1" {
		t.Fatalf("expected tunnel_id tunnel_1, got %q", session.TunnelID)
	}
	if session.Status != "active" {
		t.Fatalf("expected status active, got %q", session.Status)
	}
	if session.TargetPort != 8080 {
		t.Fatalf("expected target port 8080, got %d", session.TargetPort)
	}
	if session.ExpiresAt.IsZero() {
		t.Fatal("expected expires_at to be set")
	}
}

func TestTunnelRelayMaxActiveTunnels(t *testing.T) {
	r := NewTunnelRelay(nil, TunnelRelayConfig{
		MaxActiveTunnels: 2,
		SessionTimeout:   10 * time.Second,
	})
	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	}

	_, err := r.CreateSession("tunnel_1", "prj_123", "bind_123", "rev_123", "api", 8080)
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	_, err = r.CreateSession("tunnel_2", "prj_123", "bind_123", "rev_123", "web", 3000)
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}
	_, err = r.CreateSession("tunnel_3", "prj_123", "bind_123", "rev_123", "worker", 9090)
	if err == nil {
		t.Fatal("expected error when max tunnels reached")
	}
}

func TestTunnelRelayCloseSession(t *testing.T) {
	r := testTunnelRelay()
	_, err := r.CreateSession("tunnel_1", "prj_123", "bind_123", "rev_123", "api", 8080)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ok := r.CloseSession("tunnel_1")
	if !ok {
		t.Fatal("expected close session to succeed")
	}

	total, active, _ := r.Stats()
	if total != 1 {
		t.Fatalf("expected 1 total, got %d", total)
	}
	if active != 0 {
		t.Fatalf("expected 0 active after close, got %d", active)
	}
}

func TestTunnelRelayCloseSessionNotFound(t *testing.T) {
	r := testTunnelRelay()
	ok := r.CloseSession("nonexistent")
	if ok {
		t.Fatal("expected close session to fail for nonexistent tunnel")
	}
}

func TestTunnelRelayCollectExpiredSessions(t *testing.T) {
	r := testTunnelRelay()
	_, err := r.CreateSession("tunnel_1", "prj_123", "bind_123", "rev_123", "api", 8080)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 0, 5, 0, time.UTC)
	}

	expired := r.CollectExpiredSessions()
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired session, got %d", len(expired))
	}
	if expired[0].Status != "expired" {
		t.Fatalf("expected status expired, got %q", expired[0].Status)
	}

	_, active, expiredCount := r.Stats()
	if active != 0 {
		t.Fatalf("expected 0 active after collection, got %d", active)
	}
	if expiredCount != 1 {
		t.Fatalf("expected 1 expired count, got %d", expiredCount)
	}
}

func TestTunnelRelayActiveSessions(t *testing.T) {
	r := testTunnelRelay()
	r.CreateSession("tunnel_1", "prj_123", "bind_123", "rev_123", "api", 8080)
	r.CreateSession("tunnel_2", "prj_123", "bind_123", "rev_123", "web", 3000)

	sessions := r.ActiveSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected 2 active sessions, got %d", len(sessions))
	}
}

func TestTunnelRelayPersistTunnelState(t *testing.T) {
	r := testTunnelRelay()
	r.CreateSession("tunnel_1", "prj_123", "bind_123", "rev_123", "api", 8080)

	root := filepath.Join(t.TempDir(), "runtime-root")
	statePath, err := r.PersistTunnelState(root, "prj_123", "bind_123")
	if err != nil {
		t.Fatalf("persist tunnel state: %v", err)
	}

	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected tunnel state file to exist: %v", err)
	}

	var loaded []*TunnelSession
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read tunnel state file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode tunnel state file: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 session in persisted state, got %d", len(loaded))
	}
}

func TestTunnelRelayHandleReportTunnelState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	r := testTunnelRelay()
	r.CreateSession("tunnel_1", "prj_123", "bind_123", "rev_123", "api", 8080)

	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 0, 5, 0, time.UTC)
	}

	expired, err := r.HandleReportTunnelState(context.Background(), nil, ReportTunnelStatePayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: root,
	})
	if err != nil {
		t.Fatalf("handle report tunnel state: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expected 1 expired, got %d", expired)
	}

	statePath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "tunnels", "state_20260404T120005Z.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected tunnel state file at %s: %v", statePath, err)
	}
}

func TestTunnelRelayHandleReportTunnelStateNoSessions(t *testing.T) {
	r := testTunnelRelay()

	expired, err := r.HandleReportTunnelState(context.Background(), nil, ReportTunnelStatePayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("handle report tunnel state: %v", err)
	}
	if expired != 0 {
		t.Fatalf("expected 0 expired, got %d", expired)
	}
}
