package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/internal/runtime"
)

type fakeRolloutDispatcher struct {
	commands []runtime.AgentCommand
	agentIDs []string
	err      error
}

func (f *fakeRolloutDispatcher) DispatchCommand(_ context.Context, agentID string, cmd runtime.AgentCommand) (*runtime.CommandResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.agentIDs = append(f.agentIDs, agentID)
	f.commands = append(f.commands, cmd)
	return &runtime.CommandResult{RequestID: cmd.RequestID, Status: "dispatched"}, nil
}

func (f *fakeRolloutDispatcher) WaitForCommand(_ context.Context, requestID string) (*TrackedCommand, error) {
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
