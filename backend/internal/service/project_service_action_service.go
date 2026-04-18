package service

import (
	"context"
	"fmt"
	"strings"

	"lazyops-server/internal/models"
	backendruntime "lazyops-server/internal/runtime"
)

type ProjectServiceActionService struct {
	projects    ProjectStore
	services    ProjectServiceStore
	deployments *DeploymentService
	rollouts    *RolloutExecutionService
	bindings    DeploymentBindingStore
	clusters    ClusterStore
	instances   InstanceStore
	control     *ControlService
}

func NewProjectServiceActionService(
	projects ProjectStore,
	services ProjectServiceStore,
	deployments *DeploymentService,
	rollouts *RolloutExecutionService,
	bindings DeploymentBindingStore,
	clusters ClusterStore,
	instances InstanceStore,
	control *ControlService,
) *ProjectServiceActionService {
	return &ProjectServiceActionService{
		projects:    projects,
		services:    services,
		deployments: deployments,
		rollouts:    rollouts,
		bindings:    bindings,
		clusters:    clusters,
		instances:   instances,
		control:     control,
	}
}

func (s *ProjectServiceActionService) Act(ctx context.Context, requesterUserID, requesterRole, projectID, serviceID, action string) (*ProjectServiceActionResult, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}
	serviceRecord, err := s.resolveProjectService(project.ID, serviceID)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "deploy":
		return s.createAndStartDeployment(ctx, *project, serviceRecord, "service_deploy")
	case "rebuild":
		if strings.TrimSpace(serviceRecord.SourceType) != serviceSourceTypeRepo {
			return nil, ErrInvalidInput
		}
		return s.createAndStartDeployment(ctx, *project, serviceRecord, "service_rebuild")
	case "restart":
		return s.restartService(ctx, *project, serviceRecord)
	default:
		return nil, ErrInvalidInput
	}
}

func (s *ProjectServiceActionService) createAndStartDeployment(ctx context.Context, project models.Project, serviceRecord ProjectServiceRecord, triggerKind string) (*ProjectServiceActionResult, error) {
	result, err := s.deployments.Create(CreateDeploymentCommand{
		RequesterUserID: project.UserID,
		RequesterRole:   RoleAdmin,
		ProjectID:       project.ID,
		TriggerKind:     triggerKind,
		ServiceIDs:      []string{serviceRecord.ID},
	})
	if err != nil {
		return nil, err
	}
	if s.rollouts != nil {
		if _, err := s.rollouts.StartDeployment(ctx, project.ID, result.Deployment.ID); err != nil {
			return nil, err
		}
	}
	return &ProjectServiceActionResult{
		Action:       triggerKind,
		ServiceID:    serviceRecord.ID,
		ServiceName:  serviceRecord.Name,
		Status:       "started",
		TriggerKind:  triggerKind,
		DeploymentID: result.Deployment.ID,
		RevisionID:   result.Revision.ID,
		Message:      fmt.Sprintf("service %s deployment started", serviceRecord.Name),
	}, nil
}

func (s *ProjectServiceActionService) restartService(ctx context.Context, project models.Project, serviceRecord ProjectServiceRecord) (*ProjectServiceActionResult, error) {
	if s.bindings == nil || s.clusters == nil || s.instances == nil || s.control == nil {
		return nil, ErrInvalidInput
	}
	binding, err := s.resolvePrimaryClusterBinding(project.ID)
	if err != nil {
		return nil, err
	}
	if binding == nil || strings.TrimSpace(binding.TargetKind) != "cluster" {
		return nil, ErrTargetNotFound
	}
	cluster, err := s.clusters.GetByID(binding.TargetID)
	if err != nil {
		return nil, err
	}
	if cluster == nil || cluster.InstanceID == nil || strings.TrimSpace(*cluster.InstanceID) == "" {
		return nil, ErrRolloutAgentUnavailable
	}
	instance, err := s.instances.GetByID(strings.TrimSpace(*cluster.InstanceID))
	if err != nil {
		return nil, err
	}
	if instance == nil || instance.AgentID == nil || strings.TrimSpace(*instance.AgentID) == "" {
		return nil, ErrRolloutAgentUnavailable
	}
	command, err := s.control.DispatchCommand(ctx, strings.TrimSpace(*instance.AgentID), backendruntime.AgentCommand{
		Type:      backendruntime.CommandTypeRestartK3sService,
		Source:    "project_service_actions",
		ProjectID: project.ID,
		Payload: map[string]any{
			"namespace":    project.NamespaceSlug,
			"service_name": serviceRecord.Name,
		},
	})
	if err != nil {
		return nil, err
	}
	tracked, err := s.control.WaitForCommand(ctx, command.RequestID)
	if err != nil {
		return nil, err
	}
	if tracked.State != CommandStateDone {
		if tracked.Error != "" {
			return nil, fmt.Errorf("restart service: %s", tracked.Error)
		}
		return nil, fmt.Errorf("restart service command %s ended in state %s", tracked.RequestID, tracked.State)
	}
	return &ProjectServiceActionResult{
		Action:      "restart",
		ServiceID:   serviceRecord.ID,
		ServiceName: serviceRecord.Name,
		Status:      "completed",
		Message:     fmt.Sprintf("service %s restarted", serviceRecord.Name),
	}, nil
}

func (s *ProjectServiceActionService) resolveProjectService(projectID, serviceID string) (ProjectServiceRecord, error) {
	items, err := s.services.ListByProject(projectID)
	if err != nil {
		return ProjectServiceRecord{}, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) != strings.TrimSpace(serviceID) {
			continue
		}
		return ToProjectServiceRecord(item)
	}
	return ProjectServiceRecord{}, ErrInvalidInput
}

func (s *ProjectServiceActionService) resolvePrimaryClusterBinding(projectID string) (*models.DeploymentBinding, error) {
	autoBinding, err := s.bindings.GetByTargetRefForProject(projectID, "auto-primary")
	if err != nil {
		return nil, err
	}
	if autoBinding != nil && strings.TrimSpace(autoBinding.TargetKind) == "cluster" {
		return autoBinding, nil
	}
	items, err := s.bindings.ListByProject(projectID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.TargetKind) == "cluster" {
			binding := item
			return &binding, nil
		}
	}
	return nil, nil
}
