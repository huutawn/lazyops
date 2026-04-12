package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lazyops-agent/internal/config"
	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/control"
	"lazyops-agent/internal/dispatcher"
	"lazyops-agent/internal/enroll"
	agentlogger "lazyops-agent/internal/logger"
	"lazyops-agent/internal/reporting"
	agentruntime "lazyops-agent/internal/runtime"
	"lazyops-agent/internal/state"
)

type App struct {
	cfg        config.Config
	logger     *slog.Logger
	store      *state.Store
	control    control.Client
	dispatcher *dispatcher.CommandDispatcher
	enroll     *enroll.Service
	reporter   *reporting.Reporter
}

func New(cfg config.Config) (*App, error) {
	logger := agentlogger.New(cfg.LogLevel)

	var client control.Client
	if cfg.UseMockControl {
		client = control.NewMockClient(logger)
	} else {
		client = control.NewWebSocketClient(logger, control.WebSocketClientConfig{
			ControlPlaneURL:     cfg.ControlPlaneURL,
			DialTimeout:         cfg.ControlDialTimeout,
			WriteTimeout:        cfg.ControlWriteTimeout,
			PongWait:            cfg.ControlPongWait,
			PingPeriod:          cfg.ControlPingPeriod,
			ReconnectMinBackoff: cfg.ReconnectMinBackoff,
			ReconnectMaxBackoff: cfg.ReconnectMaxBackoff,
			ReconnectJitter:     cfg.ReconnectJitter,
		})
	}

	statePath := cfg.StateDir
	if !strings.HasSuffix(statePath, ".json") {
		statePath = strings.TrimRight(statePath, "/") + "/agent-state.json"
	}
	runtimeRoot := strings.TrimSpace(cfg.RuntimeRootDir)
	if runtimeRoot == "" {
		runtimeRoot = filepath.Join(filepath.Dir(statePath), "runtime")
	}

	store := state.New(statePath)
	runtimeDriver := agentruntime.NewFilesystemDriver(logger, runtimeRoot)
	runtimeService := agentruntime.NewService(logger, store, runtimeDriver)
	metricAggregator := agentruntime.NewMetricAggregator(logger, agentruntime.DefaultMetricAggregatorConfig())
	nodeMetricsCollector := agentruntime.NewNodeMetricsCollector(logger, agentruntime.DefaultNodeMetricsConfig())
	runtimeService.WithMetricAggregator(metricAggregator)
	runtimeService.WithNodeMetrics(nodeMetricsCollector)
	runtimeService.WithMetricSender(client)
	topologyReporter := agentruntime.NewTopologyReporter(logger, agentruntime.DefaultTopologyReporterConfig())
	runtimeService.WithTopologyReporter(topologyReporter)
	meshService := agentruntime.NewMeshService(logger, store, agentruntime.NewMeshManager(logger, runtimeRoot))

	registry := dispatcher.NewDefaultRegistry()
	runtimeService.Register(registry)
	meshService.Register(registry)
	commandDispatcher := dispatcher.New(logger, registry, client)
	client.RegisterCommandHandler(commandDispatcher.Handler())

	return &App{
		cfg:        cfg,
		logger:     logger,
		store:      store,
		control:    client,
		dispatcher: commandDispatcher,
		enroll:     enroll.New(store, client, logger, cfg.StateEncryptionKey),
		reporter:   reporting.New(logger, cfg.HeartbeatInterval),
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
	if machine.Labels == nil {
		machine.Labels = make(map[string]string)
	}
	machine.Labels["target_ref"] = a.cfg.TargetRef

	if err := a.bootstrapLocalState(ctx, current, machine); err != nil {
		return err
	}

	current, err = a.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("reload local state after bootstrap: %w", err)
	}

	if !current.Enrollment.Enrolled && strings.TrimSpace(a.cfg.BootstrapToken) != "" {
		if _, err := a.enroll.Enroll(
			ctx,
			a.cfg.BootstrapToken,
			machine,
			current.CapabilitySnapshot.Payload,
			a.cfg.RuntimeMode,
			a.cfg.AgentKind,
		); err != nil {
			return fmt.Errorf("enroll agent: %w", err)
		}
		current, err = a.store.Load(ctx)
		if err != nil {
			return fmt.Errorf("reload local state after enrollment: %w", err)
		}
	}

	sessionAuth, err := a.sessionAuthFromState(ctx, current)
	if err != nil {
		return err
	}
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
		local.Health = a.reporter.EvaluateHealth(local)
		a.reporter.MarkCapabilitiesReported(&local.CapabilitySnapshot)
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
			current, err := a.store.Update(ctx, func(local *state.AgentLocalState) error {
				if err := a.reporter.ReconcileCapabilitySnapshot(&local.CapabilitySnapshot, defaultCapabilities(a.cfg)); err != nil {
					return err
				}
				local.Health = a.reporter.EvaluateHealth(local)
				return nil
			})
			if err != nil {
				return fmt.Errorf("refresh state before heartbeat: %w", err)
			}

			heartbeat, health, err := a.reporter.BuildHeartbeat(current)
			if err != nil {
				return fmt.Errorf("build heartbeat: %w", err)
			}

			if err := a.control.SendHeartbeat(ctx, heartbeat); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
			}

			if _, err := a.store.Update(ctx, func(local *state.AgentLocalState) error {
				local.Health = health
				a.reporter.MarkCapabilitiesReported(&local.CapabilitySnapshot)
				return nil
			}); err != nil {
				return fmt.Errorf("persist heartbeat reporting state: %w", err)
			}

			a.logger.Debug("heartbeat sent",
				"agent_id", current.Metadata.AgentID,
				"session_id", current.Enrollment.SessionID,
				"state", current.Metadata.CurrentState,
				"health_status", health.Status,
				"capability_hash", current.CapabilitySnapshot.Fingerprint,
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
		local.Health = a.reporter.EvaluateHealth(local)
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
		if err := a.reporter.ReconcileCapabilitySnapshot(&local.CapabilitySnapshot, capabilities); err != nil {
			return err
		}
		local.Health = a.reporter.EvaluateHealth(local)
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

func (a *App) sessionAuthFromState(ctx context.Context, local *state.AgentLocalState) (contracts.SessionAuthPayload, error) {
	agentToken := "mock-agent-token"
	if local.Enrollment.Enrolled {
		decrypted, err := a.enroll.LoadAgentToken(ctx)
		if err != nil {
			return contracts.SessionAuthPayload{}, fmt.Errorf("load enrolled agent token: %w", err)
		}
		agentToken = decrypted
	}

	sessionID := strings.TrimSpace(local.Enrollment.SessionID)
	if sessionID == "" {
		sessionID = randomID("sess")
	}

	return contracts.SessionAuthPayload{
		AgentID:      local.Metadata.AgentID,
		AgentToken:   agentToken,
		SessionID:    sessionID,
		RuntimeMode:  a.cfg.RuntimeMode,
		AgentKind:    a.cfg.AgentKind,
		HandshakeVer: a.cfg.HandshakeVersion,
		SentAt:       time.Now().UTC(),
	}, nil
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
		Network: contracts.NetworkCapability{
			OutboundOnly:            true,
			PrivateOverlay:          meshEnabled,
			CrossNodePrivateOnly:    meshEnabled,
			SupportedMeshProviders:  []contracts.MeshProvider{contracts.MeshProviderWireGuard, contracts.MeshProviderTailscale},
			SupportsLocalhostRescue: cfg.AgentKind == contracts.AgentKindInstance,
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
