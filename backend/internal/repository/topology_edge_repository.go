package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
)

type TopologyEdgeRepository struct {
	db *gorm.DB
}

func NewTopologyEdgeRepository(db *gorm.DB) *TopologyEdgeRepository {
	return &TopologyEdgeRepository{db: db}
}

func (r *TopologyEdgeRepository) Upsert(edge *models.TopologyEdge) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "source_id"}, {Name: "target_id"}, {Name: "edge_kind"}},
		DoUpdates: clause.AssignmentColumns([]string{"protocol", "metadata_json"}),
	}).Create(edge).Error
}

func (r *TopologyEdgeRepository) ListByProject(projectID string) ([]models.TopologyEdge, error) {
	var edges []models.TopologyEdge
	if err := r.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&edges).Error; err != nil {
		return nil, err
	}
	return edges, nil
}

func (r *TopologyEdgeRepository) DeleteByProject(projectID string) error {
	return r.db.Where("project_id = ?", projectID).Delete(&models.TopologyEdge{}).Error
}
