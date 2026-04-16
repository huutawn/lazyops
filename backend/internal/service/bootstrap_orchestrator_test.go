package service

import (
	"encoding/json"
	"errors"
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
		newFakeClusterStore(),
		nil,
	)

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
		t.Fatalf("expected connect_infra ready, got %q", status.Steps[1].State)
	}
	if status.Steps[2].State != "ready" {
		t.Fatalf("expected deploy ready, got %q", status.Steps[2].State)
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
	).WithOneClickPipeline(nil, nil, nil, deploymentSvc, nil).WithInternalServiceStore(internalServices)

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
	if len(bindings) != 1 {
		t.Fatalf("expected one deployment binding, got %d", len(bindings))
	}
	if bindings[0].RuntimeMode != bootstrapModeStandalone {
		t.Fatalf("expected standalone mode, got %q", bindings[0].RuntimeMode)
	}
	if bindings[0].TargetKind != "instance" || bindings[0].TargetID != "inst_123" {
		t.Fatalf("expected instance binding to inst_123, got kind=%q id=%q", bindings[0].TargetKind, bindings[0].TargetID)
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

func TestBootstrapOrchestratorAutoBootstrapPrefersMeshWhenTwoInstancesHealthy(t *testing.T) {
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
	if len(bindings) != 1 {
		t.Fatalf("expected one deployment binding, got %d", len(bindings))
	}
	if bindings[0].RuntimeMode != bootstrapModeDistributedMesh {
		t.Fatalf("expected distributed-mesh mode, got %q", bindings[0].RuntimeMode)
	}
	if bindings[0].TargetKind != "instance" {
		t.Fatalf("expected instance target for distributed-mesh fallback, got %q", bindings[0].TargetKind)
	}
	if bindings[0].TargetID == "" {
		t.Fatal("expected target id to be set")
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

func TestBootstrapOrchestratorOnInventoryChangedUpshiftsToMeshWhenSecondInstanceAdded(t *testing.T) {
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
	if bindings[0].RuntimeMode != bootstrapModeDistributedMesh {
		t.Fatalf("expected runtime mode distributed-mesh, got %q", bindings[0].RuntimeMode)
	}
	if bindings[0].TargetKind != "instance" {
		t.Fatalf("expected target kind instance, got %q", bindings[0].TargetKind)
	}
}

func TestBootstrapOrchestratorOneClickDeployCreatesDeploymentWithDefaultService(t *testing.T) {
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

	initContracts := NewInitContractService(projectStore, bindingStore, instanceStore, meshStore, clusterStore)
	blueprintSvc := NewBlueprintService(projectStore, repoLinkStore, bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore)

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
	).WithOneClickPipeline(projectServiceStore, initContracts, blueprintSvc, deploymentSvc, nil)

	result, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if err != nil {
		t.Fatalf("one-click deploy: %v", err)
	}
	if result.ProjectID != "prj_123" {
		t.Fatalf("expected project prj_123, got %q", result.ProjectID)
	}
	if result.BlueprintID == "" || result.DeploymentID == "" || result.RevisionID == "" {
		t.Fatalf("expected blueprint/deployment/revision ids, got %#v", result)
	}
	if result.RolloutStatus != "not_started" {
		t.Fatalf("expected rollout status not_started, got %q", result.RolloutStatus)
	}
	if len(result.Timeline) < 4 {
		t.Fatalf("expected at least 4 timeline events, got %d", len(result.Timeline))
	}
	if result.Timeline[0].ID != "validate" || result.Timeline[0].State != "completed" {
		t.Fatalf("expected completed validate step, got %#v", result.Timeline[0])
	}
	if result.Timeline[1].ID != "compile" || result.Timeline[1].State != "completed" {
		t.Fatalf("expected completed compile step, got %#v", result.Timeline[1])
	}
	if result.Timeline[2].ID != "create_deployment" || result.Timeline[2].State != "completed" {
		t.Fatalf("expected completed create_deployment step, got %#v", result.Timeline[2])
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
	if len(blueprintRecord.Compiled.Services) != 1 {
		t.Fatalf("expected one default service, got %d", len(blueprintRecord.Compiled.Services))
	}
	if blueprintRecord.Compiled.Services[0].Name != "app" || blueprintRecord.Compiled.Services[0].Path != "." {
		t.Fatalf("expected default app service, got %#v", blueprintRecord.Compiled.Services[0])
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
	projectServiceStore := newFakeProjectServiceStore()
	blueprintStore := newFakeBlueprintStore()
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	initContracts := NewInitContractService(projectStore, bindingStore, instanceStore, meshStore, clusterStore)
	blueprintSvc := NewBlueprintService(projectStore, newFakeProjectRepoLinkStore(), bindingStore, projectServiceStore, blueprintStore)
	deploymentSvc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore)

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
	).WithOneClickPipeline(projectServiceStore, initContracts, blueprintSvc, deploymentSvc, nil)

	_, err := orchestrator.OneClickDeploy(BootstrapOneClickDeployCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if !errors.Is(err, ErrRepoLinkNotFound) {
		t.Fatalf("expected ErrRepoLinkNotFound, got %v", err)
	}
}
