package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testContainerLogTailer() *ContainerLogTailer {
	tailer := NewContainerLogTailer(nil, ContainerLogTailerConfig{
		MaxTailLines:       5,
		MaxBufferSize:      64 * 1024,
		CollectionInterval: 1 * time.Second,
	})
	tailer.now = func() time.Time {
		return time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC)
	}
	return tailer
}

func TestContainerLogTailerDefaultConfig(t *testing.T) {
	tailer := NewContainerLogTailer(nil, ContainerLogTailerConfig{})
	if tailer.cfg.MaxTailLines != 100 {
		t.Fatalf("expected default max tail lines 100, got %d", tailer.cfg.MaxTailLines)
	}
	if tailer.cfg.MaxBufferSize != 64*1024 {
		t.Fatalf("expected default max buffer size 64KB, got %d", tailer.cfg.MaxBufferSize)
	}
	if tailer.cfg.CollectionInterval != 10*time.Second {
		t.Fatalf("expected default collection interval 10s, got %s", tailer.cfg.CollectionInterval)
	}
}

func TestContainerLogTailerTailFile(t *testing.T) {
	tailer := testContainerLogTailer()

	logFile := filepath.Join(t.TempDir(), "container.log")
	lines := []string{"2026-04-04T14:00:00Z INFO server started", "2026-04-04T14:00:01Z INFO request received"}
	if err := os.WriteFile(logFile, []byte(lines[0]+"\n"+lines[1]+"\n"), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	err := tailer.TailFile(context.Background(), "ctr_123", "pod_123", "default", logFile)
	if err != nil {
		t.Fatalf("tail file: %v", err)
	}

	total, dropped, pending := tailer.Stats()
	if total != 2 {
		t.Fatalf("expected 2 total entries, got %d", total)
	}
	if dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if pending != 2 {
		t.Fatalf("expected 2 pending, got %d", pending)
	}
}

func TestContainerLogTailerCollectEntries(t *testing.T) {
	tailer := testContainerLogTailer()

	logFile := filepath.Join(t.TempDir(), "container.log")
	if err := os.WriteFile(logFile, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	tailer.TailFile(context.Background(), "ctr_123", "pod_123", "default", logFile)

	entries := tailer.CollectEntries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].ContainerID != "ctr_123" {
		t.Fatalf("expected container_id ctr_123, got %q", entries[0].ContainerID)
	}
	if entries[0].PodName != "pod_123" {
		t.Fatalf("expected pod_name pod_123, got %q", entries[0].PodName)
	}
	if entries[0].Namespace != "default" {
		t.Fatalf("expected namespace default, got %q", entries[0].Namespace)
	}

	_, _, pending := tailer.Stats()
	if pending != 0 {
		t.Fatalf("expected 0 pending after collect, got %d", pending)
	}
}

func TestContainerLogTailerMaxTailLines(t *testing.T) {
	tailer := testContainerLogTailer()

	logFile := filepath.Join(t.TempDir(), "container.log")
	content := ""
	for i := 0; i < 10; i++ {
		content += fmt.Sprintf("line %d\n", i)
	}
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	tailer.TailFile(context.Background(), "ctr_123", "pod_123", "default", logFile)

	total, dropped, pending := tailer.Stats()
	if total != 10 {
		t.Fatalf("expected 10 total, got %d", total)
	}
	if dropped != 5 {
		t.Fatalf("expected 5 dropped (max 5), got %d", dropped)
	}
	if pending != 5 {
		t.Fatalf("expected 5 pending, got %d", pending)
	}
}

func TestContainerLogTailerFileNotFound(t *testing.T) {
	tailer := testContainerLogTailer()

	err := tailer.TailFile(context.Background(), "ctr_123", "pod_123", "default", "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestContainerLogTailerPersistLogs(t *testing.T) {
	tailer := testContainerLogTailer()

	logFile := filepath.Join(t.TempDir(), "container.log")
	if err := os.WriteFile(logFile, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	tailer.TailFile(context.Background(), "ctr_123", "pod_123", "default", logFile)
	entries := tailer.CollectEntries()

	root := filepath.Join(t.TempDir(), "runtime-root")
	logPath, err := tailer.PersistLogs(root, "prj_123", "bind_123", entries)
	if err != nil {
		t.Fatalf("persist logs: %v", err)
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}

	var loaded []ContainerLogEntry
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode log file: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries in persisted log, got %d", len(loaded))
	}
}

func TestContainerLogTailerPersistLogsEmpty(t *testing.T) {
	tailer := testContainerLogTailer()

	root := filepath.Join(t.TempDir(), "runtime-root")
	logPath, err := tailer.PersistLogs(root, "prj_123", "bind_123", nil)
	if err != nil {
		t.Fatalf("persist logs: %v", err)
	}
	if logPath != "" {
		t.Fatal("expected empty path for empty entries")
	}
}
