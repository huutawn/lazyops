package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

const (
	bootstrapModeStandalone      = "standalone"
	bootstrapModeDistributedMesh = "distributed-mesh"
	bootstrapModeDistributedK3s  = "distributed-k3s"
	bootstrapDeployStuckTimeout  = 10 * time.Minute
)

type BootstrapOrchestrator struct {
	projects         ProjectStore
	projectSvc       *ProjectService
	repoLinks        *ProjectRepoLinkService
	repoLinkStore    ProjectRepoLinkStore
	bindings         *DeploymentBindingService
	bindingStore     DeploymentBindingStore
	deployments      DeploymentStore
	instances        InstanceStore
	meshes           MeshNetworkStore
	clusters         ClusterStore
	installations    GitHubInstallationStore
	projectServices  ProjectServiceStore
	internalServices ProjectInternalServiceStore
	initContracts    *InitContractService
	blueprints       *BlueprintService
	deploymentSvc    *DeploymentService
	rolloutSvc       *RolloutExecutionService
}

type BootstrapAutoCommand struct {
	RequesterUserID      string
	RequesterRole        string
	ProjectID            string
	ProjectName          string
	DefaultBranch        string
	RepoFullName         string
	GitHubInstallationID int64
	GitHubRepoID         int64
	TrackedBranch        string
	InstanceID           string
	MeshNetworkID        string
	ClusterID            string
	AutoModeEnabled      *bool
	LockedRuntimeMode    string
}

type BootstrapAutoAcceptedRecord struct {
	JobID     string
	Status    string
	ProjectID string
}

type BootstrapOneClickDeployCommand struct {
	RequesterUserID string
	RequesterRole   string
	ProjectID       string
	SourceRef       string
	CommitSHA       string
	ArtifactRef     string
	ImageRef        string
	TriggerKind     string
}

type BootstrapPipelineEventRecord struct {
	ID        string
	State     string
	Label     string
	Message   string
	Timestamp time.Time
}

type BootstrapOneClickDeployRecord struct {
	ProjectID     string
	BlueprintID   string
	RevisionID    string
	DeploymentID  string
	RolloutStatus string
	RolloutReason string
	CorrelationID string
	AgentID       string
	Timeline      []BootstrapPipelineEventRecord
}

type BootstrapStepActionRecord struct {
	ID       string
	Label    string
	Kind     string
	Href     string
	Method   string
	Endpoint string
}

type BootstrapStepRecord struct {
	ID      string
	State   string
	Summary string
	Actions []BootstrapStepActionRecord
}

type BootstrapAutoModeRecord struct {
	Enabled              bool
	SelectedMode         string
	ModeSource           string
	ModeReasonCode       string
	ModeReasonHuman      string
	UpshiftAllowed       bool
	DownshiftAllowed     bool
	DownshiftBlockReason string
}

type BootstrapInventoryRecord struct {
	HealthyInstances    int
	HealthyMeshNetworks int
	HealthyK3sClusters  int
}

type ProjectBootstrapStatusRecord struct {
	ProjectID    string
	OverallState string
	Steps        []BootstrapStepRecord
	AutoMode     BootstrapAutoModeRecord
	Inventory    BootstrapInventoryRecord
	UpdatedAt    time.Time
}

type bootstrapInventorySnapshot struct {
	instances []models.Instance
	meshes    []models.MeshNetwork
	clusters  []models.Cluster
}

type bootstrapModeDecision struct {
	mode             string
	source           string
	reasonCode       string
	reasonHuman      string
	upshiftAllowed   bool
	downshiftAllowed bool
	downshiftBlock   string
}

func NewBootstrapOrchestrator(
	projects ProjectStore,
	projectSvc *ProjectService,
	repoLinks *ProjectRepoLinkService,
	repoLinkStore ProjectRepoLinkStore,
	bindings *DeploymentBindingService,
	bindingStore DeploymentBindingStore,
	deployments DeploymentStore,
	instances InstanceStore,
	meshes MeshNetworkStore,
	clusters ClusterStore,
	installations GitHubInstallationStore,
) *BootstrapOrchestrator {
	return &BootstrapOrchestrator{
		projects:      projects,
		projectSvc:    projectSvc,
		repoLinks:     repoLinks,
		repoLinkStore: repoLinkStore,
		bindings:      bindings,
		bindingStore:  bindingStore,
		deployments:   deployments,
		instances:     instances,
		meshes:        meshes,
		clusters:      clusters,
		installations: installations,
	}
}

func (s *BootstrapOrchestrator) WithOneClickPipeline(
	projectServices ProjectServiceStore,
	initContracts *InitContractService,
	blueprints *BlueprintService,
	deploymentSvc *DeploymentService,
	rolloutSvc *RolloutExecutionService,
) *BootstrapOrchestrator {
	if s == nil {
		return s
	}
	s.projectServices = projectServices
	s.initContracts = initContracts
	s.blueprints = blueprints
	s.deploymentSvc = deploymentSvc
	s.rolloutSvc = rolloutSvc
	return s
}

func (s *BootstrapOrchestrator) WithInternalServiceStore(store ProjectInternalServiceStore) *BootstrapOrchestrator {
	if s == nil {
		return s
	}
	s.internalServices = store
	return s
}

func (s *BootstrapOrchestrator) AutoBootstrap(cmd BootstrapAutoCommand) (*BootstrapAutoAcceptedRecord, error) {
	if s == nil || s.projects == nil || s.projectSvc == nil || s.bindings == nil || s.instances == nil || s.meshes == nil || s.clusters == nil {
		return nil, ErrInvalidInput
	}

	project, err := s.ensureProject(cmd)
	if err != nil {
		return nil, err
	}

	if err := s.ensureRepoLink(project, cmd); err != nil {
		return nil, err
	}

	if err := s.ensureBinding(project, cmd); err != nil {
		return nil, err
	}

	return &BootstrapAutoAcceptedRecord{
		JobID:     utils.NewPrefixedID("job_bootstrap"),
		Status:    "accepted",
		ProjectID: project.ID,
	}, nil
}

func (s *BootstrapOrchestrator) OneClickDeploy(cmd BootstrapOneClickDeployCommand) (*BootstrapOneClickDeployRecord, error) {
	if s == nil ||
		s.projects == nil ||
		s.repoLinkStore == nil ||
		s.bindingStore == nil ||
		s.initContracts == nil ||
		s.blueprints == nil ||
		s.deploymentSvc == nil {
		return nil, ErrInvalidInput
	}

	project, err := resolveProjectForAccess(s.projects, strings.TrimSpace(cmd.RequesterUserID), strings.TrimSpace(cmd.RequesterRole), strings.TrimSpace(cmd.ProjectID))
	if err != nil {
		return nil, err
	}

	repoLink, err := s.repoLinkStore.GetByProjectID(project.ID)
	if err != nil {
		return nil, err
	}
	if repoLink == nil {
		return nil, ErrRepoLinkNotFound
	}

	binding, err := s.resolvePrimaryBinding(project.ID)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, ErrUnknownTargetRef
	}

	lazyopsDocument, err := s.buildOneClickLazyopsDocument(*project, *binding)
	if err != nil {
		return nil, err
	}
	lazyopsRaw, err := json.Marshal(lazyopsDocument)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	timeline := []BootstrapPipelineEventRecord{
		{
			ID:        "validate",
			State:     "running",
			Label:     "Validate contract",
			Message:   "Validating generated lazyops.yaml contract",
			Timestamp: now,
		},
	}

	if _, err := s.initContracts.ValidateLazyopsYAML(ValidateLazyopsYAMLCommand{
		RequesterUserID: cmd.RequesterUserID,
		RequesterRole:   cmd.RequesterRole,
		ProjectID:       project.ID,
		RawDocument:     lazyopsRaw,
	}); err != nil {
		return nil, err
	}
	timeline[0].State = "completed"
	timeline[0].Timestamp = time.Now().UTC()
	timeline[0].Message = "Contract validation passed"

	timeline = append(timeline, BootstrapPipelineEventRecord{
		ID:        "compile",
		State:     "running",
		Label:     "Compile blueprint",
		Message:   "Compiling deployment blueprint",
		Timestamp: time.Now().UTC(),
	})

	commitSHA := resolveOneClickCommitSHA(cmd.CommitSHA)
	artifactRef := strings.TrimSpace(cmd.ArtifactRef)
	if artifactRef == "" {
		artifactRef = fmt.Sprintf("artifact://one-click/%s/%s", project.ID, commitSHA)
	}
	triggerKind := strings.TrimSpace(cmd.TriggerKind)
	if triggerKind == "" {
		triggerKind = "one_click_deploy"
	}

	compileResult, err := s.blueprints.Compile(CompileBlueprintCommand{
		RequesterUserID: cmd.RequesterUserID,
		RequesterRole:   cmd.RequesterRole,
		ProjectID:       project.ID,
		SourceRef:       resolveOneClickSourceRef(cmd.SourceRef, *repoLink),
		TriggerKind:     triggerKind,
		Artifact: BlueprintArtifactMetadata{
			CommitSHA:   commitSHA,
			ArtifactRef: artifactRef,
			ImageRef:    strings.TrimSpace(cmd.ImageRef),
		},
		LazyopsYAMLRaw: lazyopsRaw,
	})
	if err != nil {
		return nil, err
	}
	timeline[len(timeline)-1].State = "completed"
	timeline[len(timeline)-1].Timestamp = time.Now().UTC()
	timeline[len(timeline)-1].Message = fmt.Sprintf("Blueprint %s compiled", compileResult.Blueprint.ID)

	timeline = append(timeline, BootstrapPipelineEventRecord{
		ID:        "create_deployment",
		State:     "running",
		Label:     "Create deployment",
		Message:   "Creating deployment revision",
		Timestamp: time.Now().UTC(),
	})

	deployResult, err := s.deploymentSvc.Create(CreateDeploymentCommand{
		RequesterUserID: cmd.RequesterUserID,
		RequesterRole:   cmd.RequesterRole,
		ProjectID:       project.ID,
		BlueprintID:     compileResult.Blueprint.ID,
		TriggerKind:     triggerKind,
	})
	if err != nil {
		return nil, err
	}
	timeline[len(timeline)-1].State = "completed"
	timeline[len(timeline)-1].Timestamp = time.Now().UTC()
	timeline[len(timeline)-1].Message = fmt.Sprintf("Deployment %s created", deployResult.Deployment.ID)

	rolloutStatus := "not_started"
	rolloutReason := ""
	correlationID := ""
	agentID := ""

	timeline = append(timeline, BootstrapPipelineEventRecord{
		ID:        "rollout",
		State:     "pending",
		Label:     "Start rollout",
		Message:   "Rollout kickoff is pending",
		Timestamp: time.Now().UTC(),
	})

	if s.rolloutSvc != nil {
		rolloutResult, rolloutErr := s.rolloutSvc.StartDeployment(context.Background(), project.ID, deployResult.Deployment.ID)
		switch {
		case rolloutErr == nil:
			rolloutStatus = "started"
			if rolloutResult != nil {
				correlationID = rolloutResult.CorrelationID
				agentID = rolloutResult.AgentID
			}
			timeline[len(timeline)-1].State = "completed"
			timeline[len(timeline)-1].Timestamp = time.Now().UTC()
			timeline[len(timeline)-1].Message = "Rollout started"
		case errors.Is(rolloutErr, ErrRolloutArtifactPending),
			errors.Is(rolloutErr, ErrRolloutAgentUnavailable),
			errors.Is(rolloutErr, ErrRolloutUnsupportedTarget),
			errors.Is(rolloutErr, ErrRolloutAlreadyStarted):
			rolloutStatus = "pending"
			rolloutReason = rolloutErr.Error()
			timeline[len(timeline)-1].State = "pending"
			timeline[len(timeline)-1].Timestamp = time.Now().UTC()
			timeline[len(timeline)-1].Message = rolloutReason
		default:
			rolloutStatus = "failed_to_start"
			rolloutReason = rolloutErr.Error()
			timeline[len(timeline)-1].State = "failed"
			timeline[len(timeline)-1].Timestamp = time.Now().UTC()
			timeline[len(timeline)-1].Message = rolloutReason
		}
	}

	return &BootstrapOneClickDeployRecord{
		ProjectID:     project.ID,
		BlueprintID:   compileResult.Blueprint.ID,
		RevisionID:    deployResult.Revision.ID,
		DeploymentID:  deployResult.Deployment.ID,
		RolloutStatus: rolloutStatus,
		RolloutReason: rolloutReason,
		CorrelationID: correlationID,
		AgentID:       agentID,
		Timeline:      timeline,
	}, nil
}

func (s *BootstrapOrchestrator) OnInventoryChanged(userID string) error {
	if s == nil || s.projects == nil || s.bindings == nil {
		return ErrInvalidInput
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrInvalidInput
	}

	projects, err := s.projects.ListByUser(userID)
	if err != nil {
		return err
	}
	for _, project := range projects {
		item := project
		if err := s.ensureBinding(&item, BootstrapAutoCommand{
			RequesterUserID: userID,
			RequesterRole:   RoleViewer,
			ProjectID:       item.ID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *BootstrapOrchestrator) GetStatus(requesterUserID, requesterRole, projectID string) (*ProjectBootstrapStatusRecord, error) {
	if s == nil || s.projects == nil || s.instances == nil || s.meshes == nil || s.clusters == nil {
		return nil, ErrInvalidInput
	}

	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}

	codeState, codeSummary := "missing", "Chưa kết nối kho mã nguồn GitHub"
	if s.repoLinkStore != nil {
		link, getErr := s.repoLinkStore.GetByProjectID(project.ID)
		if getErr != nil {
			return nil, getErr
		}
		if link != nil {
			codeState = "healthy"
			codeSummary = fmt.Sprintf("Đã kết nối: %s/%s@%s", link.RepoOwner, link.RepoName, link.TrackedBranch)
		}
	}

	inventory, err := s.collectInventory(project.UserID)
	if err != nil {
		return nil, err
	}

	infraState, infraSummary := deriveInfraStateSummary(inventory)
	deployState, deploySummary, err := s.deriveDeployState(project.ID, codeState, infraState)
	if err != nil {
		return nil, err
	}

	autoEnabled := true
	decision := inferBootstrapMode(inventory, autoEnabled, "")

	steps := []BootstrapStepRecord{
		{
			ID:      "connect_code",
			State:   codeState,
			Summary: codeSummary,
			Actions: []BootstrapStepActionRecord{{
				ID:    "reconnect_github",
				Label: "Kết nối GitHub",
				Kind:  "link",
				Href:  fmt.Sprintf("/api/auth/oauth/github/start?next=/projects/%s", project.ID),
			}},
		},
		{
			ID:      "connect_infra",
			State:   infraState,
			Summary: infraSummary,
			Actions: []BootstrapStepActionRecord{{
				ID:    "add_server",
				Label: "Kết nối máy chủ",
				Kind:  "screen",
				Href:  "/instances",
			}},
		},
		{
			ID:      "deploy",
			State:   deployState,
			Summary: deploySummary,
			Actions: buildDeployActions(project.ID, deployState),
		},
	}

	status := &ProjectBootstrapStatusRecord{
		ProjectID:    project.ID,
		OverallState: deriveOverallState(codeState, infraState, deployState),
		Steps:        steps,
		AutoMode: BootstrapAutoModeRecord{
			Enabled:              autoEnabled,
			SelectedMode:         decision.mode,
			ModeSource:           decision.source,
			ModeReasonCode:       decision.reasonCode,
			ModeReasonHuman:      decision.reasonHuman,
			UpshiftAllowed:       decision.upshiftAllowed,
			DownshiftAllowed:     decision.downshiftAllowed,
			DownshiftBlockReason: decision.downshiftBlock,
		},
		Inventory: BootstrapInventoryRecord{
			HealthyInstances:    healthyInstanceCount(inventory),
			HealthyMeshNetworks: healthyMeshCount(inventory),
			HealthyK3sClusters:  healthyClusterCount(inventory),
		},
		UpdatedAt: time.Now().UTC(),
	}

	return status, nil
}

func (s *BootstrapOrchestrator) ensureProject(cmd BootstrapAutoCommand) (*models.Project, error) {
	requesterUserID := strings.TrimSpace(cmd.RequesterUserID)
	if requesterUserID == "" {
		return nil, ErrInvalidInput
	}

	projectID := strings.TrimSpace(cmd.ProjectID)
	if projectID != "" {
		return resolveProjectForAccess(s.projects, requesterUserID, cmd.RequesterRole, projectID)
	}

	projectName := utils.NormalizeSpace(cmd.ProjectName)
	if projectName == "" {
		_, repoName, ok := splitRepoFullName(cmd.RepoFullName)
		if ok {
			projectName = repoName
		}
	}
	if projectName == "" {
		return nil, ErrInvalidInput
	}

	slug := normalizeProjectSlug(projectName)
	if slug == "" {
		return nil, ErrInvalidInput
	}

	existing, err := s.projects.GetBySlugForUser(requesterUserID, slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	created, err := s.projectSvc.Create(CreateProjectCommand{
		UserID:        requesterUserID,
		Name:          projectName,
		Slug:          slug,
		DefaultBranch: cmd.DefaultBranch,
	})
	if err != nil {
		if err != ErrProjectSlugExists {
			return nil, err
		}
		existing, lookupErr := s.projects.GetBySlugForUser(requesterUserID, slug)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing != nil {
			return existing, nil
		}
		return nil, err
	}

	project, err := s.projects.GetByIDForUser(requesterUserID, created.ID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, ErrProjectNotFound
	}

	return project, nil
}

func (s *BootstrapOrchestrator) ensureRepoLink(project *models.Project, cmd BootstrapAutoCommand) error {
	if project == nil || s.repoLinks == nil || s.repoLinkStore == nil {
		return nil
	}

	existing, err := s.repoLinkStore.GetByProjectID(project.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	repoFullName := strings.TrimSpace(cmd.RepoFullName)
	installationID := cmd.GitHubInstallationID
	repoID := cmd.GitHubRepoID

	if installationID <= 0 || repoID <= 0 {
		resolvedInstallationID, resolvedRepoID, found, discoverErr := s.discoverRepositoryScope(project.UserID, repoFullName, installationID, repoID)
		if discoverErr != nil {
			return discoverErr
		}
		if !found {
			return nil
		}
		installationID = resolvedInstallationID
		repoID = resolvedRepoID
	}

	if installationID <= 0 || repoID <= 0 {
		return nil
	}

	trackedBranch := strings.TrimSpace(cmd.TrackedBranch)
	if trackedBranch == "" {
		trackedBranch = project.DefaultBranch
	}

	_, err = s.repoLinks.LinkRepository(CreateProjectRepoLinkCommand{
		RequesterUserID:      strings.TrimSpace(cmd.RequesterUserID),
		RequesterRole:        cmd.RequesterRole,
		ProjectID:            project.ID,
		GitHubInstallationID: installationID,
		GitHubRepoID:         repoID,
		TrackedBranch:        trackedBranch,
		PreviewEnabled:       false,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *BootstrapOrchestrator) discoverRepositoryScope(userID, repoFullName string, installationHint, repoIDHint int64) (int64, int64, bool, error) {
	if s.installations == nil {
		return 0, 0, false, nil
	}

	repoFullName = strings.TrimSpace(repoFullName)
	if repoFullName == "" && repoIDHint <= 0 {
		return 0, 0, false, nil
	}

	ownerHint, repoHint, hasNameHint := splitRepoFullName(repoFullName)
	installations, err := s.installations.ListByUser(strings.TrimSpace(userID))
	if err != nil {
		return 0, 0, false, err
	}

	for _, installation := range installations {
		if installation.RevokedAt != nil {
			continue
		}
		if installationHint > 0 && installation.GitHubInstallationID != installationHint {
			continue
		}

		scope, parseErr := parseGitHubInstallationScope(installation.ScopeJSON)
		if parseErr != nil {
			return 0, 0, false, parseErr
		}

		for _, repository := range scope.Repositories {
			if repoIDHint > 0 && repository.ID != repoIDHint {
				continue
			}
			if hasNameHint {
				if strings.EqualFold(strings.TrimSpace(repository.FullName), ownerHint+"/"+repoHint) {
					return installation.GitHubInstallationID, repository.ID, true, nil
				}
				if strings.EqualFold(strings.TrimSpace(repository.OwnerLogin), ownerHint) && strings.EqualFold(strings.TrimSpace(repository.Name), repoHint) {
					return installation.GitHubInstallationID, repository.ID, true, nil
				}
				continue
			}
			if repoIDHint > 0 {
				return installation.GitHubInstallationID, repository.ID, true, nil
			}
		}
	}

	return 0, 0, false, nil
}

func (s *BootstrapOrchestrator) ensureBinding(project *models.Project, cmd BootstrapAutoCommand) error {
	if project == nil || s.bindings == nil {
		return nil
	}

	requesterRole := strings.TrimSpace(cmd.RequesterRole)
	if requesterRole == "" {
		requesterRole = RoleViewer
	}

	existing, err := s.bindings.List(strings.TrimSpace(cmd.RequesterUserID), requesterRole, project.ID)
	if err != nil {
		return err
	}

	var autoBinding *DeploymentBindingRecord
	manualBindings := 0
	if existing != nil {
		for _, item := range existing.Items {
			if item.TargetRef == "auto-primary" {
				copyItem := item
				autoBinding = &copyItem
				continue
			}
			manualBindings++
		}
	}
	if autoBinding == nil && manualBindings > 0 {
		return nil
	}

	inventory, err := s.collectInventory(project.UserID)
	if err != nil {
		return err
	}
	if totalTargetCount(inventory) == 0 {
		return nil
	}

	decision := inferBootstrapMode(inventory, resolveAutoModeEnabled(cmd.AutoModeEnabled), cmd.LockedRuntimeMode)
	targetKind, targetID, ok := chooseBindingTarget(inventory, decision.mode, strings.TrimSpace(cmd.InstanceID), strings.TrimSpace(cmd.MeshNetworkID), strings.TrimSpace(cmd.ClusterID))
	if !ok {
		return nil
	}

	name := utils.NormalizeSpace("Auto " + strings.ReplaceAll(decision.mode, "-", " "))
	if name == "" {
		name = "Auto binding"
	}

	if autoBinding != nil && autoBinding.RuntimeMode == decision.mode && autoBinding.TargetKind == targetKind && autoBinding.TargetID == targetID {
		return nil
	}

	if s.bindingStore == nil {
		if autoBinding != nil {
			return nil
		}
		_, err = s.bindings.Create(CreateDeploymentBindingCommand{
			RequesterUserID: strings.TrimSpace(cmd.RequesterUserID),
			RequesterRole:   requesterRole,
			ProjectID:       project.ID,
			Name:            name,
			TargetRef:       "auto-primary",
			RuntimeMode:     decision.mode,
			TargetKind:      targetKind,
			TargetID:        targetID,
			PlacementPolicy: defaultPlacementPolicy(decision.mode),
			DomainPolicy: map[string]any{
				"mode": "auto",
			},
			CompatibilityPolicy: map[string]any{
				"min_version": "1.0",
			},
			ScaleToZeroPolicy: map[string]any{
				"enabled": false,
			},
		})
		if err != nil && err != ErrDuplicateTargetRef {
			return err
		}
		return nil
	}

	id := utils.NewPrefixedID("bind")
	if autoBinding != nil && strings.TrimSpace(autoBinding.ID) != "" {
		id = autoBinding.ID
	}

	placementPolicyJSON, err := marshalBindingPolicyJSON(defaultPlacementPolicy(decision.mode))
	if err != nil {
		return err
	}
	domainPolicyJSON, err := marshalBindingPolicyJSON(map[string]any{
		"mode": "auto",
	})
	if err != nil {
		return err
	}
	compatibilityPolicyJSON, err := marshalBindingPolicyJSON(map[string]any{
		"min_version": "1.0",
	})
	if err != nil {
		return err
	}
	scaleToZeroPolicyJSON, err := marshalBindingPolicyJSON(map[string]any{
		"enabled": false,
	})
	if err != nil {
		return err
	}

	err = s.bindingStore.UpsertAuto(&models.DeploymentBinding{
		ID:                      id,
		ProjectID:               project.ID,
		Name:                    name,
		TargetRef:               "auto-primary",
		RuntimeMode:             decision.mode,
		TargetKind:              targetKind,
		TargetID:                targetID,
		PlacementPolicyJSON:     placementPolicyJSON,
		DomainPolicyJSON:        domainPolicyJSON,
		CompatibilityPolicyJSON: compatibilityPolicyJSON,
		ScaleToZeroPolicyJSON:   scaleToZeroPolicyJSON,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *BootstrapOrchestrator) collectInventory(userID string) (bootstrapInventorySnapshot, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return bootstrapInventorySnapshot{}, ErrInvalidInput
	}

	instances, err := s.instances.ListByUser(userID)
	if err != nil {
		return bootstrapInventorySnapshot{}, err
	}
	meshes, err := s.meshes.ListByUser(userID)
	if err != nil {
		return bootstrapInventorySnapshot{}, err
	}
	clusters, err := s.clusters.ListByUser(userID)
	if err != nil {
		return bootstrapInventorySnapshot{}, err
	}

	return bootstrapInventorySnapshot{
		instances: instances,
		meshes:    meshes,
		clusters:  clusters,
	}, nil
}

func (s *BootstrapOrchestrator) deriveDeployState(projectID, codeState, infraState string) (string, string, error) {
	if strings.TrimSpace(projectID) == "" {
		return "blocked", "Dự án chưa sẵn sàng", nil
	}

	if codeState != "healthy" || (infraState != "ready" && infraState != "degraded") {
		return "blocked", "Hãy kết nối mã nguồn và máy chủ trước", nil
	}

	if s.deployments == nil {
		return "ready", "Đã sẵn sàng triển khai", nil
	}

	deployments, err := s.deployments.ListByProject(projectID)
	if err != nil {
		return "error", "Failed to inspect deployments", err
	}
	if len(deployments) == 0 {
		return "ready", "Đã sẵn sàng triển khai", nil
	}

	latest := deployments[0]
	status := strings.ToLower(strings.TrimSpace(latest.Status))
	switch status {
	case DeploymentStatusQueued, DeploymentStatusRunning, DeploymentStatusCandidateReady:
		lastActivity := latest.UpdatedAt
		if lastActivity.IsZero() {
			lastActivity = latest.CreatedAt
		}
		if !lastActivity.IsZero() && time.Since(lastActivity) > bootstrapDeployStuckTimeout {
			return "error", "Triển khai trước đó có thể đang bị kẹt. Vui lòng triển khai lại", nil
		}
		return "deploying", "Đang triển khai", nil
	case DeploymentStatusPromoted:
		return "healthy", "Bản triển khai mới nhất đang hoạt động tốt", nil
	case DeploymentStatusRolledBack:
		return "rolled_back", "Bản triển khai mới nhất đã bị hoàn tác", nil
	case DeploymentStatusFailed, DeploymentStatusCanceled:
		return "error", "Bản triển khai mới nhất thất bại", nil
	default:
		return "ready", "Đã sẵn sàng triển khai", nil
	}
}

func (s *BootstrapOrchestrator) resolvePrimaryBinding(projectID string) (*models.DeploymentBinding, error) {
	if s.bindingStore == nil {
		return nil, ErrInvalidInput
	}

	autoBinding, err := s.bindingStore.GetByTargetRefForProject(projectID, "auto-primary")
	if err != nil {
		return nil, err
	}
	if autoBinding != nil {
		return autoBinding, nil
	}

	items, err := s.bindingStore.ListByProject(projectID)
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

func (s *BootstrapOrchestrator) buildOneClickLazyopsDocument(project models.Project, binding models.DeploymentBinding) (LazyopsYAMLDocument, error) {
	services := make([]LazyopsYAMLService, 0)
	if s.projectServices != nil {
		persisted, err := s.projectServices.ListByProject(project.ID)
		if err != nil {
			return LazyopsYAMLDocument{}, err
		}
		for _, item := range persisted {
			services = append(services, toLazyopsService(item))
		}
	}

	if len(services) == 0 {
		services = append(services, LazyopsYAMLService{
			Name:   "app",
			Path:   ".",
			Public: true,
			Healthcheck: LazyopsYAMLServiceHealthcheck{
				Path: "/health",
				Port: 8080,
			},
		})
	}

	internalServices := make([]models.ProjectInternalService, 0)
	if s.internalServices != nil {
		persisted, err := s.internalServices.ListByProject(project.ID)
		if err != nil {
			return LazyopsYAMLDocument{}, err
		}
		internalServices = append(internalServices, persisted...)
	}

	services, dependencyBindings := buildInternalServicesDependencyBindings(services, internalServices)

	compatibilityPolicy, err := decodeAnyMapJSON(binding.CompatibilityPolicyJSON)
	if err != nil {
		return LazyopsYAMLDocument{}, err
	}
	scaleToZeroPolicy, err := decodeAnyMapJSON(binding.ScaleToZeroPolicyJSON)
	if err != nil {
		return LazyopsYAMLDocument{}, err
	}

	defaultEnvInjection := true
	defaultLocalhostRescue := false
	if len(dependencyBindings) > 0 {
		defaultEnvInjection = false
		defaultLocalhostRescue = true
	}
	compatibility := LazyopsYAMLCompatibilityPolicy{
		EnvInjection:       boolFromPolicy(compatibilityPolicy, "env_injection", defaultEnvInjection),
		ManagedCredentials: boolFromPolicy(compatibilityPolicy, "managed_credentials", false),
		LocalhostRescue:    boolFromPolicy(compatibilityPolicy, "localhost_rescue", defaultLocalhostRescue),
	}
	if !compatibility.EnvInjection && !compatibility.ManagedCredentials && !compatibility.LocalhostRescue {
		if len(dependencyBindings) > 0 {
			compatibility.LocalhostRescue = true
		} else {
			compatibility.EnvInjection = true
		}
	}

	return LazyopsYAMLDocument{
		ProjectSlug: project.Slug,
		RuntimeMode: binding.RuntimeMode,
		DeploymentBinding: LazyopsYAMLDeploymentBindingRef{
			TargetRef: binding.TargetRef,
		},
		Services:            services,
		DependencyBindings:  dependencyBindings,
		CompatibilityPolicy: compatibility,
		MagicDomainPolicy: LazyopsYAMLMagicDomainPolicy{
			Enabled: false,
		},
		PreviewPolicy: LazyopsYAMLPreviewPolicy{
			Enabled: false,
		},
		ScaleToZeroPolicy: LazyopsYAMLScaleToZeroPolicy{
			Enabled: boolFromPolicy(scaleToZeroPolicy, "enabled", false),
		},
	}, nil
}

func toLazyopsService(item models.Service) LazyopsYAMLService {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = "app"
	}

	path := strings.TrimSpace(item.Path)
	if path == "" {
		path = "."
	}

	healthcheck := LazyopsYAMLServiceHealthcheck{}
	decoded, err := decodeAnyMapJSON(item.HealthcheckJSON)
	if err == nil {
		healthcheck.Path = strings.TrimSpace(policyString(decoded, "path"))
		healthcheck.Port = policyInt(decoded, "port")
	}

	return LazyopsYAMLService{
		Name:        name,
		Path:        path,
		Public:      item.Public,
		Healthcheck: healthcheck,
	}
}

func resolveOneClickCommitSHA(raw string) string {
	commit := strings.TrimSpace(raw)
	if commit == "" || strings.ContainsAny(commit, " \t\r\n") {
		return "autogen-" + time.Now().UTC().Format("20060102T150405Z")
	}
	return commit
}

func resolveOneClickSourceRef(raw string, repoLink models.ProjectRepoLink) string {
	sourceRef := strings.TrimSpace(raw)
	if sourceRef != "" {
		return sourceRef
	}

	repoFullName := strings.TrimSpace(repoLink.RepoOwner + "/" + repoLink.RepoName)
	branch := strings.TrimSpace(repoLink.TrackedBranch)
	if repoFullName != "" && branch != "" {
		return repoFullName + "@" + branch
	}
	if repoFullName != "" {
		return repoFullName
	}
	return "lazyops/unknown@main"
}

func boolFromPolicy(policy map[string]any, key string, fallback bool) bool {
	raw, ok := policy[key]
	if !ok {
		return fallback
	}

	switch typed := raw.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		if normalized == "true" {
			return true
		}
		if normalized == "false" {
			return false
		}
	}

	return fallback
}

func policyString(policy map[string]any, key string) string {
	raw, ok := policy[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}

func policyInt(policy map[string]any, key string) int {
	raw, ok := policy[key]
	if !ok {
		return 0
	}
	switch typed := raw.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		value, err := typed.Int64()
		if err != nil {
			return 0
		}
		return int(value)
	default:
		return 0
	}
}

func deriveInfraStateSummary(inventory bootstrapInventorySnapshot) (string, string) {
	total := totalTargetCount(inventory)
	healthy := healthyTargetCount(inventory)
	if total == 0 {
		return "missing", "Chưa có máy chủ nào được kết nối"
	}
	if healthy > 0 {
		return "ready", fmt.Sprintf("Có %d máy chủ/hạ tầng khả dụng", healthy)
	}
	return "degraded", "Đã kết nối hạ tầng nhưng chưa sẵn sàng"
}

func inferBootstrapMode(inventory bootstrapInventorySnapshot, autoEnabled bool, lockedMode string) bootstrapModeDecision {
	if !autoEnabled {
		mode, err := normalizeBindingRuntimeMode(strings.TrimSpace(lockedMode))
		if err != nil {
			mode = bootstrapModeStandalone
		}
		return bootstrapModeDecision{
			mode:             mode,
			source:           "manual_lock",
			reasonCode:       "manual_mode_locked",
			reasonHuman:      "Chế độ chạy đã bị khóa thủ công",
			upshiftAllowed:   false,
			downshiftAllowed: false,
			downshiftBlock:   "manual_lock",
		}
	}

	decision := bootstrapModeDecision{
		mode:             bootstrapModeStandalone,
		source:           "auto",
		reasonCode:       "single_instance_only",
		reasonHuman:      "Chỉ có một máy chủ khả dụng",
		upshiftAllowed:   true,
		downshiftAllowed: false,
		downshiftBlock:   "already_lowest_mode",
	}

	if healthyClusterCount(inventory) >= 1 {
		decision.mode = bootstrapModeDistributedK3s
		decision.reasonCode = "k3s_detected"
		decision.reasonHuman = "Phát hiện cụm K3s khả dụng"
		decision.downshiftBlock = "hysteresis_24h"
		return decision
	}

	if healthyInstanceCount(inventory) >= 2 {
		decision.mode = bootstrapModeDistributedMesh
		decision.reasonCode = "multi_instance_detected"
		decision.reasonHuman = "Phát hiện từ 2 máy chủ khả dụng"
		decision.downshiftBlock = "hysteresis_24h"
		return decision
	}

	if healthyMeshCount(inventory) >= 1 {
		decision.mode = bootstrapModeDistributedMesh
		decision.reasonCode = "mesh_network_detected"
		decision.reasonHuman = "Phát hiện mạng mesh khả dụng"
		decision.downshiftBlock = "hysteresis_24h"
		return decision
	}

	return decision
}

func chooseBindingTarget(inventory bootstrapInventorySnapshot, runtimeMode, preferredInstanceID, preferredMeshID, preferredClusterID string) (string, string, bool) {
	runtimeMode = strings.TrimSpace(runtimeMode)
	switch runtimeMode {
	case bootstrapModeDistributedK3s:
		if preferredClusterID != "" {
			if cluster, ok := findClusterByID(inventory, preferredClusterID); ok {
				return "cluster", cluster.ID, true
			}
		}
		if preferredClusterID != "" {
			if cluster, ok := findHealthyClusterByID(inventory, preferredClusterID); ok {
				return "cluster", cluster.ID, true
			}
		}
		if cluster, ok := firstHealthyCluster(inventory); ok {
			return "cluster", cluster.ID, true
		}
		return "", "", false
	case bootstrapModeDistributedMesh:
		if preferredMeshID != "" {
			if mesh, ok := findMeshByID(inventory, preferredMeshID); ok {
				return "mesh", mesh.ID, true
			}
		}
		if preferredMeshID != "" {
			if mesh, ok := findHealthyMeshByID(inventory, preferredMeshID); ok {
				return "mesh", mesh.ID, true
			}
		}
		if mesh, ok := firstHealthyMesh(inventory); ok {
			return "mesh", mesh.ID, true
		}
		if preferredInstanceID != "" {
			if instance, ok := findHealthyInstanceByID(inventory, preferredInstanceID); ok {
				return "instance", instance.ID, true
			}
		}
		if instance, ok := firstHealthyInstance(inventory); ok {
			return "instance", instance.ID, true
		}
		return "", "", false
	default:
		if preferredInstanceID != "" {
			if instance, ok := findInstanceByID(inventory, preferredInstanceID); ok {
				return "instance", instance.ID, true
			}
		}
		if preferredInstanceID != "" {
			if instance, ok := findHealthyInstanceByID(inventory, preferredInstanceID); ok {
				return "instance", instance.ID, true
			}
		}
		if instance, ok := firstHealthyInstance(inventory); ok {
			return "instance", instance.ID, true
		}
		return "", "", false
	}
}

func defaultPlacementPolicy(runtimeMode string) map[string]any {
	if runtimeMode == bootstrapModeStandalone {
		return map[string]any{"strategy": "single-node"}
	}
	return map[string]any{"strategy": "spread"}
}

func buildDeployActions(projectID, deployState string) []BootstrapStepActionRecord {
	actions := []BootstrapStepActionRecord{}
	if deployState == "ready" {
		actions = append(actions, BootstrapStepActionRecord{
			ID:       "deploy_now",
			Label:    "Triển khai ngay",
			Kind:     "api",
			Method:   "POST",
			Endpoint: fmt.Sprintf("/api/v1/projects/%s/deploy/one-click", projectID),
		})
		return actions
	}

	actions = append(actions, BootstrapStepActionRecord{
		ID:    "view_deployments",
		Label: "Xem lịch sử triển khai",
		Kind:  "screen",
		Href:  fmt.Sprintf("/projects/%s/deployments", projectID),
	})
	return actions
}

func deriveOverallState(codeState, infraState, deployState string) string {
	switch deployState {
	case "deploying":
		return "deploying"
	case "healthy", "degraded":
		return "running"
	case "error", "rolled_back":
		return "attention_required"
	}

	if codeState == "healthy" && infraState == "ready" && deployState == "ready" {
		return "ready_to_deploy"
	}
	if codeState == "missing" && infraState == "missing" {
		return "not_ready"
	}
	return "partially_ready"
}

func healthyInstanceCount(inventory bootstrapInventorySnapshot) int {
	count := 0
	for _, instance := range inventory.instances {
		if normalizeInstanceStatus(instance.Status) == "online" {
			count++
		}
	}
	return count
}

func healthyMeshCount(inventory bootstrapInventorySnapshot) int {
	count := 0
	for _, mesh := range inventory.meshes {
		if normalizeMeshNetworkStatus(mesh.Status) == "active" {
			count++
		}
	}
	return count
}

func healthyClusterCount(inventory bootstrapInventorySnapshot) int {
	count := 0
	for _, cluster := range inventory.clusters {
		if normalizeClusterStatus(cluster.Status) == "ready" {
			count++
		}
	}
	return count
}

func healthyTargetCount(inventory bootstrapInventorySnapshot) int {
	return healthyInstanceCount(inventory) + healthyMeshCount(inventory) + healthyClusterCount(inventory)
}

func totalTargetCount(inventory bootstrapInventorySnapshot) int {
	return len(inventory.instances) + len(inventory.meshes) + len(inventory.clusters)
}

func firstHealthyInstance(inventory bootstrapInventorySnapshot) (models.Instance, bool) {
	for _, item := range inventory.instances {
		if normalizeInstanceStatus(item.Status) == "online" {
			return item, true
		}
	}
	return models.Instance{}, false
}

func firstHealthyMesh(inventory bootstrapInventorySnapshot) (models.MeshNetwork, bool) {
	for _, item := range inventory.meshes {
		if normalizeMeshNetworkStatus(item.Status) == "active" {
			return item, true
		}
	}
	return models.MeshNetwork{}, false
}

func firstHealthyCluster(inventory bootstrapInventorySnapshot) (models.Cluster, bool) {
	for _, item := range inventory.clusters {
		if normalizeClusterStatus(item.Status) == "ready" {
			return item, true
		}
	}
	return models.Cluster{}, false
}

func findHealthyInstanceByID(inventory bootstrapInventorySnapshot, id string) (models.Instance, bool) {
	for _, item := range inventory.instances {
		if item.ID == id && normalizeInstanceStatus(item.Status) == "online" {
			return item, true
		}
	}
	return models.Instance{}, false
}

func findInstanceByID(inventory bootstrapInventorySnapshot, id string) (models.Instance, bool) {
	for _, item := range inventory.instances {
		if item.ID == id {
			return item, true
		}
	}
	return models.Instance{}, false
}

func findHealthyMeshByID(inventory bootstrapInventorySnapshot, id string) (models.MeshNetwork, bool) {
	for _, item := range inventory.meshes {
		if item.ID == id && normalizeMeshNetworkStatus(item.Status) == "active" {
			return item, true
		}
	}
	return models.MeshNetwork{}, false
}

func findMeshByID(inventory bootstrapInventorySnapshot, id string) (models.MeshNetwork, bool) {
	for _, item := range inventory.meshes {
		if item.ID == id {
			return item, true
		}
	}
	return models.MeshNetwork{}, false
}

func findHealthyClusterByID(inventory bootstrapInventorySnapshot, id string) (models.Cluster, bool) {
	for _, item := range inventory.clusters {
		if item.ID == id && normalizeClusterStatus(item.Status) == "ready" {
			return item, true
		}
	}
	return models.Cluster{}, false
}

func findClusterByID(inventory bootstrapInventorySnapshot, id string) (models.Cluster, bool) {
	for _, item := range inventory.clusters {
		if item.ID == id {
			return item, true
		}
	}
	return models.Cluster{}, false
}

func splitRepoFullName(input string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(input), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

func resolveAutoModeEnabled(input *bool) bool {
	if input == nil {
		return true
	}
	return *input
}
