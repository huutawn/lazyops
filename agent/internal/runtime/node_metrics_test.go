package runtime

import (
	"testing"
	"time"
)

func testNodeMetricsCollector() *NodeMetricsCollector {
	c := NewNodeMetricsCollector(nil, NodeMetricsConfig{
		CollectionInterval: 1 * time.Second,
	})
	c.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)
	}
	return c
}

func TestNodeMetricsCollectorDefaultConfig(t *testing.T) {
	c := NewNodeMetricsCollector(nil, NodeMetricsConfig{})
	if c.cfg.CollectionInterval != 10*time.Second {
		t.Fatalf("expected default interval 10s, got %s", c.cfg.CollectionInterval)
	}
}

func TestNodeMetricsCollectorCollect(t *testing.T) {
	c := testNodeMetricsCollector()
	snapshot := c.Collect()

	if snapshot.CollectedAt.IsZero() {
		t.Fatal("expected collected_at to be set")
	}
	if snapshot.CPUUsagePercent < 0 {
		t.Fatalf("expected non-negative cpu usage, got %f", snapshot.CPUUsagePercent)
	}
	if snapshot.MemoryUsedBytes == 0 {
		t.Fatal("expected non-zero memory usage")
	}
	if snapshot.Goroutines < 1 {
		t.Fatalf("expected at least 1 goroutine, got %d", snapshot.Goroutines)
	}
}

func TestNodeMetricsCollectorFeedToAggregator(t *testing.T) {
	c := testNodeMetricsCollector()
	agg := NewMetricAggregator(nil, MetricAggregatorConfig{
		WindowDuration:    1 * time.Minute,
		MaxSamplesPerSlot: 100,
	})
	agg.now = func() time.Time {
		return time.Date(2026, 4, 4, 11, 0, 0, 0, time.UTC)
	}

	snapshot := c.Collect()
	c.FeedToAggregator(agg, snapshot)

	_, active := agg.Stats()
	if active < 2 {
		t.Fatalf("expected at least 2 active slots (cpu, ram), got %d", active)
	}
}

func TestNodeMetricsCollectorFeedToAggregatorNil(t *testing.T) {
	c := testNodeMetricsCollector()
	snapshot := c.Collect()
	c.FeedToAggregator(nil, snapshot)
}

func TestNodeMetricsCollectorLastSnapshot(t *testing.T) {
	c := testNodeMetricsCollector()
	if c.LastSnapshot() != nil {
		t.Fatal("expected nil before first collection")
	}

	c.Collect()
	last := c.LastSnapshot()
	if last == nil {
		t.Fatal("expected snapshot after collection")
	}
	if last.MemoryUsedBytes == 0 {
		t.Fatal("expected non-zero memory in last snapshot")
	}
}

func TestNodeMetricsCollectorStats(t *testing.T) {
	c := testNodeMetricsCollector()
	if c.Stats() != 0 {
		t.Fatal("expected 0 samples before collection")
	}

	c.Collect()
	c.Collect()
	if c.Stats() != 2 {
		t.Fatalf("expected 2 samples, got %d", c.Stats())
	}
}

func TestNodeMetricsCollectorMultipleCollections(t *testing.T) {
	c := testNodeMetricsCollector()
	for i := 0; i < 5; i++ {
		c.Collect()
	}

	if c.Stats() != 5 {
		t.Fatalf("expected 5 samples, got %d", c.Stats())
	}

	last := c.LastSnapshot()
	if last == nil {
		t.Fatal("expected last snapshot")
	}
	if last.Goroutines < 1 {
		t.Fatalf("expected goroutines >= 1, got %d", last.Goroutines)
	}
}
