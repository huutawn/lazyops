package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

type BlueprintService struct {
	projects   ProjectStore
	repoLinks  ProjectRepoLinkStore
	bindings   DeploymentBindingStore
	services   ProjectServiceStore
	blueprints BlueprintStore
}

func NewBlueprintService(
	projects ProjectStore,
	repoLinks ProjectRepoLinkStore,
	bindings DeploymentBindingStore,
	services ProjectServiceStore,
	blueprints BlueprintStore,
) *BlueprintService {
	return &BlueprintService{
		projects:   projects,
		repoLinks:  repoLinks,
		bindings:   bindings,
		services:   services,
		blueprints: blueprints,
	}
}

func (s *BlueprintService) Compile(cmd CompileBlueprintCommand) (*CompileBlueprintResult, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}

	repoLink, err := s.repoLinks.GetByProjectID(project.ID)
	if err != nil {
		return nil, err
	}
	if repoLink == nil {
		return nil, ErrRepoLinkNotFound
	}

	raw := bytes.TrimSpace(cmd.LazyopsYAMLRaw)
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
	if err := validateRoutingPolicy(document.RoutingPolicy, document.Services); err != nil {
		return nil, err
	}

	targetRef := normalizeBindingTargetRef(document.DeploymentBinding.TargetRef)
	binding, err := s.bindings.GetByTargetRefForProject(project.ID, targetRef)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, ErrUnknownTargetRef
	}

	runtimeMode, err := normalizeBindingRuntimeMode(document.RuntimeMode)
	if err != nil {
		return nil, err
	}
	if binding.RuntimeMode != runtimeMode {
		return nil, ErrRuntimeModeMismatch
	}

	bindingRecord, err := ToDeploymentBindingRecord(*binding)
	if err != nil {
		return nil, err
	}

	artifact, err := normalizeBlueprintArtifactMetadata(cmd.Artifact)
	if err != nil {
		return nil, err
	}

	serviceModels, _, serviceContracts, err := compileProjectServices(project.ID, document.Services)
	if err != nil {
		return nil, err
	}
	if err := s.services.ReplaceForProject(project.ID, serviceModels); err != nil {
		return nil, err
	}
	persistedServices, err := s.services.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}
	serviceRecords := make([]ProjectServiceRecord, 0, len(persistedServices))
	for _, item := range persistedServices {
		record, err := ToProjectServiceRecord(item)
		if err != nil {
			return nil, err
		}
		serviceRecords = append(serviceRecords, record)
	}

	repoState := toBlueprintRepoState(*repoLink)
	magicDomainPolicy := resolveBlueprintMagicDomainPolicy(document.MagicDomainPolicy, bindingRecord.DomainPolicy)
	dependencyBindings := copyDependencyBindings(document.DependencyBindings)

	compiled := BlueprintCompiledContractRecord{
		ProjectID:           project.ID,
		ProjectSlug:         project.Slug,
		Namespace:           firstNonEmptyCompiledValue(project.NamespaceSlug, normalizeProjectSlug(project.Slug)),
		RuntimeMode:         runtimeMode,
		Repo:                repoState,
		Binding:             bindingRecord,
		Services:            serviceContracts,
		DependencyBindings:  dependencyBindings,
		CompatibilityPolicy: document.CompatibilityPolicy,
		MagicDomainPolicy:   magicDomainPolicy,
		ScaleToZeroPolicy:   document.ScaleToZeroPolicy,
		RoutingPolicy:       document.RoutingPolicy,
		ArtifactMetadata:    artifact,
	}

	compiledJSON, err := json.Marshal(compiled)
	if err != nil {
		return nil, err
	}

	sourceRef, err := resolveBlueprintSourceRef(cmd.SourceRef, repoState)
	if err != nil {
		return nil, err
	}

	blueprint := &models.Blueprint{
		ID:           utils.NewPrefixedID("bp"),
		ProjectID:    project.ID,
		SourceKind:   "lazyops_yaml",
		SourceRef:    sourceRef,
		CompiledJSON: string(compiledJSON),
	}
	if err := s.blueprints.Create(blueprint); err != nil {
		return nil, err
	}

	blueprintRecord, err := ToBlueprintRecord(*blueprint)
	if err != nil {
		return nil, err
	}

	triggerKind, err := normalizeBlueprintTriggerKind(cmd.TriggerKind)
	if err != nil {
		return nil, err
	}

	return &CompileBlueprintResult{
		Services:  serviceRecords,
		Blueprint: blueprintRecord,
		DesiredRevisionDraft: DesiredStateRevisionDraftRecord{
			RevisionID:           utils.NewPrefixedID("rev"),
			ProjectID:            project.ID,
			BlueprintID:          blueprint.ID,
			DeploymentBindingID:  binding.ID,
			Namespace:            compiled.Namespace,
			CommitSHA:            artifact.CommitSHA,
			ArtifactRef:          artifact.ArtifactRef,
			ImageRef:             artifact.ImageRef,
			TriggerKind:          triggerKind,
			RuntimeMode:          runtimeMode,
			Services:             serviceContracts,
			ServiceSpecs:         buildK3sServiceSpecs(compiled.Namespace, serviceContracts),
			DependencyBindings:   dependencyBindings,
			InternalBindings:     buildInternalBindings(serviceContracts, dependencyBindings),
			CompatibilityPolicy:  document.CompatibilityPolicy,
			MagicDomainPolicy:    magicDomainPolicy,
			ScaleToZeroPolicy:    document.ScaleToZeroPolicy,
			RoutingPolicy:        document.RoutingPolicy,
			PlacementAssignments: buildPlacementAssignments(serviceContracts, bindingRecord),
		},
	}, nil
}

func compileProjectServices(projectID string, services []LazyopsYAMLService) ([]models.Service, []ProjectServiceRecord, []BlueprintServiceContractRecord, error) {
	items := make([]models.Service, 0, len(services))
	records := make([]ProjectServiceRecord, 0, len(services))
	contracts := make([]BlueprintServiceContractRecord, 0, len(services))

	for _, item := range services {
		healthcheck, err := toHealthcheckMap(item.Healthcheck)
		if err != nil {
			return nil, nil, nil, err
		}
		healthcheckJSON, err := marshalBindingPolicyJSON(healthcheck)
		if err != nil {
			return nil, nil, nil, err
		}

		runtimeProfile := inferRuntimeProfile(item)
		model := models.Service{
			ID:                 utils.NewPrefixedID("svc"),
			ProjectID:          projectID,
			Name:               strings.TrimSpace(item.Name),
			Path:               strings.TrimSpace(item.Path),
			Kind:               inferServiceKind(item),
			Public:             item.Public,
			StartHint:          strings.TrimSpace(item.StartHint),
			DetectedPortsJSON:  "[]",
			EnvBundleJSON:      "{}",
			PVCSpecJSON:        "{}",
			DeployStrategyJSON: "{}",
			Replicas:           1,
			HealthcheckJSON:    healthcheckJSON,
		}
		if runtimeProfile != "" {
			runtimeProfileCopy := runtimeProfile
			model.RuntimeProfile = &runtimeProfileCopy
		}

		record := ProjectServiceRecord{
			ID:             model.ID,
			ProjectID:      projectID,
			Name:           model.Name,
			Path:           model.Path,
			Kind:           model.Kind,
			SourceType:     serviceSourceTypeRepo,
			Public:         item.Public,
			RuntimeProfile: runtimeProfile,
			PlacementMode:  servicePlacementModeSharedCluster,
			StartHint:      model.StartHint,
			Replicas:       1,
			Healthcheck:    healthcheck,
		}
		contract := BlueprintServiceContractRecord{
			Name:           model.Name,
			Path:           model.Path,
			Kind:           model.Kind,
			SourceType:     serviceSourceTypeRepo,
			Public:         item.Public,
			RuntimeProfile: runtimeProfile,
			PlacementMode:  servicePlacementModeSharedCluster,
			StartHint:      strings.TrimSpace(item.StartHint),
			Replicas:       1,
			Healthcheck:    healthcheck,
		}

		items = append(items, model)
		records = append(records, record)
		contracts = append(contracts, contract)
	}

	return items, records, contracts, nil
}

func toHealthcheckMap(healthcheck LazyopsYAMLServiceHealthcheck) (map[string]any, error) {
	if err := validateLazyopsHealthcheck(healthcheck); err != nil {
		return nil, err
	}
	if strings.TrimSpace(healthcheck.Path) == "" && healthcheck.Port == 0 {
		return map[string]any{}, nil
	}

	return map[string]any{
		"path":     strings.TrimSpace(healthcheck.Path),
		"port":     healthcheck.Port,
		"protocol": "http",
	}, nil
}

func inferRuntimeProfile(item LazyopsYAMLService) string {
	if item.Public {
		return "web"
	}
	if strings.TrimSpace(item.Healthcheck.Path) != "" || item.Healthcheck.Port > 0 {
		return "service"
	}
	return "worker"
}

func inferServiceKind(item LazyopsYAMLService) string {
	name := strings.ToLower(strings.TrimSpace(item.Name))
	switch {
	case strings.Contains(name, "postgres"):
		return "postgres"
	case strings.Contains(name, "mysql"):
		return "mysql"
	case strings.Contains(name, "redis"):
		return "redis"
	case strings.Contains(name, "rabbit"):
		return "rabbitmq"
	default:
		return "app"
	}
}

func normalizeBlueprintArtifactMetadata(input BlueprintArtifactMetadata) (BlueprintArtifactMetadata, error) {
	commitSHA := strings.TrimSpace(input.CommitSHA)
	if commitSHA == "" || strings.ContainsAny(commitSHA, " \t\r\n") {
		return BlueprintArtifactMetadata{}, ErrInvalidInput
	}

	return BlueprintArtifactMetadata{
		CommitSHA:   commitSHA,
		ArtifactRef: strings.TrimSpace(input.ArtifactRef),
		ImageRef:    strings.TrimSpace(input.ImageRef),
	}, nil
}

func resolveBlueprintSourceRef(raw string, repo BlueprintRepoStateRecord) (string, error) {
	sourceRef := strings.TrimSpace(raw)
	if sourceRef == "" {
		sourceRef = strings.TrimSpace(repo.RepoFullName)
		if sourceRef != "" && strings.TrimSpace(repo.TrackedBranch) != "" {
			sourceRef += "@" + strings.TrimSpace(repo.TrackedBranch)
		}
	}
	if sourceRef == "" {
		return "", ErrInvalidInput
	}
	return sourceRef, nil
}

func normalizeBlueprintTriggerKind(raw string) (string, error) {
	triggerKind := strings.TrimSpace(raw)
	if triggerKind == "" {
		return "api_blueprint_compile", nil
	}
	if strings.ContainsAny(triggerKind, " \t\r\n") {
		return "", ErrInvalidInput
	}
	return triggerKind, nil
}

func toBlueprintRepoState(link models.ProjectRepoLink) BlueprintRepoStateRecord {
	return BlueprintRepoStateRecord{
		ProjectRepoLinkID: link.ID,
		RepoOwner:         link.RepoOwner,
		RepoName:          link.RepoName,
		RepoFullName:      link.RepoOwner + "/" + link.RepoName,
		TrackedBranch:     link.TrackedBranch,
		PreviewEnabled:    link.PreviewEnabled,
	}
}

func resolveBlueprintMagicDomainPolicy(docPolicy LazyopsYAMLMagicDomainPolicy, bindingDomainPolicy map[string]any) LazyopsYAMLMagicDomainPolicy {
	provider := strings.TrimSpace(strings.ToLower(docPolicy.Provider))
	if provider == "" {
		if candidate, ok := bindingDomainPolicy["magic_domain_provider"].(string); ok {
			provider = strings.TrimSpace(strings.ToLower(candidate))
		}
	}
	if provider == "" {
		provider = "sslip.io"
	}

	return LazyopsYAMLMagicDomainPolicy{
		Enabled:  docPolicy.Enabled,
		Provider: provider,
	}
}

func copyDependencyBindings(items []LazyopsYAMLDependencyBinding) []LazyopsYAMLDependencyBinding {
	out := make([]LazyopsYAMLDependencyBinding, 0, len(items))
	for _, item := range items {
		out = append(out, LazyopsYAMLDependencyBinding{
			Service:       item.Service,
			Alias:         item.Alias,
			TargetService: item.TargetService,
			Protocol:      item.Protocol,
			LocalEndpoint: item.LocalEndpoint,
		})
	}
	return out
}

func buildPlacementAssignments(services []BlueprintServiceContractRecord, binding DeploymentBindingRecord) []PlacementAssignmentRecord {
	assignments := make([]PlacementAssignmentRecord, 0, len(services))
	labels := toStringMap(binding.PlacementPolicy["labels"])
	for _, service := range services {
		targetKind := binding.TargetKind
		targetID := binding.TargetID
		if strings.TrimSpace(service.PlacementMode) == servicePlacementModePinnedNode && strings.TrimSpace(service.PlacementNodeID) != "" {
			targetKind = "instance"
			targetID = strings.TrimSpace(service.PlacementNodeID)
		}
		assignments = append(assignments, PlacementAssignmentRecord{
			ServiceName: service.Name,
			TargetID:    targetID,
			TargetKind:  targetKind,
			Labels:      labels,
		})
	}
	return assignments
}

func toStringMap(value any) map[string]string {
	out := map[string]string{}
	rawMap, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for key, raw := range rawMap {
		strValue, ok := raw.(string)
		if !ok {
			continue
		}
		out[key] = strValue
	}
	return out
}

func ToProjectServiceRecord(item models.Service) (ProjectServiceRecord, error) {
	healthcheck, err := decodeAnyMapJSON(item.HealthcheckJSON)
	if err != nil {
		return ProjectServiceRecord{}, err
	}
	detectedPorts := []ServiceDetectedPortRecord{}
	if err := unmarshalJSONWithFallback(item.DetectedPortsJSON, &detectedPorts, []ServiceDetectedPortRecord{}); err != nil {
		return ProjectServiceRecord{}, err
	}
	envBundle := map[string]string{}
	if err := unmarshalJSONWithFallback(item.EnvBundleJSON, &envBundle, map[string]string{}); err != nil {
		return ProjectServiceRecord{}, err
	}
	pvcSpec := map[string]any{}
	if err := unmarshalJSONWithFallback(item.PVCSpecJSON, &pvcSpec, map[string]any{}); err != nil {
		return ProjectServiceRecord{}, err
	}
	deployStrategy := map[string]any{}
	if err := unmarshalJSONWithFallback(item.DeployStrategyJSON, &deployStrategy, map[string]any{}); err != nil {
		return ProjectServiceRecord{}, err
	}
	sourceType, placementMode, placementNodeID, connectionTemplateKey, connectionTemplate, connectionTargetService, managedByLazyops := extractServiceContractMetadata(deployStrategy, item.Path)

	runtimeProfile := ""
	if item.RuntimeProfile != nil {
		runtimeProfile = *item.RuntimeProfile
	}

	return ProjectServiceRecord{
		ID:                      item.ID,
		ProjectID:               item.ProjectID,
		Name:                    item.Name,
		Path:                    item.Path,
		Kind:                    item.Kind,
		SourceType:              sourceType,
		Public:                  item.Public,
		RuntimeProfile:          runtimeProfile,
		PlacementMode:           placementMode,
		PlacementNodeID:         placementNodeID,
		ConnectionTemplateKey:   connectionTemplateKey,
		ConnectionTemplate:      connectionTemplate,
		ConnectionTargetService: connectionTargetService,
		ManagedByLazyops:        managedByLazyops,
		StartHint:               item.StartHint,
		ImageRef:                item.ImageRef,
		ImageDigest:             item.ImageDigest,
		DetectedPorts:           detectedPorts,
		TargetPort:              item.TargetPort,
		ServicePort:             item.ServicePort,
		Replicas:                item.Replicas,
		EnvBundle:               envBundle,
		PVCSpec:                 pvcSpec,
		DeployStrategy:          deployStrategy,
		Healthcheck:             healthcheck,
		CreatedAt:               item.CreatedAt,
		UpdatedAt:               item.UpdatedAt,
	}, nil
}

func extractServiceContractMetadata(deployStrategy map[string]any, path string) (string, string, string, string, map[string]string, string, bool) {
	sourceType := stringFromAny(deployStrategy[lazyopsServiceMetaSourceType])
	if sourceType == "" {
		if strings.HasPrefix(path, reservedManagedInternalServicePathPrefix) {
			sourceType = serviceSourceTypeInternal
		} else {
			sourceType = serviceSourceTypeRepo
		}
	}

	placementMode := stringFromAny(deployStrategy[lazyopsServiceMetaPlacementMode])
	if placementMode == "" {
		placementMode = servicePlacementModeSharedCluster
	}
	placementNodeID := stringFromAny(deployStrategy[lazyopsServiceMetaPlacementNodeID])
	connectionTemplateKey := stringFromAny(deployStrategy[lazyopsServiceMetaConnectionTemplateKey])
	connectionTemplate := map[string]string{}
	if sourceType == serviceSourceTypeInternal {
		trimmedPath := strings.TrimSpace(path)
		switch {
		case strings.HasPrefix(trimmedPath, reservedManagedInternalServicePathPrefix+"postgres"):
			connectionTemplate = coerceConnectionTemplateForKind("postgres", deployStrategy[lazyopsServiceMetaConnectionTemplate])
		case strings.HasPrefix(trimmedPath, reservedManagedInternalServicePathPrefix+"mysql"):
			connectionTemplate = coerceConnectionTemplateForKind("mysql", deployStrategy[lazyopsServiceMetaConnectionTemplate])
		}
	}
	connectionTargetService := stringFromAny(deployStrategy[lazyopsServiceMetaConnectionTargetService])
	managedByLazyops := boolFromAny(deployStrategy[lazyopsServiceMetaManagedByLazyops]) || sourceType == serviceSourceTypeInternal

	delete(deployStrategy, lazyopsServiceMetaSourceType)
	delete(deployStrategy, lazyopsServiceMetaPlacementMode)
	delete(deployStrategy, lazyopsServiceMetaPlacementNodeID)
	delete(deployStrategy, lazyopsServiceMetaConnectionTemplateKey)
	delete(deployStrategy, lazyopsServiceMetaConnectionTemplate)
	delete(deployStrategy, lazyopsServiceMetaConnectionTargetService)
	delete(deployStrategy, lazyopsServiceMetaManagedByLazyops)

	return sourceType, placementMode, placementNodeID, connectionTemplateKey, connectionTemplate, connectionTargetService, managedByLazyops
}

func stringFromAny(value any) string {
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(str)
}

func boolFromAny(value any) bool {
	v, ok := value.(bool)
	return ok && v
}

func unmarshalJSONWithFallback[T any](raw string, target *T, fallback T) error {
	if strings.TrimSpace(raw) == "" {
		*target = fallback
		return nil
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return err
	}
	return nil
}

func ToBlueprintRecord(item models.Blueprint) (BlueprintRecord, error) {
	var compiled BlueprintCompiledContractRecord
	if err := json.Unmarshal([]byte(item.CompiledJSON), &compiled); err != nil {
		return BlueprintRecord{}, err
	}

	return BlueprintRecord{
		ID:         item.ID,
		ProjectID:  item.ProjectID,
		SourceKind: item.SourceKind,
		SourceRef:  item.SourceRef,
		Compiled:   compiled,
		CreatedAt:  item.CreatedAt,
	}, nil
}

func validateRoutingPolicy(policy LazyopsYAMLRoutingPolicy, services []LazyopsYAMLService) error {
	if len(policy.Routes) == 0 {
		return nil
	}

	// Check all routes reference valid services
	serviceNames := make(map[string]bool)
	for _, svc := range services {
		serviceNames[svc.Name] = true
	}

	for _, route := range policy.Routes {
		if !serviceNames[route.Service] {
			return fmt.Errorf("route path %q references unknown service %q", route.Path, route.Service)
		}
	}

	// Check for overlapping path prefixes
	for i, r1 := range policy.Routes {
		for j, r2 := range policy.Routes {
			if i != j && r1.Path != "/" && r2.Path != "/" {
				if strings.HasPrefix(r1.Path, r2.Path) || strings.HasPrefix(r2.Path, r1.Path) {
					return fmt.Errorf("route path %q overlaps with %q", r1.Path, r2.Path)
				}
			}
		}
	}

	return nil
}
