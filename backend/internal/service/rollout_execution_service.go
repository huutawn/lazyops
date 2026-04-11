package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/runtime"
	"lazyops-server/pkg/logger"
	"lazyops-server/pkg/utils"
)

var (
	ErrRolloutArtifactPending   = errors.New("rollout artifact is not ready")
	ErrRolloutAgentUnavailable  = errors.New("target agent is unavailable")
	ErrRolloutUnsupportedTarget = errors.New("rollout target is not supported")
	ErrRolloutAlreadyStarted    = errors.New("deployment rollout already started")
	ErrHealthGateFailed         = errors.New("health gate failed")
)

type RolloutCommandDispatcher interface {
	DispatchCommand(ctx context.Context, agentID string, cmd runtime.AgentCommand) (*runtime.CommandResult, error)
	WaitForCommand(ctx context.Context, requestID string) (*TrackedCommand, error)
}

type HealthGateEvaluator func(ctx context.Context, projectID, deploymentID, revisionID string) (*HealthGateResult, error)

type RolloutExecutionService struct {
	deployments         *DeploymentService
	planner             *RolloutPlanner
	instances           InstanceStore
	dispatcher          RolloutCommandDispatcher
	operatorHub         OperatorEventBroadcaster
	healthGateEvaluator HealthGateEvaluator
	correlationID       func() string
}

type RolloutExecutionResult struct {
	DeploymentID       string
	RevisionID         string
	AgentID            string
	CorrelationID      string
	DispatchedCommands []string
	HealthGate         *HealthGateResult
	Promotion          *PromotionResult
	Rollback           *RollbackResult
	AlreadyStarted     bool
}

func NewRolloutExecutionService(
	deployments *DeploymentService,
	planner *RolloutPlanner,
	instances InstanceStore,
	dispatcher RolloutCommandDispatcher,
	operatorHub OperatorEventBroadcaster,
) *RolloutExecutionService {
	return &RolloutExecutionService{
		deployments: deployments,
		planner:     planner,
		instances:   instances,
		dispatcher:  dispatcher,
		operatorHub: operatorHub,
		correlationID: func() string {
			return utils.NewCorrelationID()
		},
	}
}

func (s *RolloutExecutionService) WithHealthGateEvaluator(evaluator HealthGateEvaluator) *RolloutExecutionService {
	s.healthGateEvaluator = evaluator
	return s
}

func (s *RolloutExecutionService) StartDeployment(ctx context.Context, projectID, deploymentID string) (*RolloutExecutionResult, error) {
	if s == nil || s.deployments == nil || s.planner == nil || s.instances == nil || s.dispatcher == nil {
		return nil, ErrInvalidInput
	}

	projectID = strings.TrimSpace(projectID)
	deploymentID = strings.TrimSpace(deploymentID)
	if projectID == "" || deploymentID == "" {
		return nil, ErrInvalidInput
	}

	logger.Info("rollout_starting",
		"project_id", projectID,
		"deployment_id", deploymentID,
	)

	deployment, err := s.deployments.deployments.GetByIDForProject(projectID, deploymentID)
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, ErrDeploymentNotFound
	}

	revision, err := s.deployments.revisions.GetByIDForProject(projectID, deployment.RevisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrRevisionNotFound
	}

	if rolloutAlreadyStarted(deployment.Status, revision.Status) {
		logger.Warn("rollout_already_started",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"deployment_status", deployment.Status,
			"revision_status", revision.Status,
		)
		return &RolloutExecutionResult{
			DeploymentID:   deployment.ID,
			RevisionID:     revision.ID,
			AlreadyStarted: true,
		}, ErrRolloutAlreadyStarted
	}

	compiled, err := parseCompiledRevision(revision.CompiledRevisionJSON)
	if err != nil {
		return nil, fmt.Errorf("parse compiled revision: %w", err)
	}
	if strings.TrimSpace(compiled.ArtifactRef) == "" && strings.TrimSpace(compiled.ImageRef) == "" {
		logger.Warn("rollout_artifact_pending",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"revision_id", revision.ID,
		)
		return nil, ErrRolloutArtifactPending
	}

	binding, err := s.planner.bindings.GetByIDForProject(projectID, compiled.DeploymentBindingID)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, ErrInvalidInput
	}
	if binding.RuntimeMode != runtime.RuntimeModeStandalone || binding.TargetKind != "instance" {
		logger.Warn("rollout_unsupported_target",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"runtime_mode", binding.RuntimeMode,
			"target_kind", binding.TargetKind,
		)
		return nil, ErrRolloutUnsupportedTarget
	}

	instance, err := s.instances.GetByID(binding.TargetID)
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.AgentID == nil || strings.TrimSpace(*instance.AgentID) == "" || strings.EqualFold(instance.Status, "offline") {
		logger.Warn("rollout_agent_unavailable",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"instance_id", binding.TargetID,
			"instance_status", instance.Status,
			"has_agent_id", instance.AgentID != nil,
		)
		return nil, ErrRolloutAgentUnavailable
	}
	agentID := strings.TrimSpace(*instance.AgentID)

	logger.Info("rollout_planning_candidate",
		"project_id", projectID,
		"deployment_id", deploymentID,
		"agent_id", agentID,
	)

	plan, err := s.planner.PlanCandidate(ctx, projectID, revision.ID)
	if err != nil {
		return nil, err
	}

	logger.Info("rollout_transitioning_to_running",
		"project_id", projectID,
		"deployment_id", deploymentID,
		"revision_id", revision.ID,
		"plan_steps", len(plan.Steps),
	)

	if _, err := s.deployments.TransitionRevisionStatus(projectID, revision.ID, RevisionStatusPlanned); err != nil {
		return nil, err
	}
	if _, err := s.deployments.TransitionDeploymentStatus(projectID, deployment.ID, DeploymentStatusRunning); err != nil {
		return nil, err
	}

	correlationID := s.correlationID()
	result := &RolloutExecutionResult{
		DeploymentID:       deployment.ID,
		RevisionID:         revision.ID,
		AgentID:            agentID,
		CorrelationID:      correlationID,
		DispatchedCommands: make([]string, 0, len(plan.Steps)+2),
	}

	if s.operatorHub != nil {
		_ = s.operatorHub.BroadcastEvent(runtime.EventDeploymentStarted, map[string]any{
			"deployment_id":  deployment.ID,
			"revision_id":    revision.ID,
			"project_id":     projectID,
			"runtime_mode":   binding.RuntimeMode,
			"target_id":      binding.TargetID,
			"correlation_id": correlationID,
		})
	}

	for i, step := range plan.Steps {
		cmd := enrichRolloutCommand(step.Command, projectID, revision.ID, correlationID)
		logger.Info("rollout_command_enriched",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"step_index", i,
			"command_type", cmd.Type,
			"source", cmd.Source,
			"agent_id", agentID,
		)

		cmdResult, err := s.dispatcher.DispatchCommand(ctx, agentID, cmd)
		if err != nil {
			logger.Error("rollout_dispatch_failed",
				"project_id", projectID,
				"deployment_id", deploymentID,
				"command_type", cmd.Type,
				"error", err.Error(),
			)
			_ = s.failDeployment(projectID, deployment.ID, revision.ID)
			return result, err
		}
		result.DispatchedCommands = append(result.DispatchedCommands, cmd.Type)

		logger.Info("rollout_waiting_for_response",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"command_type", cmd.Type,
			"request_id", cmdResult.RequestID,
			"timeout", "5m",
		)

		waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Minute)
		tracked, waitErr := s.dispatcher.WaitForCommand(waitCtx, cmdResult.RequestID)
		waitCancel()

		if waitErr != nil {
			logger.Error("rollout_command_timeout",
				"project_id", projectID,
				"deployment_id", deploymentID,
				"command_type", cmd.Type,
				"request_id", cmdResult.RequestID,
				"error", waitErr.Error(),
			)
			_ = s.failDeployment(projectID, deployment.ID, revision.ID)
			_, _ = s.planner.RecordIncident(projectID, deployment.ID, revision.ID, IncidentKindHealthGateTimeout, IncidentSeverityCritical, "command execution timed out or failed", map[string]any{
				"command_type": cmd.Type,
				"request_id":   cmdResult.RequestID,
				"error":        waitErr.Error(),
			}, "command_dispatch")
			rollbackResult, rollbackErr := s.rollbackDeployment(ctx, projectID, deployment.ID, revision.ID, agentID, correlationID, result)
			result.Rollback = rollbackResult
			if rollbackErr != nil {
				return result, rollbackErr
			}
			return result, fmt.Errorf("command %q failed: %w", cmd.Type, waitErr)
		}

		if tracked.State == CommandStateFailed {
			logger.Error("rollout_command_failed",
				"project_id", projectID,
				"deployment_id", deploymentID,
				"command_type", cmd.Type,
				"request_id", cmdResult.RequestID,
				"tracked_error", tracked.Error,
			)
			_ = s.failDeployment(projectID, deployment.ID, revision.ID)
			_, _ = s.planner.RecordIncident(projectID, deployment.ID, revision.ID, IncidentKindUnhealthyCandidate, IncidentSeverityCritical, "command execution failed", map[string]any{
				"command_type": cmd.Type,
				"request_id":   cmdResult.RequestID,
				"error":        tracked.Error,
			}, "command_dispatch")
			rollbackResult, rollbackErr := s.rollbackDeployment(ctx, projectID, deployment.ID, revision.ID, agentID, correlationID, result)
			result.Rollback = rollbackResult
			if rollbackErr != nil {
				return result, rollbackErr
			}
			return result, fmt.Errorf("command %q failed: %s", cmd.Type, tracked.Error)
		}

		logger.Info("rollout_command_completed",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"command_type", cmd.Type,
			"state", tracked.State,
		)

		switch cmd.Type {
		case runtime.CommandTypeStartReleaseCandidate:
			if _, err := s.deployments.TransitionRevisionStatus(projectID, revision.ID, RevisionStatusApplying); err != nil {
				return result, err
			}
		case runtime.CommandTypeRunHealthGate:
			healthGate, err := s.evaluateHealthGate(ctx, projectID, deployment.ID, revision.ID)
			if err != nil {
				_, _ = s.planner.RecordIncident(projectID, deployment.ID, revision.ID, IncidentKindHealthGateTimeout, IncidentSeverityCritical, "health gate evaluation failed", map[string]any{
					"error": err.Error(),
				}, "health_gate")
				rollbackResult, rollbackErr := s.rollbackDeployment(ctx, projectID, deployment.ID, revision.ID, agentID, correlationID, result)
				result.Rollback = rollbackResult
				if rollbackErr != nil {
					return result, rollbackErr
				}
				return result, err
			}
			result.HealthGate = healthGate
			if !healthGate.Passed {
				_, _ = s.planner.RecordIncident(projectID, deployment.ID, revision.ID, IncidentKindUnhealthyCandidate, IncidentSeverityCritical, "candidate failed health gate", map[string]any{
					"services": healthGate.Services,
				}, "health_gate")
				rollbackResult, rollbackErr := s.rollbackDeployment(ctx, projectID, deployment.ID, revision.ID, agentID, correlationID, result)
				result.Rollback = rollbackResult
				if rollbackErr != nil {
					return result, rollbackErr
				}
				return result, ErrHealthGateFailed
			}
			if _, err := s.deployments.TransitionDeploymentStatus(projectID, deployment.ID, DeploymentStatusCandidateReady); err != nil {
				return result, err
			}
			if s.operatorHub != nil {
				_ = s.operatorHub.BroadcastEvent(runtime.EventDeploymentCandidateReady, map[string]any{
					"deployment_id":  deployment.ID,
					"revision_id":    revision.ID,
					"project_id":     projectID,
					"correlation_id": correlationID,
				})
			}
		case runtime.CommandTypePromoteRelease:
			promotion, err := s.planner.PromoteCandidate(ctx, projectID, deployment.ID, revision.ID)
			if err != nil {
				return result, err
			}
			result.Promotion = promotion
			if err := s.dispatchGarbageCollect(ctx, projectID, revision.ID, agentID, correlationID, result); err != nil {
				return result, err
			}
		}
	}

	logger.Info("rollout_completed",
		"project_id", projectID,
		"deployment_id", deploymentID,
		"revision_id", revision.ID,
		"commands_dispatched", len(result.DispatchedCommands),
	)

	return result, nil
}

func (s *RolloutExecutionService) evaluateHealthGate(ctx context.Context, projectID, deploymentID, revisionID string) (*HealthGateResult, error) {
	if s.healthGateEvaluator != nil {
		return s.healthGateEvaluator(ctx, projectID, deploymentID, revisionID)
	}
	return s.planner.ExecuteHealthGate(ctx, projectID, deploymentID, revisionID)
}

func (s *RolloutExecutionService) rollbackDeployment(ctx context.Context, projectID, deploymentID, revisionID, agentID, correlationID string, result *RolloutExecutionResult) (*RollbackResult, error) {
	cmd := enrichRolloutCommand(runtime.AgentCommand{
		Type:      runtime.CommandTypeRollbackRelease,
		ProjectID: projectID,
		Source:    "rollout_execution_service",
		Payload: map[string]any{
			"deployment_id": deploymentID,
			"revision_id":   revisionID,
		},
	}, projectID, revisionID, correlationID)
	if _, err := s.dispatcher.DispatchCommand(ctx, agentID, cmd); err != nil {
		_ = s.failDeployment(projectID, deploymentID, revisionID)
		return nil, err
	}
	result.DispatchedCommands = append(result.DispatchedCommands, cmd.Type)

	rollbackResult, err := s.planner.RollbackDeployment(ctx, projectID, deploymentID)
	if err != nil {
		_ = s.failDeployment(projectID, deploymentID, revisionID)
		return nil, err
	}
	if err := s.dispatchGarbageCollect(ctx, projectID, revisionID, agentID, correlationID, result); err != nil {
		return rollbackResult, err
	}
	return rollbackResult, nil
}

func (s *RolloutExecutionService) dispatchGarbageCollect(ctx context.Context, projectID, revisionID, agentID, correlationID string, result *RolloutExecutionResult) error {
	cmd := enrichRolloutCommand(runtime.AgentCommand{
		Type:      runtime.CommandTypeGarbageCollectRuntime,
		ProjectID: projectID,
		Source:    "rollout_execution_service",
		Payload: map[string]any{
			"revision_id": revisionID,
		},
	}, projectID, revisionID, correlationID)
	if _, err := s.dispatcher.DispatchCommand(ctx, agentID, cmd); err != nil {
		return err
	}
	result.DispatchedCommands = append(result.DispatchedCommands, cmd.Type)
	return nil
}

func (s *RolloutExecutionService) failDeployment(projectID, deploymentID, revisionID string) error {
	if _, err := s.deployments.TransitionDeploymentStatus(projectID, deploymentID, DeploymentStatusFailed); err != nil && !errors.Is(err, ErrInvalidDeploymentStateTransition) {
		return err
	}
	if _, err := s.deployments.TransitionRevisionStatus(projectID, revisionID, RevisionStatusFailed); err != nil && !errors.Is(err, ErrInvalidRevisionStateTransition) {
		return err
	}
	return nil
}

func enrichRolloutCommand(cmd runtime.AgentCommand, projectID, revisionID, correlationID string) runtime.AgentCommand {
	cmd.ProjectID = projectID
	cmd.CorrelationID = correlationID
	// Agent dispatcher requires source="backend" for all command envelopes.
	// Drivers may set their own source internally, but the control plane
	// must always send "backend" to pass agent-side validation.
	cmd.Source = "backend"
	if cmd.Payload == nil {
		cmd.Payload = map[string]any{}
	}
	if _, ok := cmd.Payload["revision_id"]; !ok {
		cmd.Payload["revision_id"] = revisionID
	}
	return cmd
}

func rolloutAlreadyStarted(deploymentStatus, revisionStatus string) bool {
	switch deploymentStatus {
	case DeploymentStatusRunning, DeploymentStatusCandidateReady, DeploymentStatusPromoted, DeploymentStatusFailed, DeploymentStatusRolledBack, DeploymentStatusCanceled:
		return true
	}
	switch revisionStatus {
	case RevisionStatusPlanned, RevisionStatusApplying, RevisionStatusPromoted, RevisionStatusFailed, RevisionStatusRolledBack, RevisionStatusSuperseded:
		return true
	}
	return false
}
