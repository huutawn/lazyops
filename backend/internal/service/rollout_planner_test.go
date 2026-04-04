package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/internal/runtime"
)

type fakeRuntimeIncidentStore struct {
	items     []models.RuntimeIncident
	createErr error
}

func newFakeRuntimeIncidentStore(items ...models.RuntimeIncident) *fakeRuntimeIncidentStore {
	return &fakeRuntimeIncidentStore{items: items}
}

func (f *fakeRuntimeIncidentStore) Create(incident *models.RuntimeIncident) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.items = append(f.items, *incident)
	return nil
}

func (f *fakeRuntimeIncidentStore) ListByProject(projectID string) ([]models.RuntimeIncident, error) {
	out := make([]models.RuntimeIncident, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeRuntimeIncidentStore) ListByDeployment(projectID, deploymentID string) ([]models.RuntimeIncident, error) {
	out := make([]models.RuntimeIncident, 0)
	for _, item := range f.items {
		if item.ProjectID == projectID && item.DeploymentID == deploymentID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeRuntimeIncidentStore) UpdateStatus(incidentID, status string, at time.Time) error {
	for i, item := range f.items {
		if item.ID == incidentID {
			f.items[i].Status = status
			f.items[i].ResolvedAt = &at
			f.items[i].UpdatedAt = at
			return nil
		}
	}
	return nil
}

type fakeOperatorEventBroadcaster struct {
	events []struct {
		eventType string
		payload   any
		userID    string
	}
}

func (f *fakeOperatorEventBroadcaster) BroadcastEvent(eventType string, payload any) error {
	f.events = append(f.events, struct {
		eventType string
		payload   any
		userID    string
	}{eventType: eventType, payload: payload})
	return nil
}

func (f *fakeOperatorEventBroadcaster) BroadcastEventToUser(userID string, eventType string, payload any) error {
	f.events = append(f.events, struct {
		eventType string
		payload   any
		userID    string
	}{eventType: eventType, payload: payload, userID: userID})
	return nil
}

func newTestRolloutPlanner(
	registry *runtime.Registry,
	revisionStore DesiredStateRevisionStore,
	deploymentStore DeploymentStore,
	incidentStore RuntimeIncidentStore,
	bindingStore DeploymentBindingStore,
	broadcaster OperatorEventBroadcaster,
) *RolloutPlanner {
	return NewRolloutPlanner(registry, revisionStore, deploymentStore, incidentStore, bindingStore, broadcaster)
}

func TestRolloutPlannerPlanCandidateSuccess(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	deploymentStore := newFakeDeploymentStore()
	incidentStore := newFakeRuntimeIncidentStore()
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: "standalone",
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	broadcaster := &fakeOperatorEventBroadcaster{}

	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, incidentStore, bindingStore, broadcaster)

	plan, err := planner.PlanCandidate(context.Background(), "prj_123", "rev_123")
	if err != nil {
		t.Fatalf("plan candidate: %v", err)
	}

	if plan.RuntimeMode != runtime.RuntimeModeStandalone {
		t.Fatalf("expected runtime mode standalone, got %q", plan.RuntimeMode)
	}
	if plan.TargetKind != "instance" {
		t.Fatalf("expected target kind instance, got %q", plan.TargetKind)
	}
	if plan.RevisionID != "rev_123" {
		t.Fatalf("expected revision id rev_123, got %q", plan.RevisionID)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected at least one rollout step")
	}
}

func TestRolloutPlannerPlanCandidateRejectsUnknownRevision(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	planner := newTestRolloutPlanner(
		registry,
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeRuntimeIncidentStore(),
		newFakeDeploymentBindingStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := planner.PlanCandidate(context.Background(), "prj_123", "rev_missing")
	if err == nil {
		t.Fatal("expected error for unknown revision")
	}
}

func TestRolloutPlannerHealthGateSuccess(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		newFakeDeploymentStore(),
		newFakeRuntimeIncidentStore(),
		newFakeDeploymentBindingStore(),
		&fakeOperatorEventBroadcaster{},
	)

	result, err := planner.ExecuteHealthGate(context.Background(), "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("health gate: %v", err)
	}

	if !result.Passed {
		t.Fatal("expected health gate to pass")
	}
	if len(result.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(result.Services))
	}
	for _, svc := range result.Services {
		if !svc.Healthy {
			t.Fatalf("expected service %q to be healthy", svc.ServiceName)
		}
	}
}

func TestRolloutPlannerPromoteCandidateSuccess(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusRunning,
	})

	broadcaster := &fakeOperatorEventBroadcaster{}

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		deploymentStore,
		newFakeRuntimeIncidentStore(),
		newFakeDeploymentBindingStore(),
		broadcaster,
	)

	result, err := planner.PromoteCandidate(context.Background(), "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("promote candidate: %v", err)
	}

	if result.RevisionID != "rev_123" {
		t.Fatalf("expected revision id rev_123, got %q", result.RevisionID)
	}
	if result.DeploymentID != "dep_123" {
		t.Fatalf("expected deployment id dep_123, got %q", result.DeploymentID)
	}

	if len(broadcaster.events) == 0 {
		t.Fatal("expected deployment.promoted event to be broadcast")
	}
	if broadcaster.events[0].eventType != runtime.EventDeploymentPromoted {
		t.Fatalf("expected event %q, got %q", runtime.EventDeploymentPromoted, broadcaster.events[0].eventType)
	}
}

func TestRolloutPlannerPromoteRejectsInvalidRevisionStatus(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusQueued,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		newFakeDeploymentStore(&models.Deployment{
			ID:         "dep_123",
			ProjectID:  "prj_123",
			RevisionID: "rev_123",
			Status:     DeploymentStatusRunning,
		}),
		newFakeRuntimeIncidentStore(),
		newFakeDeploymentBindingStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := planner.PromoteCandidate(context.Background(), "prj_123", "dep_123", "rev_123")
	if err == nil {
		t.Fatal("expected error for invalid revision status")
	}
}

func TestRolloutPlannerRollbackSuccess(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	promotedRevisionJSON := mustCompiledRevisionJSON(t, "rev_stable", "bp_123", "prj_123")

	revisionStore := newFakeDesiredStateRevisionStore(
		&models.DesiredStateRevision{
			ID:                   "rev_stable",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "stable123",
			TriggerKind:          "push",
			Status:               RevisionStatusPromoted,
			CompiledRevisionJSON: promotedRevisionJSON,
			CreatedAt:            time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC),
		},
		&models.DesiredStateRevision{
			ID:                   "rev_current",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "current456",
			TriggerKind:          "push",
			Status:               RevisionStatusApplying,
			CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_current", "bp_123", "prj_123"),
			CreatedAt:            time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC),
		},
	)

	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_current",
		Status:     DeploymentStatusRunning,
	})

	broadcaster := &fakeOperatorEventBroadcaster{}

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		deploymentStore,
		newFakeRuntimeIncidentStore(),
		newFakeDeploymentBindingStore(),
		broadcaster,
	)

	result, err := planner.RollbackDeployment(context.Background(), "prj_123", "dep_123")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if result.RolledBackTo != "rev_stable" {
		t.Fatalf("expected rollback to rev_stable, got %q", result.RolledBackTo)
	}
	if result.CommitSHA != "stable123" {
		t.Fatalf("expected commit stable123, got %q", result.CommitSHA)
	}

	if len(broadcaster.events) == 0 {
		t.Fatal("expected deployment.rolled_back event to be broadcast")
	}
	if broadcaster.events[0].eventType != runtime.EventDeploymentRolledBack {
		t.Fatalf("expected event %q, got %q", runtime.EventDeploymentRolledBack, broadcaster.events[0].eventType)
	}
}

func TestRolloutPlannerRollbackRejectsAlreadyRolledBack(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusRolledBack,
	})

	planner := newTestRolloutPlanner(
		registry,
		newFakeDesiredStateRevisionStore(),
		deploymentStore,
		newFakeRuntimeIncidentStore(),
		newFakeDeploymentBindingStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := planner.RollbackDeployment(context.Background(), "prj_123", "dep_123")
	if !errors.Is(err, ErrRollbackAlreadyRolledBack) {
		t.Fatalf("expected ErrRollbackAlreadyRolledBack, got %v", err)
	}
}

func TestRolloutPlannerRollbackCreatesIncidentWhenNoStableRevision(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(
		&models.DesiredStateRevision{
			ID:                   "rev_failing",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "failing123",
			TriggerKind:          "push",
			Status:               RevisionStatusFailed,
			CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_failing", "bp_123", "prj_123"),
		},
	)

	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_failing",
		Status:     DeploymentStatusRunning,
	})

	incidentStore := newFakeRuntimeIncidentStore()

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		deploymentStore,
		incidentStore,
		newFakeDeploymentBindingStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := planner.RollbackDeployment(context.Background(), "prj_123", "dep_123")
	if err == nil {
		t.Fatal("expected error when no stable revision found")
	}

	if len(incidentStore.items) == 0 {
		t.Fatal("expected incident to be created")
	}
	incident := incidentStore.items[0]
	if incident.Kind != IncidentKindRollbackFailure {
		t.Fatalf("expected incident kind rollback_failure, got %q", incident.Kind)
	}
	if incident.Severity != IncidentSeverityCritical {
		t.Fatalf("expected incident severity critical, got %q", incident.Severity)
	}
}

func TestRolloutPlannerRecordIncident(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	incidentStore := newFakeRuntimeIncidentStore()
	broadcaster := &fakeOperatorEventBroadcaster{}

	planner := newTestRolloutPlanner(
		registry,
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		incidentStore,
		newFakeDeploymentBindingStore(),
		broadcaster,
	)

	incident, err := planner.RecordIncident(
		"prj_123", "dep_123", "rev_123",
		IncidentKindUnhealthyCandidate, IncidentSeverityCritical,
		"candidate failed health check",
		map[string]any{"service": "api", "error": "connection refused"},
		"health_gate",
	)
	if err != nil {
		t.Fatalf("record incident: %v", err)
	}

	if incident.ID == "" || incident.ID[:4] != "inc_" {
		t.Fatalf("expected inc_ prefixed id, got %q", incident.ID)
	}
	if incident.Kind != IncidentKindUnhealthyCandidate {
		t.Fatalf("expected kind unhealthy_candidate, got %q", incident.Kind)
	}
	if incident.Severity != IncidentSeverityCritical {
		t.Fatalf("expected severity critical, got %q", incident.Severity)
	}
	if incident.Status != IncidentStatusOpen {
		t.Fatalf("expected status open, got %q", incident.Status)
	}

	if len(broadcaster.events) == 0 {
		t.Fatal("expected incident.created event to be broadcast")
	}
	if broadcaster.events[0].eventType != runtime.EventIncidentCreated {
		t.Fatalf("expected event %q, got %q", runtime.EventIncidentCreated, broadcaster.events[0].eventType)
	}
}

func TestRolloutPlannerPlanCandidateValidatesTarget(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_wrong",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_wrong",
		ProjectID:   "prj_123",
		Name:        "Bad Binding",
		TargetRef:   "bind_wrong",
		RuntimeMode: "standalone",
		TargetKind:  "cluster",
		TargetID:    "cls_123",
	})

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		newFakeDeploymentStore(),
		newFakeRuntimeIncidentStore(),
		bindingStore,
		&fakeOperatorEventBroadcaster{},
	)

	_, err := planner.PlanCandidate(context.Background(), "prj_123", "rev_123")
	if err == nil {
		t.Fatal("expected target validation error for mismatched target kind")
	}
}

func TestNormalizeIncidentSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", IncidentSeverityCritical},
		{"warning", IncidentSeverityWarning},
		{"info", IncidentSeverityInfo},
		{"unknown", IncidentSeverityWarning},
		{"", IncidentSeverityWarning},
	}

	for _, tt := range tests {
		got := normalizeIncidentSeverity(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeIncidentSeverity(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNormalizeIncidentStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"open", IncidentStatusOpen},
		{"resolved", IncidentStatusResolved},
		{"acknowledged", IncidentStatusAcknowledged},
		{"unknown", IncidentStatusOpen},
		{"", IncidentStatusOpen},
	}

	for _, tt := range tests {
		got := normalizeIncidentStatus(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeIncidentStatus(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestToIncidentRecord(t *testing.T) {
	detailsJSON, _ := json.Marshal(map[string]any{"key": "value"})
	incident := models.RuntimeIncident{
		ID:           "inc_123",
		ProjectID:    "prj_123",
		DeploymentID: "dep_123",
		RevisionID:   "rev_123",
		Kind:         IncidentKindUnhealthyCandidate,
		Severity:     IncidentSeverityCritical,
		Status:       IncidentStatusOpen,
		Summary:      "test incident",
		DetailsJSON:  string(detailsJSON),
		TriggeredBy:  "health_gate",
		CreatedAt:    time.Now().UTC(),
	}

	record := toIncidentRecord(incident)

	if record.ID != "inc_123" {
		t.Fatalf("expected id inc_123, got %q", record.ID)
	}
	if record.Kind != IncidentKindUnhealthyCandidate {
		t.Fatalf("expected kind unhealthy_candidate, got %q", record.Kind)
	}
	if record.Details["key"] != "value" {
		t.Fatalf("expected details key=value, got %v", record.Details)
	}
}
