package config

import (
	"log/slog"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestValidateRejectsInvalidModePairing(t *testing.T) {
	cfg := Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		LogLevel:          slog.LevelInfo,
		RuntimeMode:       contracts.RuntimeModeDistributedK3s,
		AgentKind:         contracts.AgentKindInstance,
		ControlPlaneURL:   "ws://127.0.0.1:8080",
		StateDir:          t.TempDir(),
		ShutdownTimeout:   time.Second,
		HeartbeatInterval: time.Second,
		HandshakeVersion:  "v0",
		UseMockControl:    true,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid mode pairing to fail validation")
	}
}

func TestValidateAcceptsNodeAgentOnK3s(t *testing.T) {
	cfg := Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		LogLevel:          slog.LevelInfo,
		RuntimeMode:       contracts.RuntimeModeDistributedK3s,
		AgentKind:         contracts.AgentKindNode,
		TargetRef:         "k3s-dev",
		ControlPlaneURL:   "ws://127.0.0.1:8080",
		StateDir:          t.TempDir(),
		ShutdownTimeout:   time.Second,
		HeartbeatInterval: time.Second,
		HandshakeVersion:  "v0",
		UseMockControl:    true,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to validate, got %v", err)
	}
}

func TestValidateRequiresEncryptionKeyWhenBootstrapProvided(t *testing.T) {
	cfg := Config{
		AppName:           "lazyops-agent",
		AppEnv:            "test",
		LogLevel:          slog.LevelInfo,
		RuntimeMode:       contracts.RuntimeModeStandalone,
		AgentKind:         contracts.AgentKindInstance,
		BootstrapToken:    "bootstrap-valid-standalone",
		TargetRef:         "local-dev",
		ControlPlaneURL:   "ws://127.0.0.1:8080",
		StateDir:          t.TempDir(),
		ShutdownTimeout:   time.Second,
		HeartbeatInterval: time.Second,
		HandshakeVersion:  "v0",
		UseMockControl:    true,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing encryption key to fail validation")
	}
}
