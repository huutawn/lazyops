package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	mu               sync.Mutex
	slots            map[string]*metricSlot
	accessLogOffsets map[string]int64
	total            int
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
		slots:            make(map[string]*metricSlot),
		accessLogOffsets: make(map[string]int64),
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

	return computeAggregate(metricName, slot.samples), true
}

func computeAggregate(metricName string, samples []metricSample) contracts.MetricAggregate {
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
		Count: aggregateCount(metricName, sum, len(values)),
	}
}

func aggregateCount(metricName string, sum float64, sampleCount int) int64 {
	if strings.TrimSpace(metricName) == MetricNameRequestCount {
		total := int64(math.Round(sum))
		if total < 0 {
			return 0
		}
		return total
	}
	return int64(sampleCount)
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

		result[name] = computeAggregate(name, slot.samples)
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
		result[name] = computeAggregate(name, slot.samples)
		delete(a.slots, name)
	}
	return result
}

func (a *MetricAggregator) BuildMetricRollup(
	projectID string,
	targetKind contracts.TargetKind,
	targetID, serviceName string,
	window contracts.MetricWindow,
	cpu, ram, disk, networkIn, networkOut contracts.MetricAggregate,
	requestCount, latency *contracts.MetricAggregate,
) contracts.MetricRollupPayload {
	payload := contracts.MetricRollupPayload{
		ProjectID:   projectID,
		TargetKind:  targetKind,
		TargetID:    targetID,
		ServiceName: serviceName,
		Window:      window,
		CPU:         cpu,
		RAM:         ram,
		Disk:        disk,
		NetworkIn:   networkIn,
		NetworkOut:  networkOut,
	}
	if requestCount != nil {
		payload.RequestCount = *requestCount
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

const (
	MetricNameCPU          = "cpu"
	MetricNameRAM          = "ram"
	MetricNameDisk         = "disk"
	MetricNameNetworkIn    = "network_in"
	MetricNameNetworkOut   = "network_out"
	MetricNameRequestCount = "request_count"
	MetricNameLatency      = "latency"
)

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
	requestCount, hasRequestCount, err := a.consumeAccessLogRequestCount(payload.WorkspaceRoot)
	if err != nil {
		logger.Warn("could not consume gateway access log request count",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
			"error", err,
		)
	}

	if len(aggregates) == 0 && !hasRequestCount {
		logger.Info("no metric windows to report",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
		)
		return 0, nil
	}

	cpu, hasCPU := aggregates[MetricNameCPU]
	ram, hasRAM := aggregates[MetricNameRAM]
	disk, hasDisk := aggregates[MetricNameDisk]
	networkIn, hasNetworkIn := aggregates[MetricNameNetworkIn]
	networkOut, hasNetworkOut := aggregates[MetricNameNetworkOut]
	latency, hasLatency := aggregates[MetricNameLatency]

	if !hasRequestCount {
		if aggregate, ok := aggregates[MetricNameRequestCount]; ok && aggregate.Count > 0 {
			requestCount = aggregate
			hasRequestCount = true
		}
	}

	if !hasCPU && !hasRAM && !hasDisk && !hasNetworkIn && !hasNetworkOut && !hasRequestCount && !hasLatency {
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
	var requestCountPtr *contracts.MetricAggregate
	if hasRequestCount {
		requestCountPtr = &requestCount
	}

	rollup := a.BuildMetricRollup(
		payload.ProjectID,
		payload.TargetKind,
		payload.TargetID,
		payload.ServiceName,
		contracts.MetricWindow1Min,
		cpu,
		ram,
		disk,
		networkIn,
		networkOut,
		requestCountPtr,
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
			"disk_count", disk.Count,
			"network_in_count", networkIn.Count,
			"network_out_count", networkOut.Count,
			"request_count", rollup.RequestCount.Count,
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

func (a *MetricAggregator) consumeAccessLogRequestCount(workspaceRoot string) (contracts.MetricAggregate, bool, error) {
	accessLogPath := resolveAccessLogPath(workspaceRoot)
	if accessLogPath == "" {
		return contracts.MetricAggregate{}, false, nil
	}

	count, err := a.countNewAccessLogEntries(accessLogPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return contracts.MetricAggregate{}, false, nil
		}
		return contracts.MetricAggregate{}, false, err
	}
	if count <= 0 {
		return contracts.MetricAggregate{}, false, nil
	}

	value := float64(count)
	return contracts.MetricAggregate{
		P95:   value,
		Max:   value,
		Min:   value,
		Avg:   value,
		Count: count,
	}, true, nil
}

func resolveAccessLogPath(workspaceRoot string) string {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return ""
	}

	root = filepath.Clean(root)
	candidates := []string{
		filepath.Join(root, "gateway", "live", "access.log"),
		filepath.Join(filepath.Dir(filepath.Dir(root)), "gateway", "live", "access.log"),
		filepath.Join(root, "live", "access.log"),
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return candidates[1]
}

func (a *MetricAggregator) countNewAccessLogEntries(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	a.mu.Lock()
	offset := a.accessLogOffsets[path]
	if info.Size() < offset {
		offset = 0
	}
	a.mu.Unlock()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	reader := bufio.NewReader(file)
	var count int64
	currentOffset := offset
	for {
		line, readErr := reader.ReadBytes('\n')
		currentOffset += int64(len(line))
		if strings.TrimSpace(string(line)) != "" {
			count++
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return 0, readErr
		}
	}

	a.mu.Lock()
	a.accessLogOffsets[path] = currentOffset
	a.mu.Unlock()

	return count, nil
}
