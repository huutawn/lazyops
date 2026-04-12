package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type MetricAggregatorConfig struct {
	WindowDuration    time.Duration
	ReportingInterval time.Duration
	MaxSamplesPerSlot int
}

func DefaultMetricAggregatorConfig() MetricAggregatorConfig {
	return MetricAggregatorConfig{
		WindowDuration:    1 * time.Minute,
		ReportingInterval: 30 * time.Second,
		MaxSamplesPerSlot: 1000,
	}
}

type metricSample struct {
	value    float64
	ingested time.Time
}

type metricSlot struct {
	samples []metricSample
	window  contracts.MetricWindow
	started time.Time
}

type MetricAggregator struct {
	logger *slog.Logger
	cfg    MetricAggregatorConfig
	now    func() time.Time

	mu    sync.Mutex
	slots map[string]*metricSlot
	total int
}

func NewMetricAggregator(logger *slog.Logger, cfg MetricAggregatorConfig) *MetricAggregator {
	if cfg.WindowDuration <= 0 {
		cfg.WindowDuration = 1 * time.Minute
	}
	if cfg.ReportingInterval <= 0 {
		cfg.ReportingInterval = 30 * time.Second
	}
	if cfg.MaxSamplesPerSlot <= 0 {
		cfg.MaxSamplesPerSlot = 1000
	}

	return &MetricAggregator{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		slots: make(map[string]*metricSlot),
	}
}

func (a *MetricAggregator) Record(metricName string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.total++

	slot, exists := a.slots[metricName]
	if !exists {
		slot = &metricSlot{
			window:  contracts.MetricWindow1Min,
			started: a.now(),
		}
		a.slots[metricName] = slot
	}

	if a.now().Sub(slot.started) > a.cfg.WindowDuration {
		slot.samples = slot.samples[:0]
		slot.started = a.now()
	}

	if len(slot.samples) >= a.cfg.MaxSamplesPerSlot {
		return
	}

	slot.samples = append(slot.samples, metricSample{
		value:    value,
		ingested: a.now(),
	})
}

func (a *MetricAggregator) ComputeAggregate(metricName string) (contracts.MetricAggregate, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	slot, exists := a.slots[metricName]
	if !exists || len(slot.samples) == 0 {
		return contracts.MetricAggregate{}, false
	}

	return computeAggregate(slot.samples), true
}

func computeAggregate(samples []metricSample) contracts.MetricAggregate {
	if len(samples) == 0 {
		return contracts.MetricAggregate{}
	}

	values := make([]float64, len(samples))
	for i, s := range samples {
		values[i] = s.value
	}
	sort.Float64s(values)

	min := values[0]
	max := values[len(values)-1]
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	avg := sum / float64(len(values))

	p95Index := int(math.Ceil(0.95*float64(len(values)))) - 1
	if p95Index < 0 {
		p95Index = 0
	}
	if p95Index >= len(values) {
		p95Index = len(values) - 1
	}
	p95 := values[p95Index]

	return contracts.MetricAggregate{
		P95:   p95,
		Max:   max,
		Min:   min,
		Avg:   avg,
		Count: int64(len(values)),
	}
}

func (a *MetricAggregator) CollectExpiredWindows() map[string]contracts.MetricAggregate {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	result := make(map[string]contracts.MetricAggregate)

	for name, slot := range a.slots {
		if len(slot.samples) == 0 {
			continue
		}
		if now.Sub(slot.started) < a.cfg.WindowDuration {
			continue
		}

		result[name] = computeAggregate(slot.samples)
		delete(a.slots, name)
	}

	return result
}

func (a *MetricAggregator) CollectAllWindows() map[string]contracts.MetricAggregate {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make(map[string]contracts.MetricAggregate)
	for name, slot := range a.slots {
		if len(slot.samples) == 0 {
			continue
		}
		result[name] = computeAggregate(slot.samples)
		delete(a.slots, name)
	}
	return result
}

func (a *MetricAggregator) BuildMetricRollup(projectID string, targetKind contracts.TargetKind, targetID, serviceName string, window contracts.MetricWindow, cpu, ram contracts.MetricAggregate, latency *contracts.MetricAggregate) contracts.MetricRollupPayload {
	payload := contracts.MetricRollupPayload{
		ProjectID:   projectID,
		TargetKind:  targetKind,
		TargetID:    targetID,
		ServiceName: serviceName,
		Window:      window,
		CPU:         cpu,
		RAM:         ram,
	}
	if latency != nil {
		payload.Latency = *latency
	}
	return payload
}

func (a *MetricAggregator) Stats() (total, activeSlots int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.total, len(a.slots)
}

func (a *MetricAggregator) PersistMetricRollup(workspaceRoot, projectID, bindingID string, rollup contracts.MetricRollupPayload) (string, error) {
	metricDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "metrics")
	if err := os.MkdirAll(metricDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create metric directory: %w", err)
	}

	timestamp := a.now().Format("20060102T150405Z")
	metricPath := filepath.Join(metricDir, "rollup_"+timestamp+".json")

	raw, err := json.MarshalIndent(rollup, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal metric rollup: %w", err)
	}

	if err := os.WriteFile(metricPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write metric rollup: %w", err)
	}

	return metricPath, nil
}

type MetricSender interface {
	SendMetricRollup(context.Context, contracts.MetricRollupPayload) error
}

type ReportMetricRollupPayload struct {
	ProjectID     string                `json:"project_id"`
	BindingID     string                `json:"binding_id"`
	RevisionID    string                `json:"revision_id"`
	RuntimeMode   contracts.RuntimeMode `json:"runtime_mode"`
	TargetKind    contracts.TargetKind  `json:"target_kind"`
	TargetID      string                `json:"target_id"`
	ServiceName   string                `json:"service_name,omitempty"`
	Force         bool                  `json:"force,omitempty"`
	WorkspaceRoot string                `json:"workspace_root"`
	MetricSender  MetricSender          `json:"-"`
}

func (a *MetricAggregator) HandleReportMetricRollup(ctx context.Context, logger *slog.Logger, payload ReportMetricRollupPayload) (int, error) {
	if logger == nil {
		logger = slog.Default()
	}

	var aggregates map[string]contracts.MetricAggregate
	if payload.Force {
		aggregates = a.CollectAllWindows()
	} else {
		aggregates = a.CollectExpiredWindows()
	}
	if len(aggregates) == 0 {
		logger.Info("no metric windows to report",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
		)
		return 0, nil
	}

	cpu, hasCPU := aggregates["cpu"]
	ram, hasRAM := aggregates["ram"]
	latency, hasLatency := aggregates["latency"]

	if !hasCPU && !hasRAM && !hasLatency {
		logger.Info("no relevant metric aggregates found",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
		)
		return 0, nil
	}

	var latencyPtr *contracts.MetricAggregate
	if hasLatency {
		latencyPtr = &latency
	}

	rollup := a.BuildMetricRollup(
		payload.ProjectID,
		payload.TargetKind,
		payload.TargetID,
		payload.ServiceName,
		contracts.MetricWindow1Min,
		cpu,
		ram,
		latencyPtr,
	)

	reported := 0
	if payload.MetricSender != nil {
		if err := payload.MetricSender.SendMetricRollup(ctx, rollup); err != nil {
			logger.Warn("could not send metric rollup to backend",
				"project_id", payload.ProjectID,
				"error", err,
			)
		} else {
			reported++
		}
	}

	workspaceRoot := payload.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(
			"/var/lib/lazyops",
			"projects", payload.ProjectID,
			"bindings", payload.BindingID,
			"revisions", payload.RevisionID,
		)
	}

	metricPath, err := a.PersistMetricRollup(workspaceRoot, payload.ProjectID, payload.BindingID, rollup)
	if err != nil {
		logger.Warn("could not persist metric rollup",
			"project_id", payload.ProjectID,
			"error", err,
		)
	} else {
		logger.Info("metric rollup collected",
			"project_id", payload.ProjectID,
			"cpu_count", cpu.Count,
			"ram_count", ram.Count,
			"metric_path", metricPath,
		)
	}

	logger.Info("metric rollup report completed",
		"project_id", payload.ProjectID,
		"binding_id", payload.BindingID,
		"reported", reported,
	)

	return reported, nil
}
