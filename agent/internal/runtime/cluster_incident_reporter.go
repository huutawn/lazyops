package runtime

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type ClusterIncidentReporter struct {
	logger *slog.Logger
	now    func() time.Time

	mu         sync.Mutex
	incidents  []ClusterIncident
	total      int
	suppressed int
	cooldowns  map[string]time.Time
}

type ClusterIncident struct {
	Kind       string             `json:"kind"`
	Severity   contracts.Severity `json:"severity"`
	Summary    string             `json:"summary"`
	NodeName   string             `json:"node_name,omitempty"`
	PodName    string             `json:"pod_name,omitempty"`
	Namespace  string             `json:"namespace,omitempty"`
	Details    map[string]any     `json:"details,omitempty"`
	OccurredAt time.Time          `json:"occurred_at"`
}

func NewClusterIncidentReporter(logger *slog.Logger) *ClusterIncidentReporter {
	return &ClusterIncidentReporter{
		logger: logger,
		now: func() time.Time {
			return time.Now().UTC()
		},
		cooldowns: make(map[string]time.Time),
	}
}

func (r *ClusterIncidentReporter) Report(incident ClusterIncident) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.total++
	incident.OccurredAt = r.now()

	cooldownKey := incident.Kind + ":" + incident.Summary
	if last, ok := r.cooldowns[cooldownKey]; ok && r.now().Sub(last) < 30*time.Second {
		r.suppressed++
		return
	}

	r.incidents = append(r.incidents, incident)
	r.cooldowns[cooldownKey] = r.now()
}

func (r *ClusterIncidentReporter) ReportUnhealthyNode(nodeName string, details map[string]any) {
	r.Report(ClusterIncident{
		Kind:     "unhealthy_node",
		Severity: contracts.SeverityWarning,
		Summary:  "node " + nodeName + " is unhealthy",
		NodeName: nodeName,
		Details:  details,
	})
}

func (r *ClusterIncidentReporter) ReportPodCrashLoop(podName, namespace string, restartCount int) {
	r.Report(ClusterIncident{
		Kind:      "pod_crash_loop",
		Severity:  contracts.SeverityCritical,
		Summary:   "pod " + podName + " in crash loop",
		PodName:   podName,
		Namespace: namespace,
		Details: map[string]any{
			"restart_count": restartCount,
		},
	})
}

func (r *ClusterIncidentReporter) ReportMeshIssue(summary string, details map[string]any) {
	r.Report(ClusterIncident{
		Kind:     "mesh_issue",
		Severity: contracts.SeverityWarning,
		Summary:  summary,
		Details:  details,
	})
}

func (r *ClusterIncidentReporter) ReportTunnelIssue(summary string, details map[string]any) {
	r.Report(ClusterIncident{
		Kind:     "tunnel_issue",
		Severity: contracts.SeverityWarning,
		Summary:  summary,
		Details:  details,
	})
}

func (r *ClusterIncidentReporter) CollectIncidents() []ClusterIncident {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.incidents) == 0 {
		return nil
	}

	incidents := make([]ClusterIncident, len(r.incidents))
	copy(incidents, r.incidents)
	r.incidents = r.incidents[:0]

	return incidents
}

func (r *ClusterIncidentReporter) Stats() (total, suppressed, pending int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.total, r.suppressed, len(r.incidents)
}

func (r *ClusterIncidentReporter) PersistIncidents(workspaceRoot string, incidents []ClusterIncident) (string, error) {
	if len(incidents) == 0 {
		return "", nil
	}

	incidentDir := filepath.Join(workspaceRoot, "node-agent", "cluster-incidents")
	if err := os.MkdirAll(incidentDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create cluster incident directory: %w", err)
	}

	timestamp := r.now().Format("20060102T150405Z")
	incidentPath := filepath.Join(incidentDir, "incidents_"+timestamp+".json")

	raw, err := json.MarshalIndent(incidents, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not marshal cluster incidents: %w", err)
	}

	if err := os.WriteFile(incidentPath, raw, 0o644); err != nil {
		return "", fmt.Errorf("could not write cluster incidents: %w", err)
	}

	return incidentPath, nil
}
