package initplan

import (
	"fmt"
	"strings"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/repo"
)

type RuntimeMode string

const (
	RuntimeModeStandalone      RuntimeMode = "standalone"
	RuntimeModeDistributedMesh RuntimeMode = "distributed-mesh"
	RuntimeModeDistributedK3s  RuntimeMode = "distributed-k3s"
)

type InitPlan struct {
	RepoRoot            string                   `json:"repo_root"`
	Layout              string                   `json:"layout"`
	Projects            []ProjectSummary         `json:"projects,omitempty"`
	SelectedProject     *ProjectSummary          `json:"selected_project,omitempty"`
	RuntimeMode         RuntimeMode              `json:"runtime_mode,omitempty"`
	Targets             []TargetSummary          `json:"targets,omitempty"`
	SelectedTarget      *TargetSummary           `json:"selected_target,omitempty"`
	Bindings            []BindingSummary         `json:"bindings,omitempty"`
	SelectedBinding     *BindingSummary          `json:"selected_binding,omitempty"`
	Services            []ServiceCandidate       `json:"services"`
	DependencyBindings  []DependencyBindingDraft `json:"dependency_bindings,omitempty"`
	CompatibilityPolicy CompatibilityPolicyDraft `json:"compatibility_policy"`
}

type ProjectSummary struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

type TargetSummary struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Status      string      `json:"status"`
	RuntimeMode RuntimeMode `json:"runtime_mode"`
}

type BindingSummary struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	TargetRef    string      `json:"target_ref"`
	RuntimeMode  RuntimeMode `json:"runtime_mode"`
	TargetKind   string      `json:"target_kind"`
	TargetID     string      `json:"target_id"`
	TargetStatus string      `json:"target_status,omitempty"`
	Reusable     bool        `json:"reusable,omitempty"`
}

type ServiceCandidate struct {
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	Signals     []string        `json:"signals"`
	StartHint   string          `json:"start_hint,omitempty"`
	Healthcheck HealthcheckHint `json:"healthcheck,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
}

type HealthcheckHint struct {
	Path string `json:"path,omitempty"`
	Port int    `json:"port,omitempty"`
}

type DependencyBindingDraft struct {
	Service       string `json:"service"`
	Alias         string `json:"alias"`
	TargetService string `json:"target_service"`
	Protocol      string `json:"protocol"`
	LocalEndpoint string `json:"local_endpoint,omitempty"`
}

type CompatibilityPolicyDraft struct {
	EnvInjection       bool `json:"env_injection"`
	ManagedCredentials bool `json:"managed_credentials"`
	LocalhostRescue    bool `json:"localhost_rescue"`
}

func Build(scanResult repo.RepoScanResult, detectionResult repo.DetectionResult) (InitPlan, error) {
	plan := InitPlan{
		RepoRoot:            scanResult.RepoRoot,
		Layout:              scanResult.LayoutLabel(),
		Services:            make([]ServiceCandidate, 0, len(detectionResult.Candidates)),
		DependencyBindings:  []DependencyBindingDraft{},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	for _, candidate := range detectionResult.Candidates {
		plan.Services = append(plan.Services, ServiceCandidate{
			Name:      candidate.Name,
			Path:      candidate.Path,
			Signals:   signalNames(candidate.Signals),
			StartHint: candidate.StartHint,
			Healthcheck: HealthcheckHint{
				Path: candidate.Healthcheck.Path,
				Port: candidate.Healthcheck.Port,
			},
			Warnings: append([]string(nil), candidate.Warnings...),
		})
	}

	if err := plan.Validate(); err != nil {
		return InitPlan{}, err
	}

	return plan, nil
}

type SelectionInput struct {
	Project     string
	RuntimeMode RuntimeMode
	Target      string
}

type BindingSelectionInput struct {
	Binding     string
	Create      bool
	BindingName string
}

func ParseRuntimeMode(value string) (RuntimeMode, error) {
	mode := RuntimeMode(strings.TrimSpace(value))
	if mode == "" {
		return "", nil
	}
	if err := mode.Validate(); err != nil {
		return "", err
	}
	return mode, nil
}

func ApplyDiscovery(
	plan InitPlan,
	projects []contracts.Project,
	instances []contracts.Instance,
	meshNetworks []contracts.MeshNetwork,
	clusters []contracts.Cluster,
	selection SelectionInput,
) (InitPlan, error) {
	plan.Projects = summarizeProjects(projects)
	plan.Targets = SummarizeTargets(instances, meshNetworks, clusters)

	if selection.RuntimeMode != "" {
		if err := selection.RuntimeMode.Validate(); err != nil {
			return InitPlan{}, err
		}
		plan.RuntimeMode = selection.RuntimeMode
		if plan.RuntimeMode == RuntimeModeDistributedMesh {
			plan.DependencyBindings = inferDependencyBindings(plan.Services)
		} else {
			plan.DependencyBindings = []DependencyBindingDraft{}
		}
	}

	selectedProject, err := pickProject(plan.Projects, selection.Project)
	if err != nil {
		return InitPlan{}, err
	}
	plan.SelectedProject = selectedProject

	if strings.TrimSpace(selection.Target) != "" && plan.RuntimeMode == "" {
		return InitPlan{}, fmt.Errorf("target selection requires a runtime mode first. next: rerun `lazyops init --runtime-mode <standalone|distributed-mesh|distributed-k3s> --target <id|name>`")
	}

	if plan.RuntimeMode != "" {
		eligibleTargets := eligibleTargetsForMode(plan.Targets, plan.RuntimeMode, plan.SelectedProject)
		if len(eligibleTargets) == 0 && strings.TrimSpace(selection.Target) == "" {
			return InitPlan{}, fmt.Errorf("no valid target exists for runtime mode %q. next: enroll a compatible target or choose a different runtime mode", plan.RuntimeMode)
		}

		selectedTarget, err := pickTarget(plan.Targets, plan.RuntimeMode, plan.SelectedProject, selection.Target)
		if err != nil {
			return InitPlan{}, err
		}
		plan.SelectedTarget = selectedTarget

		if len(eligibleTargets) == 0 && plan.SelectedTarget == nil {
			return InitPlan{}, fmt.Errorf("no valid target exists for runtime mode %q. next: enroll a compatible target or choose a different runtime mode", plan.RuntimeMode)
		}
	}

	if err := plan.Validate(); err != nil {
		return InitPlan{}, err
	}

	return plan, nil
}

func ApplyBindings(
	plan InitPlan,
	bindings []contracts.DeploymentBinding,
	selection BindingSelectionInput,
) (InitPlan, error) {
	plan.Bindings = SummarizeBindings(bindings)

	if plan.SelectedProject == nil {
		return plan, nil
	}

	filtered := plan.Bindings
	if plan.RuntimeMode != "" {
		filtered = FilterBindingsByRuntimeMode(filtered, plan.RuntimeMode)
	}
	if plan.SelectedTarget != nil {
		filtered = FilterBindingsByTarget(filtered, *plan.SelectedTarget)
	}

	if strings.TrimSpace(selection.Binding) != "" {
		binding, err := pickBinding(plan.Bindings, selection.Binding, plan.RuntimeMode, plan.SelectedTarget)
		if err != nil {
			return InitPlan{}, err
		}
		plan.SelectedBinding = binding
		return plan, plan.Validate()
	}

	if selection.Create {
		if plan.RuntimeMode == "" || plan.SelectedTarget == nil {
			return InitPlan{}, fmt.Errorf("binding creation requires a runtime mode and target first. next: rerun `lazyops init --runtime-mode <mode> --target <id|name> --create-binding`")
		}

		name := strings.TrimSpace(selection.BindingName)
		if name == "" {
			name = defaultBindingName(*plan.SelectedTarget)
		}
		plan.SelectedBinding = &BindingSummary{
			Name:        name,
			TargetRef:   plan.SelectedTarget.Name,
			RuntimeMode: plan.RuntimeMode,
			TargetKind:  plan.SelectedTarget.Kind,
			TargetID:    plan.SelectedTarget.ID,
		}
		return plan, plan.Validate()
	}

	if len(filtered) == 1 {
		selected := filtered[0]
		plan.SelectedBinding = &selected
		return plan, plan.Validate()
	}

	return plan, plan.Validate()
}

func DefaultCompatibilityPolicyDraft() CompatibilityPolicyDraft {
	return CompatibilityPolicyDraft{
		EnvInjection:       true,
		ManagedCredentials: true,
		LocalhostRescue:    true,
	}
}

func (plan InitPlan) Validate() error {
	if strings.TrimSpace(plan.RepoRoot) == "" {
		return fmt.Errorf("init plan repo root is required")
	}
	if strings.TrimSpace(plan.Layout) == "" {
		return fmt.Errorf("init plan layout is required")
	}

	for _, project := range plan.Projects {
		if err := project.Validate(); err != nil {
			return err
		}
	}
	if plan.SelectedProject != nil {
		if err := plan.SelectedProject.Validate(); err != nil {
			return err
		}
	}
	if plan.RuntimeMode != "" {
		if err := plan.RuntimeMode.Validate(); err != nil {
			return err
		}
	}
	for _, target := range plan.Targets {
		if err := target.Validate(); err != nil {
			return err
		}
	}
	for _, binding := range plan.Bindings {
		if err := binding.Validate(); err != nil {
			return err
		}
	}
	if plan.SelectedTarget != nil {
		if err := plan.SelectedTarget.Validate(); err != nil {
			return err
		}
		if plan.RuntimeMode == "" {
			return fmt.Errorf("selected target requires a runtime mode")
		}
		if plan.SelectedTarget.RuntimeMode != plan.RuntimeMode {
			return fmt.Errorf("selected target %q is incompatible with runtime mode %q", plan.SelectedTarget.Name, plan.RuntimeMode)
		}
	}
	if plan.SelectedBinding != nil {
		if err := plan.SelectedBinding.Validate(); err != nil {
			return err
		}
		if plan.RuntimeMode != "" && plan.SelectedBinding.RuntimeMode != plan.RuntimeMode {
			return fmt.Errorf("selected binding %q is incompatible with runtime mode %q", plan.SelectedBinding.Name, plan.RuntimeMode)
		}
		if plan.SelectedTarget != nil && (plan.SelectedBinding.TargetKind != plan.SelectedTarget.Kind || plan.SelectedBinding.TargetID != plan.SelectedTarget.ID) {
			return fmt.Errorf("selected binding %q does not match target %q", plan.SelectedBinding.Name, plan.SelectedTarget.Name)
		}
	}

	names := map[string]struct{}{}
	paths := map[string]struct{}{}
	for _, service := range plan.Services {
		if err := service.Validate(); err != nil {
			return err
		}
		if _, exists := names[service.Name]; exists {
			return fmt.Errorf("init plan has duplicate service name %q", service.Name)
		}
		names[service.Name] = struct{}{}
		if _, exists := paths[service.Path]; exists {
			return fmt.Errorf("init plan has duplicate service path %q", service.Path)
		}
		paths[service.Path] = struct{}{}
	}

	for _, binding := range plan.DependencyBindings {
		if err := binding.Validate(); err != nil {
			return err
		}
		if _, exists := names[binding.Service]; !exists {
			return fmt.Errorf("dependency binding service %q is not declared in init plan services", binding.Service)
		}
		if _, exists := names[binding.TargetService]; !exists {
			return fmt.Errorf("dependency binding target_service %q is not declared in init plan services", binding.TargetService)
		}
		if plan.RuntimeMode == RuntimeModeDistributedK3s && strings.TrimSpace(binding.LocalEndpoint) != "" {
			return fmt.Errorf("distributed-k3s init must not inject local dependency endpoints; K3s scheduling must stay cluster-native")
		}
	}

	return plan.CompatibilityPolicy.Validate()
}

func (project ProjectSummary) Validate() error {
	if strings.TrimSpace(project.ID) == "" {
		return fmt.Errorf("project summary id is required")
	}
	if strings.TrimSpace(project.Name) == "" {
		return fmt.Errorf("project summary name is required")
	}
	if strings.TrimSpace(project.Slug) == "" {
		return fmt.Errorf("project summary slug is required")
	}
	return nil
}

func (mode RuntimeMode) Validate() error {
	switch mode {
	case RuntimeModeStandalone, RuntimeModeDistributedMesh, RuntimeModeDistributedK3s:
		return nil
	case "":
		return nil
	default:
		return fmt.Errorf("runtime mode %q is invalid. next: use standalone, distributed-mesh, or distributed-k3s", mode)
	}
}

func (target TargetSummary) Validate() error {
	if strings.TrimSpace(target.ID) == "" {
		return fmt.Errorf("target summary id is required")
	}
	if strings.TrimSpace(target.Name) == "" {
		return fmt.Errorf("target summary name is required")
	}
	if strings.TrimSpace(target.Kind) == "" {
		return fmt.Errorf("target summary kind is required")
	}
	if strings.TrimSpace(target.Status) == "" {
		return fmt.Errorf("target summary status is required")
	}
	return target.RuntimeMode.Validate()
}

func (binding BindingSummary) Validate() error {
	if strings.TrimSpace(binding.Name) == "" {
		return fmt.Errorf("binding summary name is required")
	}
	if strings.TrimSpace(binding.TargetRef) == "" {
		return fmt.Errorf("binding summary target_ref is required")
	}
	if strings.TrimSpace(binding.TargetKind) == "" {
		return fmt.Errorf("binding summary target_kind is required")
	}
	if strings.TrimSpace(binding.TargetID) == "" {
		return fmt.Errorf("binding summary target_id is required")
	}
	return binding.RuntimeMode.Validate()
}

func (candidate ServiceCandidate) Validate() error {
	if strings.TrimSpace(candidate.Name) == "" {
		return fmt.Errorf("service candidate name is required")
	}
	if strings.TrimSpace(candidate.Path) == "" {
		return fmt.Errorf("service candidate path is required")
	}
	return nil
}

func (binding DependencyBindingDraft) Validate() error {
	if strings.TrimSpace(binding.Service) == "" &&
		strings.TrimSpace(binding.Alias) == "" &&
		strings.TrimSpace(binding.TargetService) == "" &&
		strings.TrimSpace(binding.Protocol) == "" &&
		strings.TrimSpace(binding.LocalEndpoint) == "" {
		return nil
	}
	if strings.TrimSpace(binding.Service) == "" {
		return fmt.Errorf("dependency binding service is required")
	}
	if strings.TrimSpace(binding.Alias) == "" {
		return fmt.Errorf("dependency binding alias is required")
	}
	if strings.TrimSpace(binding.TargetService) == "" {
		return fmt.Errorf("dependency binding target_service is required")
	}
	if strings.TrimSpace(binding.Protocol) == "" {
		return fmt.Errorf("dependency binding protocol is required")
	}
	return nil
}

func (policy CompatibilityPolicyDraft) Validate() error {
	if !policy.EnvInjection && !policy.ManagedCredentials && !policy.LocalhostRescue {
		return fmt.Errorf("compatibility policy must keep at least one compatibility flag enabled")
	}
	return nil
}

func signalNames(signals []repo.ServiceSignal) []string {
	names := make([]string, 0, len(signals))
	for _, signal := range signals {
		names = append(names, string(signal))
	}
	return names
}

func (plan InitPlan) TargetsForMode(mode RuntimeMode) []TargetSummary {
	targets := make([]TargetSummary, 0, len(plan.Targets))
	for _, target := range plan.Targets {
		if target.RuntimeMode != mode {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func summarizeProjects(projects []contracts.Project) []ProjectSummary {
	summaries := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		summaries = append(summaries, ProjectSummary{
			ID:            project.ID,
			Slug:          project.Slug,
			Name:          project.Name,
			DefaultBranch: project.DefaultBranch,
		})
	}
	return summaries
}

func SummarizeTargets(instances []contracts.Instance, meshNetworks []contracts.MeshNetwork, clusters []contracts.Cluster) []TargetSummary {
	targets := make([]TargetSummary, 0, len(instances)+len(meshNetworks)+len(clusters))

	for _, instance := range instances {
		targets = append(targets, TargetSummary{
			ID:          instance.ID,
			Name:        instance.Name,
			Kind:        "instance",
			Status:      instance.Status,
			RuntimeMode: RuntimeModeStandalone,
		})
	}
	for _, network := range meshNetworks {
		targets = append(targets, TargetSummary{
			ID:          network.ID,
			Name:        network.Name,
			Kind:        "mesh",
			Status:      network.Status,
			RuntimeMode: RuntimeModeDistributedMesh,
		})
	}
	for _, cluster := range clusters {
		targets = append(targets, TargetSummary{
			ID:          cluster.ID,
			Name:        cluster.Name,
			Kind:        "cluster",
			Status:      cluster.Status,
			RuntimeMode: RuntimeModeDistributedK3s,
		})
	}

	return targets
}

func SummarizeBindings(bindings []contracts.DeploymentBinding) []BindingSummary {
	summaries := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		summaries = append(summaries, BindingSummary{
			ID:          binding.ID,
			Name:        binding.Name,
			TargetRef:   binding.TargetRef,
			RuntimeMode: RuntimeMode(binding.RuntimeMode),
			TargetKind:  binding.TargetKind,
			TargetID:    binding.TargetID,
		})
	}
	return summaries
}

func AnnotateBindingsWithTargets(bindings []BindingSummary, targets []TargetSummary, project *ProjectSummary) []BindingSummary {
	indexedTargets := make(map[string]TargetSummary, len(targets))
	for _, target := range targets {
		indexedTargets[bindingTargetKey(target.Kind, target.ID)] = target
	}

	annotated := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		enriched := binding
		target, ok := indexedTargets[bindingTargetKey(binding.TargetKind, binding.TargetID)]
		if !ok {
			enriched.TargetStatus = "missing"
			enriched.Reusable = false
			annotated = append(annotated, enriched)
			continue
		}

		enriched.TargetStatus = target.Status
		enriched.Reusable, _ = targetSelectableForProject(target, project)
		annotated = append(annotated, enriched)
	}

	return annotated
}

func FilterBindingsByTargetRef(bindings []BindingSummary, targetRef string) []BindingSummary {
	normalized := strings.TrimSpace(targetRef)
	if normalized == "" {
		return append([]BindingSummary(nil), bindings...)
	}

	filtered := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		if binding.TargetRef != normalized {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}

func FilterBindingsByTargetKind(bindings []BindingSummary, targetKind string) []BindingSummary {
	normalized := strings.TrimSpace(targetKind)
	if normalized == "" {
		return append([]BindingSummary(nil), bindings...)
	}

	filtered := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		if binding.TargetKind != normalized {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}

func FilterBindingsByStatus(bindings []BindingSummary, status string) []BindingSummary {
	normalized := strings.TrimSpace(status)
	if normalized == "" {
		return append([]BindingSummary(nil), bindings...)
	}

	filtered := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		if !strings.EqualFold(binding.TargetStatus, normalized) {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}

func ReusableBindings(bindings []BindingSummary) []BindingSummary {
	filtered := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		if !binding.Reusable {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}

func pickProject(projects []ProjectSummary, selector string) (*ProjectSummary, error) {
	selector = strings.TrimSpace(selector)
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects are available for init. next: create a project or verify CLI auth")
	}
	if selector == "" {
		if len(projects) == 1 {
			project := projects[0]
			return &project, nil
		}
		return nil, nil
	}

	for _, project := range projects {
		if selector == project.ID || selector == project.Slug || selector == project.Name {
			selected := project
			return &selected, nil
		}
	}

	return nil, fmt.Errorf("project %q was not found. next: rerun `lazyops init --project <project-id-or-slug>` with one of the listed projects", selector)
}

func pickTarget(targets []TargetSummary, mode RuntimeMode, project *ProjectSummary, selector string) (*TargetSummary, error) {
	compatible := make([]TargetSummary, 0, len(targets))
	incompatible := make([]TargetSummary, 0, len(targets))
	for _, target := range targets {
		if target.RuntimeMode == mode {
			compatible = append(compatible, target)
		} else {
			incompatible = append(incompatible, target)
		}
	}

	selector = strings.TrimSpace(selector)
	if selector == "" {
		eligible := eligibleTargetsForMode(compatible, mode, project)
		if len(eligible) == 1 {
			target := eligible[0]
			return &target, nil
		}
		return nil, nil
	}

	for _, target := range compatible {
		if selector == target.ID || selector == target.Name {
			if ok, reason := targetSelectableForProject(target, project); !ok {
				return nil, invalidTargetSelectionError(target, reason)
			}
			selected := target
			return &selected, nil
		}
	}
	for _, target := range incompatible {
		if selector == target.ID || selector == target.Name {
			return nil, fmt.Errorf("target %q is incompatible with runtime mode %q. next: pick a %s target or change `--runtime-mode`", selector, mode, mode)
		}
	}

	return nil, fmt.Errorf("target %q was not found for runtime mode %q. next: rerun `lazyops init --runtime-mode %s --target <id|name>` with a listed target", selector, mode, mode)
}

func eligibleTargetsForMode(targets []TargetSummary, mode RuntimeMode, project *ProjectSummary) []TargetSummary {
	eligible := make([]TargetSummary, 0, len(targets))
	for _, target := range targets {
		if target.RuntimeMode != mode {
			continue
		}
		if ok, _ := targetSelectableForProject(target, project); !ok {
			continue
		}
		eligible = append(eligible, target)
	}
	return eligible
}

func targetSelectableForProject(target TargetSummary, project *ProjectSummary) (bool, string) {
	switch target.RuntimeMode {
	case RuntimeModeStandalone, RuntimeModeDistributedMesh:
		if !strings.EqualFold(target.Status, "online") {
			return false, "offline"
		}
	case RuntimeModeDistributedK3s:
		if !strings.EqualFold(target.Status, "registered") &&
			!strings.EqualFold(target.Status, "online") &&
			!strings.EqualFold(target.Status, "available") {
			return false, "unavailable"
		}
	}

	return true, ""
}

func invalidTargetSelectionError(target TargetSummary, reason string) error {
	switch reason {
	case "ownership":
		return fmt.Errorf("%s target %q is not owned by the selected project user. next: choose a %s target that belongs to the project owner or switch `--project`", target.Kind, target.Name, target.Kind)
	case "offline":
		return fmt.Errorf("%s target %q is not currently online. next: bring the target online or choose a different %s target", target.Kind, target.Name, target.Kind)
	case "unavailable":
		return fmt.Errorf("%s target %q is not currently available. next: wait for the target to register or choose a different %s target", target.Kind, target.Name, target.Kind)
	default:
		return fmt.Errorf("%s target %q is not selectable. next: choose a different target", target.Kind, target.Name)
	}
}

func pickBinding(bindings []BindingSummary, selector string, mode RuntimeMode, target *TargetSummary) (*BindingSummary, error) {
	selector = strings.TrimSpace(selector)
	for _, binding := range bindings {
		if selector != binding.ID && selector != binding.Name && selector != binding.TargetRef {
			continue
		}
		if mode != "" && binding.RuntimeMode != mode {
			return nil, fmt.Errorf("binding %q is incompatible with runtime mode %q. next: pick a binding for %s or change `--runtime-mode`", selector, mode, mode)
		}
		if target != nil && (binding.TargetKind != target.Kind || binding.TargetID != target.ID) {
			return nil, fmt.Errorf("binding %q does not match target %q. next: pick a binding for the selected target or change `--target`", selector, target.Name)
		}
		selected := binding
		return &selected, nil
	}
	return nil, fmt.Errorf("binding %q was not found. next: rerun `lazyops init --binding <binding-id|name|target-ref>` with one of the listed bindings", selector)
}

func FilterBindingsByRuntimeMode(bindings []BindingSummary, mode RuntimeMode) []BindingSummary {
	filtered := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		if binding.RuntimeMode != mode {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}

func FilterBindingsByTarget(bindings []BindingSummary, target TargetSummary) []BindingSummary {
	filtered := make([]BindingSummary, 0, len(bindings))
	for _, binding := range bindings {
		if binding.TargetKind != target.Kind || binding.TargetID != target.ID {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}

func bindingTargetKey(kind string, id string) string {
	return strings.TrimSpace(kind) + "::" + strings.TrimSpace(id)
}

func defaultBindingName(target TargetSummary) string {
	suffix := map[RuntimeMode]string{
		RuntimeModeStandalone:      "standalone",
		RuntimeModeDistributedMesh: "mesh",
		RuntimeModeDistributedK3s:  "k3s",
	}

	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(target.Name)), " ", "-")
	if normalized == "" {
		normalized = "binding"
	}
	return normalized + "-" + suffix[target.RuntimeMode]
}

func inferDependencyBindings(services []ServiceCandidate) []DependencyBindingDraft {
	if len(services) < 2 {
		return []DependencyBindingDraft{}
	}

	byName := map[string]ServiceCandidate{}
	for _, service := range services {
		byName[strings.ToLower(strings.TrimSpace(service.Name))] = service
	}

	api, ok := byName["api"]
	if !ok {
		return []DependencyBindingDraft{}
	}

	drafts := make([]DependencyBindingDraft, 0, 2)
	for _, sourceName := range []string{"web", "gateway"} {
		source, exists := byName[sourceName]
		if !exists {
			continue
		}
		drafts = append(drafts, DependencyBindingDraft{
			Service:       source.Name,
			Alias:         "api",
			TargetService: api.Name,
			Protocol:      "http",
			LocalEndpoint: fmt.Sprintf("localhost:%d", inferredHTTPPort(api)),
		})
	}

	return drafts
}

func inferredHTTPPort(service ServiceCandidate) int {
	if service.Healthcheck.Port > 0 {
		return service.Healthcheck.Port
	}
	return 8080
}
