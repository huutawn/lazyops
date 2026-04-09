package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
)

type DeploymentBindingRepository struct {
	db *gorm.DB
}

func NewDeploymentBindingRepository(db *gorm.DB) *DeploymentBindingRepository {
	return &DeploymentBindingRepository{db: db}
}

func (r *DeploymentBindingRepository) Create(binding *models.DeploymentBinding) error {
	return r.db.Create(binding).Error
}

func (r *DeploymentBindingRepository) UpsertAuto(binding *models.DeploymentBinding) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "project_id"},
			{Name: "target_ref"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"name":                      binding.Name,
			"runtime_mode":              binding.RuntimeMode,
			"target_kind":               binding.TargetKind,
			"target_id":                 binding.TargetID,
			"placement_policy_json":     binding.PlacementPolicyJSON,
			"domain_policy_json":        binding.DomainPolicyJSON,
			"compatibility_policy_json": binding.CompatibilityPolicyJSON,
			"scale_to_zero_policy_json": binding.ScaleToZeroPolicyJSON,
			"updated_at":                time.Now().UTC(),
		}),
	}).Create(binding).Error
}

func (r *DeploymentBindingRepository) ListByProject(projectID string) ([]models.DeploymentBinding, error) {
	var items []models.DeploymentBinding
	if err := r.db.Where("project_id = ?", projectID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *DeploymentBindingRepository) GetByTargetRefForProject(projectID, targetRef string) (*models.DeploymentBinding, error) {
	var binding models.DeploymentBinding
	if err := r.db.Where("project_id = ? AND target_ref = ?", projectID, targetRef).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &binding, nil
}

func (r *DeploymentBindingRepository) GetByIDForProject(projectID, bindingID string) (*models.DeploymentBinding, error) {
	var binding models.DeploymentBinding
	if err := r.db.Where("project_id = ? AND id = ?", projectID, bindingID).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &binding, nil
}
