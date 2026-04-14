package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type logWatcherHandle struct {
	id     uint64
	cancel context.CancelFunc
}

func (m *ProcessManager) startDockerLogFollower(containerName, source string, labels map[string]string) {
	if m == nil || m.logCollector == nil {
		return
	}
	containerName = strings.TrimSpace(containerName)
	source = strings.TrimSpace(source)
	if containerName == "" || source == "" {
		return
	}

	key := "docker:" + containerName
	ctx, cancel := context.WithCancel(context.Background())
	m.logWatcherMu.Lock()
	m.logWatcherSeq++
	watcherID := m.logWatcherSeq
	previous := m.logWatcherCancels[key]
	m.logWatcherCancels[key] = logWatcherHandle{id: watcherID, cancel: cancel}
	m.logWatcherMu.Unlock()
	if previous.cancel != nil {
		previous.cancel()
	}

	go m.followDockerLogs(ctx, key, watcherID, containerName, source, labels)
}

func (m *ProcessManager) stopLogFollower(key string) {
	if m == nil {
		return
	}
	m.logWatcherMu.Lock()
	handle := m.logWatcherCancels[key]
	delete(m.logWatcherCancels, key)
	m.logWatcherMu.Unlock()
	if handle.cancel != nil {
		handle.cancel()
	}
}

func (m *ProcessManager) clearLogFollower(watcherKey string, watcherID uint64) {
	if m == nil {
		return
	}
	m.logWatcherMu.Lock()
	handle, ok := m.logWatcherCancels[watcherKey]
	if ok && handle.id == watcherID {
		delete(m.logWatcherCancels, watcherKey)
	}
	m.logWatcherMu.Unlock()
}

func (m *ProcessManager) followDockerLogs(ctx context.Context, watcherKey string, watcherID uint64, containerName, source string, labels map[string]string) {
	defer m.clearLogFollower(watcherKey, watcherID)

	cmd := exec.CommandContext(ctx, "docker", "logs", "--timestamps", "--follow", "--tail", "50", containerName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.ingestAgentLogLine(source, fmt.Sprintf("failed to attach stdout log pipe for %s: %v", containerName, err), contracts.SeverityWarning, labels)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.ingestAgentLogLine(source, fmt.Sprintf("failed to attach stderr log pipe for %s: %v", containerName, err), contracts.SeverityWarning, labels)
		return
	}
	if err := cmd.Start(); err != nil {
		m.ingestAgentLogLine(source, fmt.Sprintf("failed to start docker log follower for %s: %v", containerName, err), contracts.SeverityWarning, labels)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		m.scanLogReader(ctx, stdout, source, mergeLogLabels(labels, map[string]string{"stream": "stdout"}))
	}()
	go func() {
		defer wg.Done()
		m.scanLogReader(ctx, stderr, source, mergeLogLabels(labels, map[string]string{"stream": "stderr"}))
	}()
	wg.Wait()

	if err := cmd.Wait(); err != nil && !errors.Is(err, context.Canceled) && ctx.Err() == nil {
		m.ingestAgentLogLine(source, fmt.Sprintf("docker log follower stopped for %s: %v", containerName, err), contracts.SeverityWarning, labels)
	}
}

func (m *ProcessManager) scanLogReader(ctx context.Context, reader io.Reader, source string, labels map[string]string) {
	if m == nil || m.logCollector == nil || reader == nil {
		return
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		occurredAt, message := parseTimestampedLogLine(line)
		if occurredAt.IsZero() {
			occurredAt = time.Now().UTC()
		}
		severity := inferLogSeverity(message)
		m.logCollector.Ingest(contracts.LogEntry{
			Timestamp: occurredAt,
			Severity:  severity,
			Source:    source,
			Message:   message,
			Excerpt:   truncateLogExcerpt(message, m.logCollector.cfg.ExcerptMaxLength),
			Labels:    cloneLogLabels(labels),
		})
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		m.ingestAgentLogLine(source, fmt.Sprintf("log reader failed: %v", err), contracts.SeverityWarning, labels)
	}
}

func (m *ProcessManager) ingestAgentLogLine(source, message string, severity contracts.Severity, labels map[string]string) {
	if m == nil || m.logCollector == nil || strings.TrimSpace(message) == "" {
		return
	}
	m.logCollector.Ingest(contracts.LogEntry{
		Timestamp: time.Now().UTC(),
		Severity:  severity,
		Source:    source,
		Message:   strings.TrimSpace(message),
		Excerpt:   truncateLogExcerpt(strings.TrimSpace(message), m.logCollector.cfg.ExcerptMaxLength),
		Labels:    cloneLogLabels(labels),
	})
}

func mergeLogLabels(base, extra map[string]string) map[string]string {
	merged := cloneLogLabels(base)
	for key, value := range extra {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		merged[key] = value
	}
	return merged
}

func cloneLogLabels(items map[string]string) map[string]string {
	if len(items) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		out[key] = value
	}
	return out
}

func truncateLogExcerpt(message string, maxLen int) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	if maxLen <= 0 || len(message) <= maxLen {
		return message
	}
	return message[:maxLen]
}

func parseTimestampedLogLine(line string) (time.Time, string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return time.Time{}, ""
	}
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return time.Time{}, line
	}
	if ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(parts[0])); err == nil {
		return ts.UTC(), strings.TrimSpace(parts[1])
	}
	return time.Time{}, line
}

func inferLogSeverity(message string) contracts.Severity {
	patterns := DetectLogPatterns(message)
	if len(patterns) > 0 {
		return patterns[0].Severity
	}
	return contracts.SeverityInfo
}

type fileLogWatcher struct {
	logCollector *LogCollector
}

func newFileLogWatcher(collector *LogCollector) *fileLogWatcher {
	return &fileLogWatcher{
		logCollector: collector,
	}
}

func (w *fileLogWatcher) Follow(ctx context.Context, path, source string, labels map[string]string) {
	if w == nil || w.logCollector == nil || strings.TrimSpace(path) == "" {
		return
	}

	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		file, err := os.Open(path)
		if err != nil {
			if !os.IsNotExist(err) {
				w.ingest(source, fmt.Sprintf("failed to open log file %s: %v", path, err), contracts.SeverityWarning, labels)
			}
			time.Sleep(1 * time.Second)
			continue
		}

		stat, err := file.Stat()
		if err != nil {
			_ = file.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		if stat.Size() < offset {
			offset = 0
		}
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			_ = file.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		reader := bufio.NewReader(file)
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				offset += int64(len(line))
				w.ingest(source, line, inferLogSeverity(line), labels)
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				w.ingest(source, fmt.Sprintf("failed to read log file %s: %v", path, err), contracts.SeverityWarning, labels)
				break
			}
		}
		_ = file.Close()

		time.Sleep(1 * time.Second)
	}
}

func (w *fileLogWatcher) ingest(source, message string, severity contracts.Severity, labels map[string]string) {
	if w == nil || w.logCollector == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	w.logCollector.Ingest(contracts.LogEntry{
		Timestamp: time.Now().UTC(),
		Severity:  severity,
		Source:    source,
		Message:   message,
		Excerpt:   truncateLogExcerpt(message, w.logCollector.cfg.ExcerptMaxLength),
		Labels:    cloneLogLabels(labels),
	})
}
