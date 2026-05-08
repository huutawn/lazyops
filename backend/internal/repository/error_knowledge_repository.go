package repository

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
)

type ErrorKnowledgeRepository struct {
	db *gorm.DB
}

func NewErrorKnowledgeRepository(db *gorm.DB) *ErrorKnowledgeRepository {
	return &ErrorKnowledgeRepository{db: db}
}

func (r *ErrorKnowledgeRepository) UpsertByFingerprint(item *models.ErrorKnowledgeDocument) error {
	if item == nil {
		return nil
	}
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	if item.FirstSeenAt.IsZero() {
		item.FirstSeenAt = now
	}
	if item.LastSeenAt.IsZero() {
		item.LastSeenAt = now
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}, {Name: "fingerprint"}},
		DoUpdates: clause.Assignments(map[string]any{
			"service_name":   item.ServiceName,
			"revision_id":    item.RevisionID,
			"correlation_id": item.CorrelationID,
			"severity":       item.Severity,
			"title":          item.Title,
			"body":           item.Body,
			"metadata_json":  item.MetadataJSON,
			"embedding":      item.Embedding,
			"last_seen_at":   item.LastSeenAt,
			"updated_at":     item.UpdatedAt,
		}),
	}).Create(item).Error
}

func (r *ErrorKnowledgeRepository) ListByProjectAndService(projectID, serviceName string, limit int) ([]models.ErrorKnowledgeDocument, error) {
	if limit <= 0 {
		limit = 20
	}
	tx := r.db.Where("project_id = ?", projectID)
	if strings.TrimSpace(serviceName) != "" {
		tx = tx.Where("service_name = ?", strings.TrimSpace(serviceName))
	}
	var items []models.ErrorKnowledgeDocument
	if err := tx.Order("last_seen_at DESC, id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ErrorKnowledgeRepository) ListSimilarByProject(projectID, serviceName, embedding string, limit int) ([]models.ErrorKnowledgeDocument, error) {
	if limit <= 0 {
		limit = 5
	}
	tx := r.db.Model(&models.ErrorKnowledgeDocument{}).Where("project_id = ?", projectID)
	if strings.TrimSpace(serviceName) != "" {
		tx = tx.Where("service_name = ?", strings.TrimSpace(serviceName))
	}
	var items []models.ErrorKnowledgeDocument
	query := tx.Order(clause.Expr{SQL: "embedding <-> ?", Vars: []any{embedding}}).Limit(limit)
	if err := query.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("query similar error knowledge: %w", err)
	}
	return items, nil
}
