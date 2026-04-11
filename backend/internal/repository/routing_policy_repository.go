package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

type RoutingPolicyRepository struct {
	db *gorm.DB
}

func NewRoutingPolicyRepository(db *gorm.DB) *RoutingPolicyRepository {
	return &RoutingPolicyRepository{db: db}
}

// GetByProjectID returns the routing policy for a project.
// Returns nil, nil if no policy exists yet.
func (r *RoutingPolicyRepository) GetByProjectID(projectID string) (*models.RoutingPolicy, error) {
	var policy models.RoutingPolicy
	err := r.db.Where("project_id = ?", projectID).First(&policy).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("query routing policy: %w", err)
	}
	return &policy, nil
}

// Upsert creates or updates the routing policy for a project.
func (r *RoutingPolicyRepository) Upsert(policy *models.RoutingPolicy) error {
	if policy.ID == "" {
		policy.ID = utils.NewPrefixedID("rp")
	}
	now := time.Now().UTC()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"shared_domain", "routes_json", "updated_at"}),
	}).Create(policy).Error
	if err != nil {
		return fmt.Errorf("upsert routing policy: %w", err)
	}
	return nil
}

// DeleteByProjectID removes the routing policy for a project.
func (r *RoutingPolicyRepository) DeleteByProjectID(projectID string) error {
	err := r.db.Where("project_id = ?", projectID).Delete(&models.RoutingPolicy{}).Error
	if err != nil {
		return fmt.Errorf("delete routing policy: %w", err)
	}
	return nil
}

// ParseRoutes deserializes the RoutesJSON field into a slice of RoutingRoute.
func ParseRoutes(routesJSON string) ([]models.RoutingRoute, error) {
	var routes []models.RoutingRoute
	if routesJSON == "" {
		return routes, nil
	}
	if err := json.Unmarshal([]byte(routesJSON), &routes); err != nil {
		return nil, fmt.Errorf("parse routes JSON: %w", err)
	}
	return routes, nil
}

// SerializeRoutes serializes a slice of RoutingRoute into JSON.
func SerializeRoutes(routes []models.RoutingRoute) (string, error) {
	data, err := json.Marshal(routes)
	if err != nil {
		return "", fmt.Errorf("serialize routes: %w", err)
	}
	return string(data), nil
}
