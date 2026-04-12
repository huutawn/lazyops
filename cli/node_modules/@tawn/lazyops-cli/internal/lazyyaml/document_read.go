package lazyyaml

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"lazyops-cli/internal/initplan"
)

func ReadDocument(repoRoot string) (Document, error) {
	payload, err := os.ReadFile(DefaultPath(repoRoot))
	if err != nil {
		return Document{}, err
	}
	return ParseDocument(payload)
}

func ParseDocument(payload []byte) (Document, error) {
	if err := validatePayloadSecurity(payload); err != nil {
		return Document{}, err
	}

	document := Document{
		Services:           []Service{},
		DependencyBindings: []DependencyBinding{},
	}

	section := ""
	serviceSubsection := ""
	var currentService *Service
	var currentDependency *DependencyBinding

	scanner := bufio.NewScanner(bytes.NewReader(payload))
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := countLeadingIndent(raw)
		if indent == 0 {
			if currentService != nil {
				document.Services = append(document.Services, *currentService)
				currentService = nil
			}
			if currentDependency != nil {
				document.DependencyBindings = append(document.DependencyBindings, *currentDependency)
				currentDependency = nil
			}

			section = ""
			serviceSubsection = ""
			switch {
			case strings.HasPrefix(trimmed, "project_slug:"):
				document.ProjectSlug = parseMetadataScalar(trimmed[len("project_slug:"):])
			case strings.HasPrefix(trimmed, "runtime_mode:"):
				mode, err := initplan.ParseRuntimeMode(parseMetadataScalar(trimmed[len("runtime_mode:"):]))
				if err != nil {
					return Document{}, err
				}
				document.RuntimeMode = mode
			case trimmed == "deployment_binding:":
				section = "deployment_binding"
			case trimmed == "services:":
				section = "services"
			case trimmed == "dependency_bindings:":
				section = "dependency_bindings"
			case trimmed == "compatibility_policy:":
				section = "compatibility_policy"
			case trimmed == "magic_domain_policy:":
				section = "magic_domain_policy"
			case trimmed == "preview_policy:":
				section = "preview_policy"
			case trimmed == "scale_to_zero_policy:":
				section = "scale_to_zero_policy"
			}
			continue
		}

		switch section {
		case "deployment_binding":
			if indent == 1 && strings.HasPrefix(trimmed, "target_ref:") {
				document.DeploymentBinding.TargetRef = parseMetadataScalar(trimmed[len("target_ref:"):])
			}
		case "services":
			if indent == 1 && strings.HasPrefix(trimmed, "- ") {
				if currentService != nil {
					document.Services = append(document.Services, *currentService)
				}
				currentService = &Service{}
				serviceSubsection = ""
				parseServiceField(currentService, strings.TrimSpace(trimmed[2:]))
				continue
			}
			if currentService == nil {
				continue
			}
			if indent == 2 && trimmed == "healthcheck:" {
				serviceSubsection = "healthcheck"
				continue
			}
			if indent == 2 {
				serviceSubsection = ""
				parseServiceField(currentService, trimmed)
				continue
			}
			if indent == 3 && serviceSubsection == "healthcheck" {
				if err := parseHealthcheckField(&currentService.Healthcheck, trimmed); err != nil {
					return Document{}, err
				}
			}
		case "dependency_bindings":
			if indent == 1 && strings.HasPrefix(trimmed, "- ") {
				if currentDependency != nil {
					document.DependencyBindings = append(document.DependencyBindings, *currentDependency)
				}
				currentDependency = &DependencyBinding{}
				parseDependencyBindingField(currentDependency, strings.TrimSpace(trimmed[2:]))
				continue
			}
			if currentDependency != nil && indent == 2 {
				parseDependencyBindingField(currentDependency, trimmed)
			}
		case "compatibility_policy":
			if indent == 1 {
				if err := parseCompatibilityPolicyField(&document.CompatibilityPolicy, trimmed); err != nil {
					return Document{}, err
				}
			}
		case "magic_domain_policy":
			if indent == 1 {
				if err := parseMagicDomainPolicyField(&document.MagicDomainPolicy, trimmed); err != nil {
					return Document{}, err
				}
			}
		case "preview_policy":
			if indent == 1 {
				if err := parsePreviewPolicyField(&document.PreviewPolicy, trimmed); err != nil {
					return Document{}, err
				}
			}
		case "scale_to_zero_policy":
			if indent == 1 {
				if err := parseScaleToZeroPolicyField(&document.ScaleToZeroPolicy, trimmed); err != nil {
					return Document{}, err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Document{}, fmt.Errorf("read lazyops.yaml document: %w", err)
	}

	if currentService != nil {
		document.Services = append(document.Services, *currentService)
	}
	if currentDependency != nil {
		document.DependencyBindings = append(document.DependencyBindings, *currentDependency)
	}

	return document, nil
}

func parseServiceField(service *Service, line string) {
	switch {
	case strings.HasPrefix(line, "name:"):
		service.Name = parseMetadataScalar(line[len("name:"):])
	case strings.HasPrefix(line, "path:"):
		service.Path = parseMetadataScalar(line[len("path:"):])
	case strings.HasPrefix(line, "start_hint:"):
		service.StartHint = parseMetadataScalar(line[len("start_hint:"):])
	case strings.HasPrefix(line, "public:"):
		service.Public = parseMetadataBool(line[len("public:"):])
	}
}

func parseHealthcheckField(healthcheck *Healthcheck, line string) error {
	switch {
	case strings.HasPrefix(line, "path:"):
		healthcheck.Path = parseMetadataScalar(line[len("path:"):])
	case strings.HasPrefix(line, "port:"):
		value, err := parseMetadataInt(line[len("port:"):], "service.healthcheck.port")
		if err != nil {
			return err
		}
		healthcheck.Port = value
	}
	return nil
}

func parseDependencyBindingField(binding *DependencyBinding, line string) {
	switch {
	case strings.HasPrefix(line, "service:"):
		binding.Service = parseMetadataScalar(line[len("service:"):])
	case strings.HasPrefix(line, "alias:"):
		binding.Alias = parseMetadataScalar(line[len("alias:"):])
	case strings.HasPrefix(line, "target_service:"):
		binding.TargetService = parseMetadataScalar(line[len("target_service:"):])
	case strings.HasPrefix(line, "protocol:"):
		binding.Protocol = parseMetadataScalar(line[len("protocol:"):])
	case strings.HasPrefix(line, "local_endpoint:"):
		binding.LocalEndpoint = parseMetadataScalar(line[len("local_endpoint:"):])
	}
}

func parseCompatibilityPolicyField(policy *CompatibilityPolicy, line string) error {
	switch {
	case strings.HasPrefix(line, "env_injection:"):
		value, err := parseMetadataBoolStrict(line[len("env_injection:"):], "compatibility_policy.env_injection")
		if err != nil {
			return err
		}
		policy.EnvInjection = value
	case strings.HasPrefix(line, "managed_credentials:"):
		value, err := parseMetadataBoolStrict(line[len("managed_credentials:"):], "compatibility_policy.managed_credentials")
		if err != nil {
			return err
		}
		policy.ManagedCredentials = value
	case strings.HasPrefix(line, "localhost_rescue:"):
		value, err := parseMetadataBoolStrict(line[len("localhost_rescue:"):], "compatibility_policy.localhost_rescue")
		if err != nil {
			return err
		}
		policy.LocalhostRescue = value
	}
	return nil
}

func parseMagicDomainPolicyField(policy *MagicDomainPolicy, line string) error {
	switch {
	case strings.HasPrefix(line, "enabled:"):
		value, err := parseMetadataBoolStrict(line[len("enabled:"):], "magic_domain_policy.enabled")
		if err != nil {
			return err
		}
		policy.Enabled = value
	case strings.HasPrefix(line, "provider:"):
		policy.Provider = parseMetadataScalar(line[len("provider:"):])
	}
	return nil
}

func parsePreviewPolicyField(policy *PreviewPolicy, line string) error {
	if strings.HasPrefix(line, "enabled:") {
		value, err := parseMetadataBoolStrict(line[len("enabled:"):], "preview_policy.enabled")
		if err != nil {
			return err
		}
		policy.Enabled = value
	}
	return nil
}

func parseScaleToZeroPolicyField(policy *ScaleToZeroPolicy, line string) error {
	if strings.HasPrefix(line, "enabled:") {
		value, err := parseMetadataBoolStrict(line[len("enabled:"):], "scale_to_zero_policy.enabled")
		if err != nil {
			return err
		}
		policy.Enabled = value
	}
	return nil
}

func parseMetadataBool(value string) bool {
	parsed, _ := parseMetadataBoolStrict(value, "")
	return parsed
}

func parseMetadataBoolStrict(value string, field string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(parseMetadataScalar(value)))
	switch normalized {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "":
		if field == "" {
			return false, nil
		}
		return false, fmt.Errorf("%s must be true or false", field)
	default:
		if field == "" {
			return false, nil
		}
		return false, fmt.Errorf("%s must be true or false", field)
	}
}

func parseMetadataInt(value string, field string) (int, error) {
	normalized := strings.TrimSpace(parseMetadataScalar(value))
	parsed, err := strconv.Atoi(normalized)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", field)
	}
	return parsed, nil
}
