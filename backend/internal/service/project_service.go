package service

import (
	"encoding/json"
	"errors"
	"fmt"
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
	lazyopsServiceMetaConnectionTemplateKey   = "_lazyops_connection_template_key"
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
	if s.internalServices != nil {
		internalServiceKinds, err = normalizeInternalServiceKinds(cmd.InternalServices)
		if err != nil {
			return nil, err
		}
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
	if s.internalServices != nil {
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
		ConnectionTargetService: "",
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
	case "postgres", "mysql", "redis", "rabbitmq":
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
	case "redis":
		return 6379
	case "rabbitmq":
		return 5672
	default:
		return 0
	}
}

func defaultManagedInternalBridgeRuntimeProfile(kind string) string {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres", "mysql", "redis", "rabbitmq":
		return "internal-db"
	default:
		return "service"
	}
}

func defaultManagedInternalBridgePVCSpec(kind string) (map[string]any, bool) {
	switch normalizeManagedInternalBridgeKind(kind) {
	case "postgres", "mysql", "redis", "rabbitmq":
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
		out = append(out, model)
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
	connectionTemplateKey, err := normalizeConfiguredConnectionTemplateKey(item.ConnectionTemplateKey)
	if err != nil {
		return models.Service{}, err
	}
	connectionTargetService, err := normalizeConfiguredConnectionTargetService(item.ConnectionTargetService)
	if err != nil {
		return models.Service{}, err
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
	applyServiceContractMetadata(deployStrategy, sourceType, placementMode, placementNodeID, connectionTemplateKey, connectionTargetService, managedByLazyops)
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

func applyServiceContractMetadata(target map[string]any, sourceType, placementMode, placementNodeID, connectionTemplateKey, connectionTargetService string, managedByLazyops bool) {
	if target == nil {
		return
	}
	target[lazyopsServiceMetaSourceType] = sourceType
	target[lazyopsServiceMetaPlacementMode] = placementMode
	if placementNodeID != "" {
		target[lazyopsServiceMetaPlacementNodeID] = placementNodeID
	}
	if connectionTemplateKey != "" {
		target[lazyopsServiceMetaConnectionTemplateKey] = connectionTemplateKey
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
	case "postgres", "mysql", "redis", "rabbitmq", "internal-db":
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
	if len(out.Healthcheck) == 0 && out.TargetPort > 0 {
		out.Healthcheck = map[string]any{
			"protocol": "tcp",
			"port":     out.TargetPort,
		}
	}
	return out, nil
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
	case "redis":
		return "redis:7-alpine"
	case "rabbitmq":
		return "rabbitmq:3-management-alpine"
	default:
		return ""
	}
}

func defaultConfiguredTargetPort(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
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
