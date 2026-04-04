package service

import (
	"bytes"
	"encoding/json"
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
		RuntimeMode:         runtimeMode,
		Repo:                repoState,
		Binding:             bindingRecord,
		Services:            serviceContracts,
		DependencyBindings:  dependencyBindings,
		CompatibilityPolicy: document.CompatibilityPolicy,
		MagicDomainPolicy:   magicDomainPolicy,
		ScaleToZeroPolicy:   document.ScaleToZeroPolicy,
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
			CommitSHA:            artifact.CommitSHA,
			ArtifactRef:          artifact.ArtifactRef,
			ImageRef:             artifact.ImageRef,
			TriggerKind:          triggerKind,
			RuntimeMode:          runtimeMode,
			Services:             serviceContracts,
			DependencyBindings:   dependencyBindings,
			CompatibilityPolicy:  document.CompatibilityPolicy,
			MagicDomainPolicy:    magicDomainPolicy,
			ScaleToZeroPolicy:    document.ScaleToZeroPolicy,
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
			ID:              utils.NewPrefixedID("svc"),
			ProjectID:       projectID,
			Name:            strings.TrimSpace(item.Name),
			Path:            strings.TrimSpace(item.Path),
			Public:          item.Public,
			HealthcheckJSON: healthcheckJSON,
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
			Public:         item.Public,
			RuntimeProfile: runtimeProfile,
			Healthcheck:    healthcheck,
		}
		contract := BlueprintServiceContractRecord{
			Name:           model.Name,
			Path:           model.Path,
			Public:         item.Public,
			RuntimeProfile: runtimeProfile,
			StartHint:      strings.TrimSpace(item.StartHint),
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
		assignments = append(assignments, PlacementAssignmentRecord{
			ServiceName: service.Name,
			TargetID:    binding.TargetID,
			TargetKind:  binding.TargetKind,
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

	runtimeProfile := ""
	if item.RuntimeProfile != nil {
		runtimeProfile = *item.RuntimeProfile
	}

	return ProjectServiceRecord{
		ID:             item.ID,
		ProjectID:      item.ProjectID,
		Name:           item.Name,
		Path:           item.Path,
		Public:         item.Public,
		RuntimeProfile: runtimeProfile,
		Healthcheck:    healthcheck,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}, nil
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
