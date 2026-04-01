package service

import (
	"errors"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var (
	ErrTargetNotFound      = errors.New("target not found")
	ErrTargetAccessDenied  = errors.New("target access denied")
	ErrDuplicateTargetRef  = errors.New("duplicate target ref")
	ErrRuntimeModeMismatch = errors.New("runtime mode mismatch")
)

type DeploymentBindingService struct {
	projects  ProjectStore
	bindings  DeploymentBindingStore
	instances InstanceStore
	meshes    MeshNetworkStore
	clusters  ClusterStore
}

func NewDeploymentBindingService(
	projects ProjectStore,
	bindings DeploymentBindingStore,
	instances InstanceStore,
	meshes MeshNetworkStore,
	clusters ClusterStore,
) *DeploymentBindingService {
	return &DeploymentBindingService{
		projects:  projects,
		bindings:  bindings,
		instances: instances,
		meshes:    meshes,
		clusters:  clusters,
	}
}

func (s *DeploymentBindingService) Create(cmd CreateDeploymentBindingCommand) (*DeploymentBindingRecord, error) {
	project, err := s.resolveProjectForWrite(cmd.RequesterUserID, cmd.RequesterRole, cmd.ProjectID)
	if err != nil {
		return nil, err
	}

	name := utils.NormalizeSpace(cmd.Name)
	if name == "" || len(name) > 255 {
		return nil, ErrInvalidInput
	}

	targetRefSource := cmd.TargetRef
	if strings.TrimSpace(targetRefSource) == "" {
		targetRefSource = name
	}
	targetRef := normalizeBindingTargetRef(targetRefSource)
	if targetRef == "" {
		return nil, ErrInvalidInput
	}

	runtimeMode, err := normalizeBindingRuntimeMode(cmd.RuntimeMode)
	if err != nil {
		return nil, err
	}

	targetKind, err := normalizeBindingTargetKind(cmd.TargetKind)
	if err != nil {
		return nil, err
	}

	targetID := strings.TrimSpace(cmd.TargetID)
	if targetID == "" {
		return nil, ErrInvalidInput
	}

	existing, err := s.bindings.GetByTargetRefForProject(project.ID, targetRef)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDuplicateTargetRef
	}

	targetOwnerID, err := s.resolveTargetOwnerID(cmd.RequesterUserID, cmd.RequesterRole, targetKind, targetID)
	if err != nil {
		return nil, err
	}
	if targetOwnerID != project.UserID {
		return nil, ErrTargetAccessDenied
	}
	if err := validateBindingModeCompatibility(runtimeMode, targetKind); err != nil {
		return nil, err
	}

	placementPolicyJSON, err := marshalBindingPolicyJSON(cmd.PlacementPolicy)
	if err != nil {
		return nil, err
	}
	domainPolicyJSON, err := marshalBindingPolicyJSON(cmd.DomainPolicy)
	if err != nil {
		return nil, err
	}
	compatibilityPolicyJSON, err := marshalBindingPolicyJSON(cmd.CompatibilityPolicy)
	if err != nil {
		return nil, err
	}
	scaleToZeroPolicyJSON, err := marshalBindingPolicyJSON(cmd.ScaleToZeroPolicy)
	if err != nil {
		return nil, err
	}

	binding := &models.DeploymentBinding{
		ID:                      utils.NewPrefixedID("bind"),
		ProjectID:               project.ID,
		Name:                    name,
		TargetRef:               targetRef,
		RuntimeMode:             runtimeMode,
		TargetKind:              targetKind,
		TargetID:                targetID,
		PlacementPolicyJSON:     placementPolicyJSON,
		DomainPolicyJSON:        domainPolicyJSON,
		CompatibilityPolicyJSON: compatibilityPolicyJSON,
		ScaleToZeroPolicyJSON:   scaleToZeroPolicyJSON,
	}
	if err := s.bindings.Create(binding); err != nil {
		return nil, err
	}

	record, err := ToDeploymentBindingRecord(*binding)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (s *DeploymentBindingService) resolveProjectForWrite(requesterUserID, requesterRole, projectID string) (*models.Project, error) {
	requesterUserID = strings.TrimSpace(requesterUserID)
	projectID = strings.TrimSpace(projectID)
	if requesterUserID == "" || projectID == "" {
		return nil, ErrInvalidInput
	}

	if requesterRole == RoleAdmin {
		project, err := s.projects.GetByID(projectID)
		if err != nil {
			return nil, err
		}
		if project == nil {
			return nil, ErrProjectNotFound
		}
		return project, nil
	}

	project, err := s.projects.GetByIDForUser(requesterUserID, projectID)
	if err != nil {
		return nil, err
	}
	if project != nil {
		return project, nil
	}

	otherProject, err := s.projects.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if otherProject == nil {
		return nil, ErrProjectNotFound
	}

	return nil, ErrProjectAccessDenied
}

func (s *DeploymentBindingService) resolveTargetOwnerID(requesterUserID, requesterRole, targetKind, targetID string) (string, error) {
	switch targetKind {
	case "instance":
		return resolveInstanceOwnerID(s.instances, requesterUserID, requesterRole, targetID)
	case "mesh":
		return resolveMeshOwnerID(s.meshes, requesterUserID, requesterRole, targetID)
	case "cluster":
		return resolveClusterOwnerID(s.clusters, requesterUserID, requesterRole, targetID)
	default:
		return "", ErrInvalidInput
	}
}

func resolveInstanceOwnerID(store InstanceStore, requesterUserID, requesterRole, targetID string) (string, error) {
	var instance *models.Instance
	var err error
	if requesterRole == RoleAdmin {
		instance, err = store.GetByID(targetID)
	} else {
		instance, err = store.GetByIDForUser(strings.TrimSpace(requesterUserID), targetID)
	}
	if err != nil {
		return "", err
	}
	if instance != nil {
		return instance.UserID, nil
	}
	if requesterRole == RoleAdmin {
		return "", ErrTargetNotFound
	}

	otherInstance, err := store.GetByID(targetID)
	if err != nil {
		return "", err
	}
	if otherInstance == nil {
		return "", ErrTargetNotFound
	}

	return "", ErrTargetAccessDenied
}

func resolveMeshOwnerID(store MeshNetworkStore, requesterUserID, requesterRole, targetID string) (string, error) {
	var mesh *models.MeshNetwork
	var err error
	if requesterRole == RoleAdmin {
		mesh, err = store.GetByID(targetID)
	} else {
		mesh, err = store.GetByIDForUser(strings.TrimSpace(requesterUserID), targetID)
	}
	if err != nil {
		return "", err
	}
	if mesh != nil {
		return mesh.UserID, nil
	}
	if requesterRole == RoleAdmin {
		return "", ErrTargetNotFound
	}

	otherMesh, err := store.GetByID(targetID)
	if err != nil {
		return "", err
	}
	if otherMesh == nil {
		return "", ErrTargetNotFound
	}

	return "", ErrTargetAccessDenied
}

func resolveClusterOwnerID(store ClusterStore, requesterUserID, requesterRole, targetID string) (string, error) {
	var cluster *models.Cluster
	var err error
	if requesterRole == RoleAdmin {
		cluster, err = store.GetByID(targetID)
	} else {
		cluster, err = store.GetByIDForUser(strings.TrimSpace(requesterUserID), targetID)
	}
	if err != nil {
		return "", err
	}
	if cluster != nil {
		return cluster.UserID, nil
	}
	if requesterRole == RoleAdmin {
		return "", ErrTargetNotFound
	}

	otherCluster, err := store.GetByID(targetID)
	if err != nil {
		return "", err
	}
	if otherCluster == nil {
		return "", ErrTargetNotFound
	}

	return "", ErrTargetAccessDenied
}

func normalizeBindingRuntimeMode(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "standalone":
		return "standalone", nil
	case "distributed-mesh":
		return "distributed-mesh", nil
	case "distributed-k3s":
		return "distributed-k3s", nil
	default:
		return "", ErrInvalidInput
	}
}

func normalizeBindingTargetKind(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "instance":
		return "instance", nil
	case "mesh":
		return "mesh", nil
	case "cluster":
		return "cluster", nil
	default:
		return "", ErrInvalidInput
	}
}

func normalizeBindingTargetRef(input string) string {
	return normalizeProjectSlug(input)
}

func validateBindingModeCompatibility(runtimeMode, targetKind string) error {
	switch targetKind {
	case "instance":
		if runtimeMode == "standalone" || runtimeMode == "distributed-mesh" {
			return nil
		}
	case "mesh":
		if runtimeMode == "distributed-mesh" {
			return nil
		}
	case "cluster":
		if runtimeMode == "distributed-k3s" {
			return nil
		}
	}

	return ErrRuntimeModeMismatch
}

func marshalBindingPolicyJSON(policy map[string]any) (string, error) {
	return marshalCapabilitiesJSON(policy)
}

func ToDeploymentBindingRecord(binding models.DeploymentBinding) (DeploymentBindingRecord, error) {
	placementPolicy, err := decodeAnyMapJSON(binding.PlacementPolicyJSON)
	if err != nil {
		return DeploymentBindingRecord{}, err
	}
	domainPolicy, err := decodeAnyMapJSON(binding.DomainPolicyJSON)
	if err != nil {
		return DeploymentBindingRecord{}, err
	}
	compatibilityPolicy, err := decodeAnyMapJSON(binding.CompatibilityPolicyJSON)
	if err != nil {
		return DeploymentBindingRecord{}, err
	}
	scaleToZeroPolicy, err := decodeAnyMapJSON(binding.ScaleToZeroPolicyJSON)
	if err != nil {
		return DeploymentBindingRecord{}, err
	}

	return DeploymentBindingRecord{
		ID:                  binding.ID,
		ProjectID:           binding.ProjectID,
		Name:                binding.Name,
		TargetRef:           binding.TargetRef,
		RuntimeMode:         binding.RuntimeMode,
		TargetKind:          binding.TargetKind,
		TargetID:            binding.TargetID,
		PlacementPolicy:     placementPolicy,
		DomainPolicy:        domainPolicy,
		CompatibilityPolicy: compatibilityPolicy,
		ScaleToZeroPolicy:   scaleToZeroPolicy,
		CreatedAt:           binding.CreatedAt,
		UpdatedAt:           binding.UpdatedAt,
	}, nil
}
