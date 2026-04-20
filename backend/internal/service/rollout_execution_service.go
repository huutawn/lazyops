package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"lazyops-server/internal/models"
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
	clusters            ClusterStore
	dispatcher          RolloutCommandDispatcher
	operatorHub         OperatorEventBroadcaster
	healthGateEvaluator HealthGateEvaluator
	correlationID       func() string
	recoveryMu          sync.Mutex
	recovering          map[string]struct{}
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
		recovering: make(map[string]struct{}),
	}
}

func (s *RolloutExecutionService) WithHealthGateEvaluator(evaluator HealthGateEvaluator) *RolloutExecutionService {
	s.healthGateEvaluator = evaluator
	return s
}

func (s *RolloutExecutionService) WithClusterStore(clusters ClusterStore) *RolloutExecutionService {
	if s == nil {
		return s
	}
	s.clusters = clusters
	return s
}

func (s *RolloutExecutionService) resolveRolloutAgent(binding *models.DeploymentBinding) (string, string, error) {
	if binding == nil {
		return "", "", ErrInvalidInput
	}

	switch {
	case binding.RuntimeMode == runtime.RuntimeModeStandalone && binding.TargetKind == "instance":
		instance, err := s.instances.GetByID(binding.TargetID)
		if err != nil {
			return "", "", err
		}
		if instance == nil || instance.AgentID == nil || strings.TrimSpace(*instance.AgentID) == "" || strings.EqualFold(instance.Status, "offline") {
			return "", "", ErrRolloutAgentUnavailable
		}
		return strings.TrimSpace(*instance.AgentID), "instance:" + binding.TargetID, nil
	case binding.RuntimeMode == runtime.RuntimeModeDistributedK3s && binding.TargetKind == "cluster":
		if s.clusters == nil {
			return "", "", ErrRolloutUnsupportedTarget
		}
		cluster, err := s.clusters.GetByID(binding.TargetID)
		if err != nil {
			return "", "", err
		}
		if cluster == nil {
			return "", "", ErrTargetNotFound
		}
		if normalizeClusterStatus(cluster.Status) != ClusterStatusReady {
			return "", "", ErrClusterNotReady
		}
		if cluster.InstanceID == nil || strings.TrimSpace(*cluster.InstanceID) == "" {
			return "", "", ErrRolloutAgentUnavailable
		}
		instance, err := s.instances.GetByID(strings.TrimSpace(*cluster.InstanceID))
		if err != nil {
			return "", "", err
		}
		if instance == nil || instance.AgentID == nil || strings.TrimSpace(*instance.AgentID) == "" || strings.EqualFold(instance.Status, "offline") {
			return "", "", ErrRolloutAgentUnavailable
		}
		return strings.TrimSpace(*instance.AgentID), "cluster:" + binding.TargetID, nil
	default:
		return "", "", ErrRolloutUnsupportedTarget
	}
}

func (s *RolloutExecutionService) RecoverRunningDeploymentsForAgent(ctx context.Context, userID, agentID string) error {
	if s == nil || s.deployments == nil || s.planner == nil || s.instances == nil || s.dispatcher == nil {
		return ErrInvalidInput
	}

	userID = strings.TrimSpace(userID)
	agentID = strings.TrimSpace(agentID)
	if userID == "" || agentID == "" {
		return ErrInvalidInput
	}

	instances, err := s.instances.ListByUser(userID)
	if err != nil {
		return err
	}

	targetInstanceIDs := make(map[string]struct{})
	for _, instance := range instances {
		if instance.AgentID == nil {
			continue
		}
		if strings.TrimSpace(*instance.AgentID) == agentID {
			targetInstanceIDs[instance.ID] = struct{}{}
		}
	}
	if len(targetInstanceIDs) == 0 {
		return nil
	}

	projects, err := s.deployments.projects.ListByUser(userID)
	if err != nil {
		return err
	}

	var recoveryErrs []error
	for _, project := range projects {
		if err := s.recoverProjectDeploymentsForAgent(ctx, project.ID, agentID, targetInstanceIDs); err != nil {
			recoveryErrs = append(recoveryErrs, err)
		}
	}

	return errors.Join(recoveryErrs...)
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
	if strings.TrimSpace(compiled.ArtifactRef) == "" && strings.TrimSpace(compiled.ImageRef) == "" && !compiledServicesHaveArtifacts(compiled.ServiceSpecs, compiled.Services) {
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
	agentID, targetLabel, err := s.resolveRolloutAgent(binding)
	if err != nil {
		logger.Warn("rollout_target_resolution_failed",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"runtime_mode", binding.RuntimeMode,
			"target_kind", binding.TargetKind,
			"target_id", binding.TargetID,
			"error", err.Error(),
		)
		return nil, err
	}

	logger.Info("rollout_planning_candidate",
		"project_id", projectID,
		"deployment_id", deploymentID,
		"agent_id", agentID,
		"target", targetLabel,
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
			"target_kind":    binding.TargetKind,
			"target_id":      binding.TargetID,
			"correlation_id": correlationID,
		})
	}

	for i, step := range plan.Steps {
		if skip, err := s.shouldSkipRolloutStep(projectID, deployment.ID, revision.ID, step.Command.Type); err != nil {
			return result, err
		} else if skip {
			logger.Info("rollout_command_skipped",
				"project_id", projectID,
				"deployment_id", deploymentID,
				"command_type", step.Command.Type,
				"reason", "deployment already advanced by recovery or duplicate promotion",
			)
			continue
		}

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
			failureSummary := tracked.Error
			if summary, ok := tracked.Output["summary"].(string); ok && strings.TrimSpace(summary) != "" {
				failureSummary = summary
			}
			logger.Error("rollout_command_failed",
				"project_id", projectID,
				"deployment_id", deploymentID,
				"command_type", cmd.Type,
				"request_id", cmdResult.RequestID,
				"tracked_error", tracked.Error,
				"tracked_code", tracked.Output["code"],
				"policy_action", tracked.Output["policy_action"],
				"failure_summary", failureSummary,
				"failing_services", tracked.Output["failing_services"],
				"services", tracked.Output["services"],
			)
			_ = s.failDeployment(projectID, deployment.ID, revision.ID)
			incidentDetails := map[string]any{
				"command_type": cmd.Type,
				"request_id":   cmdResult.RequestID,
				"error":        tracked.Error,
			}
			if tracked.Output != nil {
				if code, ok := tracked.Output["code"]; ok {
					incidentDetails["code"] = code
				}
				if policyAction, ok := tracked.Output["policy_action"]; ok {
					incidentDetails["policy_action"] = policyAction
				}
				if summary, ok := tracked.Output["summary"]; ok {
					incidentDetails["summary"] = summary
				}
				if failingServices, ok := tracked.Output["failing_services"]; ok {
					incidentDetails["failing_services"] = failingServices
				}
				if services, ok := tracked.Output["services"]; ok {
					incidentDetails["services"] = services
				}
			}
			_, _ = s.planner.RecordIncident(projectID, deployment.ID, revision.ID, IncidentKindUnhealthyCandidate, IncidentSeverityCritical, "command execution failed", incidentDetails, "command_dispatch")
			rollbackResult, rollbackErr := s.rollbackDeployment(ctx, projectID, deployment.ID, revision.ID, agentID, correlationID, result)
			result.Rollback = rollbackResult
			if rollbackErr != nil {
				return result, rollbackErr
			}
			return result, fmt.Errorf("command %q failed: %s", cmd.Type, failureSummary)
		}

		logger.Info("rollout_command_completed",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"command_type", cmd.Type,
			"state", tracked.State,
		)
		s.broadcastRolloutProgress(deployment.ID, revision.ID, projectID, correlationID, i, len(plan.Steps), cmd.Type, tracked)

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
			if err := s.transitionDeploymentStatusIfNeeded(projectID, deployment.ID, DeploymentStatusCandidateReady, DeploymentStatusCandidateReady, DeploymentStatusPromoted); err != nil {
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
			promotion, err := s.promoteCandidateIfNeeded(ctx, projectID, deployment.ID, revision.ID)
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

func compiledServicesHaveArtifacts(specs []K3sServiceSpecRecord, services []BlueprintServiceContractRecord) bool {
	for _, item := range specs {
		if strings.TrimSpace(item.ImageRef) != "" {
			return true
		}
	}
	for _, item := range services {
		if strings.TrimSpace(item.ImageRef) != "" {
			return true
		}
	}
	return false
}

func (s *RolloutExecutionService) recoverProjectDeploymentsForAgent(ctx context.Context, projectID, agentID string, targetInstanceIDs map[string]struct{}) error {
	bindings, err := s.planner.bindings.ListByProject(projectID)
	if err != nil {
		return err
	}

	targetBindings := make(map[string]struct{})
	for _, binding := range bindings {
		switch {
		case binding.TargetKind == "instance" && binding.RuntimeMode == runtime.RuntimeModeStandalone:
			if _, ok := targetInstanceIDs[binding.TargetID]; !ok {
				continue
			}
		case binding.TargetKind == "cluster" && binding.RuntimeMode == runtime.RuntimeModeDistributedK3s && s.clusters != nil:
			cluster, err := s.clusters.GetByID(binding.TargetID)
			if err != nil || cluster == nil || cluster.InstanceID == nil {
				continue
			}
			if _, ok := targetInstanceIDs[strings.TrimSpace(*cluster.InstanceID)]; !ok {
				continue
			}
		default:
			continue
		}
		targetBindings[binding.ID] = struct{}{}
	}
	if len(targetBindings) == 0 {
		return nil
	}

	deployments, err := s.deployments.deployments.ListByProject(projectID)
	if err != nil {
		return err
	}

	var recoveryErrs []error
	for _, deployment := range deployments {
		status := strings.TrimSpace(deployment.Status)
		if status != DeploymentStatusQueued && status != DeploymentStatusRunning && status != DeploymentStatusCandidateReady {
			continue
		}

		revision, err := s.deployments.revisions.GetByIDForProject(projectID, deployment.RevisionID)
		if err != nil {
			recoveryErrs = append(recoveryErrs, err)
			continue
		}
		if revision == nil {
			continue
		}

		compiled, err := parseCompiledRevision(revision.CompiledRevisionJSON)
		if err != nil {
			recoveryErrs = append(recoveryErrs, fmt.Errorf("parse compiled revision %s: %w", revision.ID, err))
			continue
		}
		if _, ok := targetBindings[compiled.DeploymentBindingID]; !ok {
			continue
		}
		if !s.beginRecovery(deployment.ID) {
			continue
		}

		logger.Info("rollout_recovery_candidate_detected",
			"project_id", projectID,
			"deployment_id", deployment.ID,
			"revision_id", revision.ID,
			"agent_id", agentID,
			"deployment_status", deployment.Status,
		)

		if status == DeploymentStatusQueued {
			err = s.recoverQueuedDeploymentAfterReconnect(ctx, projectID, deployment.ID)
		} else {
			err = s.recoverDeploymentAfterReconnect(ctx, projectID, deployment.ID, revision.ID, agentID)
		}
		s.endRecovery(deployment.ID)
		if err != nil {
			recoveryErrs = append(recoveryErrs, err)
		}
	}

	return errors.Join(recoveryErrs...)
}

func (s *RolloutExecutionService) recoverQueuedDeploymentAfterReconnect(ctx context.Context, projectID, deploymentID string) error {
	result, err := s.StartDeployment(ctx, projectID, deploymentID)
	switch {
	case err == nil:
		logger.Info("rollout_queued_recovery_completed",
			"project_id", projectID,
			"deployment_id", deploymentID,
			"revision_id", result.RevisionID,
			"agent_id", result.AgentID,
			"commands_dispatched", len(result.DispatchedCommands),
		)
		return nil
	case errors.Is(err, ErrRolloutAlreadyStarted):
		logger.Info("rollout_queued_recovery_already_started",
			"project_id", projectID,
			"deployment_id", deploymentID,
		)
		return nil
	default:
		return fmt.Errorf("recover queued deployment %s: %w", deploymentID, err)
	}
}

func (s *RolloutExecutionService) recoverDeploymentAfterReconnect(ctx context.Context, projectID, deploymentID, revisionID, agentID string) error {
	plan, err := s.planner.PlanCandidate(ctx, projectID, revisionID)
	if err != nil {
		return fmt.Errorf("plan candidate recovery for deployment %s: %w", deploymentID, err)
	}

	promoteCmd, ok := findRolloutCommand(plan, runtime.CommandTypePromoteRelease)
	if !ok {
		return fmt.Errorf("recovery promote command missing for deployment %s", deploymentID)
	}

	correlationID := s.correlationID()
	promoteCmd = enrichRolloutCommand(promoteCmd, projectID, revisionID, correlationID)
	if _, err := s.executeRecoverableCommand(ctx, agentID, promoteCmd, 2*time.Minute); err != nil {
		if !isRetryablePromotionRecovery(err) {
			return fmt.Errorf("recover deployment %s promotion: %w", deploymentID, err)
		}

		healthCmd, ok := findRolloutCommand(plan, runtime.CommandTypeRunHealthGate)
		if !ok {
			return fmt.Errorf("recovery health gate command missing for deployment %s", deploymentID)
		}
		healthCmd = enrichRolloutCommand(healthCmd, projectID, revisionID, correlationID)
		if _, err := s.executeRecoverableCommand(ctx, agentID, healthCmd, 3*time.Minute); err != nil {
			return fmt.Errorf("recover deployment %s health gate: %w", deploymentID, err)
		}
		if _, err := s.executeRecoverableCommand(ctx, agentID, promoteCmd, 2*time.Minute); err != nil {
			return fmt.Errorf("recover deployment %s promotion after health gate: %w", deploymentID, err)
		}
	}

	if _, err := s.promoteCandidateIfNeeded(ctx, projectID, deploymentID, revisionID); err != nil {
		return fmt.Errorf("finalize recovered promotion for deployment %s: %w", deploymentID, err)
	}

	result := &RolloutExecutionResult{
		DeploymentID:  deploymentID,
		RevisionID:    revisionID,
		AgentID:       agentID,
		CorrelationID: correlationID,
	}
	if err := s.dispatchGarbageCollect(ctx, projectID, revisionID, agentID, correlationID, result); err != nil {
		return fmt.Errorf("recover deployment %s garbage collect: %w", deploymentID, err)
	}

	logger.Info("rollout_recovery_completed",
		"project_id", projectID,
		"deployment_id", deploymentID,
		"revision_id", revisionID,
		"agent_id", agentID,
	)

	return nil
}

func (s *RolloutExecutionService) executeRecoverableCommand(ctx context.Context, agentID string, cmd runtime.AgentCommand, timeout time.Duration) (*TrackedCommand, error) {
	cmdResult, err := s.dispatcher.DispatchCommand(ctx, agentID, cmd)
	if err != nil {
		return nil, err
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, timeout)
	defer waitCancel()

	tracked, err := s.dispatcher.WaitForCommand(waitCtx, cmdResult.RequestID)
	if err != nil {
		return nil, err
	}
	if tracked == nil {
		return nil, fmt.Errorf("command %q returned empty result", cmd.Type)
	}

	switch tracked.State {
	case CommandStateDone:
		return tracked, nil
	case CommandStateFailed:
		code, _ := tracked.Output["code"].(string)
		if code != "" {
			return tracked, fmt.Errorf("command %q failed: %s (%s)", cmd.Type, tracked.Error, code)
		}
		return tracked, fmt.Errorf("command %q failed: %s", cmd.Type, tracked.Error)
	case CommandStateCancelled:
		return tracked, fmt.Errorf("command %q timed out waiting for recovery completion", cmd.Type)
	default:
		return tracked, fmt.Errorf("command %q returned unexpected state %q", cmd.Type, tracked.State)
	}
}

func (s *RolloutExecutionService) shouldSkipRolloutStep(projectID, deploymentID, revisionID, commandType string) (bool, error) {
	deployment, err := s.deployments.deployments.GetByIDForProject(projectID, deploymentID)
	if err != nil {
		return false, err
	}
	revision, err := s.deployments.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return false, err
	}
	if deployment == nil || revision == nil {
		return false, nil
	}

	switch commandType {
	case runtime.CommandTypeRunHealthGate:
		return deployment.Status == DeploymentStatusCandidateReady || deployment.Status == DeploymentStatusPromoted, nil
	case runtime.CommandTypePromoteRelease:
		return deployment.Status == DeploymentStatusPromoted && revision.Status == RevisionStatusPromoted, nil
	default:
		return false, nil
	}
}

func (s *RolloutExecutionService) transitionDeploymentStatusIfNeeded(projectID, deploymentID, nextStatus string, alreadySatisfiedStatuses ...string) error {
	if _, err := s.deployments.TransitionDeploymentStatus(projectID, deploymentID, nextStatus); err != nil {
		if !errors.Is(err, ErrInvalidDeploymentStateTransition) {
			return err
		}
		deployment, getErr := s.deployments.deployments.GetByIDForProject(projectID, deploymentID)
		if getErr != nil {
			return getErr
		}
		if deployment == nil {
			return ErrDeploymentNotFound
		}
		current := strings.TrimSpace(deployment.Status)
		for _, allowed := range alreadySatisfiedStatuses {
			if current == allowed {
				return nil
			}
		}
		return err
	}
	return nil
}

func (s *RolloutExecutionService) promoteCandidateIfNeeded(ctx context.Context, projectID, deploymentID, revisionID string) (*PromotionResult, error) {
	deployment, err := s.deployments.deployments.GetByIDForProject(projectID, deploymentID)
	if err != nil {
		return nil, err
	}
	revision, err := s.deployments.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return nil, err
	}
	if deployment != nil && revision != nil &&
		strings.TrimSpace(deployment.Status) == DeploymentStatusPromoted &&
		strings.TrimSpace(revision.Status) == RevisionStatusPromoted {
		return &PromotionResult{
			RevisionID:   revisionID,
			DeploymentID: deploymentID,
			PromotedAt:   deployment.UpdatedAt,
		}, nil
	}
	return s.planner.PromoteCandidate(ctx, projectID, deploymentID, revisionID)
}

func (s *RolloutExecutionService) beginRecovery(deploymentID string) bool {
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	if _, exists := s.recovering[deploymentID]; exists {
		return false
	}
	s.recovering[deploymentID] = struct{}{}
	return true
}

func (s *RolloutExecutionService) endRecovery(deploymentID string) {
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	delete(s.recovering, deploymentID)
}

func findRolloutCommand(plan *RolloutPlan, commandType string) (runtime.AgentCommand, bool) {
	if plan == nil {
		return runtime.AgentCommand{}, false
	}
	for _, step := range plan.Steps {
		if step.Command.Type == commandType {
			return step.Command, true
		}
	}
	return runtime.AgentCommand{}, false
}

func isRetryablePromotionRecovery(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "promotion_candidate_not_ready") ||
		strings.Contains(message, "promotion_candidate_missing") ||
		strings.Contains(message, "timed out")
}

func (s *RolloutExecutionService) evaluateHealthGate(ctx context.Context, projectID, deploymentID, revisionID string) (*HealthGateResult, error) {
	if s.healthGateEvaluator != nil {
		return s.healthGateEvaluator(ctx, projectID, deploymentID, revisionID)
	}
	return s.planner.ExecuteHealthGate(ctx, projectID, deploymentID, revisionID)
}

func (s *RolloutExecutionService) rollbackDeployment(ctx context.Context, projectID, deploymentID, revisionID, agentID, correlationID string, result *RolloutExecutionResult) (*RollbackResult, error) {
	rollbackPlan, err := s.planner.PlanRollback(ctx, projectID, deploymentID)
	if err != nil {
		_ = s.failDeployment(projectID, deploymentID, revisionID)
		return nil, err
	}

	payload := cloneAnyMap(rollbackPlan.Payload)
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["failed_revision_id"] = rollbackPlan.FailedRevisionID
	payload["restored_revision_id"] = rollbackPlan.RestoredRevisionID

	cmd := enrichRolloutCommand(runtime.AgentCommand{
		Type:      runtime.CommandTypeRollbackRelease,
		ProjectID: projectID,
		Source:    "rollout_execution_service",
		Payload:   payload,
	}, projectID, revisionID, correlationID)
	cmdResult, err := s.dispatcher.DispatchCommand(ctx, agentID, cmd)
	if err != nil {
		_ = s.failDeployment(projectID, deploymentID, revisionID)
		return nil, err
	}
	result.DispatchedCommands = append(result.DispatchedCommands, cmd.Type)

	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Minute)
	tracked, waitErr := s.dispatcher.WaitForCommand(waitCtx, cmdResult.RequestID)
	waitCancel()
	if waitErr != nil {
		_ = s.failDeployment(projectID, deploymentID, revisionID)
		return nil, waitErr
	}
	if tracked == nil {
		_ = s.failDeployment(projectID, deploymentID, revisionID)
		return nil, fmt.Errorf("rollback command %q returned empty result", cmd.Type)
	}
	if tracked.State != CommandStateDone {
		_ = s.failDeployment(projectID, deploymentID, revisionID)
		if tracked.Error != "" {
			return nil, fmt.Errorf("rollback command %q failed: %s", cmd.Type, tracked.Error)
		}
		return nil, fmt.Errorf("rollback command %q returned state %q", cmd.Type, tracked.State)
	}
	s.broadcastRolloutProgress(deploymentID, revisionID, projectID, correlationID, -1, -1, cmd.Type, tracked)

	rollbackResult, err := s.planner.FinalizeRollback(ctx, projectID, deploymentID, rollbackPlan.FailedRevisionID, rollbackPlan.RestoredRevisionID)
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

func (s *RolloutExecutionService) broadcastRolloutProgress(deploymentID, revisionID, projectID, correlationID string, stepIndex, totalSteps int, commandType string, tracked *TrackedCommand) {
	if s == nil || s.operatorHub == nil || tracked == nil {
		return
	}
	payload := map[string]any{
		"deployment_id":  deploymentID,
		"revision_id":    revisionID,
		"project_id":     projectID,
		"correlation_id": correlationID,
		"command_type":   commandType,
		"state":          tracked.State,
		"summary":        tracked.Output["summary"],
		"step_index":     stepIndex,
		"total_steps":    totalSteps,
	}
	if len(tracked.Output) > 0 {
		payload["command_output"] = tracked.Output
		if rolloutProgress, ok := tracked.Output["rollout_progress"]; ok {
			payload["rollout_progress"] = rolloutProgress
		}
		if ingress, ok := tracked.Output["ingress_observations"]; ok {
			payload["ingress_observations"] = ingress
		}
	}
	_ = s.operatorHub.BroadcastEvent("deployment.rollout_progress", payload)
	if ingress, ok := tracked.Output["ingress_observations"]; ok {
		_ = s.operatorHub.BroadcastEvent("deployment.ingress_status", map[string]any{
			"deployment_id":        deploymentID,
			"revision_id":          revisionID,
			"project_id":           projectID,
			"correlation_id":       correlationID,
			"command_type":         commandType,
			"ingress_observations": ingress,
		})
	}
}
