package service

import (
	"context"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeProactiveAlertStateStore struct{ state *models.ProactiveAlertState }

func (f *fakeProactiveAlertStateStore) GetByFingerprint(projectID, serviceName, fingerprint string) (*models.ProactiveAlertState, error) {
	if f.state == nil {
		return nil, nil
	}
	copy := *f.state
	return &copy, nil
}

func (f *fakeProactiveAlertStateStore) Upsert(state *models.ProactiveAlertState) error {
	copy := *state
	f.state = &copy
	return nil
}

type fakeIncidentStoreForProactive struct{ items []models.RuntimeIncident }

func (f *fakeIncidentStoreForProactive) Create(incident *models.RuntimeIncident) error {
	f.items = append(f.items, *incident)
	return nil
}
func (f *fakeIncidentStoreForProactive) ListByProject(projectID string) ([]models.RuntimeIncident, error) {
	return append([]models.RuntimeIncident{}, f.items...), nil
}
func (f *fakeIncidentStoreForProactive) ListByDeployment(projectID, deploymentID string) ([]models.RuntimeIncident, error) {
	return nil, nil
}
func (f *fakeIncidentStoreForProactive) UpdateStatus(incidentID, status string, at time.Time) error {
	return nil
}

type fakeOperatorBroadcasterForProactive struct{ count int }

func (f *fakeOperatorBroadcasterForProactive) BroadcastEvent(eventType string, payload any) error {
	f.count++
	return nil
}
func (f *fakeOperatorBroadcasterForProactive) BroadcastEventToUser(userID string, eventType string, payload any) error {
	f.count++
	return nil
}

func TestProactiveAlertServiceCreatesIncidentAndSuppressesRepeat(t *testing.T) {
	states := &fakeProactiveAlertStateStore{}
	incidents := &fakeIncidentStoreForProactive{}
	broadcaster := &fakeOperatorBroadcasterForProactive{}
	svc := NewProactiveAlertService(states, incidents, nil, nil, broadcaster)
	record := models.LogStreamEntry{ID: "log_1", ProjectID: "prj_123", BindingID: "bind_1", ServiceName: "api", Level: "error", Message: "connection refused", OccurredAt: time.Now().UTC()}
	alerts, err := svc.ProcessLogBatch(context.Background(), []models.LogStreamEntry{record})
	if err != nil {
		t.Fatalf("process log batch: %v", err)
	}
	if len(alerts) != 1 || alerts[0].Suppressed {
		t.Fatalf("expected unsuppressed alert, got %#v", alerts)
	}
	if len(incidents.items) != 1 {
		t.Fatalf("expected incident creation, got %d", len(incidents.items))
	}
	if broadcaster.count != 1 {
		t.Fatalf("expected one broadcast, got %d", broadcaster.count)
	}
	alerts, err = svc.ProcessLogBatch(context.Background(), []models.LogStreamEntry{record})
	if err != nil {
		t.Fatalf("process repeat: %v", err)
	}
	if len(alerts) != 1 || !alerts[0].Suppressed {
		t.Fatalf("expected suppressed repeat alert, got %#v", alerts)
	}
	if len(incidents.items) != 1 {
		t.Fatalf("expected no duplicate incident, got %d", len(incidents.items))
	}
}
