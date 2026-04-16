package repository

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"

	"gorm.io/gorm"
)

const managedInternalServicePathPrefix = ".lazyops/internal/"

type ManagedInternalServiceRepository struct {
	db     *gorm.DB
	legacy *ProjectInternalServiceRepository
}

func NewManagedInternalServiceRepository(db *gorm.DB, legacy *ProjectInternalServiceRepository) *ManagedInternalServiceRepository {
	return &ManagedInternalServiceRepository{
		db:     db,
		legacy: legacy,
	}
}

func (r *ManagedInternalServiceRepository) ReplaceForProject(projectID string, items []models.ProjectInternalService) error {
	if r == nil || r.db == nil {
		return nil
	}

	managed := make([]models.Service, 0, len(items))
	legacyItems := make([]models.ProjectInternalService, 0, len(items))
	for _, item := range items {
		cloned := item
		if strings.TrimSpace(cloned.ID) == "" {
			cloned.ID = utils.NewPrefixedID("insvc")
		}
		cloned.ProjectID = projectID
		legacyItems = append(legacyItems, cloned)
		managed = append(managed, managedInternalServiceToService(cloned))
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ? AND path LIKE ?", projectID, managedInternalServicePathPrefix+"%").Delete(&models.Service{}).Error; err != nil {
			return err
		}
		if len(managed) > 0 {
			if err := tx.Create(&managed).Error; err != nil {
				return err
			}
		}

		if tx.Migrator().HasTable(&models.ProjectInternalService{}) {
			if err := tx.Where("project_id = ?", projectID).Delete(&models.ProjectInternalService{}).Error; err != nil {
				return err
			}
			if len(legacyItems) > 0 {
				if err := tx.Create(&legacyItems).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *ManagedInternalServiceRepository) ListByProject(projectID string) ([]models.ProjectInternalService, error) {
	if r == nil || r.db == nil {
		return []models.ProjectInternalService{}, nil
	}

	var services []models.Service
	if err := r.db.
		Where("project_id = ? AND path LIKE ?", projectID, managedInternalServicePathPrefix+"%").
		Order("name ASC").
		Find(&services).Error; err != nil {
		return nil, err
	}
	if len(services) == 0 && r.legacy != nil {
		return r.legacy.ListByProject(projectID)
	}

	items := make([]models.ProjectInternalService, 0, len(services))
	for _, svc := range services {
		items = append(items, managedServiceToInternalService(svc))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			return items[i].Alias < items[j].Alias
		}
		return items[i].Kind < items[j].Kind
	})
	return items, nil
}

func managedInternalServiceToService(item models.ProjectInternalService) models.Service {
	kind := normalizeManagedInternalServiceKind(item.Kind)
	port := item.Port
	if port <= 0 {
		port = defaultManagedInternalServicePort(kind)
	}
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	if protocol == "" {
		protocol = "tcp"
	}

	pvcSpecJSON := "{}"
	if spec, ok := defaultManagedInternalPVCSpec(kind); ok {
		pvcSpecJSON = marshalJSON(spec, "{}")
	}
	runtimeProfile := defaultManagedInternalRuntimeProfile(kind)

	service := models.Service{
		ID:                 utils.NewPrefixedID("svc"),
		ProjectID:          item.ProjectID,
		Name:               managedInternalServiceName(kind),
		Path:               managedInternalServicePathPrefix + kind,
		Kind:               kind,
		Public:             false,
		StartHint:          "managed-internal-service",
		ImageRef:           defaultManagedInternalImage(kind),
		ImageDigest:        "",
		DetectedPortsJSON:  "[]",
		TargetPort:         port,
		ServicePort:        port,
		Replicas:           1,
		EnvBundleJSON:      "{}",
		PVCSpecJSON:        pvcSpecJSON,
		DeployStrategyJSON: "{}",
		HealthcheckJSON: marshalJSON(map[string]any{
			"protocol": protocol,
			"port":     port,
		}, "{}"),
	}
	if runtimeProfile != "" {
		copy := runtimeProfile
		service.RuntimeProfile = &copy
	}
	return service
}

func managedServiceToInternalService(item models.Service) models.ProjectInternalService {
	kind := normalizeManagedInternalServiceKind(firstNonEmptyManagedString(
		item.Kind,
		strings.TrimPrefix(strings.TrimSpace(item.Path), managedInternalServicePathPrefix),
		strings.TrimPrefix(strings.TrimSpace(item.Name), "lazyops-internal-"),
	))
	port := firstPositiveManagedInt(item.ServicePort, item.TargetPort, healthcheckPort(item.HealthcheckJSON), defaultManagedInternalServicePort(kind))
	protocol := strings.ToLower(strings.TrimSpace(healthcheckProtocol(item.HealthcheckJSON)))
	if protocol == "" {
		protocol = "tcp"
	}
	return models.ProjectInternalService{
		ID:            item.ID,
		ProjectID:     item.ProjectID,
		Kind:          kind,
		Alias:         kind,
		Protocol:      protocol,
		Port:          port,
		LocalEndpoint: fmt.Sprintf("localhost:%d", port),
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func managedInternalServiceName(kind string) string {
	kind = normalizeManagedInternalServiceKind(kind)
	if kind == "" {
		return "lazyops-internal-service"
	}
	return "lazyops-internal-" + kind
}

func normalizeManagedInternalServiceKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "postgres", "mysql", "redis", "rabbitmq":
		return kind
	default:
		return kind
	}
}

func defaultManagedInternalServicePort(kind string) int {
	switch normalizeManagedInternalServiceKind(kind) {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "redis":
		return 6379
	case "rabbitmq":
		return 5672
	default:
		return 0
	}
}

func defaultManagedInternalRuntimeProfile(kind string) string {
	switch normalizeManagedInternalServiceKind(kind) {
	case "postgres", "mysql", "redis", "rabbitmq":
		return "internal-db"
	default:
		return "service"
	}
}

func defaultManagedInternalImage(kind string) string {
	switch normalizeManagedInternalServiceKind(kind) {
	case "postgres":
		return "postgres:16-alpine"
	case "mysql":
		return "mysql:8.4"
	case "redis":
		return "redis:7-alpine"
	case "rabbitmq":
		return "rabbitmq:3.13-alpine"
	default:
		return ""
	}
}

func defaultManagedInternalPVCSpec(kind string) (map[string]any, bool) {
	switch normalizeManagedInternalServiceKind(kind) {
	case "postgres", "mysql", "redis", "rabbitmq":
		return map[string]any{"size": "5Gi"}, true
	default:
		return nil, false
	}
}

func marshalJSON(value any, fallback string) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fallback
	}
	return string(raw)
}

func healthcheckPort(raw string) int {
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return 0
	}
	switch value := payload["port"].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		port, _ := strconv.Atoi(strings.TrimSpace(value))
		return port
	default:
		return 0
	}
}

func healthcheckProtocol(raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return ""
	}
	value, _ := payload["protocol"].(string)
	return strings.TrimSpace(value)
}

func firstNonEmptyManagedString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositiveManagedInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
