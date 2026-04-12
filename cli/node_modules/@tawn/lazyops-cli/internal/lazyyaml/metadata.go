package lazyyaml

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"lazyops-cli/internal/initplan"
)

type LinkMetadata struct {
	ProjectSlug string
	RuntimeMode initplan.RuntimeMode
	TargetRef   string
}

type DoctorMetadata struct {
	ProjectSlug        string
	RuntimeMode        initplan.RuntimeMode
	TargetRef          string
	Services           []DoctorService
	DependencyBindings []DoctorDependencyBinding
}

type DoctorService struct {
	Name string
	Path string
}

type DoctorDependencyBinding struct {
	Service       string
	Alias         string
	TargetService string
	Protocol      string
	LocalEndpoint string
}

func ReadLinkMetadata(repoRoot string) (LinkMetadata, error) {
	metadata, err := ReadDoctorMetadata(repoRoot)
	if err != nil {
		return LinkMetadata{}, err
	}
	if err := metadata.ValidateLinkFields(); err != nil {
		return LinkMetadata{}, err
	}

	return metadata.LinkMetadata(), nil
}

func ReadDoctorMetadata(repoRoot string) (DoctorMetadata, error) {
	path := DefaultPath(repoRoot)
	file, err := os.Open(path)
	if err != nil {
		return DoctorMetadata{}, err
	}
	defer file.Close()

	metadata := DoctorMetadata{
		Services:           []DoctorService{},
		DependencyBindings: []DoctorDependencyBinding{},
	}

	section := ""
	var currentService *DoctorService
	var currentDependency *DoctorDependencyBinding

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := countLeadingIndent(raw)
		if indent == 0 {
			if currentService != nil {
				metadata.Services = append(metadata.Services, *currentService)
				currentService = nil
			}
			if currentDependency != nil {
				metadata.DependencyBindings = append(metadata.DependencyBindings, *currentDependency)
				currentDependency = nil
			}

			section = ""
			switch {
			case strings.HasPrefix(trimmed, "project_slug:"):
				metadata.ProjectSlug = parseMetadataScalar(trimmed[len("project_slug:"):])
			case strings.HasPrefix(trimmed, "runtime_mode:"):
				mode, err := initplan.ParseRuntimeMode(parseMetadataScalar(trimmed[len("runtime_mode:"):]))
				if err != nil {
					return DoctorMetadata{}, err
				}
				metadata.RuntimeMode = mode
			case trimmed == "deployment_binding:":
				section = "deployment_binding"
			case trimmed == "services:":
				section = "services"
			case trimmed == "dependency_bindings:":
				section = "dependency_bindings"
			}
			continue
		}

		switch section {
		case "deployment_binding":
			if indent == 1 && strings.HasPrefix(trimmed, "target_ref:") {
				metadata.TargetRef = parseMetadataScalar(trimmed[len("target_ref:"):])
			}
		case "services":
			if indent == 1 && strings.HasPrefix(trimmed, "- ") {
				if currentService != nil {
					metadata.Services = append(metadata.Services, *currentService)
				}
				currentService = &DoctorService{}
				if value, ok := parseInlineMetadataField(strings.TrimSpace(trimmed[2:]), "name"); ok {
					currentService.Name = value
				}
				continue
			}
			if currentService != nil && indent == 2 {
				switch {
				case strings.HasPrefix(trimmed, "name:"):
					currentService.Name = parseMetadataScalar(trimmed[len("name:"):])
				case strings.HasPrefix(trimmed, "path:"):
					currentService.Path = parseMetadataScalar(trimmed[len("path:"):])
				}
			}
		case "dependency_bindings":
			if indent == 1 && strings.HasPrefix(trimmed, "- ") {
				if currentDependency != nil {
					metadata.DependencyBindings = append(metadata.DependencyBindings, *currentDependency)
				}
				currentDependency = &DoctorDependencyBinding{}
				if value, ok := parseInlineMetadataField(strings.TrimSpace(trimmed[2:]), "service"); ok {
					currentDependency.Service = value
				}
				continue
			}
			if currentDependency != nil && indent == 2 {
				switch {
				case strings.HasPrefix(trimmed, "service:"):
					currentDependency.Service = parseMetadataScalar(trimmed[len("service:"):])
				case strings.HasPrefix(trimmed, "alias:"):
					currentDependency.Alias = parseMetadataScalar(trimmed[len("alias:"):])
				case strings.HasPrefix(trimmed, "target_service:"):
					currentDependency.TargetService = parseMetadataScalar(trimmed[len("target_service:"):])
				case strings.HasPrefix(trimmed, "protocol:"):
					currentDependency.Protocol = parseMetadataScalar(trimmed[len("protocol:"):])
				case strings.HasPrefix(trimmed, "local_endpoint:"):
					currentDependency.LocalEndpoint = parseMetadataScalar(trimmed[len("local_endpoint:"):])
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return DoctorMetadata{}, fmt.Errorf("read lazyops.yaml metadata: %w", err)
	}

	if currentService != nil {
		metadata.Services = append(metadata.Services, *currentService)
	}
	if currentDependency != nil {
		metadata.DependencyBindings = append(metadata.DependencyBindings, *currentDependency)
	}

	return metadata, nil
}

func (metadata DoctorMetadata) LinkMetadata() LinkMetadata {
	return LinkMetadata{
		ProjectSlug: metadata.ProjectSlug,
		RuntimeMode: metadata.RuntimeMode,
		TargetRef:   metadata.TargetRef,
	}
}

func (metadata DoctorMetadata) ValidateLinkFields() error {
	if strings.TrimSpace(metadata.ProjectSlug) == "" {
		return fmt.Errorf("lazyops.yaml is missing project_slug")
	}
	if metadata.RuntimeMode == "" {
		return fmt.Errorf("lazyops.yaml is missing runtime_mode")
	}
	if strings.TrimSpace(metadata.TargetRef) == "" {
		return fmt.Errorf("lazyops.yaml is missing deployment_binding.target_ref")
	}

	return nil
}

func (metadata DoctorMetadata) ValidateDoctorContract() error {
	if err := metadata.ValidateLinkFields(); err != nil {
		return err
	}
	if len(metadata.Services) == 0 {
		return fmt.Errorf("lazyops.yaml services must include at least one service")
	}

	serviceNames := make(map[string]struct{}, len(metadata.Services))
	servicePaths := make(map[string]struct{}, len(metadata.Services))
	for index, service := range metadata.Services {
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

	return nil
}

func (metadata DoctorMetadata) ValidateDependencyDeclarations() error {
	if err := metadata.ValidateDoctorContract(); err != nil {
		return err
	}

	serviceNames := make(map[string]struct{}, len(metadata.Services))
	for _, service := range metadata.Services {
		serviceNames[service.Name] = struct{}{}
	}

	for index, binding := range metadata.DependencyBindings {
		if err := binding.Validate(serviceNames); err != nil {
			return fmt.Errorf("lazyops.yaml dependency_bindings[%d]: %w", index, err)
		}
	}

	return nil
}

func (service DoctorService) Validate() error {
	if strings.TrimSpace(service.Name) == "" {
		return fmt.Errorf("service.name is required")
	}
	if !logicalNamePattern.MatchString(service.Name) {
		return fmt.Errorf("service.name must contain only letters, digits, dots, underscores, or hyphens")
	}
	return validateRepoRelativePath("service.path", service.Path)
}

func (binding DoctorDependencyBinding) Validate(knownServices map[string]struct{}) error {
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

func countLeadingIndent(line string) int {
	width := 0
	for _, r := range line {
		switch r {
		case ' ':
			width++
		case '\t':
			width += 2
		default:
			return width / 2
		}
	}
	return width / 2
}

func parseInlineMetadataField(line string, key string) (string, bool) {
	prefix := key + ":"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	return parseMetadataScalar(line[len(prefix):]), true
}

func parseMetadataScalar(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') || (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') {
			return trimmed[1 : len(trimmed)-1]
		}
	}
	return trimmed
}
