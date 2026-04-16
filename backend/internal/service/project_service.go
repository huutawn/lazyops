package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var ErrProjectSlugExists = errors.New("project slug already exists")

const reservedManagedInternalServicePathPrefix = ".lazyops/internal/"

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
	if s.services == nil {
		return &ProjectServiceListResult{Items: []ProjectServiceRecord{}}, nil
	}

	items, err := s.services.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}
	records := make([]ProjectServiceRecord, 0, len(items))
	for _, item := range items {
		record, err := ToProjectServiceRecord(item)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
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

	items, err := buildConfiguredProjectServiceModels(project.ID, cmd.Items)
	if err != nil {
		return nil, err
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

func buildConfiguredProjectServiceModels(projectID string, items []ConfigureProjectServiceItem) ([]models.Service, error) {
	if len(items) == 0 {
		return []models.Service{}, nil
	}

	names := make(map[string]struct{}, len(items))
	paths := make(map[string]struct{}, len(items))
	out := make([]models.Service, 0, len(items))
	for index, item := range items {
		model, err := normalizeConfiguredProjectService(projectID, item)
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

func normalizeConfiguredProjectService(projectID string, item ConfigureProjectServiceItem) (models.Service, error) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return models.Service{}, fmt.Errorf("service.name is required")
	}
	if !lazyopsLogicalNamePattern.MatchString(name) {
		return models.Service{}, fmt.Errorf("service.name must contain only letters, digits, dots, underscores, or hyphens")
	}
	if strings.HasPrefix(strings.ToLower(name), "lazyops-internal-") {
		return models.Service{}, fmt.Errorf("service.name uses a reserved lazyops internal prefix")
	}

	path := strings.TrimSpace(item.Path)
	if err := validateLazyopsRepoRelativePath("service.path", path); err != nil {
		return models.Service{}, err
	}
	if strings.HasPrefix(path, reservedManagedInternalServicePathPrefix) {
		return models.Service{}, fmt.Errorf("service.path uses a reserved lazyops internal prefix")
	}

	kind := normalizeConfiguredProjectServiceKind(item.Kind, name)
	runtimeProfile := normalizeConfiguredRuntimeProfile(item.RuntimeProfile, kind, item.Public, item.Healthcheck)
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
	deployStrategyJSON, err := marshalBindingPolicyJSON(cloneConfiguredAnyMap(item.DeployStrategy))
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

func normalizeConfiguredPort(port int) (int, error) {
	if port < 0 || port > 65535 {
		return 0, fmt.Errorf("service ports must be between 1 and 65535")
	}
	return port, nil
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
