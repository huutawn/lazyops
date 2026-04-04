package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
)

type TopologyNodeRepository struct {
	db *gorm.DB
}

func NewTopologyNodeRepository(db *gorm.DB) *TopologyNodeRepository {
	return &TopologyNodeRepository{db: db}
}

func (r *TopologyNodeRepository) Upsert(node *models.TopologyNode) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}, {Name: "node_kind"}, {Name: "node_ref"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "status", "metadata_json", "updated_at"}),
	}).Create(node).Error
}

func (r *TopologyNodeRepository) ListByProject(projectID string) ([]models.TopologyNode, error) {
	var nodes []models.TopologyNode
	if err := r.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *TopologyNodeRepository) DeleteByProject(projectID string) error {
	return r.db.Where("project_id = ?", projectID).Delete(&models.TopologyNode{}).Error
}
