package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	MetricKindCPU            = "cpu"
	MetricKindMemory         = "memory"
	MetricKindDisk           = "disk"
	MetricKindNetworkIn      = "network_in"
	MetricKindNetworkOut     = "network_out"
	MetricKindRequestLatency = "request_latency"
	MetricKindRequestCount   = "request_count"

	ScaleToZeroStateActive      = "active"
	ScaleToZeroStateScalingDown = "scaling_down"
	ScaleToZeroStateScaledDown  = "scaled_down"
	ScaleToZeroStateWakingUp    = "waking_up"

	FinOpsWindow1h  = "1h"
	FinOpsWindow6h  = "6h"
	FinOpsWindow24h = "24h"
	FinOpsWindow7d  = "7d"
)

var (
	ErrRawMetricRejected     = errors.New("raw metric samples rejected: only aggregate rollups accepted")
	ErrInvalidMetricWindow   = errors.New("invalid metric window duration")
	ErrScaleToZeroNotEnabled = errors.New("scale-to-zero not enabled for this service")
)

var allowedMetricKinds = map[string]struct{}{
	MetricKindCPU:            {},
	MetricKindMemory:         {},
	MetricKindDisk:           {},
	MetricKindNetworkIn:      {},
	MetricKindNetworkOut:     {},
	MetricKindRequestLatency: {},
	MetricKindRequestCount:   {},
}

var allowedFinOpsWindows = map[string]time.Duration{
	FinOpsWindow1h:  1 * time.Hour,
	FinOpsWindow6h:  6 * time.Hour,
	FinOpsWindow24h: 24 * time.Hour,
	FinOpsWindow7d:  7 * 24 * time.Hour,
}

type FinOpsService struct {
	metricRollups MetricRollupStore
	scaleToZero   ScaleToZeroStore
	instances     InstanceStore
	meshes        MeshNetworkStore
}

type MetricRollupStore interface {
	Create(rollup *models.MetricRollup) error
	ListByProjectAndService(projectID, serviceName string, windowStart, windowEnd time.Time) ([]models.MetricRollup, error)
	ListByProject(projectID string, limit int) ([]models.MetricRollup, error)
}

type ScaleToZeroStore interface {
	Upsert(state *models.ScaleToZeroState) error
	GetByService(projectID, serviceName string) (*models.ScaleToZeroState, error)
	ListByProject(projectID string) ([]models.ScaleToZeroState, error)
}

func NewFinOpsService(
	metricRollups MetricRollupStore,
	scaleToZero ScaleToZeroStore,
	instances InstanceStore,
	meshes MeshNetworkStore,
) *FinOpsService {
	return &FinOpsService{
		metricRollups: metricRollups,
		scaleToZero:   scaleToZero,
		instances:     instances,
		meshes:        meshes,
	}
}

func (s *FinOpsService) IngestMetricRollup(ctx context.Context, cmd IngestMetricRollupCommand) (*MetricRollupRecord, error) {
	if cmd.IsRawSample {
		return nil, ErrRawMetricRejected
	}

	metricKind := strings.TrimSpace(cmd.MetricKind)
	if _, ok := allowedMetricKinds[metricKind]; !ok {
		return nil, fmt.Errorf("invalid metric kind %q", metricKind)
	}

	windowStart := cmd.WindowStart
	windowEnd := cmd.WindowEnd
	if windowEnd.Before(windowStart) {
		return nil, ErrInvalidMetricWindow
	}

	metadata := cmd.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, _ := json.Marshal(metadata)

	rollup := &models.MetricRollup{
		ID:           utils.NewPrefixedID("met"),
		ProjectID:    strings.TrimSpace(cmd.ProjectID),
		ServiceName:  strings.TrimSpace(cmd.ServiceName),
		MetricKind:   metricKind,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
		P95:          cmd.P95,
		Max:          cmd.Max,
		Min:          cmd.Min,
		Avg:          cmd.Avg,
		Count:        cmd.Count,
		MetadataJSON: string(metadataJSON),
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.metricRollups.Create(rollup); err != nil {
		return nil, err
	}

	r := toMetricRollupRecord(*rollup)
	return &r, nil
}

func (s *FinOpsService) QueryMetricRollups(ctx context.Context, projectID, serviceName, metricKind string, window string) ([]MetricRollupRecord, error) {
	duration, ok := allowedFinOpsWindows[window]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidMetricWindow, window)
	}

	now := time.Now().UTC()
	windowStart := now.Add(-duration)

	rollups, err := s.metricRollups.ListByProjectAndService(projectID, serviceName, windowStart, now)
	if err != nil {
		return nil, err
	}

	if metricKind != "" {
		filtered := make([]models.MetricRollup, 0)
		for _, r := range rollups {
			if r.MetricKind == metricKind {
				filtered = append(filtered, r)
			}
		}
		rollups = filtered
	}

	out := make([]MetricRollupRecord, len(rollups))
	for i, r := range rollups {
		out[i] = toMetricRollupRecord(r)
	}
	return out, nil
}

func (s *FinOpsService) GetFinOpsSummary(ctx context.Context, projectID string) (*FinOpsSummary, error) {
	rollups, err := s.metricRollups.ListByProject(projectID, 100)
	if err != nil {
		return nil, err
	}

	summary := &FinOpsSummary{
		ProjectID: projectID,
		Services:  make(map[string]ServiceFinOpsSummary),
		HotNodes:  make([]HotNodeSummary, 0),
		HotEdges:  make([]HotEdgeSummary, 0),
	}

	serviceMetrics := make(map[string][]models.MetricRollup)
	for _, r := range rollups {
		serviceMetrics[r.ServiceName] = append(serviceMetrics[r.ServiceName], r)
	}

	for svcName, metrics := range serviceMetrics {
		svcSummary := computeServiceFinOpsSummary(svcName, metrics)
		summary.Services[svcName] = svcSummary

		if svcSummary.AvgCPU > 80 {
			summary.HotNodes = append(summary.HotNodes, HotNodeSummary{
				ServiceName: svcName,
				AvgCPU:      svcSummary.AvgCPU,
				AvgMemory:   svcSummary.AvgMemory,
				RequestRate: svcSummary.RequestRate,
			})
		}
	}

	return summary, nil
}

func (s *FinOpsService) UpdateScaleToZeroState(ctx context.Context, projectID, serviceName, newState string) (*ScaleToZeroStateRecord, error) {
	state, err := s.scaleToZero.GetByService(projectID, serviceName)
	if err != nil {
		return nil, err
	}

	if state == nil {
		return nil, ErrScaleToZeroNotEnabled
	}

	now := time.Now().UTC()
	if err := s.scaleToZero.Upsert(&models.ScaleToZeroState{
		ID:                state.ID,
		ProjectID:         projectID,
		ServiceName:       serviceName,
		State:             normalizeScaleToZeroState(newState),
		LastStateChangeAt: now,
		WakeTimeoutAt:     computeWakeTimeout(newState, now),
		UpdatedAt:         now,
	}); err != nil {
		return nil, err
	}

	updated, _ := s.scaleToZero.GetByService(projectID, serviceName)
	return toScaleToZeroStateRecord(*updated), nil
}

func (s *FinOpsService) EnableScaleToZero(ctx context.Context, projectID, serviceName string, wakeTimeoutMs int) (*ScaleToZeroStateRecord, error) {
	now := time.Now().UTC()

	state := &models.ScaleToZeroState{
		ID:                utils.NewPrefixedID("s2z"),
		ProjectID:         projectID,
		ServiceName:       serviceName,
		State:             ScaleToZeroStateActive,
		Enabled:           true,
		WakeTimeoutMs:     wakeTimeoutMs,
		LastStateChangeAt: now,
		UpdatedAt:         now,
	}

	if err := s.scaleToZero.Upsert(state); err != nil {
		return nil, err
	}

	return toScaleToZeroStateRecord(*state), nil
}

func (s *FinOpsService) DisableScaleToZero(ctx context.Context, projectID, serviceName string) (*ScaleToZeroStateRecord, error) {
	state, err := s.scaleToZero.GetByService(projectID, serviceName)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, ErrScaleToZeroNotEnabled
	}

	now := time.Now().UTC()
	state.Enabled = false
	state.State = ScaleToZeroStateActive
	state.UpdatedAt = now

	if err := s.scaleToZero.Upsert(state); err != nil {
		return nil, err
	}

	return toScaleToZeroStateRecord(*state), nil
}

func (s *FinOpsService) CheckWakeTimeout(ctx context.Context, projectID, serviceName string) (*WakeTimeoutResult, error) {
	state, err := s.scaleToZero.GetByService(projectID, serviceName)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, ErrScaleToZeroNotEnabled
	}
	if !state.Enabled {
		return nil, ErrScaleToZeroNotEnabled
	}

	now := time.Now().UTC()
	timedOut := false
	if state.WakeTimeoutAt != nil && now.After(*state.WakeTimeoutAt) {
		timedOut = true
	}

	return &WakeTimeoutResult{
		ServiceName:   serviceName,
		State:         state.State,
		Enabled:       state.Enabled,
		WakeTimeoutMs: state.WakeTimeoutMs,
		TimedOut:      timedOut,
		CheckedAt:     now,
	}, nil
}

type IngestMetricRollupCommand struct {
	ProjectID   string
	ServiceName string
	MetricKind  string
	WindowStart time.Time
	WindowEnd   time.Time
	P95         float64
	Max         float64
	Min         float64
	Avg         float64
	Count       int64
	IsRawSample bool
	Metadata    map[string]any
}

type MetricRollupRecord struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"project_id"`
	ServiceName string         `json:"service_name"`
	MetricKind  string         `json:"metric_kind"`
	WindowStart time.Time      `json:"window_start"`
	WindowEnd   time.Time      `json:"window_end"`
	P95         float64        `json:"p95"`
	Max         float64        `json:"max"`
	Min         float64        `json:"min"`
	Avg         float64        `json:"avg"`
	Count       int64          `json:"count"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type FinOpsSummary struct {
	ProjectID string                          `json:"project_id"`
	Services  map[string]ServiceFinOpsSummary `json:"services"`
	HotNodes  []HotNodeSummary                `json:"hot_nodes"`
	HotEdges  []HotEdgeSummary                `json:"hot_edges"`
}

type ServiceFinOpsSummary struct {
	ServiceName   string  `json:"service_name"`
	AvgCPU        float64 `json:"avg_cpu"`
	AvgMemory     float64 `json:"avg_memory"`
	P95Latency    float64 `json:"p95_latency"`
	RequestRate   float64 `json:"request_rate"`
	TotalRequests int64   `json:"total_requests"`
}

type HotNodeSummary struct {
	ServiceName string  `json:"service_name"`
	AvgCPU      float64 `json:"avg_cpu"`
	AvgMemory   float64 `json:"avg_memory"`
	RequestRate float64 `json:"request_rate"`
}

type HotEdgeSummary struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	AvgLatency float64 `json:"avg_latency"`
	ErrorRate  float64 `json:"error_rate"`
}

type ScaleToZeroStateRecord struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"project_id"`
	ServiceName       string     `json:"service_name"`
	State             string     `json:"state"`
	Enabled           bool       `json:"enabled"`
	WakeTimeoutMs     int        `json:"wake_timeout_ms"`
	LastStateChangeAt time.Time  `json:"last_state_change_at"`
	WakeTimeoutAt     *time.Time `json:"wake_timeout_at,omitempty"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type WakeTimeoutResult struct {
	ServiceName   string    `json:"service_name"`
	State         string    `json:"state"`
	Enabled       bool      `json:"enabled"`
	WakeTimeoutMs int       `json:"wake_timeout_ms"`
	TimedOut      bool      `json:"timed_out"`
	CheckedAt     time.Time `json:"checked_at"`
}

func toMetricRollupRecord(item models.MetricRollup) MetricRollupRecord {
	var metadata map[string]any
	if item.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(item.MetadataJSON), &metadata)
	}
	return MetricRollupRecord{
		ID:          item.ID,
		ProjectID:   item.ProjectID,
		ServiceName: item.ServiceName,
		MetricKind:  item.MetricKind,
		WindowStart: item.WindowStart,
		WindowEnd:   item.WindowEnd,
		P95:         item.P95,
		Max:         item.Max,
		Min:         item.Min,
		Avg:         item.Avg,
		Count:       item.Count,
		Metadata:    metadata,
		CreatedAt:   item.CreatedAt,
	}
}

func toScaleToZeroStateRecord(item models.ScaleToZeroState) *ScaleToZeroStateRecord {
	return &ScaleToZeroStateRecord{
		ID:                item.ID,
		ProjectID:         item.ProjectID,
		ServiceName:       item.ServiceName,
		State:             item.State,
		Enabled:           item.Enabled,
		WakeTimeoutMs:     item.WakeTimeoutMs,
		LastStateChangeAt: item.LastStateChangeAt,
		WakeTimeoutAt:     item.WakeTimeoutAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func computeServiceFinOpsSummary(serviceName string, rollups []models.MetricRollup) ServiceFinOpsSummary {
	summary := ServiceFinOpsSummary{ServiceName: serviceName}

	var cpuSum, memSum, latencyP95Sum float64
	var cpuCount, memCount, latencyCount int
	var totalRequests int64

	for _, r := range rollups {
		switch r.MetricKind {
		case MetricKindCPU:
			cpuSum += r.Avg
			cpuCount++
		case MetricKindMemory:
			memSum += r.Avg
			memCount++
		case MetricKindRequestLatency:
			latencyP95Sum += r.P95
			latencyCount++
		case MetricKindRequestCount:
			totalRequests += r.Count
		}
	}

	if cpuCount > 0 {
		summary.AvgCPU = cpuSum / float64(cpuCount)
	}
	if memCount > 0 {
		summary.AvgMemory = memSum / float64(memCount)
	}
	if latencyCount > 0 {
		summary.P95Latency = latencyP95Sum / float64(latencyCount)
	}
	summary.TotalRequests = totalRequests

	return summary
}

func normalizeScaleToZeroState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ScaleToZeroStateActive:
		return ScaleToZeroStateActive
	case ScaleToZeroStateScalingDown:
		return ScaleToZeroStateScalingDown
	case ScaleToZeroStateScaledDown:
		return ScaleToZeroStateScaledDown
	case ScaleToZeroStateWakingUp:
		return ScaleToZeroStateWakingUp
	default:
		return ScaleToZeroStateActive
	}
}

func computeWakeTimeout(state string, now time.Time) *time.Time {
	if state != ScaleToZeroStateWakingUp {
		return nil
	}
	t := now.Add(30 * time.Second)
	return &t
}
