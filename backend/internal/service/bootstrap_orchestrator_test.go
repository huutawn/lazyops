package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

func TestBootstrapOrchestratorGetStatusReadyToDeploy(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "prod-k3s",
		InstanceID:          ptrString("inst_123"),
		Provider:            "k3s",
		KubeconfigSecretRef: "secret://clusters/prod-k3s/kubeconfig",
		Status:              "ready",
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{{
		Name:   "api",
		Path:   "apps/api",
		Kind:   "app",
		Public: true,
	}})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		repoLinkStore,
		nil,
		nil,
		newFakeDeploymentStore(),
		instanceStore,
		newFakeMeshNetworkStore(),
		clusterStore,
		nil,
	).WithOneClickPipeline(serviceStore, nil, nil, nil, nil, nil)

	status, err := orchestrator.GetStatus("usr_123", RoleViewer, "prj_123")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.OverallState != "ready_to_deploy" {
		t.Fatalf("expected overall ready_to_deploy, got %q", status.OverallState)
	}
	if len(status.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(status.Steps))
	}
	if status.Steps[0].State != "healthy" {
		t.Fatalf("expected connect_code healthy, got %q", status.Steps[0].State)
	}
	if status.Steps[1].State != "ready" {
		t.Fatalf("expected connect_infra ready with k3s cluster, got %q", status.Steps[1].State)
	}
	if status.Steps[2].State != "ready" {
		t.Fatalf("expected deploy ready, got %q", status.Steps[2].State)
	}
}

func TestBootstrapOrchestratorGetStatusBlocksDeployWhenServiceInventoryIsEmpty(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Empty Project",
		Slug:          "empty-project",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "prod-k3s",
		InstanceID:          ptrString("inst_123"),
		Provider:            "k3s",
		KubeconfigSecretRef: "secret://clusters/prod-k3s/kubeconfig",
		Status:              "ready",
	})
	serviceStore := newFakeProjectServiceStore()

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		newFakeProjectRepoLinkStore(),
		nil,
		nil,
		newFakeDeploymentStore(),
		instanceStore,
		newFakeMeshNetworkStore(),
		clusterStore,
		nil,
	).WithOneClickPipeline(serviceStore, nil, nil, nil, nil, nil)

	status, err := orchestrator.GetStatus("usr_123", RoleViewer, "prj_123")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.Steps[0].State != "healthy" {
		t.Fatalf("expected code step healthy for empty inventory, got %q", status.Steps[0].State)
	}
	if status.Steps[2].State != "blocked" {
		t.Fatalf("expected deploy step blocked, got %q", status.Steps[2].State)
	}
	if status.Steps[2].Summary != "Chưa có service nào được cấu hình. Hãy thêm ít nhất một service trong mục Dịch vụ" {
		t.Fatalf("unexpected deploy summary %q", status.Steps[2].Summary)
	}
	if len(status.Steps[2].Actions) != 1 || status.Steps[2].Actions[0].Href != "/projects/prj_123/services" {
		t.Fatalf("expected configure services action, got %#v", status.Steps[2].Actions)
	}
}

func TestBootstrapOrchestratorGetStatusIncludesRuntimeInventory(t *testing.T) {
	publicIP := "47.129.226.224"
	compiledJSON, err := json.Marshal(desiredStateRevisionCompiledRecord{
		RevisionID:          "rev_123",
		ProjectID:           "prj_123",
		ProjectSlug:         "acme-api",
		BlueprintID:         "bp_123",
		DeploymentBindingID: "bind_123",
		CommitSHA:           "abc123def456",
		ArtifactRef:         "artifact://builds/123",
		ImageRef:            "ghcr.io/lazyops/acme-api:abc123",
		TriggerKind:         "push",
		RuntimeMode:         "standalone",
		Services: []BlueprintServiceContractRecord{
			{Name: "api", Path: "apps/api", Public: true, RuntimeProfile: "service", Healthcheck: map[string]any{"path": "/healthz", "port": 8080, "protocol": "http"}},
		},
		CompatibilityPolicy: LazyopsYAMLCompatibilityPolicy{
			LocalhostRescue: true,
		},
		MagicDomainPolicy: LazyopsYAMLMagicDomainPolicy{
			Enabled:  true,
			Provider: MagicDomainProviderSSLIP,
		},
		ScaleToZeroPolicy: LazyopsYAMLScaleToZeroPolicy{
			Enabled: false,
		},
		PlacementAssignments: []PlacementAssignmentRecord{
			{ServiceName: "api", TargetID: "inst_123", TargetKind: "instance"},
		},
	})
	if err != nil {
		t.Fatalf("marshal compiled revision: %v", err)
	}

	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		PublicIP:                &publicIP,
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             bootstrapModeStandalone,
		TargetKind:              "instance",
		TargetID:                "inst_123",
		CompatibilityPolicyJSON: `{"localhost_rescue":true}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123def456",
		TriggerKind:          "push",
		Status:               RevisionStatusPromoted,
		CompiledRevisionJSON: string(compiledJSON),
		CreatedAt:            time.Date(2026, 4, 14, 8, 20, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 14, 8, 35, 0, 0, time.UTC),
	})
	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusPromoted,
		CreatedAt:  time.Date(2026, 4, 14, 8, 21, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 4, 14, 8, 35, 0, 0, time.UTC),
	})
	deploymentSvc := NewDeploymentService(projectStore, newFakeBlueprintStore(), revisionStore, deploymentStore).
		WithPublicDomainSupport(bindingStore, instanceStore)
	internalServices := newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{
		"prj_123": {{
			ID:            "isvc_123",
			ProjectID:     "prj_123",
			Kind:          "postgres",
			Alias:         "postgres",
			Protocol:      "tcp",
			LocalEndpoint: "localhost:5432",
		}},
	})

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		repoLinkStore,
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
		nil,
	).WithOneClickPipeline(nil, nil, nil, deploymentSvc, nil, nil).WithInternalServiceStore(internalServices)

	status, err := orchestrator.GetStatus("usr_123", RoleViewer, "prj_123")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}

	if status.RuntimeInventory.SyncState != "synced" {
		t.Fatalf("expected synced runtime inventory, got %q", status.RuntimeInventory.SyncState)
	}
	if status.RuntimeInventory.LiveRevision != 1 || status.RuntimeInventory.StableRevision != 1 {
		t.Fatalf("expected live/stable revision number 1, got live=%d stable=%d", status.RuntimeInventory.LiveRevision, status.RuntimeInventory.StableRevision)
	}
	if status.RuntimeInventory.AppRuntime.Status != "live" {
		t.Fatalf("expected live app runtime, got %q", status.RuntimeInventory.AppRuntime.Status)
	}
	if status.RuntimeInventory.AppRuntime.ContainerName != "lazyops-app-prj-123-bind-123-api" {
		t.Fatalf("unexpected app container %q", status.RuntimeInventory.AppRuntime.ContainerName)
	}
	if !status.RuntimeInventory.SidecarRuntime.Enabled {
		t.Fatal("expected sidecar runtime to be enabled")
	}
	if status.RuntimeInventory.SidecarRuntime.ContainerName != "lazyops-sidecar-prj-123-bind-123-api" {
		t.Fatalf("unexpected sidecar container %q", status.RuntimeInventory.SidecarRuntime.ContainerName)
	}
	if len(status.RuntimeInventory.InternalServices) != 1 {
		t.Fatalf("expected one internal runtime service, got %d", len(status.RuntimeInventory.InternalServices))
	}
	if status.RuntimeInventory.InternalServices[0].ContainerName != "lazyops-int-prj-123-bind-123-postgres" {
		t.Fatalf("unexpected internal service container %q", status.RuntimeInventory.InternalServices[0].ContainerName)
	}
	if len(status.PublicURLs) == 0 || status.PublicURLs[0] != "https://api.acme-api.47-129-226-224.sslip.io" {
		t.Fatalf("unexpected public urls %#v", status.PublicURLs)
	}
}

func TestBootstrapOrchestratorGetStatusMarksRuntimeInventoryMissingWhenNoDeployment(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             bootstrapModeStandalone,
		TargetKind:              "instance",
		TargetID:                "inst_123",
		CompatibilityPolicyJSON: `{"localhost_rescue":true}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	internalServices := newFakeProjectInternalServiceStore(map[string][]models.ProjectInternalService{
		"prj_123": {{
			ID:            "isvc_123",
			ProjectID:     "prj_123",
			Kind:          "postgres",
			Alias:         "postgres",
			Protocol:      "tcp",
			LocalEndpoint: "localhost:5432",
		}},
	})

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		repoLinkStore,
		nil,
		bindingStore,
		newFakeDeploymentStore(),
		instanceStore,
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
		nil,
	).WithInternalServiceStore(internalServices)

	status, err := orchestrator.GetStatus("usr_123", RoleViewer, "prj_123")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}

	if status.RuntimeInventory.SyncState != "missing" {
		t.Fatalf("expected missing runtime sync state, got %q", status.RuntimeInventory.SyncState)
	}
	if status.RuntimeInventory.SyncReason != "Chua dong bo du lieu runtime" {
		t.Fatalf("unexpected runtime sync reason %q", status.RuntimeInventory.SyncReason)
	}
	if status.RuntimeInventory.AppRuntime.Status != "unavailable" {
		t.Fatalf("expected unavailable app runtime, got %q", status.RuntimeInventory.AppRuntime.Status)
	}
	if len(status.RuntimeInventory.InternalServices) != 1 {
		t.Fatalf("expected internal service runtime record, got %d", len(status.RuntimeInventory.InternalServices))
	}
	if status.RuntimeInventory.InternalServices[0].Status != "configured" {
		t.Fatalf("expected configured internal service status, got %q", status.RuntimeInventory.InternalServices[0].Status)
	}
}

func TestBootstrapOrchestratorAutoBootstrapCreatesProjectRepoAndBinding(t *testing.T) {
	projectStore := newFakeProjectStore()
	projectService := NewProjectService(projectStore)
	installStore := newFakeGitHubInstallationStore(&models.GitHubInstallation{
		ID:                   "ghi_alpha",
		UserID:               "usr_123",
		GitHubInstallationID: 100,
		AccountLogin:         "lazyops",
		AccountType:          "Organization",
		ScopeJSON:            `{"repository_selection":"selected","permissions":{"contents":"read"},"repositories":[{"id":42,"name":"backend","full_name":"lazyops/backend","owner_login":"lazyops","private":true}]}`,
		InstalledAt:          time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
	})
	repoLinkStore := newFakeProjectRepoLinkStore()
	repoLinkService := NewProjectRepoLinkService(projectStore, installStore, repoLinkStore)
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore()
	bindingStore := newFakeDeploymentBindingStore()
	bindingService := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, meshStore, clusterStore)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		projectService,
		repoLinkService,
		repoLinkStore,
		bindingService,
		bindingStore,
		newFakeDeploymentStore(),
		instanceStore,
		meshStore,
		clusterStore,
		installStore,
	)

	result, err := orchestrator.AutoBootstrap(BootstrapAutoCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleViewer,
		ProjectName:     "Backend",
		RepoFullName:    "lazyops/backend",
	})
	if err != nil {
		t.Fatalf("auto bootstrap: %v", err)
	}
	if result.Status != "accepted" {
		t.Fatalf("expected accepted status, got %q", result.Status)
	}

	project, err := projectStore.GetByIDForUser("usr_123", result.ProjectID)
	if err != nil {
		t.Fatalf("load created project: %v", err)
	}
	if project == nil {
		t.Fatal("expected project to be created")
	}

	link, err := repoLinkStore.GetByProjectID(project.ID)
	if err != nil {
		t.Fatalf("load repo link: %v", err)
	}
	if link == nil {
		t.Fatal("expected repo link to be created")
	}
	if link.GitHubRepoID != 42 {
		t.Fatalf("expected repo id 42, got %d", link.GitHubRepoID)
	}

	bindings, err := bindingStore.ListByProject(project.ID)
	if err != nil {
		t.Fatalf("load deployment bindings: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected no auto binding before a k3s cluster exists, got %d", len(bindings))
	}
}

func TestBootstrapOrchestratorAutoBootstrapPrefersK3sWhenClusterHealthy(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	projectService := NewProjectService(projectStore)
	repoLinkStore := newFakeProjectRepoLinkStore()
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "prod-k3s",
		Provider:            "k3s",
		KubeconfigSecretRef: "secret://clusters/prod",
		Status:              "ready",
	})
	bindingStore := newFakeDeploymentBindingStore()
	bindingService := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, meshStore, clusterStore)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		projectService,
		nil,
		repoLinkStore,
		bindingService,
		bindingStore,
		newFakeDeploymentStore(),
		instanceStore,
		meshStore,
		clusterStore,
		nil,
	)

	_, err := orchestrator.AutoBootstrap(BootstrapAutoCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleViewer,
		ProjectID:       "prj_123",
	})
	if err != nil {
		t.Fatalf("auto bootstrap: %v", err)
	}

	bindings, err := bindingStore.ListByProject("prj_123")
	if err != nil {
		t.Fatalf("load deployment bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected one deployment binding, got %d", len(bindings))
	}
	if bindings[0].RuntimeMode != bootstrapModeDistributedK3s {
		t.Fatalf("expected distributed-k3s mode, got %q", bindings[0].RuntimeMode)
	}
	if bindings[0].TargetKind != "cluster" || bindings[0].TargetID != "cls_123" {
		t.Fatalf("expected cluster target cls_123, got kind=%q id=%q", bindings[0].TargetKind, bindings[0].TargetID)
	}
}

func TestBootstrapOrchestratorAutoBootstrapWaitsForK3sClusterWhenOnlyInstancesExist(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	projectService := NewProjectService(projectStore)
	repoLinkStore := newFakeProjectRepoLinkStore()
	instanceStore := newFakeInstanceStore(
		&models.Instance{
			ID:                      "inst_123",
			UserID:                  "usr_123",
			Name:                    "edge-a",
			Status:                  "online",
			RuntimeCapabilitiesJSON: `{}`,
			LabelsJSON:              `{}`,
		},
		&models.Instance{
			ID:                      "inst_456",
			UserID:                  "usr_123",
			Name:                    "edge-b",
			Status:                  "online",
			RuntimeCapabilitiesJSON: `{}`,
			LabelsJSON:              `{}`,
		},
	)
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore()
	bindingStore := newFakeDeploymentBindingStore()
	bindingService := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, meshStore, clusterStore)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		projectService,
		nil,
		repoLinkStore,
		bindingService,
		bindingStore,
		newFakeDeploymentStore(),
		instanceStore,
		meshStore,
		clusterStore,
		nil,
	)

	_, err := orchestrator.AutoBootstrap(BootstrapAutoCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleViewer,
		ProjectID:       "prj_123",
	})
	if err != nil {
		t.Fatalf("auto bootstrap: %v", err)
	}

	bindings, err := bindingStore.ListByProject("prj_123")
	if err != nil {
		t.Fatalf("load deployment bindings: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected no binding until cluster bootstrap completes, got %d", len(bindings))
	}
}

func TestBootstrapOrchestratorOnInventoryChangedReevaluatesAutoBinding(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "prod-k3s",
		Provider:            "k3s",
		KubeconfigSecretRef: "secret://clusters/prod",
		Status:              "ready",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             bootstrapModeStandalone,
		TargetKind:              "instance",
		TargetID:                "inst_123",
		PlacementPolicyJSON:     `{}`,
		DomainPolicyJSON:        `{}`,
		CompatibilityPolicyJSON: `{}`,
		ScaleToZeroPolicyJSON:   `{}`,
	})
	bindingService := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, meshStore, clusterStore)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		newFakeProjectRepoLinkStore(),
		bindingService,
		bindingStore,
		newFakeDeploymentStore(),
		instanceStore,
		meshStore,
		clusterStore,
		nil,
	)

	if err := orchestrator.OnInventoryChanged("usr_123"); err != nil {
		t.Fatalf("on inventory changed: %v", err)
	}

	bindings, err := bindingStore.ListByProject("prj_123")
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected one binding, got %d", len(bindings))
	}
	if bindings[0].RuntimeMode != bootstrapModeDistributedK3s {
		t.Fatalf("expected runtime mode distributed-k3s, got %q", bindings[0].RuntimeMode)
	}
	if bindings[0].TargetKind != "cluster" || bindings[0].TargetID != "cls_123" {
		t.Fatalf("expected cluster target cls_123, got kind=%q id=%q", bindings[0].TargetKind, bindings[0].TargetID)
	}
}

func TestBootstrapOrchestratorOnInventoryChangedKeepsLegacyBindingUntilClusterExists(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme",
		Slug:          "acme",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-a",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore()
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             bootstrapModeStandalone,
		TargetKind:              "instance",
		TargetID:                "inst_123",
		PlacementPolicyJSON:     `{}`,
		DomainPolicyJSON:        `{}`,
		CompatibilityPolicyJSON: `{}`,
		ScaleToZeroPolicyJSON:   `{}`,
	})
	bindingService := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, meshStore, clusterStore)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		newFakeProjectRepoLinkStore(),
		bindingService,
		bindingStore,
		newFakeDeploymentStore(),
		instanceStore,
		meshStore,
		clusterStore,
		nil,
	)

	instanceStore.byID["inst_456"] = &models.Instance{
		ID:                      "inst_456",
		UserID:                  "usr_123",
		Name:                    "edge-b",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	}
	instanceStore.byUserName["usr_123:edge-b"] = instanceStore.byID["inst_456"]

	if err := orchestrator.OnInventoryChanged("usr_123"); err != nil {
		t.Fatalf("on inventory changed: %v", err)
	}

	bindings, err := bindingStore.ListByProject("prj_123")
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("expected one binding, got %d", len(bindings))
	}
	if bindings[0].RuntimeMode != bootstrapModeStandalone {
		t.Fatalf("expected legacy standalone binding to remain until k3s cluster exists, got %q", bindings[0].RuntimeMode)
	}
	if bindings[0].TargetKind != "instance" {
		t.Fatalf("expected target kind instance, got %q", bindings[0].TargetKind)
	}
}

func TestBootstrapOrchestratorOneClickDeployRequiresConfiguredServices(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "acme-api",
		TrackedBranch:        "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore()
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             "standalone",
		TargetKind:              "instance",
		TargetID:                "inst_123",
		PlacementPolicyJSON:     `{"strategy":"single-node"}`,
		DomainPolicyJSON:        `{"mode":"auto"}`,
		CompatibilityPolicyJSON: `{"env_injection":true,"managed_credentials":false,"localhost_rescue":false}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	projectServiceStore := newFakeProjectServiceStore()
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		repoLinkStore,
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		meshStore,
		clusterStore,
		nil,
	).WithOneClickPipeline(projectServiceStore, nil, nil, deploymentSvc, nil, nil)

	result, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if !errors.Is(err, ErrNoProjectServicesConfigured) {
		t.Fatalf("expected ErrNoProjectServicesConfigured, got result=%#v err=%v", result, err)
	}
}

func TestBootstrapOrchestratorOneClickDeployAllowsInternalOnlyProjectsWithoutRepoLink(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme Internal",
		Slug:          "acme-internal",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore()
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             "standalone",
		TargetKind:              "instance",
		TargetID:                "inst_123",
		PlacementPolicyJSON:     `{"strategy":"single-node"}`,
		DomainPolicyJSON:        `{"mode":"auto"}`,
		CompatibilityPolicyJSON: `{"env_injection":true}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "standalone", []ConfigureProjectServiceItem{{
		Name:               "db",
		Kind:               "postgres",
		SourceType:         serviceSourceTypeInternal,
		ManagedByLazyops:   true,
		ConnectionTemplate: defaultPostgresConnectionTemplate(),
	}})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	projectServiceStore := newFakeProjectServiceStore()
	if err := projectServiceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	compiler := NewServiceInventoryBlueprintCompiler(newFakeProjectRepoLinkStore(), bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		newFakeProjectRepoLinkStore(),
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		meshStore,
		clusterStore,
		nil,
	).WithOneClickPipeline(projectServiceStore, nil, nil, deploymentSvc, nil, nil)

	result, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if err != nil {
		t.Fatalf("one-click deploy: %v", err)
	}
	if result.ProjectID != "prj_123" || result.BlueprintID == "" || result.DeploymentID == "" {
		t.Fatalf("unexpected one-click result %#v", result)
	}
	storedBlueprint, err := blueprintStore.GetByIDForProject("prj_123", result.BlueprintID)
	if err != nil {
		t.Fatalf("load stored blueprint: %v", err)
	}
	if storedBlueprint == nil {
		t.Fatal("expected stored blueprint")
	}
	blueprintRecord, err := ToBlueprintRecord(*storedBlueprint)
	if err != nil {
		t.Fatalf("decode blueprint record: %v", err)
	}
	if len(blueprintRecord.Compiled.Services) != 1 || blueprintRecord.Compiled.Services[0].Name != "db" {
		t.Fatalf("expected internal db service in blueprint, got %#v", blueprintRecord.Compiled.Services)
	}
	if blueprintRecord.SourceRef != "service_inventory" {
		t.Fatalf("expected service_inventory source ref, got %q", blueprintRecord.SourceRef)
	}
}

func TestBootstrapOrchestratorOneClickDeployMarksDeploymentFailedWhenRolloutCannotStart(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme Internal",
		Slug:          "acme-internal",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             "standalone",
		TargetKind:              "instance",
		TargetID:                "inst_123",
		PlacementPolicyJSON:     `{"strategy":"single-node"}`,
		DomainPolicyJSON:        `{"mode":"auto"}`,
		CompatibilityPolicyJSON: `{"env_injection":true}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "standalone", []ConfigureProjectServiceItem{{
		Name:               "db",
		Kind:               "postgres",
		SourceType:         serviceSourceTypeInternal,
		ManagedByLazyops:   true,
		ConnectionTemplate: defaultPostgresConnectionTemplate(),
	}})
	if err != nil {
		t.Fatalf("build configured service models: %v", err)
	}
	projectServiceStore := newFakeProjectServiceStore()
	if err := projectServiceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	compiler := NewServiceInventoryBlueprintCompiler(newFakeProjectRepoLinkStore(), bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		newFakeProjectRepoLinkStore(),
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
		nil,
	).WithOneClickPipeline(projectServiceStore, nil, nil, deploymentSvc, &RolloutExecutionService{}, nil)

	result, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if err != nil {
		t.Fatalf("one-click deploy: %v", err)
	}
	if result.RolloutStatus != "failed_to_start" {
		t.Fatalf("expected failed_to_start rollout status, got %#v", result)
	}

	storedDeployment, err := deploymentStore.GetByIDForProject("prj_123", result.DeploymentID)
	if err != nil {
		t.Fatalf("load deployment: %v", err)
	}
	if storedDeployment == nil || storedDeployment.Status != DeploymentStatusFailed {
		t.Fatalf("expected deployment to be marked failed, got %#v", storedDeployment)
	}

	storedRevision, err := revisionStore.GetByIDForProject("prj_123", result.RevisionID)
	if err != nil {
		t.Fatalf("load revision: %v", err)
	}
	if storedRevision == nil || storedRevision.Status != RevisionStatusFailed {
		t.Fatalf("expected revision to be marked failed, got %#v", storedRevision)
	}
}

func TestBootstrapOrchestratorOneClickDeployRequiresRepoLink(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	meshStore := newFakeMeshNetworkStore()
	clusterStore := newFakeClusterStore()
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto standalone",
		TargetRef:               "auto-primary",
		RuntimeMode:             "standalone",
		TargetKind:              "instance",
		TargetID:                "inst_123",
		PlacementPolicyJSON:     `{"strategy":"single-node"}`,
		DomainPolicyJSON:        `{"mode":"auto"}`,
		CompatibilityPolicyJSON: `{"env_injection":true}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "standalone", []ConfigureProjectServiceItem{{
		Name:   "api",
		Path:   "apps/api",
		Kind:   "app",
		Public: true,
	}})
	if err != nil {
		t.Fatalf("build configured services: %v", err)
	}
	projectServiceStore := newFakeProjectServiceStore()
	if err := projectServiceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	compiler := NewServiceInventoryBlueprintCompiler(newFakeProjectRepoLinkStore(), bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		newFakeProjectRepoLinkStore(),
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		meshStore,
		clusterStore,
		nil,
	).WithOneClickPipeline(projectServiceStore, nil, nil, deploymentSvc, nil, nil)

	_, err = orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if !errors.Is(err, ErrRepoLinkNotFound) {
		t.Fatalf("expected ErrRepoLinkNotFound, got %v", err)
	}
}

func TestBootstrapOrchestratorOneClickDeployQueuesBuildForRepoK3sServiceWithoutImageOverride(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	installStore := newFakeGitHubInstallationStore(&models.GitHubInstallation{
		ID:                   "ghi_alpha",
		UserID:               "usr_123",
		GitHubInstallationID: 100,
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto distributed k3s",
		TargetRef:               "auto-primary",
		RuntimeMode:             "distributed-k3s",
		TargetKind:              "cluster",
		TargetID:                "cls_123",
		PlacementPolicyJSON:     `{"strategy":"spread"}`,
		DomainPolicyJSON:        `{"mode":"auto"}`,
		CompatibilityPolicyJSON: `{"env_injection":false}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{{
		Name:   "be",
		Path:   "backend",
		Kind:   "api",
		Public: true,
	}})
	if err != nil {
		t.Fatalf("build configured services: %v", err)
	}
	projectServiceStore := newFakeProjectServiceStore()
	if err := projectServiceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()
	buildStore := newFakeBuildJobStore()

	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)
	buildJobSvc := NewBuildJobService(repoLinkStore, buildStore).WithServiceStore(projectServiceStore)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		repoLinkStore,
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
		installStore,
	).WithOneClickPipeline(projectServiceStore, nil, nil, deploymentSvc, nil, buildJobSvc)

	result, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if err != nil {
		t.Fatalf("expected build to be queued, got result=%#v err=%v", result, err)
	}
	if result == nil || result.BuildJobID == "" || result.BuildJobStatus != BuildJobStatusQueued {
		t.Fatalf("expected queued build job, got %#v", result)
	}
	if result.DeploymentID != "" || result.RevisionID != "" {
		t.Fatalf("expected no deployment or revision before build callback, got %#v", result)
	}
	if result.RolloutStatus != "build_queued" {
		t.Fatalf("expected build_queued rollout status, got %#v", result)
	}
	storedJob := buildStore.byProjectID["prj_123"][result.BuildJobID]
	if storedJob == nil {
		t.Fatalf("expected persisted build job %q", result.BuildJobID)
	}
	if storedJob.GitHubInstallationID != 100 {
		t.Fatalf("expected GitHub installation id 100, got %#v", storedJob)
	}
	if storedJob.TrackedBranch != "main" || storedJob.CommitSHA != "" {
		t.Fatalf("expected tracked branch main and unresolved commit sha, got %#v", storedJob)
	}
	if len(blueprintStore.items) != 0 {
		t.Fatalf("expected no blueprints to be created, got %#v", blueprintStore.items)
	}
	deployments, listErr := deploymentStore.ListByProject("prj_123")
	if listErr != nil {
		t.Fatalf("list deployments: %v", listErr)
	}
	if len(deployments) != 0 {
		t.Fatalf("expected no deployments to be created, got %#v", deployments)
	}
}

func TestBootstrapOrchestratorOneClickDeployRejectsDirectRepoK3sDeployWithoutResolvedPortWhenImageOverrideProvided(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto distributed k3s",
		TargetRef:               "auto-primary",
		RuntimeMode:             "distributed-k3s",
		TargetKind:              "cluster",
		TargetID:                "cls_123",
		PlacementPolicyJSON:     `{"strategy":"spread"}`,
		DomainPolicyJSON:        `{"mode":"auto"}`,
		CompatibilityPolicyJSON: `{"env_injection":false}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{{
		Name:   "be",
		Path:   "backend",
		Kind:   "api",
		Public: true,
	}})
	if err != nil {
		t.Fatalf("build configured services: %v", err)
	}
	projectServiceStore := newFakeProjectServiceStore()
	if err := projectServiceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		repoLinkStore,
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
		nil,
	).WithOneClickPipeline(projectServiceStore, nil, nil, deploymentSvc, nil, nil)

	result, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		ImageRef:        "ghcr.io/lazyops/be:rev_123",
	})
	if !errors.Is(err, ErrOneClickRepoServicesNotReady) {
		t.Fatalf("expected ErrOneClickRepoServicesNotReady, got result=%#v err=%v", result, err)
	}
	if !strings.Contains(err.Error(), `service "be" requires target_port/service_port or healthcheck.port`) {
		t.Fatalf("expected unresolved port detail, got %v", err)
	}
}

func TestBootstrapOrchestratorOneClickDeployAllowsDirectRepoK3sDeployWithExplicitImageOverrideAndHealthcheckPort(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:                   "prl_123",
		ProjectID:            "prj_123",
		GitHubInstallationID: "ghi_alpha",
		GitHubRepoID:         42,
		RepoOwner:            "lazyops",
		RepoName:             "backend",
		TrackedBranch:        "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-hcm",
		Status:                  "online",
		RuntimeCapabilitiesJSON: `{}`,
		LabelsJSON:              `{}`,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:                      "bind_123",
		ProjectID:               "prj_123",
		Name:                    "Auto distributed k3s",
		TargetRef:               "auto-primary",
		RuntimeMode:             "distributed-k3s",
		TargetKind:              "cluster",
		TargetID:                "cls_123",
		PlacementPolicyJSON:     `{"strategy":"spread"}`,
		DomainPolicyJSON:        `{"mode":"auto"}`,
		CompatibilityPolicyJSON: `{"env_injection":false}`,
		ScaleToZeroPolicyJSON:   `{"enabled":false}`,
	})
	serviceModels, err := buildConfiguredProjectServiceModels("prj_123", "distributed-k3s", []ConfigureProjectServiceItem{{
		Name:   "be",
		Path:   "backend",
		Kind:   "api",
		Public: true,
		Healthcheck: map[string]any{
			"path": "/healthz",
			"port": 8080,
		},
	}})
	if err != nil {
		t.Fatalf("build configured services: %v", err)
	}
	projectServiceStore := newFakeProjectServiceStore()
	if err := projectServiceStore.ReplaceForProject("prj_123", serviceModels); err != nil {
		t.Fatalf("seed service store: %v", err)
	}
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	compiler := NewServiceInventoryBlueprintCompiler(repoLinkStore, bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore).
		WithServiceInventoryCompiler(compiler)

	orchestrator := NewBootstrapOrchestrator(
		projectStore,
		NewProjectService(projectStore),
		nil,
		repoLinkStore,
		nil,
		bindingStore,
		deploymentStore,
		instanceStore,
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
		nil,
	).WithOneClickPipeline(projectServiceStore, nil, nil, deploymentSvc, nil, nil)

	result, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		ImageRef:        "ghcr.io/lazyops/be:rev_123",
	})
	if err != nil {
		t.Fatalf("one-click deploy: %v", err)
	}
	if result == nil || result.DeploymentID == "" || result.RevisionID == "" {
		t.Fatalf("expected created deployment, got %#v", result)
	}
}
