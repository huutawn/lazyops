package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessManagerStartProcess(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	info, err := pm.StartProcess(context.Background(), "web", filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	if info.PID <= 0 {
		t.Fatal("expected positive PID")
	}
	if info.State != ProcessStateRunning {
		t.Fatalf("expected running state, got %s", info.State)
	}
	if info.StartedAt.IsZero() {
		t.Fatal("expected non-zero started_at")
	}
}

func TestProcessManagerStartProcessAlreadyRunning(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	info1, err := pm.StartProcess(context.Background(), "web", filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatalf("start process: %v", err)
	}

	info2, err := pm.StartProcess(context.Background(), "web", filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatalf("start process again: %v", err)
	}
	if info1.PID != info2.PID {
		t.Fatal("expected same PID for already running process")
	}
}

func TestProcessManagerStopProcess(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	_, err := pm.StartProcess(context.Background(), "web", filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatalf("start process: %v", err)
	}

	if err := pm.StopProcess("web"); err != nil {
		t.Fatalf("stop process: %v", err)
	}

	info, ok := pm.GetProcess("web")
	if !ok {
		t.Fatal("expected process info to exist")
	}
	if info.State != ProcessStateStopped {
		t.Fatalf("expected stopped state, got %s", info.State)
	}
}

func TestProcessManagerStopNonExistentProcess(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	if err := pm.StopProcess("nonexistent"); err != nil {
		t.Fatalf("stop non-existent process should not error: %v", err)
	}
}

func TestProcessManagerRestartProcess(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	info1, err := pm.StartProcess(context.Background(), "web", filepath.Join(root, "config1.json"))
	if err != nil {
		t.Fatalf("start process: %v", err)
	}

	info2, err := pm.RestartProcess(context.Background(), "web", filepath.Join(root, "config2.json"))
	if err != nil {
		t.Fatalf("restart process: %v", err)
	}
	if info2.PID <= 0 {
		t.Fatal("expected positive PID after restart")
	}
	if info2.PID == info1.PID {
		t.Fatal("expected different PID after restart")
	}
	if info2.State != ProcessStateRunning {
		t.Fatalf("expected running state after restart, got %s", info2.State)
	}
}

func TestProcessManagerHealthCheck(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	pm.healthCheckAttempts = 1
	pm.healthCheckInterval = 100 * time.Millisecond

	err := pm.HealthCheck(context.Background(), "web", 0)
	if err != nil {
		t.Logf("health check error (expected since no listener): %v", err)
	}
}

func TestProcessManagerHealthCheckNotRunning(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	err := pm.HealthCheck(context.Background(), "nonexistent", 8080)
	if err == nil {
		t.Fatal("expected error for non-running process")
	}
}

func TestProcessManagerStopAll(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	_, _ = pm.StartProcess(context.Background(), "web", filepath.Join(root, "web.json"))
	_, _ = pm.StartProcess(context.Background(), "api", filepath.Join(root, "api.json"))

	pm.StopAll()

	_, active, _ := pm.Stats()
	if active != 0 {
		t.Fatalf("expected 0 active processes after stop all, got %d", active)
	}
}

func TestProcessManagerCleanupStoppedProcesses(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	_, _ = pm.StartProcess(context.Background(), "web", filepath.Join(root, "web.json"))
	_ = pm.StopProcess("web")

	cleaned := pm.CleanupStoppedProcesses()
	if cleaned != 1 {
		t.Fatalf("expected 1 cleaned process, got %d", cleaned)
	}

	_, ok := pm.GetProcess("web")
	if ok {
		t.Fatal("expected process to be cleaned up")
	}
}

func TestProcessManagerPersistProcessState(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	_, _ = pm.StartProcess(context.Background(), "web", filepath.Join(root, "config.json"))

	path, err := pm.PersistProcessState(root, "prj_1", "bind_1")
	if err != nil {
		t.Fatalf("persist process state: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}
}

func TestProcessManagerStats(t *testing.T) {
	root := t.TempDir()
	pm := NewProcessManager(nil, root)

	_, _ = pm.StartProcess(context.Background(), "web", filepath.Join(root, "web.json"))
	_, _ = pm.StartProcess(context.Background(), "api", filepath.Join(root, "api.json"))
	_ = pm.StopProcess("web")

	total, active, _ := pm.Stats()
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if active != 1 {
		t.Fatalf("expected active 1, got %d", active)
	}
}
