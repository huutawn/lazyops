package tunnel

import (
	"testing"
	"time"
)

func TestParseTunnelType(t *testing.T) {
	tests := []struct {
		input   string
		want    TunnelType
		wantErr bool
	}{
		{"db", TypeDB, false},
		{"DB", TypeDB, false},
		{"tcp", TypeTCP, false},
		{"TCP", TypeTCP, false},
		{"udp", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := ParseTunnelType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseTunnelType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseTunnelType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	validCfg := Config{
		Type:      TypeDB,
		LocalPort: 15432,
		Remote:    "localhost:5432",
		Timeout:   30 * time.Minute,
		ProjectID: "prj_demo",
	}

	if err := validCfg.Validate(); err != nil {
		t.Fatalf("expected valid config to pass, got %v", err)
	}

	badType := validCfg
	badType.Type = ""
	if err := badType.Validate(); err == nil {
		t.Fatal("expected missing type error, got nil")
	}

	badPort := validCfg
	badPort.LocalPort = 80
	if err := badPort.Validate(); err == nil {
		t.Fatal("expected invalid port error, got nil")
	}

	badPortHigh := validCfg
	badPortHigh.LocalPort = 70000
	if err := badPortHigh.Validate(); err == nil {
		t.Fatal("expected high port error, got nil")
	}

	badRemote := validCfg
	badRemote.Remote = ""
	if err := badRemote.Validate(); err == nil {
		t.Fatal("expected missing remote error, got nil")
	}

	badTimeout := validCfg
	badTimeout.Timeout = 1 * time.Minute
	if err := badTimeout.Validate(); err == nil {
		t.Fatal("expected short timeout error, got nil")
	}

	badProject := validCfg
	badProject.ProjectID = ""
	if err := badProject.Validate(); err == nil {
		t.Fatal("expected missing project id error, got nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	dbCfg := DefaultConfig(TypeDB)
	if dbCfg.LocalPort != DefaultDBPort {
		t.Fatalf("expected default db port %d, got %d", DefaultDBPort, dbCfg.LocalPort)
	}
	if dbCfg.Remote != DefaultDBRemote {
		t.Fatalf("expected default db remote %q, got %q", DefaultDBRemote, dbCfg.Remote)
	}
	if dbCfg.Timeout != DefaultTunnelTimeout {
		t.Fatalf("expected default timeout %v, got %v", DefaultTunnelTimeout, dbCfg.Timeout)
	}

	tcpCfg := DefaultConfig(TypeTCP)
	if tcpCfg.LocalPort != 19090 {
		t.Fatalf("expected default tcp port 19090, got %d", tcpCfg.LocalPort)
	}
	if tcpCfg.Type != TypeTCP {
		t.Fatalf("expected type tcp, got %s", tcpCfg.Type)
	}
}

func TestNewSessionFromContract(t *testing.T) {
	session := NewSessionFromContract("tun_001", TypeDB, 15432, "localhost:5432", "ready", 30*time.Minute)

	if session.ID != "tun_001" {
		t.Fatalf("expected session id tun_001, got %s", session.ID)
	}
	if session.Type != TypeDB {
		t.Fatalf("expected type db, got %s", session.Type)
	}
	if session.LocalPort != 15432 {
		t.Fatalf("expected port 15432, got %d", session.LocalPort)
	}
	if session.Remote != "localhost:5432" {
		t.Fatalf("expected remote localhost:5432, got %s", session.Remote)
	}
	if session.Status != "ready" {
		t.Fatalf("expected status ready, got %s", session.Status)
	}
	if session.ExpiresAt.Before(session.CreatedAt) {
		t.Fatal("expected expires_at after created_at")
	}
}

func TestManagerCreateRejectsInvalidConfig(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.Create(Config{})
	if err == nil {
		t.Fatal("expected config validation error, got nil")
	}
}

func TestManagerCreateAndStop(t *testing.T) {
	mgr := NewManagerWithPortChecker(&mockPortChecker{available: true})

	session, err := mgr.Create(Config{
		Type:      TypeDB,
		LocalPort: 15432,
		Remote:    "localhost:5432",
		Timeout:   30 * time.Minute,
		ProjectID: "prj_demo",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if session.Status != "pending" {
		t.Fatalf("expected pending status, got %s", session.Status)
	}

	session.Status = "active"
	mgr.Register(session)

	if err := mgr.Stop(session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	active := mgr.ActiveSessions()
	if len(active) != 0 {
		t.Fatalf("expected no active sessions after stop, got %d", len(active))
	}
}

func TestManagerStopNotFound(t *testing.T) {
	mgr := NewManager()
	err := mgr.Stop("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
}

func TestManagerStopsInactiveSession(t *testing.T) {
	mgr := NewManagerWithPortChecker(&mockPortChecker{available: true})

	session, _ := mgr.Create(Config{
		Type:      TypeTCP,
		LocalPort: 19090,
		Remote:    "localhost:8080",
		Timeout:   30 * time.Minute,
		ProjectID: "prj_demo",
	})
	mgr.Register(session)

	err := mgr.Stop(session.ID)
	if err == nil {
		t.Fatal("expected error when stopping non-active session, got nil")
	}
}

func TestManagerCleanupExpiresOldSessions(t *testing.T) {
	mgr := NewManager()

	oldSession := Session{
		ID:        "tun_old",
		Type:      TypeDB,
		LocalPort: 15432,
		Remote:    "localhost:5432",
		Status:    "active",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	mgr.Register(oldSession)

	freshSession := Session{
		ID:        "tun_fresh",
		Type:      TypeTCP,
		LocalPort: 19090,
		Remote:    "localhost:8080",
		Status:    "active",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}
	mgr.Register(freshSession)

	mgr.Cleanup()

	active := mgr.ActiveSessions()
	if len(active) != 1 {
		t.Fatalf("expected 1 active session after cleanup, got %d", len(active))
	}
	if active[0].ID != "tun_fresh" {
		t.Fatalf("expected fresh session to remain active, got %s", active[0].ID)
	}
}

func TestManagerRejectsDuplicatePort(t *testing.T) {
	mgr := NewManagerWithPortChecker(&mockPortChecker{available: true})

	session, _ := mgr.Create(Config{
		Type:      TypeDB,
		LocalPort: 15432,
		Remote:    "localhost:5432",
		Timeout:   30 * time.Minute,
		ProjectID: "prj_demo",
	})
	session.Status = "active"
	mgr.Register(session)

	_, err := mgr.Create(Config{
		Type:      TypeTCP,
		LocalPort: 15432,
		Remote:    "localhost:8080",
		Timeout:   30 * time.Minute,
		ProjectID: "prj_demo",
	})
	if err == nil {
		t.Fatal("expected duplicate port error, got nil")
	}
}

func TestDebugWarningMessage(t *testing.T) {
	msg := DebugWarningMessage(TypeDB)
	if msg == "" {
		t.Fatal("expected non-empty debug warning message")
	}
	if !containsAll(msg, "debug", "db", "production") {
		t.Fatalf("expected debug warning to mention debug, db, and production, got %q", msg)
	}
}

type mockPortChecker struct {
	available bool
}

func (m *mockPortChecker) IsPortAvailable(port int) error {
	if !m.available {
		return nil
	}
	return nil
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) < len(sub) {
			return false
		}
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
