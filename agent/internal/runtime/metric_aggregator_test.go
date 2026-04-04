package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testMetricAggregator() *MetricAggregator {
	a := NewMetricAggregator(nil, MetricAggregatorConfig{
		WindowDuration:    1 * time.Second,
		ReportingInterval: 1 * time.Second,
		MaxSamplesPerSlot: 100,
	})
	a.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)
	}
	return a
}

func TestMetricAggregatorDefaultConfig(t *testing.T) {
	a := NewMetricAggregator(nil, MetricAggregatorConfig{})
	if a.cfg.WindowDuration != 1*time.Minute {
		t.Fatalf("expected default window 1m, got %s", a.cfg.WindowDuration)
	}
	if a.cfg.ReportingInterval != 30*time.Second {
		t.Fatalf("expected default reporting 30s, got %s", a.cfg.ReportingInterval)
	}
}

func TestMetricAggregatorRecord(t *testing.T) {
	a := testMetricAggregator()
	a.Record("cpu", 45.0)
	a.Record("cpu", 60.0)
	a.Record("ram", 1024.0)

	total, active := a.Stats()
	if total != 3 {
		t.Fatalf("expected 3 total samples, got %d", total)
	}
	if active != 2 {
		t.Fatalf("expected 2 active slots, got %d", active)
	}
}

func TestMetricAggregatorComputeAggregate(t *testing.T) {
	a := testMetricAggregator()
	values := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for _, v := range values {
		a.Record("latency", v)
	}

	agg, ok := a.ComputeAggregate("latency")
	if !ok {
		t.Fatal("expected aggregate to exist")
	}
	if agg.Min != 10 {
		t.Fatalf("expected min 10, got %f", agg.Min)
	}
	if agg.Max != 100 {
		t.Fatalf("expected max 100, got %f", agg.Max)
	}
	if agg.Avg != 55 {
		t.Fatalf("expected avg 55, got %f", agg.Avg)
	}
	if agg.Count != 10 {
		t.Fatalf("expected count 10, got %d", agg.Count)
	}
	if agg.P95 != 100 {
		t.Fatalf("expected p95 100, got %f", agg.P95)
	}
}

func TestMetricAggregatorComputeAggregateEmpty(t *testing.T) {
	a := testMetricAggregator()
	_, ok := a.ComputeAggregate("missing")
	if ok {
		t.Fatal("expected no aggregate for missing metric")
	}
}

func TestMetricAggregatorMaxSamplesPerSlot(t *testing.T) {
	a := NewMetricAggregator(nil, MetricAggregatorConfig{
		WindowDuration:    10 * time.Second,
		MaxSamplesPerSlot: 5,
	})
	a.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)
	}

	for i := 0; i < 10; i++ {
		a.Record("cpu", float64(i))
	}

	total, _ := a.Stats()
	if total != 10 {
		t.Fatalf("expected 10 total, got %d", total)
	}

	agg, ok := a.ComputeAggregate("cpu")
	if !ok {
		t.Fatal("expected aggregate to exist")
	}
	if agg.Count != 5 {
		t.Fatalf("expected 5 samples (max per slot), got %d", agg.Count)
	}
}

func TestMetricAggregatorWindowRotation(t *testing.T) {
	a := testMetricAggregator()
	a.Record("cpu", 50.0)

	a.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 5, 0, time.UTC)
	}

	a.Record("cpu", 75.0)

	agg, ok := a.ComputeAggregate("cpu")
	if !ok {
		t.Fatal("expected aggregate to exist")
	}
	if agg.Count != 1 {
		t.Fatalf("expected 1 sample after window rotation, got %d", agg.Count)
	}
	if agg.Avg != 75.0 {
		t.Fatalf("expected avg 75 after rotation, got %f", agg.Avg)
	}
}

func TestMetricAggregatorCollectExpiredWindows(t *testing.T) {
	a := testMetricAggregator()
	a.Record("cpu", 50.0)
	a.Record("ram", 2048.0)

	a.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 5, 0, time.UTC)
	}

	expired := a.CollectExpiredWindows()
	if len(expired) != 2 {
		t.Fatalf("expected 2 expired windows, got %d", len(expired))
	}
	if expired["cpu"].Count != 1 {
		t.Fatalf("expected cpu count 1, got %d", expired["cpu"].Count)
	}
	if expired["ram"].Count != 1 {
		t.Fatalf("expected ram count 1, got %d", expired["ram"].Count)
	}

	_, active := a.Stats()
	if active != 0 {
		t.Fatalf("expected 0 active slots after collection, got %d", active)
	}
}

func TestMetricAggregatorCollectExpiredWindowsNoExpiry(t *testing.T) {
	a := testMetricAggregator()
	a.Record("cpu", 50.0)

	expired := a.CollectExpiredWindows()
	if len(expired) != 0 {
		t.Fatalf("expected 0 expired windows before timeout, got %d", len(expired))
	}
}

func TestMetricAggregatorBuildMetricRollup(t *testing.T) {
	a := testMetricAggregator()
	cpu := contracts.MetricAggregate{P95: 80, Max: 95, Min: 10, Avg: 50, Count: 100}
	ram := contracts.MetricAggregate{P95: 4096, Max: 8192, Min: 1024, Avg: 3072, Count: 100}
	latency := contracts.MetricAggregate{P95: 200, Max: 500, Min: 5, Avg: 50, Count: 1000}

	rollup := a.BuildMetricRollup("prj_123", contracts.TargetKindInstance, "inst_123", "api", contracts.MetricWindow1Min, cpu, ram, &latency)

	if rollup.ProjectID != "prj_123" {
		t.Fatalf("expected project_id prj_123, got %q", rollup.ProjectID)
	}
	if rollup.TargetKind != contracts.TargetKindInstance {
		t.Fatalf("expected target_kind instance, got %q", rollup.TargetKind)
	}
	if rollup.CPU.Avg != 50 {
		t.Fatalf("expected cpu avg 50, got %f", rollup.CPU.Avg)
	}
	if rollup.RAM.Avg != 3072 {
		t.Fatalf("expected ram avg 3072, got %f", rollup.RAM.Avg)
	}
	if rollup.Latency.Avg != 50 {
		t.Fatalf("expected latency avg 50, got %f", rollup.Latency.Avg)
	}
}

func TestMetricAggregatorBuildMetricRollupNoLatency(t *testing.T) {
	a := testMetricAggregator()
	cpu := contracts.MetricAggregate{P95: 80, Max: 95, Min: 10, Avg: 50, Count: 100}
	ram := contracts.MetricAggregate{P95: 4096, Max: 8192, Min: 1024, Avg: 3072, Count: 100}

	rollup := a.BuildMetricRollup("prj_123", contracts.TargetKindInstance, "inst_123", "", contracts.MetricWindow1Min, cpu, ram, nil)

	if rollup.Latency.Count != 0 {
		t.Fatalf("expected no latency data, got count %d", rollup.Latency.Count)
	}
}

func TestMetricAggregatorPersistMetricRollup(t *testing.T) {
	a := testMetricAggregator()
	cpu := contracts.MetricAggregate{P95: 80, Max: 95, Min: 10, Avg: 50, Count: 100}
	ram := contracts.MetricAggregate{P95: 4096, Max: 8192, Min: 1024, Avg: 3072, Count: 100}
	rollup := a.BuildMetricRollup("prj_123", contracts.TargetKindInstance, "inst_123", "api", contracts.MetricWindow1Min, cpu, ram, nil)

	root := filepath.Join(t.TempDir(), "runtime-root")
	metricPath, err := a.PersistMetricRollup(root, "prj_123", "bind_123", rollup)
	if err != nil {
		t.Fatalf("persist metric rollup: %v", err)
	}

	if _, err := os.Stat(metricPath); err != nil {
		t.Fatalf("expected metric file to exist: %v", err)
	}

	var loaded contracts.MetricRollupPayload
	raw, err := os.ReadFile(metricPath)
	if err != nil {
		t.Fatalf("read metric file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode metric file: %v", err)
	}
	if loaded.CPU.Avg != 50 {
		t.Fatalf("expected cpu avg 50, got %f", loaded.CPU.Avg)
	}
}

func TestComputeAggregateEmptySamples(t *testing.T) {
	agg := computeAggregate(nil)
	if agg.Count != 0 {
		t.Fatalf("expected count 0 for empty samples, got %d", agg.Count)
	}
}

func TestMetricAggregatorHandleReportMetricRollup(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	a := testMetricAggregator()
	a.Record("cpu", 50.0)
	a.Record("ram", 2048.0)

	a.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 5, 0, time.UTC)
	}

	reported, err := a.HandleReportMetricRollup(context.Background(), nil, ReportMetricRollupPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		TargetKind:    contracts.TargetKindInstance,
		TargetID:      "inst_123",
		WorkspaceRoot: root,
	})
	if err != nil {
		t.Fatalf("handle report metric rollup: %v", err)
	}
	if reported != 0 {
		t.Fatalf("expected 0 reported (no sender), got %d", reported)
	}

	metricPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "metrics", "rollup_20260404T110005Z.json")
	if _, err := os.Stat(metricPath); err != nil {
		t.Fatalf("expected metric file at %s: %v", metricPath, err)
	}
}

func TestMetricAggregatorHandleReportMetricRollupNoWindows(t *testing.T) {
	a := testMetricAggregator()

	reported, err := a.HandleReportMetricRollup(context.Background(), nil, ReportMetricRollupPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		TargetKind:    contracts.TargetKindInstance,
		TargetID:      "inst_123",
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("handle report metric rollup: %v", err)
	}
	if reported != 0 {
		t.Fatalf("expected 0 reported, got %d", reported)
	}
}

type fakeMetricSender struct {
	mu      sync.Mutex
	sent    []contracts.MetricRollupPayload
	sendErr error
}

func (f *fakeMetricSender) SendMetricRollup(_ context.Context, payload contracts.MetricRollupPayload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, payload)
	return f.sendErr
}

func TestMetricAggregatorHandleReportMetricRollupWithSender(t *testing.T) {
	a := testMetricAggregator()
	a.Record("cpu", 50.0)
	a.Record("ram", 2048.0)
	a.Record("latency", 100.0)

	a.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 5, 0, time.UTC)
	}

	sender := &fakeMetricSender{}
	reported, err := a.HandleReportMetricRollup(context.Background(), nil, ReportMetricRollupPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		TargetKind:    contracts.TargetKindInstance,
		TargetID:      "inst_123",
		WorkspaceRoot: filepath.Join(t.TempDir(), "runtime-root"),
		MetricSender:  sender,
	})
	if err != nil {
		t.Fatalf("handle report metric rollup: %v", err)
	}
	if reported != 1 {
		t.Fatalf("expected 1 reported, got %d", reported)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 rollup sent, got %d", len(sender.sent))
	}
	if sender.sent[0].CPU.Count != 1 {
		t.Fatalf("expected cpu count 1, got %d", sender.sent[0].CPU.Count)
	}
	if sender.sent[0].Latency.Count != 1 {
		t.Fatalf("expected latency count 1, got %d", sender.sent[0].Latency.Count)
	}
}
