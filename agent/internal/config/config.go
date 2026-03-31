package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
)

type Config struct {
	AppName            string
	AppEnv             string
	LogLevel           slog.Level
	RuntimeMode        contracts.RuntimeMode
	AgentKind          contracts.AgentKind
	BootstrapToken     string
	TargetRef          string
	ControlPlaneURL    string
	StateDir           string
	StateEncryptionKey string
	ShutdownTimeout    time.Duration
	HeartbeatInterval  time.Duration
	HandshakeVersion   string
	UseMockControl     bool
}

func Load() (Config, error) {
	cfg := Config{
		AppName:            envOrDefault("AGENT_APP_NAME", "lazyops-agent"),
		AppEnv:             envOrDefault("AGENT_APP_ENV", "development"),
		RuntimeMode:        contracts.RuntimeMode(envOrDefault("AGENT_RUNTIME_MODE", string(contracts.RuntimeModeStandalone))),
		AgentKind:          contracts.AgentKind(envOrDefault("AGENT_KIND", string(contracts.AgentKindInstance))),
		BootstrapToken:     strings.TrimSpace(os.Getenv("AGENT_BOOTSTRAP_TOKEN")),
		TargetRef:          envOrDefault("AGENT_TARGET_REF", "local-dev"),
		ControlPlaneURL:    envOrDefault("AGENT_CONTROL_PLANE_URL", "ws://127.0.0.1:8080"),
		StateDir:           envOrDefault("AGENT_STATE_DIR", ".agent-state"),
		StateEncryptionKey: strings.TrimSpace(os.Getenv("AGENT_STATE_ENCRYPTION_KEY")),
		ShutdownTimeout:    durationOrDefault("AGENT_SHUTDOWN_TIMEOUT", 10*time.Second),
		HeartbeatInterval:  durationOrDefault("AGENT_HEARTBEAT_INTERVAL", 30*time.Second),
		HandshakeVersion:   envOrDefault("AGENT_HANDSHAKE_VERSION", "v0"),
		UseMockControl:     boolOrDefault("AGENT_USE_MOCK_CONTROL", true),
	}

	level, err := parseLogLevel(envOrDefault("AGENT_LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch c.RuntimeMode {
	case contracts.RuntimeModeStandalone, contracts.RuntimeModeDistributedMesh, contracts.RuntimeModeDistributedK3s:
	default:
		return fmt.Errorf("invalid runtime mode %q", c.RuntimeMode)
	}

	switch c.AgentKind {
	case contracts.AgentKindInstance, contracts.AgentKindNode:
	default:
		return fmt.Errorf("invalid agent kind %q", c.AgentKind)
	}

	switch c.AgentKind {
	case contracts.AgentKindInstance:
		if c.RuntimeMode == contracts.RuntimeModeDistributedK3s {
			return fmt.Errorf("instance_agent cannot run in %q", c.RuntimeMode)
		}
	case contracts.AgentKindNode:
		if c.RuntimeMode != contracts.RuntimeModeDistributedK3s {
			return fmt.Errorf("node_agent requires %q", contracts.RuntimeModeDistributedK3s)
		}
	}

	if strings.TrimSpace(c.ControlPlaneURL) == "" {
		return fmt.Errorf("control plane URL is required")
	}
	if strings.TrimSpace(c.TargetRef) == "" {
		return fmt.Errorf("target ref is required")
	}
	if strings.TrimSpace(c.StateDir) == "" {
		return fmt.Errorf("state dir is required")
	}
	if c.BootstrapToken != "" && strings.TrimSpace(c.StateEncryptionKey) == "" {
		return fmt.Errorf("state encryption key is required when bootstrap token is provided")
	}
	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat interval must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	if strings.TrimSpace(c.HandshakeVersion) == "" {
		return fmt.Errorf("handshake version is required")
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolOrDefault(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q", raw)
	}
}
