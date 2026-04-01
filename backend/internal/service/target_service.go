package service

import (
	"errors"
	"net"
	"strings"

	"lazyops-server/internal/models"
	"lazyops-server/pkg/utils"
)

var (
	ErrMeshNetworkNameExists = errors.New("mesh network name already exists")
	ErrClusterNameExists     = errors.New("cluster name already exists")
	ErrInvalidProvider       = errors.New("invalid provider")
	ErrInvalidCIDR           = errors.New("invalid cidr")
)

type MeshNetworkService struct {
	meshNetworks MeshNetworkStore
}

type ClusterService struct {
	clusters ClusterStore
}

func NewMeshNetworkService(meshNetworks MeshNetworkStore) *MeshNetworkService {
	return &MeshNetworkService{meshNetworks: meshNetworks}
}

func NewClusterService(clusters ClusterStore) *ClusterService {
	return &ClusterService{clusters: clusters}
}

func (s *MeshNetworkService) Create(cmd CreateMeshNetworkCommand) (*MeshNetworkSummary, error) {
	userID := strings.TrimSpace(cmd.UserID)
	name := utils.NormalizeSpace(cmd.Name)
	if userID == "" || name == "" || len(name) > 255 {
		return nil, ErrInvalidInput
	}

	provider, err := normalizeMeshProvider(cmd.Provider)
	if err != nil {
		return nil, err
	}

	cidr, err := normalizeCIDR(cmd.CIDR)
	if err != nil {
		return nil, err
	}

	existing, err := s.meshNetworks.GetByNameForUser(userID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrMeshNetworkNameExists
	}

	mesh := &models.MeshNetwork{
		ID:       utils.NewPrefixedID("mesh"),
		UserID:   userID,
		Name:     name,
		Provider: provider,
		CIDR:     cidr,
		Status:   "provisioning",
	}
	if err := s.meshNetworks.Create(mesh); err != nil {
		return nil, err
	}

	summary := ToMeshNetworkSummary(*mesh)
	return &summary, nil
}

func (s *MeshNetworkService) List(userID string) (*MeshNetworkListResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.meshNetworks.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	resultItems := make([]MeshNetworkSummary, 0, len(items))
	for _, item := range items {
		resultItems = append(resultItems, ToMeshNetworkSummary(item))
	}

	return &MeshNetworkListResult{Items: resultItems}, nil
}

func (s *ClusterService) Create(cmd CreateClusterCommand) (*ClusterSummary, error) {
	userID := strings.TrimSpace(cmd.UserID)
	name := utils.NormalizeSpace(cmd.Name)
	secretRef := utils.NormalizeSpace(cmd.KubeconfigSecretRef)
	if userID == "" || name == "" || len(name) > 255 || secretRef == "" || strings.ContainsAny(secretRef, "\r\n\t") {
		return nil, ErrInvalidInput
	}

	provider, err := normalizeClusterProvider(cmd.Provider)
	if err != nil {
		return nil, err
	}

	existing, err := s.clusters.GetByNameForUser(userID, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrClusterNameExists
	}

	cluster := &models.Cluster{
		ID:                  utils.NewPrefixedID("cls"),
		UserID:              userID,
		Name:                name,
		Provider:            provider,
		KubeconfigSecretRef: secretRef,
		Status:              "validating",
	}
	if err := s.clusters.Create(cluster); err != nil {
		return nil, err
	}

	summary := ToClusterSummary(*cluster)
	return &summary, nil
}

func (s *ClusterService) List(userID string) (*ClusterListResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.clusters.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	resultItems := make([]ClusterSummary, 0, len(items))
	for _, item := range items {
		resultItems = append(resultItems, ToClusterSummary(item))
	}

	return &ClusterListResult{Items: resultItems}, nil
}

func ToMeshNetworkSummary(mesh models.MeshNetwork) MeshNetworkSummary {
	return MeshNetworkSummary{
		ID:         mesh.ID,
		TargetKind: "mesh",
		Name:       mesh.Name,
		Provider:   mesh.Provider,
		CIDR:       mesh.CIDR,
		Status:     normalizeMeshNetworkStatus(mesh.Status),
		CreatedAt:  mesh.CreatedAt,
		UpdatedAt:  mesh.UpdatedAt,
	}
}

func ToClusterSummary(cluster models.Cluster) ClusterSummary {
	return ClusterSummary{
		ID:         cluster.ID,
		TargetKind: "cluster",
		Name:       cluster.Name,
		Provider:   cluster.Provider,
		Status:     normalizeClusterStatus(cluster.Status),
		CreatedAt:  cluster.CreatedAt,
		UpdatedAt:  cluster.UpdatedAt,
	}
}

func normalizeMeshProvider(provider string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "wireguard":
		return "wireguard", nil
	case "tailscale":
		return "tailscale", nil
	default:
		return "", ErrInvalidProvider
	}
}

func normalizeClusterProvider(provider string) (string, error) {
	if strings.EqualFold(strings.TrimSpace(provider), "k3s") {
		return "k3s", nil
	}

	return "", ErrInvalidProvider
}

func normalizeCIDR(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ErrInvalidInput
	}

	_, ipNet, err := net.ParseCIDR(value)
	if err != nil {
		return "", ErrInvalidCIDR
	}

	return ipNet.String(), nil
}

func normalizeMeshNetworkStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "active":
		return "active"
	case "degraded":
		return "degraded"
	case "revoked":
		return "revoked"
	default:
		return "provisioning"
	}
}

func normalizeClusterStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "ready":
		return "ready"
	case "degraded":
		return "degraded"
	case "unreachable":
		return "unreachable"
	case "revoked":
		return "revoked"
	default:
		return "validating"
	}
}
