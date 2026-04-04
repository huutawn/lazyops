package initplan

import (
	"encoding/json"
	"strings"
	"testing"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/repo"
)

func TestBuildCreatesInitPlanWithCompatibilityDefaults(t *testing.T) {
	scanResult := repo.RepoScanResult{
		RepoRoot: "/tmp/repo",
		Monorepo: true,
		Services: []repo.DetectedService{
			{Name: "api", Path: "apps/api", Signals: []repo.ServiceSignal{repo.SignalGoMod}},
		},
	}

	detectionResult := repo.DetectionResult{
		RepoRoot: "/tmp/repo",
		Monorepo: true,
		Candidates: []repo.ServiceCandidate{
			{
				Name:      "api",
				Path:      "apps/api",
				Signals:   []repo.ServiceSignal{repo.SignalGoMod},
				StartHint: "go run ./cmd/server",
				Healthcheck: repo.HealthcheckHint{
					Path: "/health",
					Port: 8080,
				},
				Warnings: []string{"review port inference"},
			},
		},
	}

	plan, err := Build(scanResult, detectionResult)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if plan.Layout != "monorepo" {
		t.Fatalf("expected monorepo layout, got %q", plan.Layout)
	}
	if len(plan.Services) != 1 {
		t.Fatalf("expected one service candidate, got %d", len(plan.Services))
	}
	if plan.Services[0].StartHint != "go run ./cmd/server" {
		t.Fatalf("expected start hint to be copied, got %q", plan.Services[0].StartHint)
	}
	if !plan.CompatibilityPolicy.EnvInjection || !plan.CompatibilityPolicy.ManagedCredentials || !plan.CompatibilityPolicy.LocalhostRescue {
		t.Fatalf("expected compatibility defaults to be enabled, got %+v", plan.CompatibilityPolicy)
	}
}

func TestInitPlanValidateRejectsDuplicateServiceNames(t *testing.T) {
	plan := InitPlan{
		RepoRoot: "/tmp/repo",
		Layout:   "monorepo",
		Services: []ServiceCandidate{
			{Name: "api", Path: "apps/api"},
			{Name: "api", Path: "services/api"},
		},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	err := plan.Validate()
	if err == nil {
		t.Fatal("expected duplicate service name error, got nil")
	}
	if !strings.Contains(err.Error(), `duplicate service name "api"`) {
		t.Fatalf("expected duplicate service name error, got %v", err)
	}
}

func TestDependencyBindingDraftValidateRequiresFieldsOnceStarted(t *testing.T) {
	binding := DependencyBindingDraft{
		Service: "api",
		Alias:   "postgres",
	}

	err := binding.Validate()
	if err == nil {
		t.Fatal("expected incomplete dependency binding error, got nil")
	}
	if !strings.Contains(err.Error(), "target_service") {
		t.Fatalf("expected target_service validation error, got %v", err)
	}
}

func TestApplyDiscoverySelectsProjectAndCompatibleTarget(t *testing.T) {
	plan := InitPlan{
		RepoRoot:            "/tmp/repo",
		Layout:              "single-service",
		Services:            []ServiceCandidate{{Name: "api", Path: "."}},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	enriched, err := ApplyDiscovery(
		plan,
		[]contracts.Project{{ID: "prj_demo", Slug: "acme-shop", Name: "Acme Shop", DefaultBranch: "main"}},
		[]contracts.Instance{{ID: "inst_demo", Name: "prod-solo-1", PublicIP: "203.0.113.10", PrivateIP: "10.10.0.10", Status: "online"}},
		[]contracts.MeshNetwork{{ID: "mesh_demo", Name: "prod-ap", Status: "online", Provider: "wireguard"}},
		[]contracts.Cluster{{ID: "cls_demo", Name: "prod-k3s-ap", Status: "registered", Provider: "k3s", KubeconfigSecretRef: "secret://clusters/cls_demo/kubeconfig"}},
		SelectionInput{
			Project:     "acme-shop",
			RuntimeMode: RuntimeModeStandalone,
			Target:      "prod-solo-1",
		},
	)
	if err != nil {
		t.Fatalf("ApplyDiscovery() error = %v", err)
	}

	if enriched.SelectedProject == nil || enriched.SelectedProject.Slug != "acme-shop" {
		t.Fatalf("expected project selection, got %+v", enriched.SelectedProject)
	}
	if enriched.SelectedTarget == nil || enriched.SelectedTarget.Name != "prod-solo-1" {
		t.Fatalf("expected target selection, got %+v", enriched.SelectedTarget)
	}
	if enriched.SelectedTarget.RuntimeMode != RuntimeModeStandalone {
		t.Fatalf("expected standalone target mode, got %q", enriched.SelectedTarget.RuntimeMode)
	}

	payload, err := json.Marshal(enriched)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	rendered := string(payload)
	for _, forbidden := range []string{"203.0.113.10", "10.10.0.10", "secret://clusters/cls_demo/kubeconfig"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("expected sanitized init plan, but found %q in %s", forbidden, rendered)
		}
	}
}

func TestApplyDiscoveryRejectsIncompatibleTargetForMode(t *testing.T) {
	plan := InitPlan{
		RepoRoot:            "/tmp/repo",
		Layout:              "single-service",
		Services:            []ServiceCandidate{{Name: "api", Path: "."}},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	_, err := ApplyDiscovery(
		plan,
		[]contracts.Project{{ID: "prj_demo", Slug: "acme-shop", Name: "Acme Shop"}},
		[]contracts.Instance{{ID: "inst_demo", Name: "prod-solo-1", Status: "online"}},
		[]contracts.MeshNetwork{{ID: "mesh_demo", Name: "prod-ap", Status: "online", Provider: "wireguard"}},
		nil,
		SelectionInput{
			RuntimeMode: RuntimeModeStandalone,
			Target:      "prod-ap",
		},
	)
	if err == nil {
		t.Fatal("expected incompatible target error, got nil")
	}
	if !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("expected incompatible target error, got %v", err)
	}
}

func TestApplyDiscoveryInfersMeshDependencyBindings(t *testing.T) {
	plan := InitPlan{
		RepoRoot: "/tmp/repo",
		Layout:   "monorepo",
		Services: []ServiceCandidate{
			{Name: "web", Path: "apps/web"},
			{
				Name:      "api",
				Path:      "apps/api",
				StartHint: "go run ./cmd/server",
				Healthcheck: HealthcheckHint{
					Path: "/healthz",
					Port: 8080,
				},
			},
		},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	enriched, err := ApplyDiscovery(
		plan,
		[]contracts.Project{{ID: "prj_demo", UserID: "usr_demo", Slug: "acme-shop", Name: "Acme Shop"}},
		nil,
		[]contracts.MeshNetwork{{ID: "mesh_demo", UserID: "usr_demo", Name: "prod-ap", Status: "online", Provider: "wireguard"}},
		nil,
		SelectionInput{
			Project:     "acme-shop",
			RuntimeMode: RuntimeModeDistributedMesh,
		},
	)
	if err != nil {
		t.Fatalf("ApplyDiscovery() error = %v", err)
	}

	if len(enriched.DependencyBindings) != 1 {
		t.Fatalf("expected one inferred mesh dependency binding, got %+v", enriched.DependencyBindings)
	}
	binding := enriched.DependencyBindings[0]
	if binding.Service != "web" || binding.TargetService != "api" || binding.Protocol != "http" || binding.LocalEndpoint != "localhost:8080" {
		t.Fatalf("expected inferred web->api dependency binding, got %+v", binding)
	}
}

func TestApplyDiscoveryRejectsMeshOwnershipMismatch(t *testing.T) {
	plan := InitPlan{
		RepoRoot:            "/tmp/repo",
		Layout:              "single-service",
		Services:            []ServiceCandidate{{Name: "api", Path: "."}},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	_, err := ApplyDiscovery(
		plan,
		[]contracts.Project{{ID: "prj_demo", UserID: "usr_demo", Slug: "acme-shop", Name: "Acme Shop"}},
		nil,
		[]contracts.MeshNetwork{{ID: "mesh_demo", UserID: "usr_other", Name: "prod-ap", Status: "online", Provider: "wireguard"}},
		nil,
		SelectionInput{
			Project:     "acme-shop",
			RuntimeMode: RuntimeModeDistributedMesh,
			Target:      "prod-ap",
		},
	)
	if err == nil {
		t.Fatal("expected ownership mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("expected ownership mismatch error, got %v", err)
	}
}

func TestApplyDiscoveryRejectsOfflineMeshTarget(t *testing.T) {
	plan := InitPlan{
		RepoRoot:            "/tmp/repo",
		Layout:              "single-service",
		Services:            []ServiceCandidate{{Name: "api", Path: "."}},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	_, err := ApplyDiscovery(
		plan,
		[]contracts.Project{{ID: "prj_demo", UserID: "usr_demo", Slug: "acme-shop", Name: "Acme Shop"}},
		nil,
		[]contracts.MeshNetwork{{ID: "mesh_demo", UserID: "usr_demo", Name: "prod-ap", Status: "offline", Provider: "wireguard"}},
		nil,
		SelectionInput{
			Project:     "acme-shop",
			RuntimeMode: RuntimeModeDistributedMesh,
			Target:      "prod-ap",
		},
	)
	if err == nil {
		t.Fatal("expected offline mesh error, got nil")
	}
	if !strings.Contains(err.Error(), "not currently online") {
		t.Fatalf("expected offline mesh error, got %v", err)
	}
}

func TestInitPlanValidateRejectsK3sLocalDependencyBypass(t *testing.T) {
	plan := InitPlan{
		RepoRoot: "/tmp/repo",
		Layout:   "monorepo",
		Services: []ServiceCandidate{
			{Name: "web", Path: "apps/web"},
			{Name: "api", Path: "apps/api"},
		},
		RuntimeMode: RuntimeModeDistributedK3s,
		DependencyBindings: []DependencyBindingDraft{
			{
				Service:       "web",
				Alias:         "api",
				TargetService: "api",
				Protocol:      "http",
				LocalEndpoint: "localhost:8080",
			},
		},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	err := plan.Validate()
	if err == nil {
		t.Fatal("expected distributed-k3s local endpoint rejection, got nil")
	}
	if !strings.Contains(err.Error(), "cluster-native") {
		t.Fatalf("expected distributed-k3s boundary error, got %v", err)
	}
}

func TestApplyDiscoveryRejectsClusterOwnershipMismatch(t *testing.T) {
	plan := InitPlan{
		RepoRoot:            "/tmp/repo",
		Layout:              "single-service",
		Services:            []ServiceCandidate{{Name: "api", Path: "."}},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	_, err := ApplyDiscovery(
		plan,
		[]contracts.Project{{ID: "prj_demo", UserID: "usr_demo", Slug: "acme-shop", Name: "Acme Shop"}},
		nil,
		nil,
		[]contracts.Cluster{{ID: "cls_demo", UserID: "usr_other", Name: "prod-k3s-ap", Status: "registered", Provider: "k3s"}},
		SelectionInput{
			Project:     "acme-shop",
			RuntimeMode: RuntimeModeDistributedK3s,
			Target:      "prod-k3s-ap",
		},
	)
	if err == nil {
		t.Fatal("expected cluster ownership mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("expected cluster ownership mismatch error, got %v", err)
	}
}

func TestApplyDiscoveryRejectsUnavailableCluster(t *testing.T) {
	plan := InitPlan{
		RepoRoot:            "/tmp/repo",
		Layout:              "single-service",
		Services:            []ServiceCandidate{{Name: "api", Path: "."}},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	_, err := ApplyDiscovery(
		plan,
		[]contracts.Project{{ID: "prj_demo", UserID: "usr_demo", Slug: "acme-shop", Name: "Acme Shop"}},
		nil,
		nil,
		[]contracts.Cluster{{ID: "cls_demo", UserID: "usr_demo", Name: "prod-k3s-ap", Status: "unavailable", Provider: "k3s"}},
		SelectionInput{
			Project:     "acme-shop",
			RuntimeMode: RuntimeModeDistributedK3s,
			Target:      "prod-k3s-ap",
		},
	)
	if err == nil {
		t.Fatal("expected unavailable cluster error, got nil")
	}
	if !strings.Contains(err.Error(), "not currently available") {
		t.Fatalf("expected unavailable cluster error, got %v", err)
	}
}

func TestAnnotateBindingsWithTargetsMarksReusableStatus(t *testing.T) {
	bindings := []BindingSummary{
		{
			ID:          "bind_standalone_demo",
			Name:        "prod-solo-binding",
			TargetRef:   "prod-solo-1",
			RuntimeMode: RuntimeModeStandalone,
			TargetKind:  "instance",
			TargetID:    "inst_demo",
		},
		{
			ID:          "bind_mesh_demo",
			Name:        "prod-ap-mesh",
			TargetRef:   "prod-ap",
			RuntimeMode: RuntimeModeDistributedMesh,
			TargetKind:  "mesh",
			TargetID:    "mesh_demo",
		},
	}
	targets := []TargetSummary{
		{ID: "inst_demo", Name: "prod-solo-1", Kind: "instance", Status: "online", OwnerUserID: "usr_demo", RuntimeMode: RuntimeModeStandalone},
		{ID: "mesh_demo", Name: "prod-ap", Kind: "mesh", Status: "offline", OwnerUserID: "usr_demo", RuntimeMode: RuntimeModeDistributedMesh},
	}
	project := &ProjectSummary{ID: "prj_demo", UserID: "usr_demo", Slug: "acme-shop", Name: "Acme Shop"}

	annotated := AnnotateBindingsWithTargets(bindings, targets, project)
	if len(annotated) != 2 {
		t.Fatalf("expected two annotated bindings, got %+v", annotated)
	}
	if annotated[0].TargetStatus != "online" || !annotated[0].Reusable {
		t.Fatalf("expected standalone binding to be reusable, got %+v", annotated[0])
	}
	if annotated[1].TargetStatus != "offline" || annotated[1].Reusable {
		t.Fatalf("expected mesh binding to be non-reusable, got %+v", annotated[1])
	}
}

func TestBindingFiltersSupportTargetRefKindAndStatus(t *testing.T) {
	bindings := []BindingSummary{
		{ID: "bind1", Name: "prod-solo-binding", TargetRef: "prod-solo-1", RuntimeMode: RuntimeModeStandalone, TargetKind: "instance", TargetID: "inst_demo", TargetStatus: "online", Reusable: true},
		{ID: "bind2", Name: "prod-ap-mesh", TargetRef: "prod-ap", RuntimeMode: RuntimeModeDistributedMesh, TargetKind: "mesh", TargetID: "mesh_demo", TargetStatus: "offline"},
		{ID: "bind3", Name: "prod-k3s-binding", TargetRef: "prod-k3s-ap", RuntimeMode: RuntimeModeDistributedK3s, TargetKind: "cluster", TargetID: "cls_demo", TargetStatus: "registered", Reusable: true},
	}

	filtered := FilterBindingsByRuntimeMode(bindings, RuntimeModeDistributedK3s)
	filtered = FilterBindingsByTargetKind(filtered, "cluster")
	filtered = FilterBindingsByTargetRef(filtered, "prod-k3s-ap")
	filtered = FilterBindingsByStatus(filtered, "registered")
	if len(filtered) != 1 || filtered[0].ID != "bind3" {
		t.Fatalf("expected one k3s binding after filters, got %+v", filtered)
	}

	reusable := ReusableBindings(bindings)
	if len(reusable) != 2 {
		t.Fatalf("expected two reusable bindings, got %+v", reusable)
	}
}

func TestApplyBindingsAutoSelectsCompatibleBinding(t *testing.T) {
	plan := InitPlan{
		RepoRoot: "/tmp/repo",
		Layout:   "single-service",
		Services: []ServiceCandidate{{Name: "api", Path: "."}},
		SelectedProject: &ProjectSummary{
			ID:   "prj_demo",
			Name: "Acme Shop",
			Slug: "acme-shop",
		},
		RuntimeMode: RuntimeModeDistributedMesh,
		SelectedTarget: &TargetSummary{
			ID:          "mesh_demo",
			Name:        "prod-ap",
			Kind:        "mesh",
			Status:      "online",
			RuntimeMode: RuntimeModeDistributedMesh,
		},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	enriched, err := ApplyBindings(plan, []contracts.DeploymentBinding{
		{
			ID:          "bind_demo",
			Name:        "prod-ap-mesh",
			TargetRef:   "prod-ap",
			RuntimeMode: string(RuntimeModeDistributedMesh),
			TargetKind:  "mesh",
			TargetID:    "mesh_demo",
		},
	}, BindingSelectionInput{})
	if err != nil {
		t.Fatalf("ApplyBindings() error = %v", err)
	}

	if enriched.SelectedBinding == nil || enriched.SelectedBinding.Name != "prod-ap-mesh" {
		t.Fatalf("expected compatible binding to auto-select, got %+v", enriched.SelectedBinding)
	}
}

func TestApplyBindingsCreatesPendingBindingSelection(t *testing.T) {
	plan := InitPlan{
		RepoRoot: "/tmp/repo",
		Layout:   "single-service",
		Services: []ServiceCandidate{{Name: "api", Path: "."}},
		SelectedProject: &ProjectSummary{
			ID:   "prj_demo",
			Name: "Acme Shop",
			Slug: "acme-shop",
		},
		RuntimeMode: RuntimeModeStandalone,
		SelectedTarget: &TargetSummary{
			ID:          "inst_demo",
			Name:        "prod-solo-1",
			Kind:        "instance",
			Status:      "online",
			RuntimeMode: RuntimeModeStandalone,
		},
		CompatibilityPolicy: DefaultCompatibilityPolicyDraft(),
	}

	enriched, err := ApplyBindings(plan, nil, BindingSelectionInput{
		Create:      true,
		BindingName: "prod-solo-main",
	})
	if err != nil {
		t.Fatalf("ApplyBindings() error = %v", err)
	}

	if enriched.SelectedBinding == nil {
		t.Fatal("expected pending binding selection, got nil")
	}
	if enriched.SelectedBinding.Name != "prod-solo-main" {
		t.Fatalf("expected provided binding name, got %+v", enriched.SelectedBinding)
	}
	if enriched.SelectedBinding.TargetRef != "prod-solo-1" {
		t.Fatalf("expected target ref to come from selected target, got %+v", enriched.SelectedBinding)
	}
}
