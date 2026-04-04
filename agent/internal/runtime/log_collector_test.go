package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testLogCollector() *LogCollector {
	c := NewLogCollector(nil, LogCollectorConfig{
		MaxEntriesPerBatch: 5,
		ReportingInterval:  1 * time.Second,
		MaxBufferAge:       2 * time.Second,
		MaxBufferSize:      50,
		ExcerptMaxLength:   256,
		CooldownDuration:   1 * time.Second,
	})
	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	}
	return c
}

func TestLogCollectorDefaultConfig(t *testing.T) {
	c := NewLogCollector(nil, LogCollectorConfig{})
	if c.cfg.MaxEntriesPerBatch != 100 {
		t.Fatalf("expected default max entries 100, got %d", c.cfg.MaxEntriesPerBatch)
	}
	if c.cfg.ReportingInterval != 30*time.Second {
		t.Fatalf("expected default reporting interval 30s, got %s", c.cfg.ReportingInterval)
	}
	if c.cfg.MaxBufferAge != 5*time.Minute {
		t.Fatalf("expected default max buffer age 5m, got %s", c.cfg.MaxBufferAge)
	}
	if c.cfg.CooldownDuration != 60*time.Second {
		t.Fatalf("expected default cooldown 60s, got %s", c.cfg.CooldownDuration)
	}
}

func TestLogCollectorIngest(t *testing.T) {
	c := testLogCollector()
	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  contracts.SeverityInfo,
		Source:    "api",
		Message:   "server started on :8080",
	})

	total, _, dropped, active := c.Stats()
	if total != 1 {
		t.Fatalf("expected 1 total, got %d", total)
	}
	if dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if active != 1 {
		t.Fatalf("expected 1 active buffer, got %d", active)
	}
}

func TestLogCollectorIngestLine(t *testing.T) {
	c := testLogCollector()
	c.IngestLine("api", []byte("2026-04-04T10:00:00Z INFO request received"), contracts.SeverityInfo)

	total, _, _, _ := c.Stats()
	if total != 1 {
		t.Fatalf("expected 1 total, got %d", total)
	}
}

func TestLogCollectorIngestLineEmpty(t *testing.T) {
	c := testLogCollector()
	c.IngestLine("api", []byte(""), contracts.SeverityInfo)

	total, _, _, _ := c.Stats()
	if total != 0 {
		t.Fatal("expected empty line to be ignored")
	}
}

func TestLogCollectorCooldownPreventsDuplicateForwarding(t *testing.T) {
	c := testLogCollector()
	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  contracts.SeverityCritical,
		Source:    "api",
		Message:   "connection refused",
	})

	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  contracts.SeverityCritical,
		Source:    "api",
		Message:   "connection refused",
	})

	total, _, _, active := c.Stats()
	if total != 2 {
		t.Fatalf("expected 2 total ingests, got %d", total)
	}
	if active != 1 {
		t.Fatalf("expected 1 active buffer, got %d", active)
	}

	c.mu.Lock()
	entries := c.buffers["api"]
	c.mu.Unlock()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in buffer after cooldown, got %d", len(entries))
	}
}

func TestLogCollectorMaxBufferSize(t *testing.T) {
	c := NewLogCollector(nil, LogCollectorConfig{
		MaxEntriesPerBatch: 100,
		MaxBufferAge:       10 * time.Second,
		MaxBufferSize:      3,
		CooldownDuration:   0,
	})
	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	}

	for i := 0; i < 5; i++ {
		c.Ingest(contracts.LogEntry{
			Timestamp: c.now(),
			Severity:  contracts.SeverityInfo,
			Source:    "api",
			Message:   fmt.Sprintf("log line %d", i),
		})
	}

	total, _, dropped, active := c.Stats()
	if total != 5 {
		t.Fatalf("expected 5 total, got %d", total)
	}
	if dropped != 2 {
		t.Fatalf("expected 2 dropped due to max buffer size, got %d", dropped)
	}
	if active != 1 {
		t.Fatalf("expected 1 active buffer, got %d", active)
	}
}

func TestLogCollectorCollectExpiredBatches(t *testing.T) {
	c := testLogCollector()
	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  contracts.SeverityInfo,
		Source:    "api",
		Message:   "request received",
	})
	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  contracts.SeverityCritical,
		Source:    "api",
		Message:   "panic: nil pointer",
	})

	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 10, 0, 5, 0, time.UTC)
	}

	batches := c.CollectExpiredBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches["api"]) != 2 {
		t.Fatalf("expected 2 entries in api batch, got %d", len(batches["api"]))
	}
	if batches["api"][0].Severity != contracts.SeverityInfo {
		t.Fatalf("expected first entry to be info (sorted by time), got %q", batches["api"][0].Severity)
	}

	_, _, _, active := c.Stats()
	if active != 0 {
		t.Fatalf("expected 0 active buffers after collection, got %d", active)
	}
}

func TestLogCollectorCollectExpiredBatchesNoExpiry(t *testing.T) {
	c := testLogCollector()
	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  contracts.SeverityInfo,
		Source:    "api",
		Message:   "request received",
	})

	batches := c.CollectExpiredBatches()
	if len(batches) != 0 {
		t.Fatalf("expected 0 batches before expiry, got %d", len(batches))
	}
}

func TestLogCollectorCollectExpiredBatchesMaxEntries(t *testing.T) {
	c := NewLogCollector(nil, LogCollectorConfig{
		MaxEntriesPerBatch: 5,
		ReportingInterval:  1 * time.Second,
		MaxBufferAge:       10 * time.Second,
		MaxBufferSize:      50,
		CooldownDuration:   0,
	})
	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	}

	for i := 0; i < 6; i++ {
		c.Ingest(contracts.LogEntry{
			Timestamp: c.now(),
			Severity:  contracts.SeverityInfo,
			Source:    "api",
			Message:   fmt.Sprintf("log line %d", i),
		})
	}

	batches := c.CollectExpiredBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch due to max entries, got %d", len(batches))
	}
	if len(batches["api"]) != 6 {
		t.Fatalf("expected 6 entries (exceeds max per batch triggers collection), got %d", len(batches["api"]))
	}
}

func TestLogCollectorBuildLogBatch(t *testing.T) {
	c := testLogCollector()
	entries := []contracts.LogEntry{
		{Timestamp: c.now(), Severity: contracts.SeverityInfo, Source: "api", Message: "started"},
		{Timestamp: c.now(), Severity: contracts.SeverityCritical, Source: "api", Message: "error"},
	}

	batch := c.BuildLogBatch("prj_123", "bind_123", "rev_123", entries)

	if batch.ProjectID != "prj_123" {
		t.Fatalf("expected project_id prj_123, got %q", batch.ProjectID)
	}
	if batch.BindingID != "bind_123" {
		t.Fatalf("expected binding_id bind_123, got %q", batch.BindingID)
	}
	if len(batch.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(batch.Entries))
	}
	if batch.CollectedAt.IsZero() {
		t.Fatal("expected collected_at to be set")
	}
}

func TestLogCollectorPersistLogBatch(t *testing.T) {
	c := testLogCollector()
	entries := []contracts.LogEntry{
		{Timestamp: c.now(), Severity: contracts.SeverityInfo, Source: "api", Message: "started"},
	}
	batch := c.BuildLogBatch("prj_123", "bind_123", "rev_123", entries)

	root := filepath.Join(t.TempDir(), "runtime-root")
	logPath, err := c.PersistLogBatch(root, "prj_123", "bind_123", "api", batch)
	if err != nil {
		t.Fatalf("persist log batch: %v", err)
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}

	var loaded contracts.LogBatchPayload
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode log file: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry in persisted log, got %d", len(loaded.Entries))
	}
}

func TestLogCollectorScanLine(t *testing.T) {
	c := testLogCollector()
	patterns := []LogPattern{
		{Severity: contracts.SeverityCritical, Bytes: []byte("error")},
		{Severity: contracts.SeverityWarning, Bytes: []byte("warn")},
		{Severity: contracts.SeverityInfo, Bytes: []byte("info")},
	}

	c.ScanLine("api", []byte("2026-04-04T10:00:00Z ERROR connection refused"), patterns)

	total, _, _, _ := c.Stats()
	if total != 1 {
		t.Fatalf("expected 1 total from scan, got %d", total)
	}
}

func TestLogCollectorScanLineNoMatch(t *testing.T) {
	c := testLogCollector()
	patterns := []LogPattern{
		{Severity: contracts.SeverityCritical, Bytes: []byte("error")},
	}

	c.ScanLine("api", []byte("2026-04-04T10:00:00Z normal operation"), patterns)

	total, _, _, _ := c.Stats()
	if total != 0 {
		t.Fatal("expected no ingestion when no pattern matches")
	}
}

func TestLogCollectorScanLineEmptyLine(t *testing.T) {
	c := testLogCollector()
	patterns := []LogPattern{
		{Severity: contracts.SeverityCritical, Bytes: []byte("error")},
	}

	c.ScanLine("api", []byte(""), patterns)

	total, _, _, _ := c.Stats()
	if total != 0 {
		t.Fatal("expected empty line to be ignored")
	}
}

func TestClassifySeverity(t *testing.T) {
	patterns := []LogPattern{
		{Severity: contracts.SeverityCritical, Bytes: []byte("error")},
		{Severity: contracts.SeverityWarning, Bytes: []byte("warn")},
		{Severity: contracts.SeverityInfo, Bytes: []byte("info")},
	}

	cases := []struct {
		line     string
		expected contracts.Severity
	}{
		{"ERROR: connection refused", contracts.SeverityCritical},
		{"error: timeout", contracts.SeverityCritical},
		{"WARN: slow query", contracts.SeverityWarning},
		{"warning: deprecated", contracts.SeverityWarning},
		{"INFO: server started", contracts.SeverityInfo},
		{"normal log line", ""},
	}

	for _, tc := range cases {
		got := classifySeverity([]byte(tc.line), patterns)
		if got != tc.expected {
			t.Errorf("classifySeverity(%q) = %q, want %q", tc.line, got, tc.expected)
		}
	}
}

func TestDetectLogPatterns(t *testing.T) {
	patterns := DetectLogPatterns("ERROR: something went wrong")
	if len(patterns) == 0 {
		t.Fatal("expected patterns for error line")
	}
	if patterns[0].Severity != contracts.SeverityCritical {
		t.Fatalf("expected critical severity, got %q", patterns[0].Severity)
	}

	patterns = DetectLogPatterns("WARN: slow response")
	if len(patterns) == 0 {
		t.Fatal("expected patterns for warn line")
	}
	if patterns[0].Severity != contracts.SeverityWarning {
		t.Fatalf("expected warning severity, got %q", patterns[0].Severity)
	}

	patterns = DetectLogPatterns("INFO: started successfully")
	if len(patterns) == 0 {
		t.Fatal("expected patterns for info line")
	}
	if patterns[0].Severity != contracts.SeverityInfo {
		t.Fatalf("expected info severity, got %q", patterns[0].Severity)
	}

	patterns = DetectLogPatterns("just a normal line")
	if len(patterns) != 0 {
		t.Fatalf("expected no patterns for normal line, got %d", len(patterns))
	}
}

func TestLogCollectorHandleReportLogBatch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	c := testLogCollector()
	c.Ingest(contracts.LogEntry{
		Timestamp: c.now(),
		Severity:  contracts.SeverityCritical,
		Source:    "api",
		Message:   "panic: nil pointer dereference",
	})

	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 10, 0, 5, 0, time.UTC)
	}

	reported, err := c.HandleReportLogBatch(context.Background(), nil, ReportLogBatchPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: root,
	})
	if err != nil {
		t.Fatalf("handle report log batch: %v", err)
	}
	if reported != 1 {
		t.Fatalf("expected 1 reported batch, got %d", reported)
	}

	logPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "logs", "api_20260404T100005Z.json")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file at %s: %v", logPath, err)
	}
}

func TestLogCollectorHandleReportLogBatchNoBatches(t *testing.T) {
	c := testLogCollector()

	reported, err := c.HandleReportLogBatch(context.Background(), nil, ReportLogBatchPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("handle report log batch: %v", err)
	}
	if reported != 0 {
		t.Fatalf("expected 0 reported batches, got %d", reported)
	}
}

func TestLogCollectorExcerptTruncation(t *testing.T) {
	c := NewLogCollector(nil, LogCollectorConfig{
		MaxEntriesPerBatch: 100,
		MaxBufferAge:       10 * time.Second,
		MaxBufferSize:      50,
		ExcerptMaxLength:   10,
		CooldownDuration:   0,
	})
	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	}

	c.IngestLine("api", []byte("this is a very long log message that should be truncated"), contracts.SeverityInfo)

	c.mu.Lock()
	entries := c.buffers["api"]
	c.mu.Unlock()

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if len(entries[0].entry.Excerpt) != 10 {
		t.Fatalf("expected excerpt length 10, got %d", len(entries[0].entry.Excerpt))
	}
	if entries[0].entry.Excerpt != "this is a " {
		t.Fatalf("expected truncated excerpt 'this is a ', got %q", entries[0].entry.Excerpt)
	}
}
