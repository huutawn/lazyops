package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/internal/runtime"
)

type fakeRolloutDispatcher struct {
	commands          []runtime.AgentCommand
	agentIDs          []string
	err               error
	requestSeq        int
	requestToCommand  map[string]runtime.AgentCommand
	waitResultsByType map[string][]*TrackedCommand
	waitErrorsByType  map[string][]error
}

func (f *fakeRolloutDispatcher) DispatchCommand(_ context.Context, agentID string, cmd runtime.AgentCommand) (*runtime.CommandResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.requestToCommand == nil {
		f.requestToCommand = make(map[string]runtime.AgentCommand)
	}
	if cmd.RequestID == "" {
		f.requestSeq++
		cmd.RequestID = fmt.Sprintf("req_%d", f.requestSeq)
	}
	f.agentIDs = append(f.agentIDs, agentID)
	f.commands = append(f.commands, cmd)
	f.requestToCommand[cmd.RequestID] = cmd
	return &runtime.CommandResult{RequestID: cmd.RequestID, Status: "dispatched"}, nil
}

func (f *fakeRolloutDispatcher) WaitForCommand(_ context.Context, requestID string) (*TrackedCommand, error) {
	if f.requestToCommand != nil {
		if cmd, ok := f.requestToCommand[requestID]; ok {
			if len(f.waitErrorsByType[cmd.Type]) > 0 {
				err := f.waitErrorsByType[cmd.Type][0]
				f.waitErrorsByType[cmd.Type] = f.waitErrorsByType[cmd.Type][1:]
				if err != nil {
					return nil, err
				}
			}
			if len(f.waitResultsByType[cmd.Type]) > 0 {
				result := *f.waitResultsByType[cmd.Type][0]
				f.waitResultsByType[cmd.Type] = f.waitResultsByType[cmd.Type][1:]
				result.RequestID = requestID
				return &result, nil
			}
		}
	}
	return &TrackedCommand{RequestID: requestID, State: CommandStateDone}, nil
}

func dispatchedTypes(commands []runtime.AgentCommand) []string {
	out := make([]string, 0, len(commands))
	for _, command := range commands {
		out = append(out, command.Type)
	}
	return out
}

func TestRolloutExecutionServiceStartDeploymentHappyPath(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "manual",
		Status:               RevisionStatusArtifactReady,
		CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
	})
	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusQueued,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: runtime.RuntimeModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:      "inst_123",
		UserID:  "usr_123",
		Name:    "edge-1",
		Status:  "online",
		AgentID: ptrString("agt_123"),
	})
	incidentStore := newFakeRuntimeIncidentStore()
	broadcaster := &fakeOperatorEventBroadcaster{}
	dispatcher := &fakeRolloutDispatcher{}

	deployments := NewDeploymentService(newFakeProjectStore(), newFakeBlueprintStore(), revisionStore, deploymentStore)
	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, incidentStore, bindingStore, broadcaster)
	service := NewRolloutExecutionService(deployments, planner, instanceStore, dispatcher, broadcaster)

	result, err := service.StartDeployment(context.Background(), "prj_123", "dep_123")
	if err != nil {
		t.Fatalf("start deployment: %v", err)
	}

	expected := []string{
		runtime.CommandTypePrepareReleaseWorkspace,
		runtime.CommandTypeRenderSidecars,
		runtime.CommandTypeRenderGatewayConfig,
		runtime.CommandTypeReconcileRevision,
		runtime.CommandTypeProvisionInternalSvc,
		runtime.CommandTypeStartReleaseCandidate,
		runtime.CommandTypeRunHealthGate,
		runtime.CommandTypePromoteRelease,
		runtime.CommandTypeGarbageCollectRuntime,
	}
	if got := dispatchedTypes(dispatcher.commands); len(got) != len(expected) {
		t.Fatalf("expected %d commands, got %d (%v)", len(expected), len(got), got)
	}
	for index, commandType := range expected {
		if dispatcher.commands[index].Type != commandType {
			t.Fatalf("expected command %d to be %q, got %q", index, commandType, dispatcher.commands[index].Type)
		}
	}
	if result.Promotion == nil {
		t.Fatal("expected promotion result")
	}

	deployment, _ := deploymentStore.GetByIDForProject("prj_123", "dep_123")
	if deployment.Status != DeploymentStatusPromoted {
		t.Fatalf("expected deployment promoted, got %q", deployment.Status)
	}
	revision, _ := revisionStore.GetByIDForProject("prj_123", "rev_123")
	if revision.Status != RevisionStatusPromoted {
		t.Fatalf("expected revision promoted, got %q", revision.Status)
	}
	if len(broadcaster.events) < 3 {
		t.Fatalf("expected deployment.started, candidate_ready, and deployment.promoted events, got %d", len(broadcaster.events))
	}
}

func TestRolloutExecutionServiceRollbacksFailedHealthGate(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(
		&models.DesiredStateRevision{
			ID:                   "rev_stable",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "stable123",
			TriggerKind:          "push",
			Status:               RevisionStatusPromoted,
			CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_stable", "bp_123", "prj_123"),
			CreatedAt:            time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC),
		},
		&models.DesiredStateRevision{
			ID:                   "rev_123",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "abc123",
			TriggerKind:          "manual",
			Status:               RevisionStatusArtifactReady,
			CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
			CreatedAt:            time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC),
		},
	)
	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusQueued,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: runtime.RuntimeModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:      "inst_123",
		UserID:  "usr_123",
		Name:    "edge-1",
		Status:  "online",
		AgentID: ptrString("agt_123"),
	})
	incidentStore := newFakeRuntimeIncidentStore()
	broadcaster := &fakeOperatorEventBroadcaster{}
	dispatcher := &fakeRolloutDispatcher{}

	deployments := NewDeploymentService(newFakeProjectStore(), newFakeBlueprintStore(), revisionStore, deploymentStore)
	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, incidentStore, bindingStore, broadcaster)
	service := NewRolloutExecutionService(deployments, planner, instanceStore, dispatcher, broadcaster).
		WithHealthGateEvaluator(func(ctx context.Context, projectID, deploymentID, revisionID string) (*HealthGateResult, error) {
			return &HealthGateResult{
				RevisionID:   revisionID,
				DeploymentID: deploymentID,
				Passed:       false,
				Services: []ServiceHealthResult{
					{ServiceName: "api", Healthy: false, Message: "connection refused"},
				},
			}, nil
		})

	result, err := service.StartDeployment(context.Background(), "prj_123", "dep_123")
	if !errors.Is(err, ErrHealthGateFailed) {
		t.Fatalf("expected ErrHealthGateFailed, got %v", err)
	}
	if result.Rollback == nil {
		t.Fatal("expected rollback result")
	}

	expected := []string{
		runtime.CommandTypePrepareReleaseWorkspace,
		runtime.CommandTypeRenderSidecars,
		runtime.CommandTypeRenderGatewayConfig,
		runtime.CommandTypeReconcileRevision,
		runtime.CommandTypeProvisionInternalSvc,
		runtime.CommandTypeStartReleaseCandidate,
		runtime.CommandTypeRunHealthGate,
		runtime.CommandTypeRollbackRelease,
		runtime.CommandTypeGarbageCollectRuntime,
	}
	for index, commandType := range expected {
		if dispatcher.commands[index].Type != commandType {
			t.Fatalf("expected command %d to be %q, got %q", index, commandType, dispatcher.commands[index].Type)
		}
	}

	deployment, _ := deploymentStore.GetByIDForProject("prj_123", "dep_123")
	if deployment.Status != DeploymentStatusRolledBack {
		t.Fatalf("expected deployment rolled_back, got %q", deployment.Status)
	}
	revision, _ := revisionStore.GetByIDForProject("prj_123", "rev_123")
	if revision.Status != RevisionStatusRolledBack {
		t.Fatalf("expected revision rolled_back, got %q", revision.Status)
	}
	if len(incidentStore.items) == 0 || incidentStore.items[0].Kind != IncidentKindUnhealthyCandidate {
		t.Fatalf("expected unhealthy_candidate incident, got %#v", incidentStore.items)
	}
}

func TestRolloutExecutionServicePreservesTrackedHealthGateFailureDetails(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(
		&models.DesiredStateRevision{
			ID:                   "rev_stable",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "stable123",
			TriggerKind:          "push",
			Status:               RevisionStatusPromoted,
			CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_stable", "bp_123", "prj_123"),
			CreatedAt:            time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC),
		},
		&models.DesiredStateRevision{
			ID:                   "rev_123",
			ProjectID:            "prj_123",
			BlueprintID:          "bp_123",
			DeploymentBindingID:  "bind_123",
			CommitSHA:            "abc123",
			TriggerKind:          "manual",
			Status:               RevisionStatusArtifactReady,
			CompiledRevisionJSON: mustCompiledRevisionJSON(t, "rev_123", "bp_123", "prj_123"),
			CreatedAt:            time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC),
		},
	)
	deploymentStore := newFakeDeploymentStore(&models.Deployment{
		ID:         "dep_123",
		ProjectID:  "prj_123",
		RevisionID: "rev_123",
		Status:     DeploymentStatusQueued,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: runtime.RuntimeModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:      "inst_123",
		UserID:  "usr_123",
		Name:    "edge-1",
		Status:  "online",
		AgentID: ptrString("agt_123"),
	})
	incidentStore := newFakeRuntimeIncidentStore()
	broadcaster := &fakeOperatorEventBroadcaster{}
	dispatcher := &fakeRolloutDispatcher{
		waitResultsByType: map[string][]*TrackedCommand{
			runtime.CommandTypeRunHealthGate: {
				{
					State: CommandStateFailed,
					Error: "health gate failed for 1/3 services: be",
					Output: map[string]any{
						"code":             "health_gate_failed",
						"policy_action":    "rollback_release",
						"summary":          "health gate failed for 1/3 services: be",
						"failing_services": []any{"be"},
						"services": []map[string]any{
							{
								"service_name": "be",
								"passed":       false,
								"message":      "rollout progressing: waiting for available replicas",
							},
						},
					},
				},
			},
		},
	}

	deployments := NewDeploymentService(newFakeProjectStore(), newFakeBlueprintStore(), revisionStore, deploymentStore)
	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, incidentStore, bindingStore, broadcaster)
	service := NewRolloutExecutionService(deployments, planner, instanceStore, dispatcher, broadcaster)

	result, err := service.StartDeployment(context.Background(), "prj_123", "dep_123")
	if err == nil {
		t.Fatal("expected health gate command failure")
	}
	if result.Rollback == nil {
		t.Fatal("expected rollback result after health gate command failure")
	}

	expected := []string{
		runtime.CommandTypePrepareReleaseWorkspace,
		runtime.CommandTypeRenderSidecars,
		runtime.CommandTypeRenderGatewayConfig,
		runtime.CommandTypeReconcileRevision,
		runtime.CommandTypeProvisionInternalSvc,
		runtime.CommandTypeStartReleaseCandidate,
		runtime.CommandTypeRunHealthGate,
		runtime.CommandTypeRollbackRelease,
		runtime.CommandTypeGarbageCollectRuntime,
	}
	for index, commandType := range expected {
		if dispatcher.commands[index].Type != commandType {
			t.Fatalf("expected command %d to be %q, got %q", index, commandType, dispatcher.commands[index].Type)
		}
	}

	if len(incidentStore.items) == 0 {
		t.Fatal("expected unhealthy candidate incident to be recorded")
	}
	incident := incidentStore.items[0]
	if incident.Kind != IncidentKindUnhealthyCandidate {
		t.Fatalf("expected unhealthy candidate incident, got %#v", incident)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(incident.DetailsJSON), &details); err != nil {
		t.Fatalf("decode incident details: %v", err)
	}
	if details["policy_action"] != "rollback_release" {
		t.Fatalf("expected rollback policy in incident details, got %#v", details)
	}
	failingServices, ok := details["failing_services"].([]any)
	if !ok || len(failingServices) != 1 || failingServices[0] != "be" {
		t.Fatalf("expected failing_services to include be, got %#v", details["failing_services"])
	}
	services, ok := details["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("expected services details to be preserved, got %#v", details["services"])
	}
	firstService, ok := services[0].(map[string]any)
	if !ok || firstService["service_name"] != "be" {
		t.Fatalf("expected be service details, got %#v", services[0])
	}
}

func TestRolloutExecutionServiceIsRetrySafe(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "manual",
		Status:               RevisionStatusApplying,
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
		RuntimeMode: runtime.RuntimeModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:      "inst_123",
		UserID:  "usr_123",
		Name:    "edge-1",
		Status:  "online",
		AgentID: ptrString("agt_123"),
	})
	dispatcher := &fakeRolloutDispatcher{}

	deployments := NewDeploymentService(newFakeProjectStore(), newFakeBlueprintStore(), revisionStore, deploymentStore)
	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, newFakeRuntimeIncidentStore(), bindingStore, &fakeOperatorEventBroadcaster{})
	service := NewRolloutExecutionService(deployments, planner, instanceStore, dispatcher, &fakeOperatorEventBroadcaster{})

	result, err := service.StartDeployment(context.Background(), "prj_123", "dep_123")
	if !errors.Is(err, ErrRolloutAlreadyStarted) {
		t.Fatalf("expected ErrRolloutAlreadyStarted, got %v", err)
	}
	if !result.AlreadyStarted {
		t.Fatal("expected already started result")
	}
	if len(dispatcher.commands) != 0 {
		t.Fatalf("expected no commands to be redispatched, got %d", len(dispatcher.commands))
	}
}

func TestRolloutExecutionServiceRecoversRunningDeploymentAfterReconnect(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusApplying,
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
		RuntimeMode: runtime.RuntimeModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:      "inst_123",
		UserID:  "usr_123",
		Name:    "edge-1",
		Status:  "online",
		AgentID: ptrString("agt_123"),
	})
	dispatcher := &fakeRolloutDispatcher{
		waitResultsByType: map[string][]*TrackedCommand{
			runtime.CommandTypePromoteRelease: {
				{State: CommandStateDone},
			},
		},
	}
	deployments := NewDeploymentService(projectStore, newFakeBlueprintStore(), revisionStore, deploymentStore)
	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, newFakeRuntimeIncidentStore(), bindingStore, &fakeOperatorEventBroadcaster{})
	service := NewRolloutExecutionService(deployments, planner, instanceStore, dispatcher, &fakeOperatorEventBroadcaster{})

	if err := service.RecoverRunningDeploymentsForAgent(context.Background(), "usr_123", "agt_123"); err != nil {
		t.Fatalf("recover running deployment: %v", err)
	}

	if got := dispatchedTypes(dispatcher.commands); len(got) != 2 {
		t.Fatalf("expected 2 recovery commands, got %d (%v)", len(got), got)
	}
	if dispatcher.commands[0].Type != runtime.CommandTypePromoteRelease {
		t.Fatalf("expected first recovery command to be promote_release, got %q", dispatcher.commands[0].Type)
	}
	if dispatcher.commands[1].Type != runtime.CommandTypeGarbageCollectRuntime {
		t.Fatalf("expected second recovery command to be garbage_collect_runtime, got %q", dispatcher.commands[1].Type)
	}

	deployment, _ := deploymentStore.GetByIDForProject("prj_123", "dep_123")
	if deployment.Status != DeploymentStatusPromoted {
		t.Fatalf("expected deployment promoted after recovery, got %q", deployment.Status)
	}
	revision, _ := revisionStore.GetByIDForProject("prj_123", "rev_123")
	if revision.Status != RevisionStatusPromoted {
		t.Fatalf("expected revision promoted after recovery, got %q", revision.Status)
	}
}

func TestRolloutExecutionServiceRecoversQueuedDeploymentAfterReconnect(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
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
		Status:     DeploymentStatusQueued,
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:          "bind_123",
		ProjectID:   "prj_123",
		Name:        "Production",
		TargetRef:   "prod-main",
		RuntimeMode: runtime.RuntimeModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:      "inst_123",
		UserID:  "usr_123",
		Name:    "edge-1",
		Status:  "online",
		AgentID: ptrString("agt_123"),
	})
	dispatcher := &fakeRolloutDispatcher{}
	deployments := NewDeploymentService(projectStore, newFakeBlueprintStore(), revisionStore, deploymentStore)
	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, newFakeRuntimeIncidentStore(), bindingStore, &fakeOperatorEventBroadcaster{})
	service := NewRolloutExecutionService(deployments, planner, instanceStore, dispatcher, &fakeOperatorEventBroadcaster{})

	if err := service.RecoverRunningDeploymentsForAgent(context.Background(), "usr_123", "agt_123"); err != nil {
		t.Fatalf("recover queued deployment: %v", err)
	}

	expected := []string{
		runtime.CommandTypePrepareReleaseWorkspace,
		runtime.CommandTypeRenderSidecars,
		runtime.CommandTypeRenderGatewayConfig,
		runtime.CommandTypeReconcileRevision,
		runtime.CommandTypeProvisionInternalSvc,
		runtime.CommandTypeStartReleaseCandidate,
		runtime.CommandTypeRunHealthGate,
		runtime.CommandTypePromoteRelease,
		runtime.CommandTypeGarbageCollectRuntime,
	}
	got := dispatchedTypes(dispatcher.commands)
	if len(got) != len(expected) {
		t.Fatalf("expected %d queued recovery commands, got %d (%v)", len(expected), len(got), got)
	}
	for index, want := range expected {
		if got[index] != want {
			t.Fatalf("expected queued recovery command %d to be %q, got %q", index, want, got[index])
		}
	}

	deployment, _ := deploymentStore.GetByIDForProject("prj_123", "dep_123")
	if deployment.Status != DeploymentStatusPromoted {
		t.Fatalf("expected deployment promoted after queued recovery, got %q", deployment.Status)
	}
	revision, _ := revisionStore.GetByIDForProject("prj_123", "rev_123")
	if revision.Status != RevisionStatusPromoted {
		t.Fatalf("expected revision promoted after queued recovery, got %q", revision.Status)
	}
}

func TestRolloutExecutionServiceRecoveryFallsBackToHealthGate(t *testing.T) {
	registry := runtime.NewRegistry()
	registry.Register(runtime.NewStandaloneDriver())

	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		TriggerKind:          "push",
		Status:               RevisionStatusApplying,
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
		RuntimeMode: runtime.RuntimeModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:      "inst_123",
		UserID:  "usr_123",
		Name:    "edge-1",
		Status:  "online",
		AgentID: ptrString("agt_123"),
	})
	dispatcher := &fakeRolloutDispatcher{
		waitResultsByType: map[string][]*TrackedCommand{
			runtime.CommandTypePromoteRelease: {
				{
					State: CommandStateFailed,
					Error: "candidate is not promotable yet",
					Output: map[string]any{
						"code": "promotion_candidate_not_ready",
					},
				},
				{State: CommandStateDone},
			},
			runtime.CommandTypeRunHealthGate: {
				{State: CommandStateDone},
			},
		},
	}
	deployments := NewDeploymentService(projectStore, newFakeBlueprintStore(), revisionStore, deploymentStore)
	planner := newTestRolloutPlanner(registry, revisionStore, deploymentStore, newFakeRuntimeIncidentStore(), bindingStore, &fakeOperatorEventBroadcaster{})
	service := NewRolloutExecutionService(deployments, planner, instanceStore, dispatcher, &fakeOperatorEventBroadcaster{})

	if err := service.RecoverRunningDeploymentsForAgent(context.Background(), "usr_123", "agt_123"); err != nil {
		t.Fatalf("recover running deployment with health fallback: %v", err)
	}

	expected := []string{
		runtime.CommandTypePromoteRelease,
		runtime.CommandTypeRunHealthGate,
		runtime.CommandTypePromoteRelease,
		runtime.CommandTypeGarbageCollectRuntime,
	}
	got := dispatchedTypes(dispatcher.commands)
	if len(got) != len(expected) {
		t.Fatalf("expected %d recovery commands, got %d (%v)", len(expected), len(got), got)
	}
	for index, want := range expected {
		if got[index] != want {
			t.Fatalf("expected recovery command %d to be %q, got %q", index, want, got[index])
		}
	}
}
