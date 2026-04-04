package repository

import (
	"lazyops-server/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TopologyStateRepository struct {
	db *gorm.DB
}

func NewTopologyStateRepository(db *gorm.DB) *TopologyStateRepository {
	return &TopologyStateRepository{db: db}
}

func (r *TopologyStateRepository) Upsert(state *models.TopologyState) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "instance_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"state", "metadata_json", "mesh_id", "last_seen_at", "updated_at"}),
	}).Create(state).Error
}

func (r *TopologyStateRepository) GetByInstance(instanceID string) (*models.TopologyState, error) {
	var state models.TopologyState
	if err := r.db.Where("instance_id = ?", instanceID).First(&state).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (r *TopologyStateRepository) ListByProject(projectID string) ([]models.TopologyState, error) {
	var states []models.TopologyState
	if err := r.db.Joins("JOIN instances ON instances.id = topology_states.instance_id").
		Where("instances.user_id = (SELECT user_id FROM projects WHERE id = ?)", projectID).
		Order("topology_states.last_seen_at DESC").
		Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}

func (r *TopologyStateRepository) ListActiveByMesh(meshID string) ([]models.TopologyState, error) {
	var states []models.TopologyState
	if err := r.db.Where("mesh_id = ? AND state != ?", meshID, "offline").
		Order("last_seen_at DESC").
		Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}
