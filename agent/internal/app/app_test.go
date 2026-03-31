package app

import (
	"context"
	"path/filepath"
	"testing"

	"lazyops-agent/internal/config"
	"lazyops-agent/internal/contracts"
)

func TestNewCreatesApplicationWithMockClient(t *testing.T) {
	cfg := config.Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		RuntimeMode:       contracts.RuntimeModeStandalone,
		AgentKind:         contracts.AgentKindInstance,
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
	if app.store == nil || app.control == nil || app.logger == nil {
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

func TestBootstrapLocalStatePersistsSnapshot(t *testing.T) {
	cfg := config.Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		RuntimeMode:       contracts.RuntimeModeStandalone,
		AgentKind:         contracts.AgentKindInstance,
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
}

const testDuration = 10
