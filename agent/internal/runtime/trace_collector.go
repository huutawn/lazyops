package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type TraceHopRecord struct {
	From        string    `json:"from"`
	To          string    `json:"to"`
	Protocol    string    `json:"protocol"`
	LatencyMS   float64   `json:"latency_ms"`
	Status      string    `json:"status"`
	LocalSignal bool      `json:"local_signal"`
	RecordedAt  time.Time `json:"recorded_at"`
}

type TraceWindow struct {
	ProjectID     string           `json:"project_id"`
	CorrelationID string           `json:"correlation_id"`
	StartedAt     time.Time        `json:"started_at"`
	Hops          []TraceHopRecord `json:"hops"`
	Completed     bool             `json:"completed"`
}

type TraceCollectorConfig struct {
	SampleRate        float64
	MaxHopsPerTrace   int
	ReportingInterval time.Duration
	MaxWindowAge      time.Duration
}

func DefaultTraceCollectorConfig() TraceCollectorConfig {
	return TraceCollectorConfig{
		SampleRate:        0.1,
		MaxHopsPerTrace:   16,
		ReportingInterval: 30 * time.Second,
		MaxWindowAge:      5 * time.Minute,
	}
}

type TraceCollector struct {
	logger *slog.Logger
	cfg    TraceCollectorConfig
	now    func() time.Time

	mu      sync.Mutex
	windows map[string]*TraceWindow
	sampled int
	total   int
}

func NewTraceCollector(logger *slog.Logger, cfg TraceCollectorConfig) *TraceCollector {
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 0.1
	}
	if cfg.MaxHopsPerTrace <= 0 {
		cfg.MaxHopsPerTrace = 16
	}
	if cfg.ReportingInterval <= 0 {
		cfg.ReportingInterval = 30 * time.Second
	}
	if cfg.MaxWindowAge <= 0 {
		cfg.MaxWindowAge = 5 * time.Minute
	}

	return &TraceCollector{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		windows: make(map[string]*TraceWindow),
	}
}

func (c *TraceCollector) ShouldSample() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.total++
	if c.cfg.SampleRate >= 1.0 {
		c.sampled++
		return true
	}
	if c.cfg.SampleRate <= 0 {
		return false
	}

	if rand.Float64() < c.cfg.SampleRate {
		c.sampled++
		return true
	}
	return false
}

func (c *TraceCollector) RecordHop(correlationID, from, to, protocol string, latencyMS float64, status string, localSignal bool) {
	if correlationID == "" || from == "" || to == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	window, exists := c.windows[correlationID]
	if !exists {
		window = &TraceWindow{
			CorrelationID: correlationID,
			StartedAt:     c.now(),
		}
		c.windows[correlationID] = window
	}

	if len(window.Hops) >= c.cfg.MaxHopsPerTrace {
		return
	}

	window.Hops = append(window.Hops, TraceHopRecord{
		From:        from,
		To:          to,
		Protocol:    protocol,
		LatencyMS:   latencyMS,
		Status:      status,
		LocalSignal: localSignal,
		RecordedAt:  c.now(),
	})
}

func (c *TraceCollector) CompleteTrace(projectID, correlationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	window, exists := c.windows[correlationID]
	if !exists {
		return
	}
	window.ProjectID = projectID
	window.Completed = true
}

func (c *TraceCollector) CollectExpiredWindows() []TraceWindow {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	var expired []TraceWindow

	for id, window := range c.windows {
		if now.Sub(window.StartedAt) > c.cfg.MaxWindowAge || (window.Completed && now.Sub(window.StartedAt) > c.cfg.ReportingInterval) {
			expired = append(expired, *window)
			delete(c.windows, id)
		}
	}

	sort.Slice(expired, func(i, j int) bool {
		return expired[i].StartedAt.Before(expired[j].StartedAt)
	})

	return expired
}

func (c *TraceCollector) BuildTraceSummary(window TraceWindow) contracts.TraceSummaryPayload {
	hops := make([]contracts.TraceHopSummary, 0, len(window.Hops))
	for _, hop := range window.Hops {
		hops = append(hops, contracts.TraceHopSummary{
			From:        hop.From,
			To:          hop.To,
			Protocol:    hop.Protocol,
			LatencyMS:   hop.LatencyMS,
			Status:      hop.Status,
			LocalSignal: hop.LocalSignal,
		})
	}

	return contracts.TraceSummaryPayload{
		ProjectID:     window.ProjectID,
		CorrelationID: window.CorrelationID,
		StartedAt:     window.StartedAt,
		EndedAt:       c.now(),
		Hops:          hops,
	}
}

func (c *TraceCollector) Stats() (total, sampled int, activeWindows int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total, c.sampled, len(c.windows)
}

func (c *TraceCollector) PersistTraceWindow(workspaceRoot, projectID, bindingID, correlationID string, window TraceWindow) (string, error) {
	traceDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "traces")
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create trace directory: %w", err)
	}

	sanitized := sanitizePathToken(correlationID)
	tracePath := filepath.Join(traceDir, sanitized+".json")

	raw, err := json.MarshalIndent(window, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal trace window: %w", err)
	}

	if err := os.WriteFile(tracePath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write trace window: %w", err)
	}

	return tracePath, nil
}

type ReportTraceSummaryPayload struct {
	ProjectID     string                `json:"project_id"`
	BindingID     string                `json:"binding_id"`
	RevisionID    string                `json:"revision_id"`
	RuntimeMode   contracts.RuntimeMode `json:"runtime_mode"`
	WorkspaceRoot string                `json:"workspace_root"`
	TraceSender   TraceSender           `json:"-"`
}

func (c *TraceCollector) HandleReportTraceSummary(ctx context.Context, logger *slog.Logger, payload ReportTraceSummaryPayload) (int, error) {
	if logger == nil {
		logger = slog.Default()
	}

	expired := c.CollectExpiredWindows()
	if len(expired) == 0 {
		logger.Info("no trace windows to report",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
		)
		return 0, nil
	}

	reported := 0
	for _, window := range expired {
		if window.ProjectID == "" {
			window.ProjectID = payload.ProjectID
		}

		summary := c.BuildTraceSummary(window)

		if payload.TraceSender != nil {
			if err := payload.TraceSender.SendTraceSummary(ctx, summary); err != nil {
				logger.Warn("could not send trace summary to backend",
					"correlation_id", window.CorrelationID,
					"error", err,
				)
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

		tracePath, err := c.PersistTraceWindow(workspaceRoot, payload.ProjectID, payload.BindingID, window.CorrelationID, window)
		if err != nil {
			logger.Warn("could not persist trace window",
				"correlation_id", window.CorrelationID,
				"error", err,
			)
			continue
		}

		logger.Info("trace summary collected",
			"correlation_id", window.CorrelationID,
			"project_id", payload.ProjectID,
			"hops", len(window.Hops),
			"trace_path", tracePath,
		)
		reported++
	}

	logger.Info("trace summary report completed",
		"project_id", payload.ProjectID,
		"binding_id", payload.BindingID,
		"reported", reported,
		"total_windows", len(expired),
	)

	return reported, nil
}
