package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"lazyops-server/internal/models"
)

type ClusterRepository struct {
	db *gorm.DB
}

func NewClusterRepository(db *gorm.DB) *ClusterRepository {
	return &ClusterRepository{db: db}
}

func (r *ClusterRepository) Create(cluster *models.Cluster) error {
	return r.db.Create(cluster).Error
}

func (r *ClusterRepository) ListByUser(userID string) ([]models.Cluster, error) {
	var items []models.Cluster
	if err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Order("name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

func (r *ClusterRepository) GetByNameForUser(userID, name string) (*models.Cluster, error) {
	var cluster models.Cluster
	if err := r.db.Where("user_id = ? AND name = ?", userID, name).First(&cluster).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &cluster, nil
}

func (r *ClusterRepository) GetByIDForUser(userID, clusterID string) (*models.Cluster, error) {
	var cluster models.Cluster
	if err := r.db.Where("user_id = ? AND id = ?", userID, clusterID).First(&cluster).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &cluster, nil
}

func (r *ClusterRepository) GetByID(clusterID string) (*models.Cluster, error) {
	var cluster models.Cluster
	if err := r.db.Where("id = ?", clusterID).First(&cluster).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &cluster, nil
}

func (r *ClusterRepository) UpdateBootstrapMetadata(clusterID, kubeconfigSecretRef string, publicIP, instanceID *string, status string, at time.Time) error {
	updates := map[string]any{
		"updated_at": at,
	}
	if kubeconfigSecretRef != "" {
		updates["kubeconfig_secret_ref"] = kubeconfigSecretRef
	}
	if publicIP != nil {
		updates["public_ip"] = *publicIP
	}
	if instanceID != nil {
		updates["instance_id"] = *instanceID
	}
	if status != "" {
		updates["status"] = status
	}

	return r.db.Model(&models.Cluster{}).
		Where("id = ?", clusterID).
		Updates(updates).Error
}

func (r *ClusterRepository) UpdateManagedMetadata(clusterID, managedMetadataJSON string, at time.Time) error {
	return r.db.Model(&models.Cluster{}).
		Where("id = ?", clusterID).
		Updates(map[string]any{
			"managed_metadata_json": managedMetadataJSON,
			"updated_at":            at,
		}).Error
}

func (r *ClusterRepository) UpdateStatus(clusterID, status string, at time.Time) error {
	return r.db.Model(&models.Cluster{}).
		Where("id = ?", clusterID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": at,
		}).Error
}
