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
	"sync"
	"syscall"
	"time"
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
}

type ProcessManager struct {
	logger              *slog.Logger
	runtimeRoot         string
	healthCheckTimeout  time.Duration
	healthCheckAttempts int
	healthCheckInterval time.Duration
	now                 func() time.Time

	mu        sync.Mutex
	processes map[string]*ProcessInfo
	cmds      map[string]*exec.Cmd
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
		processes: make(map[string]*ProcessInfo),
		cmds:      make(map[string]*exec.Cmd),
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

	cmd := exec.CommandContext(ctx, "sleep", "3600")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		info.State = ProcessStateStopped
		return info, fmt.Errorf("failed to start process for %s: %w", serviceName, err)
	}

	m.cmds[serviceName] = cmd
	info.PID = cmd.Process.Pid
	info.State = ProcessStateRunning
	info.StartedAt = m.now()

	if m.logger != nil {
		m.logger.Info("sidecar process started",
			"service", serviceName,
			"pid", info.PID,
			"config", configPath,
		)
	}

	return info, nil
}

func (m *ProcessManager) StopProcess(serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.processes[serviceName]
	if !ok || info.State == ProcessStateStopped {
		return nil
	}

	info.State = ProcessStateStopping

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
