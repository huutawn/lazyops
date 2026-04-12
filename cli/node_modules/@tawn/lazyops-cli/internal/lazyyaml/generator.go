package lazyyaml

import (
	"fmt"
	"strings"

	"lazyops-cli/internal/initplan"
)

type GenerateOptions struct {
	MagicDomainProvider string
	MagicDomainEnabled  *bool
	PreviewEnabled      *bool
	ScaleToZeroEnabled  *bool
}

func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		MagicDomainProvider: "sslip.io",
		MagicDomainEnabled:  boolPtr(true),
		PreviewEnabled:      boolPtr(true),
		ScaleToZeroEnabled:  boolPtr(false),
	}
}

func BuildDocument(plan initplan.InitPlan, options GenerateOptions) (Document, error) {
	if plan.SelectedProject == nil {
		return Document{}, fmt.Errorf("lazyops.yaml generation requires a selected project")
	}
	if plan.RuntimeMode == "" {
		return Document{}, fmt.Errorf("lazyops.yaml generation requires a selected runtime mode")
	}
	if plan.SelectedBinding == nil {
		return Document{}, fmt.Errorf("lazyops.yaml generation requires a selected deployment binding")
	}

	options = normalizeGenerateOptions(options)
	document := Document{
		ProjectSlug: plan.SelectedProject.Slug,
		RuntimeMode: plan.RuntimeMode,
		DeploymentBinding: DeploymentBindingRef{
			TargetRef: plan.SelectedBinding.TargetRef,
		},
		Services:            make([]Service, 0, len(plan.Services)),
		DependencyBindings:  make([]DependencyBinding, 0, len(plan.DependencyBindings)),
		CompatibilityPolicy: resolveCompatibilityPolicy(plan.CompatibilityPolicy),
		MagicDomainPolicy: MagicDomainPolicy{
			Enabled:  boolValue(options.MagicDomainEnabled, true),
			Provider: options.MagicDomainProvider,
		},
		PreviewPolicy: PreviewPolicy{
			Enabled: boolValue(options.PreviewEnabled, true),
		},
		ScaleToZeroPolicy: ScaleToZeroPolicy{
			Enabled: boolValue(options.ScaleToZeroEnabled, false),
		},
	}

	for _, candidate := range plan.Services {
		service := Service{
			Name:      candidate.Name,
			Path:      candidate.Path,
			StartHint: candidate.StartHint,
		}
		if strings.TrimSpace(candidate.Healthcheck.Path) != "" || candidate.Healthcheck.Port > 0 {
			service.Healthcheck = Healthcheck{
				Path: candidate.Healthcheck.Path,
				Port: candidate.Healthcheck.Port,
			}
		}
		document.Services = append(document.Services, service)
	}

	for _, binding := range plan.DependencyBindings {
		if strings.TrimSpace(binding.Service) == "" &&
			strings.TrimSpace(binding.Alias) == "" &&
			strings.TrimSpace(binding.TargetService) == "" &&
			strings.TrimSpace(binding.Protocol) == "" &&
			strings.TrimSpace(binding.LocalEndpoint) == "" {
			continue
		}
		document.DependencyBindings = append(document.DependencyBindings, DependencyBinding{
			Service:       binding.Service,
			Alias:         binding.Alias,
			TargetService: binding.TargetService,
			Protocol:      binding.Protocol,
			LocalEndpoint: binding.LocalEndpoint,
		})
	}

	if err := document.Validate(); err != nil {
		return Document{}, err
	}

	return document, nil
}

func Generate(plan initplan.InitPlan, options GenerateOptions) ([]byte, error) {
	document, err := BuildDocument(plan, options)
	if err != nil {
		return nil, err
	}
	return Render(document)
}

func Render(document Document) ([]byte, error) {
	if err := document.Validate(); err != nil {
		return nil, err
	}

	var builder strings.Builder
	writeLine(&builder, 0, "project_slug: %s", yamlScalar(document.ProjectSlug))
	writeLine(&builder, 0, "runtime_mode: %s", yamlScalar(string(document.RuntimeMode)))
	builder.WriteByte('\n')

	writeLine(&builder, 0, "deployment_binding:")
	writeLine(&builder, 1, "target_ref: %s", yamlScalar(document.DeploymentBinding.TargetRef))
	builder.WriteByte('\n')

	writeLine(&builder, 0, "services:")
	for _, service := range document.Services {
		writeLine(&builder, 1, "- name: %s", yamlScalar(service.Name))
		writeLine(&builder, 2, "path: %s", yamlScalar(service.Path))
		if strings.TrimSpace(service.StartHint) != "" {
			writeLine(&builder, 2, "start_hint: %s", yamlScalar(service.StartHint))
		}
		if service.Public {
			writeLine(&builder, 2, "public: true")
		}
		if strings.TrimSpace(service.Healthcheck.Path) != "" || service.Healthcheck.Port > 0 {
			writeLine(&builder, 2, "healthcheck:")
			writeLine(&builder, 3, "path: %s", yamlScalar(service.Healthcheck.Path))
			writeLine(&builder, 3, "port: %d", service.Healthcheck.Port)
		}
	}

	if len(document.DependencyBindings) > 0 {
		builder.WriteByte('\n')
		writeLine(&builder, 0, "dependency_bindings:")
		for _, binding := range document.DependencyBindings {
			writeLine(&builder, 1, "- service: %s", yamlScalar(binding.Service))
			writeLine(&builder, 2, "alias: %s", yamlScalar(binding.Alias))
			writeLine(&builder, 2, "target_service: %s", yamlScalar(binding.TargetService))
			writeLine(&builder, 2, "protocol: %s", yamlScalar(binding.Protocol))
			if strings.TrimSpace(binding.LocalEndpoint) != "" {
				writeLine(&builder, 2, "local_endpoint: %s", yamlScalar(binding.LocalEndpoint))
			}
		}
	}

	builder.WriteByte('\n')
	writeLine(&builder, 0, "compatibility_policy:")
	writeLine(&builder, 1, "env_injection: %t", document.CompatibilityPolicy.EnvInjection)
	writeLine(&builder, 1, "managed_credentials: %t", document.CompatibilityPolicy.ManagedCredentials)
	writeLine(&builder, 1, "localhost_rescue: %t", document.CompatibilityPolicy.LocalhostRescue)

	builder.WriteByte('\n')
	writeLine(&builder, 0, "magic_domain_policy:")
	writeLine(&builder, 1, "enabled: %t", document.MagicDomainPolicy.Enabled)
	writeLine(&builder, 1, "provider: %s", yamlScalar(document.MagicDomainPolicy.Provider))

	builder.WriteByte('\n')
	writeLine(&builder, 0, "preview_policy:")
	writeLine(&builder, 1, "enabled: %t", document.PreviewPolicy.Enabled)

	builder.WriteByte('\n')
	writeLine(&builder, 0, "scale_to_zero_policy:")
	writeLine(&builder, 1, "enabled: %t", document.ScaleToZeroPolicy.Enabled)

	return []byte(builder.String()), nil
}

func normalizeGenerateOptions(options GenerateOptions) GenerateOptions {
	defaults := DefaultGenerateOptions()
	if strings.TrimSpace(options.MagicDomainProvider) == "" {
		options.MagicDomainProvider = defaults.MagicDomainProvider
	}
	if options.MagicDomainEnabled == nil {
		options.MagicDomainEnabled = defaults.MagicDomainEnabled
	}
	if options.PreviewEnabled == nil {
		options.PreviewEnabled = defaults.PreviewEnabled
	}
	if options.ScaleToZeroEnabled == nil {
		options.ScaleToZeroEnabled = defaults.ScaleToZeroEnabled
	}
	return options
}

func resolveCompatibilityPolicy(policy initplan.CompatibilityPolicyDraft) CompatibilityPolicy {
	if !policy.EnvInjection && !policy.ManagedCredentials && !policy.LocalhostRescue {
		policy = initplan.DefaultCompatibilityPolicyDraft()
	}
	return CompatibilityPolicy{
		EnvInjection:       policy.EnvInjection,
		ManagedCredentials: policy.ManagedCredentials,
		LocalhostRescue:    policy.LocalhostRescue,
	}
}

func writeLine(builder *strings.Builder, indent int, format string, args ...any) {
	builder.WriteString(strings.Repeat("  ", indent))
	builder.WriteString(fmt.Sprintf(format, args...))
	builder.WriteByte('\n')
}

func yamlScalar(value string) string {
	if value == "" {
		return "''"
	}
	if isPlainYAMLScalar(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func isPlainYAMLScalar(value string) bool {
	for _, reserved := range []string{"true", "false", "null", "~"} {
		if strings.EqualFold(value, reserved) {
			return false
		}
	}
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-', r == '/':
		default:
			return false
		}
	}
	return true
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func boolPtr(value bool) *bool {
	return &value
}
