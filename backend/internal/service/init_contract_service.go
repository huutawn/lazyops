package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"lazyops-server/internal/models"
)

var (
	ErrUnknownTargetRef         = errors.New("unknown target ref")
	ErrInvalidDependencyMapping = errors.New("invalid dependency mapping")
	ErrSecretBearingConfig      = errors.New("secret-bearing config")
	ErrHardCodedDeployAuthority = errors.New("hard-coded deploy authority")

	lazyopsProjectSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	lazyopsLogicalNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	lazyopsIPv4Pattern        = regexp.MustCompile(`\b(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\b`)

	lazyopsForbiddenMarkers = []string{
		"secret://",
		"kubeconfig",
		"-----begin private key-----",
		"-----begin rsa private key-----",
		"-----begin openssh private key-----",
		"ssh-rsa ",
		"github_pat_",
		"ghp_",
		"glpat-",
		"bearer ",
	}
	lazyopsAllowedDependencyProtocols = map[string]struct{}{
		"http":  {},
		"https": {},
		"tcp":   {},
		"grpc":  {},
	}
	lazyopsAllowedMagicDomainProviders = map[string]struct{}{
		"sslip.io": {},
		"nip.io":   {},
	}
	lazyopsForbiddenFieldReasons = map[string]lazyopsFieldViolation{
		"ssh":                   {ErrSecretBearingConfig, "repo contracts must not carry SSH access"},
		"ssh_key":               {ErrSecretBearingConfig, "repo contracts must not carry SSH access"},
		"private_key":           {ErrSecretBearingConfig, "repo contracts must not carry private keys"},
		"password":              {ErrSecretBearingConfig, "repo contracts must not carry passwords"},
		"pat":                   {ErrSecretBearingConfig, "repo contracts must not carry PATs"},
		"token":                 {ErrSecretBearingConfig, "repo contracts must not carry tokens"},
		"agent_token":           {ErrSecretBearingConfig, "repo contracts must not carry agent credentials"},
		"github_token":          {ErrSecretBearingConfig, "repo contracts must not carry GitHub credentials"},
		"secret":                {ErrSecretBearingConfig, "repo contracts must not carry raw secrets"},
		"kubeconfig":            {ErrSecretBearingConfig, "repo contracts must not carry kubeconfig data"},
		"kubeconfig_secret_ref": {ErrSecretBearingConfig, "repo contracts must not carry kubeconfig refs"},
		"public_ip":             {ErrHardCodedDeployAuthority, "repo contracts must not carry raw target IPs"},
		"private_ip":            {ErrHardCodedDeployAuthority, "repo contracts must not carry raw target IPs"},
		"server_ip":             {ErrHardCodedDeployAuthority, "repo contracts must not carry raw target IPs"},
		"project_id":            {ErrHardCodedDeployAuthority, "repo contracts must stay logical and avoid backend ids"},
		"deployment_binding_id": {ErrHardCodedDeployAuthority, "repo contracts must stay logical and avoid backend ids"},
		"target_id":             {ErrHardCodedDeployAuthority, "repo contracts must stay logical and avoid concrete target ids"},
		"target_kind":           {ErrHardCodedDeployAuthority, "repo contracts must stay logical and avoid concrete target kinds"},
		"instance_id":           {ErrHardCodedDeployAuthority, "repo contracts must stay logical and avoid concrete target ids"},
		"mesh_network_id":       {ErrHardCodedDeployAuthority, "repo contracts must stay logical and avoid concrete target ids"},
		"cluster_id":            {ErrHardCodedDeployAuthority, "repo contracts must stay logical and avoid concrete target ids"},
		"deploy_command":        {ErrHardCodedDeployAuthority, "repo contracts must not bypass git-push deploy triggers"},
	}
)

type lazyopsFieldViolation struct {
	err    error
	reason string
}

type InitContractService struct {
	projects  ProjectStore
	bindings  DeploymentBindingStore
	instances InstanceStore
	meshes    MeshNetworkStore
	clusters  ClusterStore
}

func NewInitContractService(
	projects ProjectStore,
	bindings DeploymentBindingStore,
	instances InstanceStore,
	meshes MeshNetworkStore,
	clusters ClusterStore,
) *InitContractService {
	return &InitContractService{
		projects:  projects,
		bindings:  bindings,
		instances: instances,
		meshes:    meshes,
		clusters:  clusters,
	}
}

func (s *InitContractService) ValidateLazyopsYAML(cmd ValidateLazyopsYAMLCommand) (*ValidateLazyopsYAMLResult, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}

	raw := bytes.TrimSpace(cmd.RawDocument)
	if len(raw) == 0 {
		return nil, ErrInvalidInput
	}
	if err := _lazyopsSchemaTypeLock(LazyopsYAMLDocument{}); err != nil {
		return nil, err
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, ErrInvalidInput
	}
	if err := validateLazyopsRawPayload(payload, "lazyops.yaml"); err != nil {
		return nil, err
	}

	var document LazyopsYAMLDocument
	if err := decodeLazyopsYAMLStrict(raw, &document); err != nil {
		return nil, err
	}
	if err := validateLazyopsDocument(document, *project); err != nil {
		return nil, err
	}

	targetRef := normalizeBindingTargetRef(document.DeploymentBinding.TargetRef)
	binding, err := s.bindings.GetByTargetRefForProject(project.ID, targetRef)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, fmt.Errorf("%w: deployment_binding.target_ref %q is not registered for project %q", ErrUnknownTargetRef, targetRef, project.Slug)
	}

	runtimeMode, err := normalizeBindingRuntimeMode(document.RuntimeMode)
	if err != nil {
		return nil, err
	}
	if binding.RuntimeMode != runtimeMode {
		return nil, fmt.Errorf("%w: binding %q uses runtime mode %q", ErrRuntimeModeMismatch, binding.TargetRef, binding.RuntimeMode)
	}

	record, err := ToDeploymentBindingRecord(*binding)
	if err != nil {
		return nil, err
	}
	targetSummary, err := s.resolveTargetSummary(*binding)
	if err != nil {
		return nil, err
	}

	return &ValidateLazyopsYAMLResult{
		Project:           ToProjectSummary(*project),
		DeploymentBinding: record,
		TargetSummary:     targetSummary,
		Schema: LazyopsYAMLSchemaSummary{
			AllowedDependencyProtocols: lazyopsAllowedDependencyProtocolList(),
			AllowedMagicDomainProviders: []string{
				"sslip.io",
				"nip.io",
			},
			ForbiddenFieldNames: lazyopsForbiddenFieldNameList(),
		},
	}, nil
}

func (s *InitContractService) resolveTargetSummary(binding models.DeploymentBinding) (InitTargetSummary, error) {
	switch binding.TargetKind {
	case "instance":
		instance, err := s.instances.GetByID(binding.TargetID)
		if err != nil {
			return InitTargetSummary{}, err
		}
		if instance == nil {
			return InitTargetSummary{}, ErrTargetNotFound
		}
		return InitTargetSummary{
			ID:          instance.ID,
			Name:        instance.Name,
			Kind:        "instance",
			Status:      instance.Status,
			RuntimeMode: binding.RuntimeMode,
		}, nil
	case "mesh":
		mesh, err := s.meshes.GetByID(binding.TargetID)
		if err != nil {
			return InitTargetSummary{}, err
		}
		if mesh == nil {
			return InitTargetSummary{}, ErrTargetNotFound
		}
		return InitTargetSummary{
			ID:          mesh.ID,
			Name:        mesh.Name,
			Kind:        "mesh",
			Status:      mesh.Status,
			RuntimeMode: binding.RuntimeMode,
		}, nil
	case "cluster":
		cluster, err := s.clusters.GetByID(binding.TargetID)
		if err != nil {
			return InitTargetSummary{}, err
		}
		if cluster == nil {
			return InitTargetSummary{}, ErrTargetNotFound
		}
		return InitTargetSummary{
			ID:          cluster.ID,
			Name:        cluster.Name,
			Kind:        "cluster",
			Status:      cluster.Status,
			RuntimeMode: binding.RuntimeMode,
		}, nil
	default:
		return InitTargetSummary{}, ErrInvalidInput
	}
}

func resolveProjectForAccess(projects ProjectStore, requesterUserID, requesterRole, projectID string) (*models.Project, error) {
	requesterUserID = strings.TrimSpace(requesterUserID)
	projectID = strings.TrimSpace(projectID)
	if requesterUserID == "" || projectID == "" {
		return nil, ErrInvalidInput
	}

	if requesterRole == RoleAdmin {
		project, err := projects.GetByID(projectID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, ErrProjectNotFound
		}
		return project, nil
	}

	project, err := projects.GetByIDForUser(requesterUserID, projectID)
	if err != nil {
		return nil, err
	}
	if project != nil {
		return project, nil
	}

	otherProject, err := projects.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if otherProject == nil {
		return nil, ErrProjectNotFound
	}

	return nil, ErrProjectAccessDenied
}

func decodeLazyopsYAMLStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidInput
	}
	if decoder.More() {
		return ErrInvalidInput
	}
	return nil
}

func validateLazyopsDocument(document LazyopsYAMLDocument, project models.Project) error {
	projectSlug := strings.TrimSpace(document.ProjectSlug)
	if projectSlug == "" {
		return ErrInvalidInput
	}
	if !lazyopsProjectSlugPattern.MatchString(projectSlug) {
		return ErrInvalidInput
	}
	if projectSlug != project.Slug {
		return ErrInvalidInput
	}

	runtimeMode, err := normalizeBindingRuntimeMode(document.RuntimeMode)
	if err != nil {
		return err
	}

	targetRef := strings.TrimSpace(document.DeploymentBinding.TargetRef)
	if targetRef == "" {
		return ErrInvalidInput
	}
	if !lazyopsLogicalNamePattern.MatchString(targetRef) {
		return fmt.Errorf("%w: deployment_binding.target_ref must stay a logical reference", ErrHardCodedDeployAuthority)
	}

	if len(document.Services) == 0 {
		return ErrInvalidInput
	}

	serviceNames := make(map[string]struct{}, len(document.Services))
	servicePaths := make(map[string]struct{}, len(document.Services))
	for index, svc := range document.Services {
		if err := validateLazyopsService(svc); err != nil {
			return fmt.Errorf("%w: services[%d]: %s", ErrInvalidInput, index, err.Error())
		}
		if _, exists := serviceNames[svc.Name]; exists {
			return fmt.Errorf("%w: services[%d]: duplicate service name %q", ErrInvalidInput, index, svc.Name)
		}
		if _, exists := servicePaths[svc.Path]; exists {
			return fmt.Errorf("%w: services[%d]: duplicate service path %q", ErrInvalidInput, index, svc.Path)
		}
		serviceNames[svc.Name] = struct{}{}
		servicePaths[svc.Path] = struct{}{}
	}

	for index, binding := range document.DependencyBindings {
		if err := validateLazyopsDependencyBinding(binding, serviceNames, runtimeMode); err != nil {
			return fmt.Errorf("%w: dependency_bindings[%d]: %s", ErrInvalidDependencyMapping, index, err.Error())
		}
	}

	if !document.CompatibilityPolicy.EnvInjection &&
		!document.CompatibilityPolicy.ManagedCredentials &&
		!document.CompatibilityPolicy.LocalhostRescue {
		return ErrInvalidInput
	}

	provider := strings.ToLower(strings.TrimSpace(document.MagicDomainPolicy.Provider))
	if provider == "" {
		if document.MagicDomainPolicy.Enabled {
			return ErrInvalidInput
		}
	} else {
		if _, ok := lazyopsAllowedMagicDomainProviders[provider]; !ok {
			return ErrInvalidInput
		}
	}

	return nil
}

func validateLazyopsService(svc LazyopsYAMLService) error {
	if strings.TrimSpace(svc.Name) == "" {
		return fmt.Errorf("service.name is required")
	}
	if !lazyopsLogicalNamePattern.MatchString(svc.Name) {
		return fmt.Errorf("service.name must contain only letters, digits, dots, underscores, or hyphens")
	}
	if err := validateLazyopsRepoRelativePath("service.path", svc.Path); err != nil {
		return err
	}

	return validateLazyopsHealthcheck(svc.Healthcheck)
}

func validateLazyopsHealthcheck(healthcheck LazyopsYAMLServiceHealthcheck) error {
	if strings.TrimSpace(healthcheck.Path) == "" && healthcheck.Port == 0 {
		return nil
	}
	if strings.TrimSpace(healthcheck.Path) == "" || healthcheck.Port == 0 {
		return fmt.Errorf("service.healthcheck requires both path and port once started")
	}
	if !strings.HasPrefix(healthcheck.Path, "/") {
		return fmt.Errorf("service.healthcheck.path must start with `/`")
	}
	if healthcheck.Port < 1 || healthcheck.Port > 65535 {
		return fmt.Errorf("service.healthcheck.port must be between 1 and 65535")
	}
	return nil
}

func validateLazyopsDependencyBinding(binding LazyopsYAMLDependencyBinding, knownServices map[string]struct{}, runtimeMode string) error {
	if strings.TrimSpace(binding.Service) == "" {
		return fmt.Errorf("service is required")
	}
	if _, exists := knownServices[binding.Service]; !exists {
		return fmt.Errorf("service %q is not declared in services", binding.Service)
	}
	if strings.TrimSpace(binding.Alias) == "" {
		return fmt.Errorf("alias is required")
	}
	if !lazyopsLogicalNamePattern.MatchString(binding.Alias) {
		return fmt.Errorf("alias must stay a logical dependency alias")
	}
	if strings.TrimSpace(binding.TargetService) == "" {
		return fmt.Errorf("target_service is required")
	}
	if !lazyopsLogicalNamePattern.MatchString(binding.TargetService) {
		return fmt.Errorf("target_service must stay a logical service name")
	}

	protocol := strings.ToLower(strings.TrimSpace(binding.Protocol))
	if protocol == "" {
		return fmt.Errorf("protocol is required")
	}
	if _, ok := lazyopsAllowedDependencyProtocols[protocol]; !ok {
		return fmt.Errorf("protocol %q is invalid. next: use http, https, tcp, or grpc", binding.Protocol)
	}

	localEndpoint := strings.TrimSpace(binding.LocalEndpoint)
	if localEndpoint != "" {
		if runtimeMode == "distributed-k3s" {
			return fmt.Errorf("distributed-k3s must not inject local dependency endpoints")
		}
		if err := validateLazyopsLocalEndpoint(localEndpoint); err != nil {
			return err
		}
	}

	return nil
}

func validateLazyopsRepoRelativePath(fieldPath, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is required", fieldPath)
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "~") {
		return fmt.Errorf("%s must stay repo-relative", fieldPath)
	}
	cleaned := path.Clean(trimmed)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%s must not escape the repository root", fieldPath)
	}
	return nil
}

func validateLazyopsLocalEndpoint(value string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("dependency_binding.local_endpoint must be a host:port pair")
	}
	if strings.Contains(host, "@") {
		return fmt.Errorf("dependency_binding.local_endpoint must not contain credentials")
	}

	normalizedHost := strings.Trim(host, "[]")
	switch normalizedHost {
	case "localhost", "127.0.0.1", "::1":
	default:
		return fmt.Errorf("dependency_binding.local_endpoint must stay local to the workstation")
	}

	if strings.TrimSpace(port) == "" {
		return fmt.Errorf("dependency_binding.local_endpoint port is required")
	}

	return nil
}

func validateLazyopsRawPayload(value any, fieldPath string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			if violation, forbidden := lazyopsForbiddenFieldReasons[normalizedKey]; forbidden {
				return fmt.Errorf("%w: %s.%s %s", violation.err, fieldPath, key, violation.reason)
			}
			if err := validateLazyopsRawPayload(nested, fieldPath+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for index, nested := range typed {
			if err := validateLazyopsRawPayload(nested, fmt.Sprintf("%s[%d]", fieldPath, index)); err != nil {
				return err
			}
		}
	case string:
		if err := validateLazyopsRawString(fieldPath, typed); err != nil {
			return err
		}
	}

	return nil
}

func validateLazyopsRawString(fieldPath, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if strings.HasSuffix(fieldPath, ".local_endpoint") {
		if err := validateLazyopsLocalEndpoint(trimmed); err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidDependencyMapping, err.Error())
		}
		return nil
	}

	lowerValue := strings.ToLower(trimmed)
	for _, marker := range lazyopsForbiddenMarkers {
		if strings.Contains(lowerValue, marker) {
			return fmt.Errorf("%w: %s must not contain secrets, kubeconfig material, or raw credentials", ErrSecretBearingConfig, fieldPath)
		}
	}

	if lazyopsIPv4Pattern.MatchString(trimmed) {
		return fmt.Errorf("%w: %s must stay logical and must not contain raw infrastructure IP addresses", ErrHardCodedDeployAuthority, fieldPath)
	}

	return nil
}

func lazyopsAllowedDependencyProtocolList() []string {
	return []string{"http", "https", "tcp", "grpc"}
}

func lazyopsForbiddenFieldNameList() []string {
	names := make([]string, 0, len(lazyopsForbiddenFieldReasons))
	for name := range lazyopsForbiddenFieldReasons {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func _lazyopsSchemaTypeLock(value any) error {
	typ := reflect.TypeOf(value)
	if typ == nil {
		return ErrInvalidInput
	}
	visited := map[reflect.Type]struct{}{}
	return validateLazyopsSchemaType(typ, typ.Name(), visited)
}

func validateLazyopsSchemaType(typ reflect.Type, fieldPath string, visited map[reflect.Type]struct{}) error {
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}
	if _, seen := visited[typ]; seen {
		return nil
	}
	visited[typ] = struct{}{}

	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if !field.IsExported() {
			continue
		}
		jsonName := lazyopsJSONFieldName(field)
		if jsonName == "" {
			continue
		}
		if violation, forbidden := lazyopsForbiddenFieldReasons[strings.ToLower(jsonName)]; forbidden {
			return fmt.Errorf("%w: schema lock violation at %s.%s because %s", violation.err, fieldPath, field.Name, violation.reason)
		}
		if err := validateLazyopsSchemaType(field.Type, fieldPath+"."+field.Name, visited); err != nil {
			return err
		}
	}

	return nil
}

func lazyopsJSONFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	if tag == "" {
		return field.Name
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		return field.Name
	}
	return name
}
