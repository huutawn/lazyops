package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/config"
	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/state"
)

func TestNewCreatesApplicationWithMockClient(t *testing.T) {
	cfg := config.Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		LogLevel:          0,
		RuntimeMode:       contracts.RuntimeModeStandalone,
		AgentKind:         contracts.AgentKindInstance,
		TargetRef:         "local-dev",
		ControlPlaneURL:   "ws://127.0.0.1:8080",
		StateDir:          filepath.Join(t.TempDir(), "state"),
		ShutdownTimeout:   testDuration,
		HeartbeatInterval: testDuration,
		HandshakeVersion:  "v0",
		UseMockControl:    true,
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if app.store == nil || app.control == nil || app.logger == nil || app.dispatcher == nil {
		t.Fatal("expected app dependencies to be initialized")
	}
}

func TestNewCreatesApplicationWithWebSocketClient(t *testing.T) {
	cfg := config.Config{
		AppName:             "lazyops-agent",
		AppEnv:              "test",
		LogLevel:            0,
		RuntimeMode:         contracts.RuntimeModeStandalone,
		AgentKind:           contracts.AgentKindInstance,
		TargetRef:           "local-dev",
		ControlPlaneURL:     "ws://127.0.0.1:8080",
		StateDir:            filepath.Join(t.TempDir(), "state"),
		ShutdownTimeout:     testDuration,
		HeartbeatInterval:   testDuration,
		HandshakeVersion:    "v0",
		ControlDialTimeout:  time.Second,
		ControlWriteTimeout: time.Second,
		ControlPongWait:     2 * time.Second,
		ControlPingPeriod:   time.Second,
		ReconnectMinBackoff: time.Second,
		ReconnectMaxBackoff: 2 * time.Second,
		ReconnectJitter:     0,
		UseMockControl:      false,
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if app.store == nil || app.control == nil || app.logger == nil || app.dispatcher == nil {
		t.Fatal("expected app dependencies to be initialized")
	}
}

func TestDefaultCapabilitiesKeepSidecarPrecedence(t *testing.T) {
	capabilities := defaultCapabilities(config.Config{
		RuntimeMode: contracts.RuntimeModeStandalone,
		AgentKind:   contracts.AgentKindInstance,
	})

	want := []string{"env_injection", "managed_credentials", "localhost_rescue"}
	if len(capabilities.Sidecar.Precedence) != len(want) {
		t.Fatalf("unexpected precedence length %d", len(capabilities.Sidecar.Precedence))
	}
	for i := range want {
		if capabilities.Sidecar.Precedence[i] != want[i] {
			t.Fatalf("unexpected precedence[%d] = %q", i, capabilities.Sidecar.Precedence[i])
		}
	}
}

func TestDefaultCapabilitiesAdvertiseNetworkSupport(t *testing.T) {
	capabilities := defaultCapabilities(config.Config{
		RuntimeMode: contracts.RuntimeModeDistributedMesh,
		AgentKind:   contracts.AgentKindInstance,
	})

	if !capabilities.Network.OutboundOnly {
		t.Fatal("expected network capability to remain outbound-only")
	}
	if !capabilities.Network.PrivateOverlay {
		t.Fatal("expected distributed-mesh capability to advertise private overlay support")
	}
	if len(capabilities.Network.SupportedMeshProviders) != 2 {
		t.Fatalf("expected 2 supported mesh providers, got %d", len(capabilities.Network.SupportedMeshProviders))
	}
}

func TestBootstrapLocalStatePersistsSnapshot(t *testing.T) {
	cfg := config.Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		LogLevel:          0,
		RuntimeMode:       contracts.RuntimeModeStandalone,
		AgentKind:         contracts.AgentKindInstance,
		TargetRef:         "local-dev",
		ControlPlaneURL:   "ws://127.0.0.1:8080",
		StateDir:          filepath.Join(t.TempDir(), "state"),
		ShutdownTimeout:   testDuration,
		HeartbeatInterval: testDuration,
		HandshakeVersion:  "v0",
		UseMockControl:    true,
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	state, err := app.store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	err = app.bootstrapLocalState(context.Background(), state, contracts.MachineInfo{
		Hostname: "local-dev",
		OS:       "linux",
		Arch:     "amd64",
	})
	if err != nil {
		t.Fatalf("bootstrap local state: %v", err)
	}

	updated, err := app.store.Load(context.Background())
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if updated.Metadata.AgentID == "" {
		t.Fatal("expected bootstrap to persist a local agent ID")
	}
	if updated.CapabilitySnapshot.LastComputedAt.IsZero() {
		t.Fatal("expected bootstrap to persist capability snapshot")
	}
	if updated.CapabilitySnapshot.Fingerprint == "" {
		t.Fatal("expected bootstrap to persist capability fingerprint")
	}
	if updated.CapabilitySnapshot.Version == 0 {
		t.Fatal("expected bootstrap to persist capability version")
	}
}

func TestSessionAuthFromStateUsesDecryptedEnrollmentToken(t *testing.T) {
	cfg := config.Config{
		AppName:            "lazyops-agent",
		AppEnv:             "test",
		LogLevel:           0,
		RuntimeMode:        contracts.RuntimeModeStandalone,
		AgentKind:          contracts.AgentKindInstance,
		TargetRef:          "local-dev",
		ControlPlaneURL:    "ws://127.0.0.1:8080",
		StateDir:           filepath.Join(t.TempDir(), "state"),
		StateEncryptionKey: "state-key-123",
		ShutdownTimeout:    testDuration,
		HeartbeatInterval:  testDuration,
		HandshakeVersion:   "v0",
		UseMockControl:     true,
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	encrypted, err := state.EncryptSecret("agt-secret-standalone", cfg.StateEncryptionKey)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}

	if _, err := app.store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.Metadata.AgentID = "agt_enrolled_standalone"
		local.Metadata.RuntimeMode = contracts.RuntimeModeStandalone
		local.Metadata.AgentKind = contracts.AgentKindInstance
		local.Enrollment.Enrolled = true
		local.Enrollment.EncryptedAgentToken = encrypted
		return nil
	}); err != nil {
		t.Fatalf("persist enrolled state: %v", err)
	}

	local, err := app.store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	auth, err := app.sessionAuthFromState(context.Background(), local)
	if err != nil {
		t.Fatalf("session auth from state: %v", err)
	}
	if auth.AgentToken != "agt-secret-standalone" {
		t.Fatalf("expected decrypted agent token, got %q", auth.AgentToken)
	}
}

func TestSessionAuthFromStateReusesStoredSessionID(t *testing.T) {
	cfg := config.Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		LogLevel:          0,
		RuntimeMode:       contracts.RuntimeModeStandalone,
		AgentKind:         contracts.AgentKindInstance,
		TargetRef:         "local-dev",
		ControlPlaneURL:   "ws://127.0.0.1:8080",
		StateDir:          filepath.Join(t.TempDir(), "state"),
		ShutdownTimeout:   testDuration,
		HeartbeatInterval: testDuration,
		HandshakeVersion:  "v0",
		UseMockControl:    true,
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	if _, err := app.store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.Metadata.AgentID = "agt_local"
		local.Enrollment.SessionID = "sess_existing"
		return nil
	}); err != nil {
		t.Fatalf("persist session state: %v", err)
	}

	local, err := app.store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	auth, err := app.sessionAuthFromState(context.Background(), local)
	if err != nil {
		t.Fatalf("session auth from state: %v", err)
	}
	if auth.SessionID != "sess_existing" {
		t.Fatalf("expected stored session ID to be reused, got %q", auth.SessionID)
	}
}

const testDuration = 10 * time.Millisecond
