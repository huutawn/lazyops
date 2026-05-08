package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/ai"
	"lazyops-server/internal/models"
)

type failingEmbeddingClient struct{}

func (f *failingEmbeddingClient) EmbedText(text string) (string, error) {
	return "", errors.New("provider unavailable")
}

type fakeErrorKnowledgeStore struct {
	items []models.ErrorKnowledgeDocument
}

func (f *fakeErrorKnowledgeStore) UpsertByFingerprint(item *models.ErrorKnowledgeDocument) error {
	for i := range f.items {
		if f.items[i].ProjectID == item.ProjectID && f.items[i].Fingerprint == item.Fingerprint {
			f.items[i] = *item
			return nil
		}
	}
	f.items = append(f.items, *item)
	return nil
}

func (f *fakeErrorKnowledgeStore) ListByProjectAndService(projectID, serviceName string, limit int) ([]models.ErrorKnowledgeDocument, error) {
	out := make([]models.ErrorKnowledgeDocument, 0)
	for _, item := range f.items {
		if item.ProjectID != projectID {
			continue
		}
		if serviceName != "" && item.ServiceName != serviceName {
			continue
		}
		out = append(out, item)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeErrorKnowledgeStore) ListSimilarByProject(projectID, serviceName, embedding string, limit int) ([]models.ErrorKnowledgeDocument, error) {
	return f.ListByProjectAndService(projectID, serviceName, limit)
}

func TestErrorKnowledgeServiceIndexesErrorLogs(t *testing.T) {
	store := &fakeErrorKnowledgeStore{}
	svc := NewErrorKnowledgeService(store, nil)
	err := svc.IndexLogEntries([]models.LogStreamEntry{
		{
			ID:          "log_1",
			ProjectID:   "prj_123",
			ServiceName: "api",
			RevisionID:  "rev_123",
			Level:       "error",
			Message:     "postgres timeout while acquiring connection",
			OccurredAt:  time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("index log entries: %v", err)
	}
	if len(store.items) != 1 {
		t.Fatalf("expected 1 knowledge document, got %d", len(store.items))
	}
}

func TestObservabilityServiceIndexesKnowledgeOnIngest(t *testing.T) {
	knowledgeStore := &fakeErrorKnowledgeStore{}
	knowledgeSvc := NewErrorKnowledgeService(knowledgeStore, nil)
	logStore := newFakeLogStreamStore()
	svc := newTestObservabilityService(
		newFakeTraceSummaryStore(),
		newFakeRuntimeIncidentStore(),
		logStore,
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	).WithErrorKnowledgeService(knowledgeSvc)

	_, err := svc.IngestLogBatch(context.Background(), IngestLogBatchCommand{
		ProjectID: "prj_123",
		BindingID: "bind_123",
		Entries: []LogBatchEntry{{
			Timestamp: time.Now().UTC(),
			Severity:  "critical",
			Source:    "api",
			Message:   "panic: nil pointer dereference",
		}},
	})
	if err != nil {
		t.Fatalf("ingest log batch: %v", err)
	}
	if len(knowledgeStore.items) == 0 {
		t.Fatal("expected error knowledge to be indexed")
	}
}

func TestErrorKnowledgeServiceFallsBackWhenEmbeddingProviderFails(t *testing.T) {
	store := &fakeErrorKnowledgeStore{}
	svc := NewErrorKnowledgeService(store, &failingEmbeddingClient{})
	err := svc.IndexLogEntries([]models.LogStreamEntry{{
		ID:          "log_1",
		ProjectID:   "prj_123",
		ServiceName: "api",
		Level:       "error",
		Message:     "panic: nil pointer dereference",
		OccurredAt:  time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("expected deterministic fallback, got %v", err)
	}
	if len(store.items) != 1 {
		t.Fatalf("expected indexed fallback knowledge document, got %d", len(store.items))
	}
	if store.items[0].Embedding == "" {
		t.Fatal("expected fallback embedding to be populated")
	}
	if store.items[0].Embedding == "[]" {
		t.Fatal("expected non-empty fallback embedding")
	}
	_ = ai.NewDeterministicEmbeddingClient()
}
