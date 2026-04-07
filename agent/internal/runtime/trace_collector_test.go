package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testTraceCollector() *TraceCollector {
	c := NewTraceCollector(nil, TraceCollectorConfig{
		SampleRate:        1.0,
		MaxHopsPerTrace:   16,
		ReportingInterval: 1 * time.Second,
		MaxWindowAge:      2 * time.Second,
	})
	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC)
	}
	return c
}

func TestTraceCollectorDefaultConfig(t *testing.T) {
	c := NewTraceCollector(nil, TraceCollectorConfig{})
	if c.cfg.SampleRate != 0.1 {
		t.Fatalf("expected default sample rate 0.1, got %f", c.cfg.SampleRate)
	}
	if c.cfg.MaxHopsPerTrace != 16 {
		t.Fatalf("expected default max hops 16, got %d", c.cfg.MaxHopsPerTrace)
	}
	if c.cfg.ReportingInterval != 30*time.Second {
		t.Fatalf("expected default reporting interval 30s, got %s", c.cfg.ReportingInterval)
	}
	if c.cfg.MaxWindowAge != 5*time.Minute {
		t.Fatalf("expected default max window age 5m, got %s", c.cfg.MaxWindowAge)
	}
}

func TestTraceCollectorShouldSample(t *testing.T) {
	c := NewTraceCollector(nil, TraceCollectorConfig{SampleRate: 1.0})
	for i := 0; i < 100; i++ {
		if !c.ShouldSample() {
			t.Fatal("expected all traces to be sampled at rate 1.0")
		}
	}
	total, sampled, _ := c.Stats()
	if total != 100 || sampled != 100 {
		t.Fatalf("expected 100 total and 100 sampled, got %d total, %d sampled", total, sampled)
	}
}

func TestTraceCollectorShouldSampleZeroRate(t *testing.T) {
	c := NewTraceCollector(nil, TraceCollectorConfig{SampleRate: 1.0})
	c.cfg.SampleRate = 0
	if c.ShouldSample() {
		t.Fatal("expected no sampling at rate 0")
	}
}

func TestTraceCollectorShouldSampleProbabilistic(t *testing.T) {
	c := NewTraceCollector(nil, TraceCollectorConfig{SampleRate: 0.5})
	trials := 10000
	for i := 0; i < trials; i++ {
		c.ShouldSample()
	}
	total, sampled, _ := c.Stats()
	if total != trials {
		t.Fatalf("expected %d total, got %d", trials, total)
	}
	rate := float64(sampled) / float64(total)
	if rate < 0.45 || rate > 0.55 {
		t.Fatalf("expected sample rate ~0.5, got %.3f (%d/%d)", rate, sampled, total)
	}
}

func TestTraceCollectorRecordHop(t *testing.T) {
	c := testTraceCollector()
	c.RecordHop("corr_1", "gateway", "sidecar:web", "http", 12.5, "ok", true)
	c.RecordHop("corr_1", "sidecar:web", "api", "http", 8.3, "ok", true)

	_, _, active := c.Stats()
	if active != 1 {
		t.Fatalf("expected 1 active window, got %d", active)
	}
}

func TestTraceCollectorRecordHopRejectsEmptyFields(t *testing.T) {
	c := testTraceCollector()
	c.RecordHop("", "a", "b", "http", 1.0, "ok", true)
	c.RecordHop("corr_1", "", "b", "http", 1.0, "ok", true)
	c.RecordHop("corr_1", "a", "", "http", 1.0, "ok", true)

	_, _, active := c.Stats()
	if active != 0 {
		t.Fatal("expected no windows when required fields are empty")
	}
}

func TestTraceCollectorMaxHopsPerTrace(t *testing.T) {
	c := NewTraceCollector(nil, TraceCollectorConfig{
		SampleRate:      1.0,
		MaxHopsPerTrace: 3,
	})
	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC)
	}

	for i := 0; i < 10; i++ {
		c.RecordHop("corr_1", "hop_from", "hop_to", "http", 1.0, "ok", true)
	}

	c.mu.Lock()
	window := c.windows["corr_1"]
	c.mu.Unlock()

	if len(window.Hops) != 3 {
		t.Fatalf("expected 3 hops (max), got %d", len(window.Hops))
	}
}

func TestTraceCollectorCompleteTrace(t *testing.T) {
	c := testTraceCollector()
	c.RecordHop("corr_1", "gateway", "sidecar", "http", 5.0, "ok", true)
	c.CompleteTrace("prj_123", "corr_1")

	c.mu.Lock()
	window := c.windows["corr_1"]
	c.mu.Unlock()

	if !window.Completed {
		t.Fatal("expected trace to be marked completed")
	}
	if window.ProjectID != "prj_123" {
		t.Fatalf("expected project_id prj_123, got %q", window.ProjectID)
	}
}

func TestTraceCollectorCollectExpiredWindows(t *testing.T) {
	c := testTraceCollector()
	c.RecordHop("corr_1", "gateway", "sidecar", "http", 5.0, "ok", true)
	c.CompleteTrace("prj_123", "corr_1")

	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 9, 0, 5, 0, time.UTC)
	}

	expired := c.CollectExpiredWindows()
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired window, got %d", len(expired))
	}
	if expired[0].CorrelationID != "corr_1" {
		t.Fatalf("expected corr_1, got %q", expired[0].CorrelationID)
	}

	_, _, active := c.Stats()
	if active != 0 {
		t.Fatalf("expected 0 active windows after collection, got %d", active)
	}
}

func TestTraceCollectorBuildTraceSummary(t *testing.T) {
	c := testTraceCollector()
	c.RecordHop("corr_1", "gateway", "sidecar:web", "http", 12.5, "ok", true)
	c.RecordHop("corr_1", "sidecar:web", "api", "http", 8.3, "ok", true)
	c.CompleteTrace("prj_123", "corr_1")

	c.mu.Lock()
	window := *c.windows["corr_1"]
	c.mu.Unlock()

	summary := c.BuildTraceSummary(window)

	if summary.ProjectID != "prj_123" {
		t.Fatalf("expected project_id prj_123, got %q", summary.ProjectID)
	}
	if summary.CorrelationID != "corr_1" {
		t.Fatalf("expected correlation_id corr_1, got %q", summary.CorrelationID)
	}
	if len(summary.Hops) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(summary.Hops))
	}
	if summary.Hops[0].LatencyMS != 12.5 {
		t.Fatalf("expected first hop latency 12.5, got %f", summary.Hops[0].LatencyMS)
	}
	if !summary.Hops[0].LocalSignal {
		t.Fatal("expected first hop to be local signal")
	}
}

func TestTraceCollectorPersistTraceWindow(t *testing.T) {
	c := testTraceCollector()
	c.RecordHop("corr_1", "gateway", "sidecar", "http", 5.0, "ok", true)

	c.mu.Lock()
	window := *c.windows["corr_1"]
	c.mu.Unlock()

	root := filepath.Join(t.TempDir(), "runtime-root")
	tracePath, err := c.PersistTraceWindow(root, "prj_123", "bind_123", "corr_1", window)
	if err != nil {
		t.Fatalf("persist trace window: %v", err)
	}

	if _, err := os.Stat(tracePath); err != nil {
		t.Fatalf("expected trace file to exist: %v", err)
	}

	var loaded TraceWindow
	raw, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode trace file: %v", err)
	}
	if len(loaded.Hops) != 1 {
		t.Fatalf("expected 1 hop in persisted trace, got %d", len(loaded.Hops))
	}
}

func TestTraceCollectorHandleReportTraceSummary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	c := testTraceCollector()
	c.RecordHop("corr_1", "gateway", "sidecar:web", "http", 10.0, "ok", true)
	c.CompleteTrace("prj_123", "corr_1")

	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 9, 0, 5, 0, time.UTC)
	}

	reported, err := c.HandleReportTraceSummary(context.Background(), nil, ReportTraceSummaryPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: root,
	})
	if err != nil {
		t.Fatalf("handle report trace summary: %v", err)
	}
	if reported != 1 {
		t.Fatalf("expected 1 reported window, got %d", reported)
	}

	tracePath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "traces", "corr_1.json")
	if _, err := os.Stat(tracePath); err != nil {
		t.Fatalf("expected trace file at %s: %v", tracePath, err)
	}
}

func TestTraceCollectorHandleReportTraceSummaryNoWindows(t *testing.T) {
	c := testTraceCollector()

	reported, err := c.HandleReportTraceSummary(context.Background(), nil, ReportTraceSummaryPayload{
		ProjectID:   "prj_123",
		BindingID:   "bind_123",
		RevisionID:  "rev_123",
		RuntimeMode: contracts.RuntimeModeStandalone,
	})
	if err != nil {
		t.Fatalf("handle report trace summary: %v", err)
	}
	if reported != 0 {
		t.Fatalf("expected 0 reported windows, got %d", reported)
	}
}

func TestTraceCollectorExpiredWindowsSortedByStartedAt(t *testing.T) {
	c := NewTraceCollector(nil, TraceCollectorConfig{
		SampleRate:        1.0,
		MaxHopsPerTrace:   16,
		ReportingInterval: 1 * time.Second,
		MaxWindowAge:      10 * time.Second,
	})

	baseTime := time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC)
	c.now = func() time.Time {
		return baseTime
	}

	c.RecordHop("corr_old", "a", "b", "http", 1.0, "ok", true)

	c.now = func() time.Time {
		return baseTime.Add(2 * time.Second)
	}
	c.RecordHop("corr_new", "a", "b", "http", 1.0, "ok", true)

	c.now = func() time.Time {
		return baseTime.Add(20 * time.Second)
	}

	expired := c.CollectExpiredWindows()
	if len(expired) != 2 {
		t.Fatalf("expected 2 expired windows, got %d", len(expired))
	}
	if !expired[0].StartedAt.Before(expired[1].StartedAt) {
		t.Fatal("expected expired windows sorted by StartedAt ascending")
	}
}
