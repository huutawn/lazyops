package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testIncidentReporter() *IncidentReporter {
	r := NewIncidentReporter(nil, IncidentReporterConfig{
		MaxIncidentsPerWindow: 10,
		CooldownDuration:      1 * time.Second,
	})
	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	}
	return r
}

func TestIncidentReporterDefaultConfig(t *testing.T) {
	r := NewIncidentReporter(nil, IncidentReporterConfig{})
	if r.cfg.MaxIncidentsPerWindow != 10 {
		t.Fatalf("expected default max incidents 10, got %d", r.cfg.MaxIncidentsPerWindow)
	}
	if r.cfg.CooldownDuration != 30*time.Second {
		t.Fatalf("expected default cooldown 30s, got %s", r.cfg.CooldownDuration)
	}
}

func TestIncidentReporterReport(t *testing.T) {
	r := testIncidentReporter()
	r.Report(contracts.IncidentPayload{
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_unhealthy",
		Summary:    "health gate failed",
		OccurredAt: r.now(),
	})

	total, forwarded, suppressed, pending := r.Stats()
	if total != 1 {
		t.Fatalf("expected 1 total, got %d", total)
	}
	if forwarded != 0 {
		t.Fatalf("expected 0 forwarded, got %d", forwarded)
	}
	if suppressed != 0 {
		t.Fatalf("expected 0 suppressed, got %d", suppressed)
	}
	if pending != 1 {
		t.Fatalf("expected 1 pending, got %d", pending)
	}
}

func TestIncidentReporterCooldown(t *testing.T) {
	r := testIncidentReporter()
	r.Report(contracts.IncidentPayload{
		ProjectID:  "prj_123",
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_unhealthy",
		Summary:    "health gate failed",
		OccurredAt: r.now(),
	})

	r.Report(contracts.IncidentPayload{
		ProjectID:  "prj_123",
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_unhealthy",
		Summary:    "health gate failed",
		OccurredAt: r.now(),
	})

	total, _, suppressed, pending := r.Stats()
	if total != 2 {
		t.Fatalf("expected 2 total, got %d", total)
	}
	if suppressed != 1 {
		t.Fatalf("expected 1 suppressed due to cooldown, got %d", suppressed)
	}
	if pending != 1 {
		t.Fatalf("expected 1 pending, got %d", pending)
	}
}

func TestIncidentReporterMaxPerWindow(t *testing.T) {
	r := NewIncidentReporter(nil, IncidentReporterConfig{
		MaxIncidentsPerWindow: 3,
		CooldownDuration:      0,
	})
	r.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	}

	for i := 0; i < 5; i++ {
		r.Report(contracts.IncidentPayload{
			ProjectID:  "prj_123",
			Severity:   contracts.SeverityWarning,
			Kind:       "slow_response",
			Summary:    fmt.Sprintf("response time exceeded threshold %d", i),
			OccurredAt: r.now(),
		})
	}

	total, _, suppressed, pending := r.Stats()
	if total != 5 {
		t.Fatalf("expected 5 total, got %d", total)
	}
	if suppressed != 2 {
		t.Fatalf("expected 2 suppressed due to max window, got %d", suppressed)
	}
	if pending != 3 {
		t.Fatalf("expected 3 pending, got %d", pending)
	}
}

func TestIncidentReporterCollectPending(t *testing.T) {
	r := testIncidentReporter()
	r.Report(contracts.IncidentPayload{
		ProjectID:  "prj_123",
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_unhealthy",
		Summary:    "health gate failed",
		OccurredAt: r.now(),
	})

	incidents := r.CollectPending()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}
	if incidents[0].Kind != "deployment_unhealthy" {
		t.Fatalf("expected kind deployment_unhealthy, got %q", incidents[0].Kind)
	}

	_, _, _, pending := r.Stats()
	if pending != 0 {
		t.Fatalf("expected 0 pending after collection, got %d", pending)
	}
}

func TestIncidentReporterCollectPendingEmpty(t *testing.T) {
	r := testIncidentReporter()
	incidents := r.CollectPending()
	if len(incidents) != 0 {
		t.Fatalf("expected 0 incidents, got %d", len(incidents))
	}
}

func TestIncidentReporterPersistIncident(t *testing.T) {
	r := testIncidentReporter()
	incident := contracts.IncidentPayload{
		ProjectID:  "prj_123",
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_unhealthy",
		Summary:    "health gate failed",
		OccurredAt: r.now(),
	}

	root := filepath.Join(t.TempDir(), "runtime-root")
	incidentPath, err := r.PersistIncident(root, "prj_123", "bind_123", incident)
	if err != nil {
		t.Fatalf("persist incident: %v", err)
	}

	if _, err := os.Stat(incidentPath); err != nil {
		t.Fatalf("expected incident file to exist: %v", err)
	}

	var loaded contracts.IncidentPayload
	raw, err := os.ReadFile(incidentPath)
	if err != nil {
		t.Fatalf("read incident file: %v", err)
	}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("decode incident file: %v", err)
	}
	if loaded.Kind != "deployment_unhealthy" {
		t.Fatalf("expected kind deployment_unhealthy, got %q", loaded.Kind)
	}
}

func TestIncidentReporterHandleReportIncidents(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	r := testIncidentReporter()
	r.Report(contracts.IncidentPayload{
		ProjectID:  "prj_123",
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_unhealthy",
		Summary:    "health gate failed",
		OccurredAt: r.now(),
	})

	reported, err := r.HandleReportIncidents(context.Background(), nil, ReportIncidentPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: root,
	})
	if err != nil {
		t.Fatalf("handle report incidents: %v", err)
	}
	if reported != 1 {
		t.Fatalf("expected 1 reported, got %d", reported)
	}

	incidentPath := filepath.Join(root, "projects", "prj_123", "bindings", "bind_123", "incidents", "deployment_unhealthy_20260404T120000Z.json")
	if _, err := os.Stat(incidentPath); err != nil {
		t.Fatalf("expected incident file at %s: %v", incidentPath, err)
	}
}

func TestIncidentReporterHandleReportIncidentsEmpty(t *testing.T) {
	r := testIncidentReporter()

	reported, err := r.HandleReportIncidents(context.Background(), nil, ReportIncidentPayload{
		ProjectID:     "prj_123",
		BindingID:     "bind_123",
		RevisionID:    "rev_123",
		RuntimeMode:   contracts.RuntimeModeStandalone,
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("handle report incidents: %v", err)
	}
	if reported != 0 {
		t.Fatalf("expected 0 reported, got %d", reported)
	}
}

type fakeIncidentSender struct {
	mu      sync.Mutex
	sent    []contracts.IncidentPayload
	sendErr error
}

func (f *fakeIncidentSender) SendIncident(_ context.Context, payload contracts.IncidentPayload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, payload)
	return f.sendErr
}

func TestIncidentReporterHandleReportIncidentsWithSender(t *testing.T) {
	r := testIncidentReporter()
	r.Report(contracts.IncidentPayload{
		ProjectID:  "prj_123",
		Severity:   contracts.SeverityCritical,
		Kind:       "deployment_unhealthy",
		Summary:    "health gate failed",
		OccurredAt: r.now(),
	})

	sender := &fakeIncidentSender{}
	reported, err := r.HandleReportIncidents(context.Background(), nil, ReportIncidentPayload{
		ProjectID:      "prj_123",
		BindingID:      "bind_123",
		RevisionID:     "rev_123",
		RuntimeMode:    contracts.RuntimeModeStandalone,
		WorkspaceRoot:  filepath.Join(t.TempDir(), "runtime-root"),
		IncidentSender: sender,
	})
	if err != nil {
		t.Fatalf("handle report incidents: %v", err)
	}
	if reported != 1 {
		t.Fatalf("expected 1 reported, got %d", reported)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 incident sent, got %d", len(sender.sent))
	}
	if sender.sent[0].Severity != contracts.SeverityCritical {
		t.Fatalf("expected critical severity, got %q", sender.sent[0].Severity)
	}
}
