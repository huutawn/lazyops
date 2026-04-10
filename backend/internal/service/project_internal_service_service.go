package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lazyops-server/internal/models"
	"lazyops-server/internal/runtime"
	"lazyops-server/pkg/utils"
)

type InternalServiceProvisionDispatcher interface {
	DispatchCommand(ctx context.Context, agentID string, cmd runtime.AgentCommand) (*runtime.CommandResult, error)
	WaitForCommand(ctx context.Context, requestID string) (*TrackedCommand, error)
}

type ProjectInternalServiceService struct {
	projects         ProjectStore
	internalServices ProjectInternalServiceStore
	bindings         DeploymentBindingStore
	instances        InstanceStore
	dispatcher       InternalServiceProvisionDispatcher
	waitTimeout      time.Duration
}

func NewProjectInternalServiceService(projects ProjectStore, internalServices ProjectInternalServiceStore) *ProjectInternalServiceService {
	return &ProjectInternalServiceService{
		projects:         projects,
		internalServices: internalServices,
		waitTimeout:      2 * time.Minute,
	}
}

func (s *ProjectInternalServiceService) WithRuntimeProvisioner(bindings DeploymentBindingStore, instances InstanceStore, dispatcher InternalServiceProvisionDispatcher) *ProjectInternalServiceService {
	if s == nil {
		return s
	}
	s.bindings = bindings
	s.instances = instances
	s.dispatcher = dispatcher
	return s
}

func (s *ProjectInternalServiceService) Configure(cmd ConfigureProjectInternalServicesCommand) (*ProjectInternalServiceListResult, error) {
	project, err := resolveProjectForAccess(s.projects, cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}

	items, err := buildProjectInternalServiceModels(project.ID, cmd.Kinds)
	if err != nil {
		return nil, err
	}
	if err := s.internalServices.ReplaceForProject(project.ID, items); err != nil {
		return nil, err
	}

	persisted, err := s.internalServices.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}
	if err := s.provisionInternalServices(project.ID, persisted); err != nil {
		return nil, err
	}

	records := make([]ProjectInternalServiceRecord, 0, len(persisted))
	for _, item := range persisted {
		records = append(records, toProjectInternalServiceRecord(item))
	}

	return &ProjectInternalServiceListResult{Items: records}, nil
}

func (s *ProjectInternalServiceService) provisionInternalServices(projectID string, persisted []models.ProjectInternalService) error {
	if s == nil || s.dispatcher == nil || s.bindings == nil || s.instances == nil {
		return nil
	}

	binding, err := s.resolvePrimaryBinding(projectID)
	if err != nil {
		return err
	}
	if binding == nil {
		return nil
	}
	if strings.TrimSpace(binding.TargetKind) != "instance" {
		return fmt.Errorf("%w: internal services hiện chỉ hỗ trợ target instance", ErrInvalidInput)
	}

	instance, err := s.instances.GetByID(strings.TrimSpace(binding.TargetID))
	if err != nil {
		return err
	}
	if instance == nil || instance.AgentID == nil || strings.TrimSpace(*instance.AgentID) == "" {
		return fmt.Errorf("%w: agent của máy chủ chưa sẵn sàng để cài dịch vụ nội bộ", ErrInvalidInput)
	}

	payload := runtime.ProvisionInternalServicesPayload{
		ProjectID: projectID,
		BindingID: binding.ID,
		Services:  make([]runtime.InternalServiceProvisionSpec, 0, len(persisted)),
	}
	for _, item := range persisted {
		payload.Services = append(payload.Services, runtime.InternalServiceProvisionSpec{
			Kind:          item.Kind,
			Alias:         item.Alias,
			Protocol:      item.Protocol,
			Port:          item.Port,
			LocalEndpoint: item.LocalEndpoint,
		})
	}

	payloadMap, err := toRuntimePayloadMap(payload)
	if err != nil {
		return err
	}

	requestID := utils.NewPrefixedID("req")
	command := runtime.AgentCommand{
		Type:          runtime.CommandTypeProvisionInternalSvc,
		RequestID:     requestID,
		CorrelationID: utils.NewCorrelationID(),
		ProjectID:     projectID,
		Source:        "project_internal_services",
		Payload:       payloadMap,
	}

	result, err := s.dispatcher.DispatchCommand(context.Background(), strings.TrimSpace(*instance.AgentID), command)
	if err != nil {
		return fmt.Errorf("%w: không thể gửi lệnh cài dịch vụ nội bộ tới agent (%v)", ErrInvalidInput, err)
	}
	if result == nil || strings.TrimSpace(result.RequestID) == "" {
		return fmt.Errorf("%w: lệnh cài dịch vụ nội bộ không có request_id", ErrInvalidInput)
	}

	timeout := s.waitTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	tracked, err := s.dispatcher.WaitForCommand(waitCtx, result.RequestID)
	if err != nil {
		return fmt.Errorf("%w: agent không phản hồi lệnh cài dịch vụ nội bộ (%v)", ErrInvalidInput, err)
	}
	if tracked == nil {
		return fmt.Errorf("%w: agent trả về kết quả rỗng cho lệnh cài dịch vụ nội bộ", ErrInvalidInput)
	}
	if tracked.State != CommandStateDone {
		if strings.TrimSpace(tracked.Error) != "" {
			return fmt.Errorf("%w: %s", ErrInvalidInput, tracked.Error)
		}
		return fmt.Errorf("%w: lệnh cài dịch vụ nội bộ chưa hoàn tất (state=%s)", ErrInvalidInput, tracked.State)
	}

	return nil
}

func (s *ProjectInternalServiceService) resolvePrimaryBinding(projectID string) (*models.DeploymentBinding, error) {
	autoBinding, err := s.bindings.GetByTargetRefForProject(projectID, "auto-primary")
	if err != nil {
		return nil, err
	}
	if autoBinding != nil {
		return autoBinding, nil
	}

	items, err := s.bindings.ListByProject(projectID)
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

func toRuntimePayloadMap(payload runtime.ProvisionInternalServicesPayload) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ProjectInternalServiceService) List(requesterUserID, requesterRole, projectID string) (*ProjectInternalServiceListResult, error) {
	project, err := resolveProjectForAccess(s.projects, requesterUserID, requesterRole, projectID)
	if err != nil {
		return nil, err
	}

	persisted, err := s.internalServices.ListByProject(project.ID)
	if err != nil {
		return nil, err
	}

	records := make([]ProjectInternalServiceRecord, 0, len(persisted))
	for _, item := range persisted {
		records = append(records, toProjectInternalServiceRecord(item))
	}

	return &ProjectInternalServiceListResult{Items: records}, nil
}

func buildInternalServicesDependencyBindings(services []LazyopsYAMLService, internalServices []models.ProjectInternalService) ([]LazyopsYAMLService, []LazyopsYAMLDependencyBinding) {
	if len(internalServices) == 0 {
		servicesCopy := make([]LazyopsYAMLService, len(services))
		copy(servicesCopy, services)
		return servicesCopy, []LazyopsYAMLDependencyBinding{}
	}

	servicesCopy := make([]LazyopsYAMLService, 0, len(services)+len(internalServices))
	servicesCopy = append(servicesCopy, services...)

	targetByKind := make(map[string]string, len(internalServices))
	targetNames := make(map[string]struct{}, len(internalServices))
	existingServiceNames := make(map[string]struct{}, len(services))
	for _, service := range services {
		existingServiceNames[service.Name] = struct{}{}
	}
	for _, item := range internalServices {
		targetService := internalServiceTargetServiceName(item.Kind)
		targetByKind[item.Kind] = targetService
		targetNames[targetService] = struct{}{}
		if _, exists := existingServiceNames[targetService]; exists {
			continue
		}
		servicesCopy = append(servicesCopy, LazyopsYAMLService{
			Name:   targetService,
			Path:   ".lazyops/internal/" + item.Kind,
			Public: false,
		})
		existingServiceNames[targetService] = struct{}{}
	}

	dependencies := make([]LazyopsYAMLDependencyBinding, 0, len(services)*len(internalServices))
	for _, service := range services {
		if _, isInternalSynthetic := targetNames[service.Name]; isInternalSynthetic {
			continue
		}
		if service.Path == "" {
			continue
		}
		for _, item := range internalServices {
			targetService := targetByKind[item.Kind]
			dependencies = append(dependencies, LazyopsYAMLDependencyBinding{
				Service:       service.Name,
				Alias:         item.Alias,
				TargetService: targetService,
				Protocol:      item.Protocol,
				LocalEndpoint: item.LocalEndpoint,
			})
		}
	}

	return servicesCopy, dependencies
}
