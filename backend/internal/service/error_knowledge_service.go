package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"lazyops-server/internal/ai"
	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

type ErrorKnowledgeStore interface {
	UpsertByFingerprint(item *models.ErrorKnowledgeDocument) error
	ListByProjectAndService(projectID, serviceName string, limit int) ([]models.ErrorKnowledgeDocument, error)
	ListSimilarByProject(projectID, serviceName, embedding string, limit int) ([]models.ErrorKnowledgeDocument, error)
}

type ErrorKnowledgeService struct {
	store ErrorKnowledgeStore
	embeddings ai.EmbeddingClient
}

func NewErrorKnowledgeService(store ErrorKnowledgeStore, embeddings ai.EmbeddingClient) *ErrorKnowledgeService {
	return &ErrorKnowledgeService{store: store, embeddings: embeddings}
}

func (s *ErrorKnowledgeService) IndexLogEntries(entries []models.LogStreamEntry) error {
	if s == nil || s.store == nil || len(entries) == 0 {
		return nil
	}
	for _, entry := range entries {
		if !shouldIndexErrorKnowledge(entry) {
			continue
		}
		doc, err := s.buildErrorKnowledgeDocument(entry)
		if err != nil {
			return err
		}
		if err := s.store.UpsertByFingerprint(doc); err != nil {
			return err
		}
	}
	return nil
}

func (s *ErrorKnowledgeService) FindSimilar(projectID, serviceName, queryText string, limit int) ([]models.ErrorKnowledgeDocument, error) {
	if s == nil || s.store == nil || strings.TrimSpace(projectID) == "" || strings.TrimSpace(queryText) == "" {
		return nil, ErrInvalidInput
	}
	embedding, err := s.embed(queryText)
	if err != nil {
		return nil, err
	}
	return s.store.ListSimilarByProject(projectID, strings.TrimSpace(serviceName), embedding, limit)
}

func shouldIndexErrorKnowledge(entry models.LogStreamEntry) bool {
	level := strings.ToLower(strings.TrimSpace(entry.Level))
	message := strings.ToLower(strings.TrimSpace(entry.Message))
	if level == "error" || level == "critical" || level == "fatal" {
		return true
	}
	for _, token := range []string{"panic", "timeout", "refused", "crash", "exception", "failed", "oom"} {
		if strings.Contains(message, token) {
			return true
		}
	}
	return false
}

func (s *ErrorKnowledgeService) buildErrorKnowledgeDocument(entry models.LogStreamEntry) (*models.ErrorKnowledgeDocument, error) {
	metadata, _ := json.Marshal(map[string]any{
		"node":           entry.Node,
		"source":         entry.Source,
		"correlation_id": entry.CorrelationID,
		"occurred_at":    entry.OccurredAt,
	})
	normalizedBody := normalizeKnowledgeText(entry.Message)
	embedding, err := s.embed(normalizedBody)
	if err != nil {
		return nil, err
	}
	return &models.ErrorKnowledgeDocument{
		ID:            utils.NewPrefixedID("ekd"),
		ProjectID:     entry.ProjectID,
		ServiceName:   entry.ServiceName,
		RevisionID:    entry.RevisionID,
		CorrelationID: entry.CorrelationID,
		Fingerprint:   errorKnowledgeFingerprint(entry.ProjectID, entry.ServiceName, normalizedBody),
		Severity:      entry.Level,
		Title:         firstRuntimeNonEmpty(strings.TrimSpace(entry.ServiceName), "runtime") + " error knowledge",
		Body:          normalizedBody,
		MetadataJSON:  string(metadata),
		Embedding:     embedding,
		FirstSeenAt:   entry.OccurredAt,
		LastSeenAt:    entry.OccurredAt,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}, nil
}

func errorKnowledgeFingerprint(projectID, serviceName, body string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(projectID + "|" + serviceName + "|" + body))))
	return hex.EncodeToString(sum[:])
}

func normalizeKnowledgeText(text string) string {
	cleaned := strings.ToLower(strings.TrimSpace(text))
	replacements := []struct{ old, next string }{
		{"\n", " "},
		{"\t", " "},
		{"  ", " "},
	}
	for _, item := range replacements {
		cleaned = strings.ReplaceAll(cleaned, item.old, item.next)
	}
	return strings.TrimSpace(cleaned)
}

func (s *ErrorKnowledgeService) embed(text string) (string, error) {
	if s != nil && s.embeddings != nil {
		embedding, err := s.embeddings.EmbedText(text)
		if err == nil {
			return embedding, nil
		}
	}
	return ai.NewDeterministicEmbeddingClient().EmbedText(text)
}
