package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"sort"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const hiddenServiceInventoryBlueprintSourceKind = "service_inventory"

var ErrNoProjectServicesConfigured = errors.New("no project services configured")

type ServiceInventoryBlueprintCompiler struct {
	repoLinks  ProjectRepoLinkStore
	bindings   DeploymentBindingStore
	services   ProjectServiceStore
	blueprints BlueprintStore
	projectEnv *ProjectEnvService
}

type ServiceInventoryBlueprintCompileInput struct {
	Artifact    BlueprintArtifactMetadata
	TriggerKind string
	SourceRef   string
	ServiceIDs  []string
}

type ServiceInventoryBlueprintCompileResult struct {
	Blueprint       BlueprintRecord
	Services        []ProjectServiceRecord
	AppliedServices []string
}

func NewServiceInventoryBlueprintCompiler(
	repoLinks ProjectRepoLinkStore,
	bindings DeploymentBindingStore,
	services ProjectServiceStore,
	blueprints BlueprintStore,
) *ServiceInventoryBlueprintCompiler {
	return &ServiceInventoryBlueprintCompiler{
		repoLinks:  repoLinks,
		bindings:   bindings,
		services:   services,
		blueprints: blueprints,
	}
}

func (c *ServiceInventoryBlueprintCompiler) WithProjectEnvService(service *ProjectEnvService) *ServiceInventoryBlueprintCompiler {
	if c == nil {
		return c
	}
	c.projectEnv = service
	return c
}

func (c *ServiceInventoryBlueprintCompiler) Compile(project models.Project, input ServiceInventoryBlueprintCompileInput) (*ServiceInventoryBlueprintCompileResult, error) {
	if c == nil || c.services == nil || c.bindings == nil || c.blueprints == nil {
		return nil, ErrInvalidInput
	}

	binding, err := c.resolvePrimaryBinding(project.ID)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, ErrUnknownTargetRef
	}
	bindingRecord, err := ToDeploymentBindingRecord(*binding)
	if err != nil {
		return nil, err
	}

	repoState, sourceRef, err := c.resolveRepoState(project.ID, input.SourceRef)
	if err != nil {
		return nil, err
	}

	services, err := c.loadProjectServices(project.ID)
	if err != nil {
		return nil, err
	}
	services = filterServicesForDeployment(services, input.ServiceIDs)
	dependencyBindings := c.buildDependencyBindings(services, binding.RuntimeMode)
	serviceContracts, err := c.buildServiceContracts(services, dependencyBindings, binding.RuntimeMode, project.ID)
	if err != nil {
		return nil, err
	}
	appliedServices := []string{}
	artifact := normalizeHiddenBlueprintArtifact(input.Artifact, input.TriggerKind)

	compiled := BlueprintCompiledContractRecord{
		ProjectID:           project.ID,
		ProjectSlug:         project.Slug,
		Namespace:           firstNonEmptyCompiledValue(project.NamespaceSlug, normalizeProjectSlug(project.Slug)),
		RuntimeMode:         binding.RuntimeMode,
		Repo:                repoState,
		Binding:             bindingRecord,
		Services:            serviceContracts,
		DependencyBindings:  dependencyBindings,
		CompatibilityPolicy: c.resolveCompatibilityPolicy(bindingRecord, dependencyBindings),
		MagicDomainPolicy:   resolveBlueprintMagicDomainPolicy(LazyopsYAMLMagicDomainPolicy{}, bindingRecord.DomainPolicy),
		ScaleToZeroPolicy: LazyopsYAMLScaleToZeroPolicy{
			Enabled: boolFromPolicy(bindingRecord.ScaleToZeroPolicy, "enabled", false),
		},
		RoutingPolicy:    LazyopsYAMLRoutingPolicy{},
		ArtifactMetadata: artifact,
	}

	record := BlueprintRecord{
		ID:         utils.NewPrefixedID("bp"),
		ProjectID:  project.ID,
		SourceKind: hiddenServiceInventoryBlueprintSourceKind,
		SourceRef:  sourceRef,
		Compiled:   compiled,
	}
	if err := validateServiceArtifactsAgainstBlueprintServices(record.Compiled.Services, artifact.ServiceArtifacts); err != nil {
		return nil, err
	}
	if artifact.ImageRef != "" || artifact.ArtifactRef != "" || artifact.CommitSHA != "" || len(artifact.ServiceArtifacts) > 0 {
		appliedServices = applyArtifactToBlueprintServices(&record, BuildArtifactMetadataStageRecord{
			CommitSHA:        artifact.CommitSHA,
			ArtifactRef:      artifact.ArtifactRef,
			ImageRef:         artifact.ImageRef,
			ServiceArtifacts: cloneBuildServiceArtifacts(artifact.ServiceArtifacts),
		})
	}

	model, err := toBlueprintModel(record)
	if err != nil {
		return nil, err
	}
	if err := c.blueprints.Create(model); err != nil {
		return nil, err
	}
	persisted, err := ToBlueprintRecord(*model)
	if err != nil {
		return nil, err
	}
	return &ServiceInventoryBlueprintCompileResult{
		Blueprint:       persisted,
		Services:        services,
		AppliedServices: normalizeDetectedServices(appliedServices),
	}, nil
}

func filterServicesForDeployment(all []ProjectServiceRecord, selectedIDs []string) []ProjectServiceRecord {
	if len(all) == 0 {
		return all
	}
	if len(selectedIDs) == 0 {
		out := make([]ProjectServiceRecord, len(all))
		copy(out, all)
		return out
	}
	selected := make(map[string]struct{}, len(selectedIDs))
	for _, id := range selectedIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			selected[trimmed] = struct{}{}
		}
	}
	if len(selected) == 0 {
		out := make([]ProjectServiceRecord, len(all))
		copy(out, all)
		return out
	}

	byName := make(map[string]ProjectServiceRecord, len(all))
	filtered := make([]ProjectServiceRecord, 0, len(selected))
	for _, service := range all {
		byName[service.Name] = service
		if _, ok := selected[service.ID]; ok {
			filtered = append(filtered, service)
		}
	}
	addedByName := make(map[string]struct{}, len(filtered))
	for _, service := range filtered {
		addedByName[service.Name] = struct{}{}
	}
	for index := 0; index < len(filtered); index++ {
		service := filtered[index]
		for _, dependency := range service.Dependencies {
			targetName := strings.TrimSpace(dependency.TargetService)
			if targetName == "" {
				continue
			}
			target, ok := byName[targetName]
			if !ok || target.SourceType != serviceSourceTypeInternal {
				continue
			}
			if _, exists := addedByName[target.Name]; exists {
				continue
			}
			filtered = append(filtered, target)
			addedByName[target.Name] = struct{}{}
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].SourceType != filtered[j].SourceType {
			return filtered[i].SourceType < filtered[j].SourceType
		}
		if filtered[i].Name != filtered[j].Name {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Path < filtered[j].Path
	})
	return filtered
}

func (c *ServiceInventoryBlueprintCompiler) resolvePrimaryBinding(projectID string) (*models.DeploymentBinding, error) {
	autoBinding, err := c.bindings.GetByTargetRefForProject(projectID, "auto-primary")
	if err != nil {
		return nil, err
	}
	if autoBinding != nil {
		return autoBinding, nil
	}
	items, err := c.bindings.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	first := items[0]
	for _, item := range items[1:] {
		if item.CreatedAt.Before(first.CreatedAt) {
			first = item
		}
	}
	return &first, nil
}

func (c *ServiceInventoryBlueprintCompiler) resolveRepoState(projectID, rawSourceRef string) (BlueprintRepoStateRecord, string, error) {
	if c.repoLinks == nil {
		sourceRef := strings.TrimSpace(rawSourceRef)
		if sourceRef == "" {
			sourceRef = "service_inventory"
		}
		return BlueprintRepoStateRecord{}, sourceRef, nil
	}
	repoLink, err := c.repoLinks.GetByProjectID(projectID)
	if err != nil {
		return BlueprintRepoStateRecord{}, "", err
	}
	if repoLink == nil {
		sourceRef := strings.TrimSpace(rawSourceRef)
		if sourceRef == "" {
			sourceRef = "service_inventory"
		}
		return BlueprintRepoStateRecord{}, sourceRef, nil
	}
	repoState := toBlueprintRepoState(*repoLink)
	sourceRef := strings.TrimSpace(rawSourceRef)
	if sourceRef == "" {
		sourceRef = resolveOneClickSourceRef("", *repoLink)
	}
	return repoState, sourceRef, nil
}

func (c *ServiceInventoryBlueprintCompiler) loadProjectServices(projectID string) ([]ProjectServiceRecord, error) {
	items, err := c.services.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrNoProjectServicesConfigured
	}
	records := make([]ProjectServiceRecord, 0, len(items))
	for _, item := range items {
		record, err := ToProjectServiceRecord(item)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].SourceType != records[j].SourceType {
			return records[i].SourceType < records[j].SourceType
		}
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		return records[i].Path < records[j].Path
	})
	return records, nil
}

func (c *ServiceInventoryBlueprintCompiler) buildDependencyBindings(services []ProjectServiceRecord, runtimeMode string) []LazyopsYAMLDependencyBinding {
	out := make([]LazyopsYAMLDependencyBinding, 0)
	serviceIndex := make(map[string]ProjectServiceRecord, len(services))
	for _, service := range services {
		serviceIndex[service.Name] = service
	}
	for _, service := range services {
		for _, dependency := range service.Dependencies {
			targetService := strings.TrimSpace(dependency.TargetService)
			if targetService == "" {
				continue
			}
			target, ok := serviceIndex[targetService]
			if !ok || target.SourceType != serviceSourceTypeInternal {
				continue
			}
			protocol, localEndpoint := dependencyBindingProtocolAndEndpoint(target, runtimeMode)
			binding := LazyopsYAMLDependencyBinding{
				Service:       service.Name,
				Alias:         targetService,
				TargetService: targetService,
				Protocol:      protocol,
			}
			if localEndpoint != "" {
				binding.LocalEndpoint = localEndpoint
			}
			out = append(out, binding)
		}
	}
	return out
}

func (c *ServiceInventoryBlueprintCompiler) buildServiceContracts(services []ProjectServiceRecord, bindings []LazyopsYAMLDependencyBinding, runtimeMode, projectID string) ([]BlueprintServiceContractRecord, error) {
	projectEnv := map[string]string{}
	if c.projectEnv != nil {
		runtimeEnv, err := c.projectEnv.LoadRuntimeEnv(projectID)
		if err != nil {
			return nil, err
		}
		projectEnv = runtimeEnv
	}

	serviceIndex := make(map[string]ProjectServiceRecord, len(services))
	for _, service := range services {
		serviceIndex[service.Name] = service
	}
	relationalCountByService := make(map[string]int, len(services))
	dependencyKindCountByService := make(map[string]map[string]int, len(services))
	for _, service := range services {
		for _, dependency := range service.Dependencies {
			target, ok := serviceIndex[dependency.TargetService]
			if !ok {
				continue
			}
			kind := normalizeManagedInternalBridgeKind(target.Kind)
			if _, exists := dependencyKindCountByService[service.Name]; !exists {
				dependencyKindCountByService[service.Name] = make(map[string]int)
			}
			dependencyKindCountByService[service.Name][kind]++
			if isRelationalDatabaseKind(kind) {
				relationalCountByService[service.Name]++
			}
		}
	}

	out := make([]BlueprintServiceContractRecord, 0, len(services))
	for _, service := range services {
		envBundle := cloneStringMap(service.EnvBundle)
		for _, dependency := range service.Dependencies {
			target, ok := serviceIndex[strings.TrimSpace(dependency.TargetService)]
			if !ok || target.SourceType != serviceSourceTypeInternal {
				continue
			}
			kind := normalizeManagedInternalBridgeKind(target.Kind)
			for envName, value := range buildDependencyBindingEnv(
				dependency,
				target,
				projectEnv,
				runtimeMode,
				dependencyKindCountByService[service.Name][kind],
				relationalCountByService[service.Name],
			) {
				fillIfBlank(envBundle, envName, value)
			}
		}

		out = append(out, BlueprintServiceContractRecord{
			Name:                    service.Name,
			Path:                    service.Path,
			Kind:                    service.Kind,
			SourceType:              firstNonEmptyCompiledValue(service.SourceType, serviceSourceTypeRepo),
			Public:                  service.Public,
			RuntimeProfile:          service.RuntimeProfile,
			PlacementMode:           firstNonEmptyCompiledValue(service.PlacementMode, servicePlacementModeSharedCluster),
			PlacementNodeID:         service.PlacementNodeID,
			Dependencies:            cloneProjectServiceDependencyBindings(service.Dependencies),
			ConnectionTemplateKey:   service.ConnectionTemplateKey,
			ConnectionTemplate:      cloneStringMap(service.ConnectionTemplate),
			ConnectionTargetService: service.ConnectionTargetService,
			ManagedByLazyops:        service.ManagedByLazyops,
			StartHint:               service.StartHint,
			ImageRef:                service.ImageRef,
			ImageDigest:             service.ImageDigest,
			DetectedPorts:           cloneDetectedPorts(service.DetectedPorts),
			TargetPort:              service.TargetPort,
			ServicePort:             service.ServicePort,
			Replicas:                service.Replicas,
			EnvBundle:               envBundle,
			PVCSpec:                 cloneAnyMap(service.PVCSpec),
			DeployStrategy:          cloneAnyMap(service.DeployStrategy),
			Healthcheck:             cloneAnyMap(service.Healthcheck),
		})
	}
	return out, nil
}

func dependencyBindingProtocolAndEndpoint(target ProjectServiceRecord, runtimeMode string) (string, string) {
	kind := normalizeManagedInternalBridgeKind(target.Kind)
	protocol := "tcp"
	localPort := firstPositive(target.ServicePort, target.TargetPort, defaultConfiguredTargetPort(target.Kind))
	switch kind {
	case "postgres":
		protocol = "postgres"
	case "mysql":
		protocol = "mysql"
	case "mongodb":
		protocol = "mongodb"
	case "redis":
		protocol = "redis"
	case "kafka":
		protocol = "tcp"
	case "eureka-server":
		protocol = "http"
	}
	if strings.TrimSpace(runtimeMode) == "distributed-k3s" || localPort <= 0 {
		return protocol, ""
	}
	host := "localhost"
	if protocol == "http" {
		return protocol, "http://localhost:" + strconv.Itoa(localPort) + "/eureka"
	}
	return protocol, host + ":" + strconv.Itoa(localPort)
}

func (c *ServiceInventoryBlueprintCompiler) resolveCompatibilityPolicy(binding DeploymentBindingRecord, dependencyBindings []LazyopsYAMLDependencyBinding) LazyopsYAMLCompatibilityPolicy {
	defaultEnvInjection := true
	defaultLocalhostRescue := false
	if len(dependencyBindings) > 0 {
		defaultEnvInjection = false
		defaultLocalhostRescue = binding.RuntimeMode != "distributed-k3s"
	}
	return LazyopsYAMLCompatibilityPolicy{
		EnvInjection:       boolFromPolicy(binding.CompatibilityPolicy, "env_injection", defaultEnvInjection),
		ManagedCredentials: boolFromPolicy(binding.CompatibilityPolicy, "managed_credentials", false),
		LocalhostRescue:    boolFromPolicy(binding.CompatibilityPolicy, "localhost_rescue", defaultLocalhostRescue),
		TransparentProxy:   boolFromPolicy(binding.CompatibilityPolicy, "transparent_proxy", false),
	}
}

func normalizeHiddenBlueprintArtifact(input BlueprintArtifactMetadata, triggerKind string) BlueprintArtifactMetadata {
	artifact := input
	artifact.CommitSHA = strings.TrimSpace(artifact.CommitSHA)
	if artifact.CommitSHA == "" {
		prefix := strings.TrimSpace(triggerKind)
		if prefix == "" {
			prefix = "manual"
		}
		artifact.CommitSHA = prefix + "-" + time.Now().UTC().Format("20060102T150405Z")
	}
	artifact.ArtifactRef = strings.TrimSpace(artifact.ArtifactRef)
	artifact.ImageRef = strings.TrimSpace(artifact.ImageRef)
	artifact.ServiceArtifacts = normalizeBuildServiceArtifacts(artifact.ServiceArtifacts)
	return artifact
}

func toBlueprintModel(record BlueprintRecord) (*models.Blueprint, error) {
	compiledJSON, err := json.Marshal(record.Compiled)
	if err != nil {
		return nil, err
	}
	model := &models.Blueprint{
		ID:           record.ID,
		ProjectID:    record.ProjectID,
		SourceKind:   record.SourceKind,
		SourceRef:    record.SourceRef,
		CompiledJSON: string(compiledJSON),
		CreatedAt:    record.CreatedAt,
	}
	return model, nil
}

func fillIfBlank(target map[string]string, key, value string) {
	if target == nil {
		return
	}
	if existing, ok := target[key]; ok && strings.TrimSpace(existing) != "" {
		return
	}
	target[key] = value
}
