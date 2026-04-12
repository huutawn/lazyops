package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"lazyops-agent/internal/contracts"
)

type ProcessState string

const (
	ProcessStateStopped  ProcessState = "stopped"
	ProcessStateStarting ProcessState = "starting"
	ProcessStateRunning  ProcessState = "running"
	ProcessStateStopping ProcessState = "stopping"
)

type ProcessInfo struct {
	ServiceName string       `json:"service_name"`
	PID         int          `json:"pid"`
	State       ProcessState `json:"state"`
	StartedAt   time.Time    `json:"started_at,omitempty"`
	ConfigPath  string       `json:"config_path"`
	Runner      string       `json:"runner,omitempty"`
	Container   string       `json:"container,omitempty"`
}

type ProcessManager struct {
	logger              *slog.Logger
	runtimeRoot         string
	healthCheckTimeout  time.Duration
	healthCheckAttempts int
	healthCheckInterval time.Duration
	now                 func() time.Time

	mu              sync.Mutex
	processes       map[string]*ProcessInfo
	cmds            map[string]*exec.Cmd
	sidecarProxy    *SidecarProxy
	iptablesManager *IPTablesManager
}

type runtimeWorkloadConfig struct {
	Service       ServiceRuntimeContext `json:"service"`
	ArtifactRef   string                `json:"artifact_ref"`
	ImageRef      string                `json:"image_ref"`
	WorkspaceRoot string                `json:"workspace_root"`
}

func NewProcessManager(logger *slog.Logger, runtimeRoot string) *ProcessManager {
	return &ProcessManager{
		logger:              logger,
		runtimeRoot:         runtimeRoot,
		healthCheckTimeout:  10 * time.Second,
		healthCheckAttempts: 5,
		healthCheckInterval: 2 * time.Second,
		now: func() time.Time {
			return time.Now().UTC()
		},
		processes:       make(map[string]*ProcessInfo),
		cmds:            make(map[string]*exec.Cmd),
		sidecarProxy:    NewSidecarProxy(logger),
		iptablesManager: NewIPTablesManager(logger),
	}
}

func (m *ProcessManager) StartProcess(ctx context.Context, serviceName, configPath string) (*ProcessInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.processes[serviceName]; ok && existing.State == ProcessStateRunning {
		return existing, nil
	}

	info := &ProcessInfo{
		ServiceName: serviceName,
		State:       ProcessStateStarting,
		ConfigPath:  configPath,
	}
	m.processes[serviceName] = info

	if workloadCfg, ok, err := loadRuntimeWorkloadConfig(configPath); err != nil {
		info.State = ProcessStateStopped
		return info, fmt.Errorf("failed to decode runtime workload config for %s: %w", serviceName, err)
	} else if ok {
		containerName, startErr := m.startContainerWorkload(ctx, workloadCfg)
		if startErr != nil {
			info.State = ProcessStateStopped
			return info, fmt.Errorf("failed to start workload container for %s: %w", serviceName, startErr)
		}
		info.PID = 0
		info.State = ProcessStateRunning
		info.StartedAt = m.now()
		info.Runner = "docker"
		info.Container = containerName

		if m.logger != nil {
			m.logger.Info("service workload container started",
				"service", serviceName,
				"container", containerName,
				"config", configPath,
			)
		}
		return info, nil
	}

	cmd := exec.CommandContext(ctx, "sleep", "86400")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		info.State = ProcessStateStopped
		return info, fmt.Errorf("failed to start process for %s: %w", serviceName, err)
	}

	m.cmds[serviceName] = cmd
	info.PID = cmd.Process.Pid
	info.State = ProcessStateRunning
	info.StartedAt = m.now()
	info.Runner = "process"

	if m.logger != nil {
		m.logger.Info("sidecar process started",
			"service", serviceName,
			"pid", info.PID,
			"config", configPath,
		)
	}

	return info, nil
}

// StartProxyProcess starts a real sidecar proxy for the given route.
// This replaces the placeholder sleep command with actual TCP/HTTP proxying.
func (m *ProcessManager) StartProxyProcess(ctx context.Context, route SidecarProxyRoute) (*ProcessInfo, error) {
	if m.sidecarProxy == nil {
		return nil, fmt.Errorf("sidecar proxy not initialized")
	}

	if err := m.sidecarProxy.StartRoute(ctx, route); err != nil {
		return nil, fmt.Errorf("failed to start sidecar proxy for %s: %w", route.Alias, err)
	}

	// If localhost_rescue or transparent_proxy with iptables intercept, set up DNAT rule
	if (route.LocalhostRescue || route.ForwardingMode == "transparent") && route.NetworkNamespace && m.iptablesManager != nil {
		if err := m.iptablesManager.EnsureChain(); err != nil {
			m.logger.Warn("failed to ensure iptables chain, continuing without DNAT",
				"alias", route.Alias,
				"error", err,
			)
		} else {
			// Redirect the original port to the proxy port
			originalPort := route.OriginalPort
			if originalPort == 0 {
				// Fallback: use listener port for backwards compatibility
				originalPort = route.ListenerPort
			}
			iprule := IPTablesRule{
				Alias:        route.Alias,
				Protocol:     route.Protocol,
				OriginalPort: originalPort,
				RedirectPort: route.ListenerPort,
				Comment:      fmt.Sprintf("lazyops-sidecar-%s", route.Alias),
			}
			if err := m.iptablesManager.AddDNATRule(iprule); err != nil {
				m.logger.Warn("failed to add iptables DNAT rule, proxy still active via direct port",
					"alias", route.Alias,
					"error", err,
				)
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	info := &ProcessInfo{
		ServiceName: route.Alias,
		PID:         0, // In-process proxy, no external PID
		State:       ProcessStateRunning,
		StartedAt:   m.now(),
		ConfigPath:  fmt.Sprintf("%s:%d→%s", route.ListenerHost, route.ListenerPort, route.Upstream),
	}
	m.processes[route.Alias] = info

	if m.logger != nil {
		m.logger.Info("sidecar proxy process started",
			"alias", route.Alias,
			"target_service", route.TargetService,
			"protocol", route.Protocol,
			"listen", fmt.Sprintf("%s:%d", route.ListenerHost, route.ListenerPort),
			"upstream", route.Upstream,
			"forwarding_mode", route.ForwardingMode,
			"localhost_rescue", route.LocalhostRescue,
			"network_namespace", route.NetworkNamespace,
		)
	}

	return info, nil
}

// StopProxyProcess stops a sidecar proxy route and removes associated iptables rules.
func (m *ProcessManager) StopProxyProcess(alias string) error {
	if m.sidecarProxy != nil {
		_ = m.sidecarProxy.StopRoute(alias)
	}
	if m.iptablesManager != nil {
		for _, rule := range m.iptablesManager.ListRules() {
			if rule.Alias == alias {
				_ = m.iptablesManager.RemoveDNATRule(rule)
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if info, ok := m.processes[alias]; ok {
		info.State = ProcessStateStopped
	}

	return nil
}

// GetSidecarProxy returns the underlying SidecarProxy instance.
func (m *ProcessManager) GetSidecarProxy() *SidecarProxy {
	return m.sidecarProxy
}

// GetIPTablesManager returns the underlying IPTablesManager instance.
func (m *ProcessManager) GetIPTablesManager() *IPTablesManager {
	return m.iptablesManager
}

func (m *ProcessManager) StopProcess(serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.processes[serviceName]
	if !ok || info.State == ProcessStateStopped {
		return nil
	}

	info.State = ProcessStateStopping

	if strings.TrimSpace(info.Container) != "" {
		if _, err := m.runDockerCommand(context.Background(), "rm", "-f", info.Container); err != nil {
			lowered := strings.ToLower(err.Error())
			if !strings.Contains(lowered, "no such container") {
				return fmt.Errorf("failed to stop workload container for %s: %w", serviceName, err)
			}
		}
		info.State = ProcessStateStopped
		info.Container = ""
		delete(m.cmds, serviceName)
		if m.logger != nil {
			m.logger.Info("service workload container stopped",
				"service", serviceName,
			)
		}
		return nil
	}

	cmd, cmdOk := m.cmds[serviceName]
	if cmdOk && cmd.Process != nil {
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
		} else {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}

	info.State = ProcessStateStopped
	delete(m.cmds, serviceName)

	if m.logger != nil {
		m.logger.Info("sidecar process stopped",
			"service", serviceName,
			"pid", info.PID,
		)
	}

	return nil
}

func (m *ProcessManager) RestartProcess(ctx context.Context, serviceName, configPath string) (*ProcessInfo, error) {
	if err := m.StopProcess(serviceName); err != nil {
		return nil, fmt.Errorf("failed to stop process for restart: %w", err)
	}

	info, err := m.StartProcess(ctx, serviceName, configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start process for restart: %w", err)
	}

	if m.logger != nil {
		m.logger.Info("sidecar process restarted",
			"service", serviceName,
			"pid", info.PID,
		)
	}

	return info, nil
}

func (m *ProcessManager) GetProcess(serviceName string) (*ProcessInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.processes[serviceName]
	return info, ok
}

func (m *ProcessManager) HealthCheck(ctx context.Context, serviceName string, port int) error {
	info, ok := m.GetProcess(serviceName)
	if !ok || info.State != ProcessStateRunning {
		return fmt.Errorf("process %s is not running", serviceName)
	}

	for i := 0; i < m.healthCheckAttempts; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check cancelled for %s", serviceName)
		default:
		}

		if m.checkPort(port) {
			if m.logger != nil {
				m.logger.Info("sidecar process healthy",
					"service", serviceName,
					"port", port,
					"attempt", i+1,
				)
			}
			return nil
		}

		time.Sleep(m.healthCheckInterval)
	}

	return fmt.Errorf("health check failed for %s after %d attempts on port %d", serviceName, m.healthCheckAttempts, port)
}

func (m *ProcessManager) checkPort(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (m *ProcessManager) StopAll() {
	m.mu.Lock()
	services := make([]string, 0, len(m.processes))
	for name := range m.processes {
		services = append(services, name)
	}
	m.mu.Unlock()

	for _, name := range services {
		_ = m.StopProcess(name)
	}

	// Also stop all sidecar proxy routes and clean up iptables
	if m.sidecarProxy != nil {
		m.sidecarProxy.StopAll()
	}
	if m.iptablesManager != nil {
		_ = m.iptablesManager.CleanupAll()
	}
}

func (m *ProcessManager) Stats() (total, active, stopped int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, info := range m.processes {
		total++
		switch info.State {
		case ProcessStateRunning, ProcessStateStarting:
			active++
		case ProcessStateStopped, ProcessStateStopping:
			stopped++
		}
	}
	return total, active, stopped
}

func (m *ProcessManager) PersistProcessState(workspaceRoot, projectID, bindingID string) (string, error) {
	processDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "processes")
	if err := os.MkdirAll(processDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create process state directory: %w", err)
	}

	timestamp := m.now().Format("20060102T150405Z")
	statePath := filepath.Join(processDir, "state_"+timestamp+".json")

	m.mu.Lock()
	processes := make([]*ProcessInfo, 0, len(m.processes))
	for _, info := range m.processes {
		processes = append(processes, info)
	}
	m.mu.Unlock()

	raw, err := json.MarshalIndent(processes, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal process states: %w", err)
	}

	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write process states: %w", err)
	}

	return statePath, nil
}

func (m *ProcessManager) CleanupStoppedProcesses() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleaned := 0
	for name, info := range m.processes {
		if info.State == ProcessStateStopped {
			delete(m.processes, name)
			cleaned++
		}
	}
	return cleaned
}

func loadRuntimeWorkloadConfig(configPath string) (runtimeWorkloadConfig, bool, error) {
	payload, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return runtimeWorkloadConfig{}, false, nil
		}
		return runtimeWorkloadConfig{}, false, err
	}

	var cfg runtimeWorkloadConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		// Sidecar configs and other files are not runtime workload configs.
		return runtimeWorkloadConfig{}, false, nil
	}
	if strings.TrimSpace(cfg.ImageRef) == "" {
		return runtimeWorkloadConfig{}, false, nil
	}
	if strings.TrimSpace(cfg.Service.Name) == "" {
		return runtimeWorkloadConfig{}, false, fmt.Errorf("runtime workload config missing service.name")
	}
	return cfg, true, nil
}

func (m *ProcessManager) startContainerWorkload(ctx context.Context, cfg runtimeWorkloadConfig) (string, error) {
	projectID, bindingID, revisionID := parseWorkspaceIdentity(cfg.WorkspaceRoot)
	containerName := workloadContainerName(projectID, bindingID, cfg.Service.Name)

	// Replace in-place to avoid stale candidate containers hanging around.
	if _, err := m.runDockerCommand(ctx, "rm", "-f", containerName); err != nil {
		lowered := strings.ToLower(err.Error())
		if !strings.Contains(lowered, "no such container") {
			return "", err
		}
	}

	port := cfg.Service.HealthCheck.Port
	if port <= 0 {
		port = 8080
	}

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"--network", "host",
		"--label", "lazyops.managed=app-service",
		"--label", "lazyops.service=" + cfg.Service.Name,
	}
	if projectID != "" {
		args = append(args, "--label", "lazyops.project_id="+projectID)
	}
	if bindingID != "" {
		args = append(args, "--label", "lazyops.binding_id="+bindingID)
	}
	if revisionID != "" {
		args = append(args, "--label", "lazyops.revision_id="+revisionID)
	}

	args = append(args, "-e", "PORT="+strconv.Itoa(port))
	for _, envVar := range dependencyEnvVars(cfg.Service.Dependencies) {
		args = append(args, "-e", envVar)
	}

	args = append(args, cfg.ImageRef)
	if _, err := m.runDockerCommand(ctx, args...); err != nil {
		return "", err
	}
	return containerName, nil
}

func dependencyEnvVars(deps []contracts.DependencyBindingPayload) []string {
	if len(deps) == 0 {
		return nil
	}

	env := make(map[string]string, len(deps)*4)
	for _, dep := range deps {
		alias := strings.TrimSpace(dep.Alias)
		if alias == "" {
			continue
		}
		host, port := splitHostPort(dep.LocalEndpoint)
		if host == "" || port == "" {
			continue
		}

		key := strings.ToUpper(strings.ReplaceAll(alias, "-", "_"))
		env[key+"_HOST"] = host
		env[key+"_PORT"] = port
		env[key+"_URL"] = dependencyURL(dep.Protocol, host, port)

		if strings.EqualFold(alias, "postgres") {
			env["DB_HOST"] = host
			env["DB_PORT"] = port
			if _, ok := env["DB_USER"]; !ok {
				env["DB_USER"] = "lazyops"
			}
			if _, ok := env["DB_PASSWORD"]; !ok {
				env["DB_PASSWORD"] = "lazyops"
			}
			if _, ok := env["DB_NAME"]; !ok {
				env["DB_NAME"] = "app"
			}
		}
	}

	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}

func splitHostPort(endpoint string) (string, string) {
	value := strings.TrimSpace(endpoint)
	if value == "" {
		return "", ""
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		parts := strings.Split(value, ":")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
		return "", ""
	}
	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	return strings.TrimSpace(host), strings.TrimSpace(port)
}

func dependencyURL(protocol, host, port string) string {
	scheme := strings.ToLower(strings.TrimSpace(protocol))
	if scheme == "" {
		scheme = "tcp"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

func parseWorkspaceIdentity(workspaceRoot string) (string, string, string) {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(workspaceRoot)))
	if cleaned == "." || cleaned == "" {
		return "", "", ""
	}
	parts := strings.Split(cleaned, "/")
	for i := 0; i+5 < len(parts); i++ {
		if parts[i] == "projects" && parts[i+2] == "bindings" && parts[i+4] == "revisions" {
			return parts[i+1], parts[i+3], parts[i+5]
		}
	}
	return "", "", ""
}

func workloadContainerName(projectID, bindingID, serviceName string) string {
	name := fmt.Sprintf("lazyops-app-%s-%s-%s",
		normalizeContainerToken(projectID),
		normalizeContainerToken(bindingID),
		normalizeContainerToken(serviceName),
	)
	if len(name) > 63 {
		return name[:63]
	}
	return name
}

func (m *ProcessManager) runDockerCommand(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		return text, fmt.Errorf("docker %s failed: %s: %w", strings.Join(args, " "), text, err)
	}
	return text, nil
}
