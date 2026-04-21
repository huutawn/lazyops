package service

import (
	"testing"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"
)

type fakeProjectDomainStore struct {
	items map[string]*models.ProjectDomain
}

func newFakeProjectDomainStore(items ...*models.ProjectDomain) *fakeProjectDomainStore {
	store := &fakeProjectDomainStore{
		items: make(map[string]*models.ProjectDomain, len(items)),
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		copyItem := *item
		store.items[item.ProjectID+":"+item.Kind] = &copyItem
	}
	return store
}

func (f *fakeProjectDomainStore) Create(item *models.ProjectDomain) error {
	copyItem := *item
	f.items[item.ProjectID+":"+item.Kind] = &copyItem
	return nil
}

func (f *fakeProjectDomainStore) Save(item *models.ProjectDomain) error {
	copyItem := *item
	f.items[item.ProjectID+":"+item.Kind] = &copyItem
	return nil
}

func (f *fakeProjectDomainStore) GetByProjectIDAndKind(projectID, kind string) (*models.ProjectDomain, error) {
	item, ok := f.items[projectID+":"+kind]
	if !ok {
		return nil, nil
	}
	copyItem := *item
	return &copyItem, nil
}

func (f *fakeProjectDomainStore) GetByHostname(hostname string) (*models.ProjectDomain, error) {
	for _, item := range f.items {
		if item.Hostname == hostname {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, nil
}

func TestPublicDomainResolverUsesManagedHTTPSDomain(t *testing.T) {
	projectDomainStore := newFakeProjectDomainStore(&models.ProjectDomain{
		ID:        "dom_123",
		ProjectID: "prj_123",
		Hostname:  "acme-api-ab12.lazyops.cloud",
		Label:     "acme-api-ab12",
		Kind:      ProjectDomainKindManaged,
		Status:    ProjectDomainStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	projectDomainSvc := NewProjectDomainService(
		newFakeProjectStore(),
		projectDomainStore,
		newFakeProjectServiceStore(),
		newFakeDeploymentBindingStore(),
		newFakeInstanceStore(),
		newFakeClusterStore(),
		config.PublicDomainConfig{BaseDomain: "lazyops.cloud", Provider: "cloudflare", CloudflareProxied: true},
	).WithDNSClient(&NoopProjectDomainDNSClient{})
	routingSvc := NewRoutingService(newFakeRoutingPolicyRepo(), newFakeServiceRepo(nil)).WithProjectDomains(projectDomainSvc)
	resolver := NewPublicDomainResolver(
		newFakeInstanceStore(&models.Instance{
			ID:       "inst_123",
			UserID:   "usr_123",
			Name:     "api-host",
			PublicIP: ptrString("203.0.113.10"),
		}),
		newFakeClusterStore(),
		projectDomainSvc,
		routingSvc,
	)

	result := resolver.Resolve(PublicDomainResolveInput{
		ProjectID:   "prj_123",
		ProjectSlug: "acme-api",
		RuntimeMode: bootstrapModeStandalone,
		TargetKind:  "instance",
		TargetID:    "inst_123",
		Services: []BlueprintServiceContractRecord{{
			Name:   "api",
			Public: true,
		}},
	})

	if len(result.Domains) != 1 {
		t.Fatalf("expected one public domain, got %#v", result.Domains)
	}
	if got := result.Domains[0].PrimaryURL; got != "https://acme-api-ab12.lazyops.cloud" {
		t.Fatalf("expected managed primary url to use project domain, got %q", got)
	}
	if got := result.Domains[0].FallbackURL; got != "https://acme-api-ab12.lazyops.cloud" {
		t.Fatalf("expected managed fallback url to mirror project domain, got %q", got)
	}
}

func TestPublicDomainResolverUsesSharedDomainPathsForDistributedK3s(t *testing.T) {
	projectDomainStore := newFakeProjectDomainStore(&models.ProjectDomain{
		ID:        "dom_123",
		ProjectID: "prj_123",
		Hostname:  "bbb-ab12.lazyops.cloud",
		Label:     "bbb-ab12",
		Kind:      ProjectDomainKindManaged,
		Status:    ProjectDomainStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	projectDomainSvc := NewProjectDomainService(
		newFakeProjectStore(),
		projectDomainStore,
		newFakeProjectServiceStore(),
		newFakeDeploymentBindingStore(),
		newFakeInstanceStore(),
		newFakeClusterStore(),
		config.PublicDomainConfig{BaseDomain: "lazyops.cloud", Provider: "cloudflare", CloudflareProxied: true},
	).WithDNSClient(&NoopProjectDomainDNSClient{})
	routingSvc := NewRoutingService(newFakeRoutingPolicyRepo(), newFakeServiceRepo(nil)).WithProjectDomains(projectDomainSvc)
	resolver := NewPublicDomainResolver(
		newFakeInstanceStore(),
		newFakeClusterStore(&models.Cluster{
			ID:       "cls_123",
			UserID:   "usr_123",
			Name:     "edge-k3s",
			PublicIP: ptrString("203.0.113.10"),
		}),
		projectDomainSvc,
		routingSvc,
	)

	result := resolver.Resolve(PublicDomainResolveInput{
		ProjectID:   "prj_123",
		ProjectSlug: "bbb",
		RuntimeMode: bootstrapModeDistributedK3s,
		TargetKind:  "cluster",
		TargetID:    "cls_123",
		Services: []BlueprintServiceContractRecord{
			{Name: "fe", Public: true, Kind: "web", RuntimeProfile: "web"},
			{Name: "be", Public: true, Kind: "api", RuntimeProfile: "web"},
		},
		PlacementAssignments: []PlacementAssignmentRecord{
			{ServiceName: "fe", TargetID: "cls_123", TargetKind: "cluster"},
			{ServiceName: "be", TargetID: "cls_123", TargetKind: "cluster"},
		},
	})

	if len(result.Domains) != 2 {
		t.Fatalf("expected two public domain records, got %#v", result.Domains)
	}
	urls := uniqueNonEmptyStrings(result.PublicURLs)
	if len(urls) != 2 {
		t.Fatalf("expected two shared-domain public urls, got %#v", urls)
	}
	expected := map[string]struct{}{
		"https://bbb-ab12.lazyops.cloud":     {},
		"https://bbb-ab12.lazyops.cloud/api": {},
	}
	for _, item := range urls {
		if _, ok := expected[item]; !ok {
			t.Fatalf("unexpected public url %q from %#v", item, urls)
		}
	}
}
