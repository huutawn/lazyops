package service

import "lazyops-server/internal/models"

type ProjectInternalServiceService struct {
	projects         ProjectStore
	internalServices ProjectInternalServiceStore
}

func NewProjectInternalServiceService(projects ProjectStore, internalServices ProjectInternalServiceStore) *ProjectInternalServiceService {
	return &ProjectInternalServiceService{
		projects:         projects,
		internalServices: internalServices,
	}
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

	records := make([]ProjectInternalServiceRecord, 0, len(persisted))
	for _, item := range persisted {
		records = append(records, toProjectInternalServiceRecord(item))
	}

	return &ProjectInternalServiceListResult{Items: records}, nil
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
