package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
)

// RoutingPolicyStore defines the interface for routing policy persistence
type RoutingPolicyStore interface {
	GetByProjectID(projectID string) (*models.RoutingPolicy, error)
	Upsert(policy *models.RoutingPolicy) error
	DeleteByProjectID(projectID string) error
}

// ServiceStore defines the interface for service listing
type ServiceStore interface {
	ListByProject(projectID string) ([]models.Service, error)
}

type RoutingService struct {
	store      RoutingPolicyStore
	svcStore   ServiceStore
}

func NewRoutingService(store RoutingPolicyStore, svcStore ServiceStore) *RoutingService {
	return &RoutingService{
		store:    store,
		svcStore: svcStore,
	}
}

// GetRouting retrieves the routing configuration for a project
func (s *RoutingService) GetRouting(userID, role, projectID string) (*ProjectRoutingResult, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrInvalidInput
	}

	// Get routing policy from DB
	policy, err := s.store.GetByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load routing policy: %w", err)
	}

	if policy == nil {
		// No policy yet — return empty
		return &ProjectRoutingResult{
			RoutingPolicy:     RoutingPolicyRecord{Routes: []RoutingRouteRecord{}},
			AvailableServices: []string{},
		}, nil
	}

	routes, err := parseRoutes(policy.RoutesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse routes: %w", err)
	}

	routeRecords := make([]RoutingRouteRecord, 0, len(routes))
	for _, r := range routes {
		routeRecords = append(routeRecords, RoutingRouteRecord{
			Path:        r.Path,
			Service:     r.Service,
			Port:        r.Port,
			WebSocket:   r.WebSocket,
			StripPrefix: r.StripPrefix,
			CreatedAt:   policy.CreatedAt,
		})
	}

	// Get available services
	services, err := s.svcStore.ListByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	availableServices := make([]string, 0, len(services))
	for _, svc := range services {
		availableServices = append(availableServices, svc.Name)
	}

	return &ProjectRoutingResult{
		RoutingPolicy: RoutingPolicyRecord{
			SharedDomain: policy.SharedDomain,
			Routes:       routeRecords,
		},
		AvailableServices: availableServices,
	}, nil
}

// UpdateRouting updates the routing configuration for a project
func (s *RoutingService) UpdateRouting(cmd UpdateRoutingCommand) (*ProjectRoutingResult, error) {
	if !cmd.IsValid() {
		return nil, ErrInvalidInput
	}

	// Validate route service references
	services, err := s.svcStore.ListByProject(cmd.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list services for validation: %w", err)
	}

	serviceNames := make(map[string]bool)
	for _, svc := range services {
		serviceNames[svc.Name] = true
	}

	for i, route := range cmd.Routes {
		if strings.TrimSpace(route.Path) == "" {
			return nil, fmt.Errorf("route %d: path is required", i+1)
		}
		if strings.TrimSpace(route.Service) == "" {
			return nil, fmt.Errorf("route %d: service is required", i+1)
		}
		if !serviceNames[route.Service] {
			return nil, fmt.Errorf("route %d: service %q not found", i+1, route.Service)
		}
	}

	// Check for overlapping path prefixes
	for i, r1 := range cmd.Routes {
		for j, r2 := range cmd.Routes {
			if i != j && r1.Path != "/" && r2.Path != "/" {
				if strings.HasPrefix(r1.Path, r2.Path) || strings.HasPrefix(r2.Path, r1.Path) {
					return nil, fmt.Errorf("route %d path %q overlaps with route %d path %q", i+1, r1.Path, j+1, r2.Path)
				}
			}
		}
	}

	// Build model routes
	now := time.Now().UTC()
	modelRoutes := make([]models.RoutingRoute, 0, len(cmd.Routes))
	for _, r := range cmd.Routes {
		modelRoutes = append(modelRoutes, models.RoutingRoute{
			Path:        r.Path,
			Service:     r.Service,
			Port:        r.Port,
			WebSocket:   r.WebSocket,
			StripPrefix: r.StripPrefix,
		})
	}

	routesJSON, err := serializeRoutes(modelRoutes)
	if err != nil {
		return nil, err
	}

	policy := &models.RoutingPolicy{
		ProjectID:    cmd.ProjectID,
		SharedDomain: cmd.SharedDomain,
		RoutesJSON:   routesJSON,
	}

	if err := s.store.Upsert(policy); err != nil {
		return nil, fmt.Errorf("failed to save routing policy: %w", err)
	}

	availableServices := make([]string, 0, len(services))
	for _, svc := range services {
		availableServices = append(availableServices, svc.Name)
	}

	routeRecords := make([]RoutingRouteRecord, 0, len(modelRoutes))
	for _, r := range modelRoutes {
		routeRecords = append(routeRecords, RoutingRouteRecord{
			Path:        r.Path,
			Service:     r.Service,
			Port:        r.Port,
			WebSocket:   r.WebSocket,
			StripPrefix: r.StripPrefix,
			CreatedAt:   now,
		})
	}

	return &ProjectRoutingResult{
		RoutingPolicy: RoutingPolicyRecord{
			SharedDomain: cmd.SharedDomain,
			Routes:       routeRecords,
		},
		AvailableServices: availableServices,
	}, nil
}

// parseRoutes deserializes routes JSON
func parseRoutes(routesJSON string) ([]models.RoutingRoute, error) {
	var routes []models.RoutingRoute
	if routesJSON == "" {
		return routes, nil
	}
	if err := json.Unmarshal([]byte(routesJSON), &routes); err != nil {
		return nil, fmt.Errorf("parse routes JSON: %w", err)
	}
	return routes, nil
}

// serializeRoutes serializes routes to JSON
func serializeRoutes(routes []models.RoutingRoute) (string, error) {
	data, err := json.Marshal(routes)
	if err != nil {
		return "", fmt.Errorf("serialize routes: %w", err)
	}
	return string(data), nil
}
