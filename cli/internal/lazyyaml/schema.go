package lazyyaml

import (
	"fmt"
	"net"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"lazyops-cli/internal/initplan"
)

type Document struct {
	ProjectSlug         string               `json:"project_slug"`
	RuntimeMode         initplan.RuntimeMode `json:"runtime_mode"`
	DeploymentBinding   DeploymentBindingRef `json:"deployment_binding"`
	Services            []Service            `json:"services"`
	DependencyBindings  []DependencyBinding  `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy CompatibilityPolicy  `json:"compatibility_policy"`
	MagicDomainPolicy   MagicDomainPolicy    `json:"magic_domain_policy,omitempty"`
	PreviewPolicy       PreviewPolicy        `json:"preview_policy,omitempty"`
	ScaleToZeroPolicy   ScaleToZeroPolicy    `json:"scale_to_zero_policy,omitempty"`
}

type DeploymentBindingRef struct {
	TargetRef string `json:"target_ref"`
}

type Service struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	StartHint   string      `json:"start_hint,omitempty"`
	Public      bool        `json:"public,omitempty"`
	Healthcheck Healthcheck `json:"healthcheck,omitempty"`
}

type Healthcheck struct {
	Path string `json:"path,omitempty"`
	Port int    `json:"port,omitempty"`
}

type DependencyBinding struct {
	Service       string `json:"service"`
	Alias         string `json:"alias"`
	TargetService string `json:"target_service"`
	Protocol      string `json:"protocol"`
	LocalEndpoint string `json:"local_endpoint,omitempty"`
}

type CompatibilityPolicy struct {
	EnvInjection       bool `json:"env_injection"`
	ManagedCredentials bool `json:"managed_credentials"`
	LocalhostRescue    bool `json:"localhost_rescue"`
}

type MagicDomainPolicy struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type PreviewPolicy struct {
	Enabled bool `json:"enabled,omitempty"`
}

type ScaleToZeroPolicy struct {
	Enabled bool `json:"enabled"`
}

var (
	projectSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	logicalNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	ipv4Pattern        = regexp.MustCompile(`\b(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\b`)
	forbiddenMarkers   = []string{
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
	allowedMagicDomainProviders = map[string]struct{}{
		"sslip.io": {},
		"nip.io":   {},
	}
	allowedDependencyProtocols = map[string]struct{}{
		"http":  {},
		"https": {},
		"tcp":   {},
		"grpc":  {},
	}
	forbiddenJSONFields = map[string]string{
		"ssh":                   "repo contracts must not carry SSH access",
		"ssh_key":               "repo contracts must not carry SSH access",
		"private_key":           "repo contracts must not carry private keys",
		"password":              "repo contracts must not carry passwords",
		"pat":                   "repo contracts must not carry PATs",
		"token":                 "repo contracts must not carry tokens",
		"agent_token":           "repo contracts must not carry agent credentials",
		"github_token":          "repo contracts must not carry GitHub credentials",
		"secret":                "repo contracts must not carry raw secrets",
		"kubeconfig":            "repo contracts must not carry kubeconfig data",
		"kubeconfig_secret_ref": "repo contracts must not carry kubeconfig refs",
		"public_ip":             "repo contracts must not carry raw target IPs",
		"private_ip":            "repo contracts must not carry raw target IPs",
		"server_ip":             "repo contracts must not carry raw target IPs",
		"project_id":            "repo contracts must stay logical and avoid backend ids",
		"deployment_binding_id": "repo contracts must stay logical and avoid backend ids",
		"target_id":             "repo contracts must stay logical and avoid concrete target ids",
		"target_kind":           "repo contracts must stay logical and avoid concrete target kinds",
		"instance_id":           "repo contracts must stay logical and avoid concrete target ids",
		"mesh_network_id":       "repo contracts must stay logical and avoid concrete target ids",
		"cluster_id":            "repo contracts must stay logical and avoid concrete target ids",
		"deploy_command":        "repo contracts must not bypass git-push deploy triggers",
	}
)

func (doc Document) Validate() error {
	if err := ValidateSchemaLock(); err != nil {
		return err
	}
	if strings.TrimSpace(doc.ProjectSlug) == "" {
		return fmt.Errorf("lazyops.yaml project_slug is required")
	}
	if !projectSlugPattern.MatchString(doc.ProjectSlug) {
		return fmt.Errorf("lazyops.yaml project_slug must be a stable slug like `acme-shop`")
	}
	if err := doc.RuntimeMode.Validate(); err != nil {
		return err
	}
	if doc.RuntimeMode == "" {
		return fmt.Errorf("lazyops.yaml runtime_mode is required")
	}
	if err := doc.DeploymentBinding.Validate(); err != nil {
		return err
	}
	if len(doc.Services) == 0 {
		return fmt.Errorf("lazyops.yaml services must include at least one service")
	}

	serviceNames := make(map[string]struct{}, len(doc.Services))
	servicePaths := make(map[string]struct{}, len(doc.Services))
	for index, service := range doc.Services {
		if err := service.Validate(); err != nil {
			return fmt.Errorf("lazyops.yaml services[%d]: %w", index, err)
		}
		if _, exists := serviceNames[service.Name]; exists {
			return fmt.Errorf("lazyops.yaml services[%d]: duplicate service name %q", index, service.Name)
		}
		if _, exists := servicePaths[service.Path]; exists {
			return fmt.Errorf("lazyops.yaml services[%d]: duplicate service path %q", index, service.Path)
		}
		serviceNames[service.Name] = struct{}{}
		servicePaths[service.Path] = struct{}{}
	}

	for index, binding := range doc.DependencyBindings {
		if err := binding.Validate(serviceNames); err != nil {
			return fmt.Errorf("lazyops.yaml dependency_bindings[%d]: %w", index, err)
		}
	}

	if err := doc.CompatibilityPolicy.Validate(); err != nil {
		return err
	}
	if err := doc.MagicDomainPolicy.Validate(); err != nil {
		return err
	}
	if err := validateNoForbiddenValues(reflect.ValueOf(doc), "lazyops.yaml"); err != nil {
		return err
	}

	return nil
}

func (binding DeploymentBindingRef) Validate() error {
	if strings.TrimSpace(binding.TargetRef) == "" {
		return fmt.Errorf("lazyops.yaml deployment_binding.target_ref is required")
	}
	if !logicalNamePattern.MatchString(binding.TargetRef) {
		return fmt.Errorf("lazyops.yaml deployment_binding.target_ref must stay a logical reference, not raw infrastructure data")
	}
	return nil
}

func (service Service) Validate() error {
	if strings.TrimSpace(service.Name) == "" {
		return fmt.Errorf("service.name is required")
	}
	if !logicalNamePattern.MatchString(service.Name) {
		return fmt.Errorf("service.name must contain only letters, digits, dots, underscores, or hyphens")
	}
	if err := validateRepoRelativePath("service.path", service.Path); err != nil {
		return err
	}
	return service.Healthcheck.Validate()
}

func (healthcheck Healthcheck) Validate() error {
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

func (binding DependencyBinding) Validate(knownServices map[string]struct{}) error {
	if strings.TrimSpace(binding.Service) == "" {
		return fmt.Errorf("service is required")
	}
	if _, exists := knownServices[binding.Service]; !exists {
		return fmt.Errorf("service %q is not declared in services", binding.Service)
	}
	if strings.TrimSpace(binding.Alias) == "" {
		return fmt.Errorf("alias is required")
	}
	if strings.TrimSpace(binding.TargetService) == "" {
		return fmt.Errorf("target_service is required")
	}
	if !logicalNamePattern.MatchString(binding.TargetService) {
		return fmt.Errorf("target_service must stay a logical service name")
	}
	protocol := strings.ToLower(strings.TrimSpace(binding.Protocol))
	if protocol == "" {
		return fmt.Errorf("protocol is required")
	}
	if _, ok := allowedDependencyProtocols[protocol]; !ok {
		return fmt.Errorf("protocol %q is invalid. next: use http, https, tcp, or grpc", binding.Protocol)
	}
	if strings.TrimSpace(binding.LocalEndpoint) != "" {
		if err := validateLocalEndpoint(binding.LocalEndpoint); err != nil {
			return err
		}
	}
	return nil
}

func (policy CompatibilityPolicy) Validate() error {
	if !policy.EnvInjection && !policy.ManagedCredentials && !policy.LocalhostRescue {
		return fmt.Errorf("lazyops.yaml compatibility_policy must keep at least one compatibility flag enabled")
	}
	return nil
}

func (policy MagicDomainPolicy) Validate() error {
	provider := strings.ToLower(strings.TrimSpace(policy.Provider))
	if provider == "" {
		if policy.Enabled {
			return fmt.Errorf("lazyops.yaml magic_domain_policy.provider is required when magic domains are enabled")
		}
		return nil
	}
	if _, ok := allowedMagicDomainProviders[provider]; !ok {
		return fmt.Errorf("lazyops.yaml magic_domain_policy.provider must be `sslip.io` or `nip.io`")
	}
	return nil
}

func ValidateSchemaLock() error {
	return ValidateSchemaTypeLock(Document{})
}

func ValidateSchemaTypeLock(value any) error {
	typ := reflect.TypeOf(value)
	if typ == nil {
		return fmt.Errorf("schema lock type must not be nil")
	}
	visited := map[reflect.Type]struct{}{}
	return validateSchemaType(typ, typ.Name(), visited)
}

func ForbiddenFieldNames() []string {
	names := make([]string, 0, len(forbiddenJSONFields))
	for name := range forbiddenJSONFields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func AllowedMagicDomainProviders() []string {
	return []string{"sslip.io", "nip.io"}
}

func validateSchemaType(typ reflect.Type, path string, visited map[reflect.Type]struct{}) error {
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
		jsonName := jsonFieldName(field)
		if jsonName == "" {
			continue
		}
		if reason, forbidden := forbiddenJSONFields[jsonName]; forbidden {
			return fmt.Errorf("schema lock violation at %s.%s: field %q is forbidden because %s", path, field.Name, jsonName, reason)
		}
		if err := validateSchemaType(field.Type, path+"."+field.Name, visited); err != nil {
			return err
		}
	}

	return nil
}

func validateNoForbiddenValues(value reflect.Value, path string) error {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		typ := value.Type()
		for index := 0; index < value.NumField(); index++ {
			field := typ.Field(index)
			if !field.IsExported() {
				continue
			}
			jsonName := jsonFieldName(field)
			if jsonName == "" {
				continue
			}
			if err := validateNoForbiddenValues(value.Field(index), path+"."+jsonName); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < value.Len(); index++ {
			if err := validateNoForbiddenValues(value.Index(index), fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case reflect.String:
		if err := validateStringValue(path, value.String()); err != nil {
			return err
		}
	}

	return nil
}

func validateStringValue(path string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if strings.HasSuffix(path, ".local_endpoint") {
		return validateLocalEndpoint(trimmed)
	}

	lowerValue := strings.ToLower(trimmed)
	for _, marker := range forbiddenMarkers {
		if strings.Contains(lowerValue, marker) {
			return fmt.Errorf("%s must not contain secrets, kubeconfig material, or raw credentials", path)
		}
	}

	if ipv4Pattern.MatchString(trimmed) {
		return fmt.Errorf("%s must stay logical and must not contain raw infrastructure IP addresses", path)
	}

	return nil
}

func validateRepoRelativePath(fieldPath string, value string) error {
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

func validateLocalEndpoint(value string) error {
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

func jsonFieldName(field reflect.StructField) string {
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
