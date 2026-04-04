package service

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

type fakeDeploymentBindingStore struct {
	byProjectTargetRef map[string]*models.DeploymentBinding
	createErr          error
	getErr             error
}

func newFakeDeploymentBindingStore(bindings ...*models.DeploymentBinding) *fakeDeploymentBindingStore {
	store := &fakeDeploymentBindingStore{
		byProjectTargetRef: make(map[string]*models.DeploymentBinding),
	}

	for _, binding := range bindings {
		cloned := *binding
		store.byProjectTargetRef[binding.ProjectID+":"+binding.TargetRef] = &cloned
	}

	return store
}

func (f *fakeDeploymentBindingStore) Create(binding *models.DeploymentBinding) error {
	if f.createErr != nil {
		return f.createErr
	}

	cloned := *binding
	now := time.Now().UTC()
	if cloned.CreatedAt.IsZero() {
		cloned.CreatedAt = now
	}
	if cloned.UpdatedAt.IsZero() {
		cloned.UpdatedAt = cloned.CreatedAt
	}
	f.byProjectTargetRef[cloned.ProjectID+":"+cloned.TargetRef] = &cloned
	binding.CreatedAt = cloned.CreatedAt
	binding.UpdatedAt = cloned.UpdatedAt
	return nil
}

func (f *fakeDeploymentBindingStore) ListByProject(projectID string) ([]models.DeploymentBinding, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	items := make([]models.DeploymentBinding, 0)
	for key, binding := range f.byProjectTargetRef {
		if !strings.HasPrefix(key, projectID+":") {
			continue
		}
		items = append(items, *binding)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].TargetRef < items[j].TargetRef
	})

	return items, nil
}

func (f *fakeDeploymentBindingStore) GetByTargetRefForProject(projectID, targetRef string) (*models.DeploymentBinding, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if binding, ok := f.byProjectTargetRef[projectID+":"+targetRef]; ok {
		return binding, nil
	}

	return nil, nil
}

func TestDeploymentBindingServiceCreateSuccess(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:                      "inst_123",
		UserID:                  "usr_123",
		Name:                    "edge-sg-1",
		PublicIP:                ptrString("203.0.113.10"),
		Status:                  "online",
		LabelsJSON:              `{"region":"sg"}`,
		RuntimeCapabilitiesJSON: `{}`,
	})
	bindingStore := newFakeDeploymentBindingStore()
	service := NewDeploymentBindingService(projectStore, bindingStore, instanceStore, newFakeMeshNetworkStore(), newFakeClusterStore())

	record, err := service.Create(CreateDeploymentBindingCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Name:            "  Production Binding  ",
		TargetRef:       "Prod Main",
		RuntimeMode:     "standalone",
		TargetKind:      "instance",
		TargetID:        "inst_123",
		PlacementPolicy: map[string]any{
			"strategy": "single-node",
		},
		DomainPolicy: map[string]any{
			"magic_domain_provider": "sslip.io",
		},
		CompatibilityPolicy: map[string]any{
			"localhost_rescue": true,
		},
		ScaleToZeroPolicy: map[string]any{
			"enabled": false,
		},
	})
	if err != nil {
		t.Fatalf("create deployment binding: %v", err)
	}

	if record.ID == "" || record.ID[:5] != "bind_" {
		t.Fatalf("expected bind_ prefixed id, got %q", record.ID)
	}
	if record.Name != "Production Binding" {
		t.Fatalf("expected normalized binding name, got %q", record.Name)
	}
	if record.TargetRef != "prod-main" {
		t.Fatalf("expected normalized target ref prod-main, got %q", record.TargetRef)
	}
	if record.RuntimeMode != "standalone" {
		t.Fatalf("expected runtime mode standalone, got %q", record.RuntimeMode)
	}
	if record.TargetKind != "instance" || record.TargetID != "inst_123" {
		t.Fatalf("unexpected target binding %#v", record)
	}
	if record.DomainPolicy["magic_domain_provider"] != "sslip.io" {
		t.Fatalf("expected domain policy to persist, got %#v", record.DomainPolicy)
	}
}

func TestDeploymentBindingServiceRejectsDuplicateTargetRef(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:        "bind_existing",
		ProjectID: "prj_123",
		Name:      "Existing Binding",
		TargetRef: "prod-main",
	})
	service := NewDeploymentBindingService(projectStore, bindingStore, newFakeInstanceStore(), newFakeMeshNetworkStore(), newFakeClusterStore())

	_, err := service.Create(CreateDeploymentBindingCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Name:            "New Binding",
		TargetRef:       "prod-main",
		RuntimeMode:     "standalone",
		TargetKind:      "instance",
		TargetID:        "inst_123",
	})
	if !errors.Is(err, ErrDuplicateTargetRef) {
		t.Fatalf("expected ErrDuplicateTargetRef, got %v", err)
	}
}

func TestDeploymentBindingServiceRejectsUnknownTarget(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	service := NewDeploymentBindingService(projectStore, newFakeDeploymentBindingStore(), newFakeInstanceStore(), newFakeMeshNetworkStore(), newFakeClusterStore())

	_, err := service.Create(CreateDeploymentBindingCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Name:            "Unknown Target",
		RuntimeMode:     "standalone",
		TargetKind:      "instance",
		TargetID:        "inst_missing",
	})
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("expected ErrTargetNotFound, got %v", err)
	}
}

func TestDeploymentBindingServiceRejectsMismatchedRuntimeMode(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:            "prj_123",
		UserID:        "usr_123",
		Name:          "Acme API",
		Slug:          "acme-api",
		DefaultBranch: "main",
	})
	clusterStore := newFakeClusterStore(&models.Cluster{
		ID:                  "cls_123",
		UserID:              "usr_123",
		Name:                "prod-k3s",
		Provider:            "k3s",
		KubeconfigSecretRef: "secret://clusters/prod",
		Status:              "ready",
	})
	service := NewDeploymentBindingService(projectStore, newFakeDeploymentBindingStore(), newFakeInstanceStore(), newFakeMeshNetworkStore(), clusterStore)

	_, err := service.Create(CreateDeploymentBindingCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Name:            "Bad Mode",
		RuntimeMode:     "standalone",
		TargetKind:      "cluster",
		TargetID:        "cls_123",
	})
	if !errors.Is(err, ErrRuntimeModeMismatch) {
		t.Fatalf("expected ErrRuntimeModeMismatch, got %v", err)
	}
}
