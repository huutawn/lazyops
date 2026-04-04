package status

import (
	"testing"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/initplan"
	"lazyops-cli/internal/lazyyaml"
)

func TestBuildAdapterSummaryHealthyStandalone(t *testing.T) {
	summary, err := BuildAdapterSummary(Input{
		Contract: lazyyaml.DoctorMetadata{
			ProjectSlug: "acme-shop",
			RuntimeMode: initplan.RuntimeModeStandalone,
			TargetRef:   "prod-solo-1",
			Services: []lazyyaml.DoctorService{
				{Name: "api", Path: "apps/api"},
			},
		},
		Project: contracts.Project{
			ID: "prj_demo",

			Name: "Acme Shop",
			Slug: "acme-shop",
		},
		Binding: &contracts.DeploymentBinding{
			ID:          "bind_standalone_demo",
			ProjectID:   "prj_demo",
			Name:        "prod-solo-binding",
			TargetRef:   "prod-solo-1",
			RuntimeMode: "standalone",
			TargetKind:  "instance",
			TargetID:    "inst_demo",
		},
		Target: &TargetSnapshot{
			ID:     "inst_demo",
			Kind:   "instance",
			Name:   "prod-solo-1",
			Status: "online",
		},
	})
	if err != nil {
		t.Fatalf("BuildAdapterSummary() error = %v", err)
	}

	if summary.Source != AdapterName {
		t.Fatalf("expected adapter source %q, got %q", AdapterName, summary.Source)
	}
	if summary.Binding.State != "attached" {
		t.Fatalf("expected attached binding, got %+v", summary.Binding)
	}
	if summary.Topology.State != "healthy" {
		t.Fatalf("expected healthy topology, got %+v", summary.Topology)
	}
	if summary.Deployment.State != "ready" || summary.Deployment.Rollout != "idle" {
		t.Fatalf("expected ready deployment/idle rollout, got %+v", summary.Deployment)
	}
}

func TestBuildAdapterSummaryMissingBindingBlocksDeployment(t *testing.T) {
	summary, err := BuildAdapterSummary(Input{
		Contract: lazyyaml.DoctorMetadata{
			ProjectSlug: "acme-shop",
			RuntimeMode: initplan.RuntimeModeStandalone,
			TargetRef:   "prod-solo-1",
			Services: []lazyyaml.DoctorService{
				{Name: "api", Path: "apps/api"},
			},
		},
		Project: contracts.Project{
			ID:   "prj_demo",
			Name: "Acme Shop",
			Slug: "acme-shop",
		},
	})
	if err != nil {
		t.Fatalf("BuildAdapterSummary() error = %v", err)
	}

	if summary.Binding.State != "missing" {
		t.Fatalf("expected missing binding, got %+v", summary.Binding)
	}
	if summary.Topology.State != "pending" {
		t.Fatalf("expected pending topology, got %+v", summary.Topology)
	}
	if summary.Deployment.State != "blocked" || summary.Deployment.Rollout != "blocked" {
		t.Fatalf("expected blocked deployment, got %+v", summary.Deployment)
	}
}

func TestBuildAdapterSummaryOfflineTargetDegradesDeployment(t *testing.T) {
	summary, err := BuildAdapterSummary(Input{
		Contract: lazyyaml.DoctorMetadata{
			ProjectSlug: "acme-shop",
			RuntimeMode: initplan.RuntimeModeDistributedMesh,
			TargetRef:   "prod-ap",
			Services: []lazyyaml.DoctorService{
				{Name: "api", Path: "apps/api"},
				{Name: "web", Path: "apps/web"},
			},
		},
		Project: contracts.Project{
			ID: "prj_demo",

			Name: "Acme Shop",
			Slug: "acme-shop",
		},
		Binding: &contracts.DeploymentBinding{
			ID:          "bind_demo",
			ProjectID:   "prj_demo",
			Name:        "prod-ap-mesh",
			TargetRef:   "prod-ap",
			RuntimeMode: "distributed-mesh",
			TargetKind:  "mesh",
			TargetID:    "mesh_demo",
		},
		Target: &TargetSnapshot{
			ID:     "mesh_demo",
			Kind:   "mesh",
			Name:   "prod-ap",
			Status: "offline",
		},
	})
	if err != nil {
		t.Fatalf("BuildAdapterSummary() error = %v", err)
	}

	if summary.Topology.State != "degraded" {
		t.Fatalf("expected degraded topology, got %+v", summary.Topology)
	}
	if summary.Deployment.State != "degraded" || summary.Deployment.Rollout != "paused" {
		t.Fatalf("expected degraded deployment/paused rollout, got %+v", summary.Deployment)
	}
}
