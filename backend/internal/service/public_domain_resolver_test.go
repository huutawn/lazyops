package service

import (
	"testing"

	"lazyops-server/internal/models"
)

func TestPublicDomainResolverUsesHTTPSForStandalone(t *testing.T) {
	resolver := NewPublicDomainResolver(
		newFakeInstanceStore(&models.Instance{
			ID:       "inst_123",
			UserID:   "usr_123",
			Name:     "api-host",
			PublicIP: ptrString("203.0.113.10"),
		}),
		newFakeClusterStore(),
	)

	result := resolver.Resolve(PublicDomainResolveInput{
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
	if got := result.Domains[0].PrimaryURL; got != "https://api.acme-api.203-0-113-10.sslip.io" {
		t.Fatalf("expected standalone primary url to use https, got %q", got)
	}
	if got := result.Domains[0].FallbackURL; got != "https://api.acme-api.203.0.113.10.nip.io" {
		t.Fatalf("expected standalone fallback url to use https, got %q", got)
	}
}

func TestPublicDomainResolverUsesHTTPForDistributedK3s(t *testing.T) {
	resolver := NewPublicDomainResolver(
		newFakeInstanceStore(),
		newFakeClusterStore(&models.Cluster{
			ID:       "cls_123",
			UserID:   "usr_123",
			Name:     "edge-k3s",
			PublicIP: ptrString("203.0.113.10"),
		}),
	)

	result := resolver.Resolve(PublicDomainResolveInput{
		ProjectSlug: "bbb",
		RuntimeMode: bootstrapModeDistributedK3s,
		TargetKind:  "cluster",
		TargetID:    "cls_123",
		Services: []BlueprintServiceContractRecord{{
			Name:   "fe",
			Public: true,
		}},
		PlacementAssignments: []PlacementAssignmentRecord{{
			ServiceName: "fe",
			TargetID:    "cls_123",
			TargetKind:  "cluster",
		}},
	})

	if len(result.Domains) != 1 {
		t.Fatalf("expected one public domain, got %#v", result.Domains)
	}
	if got := result.Domains[0].PrimaryURL; got != "http://fe.bbb.203-0-113-10.sslip.io" {
		t.Fatalf("expected distributed-k3s primary url to use http, got %q", got)
	}
	if got := result.Domains[0].FallbackURL; got != "http://fe.bbb.203.0.113.10.nip.io" {
		t.Fatalf("expected distributed-k3s fallback url to use http, got %q", got)
	}
	if len(result.PublicURLs) != 2 {
		t.Fatalf("expected both distributed-k3s public urls, got %#v", result.PublicURLs)
	}
}
