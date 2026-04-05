package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/internal/runtime"
)

func TestDay30SecurityMatrixAuthOwnershipIsolation(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_a",
		UserID:        "usr_a",
		Name:          "Project A",
		Slug:          "project-a",
		DefaultBranch: "main",
	})

	project, err := projectStore.GetByIDForUser("usr_b", "prj_a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project != nil {
		t.Fatal("user B should not see project A")
	}
}

func TestDay30AcceptanceMatrixRollbackToStableRevision(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	promotedRevisionJSON := mustCompiledRevisionJSON(t, "rev_stable", "bp_123", "prj_123")

	revisionStore := newFakeDesiredStateRevisionStore(
		&models.DesiredStateRevision{
			ID:                   "rev_stable",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "stable123",
			TriggerKind:          "push",
			Status:               RevisionStatusPromoted,
			CompiledRevisionJSON: promotedRevisionJSON,
			CreatedAt:            time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC),
		},
		&models.DesiredStateRevision{
			ID:                   "rev_failing",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "failing456",
			TriggerKind:          "push",
			Status:               RevisionStatusApplying,
			CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_failing", "bp_123", "prj_123"),
			CreatedAt:            time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC),
		},
	)

	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_failing",
		Status:     DeploymentStatusRunning,
	})

	incidentStore := newFakeRuntimeIncidentStore()

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		deploymentStore,
		incidentStore,
		newFakeDeploymentBindingStore(&models.DeploymentBinding{
			ID:          "bind_123",
			ProjectID:   "prj_123",
			Name:        "Production",
			TargetRef:   "prod-main",
			RuntimeMode: "standalone",
			TargetKind:  "instance",
			TargetID:    "inst_123",
		}),
		&fakeOperatorEventBroadcaster{},
	)

	result, err := planner.RollbackDeployment(context.Background(), "prj_123", "dep_123")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if result.RolledBackTo != "rev_stable" {
		t.Fatalf("expected rollback to rev_stable, got %q", result.RolledBackTo)
	}
	if result.CommitSHA != "stable123" {
		t.Fatalf("expected commit stable123, got %q", result.CommitSHA)
	}
}

func TestDay30AcceptanceMatrixPreviewCleanupIdempotency(t *testing.T) {
	previewStore := newFakePreviewEnvironmentStore(models.PreviewEnvironment{
		ID:            "prev_123",
		ProjectID:     "prj_123",
		PRNumber:      42,
		Status:        PreviewStatusDestroyed,
		DestroyedAt:   ptrTime(time.Now().Add(-1 * time.Hour)),
		DestroyReason: "PR merged",
		DomainJSON:    `[]`,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		newFakeProjectRepoLinkStore(),
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		previewStore,
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	result, err := svc.DestroyPreviewByPR(context.Background(), "prj_123", 42, "cleanup")
	if err != nil {
		t.Fatalf("destroy preview by PR (idempotent): %v", err)
	}

	if result.Status != PreviewStatusDestroyed {
		t.Fatalf("expected status destroyed, got %q", result.Status)
	}
}

func TestDay30AcceptanceMatrixMeshFailureOfflineTarget(t *testing.T) {
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_offline",
		UserID:     "usr_123",
		Name:       "edge-sg-2",
		PublicIP:   ptrString("203.0.113.11"),
		PrivateIP:  ptrString("10.0.1.6"),
		Status:     "offline",
		LabelsJSON: `{"services":["db"]}`,
	})

	topologyStore := newFakeTopologyStateStore(models.TopologyState{
		ID:           "topo_offline",
		InstanceID:   "inst_offline",
		MeshID:       "mesh_123",
		State:        TopologyStateOffline,
		MetadataJSON: `{}`,
		LastSeenAt:   time.Now().UTC().Add(-1 * time.Hour),
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		TargetRef:  "main",
		TargetKind: "instance",
		TargetID:   "inst_offline",
	})

	svc := newTestMeshPlanningService(
		instanceStore,
		bindingStore,
		newFakeDesiredStateRevisionStore(),
		newFakeTunnelSessionStore(),
		topologyStore,
	)

	_, err := svc.ResolveDependencyBinding(context.Background(), "prj_123", "api", LazyopsYAMLDependencyBinding{
		Service:       "api",
		Alias:         "db",
		TargetService: "db",
		Protocol:      "tcp",
		LocalEndpoint: "localhost:5432",
	})
	if err == nil {
		t.Fatal("expected error when target is offline")
	}
	if !strings.Contains(err.Error(), "no online instance found") && !errors.Is(err, ErrTargetOffline) {
		t.Fatalf("expected offline target error, got %v", err)
	}
}

func TestDay30AcceptanceMatrixK3sBoundaryEnforcement(t *testing.T) {
	svc := newTestK3sClusterService(
		newFakeClusterStore(),
		newFakeTopologyStateStore(),
	)

	forbiddenCommands := []string{
		"docker_run", "docker_stop", "docker_rm",
		"direct_deploy", "process_start", "process_stop",
		"file_deploy", "systemctl_start", "systemctl_stop",
	}

	for _, cmdType := range forbiddenCommands {
		err := svc.EnforceK3sBoundary(cmdType)
		if err == nil {
			t.Fatalf("expected error for forbidden command %q, got nil", cmdType)
		}
		if !errors.Is(err, ErrK3sBoundaryViolation) {
			t.Fatalf("expected ErrK3sBoundaryViolation for %q, got %v", cmdType, err)
		}
	}
}

func TestDay30AcceptanceMatrixFinOpsRejectsRawSamples(t *testing.T) {
	svc := newTestFinOpsService(
		newFakeMetricRollupStore(),
		newFakeScaleToZeroStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
	)

	_, err := svc.IngestMetricRollup(context.Background(), IngestMetricRollupCommand{
		ProjectID:   "prj_123",
		ServiceName: "api",
		MetricKind:  MetricKindCPU,
		WindowStart: time.Now().Add(-1 * time.Hour),
		WindowEnd:   time.Now(),
		P95:         75.5,
		Max:         95.0,
		Min:         10.0,
		Avg:         45.2,
		Count:       1,
		IsRawSample: true,
	})
	if !errors.Is(err, ErrRawMetricRejected) {
		t.Fatalf("expected ErrRawMetricRejected, got %v", err)
	}
}

func TestDay30AcceptanceMatrixObservabilityCorrelationIDPropagation(t *testing.T) {
	traceStore := newFakeTraceSummaryStore()

	svc := newTestObservabilityService(
		traceStore,
		newFakeRuntimeIncidentStore(),
		newFakeLogStreamStore(),
		newFakeTopologyNodeStore(),
		newFakeTopologyEdgeStore(),
		newFakeInstanceStore(),
		newFakeMeshNetworkStore(),
		newFakeClusterStore(),
	)

	_, err := svc.IngestTraceSummary(context.Background(), IngestTraceCommand{
		CorrelationID:  "corr_test_propagation",
		ProjectID:      "prj_123",
		ServiceName:    "api",
		Operation:      "GET /health",
		HTTPMethod:     "GET",
		HTTPStatusCode: 200,
		DurationMs:     5.0,
		Status:         "ok",
		SpanCount:      1,
		Metadata:       map[string]any{"x_correlation_id": "corr_test_propagation"},
	})
	if err != nil {
		t.Fatalf("ingest trace: %v", err)
	}

	record, err := svc.GetTraceByCorrelationID(context.Background(), "corr_test_propagation")
	if err != nil {
		t.Fatalf("get trace: %v", err)
	}

	if record.Metadata["x_correlation_id"] != "corr_test_propagation" {
		t.Fatalf("expected correlation id in metadata, got %v", record.Metadata["x_correlation_id"])
	}
}

func TestDay30AcceptanceMatrixDeployContractValidation(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))
	revisionStore := newFakeDesiredStateRevisionStore()
	deploymentStore := newFakeDeploymentStore()

	svc := NewDeploymentService(projectStore, blueprintStore, revisionStore, deploymentStore)

	_, err := svc.Create(CreateDeploymentCommand{
		RequesterUserID: "usr_other",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		BlueprintID:     "bp_123",
	})
	if !errors.Is(err, ErrProjectAccessDenied) {
		t.Fatalf("expected ErrProjectAccessDenied for ownership mismatch, got %v", err)
	}
}

func TestDay30AcceptanceMatrixStandaloneRollout(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})

	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusRunning,
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: "standalone",
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})

	broadcaster := &fakeOperatorEventBroadcaster{}

	planner := newTestRolloutPlanner(
		registry,
		revisionStore,
		deploymentStore,
		newFakeRuntimeIncidentStore(),
		bindingStore,
		broadcaster,
	)

	plan, err := planner.PlanCandidate(context.Background(), "prj_123", "rev_123")
	if err != nil {
		t.Fatalf("plan candidate: %v", err)
	}

	if plan.RuntimeMode != runtime.RuntimeModeStandalone {
		t.Fatalf("expected runtime mode standalone, got %q", plan.RuntimeMode)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected at least one rollout step")
	}

	promotionResult, err := planner.PromoteCandidate(context.Background(), "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("promote candidate: %v", err)
	}
	if promotionResult.RevisionID != "rev_123" {
		t.Fatalf("expected revision id rev_123, got %q", promotionResult.RevisionID)
	}
}

func TestDay30AcceptanceMatrixBuildCallbackSuccess(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})
	blueprintStore := newFakeBlueprintStore()
	blueprintStore.items = append(blueprintStore.items, mustBlueprintModel(t, "bp_123", "prj_123"))
	revisionStore := newFakeDesiredStateRevisionStore()
	buildStore := newFakeBuildJobStore(&models.BuildJob{
		ID:                   "bld_123",
		ProjectID:            "prj_123",
		ProjectRepoLinkID:    "prl_123",
		GitHubDeliveryID:     "delivery_123",
		GitHubInstallationID: 100,
		GitHubRepoID:         42,
		RepoFullName:         "lazyops/backend",
		TriggerKind:          "push",
		Status:               BuildJobStatusQueued,
		CommitSHA:            "abc123def456",
		TrackedBranch:        "main",
		WorkerInputJSON:      `{"build_job_id":"bld_123","project_id":"prj_123","project_repo_link_id":"prl_123","github_delivery_id":"delivery_123","github_installation_id":100,"github_repo_id":42,"repo_owner":"lazyops","repo_name":"backend","repo_full_name":"lazyops/backend","tracked_branch":"main","commit_sha":"abc123def456","trigger_kind":"push","preview_enabled":false,"artifact_metadata_stage":{"commit_sha":"abc123def456"},"retry_policy":{"max_attempts":3,"backoff":"linear"},"callback_expectation":{"path":"/api/v1/builds/callback","required_fields":["build_job_id","project_id","commit_sha","status","image_ref","image_digest","metadata.detected_services"]}}`,
		ArtifactMetadataJSON: `{"commit_sha":"abc123def456"}`,
	})

	svc := NewBuildCallbackService(projectStore, blueprintStore, revisionStore, buildStore, nil)

	result, err := svc.Handle(BuildCallbackCommand{
		BuildJobID:       "bld_123",
		ProjectID:        "prj_123",
		CommitSHA:        "abc123def456",
		Status:           "succeeded",
		ImageRef:         "ghcr.io/lazyops/backend:abc123",
		ImageDigest:      "sha256:deadbeef",
		DetectedServices: []string{"api", "web"},
	})
	if err != nil {
		t.Fatalf("build callback: %v", err)
	}
	if result.BuildJob.Status != BuildJobStatusSucceeded {
		t.Fatalf("expected status succeeded, got %q", result.BuildJob.Status)
	}
	if result.Revision == nil {
		t.Fatal("expected artifact-ready revision")
	}
}

func TestDay30AcceptanceMatrixPreviewNotEnabledBlocksCreation(t *testing.T) {
	repoLinkStore := newFakeProjectRepoLinkStore(&models.ProjectRepoLink{
		ID:             "prl_123",
		ProjectID:      "prj_123",
		RepoOwner:      "lazyops",
		RepoName:       "acme-api",
		TrackedBranch:  "main",
		PreviewEnabled: false,
	})

	svc := newTestPreviewService(
		newFakeProjectStore(),
		repoLinkStore,
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeBlueprintStore(),
		newFakePreviewEnvironmentStore(),
		newFakePublicRouteStore(),
		&fakeOperatorEventBroadcaster{},
	)

	_, err := svc.CreateFromPR(context.Background(), "prj_123", 42, "Test", "dev", "abc123", "main")
	if !errors.Is(err, ErrPreviewNotEnabled) {
		t.Fatalf("expected ErrPreviewNotEnabled, got %v", err)
	}
}

func TestDay30AcceptanceMatrixGatewayMagicDomainRejectsPrivateIP(t *testing.T) {
	svc := newTestGatewayConfigService(
		newFakeDesiredStateRevisionStore(),
		newFakeDeploymentStore(),
		newFakeDeploymentBindingStore(),
		newFakePublicRouteStore(),
		newFakeGatewayConfigIntentStore(),
		newFakeReleaseHistoryStore(),
 nil,
	)

	privateIPs := []string{"192.168.1.1", "10.0.0.1", "172.16.0.1", "127.0.0.1"}
	for _, ip := range privateIPs {
		_, err := svc.AllocateMagicDomain("prj_123", "api", ip, "")
		if err == nil {
			t.Fatalf("expected error for private IP %q, got nil", ip)
		}
	}
}

func TestDay30AcceptanceMatrixRuntimeDriverRegistry(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())
	registry.Register(runtime.NewDistributedMeshDriver())
	registry.Register(runtime.NewDistributedK3sDriver())

	if len(registry.List()) != 3 {
		t.Fatalf("expected 3 drivers, got %d", len(registry.List()))
	}

	for _, mode := range []string{runtime.RuntimeModeStandalone, runtime.RuntimeModeDistributedMesh, runtime.RuntimeModeDistributedK3s} {
		driver, err := registry.Get(mode)
		if err != nil {
			t.Fatalf("get driver for %q: %v", mode, err)
		}
		if driver.Mode() != mode {
			t.Fatalf("expected mode %q, got %q", mode, driver.Mode())
		}
	}
}

func TestDay30AcceptanceMatrixControlHubDispatch(t *testing.T) {
	hub := NewControlHub()
	hub.Start()

	conn := &mockControlConn{}
	client := &ControlClient{
		AgentID:    "agent_123",
		InstanceID: "inst_123",
		Conn:       conn,
		Registered: time.Now().UTC(),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	if !hub.IsConnected("agent_123") {
		t.Fatal("expected agent to be connected")
	}

	payload := map[string]any{"type": "reconcile_revision", "project_id": "prj_123"}
	if err := hub.SendToAgent("agent_123", payload); err != nil {
		t.Fatalf("send to agent: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	written := conn.lastWritten()
	if written == nil {
		t.Fatal("expected message to be written")
	}

	var decoded map[string]any
	if err := json.Unmarshal(written, &decoded); err != nil {
		t.Fatalf("decode written message: %v", err)
	}
	if decoded["type"] != "reconcile_revision" {
		t.Fatalf("expected type reconcile_revision, got %v", decoded["type"])
	}
}
