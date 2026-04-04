package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type IncidentReporterConfig struct {
	MaxIncidentsPerWindow int
	CooldownDuration      time.Duration
}

func DefaultIncidentReporterConfig() IncidentReporterConfig {
	return IncidentReporterConfig{
		MaxIncidentsPerWindow: 10,
		CooldownDuration:      30 * time.Second,
	}
}

type incidentRecord struct {
	incident contracts.IncidentPayload
	reported time.Time
}

type IncidentReporter struct {
	logger *slog.Logger
	cfg    IncidentReporterConfig
	now    func() time.Time

	mu         sync.Mutex
	pending    []incidentRecord
	cooldowns  map[string]time.Time
	total      int
	forwarded  int
	suppressed int
}

func NewIncidentReporter(logger *slog.Logger, cfg IncidentReporterConfig) *IncidentReporter {
	if cfg.MaxIncidentsPerWindow <= 0 {
		cfg.MaxIncidentsPerWindow = 10
	}
	if cfg.CooldownDuration <= 0 {
		cfg.CooldownDuration = 30 * time.Second
	}

	return &IncidentReporter{
		logger: logger,
		cfg:    cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		cooldowns: make(map[string]time.Time),
	}
}

func (r *IncidentReporter) Report(incident contracts.IncidentPayload) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.total++

	cooldownKey := incident.Kind + ":" + incident.Summary
	if last, ok := r.cooldowns[cooldownKey]; ok && r.now().Sub(last) < r.cfg.CooldownDuration {
		r.suppressed++
		return
	}

	if len(r.pending) >= r.cfg.MaxIncidentsPerWindow {
		r.suppressed++
		return
	}

	r.pending = append(r.pending, incidentRecord{
		incident: incident,
		reported: r.now(),
	})
	r.cooldowns[cooldownKey] = r.now()
}

func (r *IncidentReporter) CollectPending() []contracts.IncidentPayload {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.pending) == 0 {
		return nil
	}

	incidents := make([]contracts.IncidentPayload, 0, len(r.pending))
	for _, rec := range r.pending {
		incidents = append(incidents, rec.incident)
	}

	r.pending = r.pending[:0]
	return incidents
}

func (r *IncidentReporter) Stats() (total, forwarded, suppressed, pending int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.total, r.forwarded, r.suppressed, len(r.pending)
}

func (r *IncidentReporter) PersistIncident(workspaceRoot, projectID, bindingID string, incident contracts.IncidentPayload) (string, error) {
	incidentDir := filepath.Join(workspaceRoot, "projects", projectID, "bindings", bindingID, "incidents")
	if err := os.MkdirAll(incidentDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create incident directory: %w", err)
	}

	timestamp := incident.OccurredAt.Format("20060102T150405Z")
	sanitized := sanitizePathToken(incident.Kind)
	incidentPath := filepath.Join(incidentDir, sanitized+"_"+timestamp+".json")

	raw, err := json.MarshalIndent(incident, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal incident: %w", err)
	}

	if err := os.WriteFile(incidentPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write incident: %w", err)
	}

	return incidentPath, nil
}

type IncidentSender interface {
	SendIncident(context.Context, contracts.IncidentPayload) error
}

type ReportIncidentPayload struct {
	ProjectID      string                `json:"project_id"`
	BindingID      string                `json:"binding_id"`
	RevisionID     string                `json:"revision_id"`
	RuntimeMode    contracts.RuntimeMode `json:"runtime_mode"`
	WorkspaceRoot  string                `json:"workspace_root"`
	IncidentSender IncidentSender        `json:"-"`
}

func (r *IncidentReporter) HandleReportIncidents(ctx context.Context, logger *slog.Logger, payload ReportIncidentPayload) (int, error) {
	if logger == nil {
		logger = slog.Default()
	}

	incidents := r.CollectPending()
	if len(incidents) == 0 {
		logger.Info("no incidents to report",
			"project_id", payload.ProjectID,
			"binding_id", payload.BindingID,
		)
		return 0, nil
	}

	reported := 0
	for _, incident := range incidents {
		if payload.IncidentSender != nil {
			if err := payload.IncidentSender.SendIncident(ctx, incident); err != nil {
				logger.Warn("could not send incident to backend",
					"kind", incident.Kind,
					"severity", incident.Severity,
					"error", err,
				)
				continue
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

		incidentPath, err := r.PersistIncident(workspaceRoot, payload.ProjectID, payload.BindingID, incident)
		if err != nil {
			logger.Warn("could not persist incident",
				"kind", incident.Kind,
				"error", err,
			)
			continue
		}

		logger.Info("incident collected",
			"kind", incident.Kind,
			"severity", incident.Severity,
			"incident_path", incidentPath,
		)
		reported++
	}

	r.mu.Lock()
	r.forwarded += reported
	r.mu.Unlock()

	logger.Info("incident report completed",
		"project_id", payload.ProjectID,
		"binding_id", payload.BindingID,
		"reported", reported,
		"total_incidents", len(incidents),
	)

	return reported, nil
}
