package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sort"
	"strings"
	"unicode"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var ErrProjectSlugExists = errors.New("project slug already exists")

const reservedManagedInternalServicePathPrefix = ".lazyops/internal/"

const (
	serviceSourceTypeRepo     = "repo"
	serviceSourceTypeInternal = "internal"

	servicePlacementModeSharedCluster = "shared_cluster"
	servicePlacementModePinnedNode    = "pinned_node"

	lazyopsServiceMetaSourceType              = "_lazyops_source_type"
	lazyopsServiceMetaPlacementMode           = "_lazyops_placement_mode"
	lazyopsServiceMetaPlacementNodeID         = "_lazyops_placement_node_id"
	lazyopsServiceMetaDependencies            = "_lazyops_dependencies"
	lazyopsServiceMetaConnectionTemplateKey   = "_lazyops_connection_template_key"
	lazyopsServiceMetaConnectionTemplate      = "_lazyops_connection_template"
	lazyopsServiceMetaConnectionTargetService = "_lazyops_connection_target_service"
	lazyopsServiceMetaManagedByLazyops        = "_lazyops_managed_by_lazyops"
)

type ProjectService struct {
	projects         ProjectStore
	internalServices ProjectInternalServiceStore
	services         ProjectServiceStore
}

func NewProjectService(projects ProjectStore, internalServices ...ProjectInternalServiceStore) *ProjectService {
	svc := &ProjectService{projects: projects}
	if len(internalServices) > 0 {
		svc.internalServices = internalServices[0]
	}
	return svc
}

func (s *ProjectService) WithServiceStore(services ProjectServiceStore) *ProjectService {
	if s == nil {
		return s
	}
	s.services = services
	return s
}

func (s *ProjectService) Create(cmd CreateProjectCommand) (*ProjectSummary, error) {
	userID := strings.TrimSpace(cmd.UserID)
	name := utils.NormalizeSpace(cmd.Name)
	if userID == "" || name == "" {
		return nil, ErrInvalidInput
	}

	slugSource := cmd.Slug
	if strings.TrimSpace(slugSource) == "" {
		slugSource = name
	}
	slug := normalizeProjectSlug(slugSource)
	if slug == "" {
		return nil, ErrInvalidInput
	}
	namespaceSlug := normalizeNamespaceSlug(cmd.NamespaceSlug, slug)
	if namespaceSlug == "" {
		return nil, ErrInvalidInput
	}

	defaultBranch, err := normalizeDefaultBranch(cmd.DefaultBranch)
	if err != nil {
		return nil, err
	}
	runtimeMode, err := normalizeBindingRuntimeMode(cmd.RuntimeMode)
	if err != nil {
		if strings.TrimSpace(cmd.RuntimeMode) == "" {
			runtimeMode = "distributed-k3s"
		} else {
			return nil, err
		}
	}
	if runtimeMode == "distributed-k3s" {
		if err := validateK3sResourceName("project.namespace_slug", namespaceSlug); err != nil {
			return nil, err
		}
	}
	internalServiceKinds := []string{}
	if len(cmd.InternalServices) > 0 {
		internalServiceKinds, err = normalizeInternalServiceKinds(cmd.InternalServices)
		if err != nil {
			return nil, err
		}
	}
	initialServices := buildInitialProjectServiceItems(cmd.Services, internalServiceKinds)
	if len(initialServices) > 0 && s.services == nil {
		return nil, ErrInvalidInput
	}

	existing, err := s.projects.GetBySlugForUser(userID, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrProjectSlugExists
	}

	project := &models.Project{
		ID:            utils.NewPrefixedID("prj"),
		UserID:        userID,
		Name:          name,
		Slug:          slug,
		NamespaceSlug: namespaceSlug,
		RuntimeMode:   runtimeMode,
		DefaultBranch: defaultBranch,
	}
	if clusterID := strings.TrimSpace(cmd.ClusterID); clusterID != "" {
		project.ClusterID = &clusterID
	}
	if err := s.projects.Create(project); err != nil {
		return nil, err
	}
	if len(initialServices) > 0 {
		items, err := buildConfiguredProjectServiceModels(project.ID, runtimeMode, initialServices)
		if err != nil {
			return nil, err
		}
		if err := s.services.ReplaceForProject(project.ID, items); err != nil {
			return nil, err
		}
	} else if s.internalServices != nil {
		items, err := buildProjectInternalServiceModels(project.ID, internalServiceKinds)
		if err != nil {
			return nil, err
		}
		if err := s.internalServices.ReplaceForProject(project.ID, items); err != nil {
			return nil, err
		}
	}

	summary := ToProjectSummary(*project)
	return &summary, nil
}

func (s *ProjectService) List(userID string) ([]ProjectSummary, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}

	projects, err := s.projects.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	items := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		items = append(items, ToProjectSummary(project))
	}

	return items, nil
}

func (s *ProjectService) GetSummary(requesterUserID, requesterRole, projectID string) (*ProjectSummary, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}
	summary := ToProjectSummary(*project)
	return &summary, nil
}

func (s *ProjectService) ListServices(requesterUserID, requesterRole, projectID string) (*ProjectServiceListResult, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}
	records := make([]ProjectServiceRecord, 0)
	internalKeys := make(map[string]struct{})
	internalKinds := make(map[string]struct{})

	if s.services != nil {
		items, err := s.services.ListByProject(project.ID)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			record, err := ToProjectServiceRecord(item)
			if err != nil {
				return nil, err
			}
			records = append(records, record)
			if key, ok := internalProjectServiceKey(record.Kind, record.Path, record.SourceType); ok {
				internalKeys[key] = struct{}{}
			}
			if record.SourceType == serviceSourceTypeInternal {
				internalKinds[normalizeManagedInternalBridgeKind(record.Kind)] = struct{}{}
			}
		}
	}

	if s.internalServices != nil {
		internalItems, err := s.internalServices.ListByProject(project.ID)
		if err != nil {
			return nil, err
		}
		for _, item := range internalItems {
			if _, exists := internalKinds[normalizeManagedInternalBridgeKind(item.Kind)]; exists {
				continue
			}
			key, ok := internalProjectServiceKey(item.Kind, reservedManagedInternalServicePathPrefix+normalizeManagedInternalBridgeKind(item.Kind), serviceSourceTypeInternal)
			if ok {
				if _, exists := internalKeys[key]; exists {
					continue
				}
				internalKeys[key] = struct{}{}
			}
			records = append(records, legacyInternalServiceToProjectServiceRecord(item))
		}
	}

	sort.SliceStable(records, func(i, j int) bool {
		leftInternal := records[i].SourceType == serviceSourceTypeInternal
		rightInternal := records[j].SourceType == serviceSourceTypeInternal
		if leftInternal != rightInternal {
			return !leftInternal
		}
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		return records[i].ID < records[j].ID
	})
	return &ProjectServiceListResult{Items: records}, nil
}

func (s *ProjectService) ConfigureServices(cmd ConfigureProjectServicesCommand) (*ProjectServiceListResult, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}
	if s.services == nil {
		return &ProjectServiceListResult{Items: []ProjectServiceRecord{}}, nil
	}

	items, err := buildConfiguredProjectServiceModels(project.ID, project.RuntimeMode, cmd.Items)
	if err != nil {
		return nil, err
	}
	if !configurePayloadContainsInternalServices(cmd.Items) {
		preservedInternalItems, err := listManagedInternalProjectServices(s.services, project.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, preservedInternalItems...)
	}
	if err := s.services.ReplaceForProject(project.ID, items); err != nil {
		return nil, err
	}
	return s.ListServices(cmd.RequesterUserID, cmd.RequesterRole, project.ID)
}

func ToProjectSummary(project models.Project) ProjectSummary {
	clusterID := ""
	if project.ClusterID != nil {
		clusterID = strings.TrimSpace(*project.ClusterID)
	}
	return ProjectSummary{
		ID:            project.ID,
		Name:          project.Name,
		Slug:          project.Slug,
		NamespaceSlug: project.NamespaceSlug,
		ClusterID:     clusterID,
		RuntimeMode:   project.RuntimeMode,
		DefaultBranch: project.DefaultBranch,
		CreatedAt:     project.CreatedAt,
		UpdatedAt:     project.UpdatedAt,
	}
}

func legacyInternalServiceToProjectServiceRecord(item models.ProjectInternalService) ProjectServiceRecord {
	kind := normalizeManagedInternalBridgeKind(item.Kind)
	port := item.Port
	if port <= 0 {
		port = defaultManagedInternalBridgePort(kind)
	}
	protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
	if protocol == "" {
		protocol = "tcp"
	}

	record := ProjectServiceRecord{
		ID:                      item.ID,
		ProjectID:               item.ProjectID,
		Name:                    managedInternalBridgeName(kind),
		Path:                    reservedManagedInternalServicePathPrefix + kind,
		Kind:                    kind,
		SourceType:              serviceSourceTypeInternal,
		Public:                  false,
		RuntimeProfile:          defaultManagedInternalBridgeRuntimeProfile(kind),
		PlacementMode:           servicePlacementModeSharedCluster,
		PlacementNodeID:         "",
		ConnectionTemplateKey:   "",
		ConnectionTemplate:      defaultConnectionTemplateForKind(kind),
		ConnectionTargetService: "",
		Dependencies:            nil,
		ManagedByLazyops:        true,
		StartHint:               "managed-internal-service",
		ImageRef:                "",
		ImageDigest:             "",
		DetectedPorts:           []ServiceDetectedPortRecord{},
		TargetPort:              port,
		ServicePort:             port,
		Replicas:                1,
		EnvBundle:               map[string]string{},
		PVCSpec:                 map[string]any{},
		DeployStrategy:          map[string]any{},
		Healthcheck: map[string]any{
			"protocol": protocol,
			"port":     port,
		},
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}

	if spec, ok := defaultManagedInternalBridgePVCSpec(kind); ok {
		record.PVCSpec = spec
	}

	return record
}

func internalProjectServiceKey(kind, path, sourceType string) (string, bool) {
	normalizedPath := strings.TrimSpace(path)
	normalizedKind := normalizeManagedInternalBridgeKind(kind)
	isInternal := strings.TrimSpace(sourceType) == serviceSourceTypeInternal || strings.HasPrefix(normalizedPath, reservedManagedInternalServicePathPrefix)
	if !isInternal {
		return "", false
	}
	if normalizedPath == "" {
		normalizedPath = reservedManagedInternalServicePathPrefix + normalizedKind
	}
	return normalizedPath + "|" + normalizedKind, true
}

func normalizeManagedInternalBridgeKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "mongo":
		return "mongodb"
	case "eureka", "euruka", "euruka-server":
		return "eureka-server"
	case "postgres", "mysql", "mongodb", "redis", "rabbitmq", "kafka", "eureka-server":
		return kind
	default:
		return kind
	}
}

func managedInternalBridgeName(kind string) string {
	kind = normalizeManagedInternalBridgeKind(kind)
	if kind == "" {
		return "lazyops-internal-service"
	}
	return "lazyops-internal-" + kind
}

func defaultManagedInternalBridgePort(kind string) int {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "mongodb":
		return 27017
	case "redis":
		return 6379
	case "rabbitmq":
		return 5672
	case "kafka":
		return 9092
	case "eureka-server":
		return 8761
	default:
		return 0
	}
}

func defaultManagedInternalBridgeRuntimeProfile(kind string) string {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres", "mysql", "mongodb", "redis", "rabbitmq", "kafka":
		return "internal-db"
	default:
		return "service"
	}
}

func defaultManagedInternalBridgePVCSpec(kind string) (map[string]any, bool) {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres", "mysql", "mongodb", "redis", "rabbitmq", "kafka":
		return map[string]any{"size": "5Gi"}, true
	default:
		return nil, false
	}
}

func normalizeDefaultBranch(branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main", nil
	}
	if strings.ContainsAny(branch, " \t\r\n") {
		return "", ErrInvalidInput
	}

	return branch, nil
}

func normalizeProjectSlug(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(input))
	lastHyphen := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		default:
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	slug := strings.Trim(b.String(), "-")
	if len(slug) > 63 {
		slug = strings.Trim(slug[:63], "-")
	}

	return slug
}

func normalizeNamespaceSlug(raw, fallback string) string {
	if strings.TrimSpace(raw) == "" {
		raw = fallback
	}
	return normalizeProjectSlug(raw)
}

func buildConfiguredProjectServiceModels(projectID, runtimeMode string, items []ConfigureProjectServiceItem) ([]models.Service, error) {
	if len(items) == 0 {
		return []models.Service{}, nil
	}

	names := make(map[string]struct{}, len(items))
	paths := make(map[string]struct{}, len(items))
	out := make([]models.Service, 0, len(items))
	records := make([]ProjectServiceRecord, 0, len(items))
	for index, item := range items {
		model, err := normalizeConfiguredProjectService(projectID, runtimeMode, item)
		if err != nil {
			return nil, fmt.Errorf("%w: services[%d]: %s", ErrInvalidInput, index, err.Error())
		}
		if _, exists := names[model.Name]; exists {
			return nil, fmt.Errorf("%w: services[%d]: duplicate service name %q", ErrInvalidInput, index, model.Name)
		}
		if _, exists := paths[model.Path]; exists {
			return nil, fmt.Errorf("%w: services[%d]: duplicate service path %q", ErrInvalidInput, index, model.Path)
		}
		names[model.Name] = struct{}{}
		paths[model.Path] = struct{}{}
		record, err := ToProjectServiceRecord(model)
		if err != nil {
			return nil, fmt.Errorf("%w: services[%d]: %s", ErrInvalidInput, index, err.Error())
		}
		records = append(records, record)
		out = append(out, model)
	}
	if err := validateConfiguredProjectServiceInventory(records); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeConfiguredProjectService(projectID, runtimeMode string, item ConfigureProjectServiceItem) (models.Service, error) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return models.Service{}, fmt.Errorf("service.name is required")
	}
	if !lazyopsLogicalNamePattern.MatchString(name) {
		return models.Service{}, fmt.Errorf("service.name must contain only letters, digits, dots, underscores, or hyphens")
	}
	if strings.TrimSpace(runtimeMode) == "distributed-k3s" {
		if err := validateK3sResourceName("service.name", name); err != nil {
			return models.Service{}, err
		}
	}
	if strings.HasPrefix(strings.ToLower(name), "lazyops-internal-") {
		return models.Service{}, fmt.Errorf("service.name uses a reserved lazyops internal prefix")
	}

	sourceType, err := normalizeConfiguredServiceSourceType(item.SourceType, item.Path)
	if err != nil {
		return models.Service{}, err
	}

	kind := normalizeConfiguredProjectServiceKind(item.Kind, name)
	path, err := normalizeConfiguredServicePath(sourceType, kind, name, item.Path)
	if err != nil {
		return models.Service{}, err
	}
	runtimeProfile := normalizeConfiguredRuntimeProfile(item.RuntimeProfile, kind, item.Public, item.Healthcheck)
	placementMode, placementNodeID, err := normalizeConfiguredServicePlacement(item.PlacementMode, item.PlacementNodeID)
	if err != nil {
		return models.Service{}, err
	}
	dependencies, err := normalizeConfiguredDependencies(item.Dependencies, item.ConnectionTargetService, item.ConnectionTemplateKey, item.ConnectionTemplate)
	if err != nil {
		return models.Service{}, err
	}
	rootConnectionTemplateKey := ""
	rootConnectionTemplate := map[string]string{}
	if sourceType == serviceSourceTypeInternal {
		rootConnectionTemplateKey, err = normalizeConfiguredConnectionTemplateKey(item.ConnectionTemplateKey)
		if err != nil {
			return models.Service{}, err
		}
		rootConnectionTemplate, err = normalizeConfiguredConnectionTemplate(item.ConnectionTemplate, sourceType, kind)
		if err != nil {
			return models.Service{}, err
		}
	}
	managedByLazyops := item.ManagedByLazyops || sourceType == serviceSourceTypeInternal || strings.HasPrefix(path, reservedManagedInternalServicePathPrefix)
	targetPort, err := normalizeConfiguredPort(item.TargetPort)
	if err != nil {
		return models.Service{}, err
	}
	servicePort, err := normalizeConfiguredPort(item.ServicePort)
	if err != nil {
		return models.Service{}, err
	}
	if defaultPort := defaultConfiguredTargetPort(kind); targetPort == 0 && defaultPort > 0 {
		targetPort = defaultPort
	}
	if servicePort == 0 && targetPort > 0 {
		servicePort = targetPort
	}
	replicas := item.Replicas
	if replicas == 0 {
		replicas = 1
	}
	if replicas < 0 {
		return models.Service{}, fmt.Errorf("service.replicas must be positive")
	}

	healthcheck, err := normalizeConfiguredServiceHealthcheck(item.Healthcheck)
	if err != nil {
		return models.Service{}, err
	}
	if sourceType == serviceSourceTypeInternal {
		if defaults, defaultErr := normalizeManagedInternalServiceDefaults(kind, name, item.Public, runtimeProfile, item.ImageRef, targetPort, servicePort, replicas, item.EnvBundle, item.PVCSpec, healthcheck); defaultErr != nil {
			return models.Service{}, defaultErr
		} else {
			runtimeProfile = defaults.RuntimeProfile
			item.Public = defaults.Public
			item.ImageRef = defaults.ImageRef
			targetPort = defaults.TargetPort
			servicePort = defaults.ServicePort
			replicas = defaults.Replicas
			item.EnvBundle = defaults.EnvBundle
			item.PVCSpec = defaults.PVCSpec
			healthcheck = defaults.Healthcheck
		}
	}
	healthcheckJSON, err := marshalBindingPolicyJSON(healthcheck)
	if err != nil {
		return models.Service{}, err
	}
	envBundleJSON, err := marshalStringMapJSON(item.EnvBundle)
	if err != nil {
		return models.Service{}, err
	}
	pvcSpecJSON, err := marshalBindingPolicyJSON(cloneConfiguredAnyMap(item.PVCSpec))
	if err != nil {
		return models.Service{}, err
	}
	deployStrategy := cloneConfiguredAnyMap(item.DeployStrategy)
	applyServiceContractMetadata(
		deployStrategy,
		sourceType,
		placementMode,
		placementNodeID,
		dependencies,
		rootConnectionTemplateKey,
		rootConnectionTemplate,
		managedByLazyops,
	)
	deployStrategyJSON, err := marshalBindingPolicyJSON(deployStrategy)
	if err != nil {
		return models.Service{}, err
	}

	model := models.Service{
		ID:                 utils.NewPrefixedID("svc"),
		ProjectID:          projectID,
		Name:               name,
		Path:               path,
		Kind:               kind,
		Public:             item.Public,
		StartHint:          strings.TrimSpace(item.StartHint),
		ImageRef:           strings.TrimSpace(item.ImageRef),
		ImageDigest:        strings.TrimSpace(item.ImageDigest),
		DetectedPortsJSON:  "[]",
		TargetPort:         targetPort,
		ServicePort:        servicePort,
		Replicas:           replicas,
		EnvBundleJSON:      envBundleJSON,
		PVCSpecJSON:        pvcSpecJSON,
		DeployStrategyJSON: deployStrategyJSON,
		HealthcheckJSON:    healthcheckJSON,
	}
	if runtimeProfile != "" {
		runtimeProfileCopy := runtimeProfile
		model.RuntimeProfile = &runtimeProfileCopy
	}
	return model, nil
}

type managedInternalServiceDefaults struct {
	Public         bool
	RuntimeProfile string
	ImageRef       string
	TargetPort     int
	ServicePort    int
	Replicas       int
	EnvBundle      map[string]string
	PVCSpec        map[string]any
	Healthcheck    map[string]any
}

func normalizeConfiguredServiceSourceType(raw, path string) (string, error) {
	sourceType := strings.ToLower(strings.TrimSpace(raw))
	switch sourceType {
	case "":
		if strings.HasPrefix(path, reservedManagedInternalServicePathPrefix) {
			return serviceSourceTypeInternal, nil
		}
		return serviceSourceTypeRepo, nil
	case serviceSourceTypeRepo, serviceSourceTypeInternal:
		return sourceType, nil
	default:
		return "", fmt.Errorf("service.source_type must be repo or internal")
	}
}

func normalizeConfiguredServicePath(sourceType, kind, name, raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if sourceType == serviceSourceTypeInternal {
		if path == "" {
			path = buildManagedInternalServicePath(kind, name)
		}
		if !strings.HasPrefix(path, reservedManagedInternalServicePathPrefix) {
			return "", fmt.Errorf("service.path for internal services must stay under %s", reservedManagedInternalServicePathPrefix)
		}
		if err := validateManagedInternalServicePath(path); err != nil {
			return "", err
		}
		return path, nil
	}
	if err := validateLazyopsRepoRelativePath("service.path", path); err != nil {
		return "", err
	}
	if strings.HasPrefix(path, reservedManagedInternalServicePathPrefix) {
		return "", fmt.Errorf("service.path uses a reserved lazyops internal prefix")
	}
	return path, nil
}

func normalizeConfiguredServicePlacement(rawMode, rawNodeID string) (string, string, error) {
	mode := strings.ToLower(strings.TrimSpace(rawMode))
	nodeID := strings.TrimSpace(rawNodeID)
	if mode == "" {
		mode = servicePlacementModeSharedCluster
	}
	switch mode {
	case servicePlacementModeSharedCluster:
		return mode, "", nil
	case servicePlacementModePinnedNode:
		if nodeID == "" {
			return "", "", fmt.Errorf("service.placement_node_id is required when placement_mode is pinned_node")
		}
		return mode, nodeID, nil
	default:
		return "", "", fmt.Errorf("service.placement_mode must be shared_cluster or pinned_node")
	}
}

func normalizeConfiguredConnectionTemplateKey(raw string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return "", nil
	}
	if !lazyopsLogicalNamePattern.MatchString(key) {
		return "", fmt.Errorf("service.connection_template_key must contain only letters, digits, dots, underscores, or hyphens")
	}
	return key, nil
}

func normalizeConfiguredConnectionTemplate(raw map[string]string, sourceType, kind string) (map[string]string, error) {
	if sourceType == serviceSourceTypeInternal && isRelationalDatabaseKind(kind) {
		return normalizeConnectionTemplateForKind(kind, raw)
	}
	if len(raw) > 0 {
		return nil, fmt.Errorf("service.connection_template is only supported for internal postgres or mysql services")
	}
	return map[string]string{}, nil
}

func normalizeConfiguredConnectionTargetService(raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", nil
	}
	if !lazyopsLogicalNamePattern.MatchString(target) {
		return "", fmt.Errorf("service.connection_target_service must contain only letters, digits, dots, underscores, or hyphens")
	}
	return target, nil
}

func applyServiceContractMetadata(
	target map[string]any,
	sourceType,
	placementMode,
	placementNodeID string,
	dependencies []ProjectServiceDependencyBinding,
	rootConnectionTemplateKey string,
	rootConnectionTemplate map[string]string,
	managedByLazyops bool,
) {
	if target == nil {
		return
	}
	target[lazyopsServiceMetaSourceType] = sourceType
	target[lazyopsServiceMetaPlacementMode] = placementMode
	if placementNodeID != "" {
		target[lazyopsServiceMetaPlacementNodeID] = placementNodeID
	}
	if len(dependencies) > 0 {
		target[lazyopsServiceMetaDependencies] = cloneProjectServiceDependencyBindings(dependencies)
	}
	connectionTemplateKey := rootConnectionTemplateKey
	connectionTemplate := cloneStringMap(rootConnectionTemplate)
	connectionTargetService := ""
	if sourceType != serviceSourceTypeInternal {
		connectionTemplateKey, connectionTemplate, connectionTargetService = legacyDependencyFields(dependencies)
	}
	if connectionTemplateKey != "" {
		target[lazyopsServiceMetaConnectionTemplateKey] = connectionTemplateKey
	}
	if len(connectionTemplate) > 0 {
		target[lazyopsServiceMetaConnectionTemplate] = cloneStringMap(connectionTemplate)
	}
	if connectionTargetService != "" {
		target[lazyopsServiceMetaConnectionTargetService] = connectionTargetService
	}
	target[lazyopsServiceMetaManagedByLazyops] = managedByLazyops
}

func normalizeConfiguredProjectServiceKind(raw, name string) string {
	kind := strings.ToLower(strings.TrimSpace(raw))
	switch kind {
	case "", "service":
		return inferServiceKind(LazyopsYAMLService{Name: name})
	case "db", "database":
		inferred := inferServiceKind(LazyopsYAMLService{Name: name})
		if inferred == "app" {
			return "internal-db"
		}
		return inferred
	default:
		return kind
	}
}

func normalizeConfiguredRuntimeProfile(raw, kind string, public bool, healthcheck map[string]any) string {
	profile := strings.ToLower(strings.TrimSpace(raw))
	if profile != "" {
		return profile
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "frontend", "web":
		return "web"
	case "worker":
		return "worker"
	case "postgres", "mysql", "mongodb", "redis", "rabbitmq", "kafka", "internal-db":
		return "internal-db"
	}
	if public {
		return "web"
	}
	if len(healthcheck) > 0 {
		return "service"
	}
	return "worker"
}

func normalizeManagedInternalServiceDefaults(kind, name string, public bool, runtimeProfile, imageRef string, targetPort, servicePort, replicas int, envBundle map[string]string, pvcSpec, healthcheck map[string]any) (managedInternalServiceDefaults, error) {
	kind = normalizeManagedInternalBridgeKind(kind)
	if kind == "" {
		return managedInternalServiceDefaults{}, fmt.Errorf("service.kind is required for internal services")
	}
	defaultPort := defaultConfiguredTargetPort(kind)
	out := managedInternalServiceDefaults{
		Public:         false,
		RuntimeProfile: firstNonEmptyCompiledValue(strings.TrimSpace(runtimeProfile), defaultManagedInternalBridgeRuntimeProfile(kind)),
		ImageRef:       strings.TrimSpace(imageRef),
		TargetPort:     firstPositive(targetPort, defaultPort),
		ServicePort:    firstPositive(servicePort, targetPort, defaultPort),
		Replicas:       replicas,
		EnvBundle:      cloneStringMap(envBundle),
		PVCSpec:        cloneConfiguredAnyMap(pvcSpec),
		Healthcheck:    cloneConfiguredAnyMap(healthcheck),
	}
	if out.ImageRef == "" {
		out.ImageRef = defaultManagedInternalImageRef(kind)
	}
	if out.Replicas <= 0 {
		out.Replicas = 1
	}
	if len(out.PVCSpec) == 0 {
		if spec, ok := defaultManagedInternalBridgePVCSpec(kind); ok {
			out.PVCSpec = spec
		}
	}
	if normalizedEnv, err := normalizeManagedInternalRuntimeEnv(kind, name, out.EnvBundle); err != nil {
		return managedInternalServiceDefaults{}, err
	} else {
		out.EnvBundle = normalizedEnv
	}
	if len(out.Healthcheck) == 0 && out.TargetPort > 0 {
		out.Healthcheck = map[string]any{
			"protocol": "tcp",
			"port":     out.TargetPort,
		}
	}
	return out, nil
}

func normalizeManagedInternalRuntimeEnv(kind, serviceName string, env map[string]string) (map[string]string, error) {
	out := cloneStringMap(env)
	if out == nil {
		out = map[string]string{}
	}
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres":
		fillIfBlankString(out, "POSTGRES_DB", "app")
		fillIfBlankString(out, "POSTGRES_USER", "postgres")
		if strings.TrimSpace(out["POSTGRES_PASSWORD"]) == "" {
			password, err := randomCredentialHex(32)
			if err != nil {
				return nil, err
			}
			out["POSTGRES_PASSWORD"] = password
		}
	case "mysql":
		fillIfBlankString(out, "MYSQL_DATABASE", "app")
		fillIfBlankString(out, "MYSQL_USER", "mysql")
		if strings.TrimSpace(out["MYSQL_PASSWORD"]) == "" {
			password, err := randomCredentialHex(32)
			if err != nil {
				return nil, err
			}
			out["MYSQL_PASSWORD"] = password
		}
		if strings.TrimSpace(out["MYSQL_ROOT_PASSWORD"]) == "" {
			rootPassword, err := randomCredentialHex(32)
			if err != nil {
				return nil, err
			}
			out["MYSQL_ROOT_PASSWORD"] = rootPassword
		}
	case "mongodb":
		fillIfBlankString(out, "MONGO_INITDB_DATABASE", "app")
	case "kafka":
		host := firstNonEmptyCompiledValue(strings.TrimSpace(serviceName), "kafka")
		port := strconv.Itoa(defaultManagedInternalBridgePort(kind))
		fillIfBlankString(out, "KAFKA_NODE_ID", "1")
		fillIfBlankString(out, "KAFKA_PROCESS_ROLES", "broker,controller")
		fillIfBlankString(out, "KAFKA_LISTENERS", "PLAINTEXT://0.0.0.0:"+port+",CONTROLLER://0.0.0.0:9093")
		fillIfBlankString(out, "KAFKA_ADVERTISED_LISTENERS", "PLAINTEXT://"+host+":"+port)
		fillIfBlankString(out, "KAFKA_CONTROLLER_LISTENER_NAMES", "CONTROLLER")
		fillIfBlankString(out, "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP", "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT")
		fillIfBlankString(out, "KAFKA_INTER_BROKER_LISTENER_NAME", "PLAINTEXT")
		fillIfBlankString(out, "KAFKA_CONTROLLER_QUORUM_VOTERS", "1@localhost:9093")
		fillIfBlankString(out, "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR", "1")
		fillIfBlankString(out, "KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR", "1")
		fillIfBlankString(out, "KAFKA_TRANSACTION_STATE_LOG_MIN_ISR", "1")
		fillIfBlankString(out, "KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS", "0")
		fillIfBlankString(out, "KAFKA_NUM_PARTITIONS", "3")
		fillIfBlankString(out, "CLUSTER_ID", "lazyops-kafka-cluster")
	case "eureka-server":
		fillIfBlankString(out, "SERVER_PORT", strconv.Itoa(defaultManagedInternalBridgePort(kind)))
		fillIfBlankString(out, "EUREKA_CLIENT_REGISTER_WITH_EUREKA", "false")
		fillIfBlankString(out, "EUREKA_CLIENT_FETCH_REGISTRY", "false")
	}
	return out, nil
}

func randomCredentialHex(numBytes int) (string, error) {
	if numBytes <= 0 {
		return "", ErrInvalidInput
	}
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate managed internal credential: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func fillIfBlankString(target map[string]string, key, value string) {
	if target == nil {
		return
	}
	if strings.TrimSpace(target[key]) != "" {
		return
	}
	target[key] = value
}

func normalizeConfiguredPort(port int) (int, error) {
	if port < 0 || port > 65535 {
		return 0, fmt.Errorf("service ports must be between 1 and 65535")
	}
	return port, nil
}

func defaultManagedInternalImageRef(kind string) string {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres":
		return "postgres:16-alpine"
	case "mysql":
		return "mysql:8"
	case "mongodb":
		return "mongo:7"
	case "redis":
		return "redis:7-alpine"
	case "rabbitmq":
		return "rabbitmq:3-management-alpine"
	case "kafka":
		return "apache/kafka:3.7.0"
	case "eureka-server":
		return "springcloud/eureka"
	default:
		return ""
	}
}

func defaultConfiguredTargetPort(kind string) int {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "mongodb":
		return 27017
	case "redis":
		return 6379
	case "rabbitmq":
		return 5672
	case "kafka":
		return 9092
	case "eureka-server":
		return 8761
	default:
		return 0
	}
}

func normalizeConfiguredServiceHealthcheck(raw map[string]any) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}

	path := strings.TrimSpace(extractHealthcheckString(raw, "path"))
	port := extractHealthcheckPort(raw)
	protocol := strings.ToLower(strings.TrimSpace(extractHealthcheckString(raw, "protocol")))
	if protocol == "" {
		protocol = "http"
	}
	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("service.healthcheck.port must be between 1 and 65535")
	}
	if protocol == "tcp" {
		if port <= 0 {
			return nil, fmt.Errorf("service.healthcheck.port must be between 1 and 65535")
		}
		return map[string]any{
			"protocol": protocol,
			"port":     port,
		}, nil
	}
	if err := validateLazyopsHealthcheck(LazyopsYAMLServiceHealthcheck{Path: path, Port: port}); err != nil {
		return nil, err
	}
	if path == "" && port == 0 {
		return map[string]any{}, nil
	}
	return map[string]any{
		"path":     path,
		"port":     port,
		"protocol": protocol,
	}, nil
}

func cloneConfiguredAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func marshalStringMapJSON(input map[string]string) (string, error) {
	if input == nil {
		input = map[string]string{}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func configurePayloadContainsInternalServices(items []ConfigureProjectServiceItem) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.SourceType), serviceSourceTypeInternal) || strings.HasPrefix(strings.TrimSpace(item.Path), reservedManagedInternalServicePathPrefix) {
			return true
		}
	}
	return false
}

func listManagedInternalProjectServices(store ProjectServiceStore, projectID string) ([]models.Service, error) {
	if store == nil {
		return nil, nil
	}
	items, err := store.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	out := make([]models.Service, 0)
	for _, item := range items {
		record, err := ToProjectServiceRecord(item)
		if err != nil {
			return nil, err
		}
		if record.SourceType == serviceSourceTypeInternal || strings.HasPrefix(strings.TrimSpace(item.Path), reservedManagedInternalServicePathPrefix) {
			out = append(out, item)
		}
	}
	return out, nil
}

func buildManagedInternalServicePath(kind, name string) string {
	kind = normalizeManagedInternalBridgeKind(kind)
	name = strings.TrimSpace(name)
	if kind == "" || name == "" {
		return reservedManagedInternalServicePathPrefix + strings.TrimPrefix(kind, "/")
	}
	return reservedManagedInternalServicePathPrefix + kind + "/" + name
}

func validateManagedInternalServicePath(path string) error {
	trimmed := strings.TrimSpace(path)
	if !strings.HasPrefix(trimmed, reservedManagedInternalServicePathPrefix) {
		return fmt.Errorf("service.path uses a reserved lazyops internal prefix")
	}
	relative := strings.TrimPrefix(trimmed, reservedManagedInternalServicePathPrefix)
	parts := strings.Split(relative, "/")
	if len(parts) < 2 {
		return fmt.Errorf("service.path for internal services must be in .lazyops/internal/<kind>/<service-name> form")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return fmt.Errorf("service.path for internal services must not contain empty segments")
		}
	}
	return nil
}

func buildInitialProjectServiceItems(explicit []ConfigureProjectServiceItem, internalKinds []string) []ConfigureProjectServiceItem {
	out := make([]ConfigureProjectServiceItem, 0, len(explicit)+len(internalKinds))
	existingInternalKinds := make(map[string]struct{}, len(explicit))

	for _, item := range explicit {
		cloned := item
		cloned.EnvBundle = cloneStringMap(item.EnvBundle)
		cloned.Dependencies = cloneProjectServiceDependencyBindings(item.Dependencies)
		cloned.ConnectionTemplate = cloneStringMap(item.ConnectionTemplate)
		cloned.PVCSpec = cloneConfiguredAnyMap(item.PVCSpec)
		cloned.DeployStrategy = cloneConfiguredAnyMap(item.DeployStrategy)
		cloned.Healthcheck = cloneConfiguredAnyMap(item.Healthcheck)
		out = append(out, cloned)

		sourceType := strings.ToLower(strings.TrimSpace(item.SourceType))
		if sourceType == "" && strings.HasPrefix(strings.TrimSpace(item.Path), reservedManagedInternalServicePathPrefix) {
			sourceType = serviceSourceTypeInternal
		}
		if sourceType != serviceSourceTypeInternal {
			continue
		}
		existingInternalKinds[normalizeManagedInternalBridgeKind(item.Kind)] = struct{}{}
	}

	for _, kind := range internalKinds {
		if _, exists := existingInternalKinds[kind]; exists {
			continue
		}
		out = append(out, ConfigureProjectServiceItem{
			Name:               kind,
			Kind:               kind,
			SourceType:         serviceSourceTypeInternal,
			ManagedByLazyops:   true,
			ConnectionTemplate: defaultConnectionTemplateForKind(kind),
		})
	}

	return out
}

func normalizeConfiguredDependencies(
	raw []ProjectServiceDependencyBinding,
	legacyTargetService, legacyTemplateKey string,
	legacyTemplate map[string]string,
) ([]ProjectServiceDependencyBinding, error) {
	out := make([]ProjectServiceDependencyBinding, 0, len(raw)+1)
	seenTargets := make(map[string]struct{}, len(raw)+1)
	appendBinding := func(binding ProjectServiceDependencyBinding, allowDuplicate bool) error {
		targetService, err := normalizeConfiguredConnectionTargetService(binding.TargetService)
		if err != nil {
			return err
		}
		if targetService == "" {
			return nil
		}
		templateKey, err := normalizeConfiguredConnectionTemplateKey(binding.ConnectionTemplateKey)
		if err != nil {
			return err
		}
		clonedTemplate := cloneStringMap(binding.ConnectionTemplate)
		if _, exists := seenTargets[targetService]; exists {
			if allowDuplicate {
				return nil
			}
			return fmt.Errorf("service.dependencies contains duplicate target_service %q", targetService)
		}
		seenTargets[targetService] = struct{}{}
		out = append(out, ProjectServiceDependencyBinding{
			TargetService:         targetService,
			ConnectionTemplateKey: templateKey,
			ConnectionTemplate:    clonedTemplate,
		})
		return nil
	}
	for _, item := range raw {
		if err := appendBinding(item, false); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(legacyTargetService) != "" {
		if err := appendBinding(ProjectServiceDependencyBinding{
			TargetService:         legacyTargetService,
			ConnectionTemplateKey: legacyTemplateKey,
			ConnectionTemplate:    legacyTemplate,
		}, true); err != nil {
			return nil, err
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func legacyDependencyFields(items []ProjectServiceDependencyBinding) (string, map[string]string, string) {
	if len(items) == 0 {
		return "", map[string]string{}, ""
	}
	first := items[0]
	return first.ConnectionTemplateKey, cloneStringMap(first.ConnectionTemplate), first.TargetService
}

func cloneProjectServiceDependencyBindings(items []ProjectServiceDependencyBinding) []ProjectServiceDependencyBinding {
	if len(items) == 0 {
		return nil
	}
	out := make([]ProjectServiceDependencyBinding, 0, len(items))
	for _, item := range items {
		out = append(out, ProjectServiceDependencyBinding{
			TargetService:         strings.TrimSpace(item.TargetService),
			ConnectionTemplateKey: strings.TrimSpace(item.ConnectionTemplateKey),
			ConnectionTemplate:    cloneStringMap(item.ConnectionTemplate),
		})
	}
	return out
}

func validateConfiguredProjectServiceInventory(records []ProjectServiceRecord) error {
	if len(records) == 0 {
		return nil
	}
	serviceIndex := make(map[string]ProjectServiceRecord, len(records))
	for _, record := range records {
		serviceIndex[record.Name] = record
	}
	for _, record := range records {
		for _, binding := range record.Dependencies {
			targetName := strings.TrimSpace(binding.TargetService)
			if targetName == "" {
				continue
			}
			if targetName == record.Name {
				return fmt.Errorf("%w: service %q must not depend on itself", ErrInvalidInput, record.Name)
			}
			target, ok := serviceIndex[targetName]
			if !ok {
				return fmt.Errorf("%w: service %q depends on unknown internal service %q", ErrInvalidInput, record.Name, targetName)
			}
			if target.SourceType != serviceSourceTypeInternal {
				return fmt.Errorf("%w: service %q can only depend on internal services; %q is %s", ErrInvalidInput, record.Name, targetName, firstNonEmptyCompiledValue(target.SourceType, serviceSourceTypeRepo))
			}
			if len(binding.ConnectionTemplate) > 0 {
				if !isRelationalDatabaseKind(target.Kind) {
					return fmt.Errorf("%w: service %q uses connection_template with non-relational dependency %q", ErrInvalidInput, record.Name, targetName)
				}
				if _, err := normalizeConnectionTemplateForKind(target.Kind, binding.ConnectionTemplate); err != nil {
					return fmt.Errorf("%w: service %q dependency %q: %s", ErrInvalidInput, record.Name, targetName, err.Error())
				}
			}
			if key := strings.TrimSpace(binding.ConnectionTemplateKey); key != "" {
				if !isRelationalDatabaseKind(target.Kind) {
					return fmt.Errorf("%w: service %q uses connection_template_key with non-relational dependency %q", ErrInvalidInput, record.Name, targetName)
				}
				expected := relationalConnectionTemplateKeyForKind(target.Kind)
				if expected != "" && key != expected {
					return fmt.Errorf("%w: service %q dependency %q expects connection_template_key %q", ErrInvalidInput, record.Name, targetName, expected)
				}
			}
		}
	}
	return validateProjectServicePortConflicts(records)
}

func validateProjectServicePortConflicts(records []ProjectServiceRecord) error {
	type portOwner struct {
		ServiceName string
		Field       string
	}
	owners := make(map[int]portOwner)
	for _, record := range records {
		for field, port := range map[string]int{
			"target_port":  record.TargetPort,
			"service_port": record.ServicePort,
		} {
			if port <= 0 {
				continue
			}
			if existing, ok := owners[port]; ok {
				if existing.ServiceName == record.Name {
					continue
				}
				return fmt.Errorf(
					"%w: port %d is already used by service %q (%s) and conflicts with service %q (%s)",
					ErrInvalidInput,
					port,
					existing.ServiceName,
					existing.Field,
					record.Name,
					field,
				)
			}
			owners[port] = portOwner{ServiceName: record.Name, Field: field}
		}
	}
	return nil
}
