package service

import (
	"encoding/json"
	"fmt"
	"sort"
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

type ProjectDomainReader interface {
	GetPrimaryManagedByProjectID(projectID string) (*ProjectDomainRecord, error)
}

// ServiceStore defines the interface for service listing
type ServiceStore interface {
	ListByProject(projectID string) ([]models.Service, error)
}

type RoutingService struct {
	store    RoutingPolicyStore
	svcStore ServiceStore
	domains  ProjectDomainReader
}

func NewRoutingService(store RoutingPolicyStore, svcStore ServiceStore) *RoutingService {
	return &RoutingService{
		store:    store,
		svcStore: svcStore,
	}
}

func (s *RoutingService) WithProjectDomains(domains ProjectDomainReader) *RoutingService {
	if s == nil {
		return s
	}
	s.domains = domains
	return s
}

// GetRouting retrieves the routing configuration for a project
func (s *RoutingService) GetRouting(userID, role, projectID string) (*ProjectRoutingResult, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrInvalidInput
	}

	services, err := s.svcStore.ListByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	availableServices := make([]string, 0, len(services))
	for _, svc := range services {
		availableServices = append(availableServices, svc.Name)
	}
	sort.Strings(availableServices)
	descriptors := routingDescriptorsFromModels(services)

	policy, err := s.store.GetByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load routing policy: %w", err)
	}
	sharedDomain := s.resolveManagedSharedDomain(projectID)
	suggested := buildDefaultRoutingPolicy(sharedDomain, descriptors)

	if policy == nil {
		effectivePaths := buildRoutingGuidanceRoutes(suggested, descriptors, "convention")
		return &ProjectRoutingResult{
			RoutingPolicy:        suggested,
			AvailableServices:    availableServices,
			SuggestedRoutes:      buildRoutingGuidanceRoutes(suggested, descriptors, "suggested"),
			EffectivePublicPaths: effectivePaths,
			Warnings:             buildRoutingWarnings(suggested, suggested, descriptors),
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

	resolvedSharedDomain := firstRuntimeNonEmpty(policy.SharedDomain, sharedDomain)
	if len(routeRecords) == 0 {
		effective := buildDefaultRoutingPolicy(resolvedSharedDomain, descriptors)
		return &ProjectRoutingResult{
			RoutingPolicy:        effective,
			AvailableServices:    availableServices,
			SuggestedRoutes:      buildRoutingGuidanceRoutes(buildDefaultRoutingPolicy(resolvedSharedDomain, descriptors), descriptors, "suggested"),
			EffectivePublicPaths: buildRoutingGuidanceRoutes(effective, descriptors, "convention"),
			Warnings:             buildRoutingWarnings(buildDefaultRoutingPolicy(resolvedSharedDomain, descriptors), effective, descriptors),
		}, nil
	}
	effective := RoutingPolicyRecord{
		SharedDomain: resolvedSharedDomain,
		Routes:       routeRecords,
	}
	return &ProjectRoutingResult{
		RoutingPolicy:        effective,
		AvailableServices:    availableServices,
		SuggestedRoutes:      buildRoutingGuidanceRoutes(buildDefaultRoutingPolicy(resolvedSharedDomain, descriptors), descriptors, "suggested"),
		EffectivePublicPaths: buildRoutingGuidanceRoutes(effective, descriptors, "configured"),
		Warnings:             buildRoutingWarnings(buildDefaultRoutingPolicy(resolvedSharedDomain, descriptors), effective, descriptors),
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
	exactPaths := make(map[string]int)
	rootCount := 0
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
		normalizedPath := normalizedPublicPath(route.Path)
		exactPaths[normalizedPath]++
		if exactPaths[normalizedPath] > 1 {
			return nil, fmt.Errorf("route %d: path %q is already assigned to another service", i+1, normalizedPath)
		}
		if normalizedPath == "/" {
			rootCount++
		}
	}
	if rootCount > 1 {
		return nil, fmt.Errorf("only one service can own the root path \"/\"; move backend traffic to /api or use a separate domain")
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
	descriptors := routingDescriptorsFromModels(services)
	for _, svc := range services {
		availableServices = append(availableServices, svc.Name)
	}
	sort.Strings(availableServices)

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
	effective := RoutingPolicyRecord{
		SharedDomain: cmd.SharedDomain,
		Routes:       routeRecords,
	}
	suggested := buildDefaultRoutingPolicy(cmd.SharedDomain, descriptors)

	return &ProjectRoutingResult{
		RoutingPolicy:        effective,
		AvailableServices:    availableServices,
		SuggestedRoutes:      buildRoutingGuidanceRoutes(suggested, descriptors, "suggested"),
		EffectivePublicPaths: buildRoutingGuidanceRoutes(effective, descriptors, "configured"),
		Warnings:             buildRoutingWarnings(suggested, effective, descriptors),
	}, nil
}

func (s *RoutingService) ResolveEffectiveRouting(projectID string, services []BlueprintServiceContractRecord, sharedDomainHint string) (RoutingPolicyRecord, error) {
	if strings.TrimSpace(projectID) == "" {
		return RoutingPolicyRecord{}, ErrInvalidInput
	}

	policy, err := s.store.GetByProjectID(projectID)
	if err != nil {
		return RoutingPolicyRecord{}, fmt.Errorf("failed to load routing policy: %w", err)
	}

	sharedDomain := firstRuntimeNonEmpty(sharedDomainHint, s.resolveManagedSharedDomain(projectID))
	descriptors := routingDescriptorsFromContracts(services)
	if policy == nil {
		return buildDefaultRoutingPolicy(sharedDomain, descriptors), nil
	}

	routes, err := parseRoutes(policy.RoutesJSON)
	if err != nil {
		return RoutingPolicyRecord{}, fmt.Errorf("failed to parse routes: %w", err)
	}
	if len(routes) == 0 {
		return buildDefaultRoutingPolicy(firstRuntimeNonEmpty(policy.SharedDomain, sharedDomain), descriptors), nil
	}

	records := make([]RoutingRouteRecord, 0, len(routes))
	for _, route := range routes {
		records = append(records, RoutingRouteRecord{
			Path:        route.Path,
			Service:     route.Service,
			Port:        route.Port,
			WebSocket:   route.WebSocket,
			StripPrefix: route.StripPrefix,
		})
	}
	return RoutingPolicyRecord{
		SharedDomain: firstRuntimeNonEmpty(policy.SharedDomain, sharedDomain),
		Routes:       records,
	}, nil
}

func (s *RoutingService) resolveManagedSharedDomain(projectID string) string {
	if s == nil || s.domains == nil {
		return ""
	}
	record, err := s.domains.GetPrimaryManagedByProjectID(projectID)
	if err != nil || record == nil {
		return ""
	}
	return strings.TrimSpace(record.Hostname)
}

type routingDescriptor struct {
	Name           string
	Kind           string
	RuntimeProfile string
	Public         bool
}

func routingDescriptorsFromModels(items []models.Service) []routingDescriptor {
	out := make([]routingDescriptor, 0, len(items))
	for _, item := range items {
		runtimeProfile := ""
		if item.RuntimeProfile != nil {
			runtimeProfile = strings.TrimSpace(*item.RuntimeProfile)
		}
		out = append(out, routingDescriptor{
			Name:           item.Name,
			Kind:           item.Kind,
			RuntimeProfile: runtimeProfile,
			Public:         item.Public,
		})
	}
	return out
}

func routingDescriptorsFromContracts(items []BlueprintServiceContractRecord) []routingDescriptor {
	out := make([]routingDescriptor, 0, len(items))
	for _, item := range items {
		out = append(out, routingDescriptor{
			Name:           item.Name,
			Kind:           item.Kind,
			RuntimeProfile: item.RuntimeProfile,
			Public:         item.Public,
		})
	}
	return out
}

func buildDefaultRoutingPolicy(sharedDomain string, services []routingDescriptor) RoutingPolicyRecord {
	publicServices := make([]routingDescriptor, 0, len(services))
	for _, svc := range services {
		if svc.Public {
			publicServices = append(publicServices, svc)
		}
	}
	if len(publicServices) == 0 {
		return RoutingPolicyRecord{
			SharedDomain: strings.TrimSpace(sharedDomain),
			Routes:       []RoutingRouteRecord{},
		}
	}
	if len(publicServices) == 1 {
		return RoutingPolicyRecord{
			SharedDomain: strings.TrimSpace(sharedDomain),
			Routes: []RoutingRouteRecord{
				{Path: "/", Service: publicServices[0].Name},
			},
		}
	}

	websocketSvc := firstMatchingService(publicServices, isWebSocketDescriptor)
	apiSvc := firstMatchingService(publicServices, isAPIDescriptor)
	frontendSvc := firstMatchingService(publicServices, isFrontendDescriptor)

	if apiSvc.Name == "" {
		apiSvc = firstMatchingService(publicServices, isBackendDescriptor)
	}
	rootSvc := frontendSvc
	if rootSvc.Name == "" {
		rootSvc = firstNonMatchingPublicService(publicServices, websocketSvc.Name, apiSvc.Name)
	}
	if rootSvc.Name == "" {
		rootSvc = publicServices[0]
	}

	routes := make([]RoutingRouteRecord, 0, len(publicServices))
	seenPaths := make(map[string]struct{}, len(publicServices)+2)
	seenServices := make(map[string]struct{}, len(publicServices))

	if websocketSvc.Name != "" {
		route := RoutingRouteRecord{
			Path:      "/ws",
			Service:   websocketSvc.Name,
			WebSocket: true,
		}
		routes = append(routes, route)
		seenPaths[route.Path] = struct{}{}
		seenServices[route.Service] = struct{}{}
	}
	if apiSvc.Name != "" && apiSvc.Name != rootSvc.Name {
		route := RoutingRouteRecord{
			Path:    "/api",
			Service: apiSvc.Name,
		}
		routes = append(routes, route)
		seenPaths[route.Path] = struct{}{}
		seenServices[route.Service] = struct{}{}
	}
	rootRoute := RoutingRouteRecord{
		Path:    "/",
		Service: rootSvc.Name,
	}
	routes = append(routes, rootRoute)
	seenPaths[rootRoute.Path] = struct{}{}
	seenServices[rootRoute.Service] = struct{}{}

	for _, svc := range publicServices {
		if _, exists := seenServices[svc.Name]; exists {
			continue
		}
		path := "/" + sanitizeDomainLabel(svc.Name)
		if path == "/" {
			path = "/" + sanitizeDomainLabel(svc.Kind)
		}
		if path == "/" {
			path = "/service"
		}
		for pathExists(path, seenPaths) {
			path = path + "-alt"
		}
		route := RoutingRouteRecord{
			Path:    path,
			Service: svc.Name,
		}
		if isWebSocketDescriptor(svc) {
			route.WebSocket = true
		}
		routes = append(routes, route)
		seenPaths[path] = struct{}{}
		seenServices[svc.Name] = struct{}{}
	}

	return RoutingPolicyRecord{
		SharedDomain: strings.TrimSpace(sharedDomain),
		Routes:       routes,
	}
}

func buildRoutingGuidanceRoutes(policy RoutingPolicyRecord, services []routingDescriptor, source string) []RoutingGuidanceRouteRecord {
	serviceIndex := make(map[string]routingDescriptor, len(services))
	for _, item := range services {
		serviceIndex[item.Name] = item
	}
	out := make([]RoutingGuidanceRouteRecord, 0, len(policy.Routes))
	for _, route := range policy.Routes {
		descriptor := serviceIndex[route.Service]
		audience := "frontend-browser"
		if route.WebSocket {
			audience = "websocket"
		} else if route.Path == "/" && isFrontendDescriptor(descriptor) {
			audience = "frontend-root"
		} else if isAPIDescriptor(descriptor) || isBackendDescriptor(descriptor) {
			audience = "browser-api"
		}
		out = append(out, RoutingGuidanceRouteRecord{
			Path:      route.Path,
			Service:   route.Service,
			Audience:  audience,
			Source:    source,
			WebSocket: route.WebSocket,
		})
	}
	return out
}

func buildRoutingWarnings(suggested, effective RoutingPolicyRecord, services []routingDescriptor) []string {
	if len(effective.Routes) == 0 {
		return nil
	}
	serviceIndex := make(map[string]routingDescriptor, len(services))
	for _, item := range services {
		serviceIndex[item.Name] = item
	}
	suggestedByService := make(map[string]RoutingRouteRecord, len(suggested.Routes))
	for _, route := range suggested.Routes {
		key := route.Service + fmt.Sprintf("|%t", route.WebSocket)
		suggestedByService[key] = route
	}
	warnings := make([]string, 0)
	rootRoute := ""
	for _, route := range effective.Routes {
		if normalizedPublicPath(route.Path) == "/" {
			rootRoute = route.Service
		}
		key := route.Service + fmt.Sprintf("|%t", route.WebSocket)
		suggestedRoute, ok := suggestedByService[key]
		if !ok {
			continue
		}
		if normalizedPublicPath(suggestedRoute.Path) != normalizedPublicPath(route.Path) {
			descriptor := serviceIndex[route.Service]
			switch {
			case route.WebSocket:
				warnings = append(warnings, fmt.Sprintf("WebSocket service %q now uses %q instead of the default /ws. Update browser clients and snippets to match.", route.Service, normalizedPublicPath(route.Path)))
			case isAPIDescriptor(descriptor) || isBackendDescriptor(descriptor):
				warnings = append(warnings, fmt.Sprintf("API service %q now uses %q instead of the default /api. Browser and client examples must use the effective path.", route.Service, normalizedPublicPath(route.Path)))
			case isFrontendDescriptor(descriptor):
				warnings = append(warnings, fmt.Sprintf("Frontend service %q now uses %q instead of the default /. Make sure the primary app URL and docs follow the custom path.", route.Service, normalizedPublicPath(route.Path)))
			default:
				warnings = append(warnings, fmt.Sprintf("Public service %q uses custom path %q. Update any client configuration to match the effective route.", route.Service, normalizedPublicPath(route.Path)))
			}
		}
	}
	frontendRoot := ""
	for _, descriptor := range services {
		if descriptor.Public && isFrontendDescriptor(descriptor) {
			frontendRoot = descriptor.Name
			break
		}
	}
	if frontendRoot != "" && rootRoute != "" && rootRoute != frontendRoot {
		warnings = append(warnings, fmt.Sprintf("Root path / currently points to %q, not the detected frontend service %q. This is valid custom mode, but browser entrypoints and docs should use the effective route.", rootRoute, frontendRoot))
	}
	return warnings
}

func pathExists(path string, seen map[string]struct{}) bool {
	_, exists := seen[path]
	return exists
}

func firstMatchingService(items []routingDescriptor, match func(routingDescriptor) bool) routingDescriptor {
	for _, item := range items {
		if match(item) {
			return item
		}
	}
	return routingDescriptor{}
}

func firstNonMatchingPublicService(items []routingDescriptor, disallowed ...string) routingDescriptor {
	blocked := make(map[string]struct{}, len(disallowed))
	for _, item := range disallowed {
		item = strings.TrimSpace(item)
		if item != "" {
			blocked[item] = struct{}{}
		}
	}
	for _, item := range items {
		if _, exists := blocked[item.Name]; exists {
			continue
		}
		return item
	}
	return routingDescriptor{}
}

func isFrontendDescriptor(item routingDescriptor) bool {
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	lowerKind := strings.ToLower(strings.TrimSpace(item.Kind))
	lowerProfile := strings.ToLower(strings.TrimSpace(item.RuntimeProfile))
	if lowerKind != "" && lowerKind != "app" {
		return lowerKind == "web" || lowerKind == "frontend"
	}
	return lowerProfile == "web" ||
		strings.Contains(lowerName, "front") ||
		strings.Contains(lowerName, "fe") ||
		strings.Contains(lowerName, "web") ||
		strings.Contains(lowerName, "ui")
}

func isBackendDescriptor(item routingDescriptor) bool {
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	lowerKind := strings.ToLower(strings.TrimSpace(item.Kind))
	if lowerKind != "" && lowerKind != "app" {
		return lowerKind == "backend" || lowerKind == "server" || lowerKind == "api"
	}
	return strings.Contains(lowerName, "backend") ||
		strings.Contains(lowerName, "server") ||
		lowerName == "be"
}

func isAPIDescriptor(item routingDescriptor) bool {
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	lowerKind := strings.ToLower(strings.TrimSpace(item.Kind))
	if lowerKind != "" && lowerKind != "app" {
		return lowerKind == "api" || lowerKind == "backend" || lowerKind == "server"
	}
	return strings.Contains(lowerName, "api") ||
		strings.Contains(lowerName, "backend") ||
		lowerName == "be"
}

func isWebSocketDescriptor(item routingDescriptor) bool {
	lowerName := strings.ToLower(strings.TrimSpace(item.Name))
	return strings.Contains(lowerName, "ws") ||
		strings.Contains(lowerName, "socket") ||
		strings.Contains(lowerName, "realtime")
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
