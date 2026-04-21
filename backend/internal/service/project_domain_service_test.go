package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
)

type fakeProjectDomainDNSClient struct {
	lastRequest ProjectDomainDNSUpsertRequest
	recordID    string
	err         error
	calls       int
}

func (f *fakeProjectDomainDNSClient) UpsertARecord(_ context.Context, req ProjectDomainDNSUpsertRequest) (ProjectDomainDNSSyncResult, error) {
	f.calls++
	f.lastRequest = req
	if f.err != nil {
		return ProjectDomainDNSSyncResult{}, f.err
	}
	recordID := f.recordID
	if recordID == "" {
		recordID = "cf_dns_123"
	}
	return ProjectDomainDNSSyncResult{RecordID: recordID}, nil
}

func TestProjectDomainServiceAllocateCreatesManagedHostnameAndSyncsDNS(t *testing.T) {
	projectStore := newFakeProjectStore(&models.Project{
		ID:     "prj_123",
		UserID: "usr_123",
		Name:   "Acme API",
		Slug:   "acme-api",
	})
	serviceStore := newFakeProjectServiceStore()
	if err := serviceStore.ReplaceForProject("prj_123", []models.Service{{
		ID:        "svc_123",
		ProjectID: "prj_123",
		Name:      "api",
		Public:    true,
		Kind:      "api",
		Path:      "backend",
	}}); err != nil {
		t.Fatalf("seed services: %v", err)
	}
	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		TargetRef:  "auto-primary",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:       "inst_123",
		UserID:   "usr_123",
		Name:     "edge-1",
		PublicIP: ptrString("203.0.113.10"),
	})
	domainStore := newFakeProjectDomainStore()
	dnsClient := &fakeProjectDomainDNSClient{}

	service := NewProjectDomainService(
		projectStore,
		domainStore,
		serviceStore,
		bindingStore,
		instanceStore,
		newFakeClusterStore(),
		config.PublicDomainConfig{BaseDomain: "lazyops.cloud", Provider: "cloudflare", CloudflareProxied: true},
	).WithDNSClient(dnsClient)

	record, err := service.Allocate(AllocateProjectDomainCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
	})
	if err != nil {
		t.Fatalf("allocate project domain: %v", err)
	}
	if !strings.HasSuffix(record.Hostname, ".lazyops.cloud") {
		t.Fatalf("expected managed hostname under lazyops.cloud, got %q", record.Hostname)
	}
	if record.Status != ProjectDomainStatusActive {
		t.Fatalf("expected active synced domain, got %q", record.Status)
	}
	if dnsClient.calls != 1 {
		t.Fatalf("expected one cloudflare sync call, got %d", dnsClient.calls)
	}
	if dnsClient.lastRequest.IPv4 != "203.0.113.10" {
		t.Fatalf("expected sync to target public ip, got %#v", dnsClient.lastRequest)
	}
}

func TestProjectDomainServiceRenameRejectsTakenLabel(t *testing.T) {
	projectStore := newFakeProjectStore(
		&models.Project{ID: "prj_123", UserID: "usr_123", Name: "Acme API", Slug: "acme-api"},
		&models.Project{ID: "prj_456", UserID: "usr_123", Name: "Acme Jobs", Slug: "acme-jobs"},
	)
	domainStore := newFakeProjectDomainStore(
		&models.ProjectDomain{
			ID:        "dom_123",
			ProjectID: "prj_123",
			Hostname:  "acme-api-ab12.lazyops.cloud",
			Label:     "acme-api-ab12",
			Kind:      ProjectDomainKindManaged,
			Status:    ProjectDomainStatusActive,
		},
		&models.ProjectDomain{
			ID:        "dom_456",
			ProjectID: "prj_456",
			Hostname:  "taken-label.lazyops.cloud",
			Label:     "taken-label",
			Kind:      ProjectDomainKindManaged,
			Status:    ProjectDomainStatusActive,
		},
	)

	service := NewProjectDomainService(
		projectStore,
		domainStore,
		newFakeProjectServiceStore(),
		newFakeDeploymentBindingStore(),
		newFakeInstanceStore(),
		newFakeClusterStore(),
		config.PublicDomainConfig{BaseDomain: "lazyops.cloud", Provider: "cloudflare", CloudflareProxied: true},
	).WithDNSClient(&NoopProjectDomainDNSClient{})

	_, err := service.Rename(RenameProjectDomainCommand{
		RequesterUserID: "usr_123",
		RequesterRole:   RoleOperator,
		ProjectID:       "prj_123",
		Label:           "taken-label",
	})
	if err == nil || !errors.Is(err, ErrProjectDomainLabelTaken) {
		t.Fatalf("expected ErrProjectDomainLabelTaken, got %v", err)
	}
}
