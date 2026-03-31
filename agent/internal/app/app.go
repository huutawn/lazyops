package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"lazyops-agent/internal/config"
	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/control"
	agentlogger "lazyops-agent/internal/logger"
	"lazyops-agent/internal/state"
)

type App struct {
	cfg     config.Config
	logger  *slog.Logger
	store   *state.Store
	control control.Client
}

func New(cfg config.Config) (*App, error) {
	logger := agentlogger.New(cfg.LogLevel)

	var client control.Client
	if cfg.UseMockControl {
		client = control.NewMockClient(logger)
	} else {
		return nil, fmt.Errorf("real control-plane client is not implemented yet")
	}

	statePath := cfg.StateDir
	if !strings.HasSuffix(statePath, ".json") {
		statePath = strings.TrimRight(statePath, "/") + "/agent-state.json"
	}

	return &App{
		cfg:     cfg,
		logger:  logger,
		store:   state.New(statePath),
		control: client,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.logger.Info("starting agent bootstrap shell",
		"app_name", a.cfg.AppName,
		"app_env", a.cfg.AppEnv,
		"runtime_mode", a.cfg.RuntimeMode,
		"agent_kind", a.cfg.AgentKind,
		"state_path", a.store.Path(),
		"use_mock_control", a.cfg.UseMockControl,
	)

	current, err := a.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("load local state: %w", err)
	}

	machine, err := currentMachineInfo()
	if err != nil {
		return fmt.Errorf("collect machine info: %w", err)
	}

	if err := a.bootstrapLocalState(ctx, current, machine); err != nil {
		return err
	}

	current, err = a.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("reload local state after bootstrap: %w", err)
	}

	sessionAuth := a.sessionAuthFromState(current)
	if err := a.control.Connect(ctx, sessionAuth); err != nil {
		return fmt.Errorf("connect control session: %w", err)
	}

	handshake := contracts.AgentHandshakePayload{
		Auth:         sessionAuth,
		Machine:      machine,
		State:        contracts.AgentStateConnected,
		Capabilities: current.CapabilitySnapshot.Payload,
	}
	if err := a.control.SendHandshake(ctx, handshake); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}

	if _, err := a.store.Update(ctx, func(local *state.AgentLocalState) error {
		local.Metadata.CurrentState = contracts.AgentStateConnected
		local.Enrollment.SessionID = sessionAuth.SessionID
		local.Enrollment.LastBootstrapAt = time.Now().UTC()
		return nil
	}); err != nil {
		return fmt.Errorf("persist connected state: %w", err)
	}

	heartbeatTicker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return a.shutdown()
		case <-heartbeatTicker.C:
			current, err := a.store.Load(ctx)
			if err != nil {
				return fmt.Errorf("load state before heartbeat: %w", err)
			}
			if err := a.control.SendHeartbeat(ctx, contracts.HeartbeatPayload{
				AgentID:       current.Metadata.AgentID,
				SessionID:     current.Enrollment.SessionID,
				State:         current.Metadata.CurrentState,
				RuntimeMode:   current.Metadata.RuntimeMode,
				AgentKind:     current.Metadata.AgentKind,
				SentAt:        time.Now().UTC(),
				UptimeSeconds: int64(time.Since(current.Metadata.LastStartedAt).Seconds()),
				Capabilities:  current.CapabilitySnapshot.Payload,
			}); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
			}
			a.logger.Debug("heartbeat sent",
				"agent_id", current.Metadata.AgentID,
				"session_id", current.Enrollment.SessionID,
				"state", current.Metadata.CurrentState,
			)
		}
	}
}

func (a *App) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	if _, err := a.store.Update(shutdownCtx, func(local *state.AgentLocalState) error {
		local.Metadata.CurrentState = contracts.AgentStateDisconnected
		local.Metadata.LastStoppedAt = time.Now().UTC()
		return nil
	}); err != nil {
		return fmt.Errorf("persist shutdown state: %w", err)
	}

	if err := a.control.Close(shutdownCtx); err != nil {
		return fmt.Errorf("close control session: %w", err)
	}

	a.logger.Info("agent shutdown complete", "state_path", a.store.Path())
	return nil
}

func (a *App) bootstrapLocalState(ctx context.Context, current *state.AgentLocalState, machine contracts.MachineInfo) error {
	capabilities := defaultCapabilities(a.cfg)
	agentID := current.Metadata.AgentID
	if agentID == "" {
		agentID = localAgentID(machine.Hostname)
	}

	_, err := a.store.Update(ctx, func(local *state.AgentLocalState) error {
		local.Metadata.AgentID = agentID
		local.Metadata.Hostname = machine.Hostname
		local.Metadata.AgentKind = a.cfg.AgentKind
		local.Metadata.RuntimeMode = a.cfg.RuntimeMode
		local.Metadata.CurrentState = contracts.AgentStateBootstrap
		local.Metadata.LastStartedAt = time.Now().UTC()
		local.CapabilitySnapshot.LastComputedAt = time.Now().UTC()
		local.CapabilitySnapshot.Payload = capabilities
		return nil
	})
	if err != nil {
		return fmt.Errorf("persist bootstrap state: %w", err)
	}

	a.logger.Info("bootstrap state prepared",
		"agent_id", agentID,
		"hostname", machine.Hostname,
		"runtime_mode", a.cfg.RuntimeMode,
		"agent_kind", a.cfg.AgentKind,
	)
	return nil
}

func (a *App) sessionAuthFromState(local *state.AgentLocalState) contracts.SessionAuthPayload {
	return contracts.SessionAuthPayload{
		AgentID:      local.Metadata.AgentID,
		AgentToken:   "mock-agent-token",
		SessionID:    randomID("sess"),
		RuntimeMode:  a.cfg.RuntimeMode,
		AgentKind:    a.cfg.AgentKind,
		HandshakeVer: a.cfg.HandshakeVersion,
		SentAt:       time.Now().UTC(),
	}
}

func defaultCapabilities(cfg config.Config) contracts.CapabilityReportPayload {
	meshEnabled := cfg.RuntimeMode == contracts.RuntimeModeDistributedMesh || cfg.RuntimeMode == contracts.RuntimeModeDistributedK3s
	nodeEnabled := cfg.AgentKind == contracts.AgentKindNode

	return contracts.CapabilityReportPayload{
		AgentKind:   cfg.AgentKind,
		RuntimeMode: cfg.RuntimeMode,
		ControlChannel: contracts.ControlChannelCapability{
			WebSocketPath: contracts.ControlWebSocketPath,
			OutboundOnly:  true,
			Reconnectable: true,
		},
		Gateway: contracts.GatewayCapability{
			Enabled:      cfg.AgentKind == contracts.AgentKindInstance,
			Provider:     "caddy",
			MagicDomains: []string{"sslip.io", "nip.io"},
			HTTPSManaged: true,
		},
		Sidecar: contracts.SidecarCapability{
			Enabled:                   cfg.AgentKind == contracts.AgentKindInstance,
			Precedence:                []string{"env_injection", "managed_credentials", "localhost_rescue"},
			SupportsHTTP:              true,
			SupportsTCP:               true,
			SupportsLocalhostRescue:   true,
			SupportsManagedCredential: true,
		},
		Mesh: contracts.MeshCapability{
			Enabled:                  meshEnabled,
			DefaultProvider:          contracts.MeshProviderWireGuard,
			SupportedProviders:       []contracts.MeshProvider{contracts.MeshProviderWireGuard, contracts.MeshProviderTailscale},
			DeterministicPeerCleanup: true,
		},
		Telemetry: contracts.TelemetryCapability{
			LogCollection:     true,
			MetricRollup:      true,
			TraceSummary:      true,
			TopologyReporting: true,
			IncidentReporting: true,
			TunnelRelay:       true,
		},
		Node: contracts.NodeCapability{
			K3sDetection:        nodeEnabled,
			DaemonSetBootstrap:  nodeEnabled,
			ContainerLogTailing: nodeEnabled,
			NodeMetrics:         true,
			PodTopology:         nodeEnabled,
		},
		PerformanceTargets: contracts.PerformanceTargets{
			IdleRAMMB:       20,
			IdleCPUPercent:  2,
			BufferPooling:   true,
			LowAllocHotPath: true,
		},
	}
}

func currentMachineInfo() (contracts.MachineInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return contracts.MachineInfo{}, err
	}

	info := contracts.MachineInfo{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}

	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return info, nil
	}
	for _, addr := range addresses {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ipNet.IP.IsLoopback() {
			continue
		}
		if ip := ipNet.IP.String(); ip != "" {
			info.IPs = append(info.IPs, ip)
		}
	}
	return info, nil
}

func localAgentID(hostname string) string {
	sanitized := strings.ToLower(strings.TrimSpace(hostname))
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	if sanitized == "" {
		sanitized = "local"
	}
	return "agt_local_" + sanitized
}

func randomID(prefix string) string {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return prefix + "_" + fmt.Sprint(time.Now().UTC().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(buffer[:])
}
