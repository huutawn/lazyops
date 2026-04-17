package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type LogCollectorConfig struct {
	MaxEntriesPerBatch int
	ReportingInterval  time.Duration
	MaxBufferAge       time.Duration
	MaxBufferSize      int
	ExcerptMaxLength   int
	CooldownDuration   time.Duration
}

func DefaultLogCollectorConfig() LogCollectorConfig {
	return LogCollectorConfig{
		MaxEntriesPerBatch: 100,
		ReportingInterval:  5 * time.Second,
		MaxBufferAge:       5 * time.Minute,
		MaxBufferSize:      500,
		ExcerptMaxLength:   512,
		CooldownDuration:   60 * time.Second,
	}
}

type logBufferEntry struct {
	entry    contracts.LogEntry
	ingested time.Time
}

type cooldownKey struct {
	Source   string
	Severity string
	Message  string
}

const logBufferKeySeparator = "\x1f"

type LogCollector struct {
	logger *slog.Logger
	cfg    LogCollectorConfig
	now    func() time.Time

	mu        sync.Mutex
	buffers   map[string][]logBufferEntry
	cooldowns map[cooldownKey]time.Time
	total     int
	forwarded int
	dropped   int
}

func NewLogCollector(logger *slog.Logger, cfg LogCollectorConfig) *LogCollector {
	if cfg.MaxEntriesPerBatch <= 0 {
		cfg.MaxEntriesPerBatch = 100
	}
	if cfg.ReportingInterval <= 0 {
		cfg.ReportingInterval = 5 * time.Second
	}
	if cfg.MaxBufferAge <= 0 {
		cfg.MaxBufferAge = 5 * time.Minute
	}
	if cfg.MaxBufferSize <= 0 {
		cfg.MaxBufferSize = 500
	}
	if cfg.ExcerptMaxLength <= 0 {
		cfg.ExcerptMaxLength = 512
	}
	if cfg.CooldownDuration <= 0 {
		cfg.CooldownDuration = 60 * time.Second
	}

	return &LogCollector{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		buffers:   make(map[string][]logBufferEntry),
		cooldowns: make(map[cooldownKey]time.Time),
	}
}

func (c *LogCollector) Ingest(entry contracts.LogEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.total++

	bufferKey := buildLogBufferKey(entry)
	if bufferKey == "" {
		c.dropped++
		return
	}

	if len(c.buffers[bufferKey]) >= c.cfg.MaxBufferSize {
		c.dropped++
		return
	}

	key := cooldownKey{
		Source:   bufferKey,
		Severity: string(entry.Severity),
		Message:  entry.Message,
	}
	if last, ok := c.cooldowns[key]; ok && c.now().Sub(last) < c.cfg.CooldownDuration {
		return
	}

	c.buffers[bufferKey] = append(c.buffers[bufferKey], logBufferEntry{
		entry:    entry,
		ingested: c.now(),
	})
	c.cooldowns[key] = c.now()
}

func (c *LogCollector) IngestLine(source string, line []byte, severity contracts.Severity) {
	if len(line) == 0 {
		return
	}

	msg := string(line)
	excerpt := msg
	if len(excerpt) > c.cfg.ExcerptMaxLength {
		excerpt = excerpt[:c.cfg.ExcerptMaxLength]
	}

	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  severity,
		Source:    source,
		Message:   msg,
		Excerpt:   excerpt,
	})
}

func (c *LogCollector) ScanLine(source string, line []byte, patterns []LogPattern) {
	if len(line) == 0 || len(patterns) == 0 {
		return
	}

	severity := classifySeverity(line, patterns)
	if severity == "" {
		return
	}

	c.IngestLine(source, line, severity)
}

type LogPattern struct {
	Severity contracts.Severity
	Bytes    []byte
}

func classifySeverity(line []byte, patterns []LogPattern) contracts.Severity {
	lineLower := bytes.ToLower(line)
	for _, p := range patterns {
		if bytes.Contains(lineLower, bytes.ToLower(p.Bytes)) {
			return p.Severity
		}
	}
	return ""
}

func (c *LogCollector) CollectExpiredBatches() map[string][]contracts.LogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	result := make(map[string][]contracts.LogEntry)

	for source, entries := range c.buffers {
		if len(entries) == 0 {
			continue
		}

		oldest := entries[0].ingested
		age := now.Sub(oldest)
		readyByInterval := age >= c.cfg.ReportingInterval
		readyByMaxAge := age >= c.cfg.MaxBufferAge
		readyBySize := len(entries) >= c.cfg.MaxEntriesPerBatch
		if !readyByInterval && !readyByMaxAge && !readyBySize {
			continue
		}

		batch := make([]contracts.LogEntry, 0, len(entries))
		for _, e := range entries {
			batch = append(batch, e.entry)
		}

		result[source] = batch
		delete(c.buffers, source)
	}

	for source := range result {
		sort.Slice(result[source], func(i, j int) bool {
			return result[source][i].Timestamp.Before(result[source][j].Timestamp)
		})
	}

	return result
}

func (c *LogCollector) ReportingInterval() time.Duration {
	if c == nil {
		return 0
	}
	return c.cfg.ReportingInterval
}

func (c *LogCollector) BuildLogBatch(projectID, bindingID, revisionID string, entries []contracts.LogEntry) contracts.LogBatchPayload {
	serviceName := ""
	if len(entries) > 0 {
		serviceName = firstNonEmptyString(logLabel(entries[0].Labels, "service"), logLabel(entries[0].Labels, "service_name"))
	}
	return contracts.LogBatchPayload{
		ProjectID:   projectID,
		BindingID:   bindingID,
		RevisionID:  revisionID,
		ServiceName: serviceName,
		Entries:     entries,
		CollectedAt: c.now(),
	}
}

func (c *LogCollector) BuildLogBatchForBufferKey(bufferKey string, fallback ReportLogBatchPayload, entries []contracts.LogEntry) contracts.LogBatchPayload {
	meta := parseLogBufferKey(bufferKey)
	return contracts.LogBatchPayload{
		ProjectID:   firstNonEmptyString(meta.ProjectID, strings.TrimSpace(fallback.ProjectID)),
		BindingID:   firstNonEmptyString(meta.BindingID, strings.TrimSpace(fallback.BindingID)),
		RevisionID:  firstNonEmptyString(meta.RevisionID, strings.TrimSpace(fallback.RevisionID)),
		ServiceName: firstNonEmptyString(meta.ServiceName, strings.TrimSpace(fallback.ServiceName)),
		Entries:     entries,
		CollectedAt: c.now(),
	}
}

func (c *LogCollector) Stats() (total, forwarded, dropped, activeBuffers int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total, c.forwarded, c.dropped, len(c.buffers)
}

func (c *LogCollector) PersistLogBatch(workspaceRoot, projectID, bindingID, source string, batch contracts.LogBatchPayload) (string, error) {
	logDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create log directory: %w", err)
	}

	sanitized := sanitizePathToken(source)
	timestamp := batch.CollectedAt.Format("20060102T150405Z")
	logPath := filepath.Join(logDir, sanitized+"_"+timestamp+".json")

	raw, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal log batch: %w", err)
	}

	if err := os.WriteFile(logPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write log batch: %w", err)
	}

	return logPath, nil
}

type LogSender interface {
	SendLogBatch(context.Context, contracts.LogBatchPayload) error
}

type ReportLogBatchPayload struct {
	ProjectID     string                `json:"project_id"`
	BindingID     string                `json:"binding_id"`
	RevisionID    string                `json:"revision_id"`
	ServiceName   string                `json:"service_name,omitempty"`
	RuntimeMode   contracts.RuntimeMode `json:"runtime_mode"`
	WorkspaceRoot string                `json:"workspace_root"`
	LogSender     LogSender             `json:"-"`
}

func (c *LogCollector) HandleReportLogBatch(ctx context.Context, logger *slog.Logger, payload ReportLogBatchPayload) (int, error) {
	if logger == nil {
		logger = slog.Default()
	}

	batches := c.CollectExpiredBatches()
	if len(batches) == 0 {
		logger.Info("no log batches to report",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
		)
		return 0, nil
	}

	reported := 0
	for bufferKey, entries := range batches {
		meta := parseLogBufferKey(bufferKey)
		batch := c.BuildLogBatchForBufferKey(bufferKey, payload, entries)
		if strings.TrimSpace(batch.ProjectID) == "" || strings.TrimSpace(batch.BindingID) == "" {
			logger.Warn("skipping log batch without routing identity",
				"source", meta.Source,
				"project_id", batch.ProjectID,
				"binding_id", batch.BindingID,
				"entries", len(entries),
			)
			continue
		}

		if payload.LogSender != nil {
			if err := payload.LogSender.SendLogBatch(ctx, batch); err != nil {
				logger.Warn("could not send log batch to backend",
					"source", meta.Source,
					"entries", len(entries),
					"error", err,
				)
				continue
			}
		}

		workspaceRoot := payload.WorkspaceRoot
		if workspaceRoot == "" {
			workspaceRoot = "/var/lib/lazyops"
		}

		logPath, err := c.PersistLogBatch(workspaceRoot, batch.ProjectID, batch.BindingID, firstNonEmptyString(meta.Source, batch.ServiceName, "logs"), batch)
		if err != nil {
			logger.Warn("could not persist log batch",
				"source", meta.Source,
				"error", err,
			)
			continue
		}

		logger.Info("log batch collected",
			"source", meta.Source,
			"project_id", batch.ProjectID,
			"binding_id", batch.BindingID,
			"revision_id", batch.RevisionID,
			"entries", len(entries),
			"log_path", logPath,
		)
		c.mu.Lock()
		c.forwarded += len(entries)
		c.mu.Unlock()
		reported++
	}

	logger.Info("log batch report completed",
		"project_id", payload.ProjectID,
		"binding_id", payload.BindingID,
		"reported", reported,
		"total_batches", len(batches),
	)

	return reported, nil
}

type logBufferMeta struct {
	ProjectID   string
	BindingID   string
	RevisionID  string
	ServiceName string
	Source      string
}

func buildLogBufferKey(entry contracts.LogEntry) string {
	source := strings.TrimSpace(entry.Source)
	if source == "" {
		source = strings.TrimSpace(logLabel(entry.Labels, "source"))
	}
	if source == "" {
		return ""
	}

	projectID := strings.TrimSpace(logLabel(entry.Labels, "project_id"))
	bindingID := strings.TrimSpace(logLabel(entry.Labels, "binding_id"))
	revisionID := strings.TrimSpace(logLabel(entry.Labels, "revision_id"))
	serviceName := firstNonEmptyString(
		strings.TrimSpace(logLabel(entry.Labels, "service")),
		strings.TrimSpace(logLabel(entry.Labels, "service_name")),
	)

	if projectID == "" && bindingID == "" && revisionID == "" && serviceName == "" {
		return source
	}

	return strings.Join([]string{
		projectID,
		bindingID,
		revisionID,
		serviceName,
		source,
	}, logBufferKeySeparator)
}

func parseLogBufferKey(bufferKey string) logBufferMeta {
	parts := strings.Split(bufferKey, logBufferKeySeparator)
	if len(parts) != 5 {
		return logBufferMeta{Source: strings.TrimSpace(bufferKey)}
	}
	return logBufferMeta{
		ProjectID:   strings.TrimSpace(parts[0]),
		BindingID:   strings.TrimSpace(parts[1]),
		RevisionID:  strings.TrimSpace(parts[2]),
		ServiceName: strings.TrimSpace(parts[3]),
		Source:      strings.TrimSpace(parts[4]),
	}
}

func logLabel(labels map[string]string, key string) string {
	if len(labels) == 0 {
		return ""
	}
	return labels[key]
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func DetectLogPatterns(text string) []LogPattern {
	var patterns []LogPattern

	textLower := strings.ToLower(text)
	if strings.Contains(textLower, "error") || strings.Contains(textLower, "err:") || strings.Contains(textLower, "failed") {
		patterns = append(patterns, LogPattern{Severity: contracts.SeverityCritical, Bytes: []byte("error")})
	}
	if strings.Contains(textLower, "warn") || strings.Contains(textLower, "warning") {
		patterns = append(patterns, LogPattern{Severity: contracts.SeverityWarning, Bytes: []byte("warn")})
	}
	if strings.Contains(textLower, "info") || strings.Contains(textLower, "started") || strings.Contains(textLower, "listening") {
		patterns = append(patterns, LogPattern{Severity: contracts.SeverityInfo, Bytes: []byte("info")})
	}

	return patterns
}
