package service

import (
	"errors"
	"sort"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeMeshNetworkStore struct {
	byID       map[string]*models.MeshNetwork
	byUserName map[string]*models.MeshNetwork
	createErr  error
	listErr    error
	getErr     error
}

type fakeClusterStore struct {
	byID       map[string]*models.Cluster
	byUserName map[string]*models.Cluster
	createErr  error
	listErr    error
	getErr     error
}

func newFakeMeshNetworkStore(items ...*models.MeshNetwork) *fakeMeshNetworkStore {
	store := &fakeMeshNetworkStore{
		byID:       make(map[string]*models.MeshNetwork),
		byUserName: make(map[string]*models.MeshNetwork),
	}

	for _, item := range items {
		cloned := *item
		store.byID[cloned.ID] = &cloned
		store.byUserName[cloned.UserID+":"+cloned.Name] = &cloned
	}

	return store
}

func newFakeClusterStore(items ...*models.Cluster) *fakeClusterStore {
	store := &fakeClusterStore{
		byID:       make(map[string]*models.Cluster),
		byUserName: make(map[string]*models.Cluster),
	}

	for _, item := range items {
		cloned := *item
		store.byID[cloned.ID] = &cloned
		store.byUserName[cloned.UserID+":"+cloned.Name] = &cloned
	}

	return store
}

func (f *fakeMeshNetworkStore) Create(mesh *models.MeshNetwork) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *mesh
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	f.byID[cloned.ID] = &cloned
	f.byUserName[cloned.UserID+":"+cloned.Name] = &cloned
	mesh.CreatedAt = cloned.CreatedAt
	mesh.UpdatedAt = cloned.UpdatedAt
	return nil
}

func (f *fakeMeshNetworkStore) ListByUser(userID string) ([]models.MeshNetwork, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	items := make([]models.MeshNetwork, 0)
	for _, item := range f.byID {
		if item.UserID == userID {
			items = append(items, *item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func (f *fakeMeshNetworkStore) GetByNameForUser(userID, name string) (*models.MeshNetwork, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if item, ok := f.byUserName[userID+":"+name]; ok {
		return item, nil
	}
	return nil, nil
}

func (f *fakeMeshNetworkStore) GetByIDForUser(userID, meshID string) (*models.MeshNetwork, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	item, ok := f.byID[meshID]
	if !ok || item.UserID != userID {
		return nil, nil
	}
	return item, nil
}

func (f *fakeMeshNetworkStore) GetByID(meshID string) (*models.MeshNetwork, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	item, ok := f.byID[meshID]
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (f *fakeClusterStore) Create(cluster *models.Cluster) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *cluster
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	f.byID[cloned.ID] = &cloned
	f.byUserName[cloned.UserID+":"+cloned.Name] = &cloned
	cluster.CreatedAt = cloned.CreatedAt
	cluster.UpdatedAt = cloned.UpdatedAt
	return nil
}

func (f *fakeClusterStore) ListByUser(userID string) ([]models.Cluster, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}

	items := make([]models.Cluster, 0)
	for _, item := range f.byID {
		if item.UserID == userID {
			items = append(items, *item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func (f *fakeClusterStore) GetByNameForUser(userID, name string) (*models.Cluster, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if item, ok := f.byUserName[userID+":"+name]; ok {
		return item, nil
	}
	return nil, nil
}

func (f *fakeClusterStore) GetByIDForUser(userID, clusterID string) (*models.Cluster, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	item, ok := f.byID[clusterID]
	if !ok || item.UserID != userID {
		return nil, nil
	}
	return item, nil
}

func (f *fakeClusterStore) GetByID(clusterID string) (*models.Cluster, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	item, ok := f.byID[clusterID]
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (f *fakeClusterStore) UpdateStatus(clusterID, status string, at time.Time) error {
	if item, ok := f.byID[clusterID]; ok {
		item.Status = status
		item.UpdatedAt = at
	}
	return nil
}

func TestMeshNetworkServiceCreateAndList(t *testing.T) {
	store := newFakeMeshNetworkStore()
	service := NewMeshNetworkService(store)

	created, err := service.Create(CreateMeshNetworkCommand{
		UserID:   "usr_123",
		Name:     "  mesh prod ap  ",
		Provider: "WireGuard",
		CIDR:     "10.20.0.0/24",
	})
	if err != nil {
		t.Fatalf("create mesh network: %v", err)
	}
	if created.TargetKind != "mesh" {
		t.Fatalf("expected target kind mesh, got %q", created.TargetKind)
	}
	if created.Provider != "wireguard" {
		t.Fatalf("expected normalized provider wireguard, got %q", created.Provider)
	}
	if created.CIDR != "10.20.0.0/24" {
		t.Fatalf("expected canonical cidr, got %q", created.CIDR)
	}
	if created.Status != "provisioning" {
		t.Fatalf("expected provisioning status, got %q", created.Status)
	}

	listed, err := service.List("usr_123")
	if err != nil {
		t.Fatalf("list mesh networks: %v", err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("expected one mesh network, got %d", len(listed.Items))
	}
	if listed.Items[0].ID != created.ID {
		t.Fatalf("expected listed mesh id %q, got %q", created.ID, listed.Items[0].ID)
	}
}

func TestClusterServiceCreateAndList(t *testing.T) {
	store := newFakeClusterStore()
	service := NewClusterService(store)

	created, err := service.Create(CreateClusterCommand{
		UserID:              "usr_123",
		Name:                "  prod cluster  ",
		Provider:            "K3S",
		KubeconfigSecretRef: "secret://clusters/prod",
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if created.TargetKind != "cluster" {
		t.Fatalf("expected target kind cluster, got %q", created.TargetKind)
	}
	if created.Provider != "k3s" {
		t.Fatalf("expected normalized provider k3s, got %q", created.Provider)
	}
	if created.Status != "validating" {
		t.Fatalf("expected validating status, got %q", created.Status)
	}

	listed, err := service.List("usr_123")
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("expected one cluster, got %d", len(listed.Items))
	}
	if listed.Items[0].ID != created.ID {
		t.Fatalf("expected listed cluster id %q, got %q", created.ID, listed.Items[0].ID)
	}
}

func TestTargetServicesRejectInvalidProvider(t *testing.T) {
	meshService := NewMeshNetworkService(newFakeMeshNetworkStore())
	if _, err := meshService.Create(CreateMeshNetworkCommand{
		UserID:   "usr_123",
		Name:     "mesh-prod",
		Provider: "openvpn",
		CIDR:     "10.20.0.0/24",
	}); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("expected ErrInvalidProvider for mesh network, got %v", err)
	}

	clusterService := NewClusterService(newFakeClusterStore())
	if _, err := clusterService.Create(CreateClusterCommand{
		UserID:              "usr_123",
		Name:                "cluster-prod",
		Provider:            "eks",
		KubeconfigSecretRef: "secret://clusters/prod",
	}); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("expected ErrInvalidProvider for cluster, got %v", err)
	}
}

func TestTargetServicesScopeListsByOwner(t *testing.T) {
	meshService := NewMeshNetworkService(newFakeMeshNetworkStore(
		&models.MeshNetwork{
			ID:        "mesh_owner",
			UserID:    "usr_owner",
			Name:      "mesh-owner",
			Provider:  "wireguard",
			CIDR:      "10.20.0.0/24",
			Status:    "active",
			CreatedAt: time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 1, 9, 5, 0, 0, time.UTC),
		},
		&models.MeshNetwork{
			ID:        "mesh_other",
			UserID:    "usr_other",
			Name:      "mesh-other",
			Provider:  "tailscale",
			CIDR:      "10.21.0.0/24",
			Status:    "active",
			CreatedAt: time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 1, 10, 5, 0, 0, time.UTC),
		},
	))
	clusterService := NewClusterService(newFakeClusterStore(
		&models.Cluster{
			ID:                  "cls_owner",
			UserID:              "usr_owner",
			Name:                "cluster-owner",
			Provider:            "k3s",
			KubeconfigSecretRef: "secret://clusters/owner",
			Status:              "ready",
			CreatedAt:           time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC),
			UpdatedAt:           time.Date(2026, 4, 1, 9, 5, 0, 0, time.UTC),
		},
		&models.Cluster{
			ID:                  "cls_other",
			UserID:              "usr_other",
			Name:                "cluster-other",
			Provider:            "k3s",
			KubeconfigSecretRef: "secret://clusters/other",
			Status:              "ready",
			CreatedAt:           time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
			UpdatedAt:           time.Date(2026, 4, 1, 10, 5, 0, 0, time.UTC),
		},
	))

	meshes, err := meshService.List("usr_owner")
	if err != nil {
		t.Fatalf("list owner meshes: %v", err)
	}
	if len(meshes.Items) != 1 || meshes.Items[0].ID != "mesh_owner" {
		t.Fatalf("expected only owner mesh, got %#v", meshes.Items)
	}

	clusters, err := clusterService.List("usr_owner")
	if err != nil {
		t.Fatalf("list owner clusters: %v", err)
	}
	if len(clusters.Items) != 1 || clusters.Items[0].ID != "cls_owner" {
		t.Fatalf("expected only owner cluster, got %#v", clusters.Items)
	}
}
