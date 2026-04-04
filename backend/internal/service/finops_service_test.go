package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeMetricRollupStore struct {
	items []models.MetricRollup
}

func newFakeMetricRollupStore(items ...models.MetricRollup) *fakeMetricRollupStore {
	return &fakeMetricRollupStore{items: items}
}

func (f *fakeMetricRollupStore) Create(rollup *models.MetricRollup) error {
	f.items = append(f.items, *rollup)
	return nil
}

func (f *fakeMetricRollupStore) ListByProjectAndService(projectID, serviceName string, windowStart, windowEnd time.Time) ([]models.MetricRollup, error) {
	out := make([]models.MetricRollup, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID && item.ServiceName == serviceName && !item.WindowStart.Before(windowStart) && !item.WindowEnd.After(windowEnd) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeMetricRollupStore) ListByProject(projectID string, limit int) ([]models.MetricRollup, error) {
	out := make([]models.MetricRollup, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type fakeScaleToZeroStore struct {
	items []models.ScaleToZeroState
}

func newFakeScaleToZeroStore(items ...models.ScaleToZeroState) *fakeScaleToZeroStore {
	return &fakeScaleToZeroStore{items: items}
}

func (f *fakeScaleToZeroStore) Upsert(state *models.ScaleToZeroState) error {
	for i, item := range f.items {
		if item.ProjectID == state.ProjectID && item.ServiceName == state.ServiceName {
			f.items[i] = *state
			return nil
		}
	}
	f.items = append(f.items, *state)
	return nil
}

func (f *fakeScaleToZeroStore) GetByService(projectID, serviceName string) (*models.ScaleToZeroState, error) {
	for _, item := range f.items {
		if item.ProjectID == projectID && item.ServiceName == serviceName {
			return &item, nil
		}
	}
	return nil, nil
}

func (f *fakeScaleToZeroStore) ListByProject(projectID string) ([]models.ScaleToZeroState, error) {
	out := make([]models.ScaleToZeroState, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func newTestFinOpsService(
	rollupStore MetricRollupStore,
	scaleToZeroStore ScaleToZeroStore,
	instanceStore InstanceStore,
	meshStore MeshNetworkStore,
) *FinOpsService {
	return NewFinOpsService(rollupStore, scaleToZeroStore, instanceStore, meshStore)
}

func TestFinOpsServiceIngestMetricRollupSuccess(t *testing.T) {
	rollupStore := newFakeMetricRollupStore()

	svc := newTestFinOpsService(
		rollupStore,
		newFakeScaleToZeroStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	record, err := svc.IngestMetricRollup(context.Background(), IngestMetricRollupCommand{
		ProjectID:   "prj_123",
		ServiceName: "api",
		MetricKind:  MetricKindCPU,
		WindowStart: time.Now().Add(-1 * time.Hour),
		WindowEnd:   time.Now(),
		P95:         75.5,
		Max:         95.0,
		Min:         10.0,
		Avg:         45.2,
		Count:       1000,
		IsRawSample: false,
		Metadata:    map[string]any{"region": "us-east"},
	})
	if err != nil {
		t.Fatalf("ingest rollup: %v", err)
	}

	if record.MetricKind != MetricKindCPU {
		t.Fatalf("expected metric kind cpu, got %q", record.MetricKind)
	}
	if record.P95 != 75.5 {
		t.Fatalf("expected p95 75.5, got %f", record.P95)
	}
	if record.Count != 1000 {
		t.Fatalf("expected count 1000, got %d", record.Count)
	}
}

func TestFinOpsServiceRejectsRawMetricSamples(t *testing.T) {
	svc := newTestFinOpsService(
		newFakeMetricRollupStore(),
		newFakeScaleToZeroStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	_, err := svc.IngestMetricRollup(context.Background(), IngestMetricRollupCommand{
		ProjectID:   "prj_123",
		ServiceName: "api",
		MetricKind:  MetricKindCPU,
		WindowStart: time.Now().Add(-1 * time.Hour),
		WindowEnd:   time.Now(),
		P95:         75.5,
		Max:         95.0,
		Min:         10.0,
		Avg:         45.2,
		Count:       1,
		IsRawSample: true,
	})
	if !errors.Is(err, ErrRawMetricRejected) {
		t.Fatalf("expected ErrRawMetricRejected, got %v", err)
	}
}

func TestFinOpsServiceGetFinOpsSummary(t *testing.T) {
	now := time.Now().UTC()
	rollupStore := newFakeMetricRollupStore(
		models.MetricRollup{
			ID:          "met_1",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindCPU,
			WindowStart: now.Add(-1 * time.Hour),
			WindowEnd:   now,
			P95:         85.0,
			Max:         95.0,
			Min:         50.0,
			Avg:         82.0,
			Count:       100,
		},
		models.MetricRollup{
			ID:          "met_2",
			ProjectID:   "prj_123",
			ServiceName: "api",
			MetricKind:  MetricKindRequestCount,
			WindowStart: now.Add(-1 * time.Hour),
			WindowEnd:   now,
			P95:         0,
			Max:         0,
			Min:         0,
			Avg:         0,
			Count:       5000,
		},
	)

	svc := newTestFinOpsService(
		rollupStore,
		newFakeScaleToZeroStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	summary, err := svc.GetFinOpsSummary(context.Background(), "prj_123")
	if err != nil {
		t.Fatalf("get finops summary: %v", err)
	}

	if _, ok := summary.Services["api"]; !ok {
		t.Fatal("expected api service in summary")
	}
	if summary.Services["api"].AvgCPU != 82.0 {
		t.Fatalf("expected avg cpu 82.0, got %f", summary.Services["api"].AvgCPU)
	}
	if summary.Services["api"].TotalRequests != 5000 {
		t.Fatalf("expected total requests 5000, got %d", summary.Services["api"].TotalRequests)
	}
	if len(summary.HotNodes) != 1 {
		t.Fatalf("expected 1 hot node, got %d", len(summary.HotNodes))
	}
}

func TestFinOpsServiceEnableScaleToZero(t *testing.T) {
	scaleToZeroStore := newFakeScaleToZeroStore()

	svc := newTestFinOpsService(
		newFakeMetricRollupStore(),
		scaleToZeroStore,
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	result, err := svc.EnableScaleToZero(context.Background(), "prj_123", "api", 5000)
	if err != nil {
		t.Fatalf("enable scale to zero: %v", err)
	}

	if !result.Enabled {
		t.Fatal("expected scale-to-zero to be enabled")
	}
	if result.WakeTimeoutMs != 5000 {
		t.Fatalf("expected wake timeout 5000, got %d", result.WakeTimeoutMs)
	}
	if result.State != ScaleToZeroStateActive {
		t.Fatalf("expected state active, got %q", result.State)
	}
}

func TestFinOpsServiceUpdateScaleToZeroState(t *testing.T) {
	scaleToZeroStore := newFakeScaleToZeroStore(models.ScaleToZeroState{
		ID:                "s2z_123",
		ProjectID:         "prj_123",
		ServiceName:       "api",
		State:             ScaleToZeroStateActive,
		Enabled:           true,
		WakeTimeoutMs:     5000,
		LastStateChangeAt: time.Now().UTC(),
	})

	svc := newTestFinOpsService(
		newFakeMetricRollupStore(),
		scaleToZeroStore,
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	result, err := svc.UpdateScaleToZeroState(context.Background(), "prj_123", "api", ScaleToZeroStateScaledDown)
	if err != nil {
		t.Fatalf("update scale to zero state: %v", err)
	}

	if result.State != ScaleToZeroStateScaledDown {
		t.Fatalf("expected state scaled_down, got %q", result.State)
	}
}

func TestFinOpsServiceCheckWakeTimeout(t *testing.T) {
	now := time.Now().UTC()
	scaleToZeroStore := newFakeScaleToZeroStore(models.ScaleToZeroState{
		ID:                "s2z_123",
		ProjectID:         "prj_123",
		ServiceName:       "api",
		State:             ScaleToZeroStateWakingUp,
		Enabled:           true,
		WakeTimeoutMs:     5000,
		LastStateChangeAt: now.Add(-10 * time.Second),
		WakeTimeoutAt:     ptrTime(now.Add(30 * time.Second)),
	})

	svc := newTestFinOpsService(
		newFakeMetricRollupStore(),
		scaleToZeroStore,
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	result, err := svc.CheckWakeTimeout(context.Background(), "prj_123", "api")
	if err != nil {
		t.Fatalf("check wake timeout: %v", err)
	}

	if result.TimedOut {
		t.Fatal("expected not timed out yet")
	}
	if result.State != ScaleToZeroStateWakingUp {
		t.Fatalf("expected state waking_up, got %q", result.State)
	}
}

func TestFinOpsServiceDisableScaleToZero(t *testing.T) {
	scaleToZeroStore := newFakeScaleToZeroStore(models.ScaleToZeroState{
		ID:                "s2z_123",
		ProjectID:         "prj_123",
		ServiceName:       "api",
		State:             ScaleToZeroStateScaledDown,
		Enabled:           true,
		WakeTimeoutMs:     5000,
		LastStateChangeAt: time.Now().UTC(),
	})

	svc := newTestFinOpsService(
		newFakeMetricRollupStore(),
		scaleToZeroStore,
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	result, err := svc.DisableScaleToZero(context.Background(), "prj_123", "api")
	if err != nil {
		t.Fatalf("disable scale to zero: %v", err)
	}

	if result.Enabled {
		t.Fatal("expected scale-to-zero to be disabled")
	}
}

func TestFinOpsServiceScaleToZeroNotEnabled(t *testing.T) {
	svc := newTestFinOpsService(
		newFakeMetricRollupStore(),
		newFakeScaleToZeroStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	_, err := svc.UpdateScaleToZeroState(context.Background(), "prj_123", "api", ScaleToZeroStateScaledDown)
	if !errors.Is(err, ErrScaleToZeroNotEnabled) {
		t.Fatalf("expected ErrScaleToZeroNotEnabled, got %v", err)
	}
}

func TestNormalizeScaleToZeroState(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"active", ScaleToZeroStateActive},
		{"scaling_down", ScaleToZeroStateScalingDown},
		{"scaled_down", ScaleToZeroStateScaledDown},
		{"waking_up", ScaleToZeroStateWakingUp},
		{"unknown", ScaleToZeroStateActive},
		{"", ScaleToZeroStateActive},
	}

	for _, tt := range tests {
		got := normalizeScaleToZeroState(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeScaleToZeroState(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
