package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ContainerLogTailerConfig struct {
	MaxTailLines       int
	MaxBufferSize      int
	CollectionInterval time.Duration
}

func DefaultContainerLogTailerConfig() ContainerLogTailerConfig {
	return ContainerLogTailerConfig{
		MaxTailLines:       100,
		MaxBufferSize:      64 * 1024,
		CollectionInterval: 10 * time.Second,
	}
}

type ContainerLogEntry struct {
	ContainerID string    `json:"container_id"`
	PodName     string    `json:"pod_name"`
	Namespace   string    `json:"namespace"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	Stream      string    `json:"stream"`
}

type ContainerLogTailer struct {
	logger *slog.Logger
	cfg    ContainerLogTailerConfig
	now    func() time.Time

	mu      sync.Mutex
	entries []ContainerLogEntry
	total   int
	dropped int
}

func NewContainerLogTailer(logger *slog.Logger, cfg ContainerLogTailerConfig) *ContainerLogTailer {
	if cfg.MaxTailLines <= 0 {
		cfg.MaxTailLines = 100
	}
	if cfg.MaxBufferSize <= 0 {
		cfg.MaxBufferSize = 64 * 1024
	}
	if cfg.CollectionInterval <= 0 {
		cfg.CollectionInterval = 10 * time.Second
	}

	return &ContainerLogTailer{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (t *ContainerLogTailer) TailFile(ctx context.Context, containerID, podName, namespace, logPath string) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("could not open log file %s: %w", logPath, err)
	}
	defer file.Close()

	return t.tailReader(ctx, containerID, podName, namespace, file)
}

func (t *ContainerLogTailer) tailReader(ctx context.Context, containerID, podName, namespace string, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, t.cfg.MaxBufferSize), t.cfg.MaxBufferSize)

	count := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		count++
		t.mu.Lock()
		t.total++
		if count > t.cfg.MaxTailLines {
			t.dropped++
			t.mu.Unlock()
			continue
		}
		t.entries = append(t.entries, ContainerLogEntry{
			ContainerID: containerID,
			PodName:     podName,
			Namespace:   namespace,
			Timestamp:   t.now(),
			Message:     line,
			Stream:      "stdout",
		})
		t.mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func (t *ContainerLogTailer) CollectEntries() []ContainerLogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries := make([]ContainerLogEntry, len(t.entries))
	copy(entries, t.entries)
	t.entries = t.entries[:0]

	return entries
}

func (t *ContainerLogTailer) Stats() (total, dropped, pending int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total, t.dropped, len(t.entries)
}

func (t *ContainerLogTailer) PersistLogs(workspaceRoot, projectID, bindingID string, entries []ContainerLogEntry) (string, error) {
	if len(entries) == 0 {
		return "", nil
	}

	logDir := filepath.Join(workspaceRoot, "node-agent", "projects", projectID, "bindings", bindingID, "container-logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create container log directory: %w", err)
	}

	timestamp := t.now().Format("20060102T150405Z")
	logPath := filepath.Join(logDir, "tail_"+timestamp+".json")

	raw, err := marshalJSON(entries)
	if err != nil {
		return "", fmt.Errorf("could not marshal container log entries: %w", err)
	}

	if err := os.WriteFile(logPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write container log entries: %w", err)
	}

	return logPath, nil
}
