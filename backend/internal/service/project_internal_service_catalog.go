package service

import (
	"fmt"
	"sort"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

type internalServicePreset struct {
	Kind          string
	Alias         string
	Protocol      string
	Port          int
	LocalEndpoint string
}

var internalServicePresets = map[string]internalServicePreset{
	"postgres": {
		Kind:          "postgres",
		Alias:         "postgres",
		Protocol:      "tcp",
		Port:          5432,
		LocalEndpoint: "localhost:5432",
	},
	"mysql": {
		Kind:          "mysql",
		Alias:         "mysql",
		Protocol:      "tcp",
		Port:          3306,
		LocalEndpoint: "localhost:3306",
	},
	"redis": {
		Kind:          "redis",
		Alias:         "redis",
		Protocol:      "tcp",
		Port:          6379,
		LocalEndpoint: "localhost:6379",
	},
	"rabbitmq": {
		Kind:          "rabbitmq",
		Alias:         "rabbitmq",
		Protocol:      "tcp",
		Port:          5672,
		LocalEndpoint: "localhost:5672",
	},
}

func normalizeInternalServiceKinds(input []string) ([]string, error) {
	if len(input) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(input))
	items := make([]string, 0, len(input))
	for _, raw := range input {
		kind := strings.ToLower(strings.TrimSpace(raw))
		if kind == "" {
			continue
		}
		if _, ok := internalServicePresets[kind]; !ok {
			return nil, fmt.Errorf("%w: unsupported internal service kind %q", ErrInvalidInput, kind)
		}
		if _, exists := seen[kind]; exists {
			continue
		}
		seen[kind] = struct{}{}
		items = append(items, kind)
	}

	sort.Strings(items)
	return items, nil
}

func buildProjectInternalServiceModels(projectID string, kinds []string) ([]models.ProjectInternalService, error) {
	normalizedKinds, err := normalizeInternalServiceKinds(kinds)
	if err != nil {
		return nil, err
	}

	items := make([]models.ProjectInternalService, 0, len(normalizedKinds))
	for _, kind := range normalizedKinds {
		preset := internalServicePresets[kind]
		items = append(items, models.ProjectInternalService{
			ID:            utils.NewPrefixedID("insvc"),
			ProjectID:     projectID,
			Kind:          preset.Kind,
			Alias:         preset.Alias,
			Protocol:      preset.Protocol,
			Port:          preset.Port,
			LocalEndpoint: preset.LocalEndpoint,
		})
	}

	return items, nil
}

func internalServiceTargetServiceName(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return "lazyops-internal-service"
	}
	return "lazyops-internal-" + kind
}

func toProjectInternalServiceRecord(item models.ProjectInternalService) ProjectInternalServiceRecord {
	return ProjectInternalServiceRecord{
		ID:            item.ID,
		ProjectID:     item.ProjectID,
		Kind:          item.Kind,
		Alias:         item.Alias,
		Protocol:      item.Protocol,
		Port:          item.Port,
		LocalEndpoint: item.LocalEndpoint,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}
