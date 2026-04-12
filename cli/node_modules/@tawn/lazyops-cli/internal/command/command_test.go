package command

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/transport"
	"lazyops-cli/internal/ui"
)

func TestRootHelpListsLockedCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    mustTestStore(t),
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{"login", "init", "bindings", "tunnel"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected help output to contain %q, got %q", expected, output)
		}
	}
}

func TestNestedTunnelDBCommandUsesMockTransport(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"tunnel", "db"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "tunnel session created") {
		t.Fatalf("expected tunnel session created message, got %q", output)
	}
	if !strings.Contains(output, "type: db") {
		t.Fatalf("expected tunnel type db, got %q", output)
	}
	if !strings.Contains(output, "local port:") {
		t.Fatalf("expected local port in output, got %q", output)
	}
	if !strings.Contains(output, "session id:") {
		t.Fatalf("expected session id in output, got %q", output)
	}
	if !strings.Contains(output, "connect: db://127.0.0.1:") {
		t.Fatalf("expected connect info in output, got %q", output)
	}
	if !strings.Contains(stderr.String(), "debug tunnel") {
		t.Fatalf("expected debug tunnel warning in stderr, got %q", stderr.String())
	}
}

func TestLoginEmailPasswordStoresCredentialAndRedactsOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"login", "--email", "demo@lazyops.local", "--password", "demo-password"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if strings.Contains(output, "lazyops_pat_mock_secret_value") {
		t.Fatalf("expected login output to redact token, got %q", output)
	}
	if !strings.Contains(output, "logged in as CLI Demo User via email/password") {
		t.Fatalf("expected login success message, got %q", output)
	}

	record, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if record.Token != "lazyops_pat_mock_secret_value" {
		t.Fatalf("expected stored token, got %q", record.Token)
	}
}

func TestLoginBrowserProviderStoresCredential(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"login", "--provider", "github"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "logged in as CLI Demo User via github browser OAuth") {
		t.Fatalf("expected browser login success message, got %q", output)
	}
}

func TestLoginReturnsActionableErrorForInvalidCredentials(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    mustTestStore(t),
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"login", "--email", "wrong@example.com", "--password", "bad-password"})
	if err == nil {
		t.Fatal("expected invalid credentials error, got nil")
	}

	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error, got %v", err)
	}

	if !strings.Contains(err.Error(), "Email or password is incorrect.") {
		t.Fatalf("expected invalid credentials message, got %v", err)
	}
}

func TestLoginSpinnerStartsAfterOneSecond(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	spinnerFactory := &fakeSpinnerFactory{}
	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: spinnerFactory,
		Transport:      transport.NewMockTransportWithLatency(transport.DefaultFixtures(), 1100*time.Millisecond),
		Credentials:    mustTestStore(t),
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"login", "--provider", "google"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if spinnerFactory.spinner.startCalls == 0 {
		t.Fatal("expected delayed spinner to start for login request over one second")
	}
	if spinnerFactory.spinner.stopCalls == 0 {
		t.Fatal("expected delayed spinner to stop after login completes")
	}
}

func TestInitPrintsRepoScanPreviewUsingServiceAbstraction(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(cwd); chdirErr != nil {
			t.Fatalf("Chdir() restore error = %v", chdirErr)
		}
	}()

	repoRoot := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "keep"), "")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "go.mod"), "module api\n\ngo 1.22.2\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "web", "package.json"), `{"name":"web"}`)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "init plan review ready") {
		t.Fatalf("expected init review header, got %q", output)
	}
	if !strings.Contains(output, "repo layout: monorepo") {
		t.Fatalf("expected monorepo label, got %q", output)
	}
	if !strings.Contains(output, "compatibility policy: env_injection=true managed_credentials=true localhost_rescue=true") {
		t.Fatalf("expected compatibility policy review, got %q", output)
	}
	if !strings.Contains(output, "selected project: Acme Shop (acme-shop)") {
		t.Fatalf("expected selected project in review, got %q", output)
	}
	if !strings.Contains(output, "runtime mode option: standalone") {
		t.Fatalf("expected runtime mode options in review, got %q", output)
	}
	if !strings.Contains(output, "target option for standalone: instance prod-solo-1 [online]") {
		t.Fatalf("expected standalone target summary, got %q", output)
	}
	if !strings.Contains(output, "target option for distributed-mesh: mesh prod-ap [online]") {
		t.Fatalf("expected mesh target summary, got %q", output)
	}
	if !strings.Contains(output, "target option for distributed-k3s: cluster prod-k3s-ap [registered]") {
		t.Fatalf("expected k3s target summary, got %q", output)
	}
	for _, forbidden := range []string{"203.0.113.10", "10.10.0.10", "secret://clusters/cls_demo/kubeconfig"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("expected sanitized review output, but found %q in %q", forbidden, output)
		}
	}
	if !strings.Contains(output, "service api -> apps/api (go.mod)") {
		t.Fatalf("expected api service preview, got %q", output)
	}
	if !strings.Contains(output, "service web -> apps/web (package.json)") {
		t.Fatalf("expected web service preview, got %q", output)
	}
	if !strings.Contains(output, "dependency bindings: none inferred yet") {
		t.Fatalf("expected dependency bindings review, got %q", output)
	}
	lowerOutput := strings.ToLower(output)
	if strings.Contains(lowerOutput, "frontend") || strings.Contains(lowerOutput, "backend") {
		t.Fatalf("expected init scan output to stay service-only, got %q", output)
	}
}

func TestInitPrintsServiceHintsAndAmbiguityWarnings(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(cwd); chdirErr != nil {
			t.Fatalf("Chdir() restore error = %v", chdirErr)
		}
	}()

	repoRoot := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "keep"), "")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "go.mod"), "module api\n\ngo 1.22.2\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "cmd", "server", "main.go"), "package main\nfunc main(){}\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "internal", "api", "routes.go"), `package api; func routes(){ GET("/healthz", nil) }`)
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "internal", "config", "config.go"), "package config\nconst _ = `SERVER_PORT\", \"8080\"`\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "workers", "jobs", "requirements.txt"), "fastapi==0.110.0\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "workers", "jobs", "app.py"), "print('hi')\n")

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "start hint for api: go run ./cmd/server") {
		t.Fatalf("expected Go start hint in init output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "health hint for api: /healthz on 8080") {
		t.Fatalf("expected Go health hint in init output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "start hint for jobs: python app.py") {
		t.Fatalf("expected Python start hint in init output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "selected project: Acme Shop (acme-shop)") {
		t.Fatalf("expected selected project in init output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "dependency bindings: none inferred yet") {
		t.Fatalf("expected dependency binding review state, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "service jobs: no health hint inferred yet") {
		t.Fatalf("expected ambiguity warning in stderr, got %q", stderr.String())
	}
}

func TestInitFiltersTargetsWhenRuntimeModeIsSelected(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "distributed-mesh"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "selected runtime mode: distributed-mesh") {
		t.Fatalf("expected selected runtime mode, got %q", output)
	}
	if !strings.Contains(output, "target option for distributed-mesh: mesh prod-ap [online]") {
		t.Fatalf("expected compatible mesh target, got %q", output)
	}
	if !strings.Contains(output, "selected target: mesh prod-ap [online]") {
		t.Fatalf("expected auto-selected mesh target, got %q", output)
	}
	if strings.Contains(output, "target option for standalone") || strings.Contains(output, "target option for distributed-k3s") {
		t.Fatalf("expected target review to be filtered by runtime mode, got %q", output)
	}
}

func TestInitRejectsIncompatibleTargetSelection(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--runtime-mode", "standalone", "--target", "prod-ap"})
	if err == nil {
		t.Fatal("expected incompatible target selection error, got nil")
	}
	if !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("expected incompatible target error, got %v", err)
	}
}

func TestInitAutoSelectsCompatibleBinding(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "distributed-mesh"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "binding option: prod-ap-mesh -> prod-ap (distributed-mesh, mesh)") {
		t.Fatalf("expected binding option in init review, got %q", output)
	}
	if !strings.Contains(output, "selected binding: prod-ap-mesh -> prod-ap (distributed-mesh, mesh)") {
		t.Fatalf("expected compatible binding to auto-select, got %q", output)
	}
}

func TestInitCreatesBindingWhenRequested(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--target", "prod-solo-1", "--create-binding", "--binding-name", "prod-solo-main"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "selected binding: prod-solo-main -> prod-solo-1 (standalone, instance)") {
		t.Fatalf("expected created binding to be selected, got %q", output)
	}
}

func TestInitStandaloneHappyPathWritesDeployContract(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--write"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"selected target: instance prod-solo-1 [online]",
		"selected binding: prod-solo-binding -> prod-solo-1 (standalone, instance)",
		"lazyops.yaml written",
		"init complete for standalone",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected standalone happy path output to contain %q, got %q", expected, output)
		}
	}

	rendered, err := os.ReadFile(filepath.Join(repoRoot, "lazyops.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range []string{"project_slug: acme-shop", "runtime_mode: standalone", "target_ref: prod-solo-1"} {
		if !strings.Contains(string(rendered), expected) {
			t.Fatalf("expected written standalone contract to contain %q, got %q", expected, string(rendered))
		}
	}
}

func TestInitDistributedMeshHappyPathWritesDeployContractWithDependencyReview(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareMeshInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "distributed-mesh", "--write"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"selected target: mesh prod-ap [online]",
		"selected binding: prod-ap-mesh -> prod-ap (distributed-mesh, mesh)",
		"dependency binding web.api -> api (http)",
		"lazyops.yaml written",
		"init complete for distributed-mesh",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected distributed-mesh happy path output to contain %q, got %q", expected, output)
		}
	}

	rendered, err := os.ReadFile(filepath.Join(repoRoot, "lazyops.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range []string{
		"runtime_mode: distributed-mesh",
		"target_ref: prod-ap",
		"dependency_bindings:",
		"service: web",
		"target_service: api",
		"local_endpoint: 'localhost:8080'",
	} {
		if !strings.Contains(string(rendered), expected) {
			t.Fatalf("expected mesh deploy contract to contain %q, got %q", expected, string(rendered))
		}
	}
}

func TestInitDistributedK3sHappyPathWritesDeployContract(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "distributed-k3s", "--write"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"selected runtime mode: distributed-k3s",
		"distributed-k3s boundary: K3s remains the workload scheduler; CLI writes logical binding refs only",
		"selected target: cluster prod-k3s-ap [registered]",
		"selected binding: prod-k3s-binding -> prod-k3s-ap (distributed-k3s, cluster)",
		"lazyops.yaml written",
		"init complete for distributed-k3s",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected distributed-k3s happy path output to contain %q, got %q", expected, output)
		}
	}

	rendered, err := os.ReadFile(filepath.Join(repoRoot, "lazyops.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, expected := range []string{
		"runtime_mode: distributed-k3s",
		"target_ref: prod-k3s-ap",
	} {
		if !strings.Contains(string(rendered), expected) {
			t.Fatalf("expected distributed-k3s contract to contain %q, got %q", expected, string(rendered))
		}
	}
	for _, forbidden := range []string{"secret://clusters/cls_demo/kubeconfig", "cluster_id", "target_id", "cls_demo"} {
		if strings.Contains(string(rendered), forbidden) {
			t.Fatalf("expected distributed-k3s contract to stay logical, but found %q in %q", forbidden, string(rendered))
		}
	}
}

func TestInitPrintsLazyopsYAMLPreviewWhenPlanIsValid(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--target", "prod-solo-1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "lazyops.yaml local validation passed") {
		t.Fatalf("expected lazyops.yaml validation output, got %q", output)
	}
	if !strings.Contains(output, "pre-write review:") {
		t.Fatalf("expected pre-write review output, got %q", output)
	}
	for _, expected := range []string{"project_slug: acme-shop", "runtime_mode: standalone", "deployment_binding:", "target_ref: prod-solo-1"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected lazyops.yaml preview to contain %q, got %q", expected, output)
		}
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "lazyops.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected preview-only init to avoid writing lazyops.yaml, stat err = %v", err)
	}
	if !strings.Contains(stderr.String(), "write pending") {
		t.Fatalf("expected write pending warning, got %q", stderr.String())
	}
}

func TestInitWriteCreatesLazyopsYAMLAtRepoRoot(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--target", "prod-solo-1", "--write"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered, err := os.ReadFile(filepath.Join(repoRoot, "lazyops.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(rendered), "project_slug: acme-shop") || !strings.Contains(string(rendered), "runtime_mode: standalone") {
		t.Fatalf("expected written lazyops.yaml contents, got %q", string(rendered))
	}
	if !strings.Contains(stdout.String(), "lazyops.yaml written") {
		t.Fatalf("expected write success output, got %q", stdout.String())
	}
}

func TestInitWriteRequiresOverwriteConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: old\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--target", "prod-solo-1", "--write"})
	if err == nil {
		t.Fatal("expected overwrite confirmation error, got nil")
	}
	if !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("expected overwrite guidance, got %v", err)
	}

	rendered, readErr := os.ReadFile(filepath.Join(repoRoot, "lazyops.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(rendered) != "project_slug: old\n" {
		t.Fatalf("expected existing lazyops.yaml to remain unchanged, got %q", string(rendered))
	}
}

func TestInitOverwriteCreatesBackupAndWritesNewLazyopsYAML(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: old\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--target", "prod-solo-1", "--magic-domain-provider", "nip.io", "--scale-to-zero", "--write", "--overwrite"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	rendered, err := os.ReadFile(filepath.Join(repoRoot, "lazyops.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(rendered), "provider: nip.io") || !strings.Contains(string(rendered), "scale_to_zero_policy:\n  enabled: true") {
		t.Fatalf("expected overwritten lazyops.yaml to include requested overrides, got %q", string(rendered))
	}

	backups, err := filepath.Glob(filepath.Join(repoRoot, "lazyops.yaml.bak.*"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected one backup file, got %d (%v)", len(backups), backups)
	}

	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatalf("ReadFile(backup) error = %v", err)
	}
	if string(backup) != "project_slug: old\n" {
		t.Fatalf("expected backup to preserve previous contents, got %q", string(backup))
	}
	if !strings.Contains(stdout.String(), "backup created:") {
		t.Fatalf("expected backup output, got %q", stdout.String())
	}
}

func TestInitRejectsOverwriteWithoutWriteFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--overwrite"})
	if err == nil {
		t.Fatal("expected overwrite flag validation error, got nil")
	}
	if !strings.Contains(err.Error(), "`--overwrite` requires `--write`") {
		t.Fatalf("expected overwrite/write validation error, got %v", err)
	}
}

func TestInitStandaloneFailsWhenNoValidTargetExists(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport: &stubTransport{
			mode: "stub",
			do: func(_ context.Context, req transport.Request) (transport.Response, error) {
				switch req.Path {
				case "/api/v1/projects":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "projects",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop"}]}}`),
					}, nil
				case "/api/v1/instances":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "instances-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				case "/api/v1/mesh-networks":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "mesh-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				case "/api/v1/clusters":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "clusters-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				default:
					return transport.Response{StatusCode: 404}, nil
				}
			},
		},
		Credentials: store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone"})
	if err == nil {
		t.Fatal("expected no valid target error, got nil")
	}
	if !strings.Contains(err.Error(), `no valid target exists for runtime mode "standalone"`) {
		t.Fatalf("expected no valid target error, got %v", err)
	}
}

func TestInitReturnsActionableErrorForInvalidPAT(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_invalid_demo",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone"})
	if err == nil {
		t.Fatal("expected invalid PAT error, got nil")
	}
	if !strings.Contains(err.Error(), "CLI PAT is invalid or revoked") {
		t.Fatalf("expected invalid PAT message, got %v", err)
	}
	if !strings.Contains(err.Error(), "lazyops login") {
		t.Fatalf("expected invalid PAT next step, got %v", err)
	}
}

func TestInitDistributedMeshFailsWhenSelectedMeshIsOffline(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareMeshInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport: &stubTransport{
			mode: "stub",
			do: func(_ context.Context, req transport.Request) (transport.Response, error) {
				switch req.Path {
				case "/api/v1/projects":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "projects",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop"}]}}`),
					}, nil
				case "/api/v1/instances":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "instances-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				case "/api/v1/mesh-networks":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "mesh-list",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"mesh_demo","name":"prod-ap","provider":"wireguard","status":"offline"}]}}`),
					}, nil
				case "/api/v1/clusters":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "clusters-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				default:
					return transport.Response{StatusCode: 404}, nil
				}
			},
		},
		Credentials: store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "distributed-mesh", "--target", "prod-ap"})
	if err == nil {
		t.Fatal("expected offline mesh error, got nil")
	}
	if !strings.Contains(err.Error(), "not currently online") {
		t.Fatalf("expected offline mesh message, got %v", err)
	}
}

func TestInitDistributedK3sFailsWhenSelectedClusterIsUnavailable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport: &stubTransport{
			mode: "stub",
			do: func(_ context.Context, req transport.Request) (transport.Response, error) {
				switch req.Path {
				case "/api/v1/projects":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "projects",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop"}]}}`),
					}, nil
				case "/api/v1/instances":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "instances-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				case "/api/v1/mesh-networks":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "mesh-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				case "/api/v1/clusters":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "clusters-list",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"cls_demo","name":"prod-k3s-ap","provider":"k3s","status":"unavailable"}]}}`),
					}, nil
				default:
					return transport.Response{StatusCode: 404}, nil
				}
			},
		},
		Credentials: store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "distributed-k3s", "--target", "prod-k3s-ap"})
	if err == nil {
		t.Fatal("expected unavailable cluster error, got nil")
	}
	if !strings.Contains(err.Error(), "not currently available") {
		t.Fatalf("expected unavailable cluster message, got %v", err)
	}
}

func TestInitRejectsIncompatibleBindingSelection(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--binding", "prod-ap-mesh"})
	if err == nil {
		t.Fatal("expected incompatible binding selection error, got nil")
	}
	if !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("expected incompatible binding error, got %v", err)
	}
}

func TestBindingsCommandRendersTypedBindingList(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"bindings"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "deployment bindings loaded") {
		t.Fatalf("expected bindings header, got %q", output)
	}
	if !strings.Contains(output, "filters: none") {
		t.Fatalf("expected bindings filter summary, got %q", output)
	}
	if !strings.Contains(output, "binding prod-ap-mesh -> prod-ap (distributed-mesh, mesh, status=online)") {
		t.Fatalf("expected typed mesh binding output, got %q", output)
	}
	if !strings.Contains(output, "binding prod-solo-binding -> prod-solo-1 (standalone, instance, status=online)") {
		t.Fatalf("expected typed standalone binding output, got %q", output)
	}
	if !strings.Contains(output, "binding prod-k3s-binding -> prod-k3s-ap (distributed-k3s, cluster, status=registered)") {
		t.Fatalf("expected typed k3s binding output, got %q", output)
	}
	if !strings.Contains(output, "reuse with: lazyops init --project acme-shop --runtime-mode standalone --binding bind_standalone_demo") {
		t.Fatalf("expected reuse hint for standalone binding, got %q", output)
	}
}

func TestBindingsCommandFiltersByRuntimeTargetKindStatusAndReuse(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"bindings", "--runtime-mode", "distributed-k3s", "--target-kind", "cluster", "--status", "registered", "--reuse"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "filters: runtime_mode=distributed-k3s, target_kind=cluster, status=registered, reuse=true") {
		t.Fatalf("expected filter summary, got %q", output)
	}
	if !strings.Contains(output, "binding prod-k3s-binding -> prod-k3s-ap (distributed-k3s, cluster, status=registered)") {
		t.Fatalf("expected filtered k3s binding output, got %q", output)
	}
	if !strings.Contains(output, "reuse candidate selected: prod-k3s-binding") {
		t.Fatalf("expected reuse candidate selection, got %q", output)
	}
	if strings.Contains(output, "prod-ap-mesh") || strings.Contains(output, "prod-solo-binding") {
		t.Fatalf("expected filtered output to omit non-matching bindings, got %q", output)
	}
}

func TestBindingsCommandWarnsWhenNoBindingsMatchFilters(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"bindings", "--target-ref", "missing-ref"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stderr.String(), "no deployment bindings match the current filters") {
		t.Fatalf("expected no-match warning, got %q", stderr.String())
	}
}

func TestInitReusesExistingBindingWithoutCreatingNewOne(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	capture := &stubTransport{
		mode: "capture-init-reuse",
		do: func(_ context.Context, req transport.Request) (transport.Response, error) {
			switch req.Path {
			case "/api/v1/projects":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "projects",
					Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop","default_branch":"main"}]}}`),
				}, nil
			case "/api/v1/instances":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "instances",
					Body:        []byte(`{"success":true,"data":{"items":[{"id":"inst_demo","name":"prod-solo-1","status":"online"}]}}`),
				}, nil
			case "/api/v1/mesh-networks":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "mesh-empty",
					Body:        []byte(`{"success":true,"data":{"items":[]}}`),
				}, nil
			case "/api/v1/clusters":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "clusters-empty",
					Body:        []byte(`{"success":true,"data":{"items":[]}}`),
				}, nil
			case "/api/v1/projects/prj_demo/deployment-bindings":
				switch req.Method {
				case "GET":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "bindings",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"bind_standalone_demo","project_id":"prj_demo","name":"prod-solo-binding","target_ref":"prod-solo-1","runtime_mode":"standalone","target_kind":"instance","target_id":"inst_demo"}]}}`),
					}, nil
				case "POST":
					return transport.Response{
						StatusCode:  500,
						FixtureName: "unexpected-binding-create",
						Body:        []byte(`{"error":"unexpected_create","message":"should not create binding in reuse flow","next_step":"reuse the existing binding instead"}`),
					}, nil
				}
			default:
				return transport.Response{StatusCode: 404}, nil
			}

			return transport.Response{StatusCode: 404}, nil
		},
	}

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      capture,
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"init", "--project", "acme-shop", "--runtime-mode", "standalone", "--target", "prod-solo-1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "selected binding: prod-solo-binding -> prod-solo-1 (standalone, instance)") {
		t.Fatalf("expected init reuse selection, got %q", output)
	}
	for _, request := range capture.requests {
		if request.Method == "POST" && request.Path == "/api/v1/projects/prj_demo/deployment-bindings" {
			t.Fatalf("expected init reuse flow to avoid create-binding POST, got %+v", request)
		}
	}
}

func TestLinkConnectsRepoToProjectInstallationAndTrackedBranch(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareLinkRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	capture := &stubTransport{
		mode: "capture-link",
		do: func(_ context.Context, req transport.Request) (transport.Response, error) {
			switch req.Path {
			case "/api/v1/projects":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "projects",
					Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop","default_branch":"main"}]}}`),
				}, nil
			case "/api/v1/projects/prj_demo/deployment-bindings":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "bindings",
					Body:        []byte(`{"success":true,"data":{"items":[{"id":"bind_standalone_demo","project_id":"prj_demo","name":"prod-solo-binding","target_ref":"prod-solo-1","runtime_mode":"standalone","target_kind":"instance","target_id":"inst_demo"}]}}`),
				}, nil
			case "/api/v1/instances":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "instances",
					Body:        []byte(`{"success":true,"data":{"items":[{"id":"inst_demo","name":"prod-solo-1","status":"online"}]}}`),
				}, nil
			case "/api/v1/mesh-networks":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "mesh-empty",
					Body:        []byte(`{"success":true,"data":{"items":[]}}`),
				}, nil
			case "/api/v1/clusters":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "clusters-empty",
					Body:        []byte(`{"success":true,"data":{"items":[]}}`),
				}, nil
			case "/api/v1/github/app/installations/sync":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "installations",
					Body:        []byte(`{"success":true,"data":{"items":[{"id":"ghi_demo","github_installation_id":48151623,"account_login":"lazyops","account_type":"Organization","scope":{"repositories":[{"id":1001,"name":"acme-shop","owner":"lazyops","default_branch":"main"}]}}]}}`),
				}, nil
			case "/api/v1/projects/prj_demo/repo-link":
				return transport.Response{
					StatusCode:  201,
					FixtureName: "repo-link",
					Body:        []byte(`{"success":true,"data":{"id":"prl_demo","project_id":"prj_demo","github_installation_id":48151623,"github_repo_id":1001,"repo_owner":"lazyops","repo_name":"acme-shop","tracked_branch":"main","preview_enabled":true}}`),
				}, nil
			default:
				return transport.Response{StatusCode: 404}, nil
			}
		},
	}

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      capture,
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"link"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"repo link review ready",
		"local repo: lazyops/acme-shop",
		"tracked branch: main",
		"github app installation: lazyops (48151623)",
		"verified binding: prod-solo-binding -> prod-solo-1 (standalone, instance)",
		"verified target: instance prod-solo-1 [online]",
		"repository linked",
		"repo link: lazyops/acme-shop -> acme-shop on main",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected link output to contain %q, got %q", expected, output)
		}
	}

	if len(capture.requests) != 7 {
		t.Fatalf("expected 7 requests during link flow, got %d", len(capture.requests))
	}
	linkRequest := capture.requests[6]
	if linkRequest.Path != "/api/v1/projects/prj_demo/repo-link" {
		t.Fatalf("expected repo link request path, got %+v", linkRequest)
	}
	if got := linkRequest.Headers["Authorization"]; got != "Bearer lazyops_pat_mock_secret_value" {
		t.Fatalf("expected auth header on repo link request, got %q", got)
	}

	var payload struct {
		InstallationID int64  `json:"installation_id"`
		RepoID         int64  `json:"repo_id"`
		TrackedBranch  string `json:"tracked_branch"`
	}
	if err := json.Unmarshal(linkRequest.Body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.InstallationID != 48151623 || payload.RepoID != 1001 || payload.TrackedBranch != "main" {
		t.Fatalf("expected typed repo-link payload, got %+v", payload)
	}
}

func TestLinkFailsWhenGitHubAppDoesNotGrantRepoAccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareLinkRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport: &stubTransport{
			mode: "stub",
			do: func(_ context.Context, req transport.Request) (transport.Response, error) {
				switch req.Path {
				case "/api/v1/projects":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "projects",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop","default_branch":"main"}]}}`),
					}, nil
				case "/api/v1/projects/prj_demo/deployment-bindings":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "bindings",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"bind_standalone_demo","project_id":"prj_demo","name":"prod-solo-binding","target_ref":"prod-solo-1","runtime_mode":"standalone","target_kind":"instance","target_id":"inst_demo"}]}}`),
					}, nil
				case "/api/v1/instances":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "instances",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"inst_demo","name":"prod-solo-1","status":"offline"}]}}`),
					}, nil
				case "/api/v1/mesh-networks":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "mesh-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				case "/api/v1/clusters":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "clusters-empty",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				default:
					return transport.Response{StatusCode: 404}, nil
				}
			},
		},
		Credentials: store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"link"})
	if err == nil {
		t.Fatal("expected offline target error, got nil")
	}
	if !strings.Contains(err.Error(), "not online or registered") {
		t.Fatalf("expected offline target error, got %v", err)
	}
}

func TestLogoutRevokesRemoteSessionAndClearsCredentials(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"logout", "--yes"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "logged out and revoked the remote CLI session") {
		t.Fatalf("expected logout success output, got %q", stdout.String())
	}

	if _, err := store.Load(context.Background()); err == nil {
		t.Fatal("expected credentials to be cleared after logout")
	} else if err != credentials.ErrNotFound {
		t.Fatalf("expected ErrNotFound after logout, got %v", err)
	}
}

func TestLogoutClearsLocalSessionWhenRemoteRevokeIsUnavailable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport: &stubTransport{
			mode: "stub",
			do: func(_ context.Context, req transport.Request) (transport.Response, error) {
				return transport.Response{
					StatusCode:  404,
					FixtureName: "pat-revoke-not-found",
				}, nil
			},
		},
		Credentials: store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"logout", "--yes"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "logged out and cleared the local CLI session") {
		t.Fatalf("expected local cleanup success output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "remote PAT revoke endpoint is unavailable") {
		t.Fatalf("expected revoke fallback warning, got %q", stderr.String())
	}

	if _, err := store.Load(context.Background()); err == nil {
		t.Fatal("expected credentials to be cleared after logout fallback")
	} else if err != credentials.ErrNotFound {
		t.Fatalf("expected ErrNotFound after logout fallback, got %v", err)
	}
}

func TestLogoutWithoutLocalSessionIsNoop(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    mustTestStore(t),
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"logout"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "no local CLI session found") {
		t.Fatalf("expected noop logout output, got %q", stdout.String())
	}
}

func TestLogoutRequiresConfirmation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"logout"})
	if err == nil {
		t.Fatal("expected confirmation required error, got nil")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes confirmation guidance, got %v", err)
	}
}

func TestDoctorHappyPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"doctor"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"pass auth:",
		"pass lazyops_yaml:",
		"pass repo_link:",
		"pass binding:",
		"pass dependency_declarations:",
		"pass backend_validation:",
		"summary: 6 pass, 0 warn, 0 fail",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected doctor output to contain %q, got %q", expected, output)
		}
	}
	if stderr.String() != "" {
		t.Fatalf("expected no warnings or errors, got stderr %q", stderr.String())
	}
}

func TestDoctorReportsMissingAuthAsCheckFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    mustTestStore(t),
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"doctor"})
	if err == nil {
		t.Fatal("expected doctor to fail when auth is missing")
	}

	if !strings.Contains(stderr.String(), "fail auth:") {
		t.Fatalf("expected auth failure output, got stderr %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "warn repo_link:") {
		t.Fatalf("expected repo_link warning output, got stderr %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "next: run `lazyops login") {
		t.Fatalf("expected next-step guidance in stdout, got %q", stdout.String())
	}
}

func TestDoctorFailsWhenBindingIsMissing(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepoWithYAML(t, ""+
		"project_slug: acme-shop\n"+
		"runtime_mode: standalone\n\n"+
		"deployment_binding:\n"+
		"  target_ref: missing-binding\n\n"+
		"services:\n"+
		"  - name: api\n"+
		"    path: apps/api\n\n"+
		"compatibility_policy:\n"+
		"  env_injection: true\n"+
		"  managed_credentials: true\n"+
		"  localhost_rescue: true\n")
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"doctor"})
	if err == nil {
		t.Fatal("expected doctor to fail when binding is missing")
	}

	if !strings.Contains(stderr.String(), "fail binding:") {
		t.Fatalf("expected binding failure output, got stderr %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "rerun `lazyops init` to create or reuse a compatible binding") {
		t.Fatalf("expected binding next-step guidance, got %q", stdout.String())
	}
}

func TestDoctorFailsWhenServiceDeclarationIsMissing(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorMeshRepoWithYAML(t, ""+
		"project_slug: acme-shop\n"+
		"runtime_mode: distributed-mesh\n\n"+
		"deployment_binding:\n"+
		"  target_ref: prod-ap\n\n"+
		"services:\n"+
		"  - name: api\n"+
		"    path: apps/api\n\n"+
		"compatibility_policy:\n"+
		"  env_injection: true\n"+
		"  managed_credentials: true\n"+
		"  localhost_rescue: true\n")
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"doctor"})
	if err == nil {
		t.Fatal("expected doctor to fail when service declarations are incomplete")
	}

	if !strings.Contains(stderr.String(), "fail dependency_declarations:") {
		t.Fatalf("expected dependency declaration failure output, got stderr %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "apps/web") {
		t.Fatalf("expected missing service path in failure output, got stderr %q", stderr.String())
	}
}

func TestDoctorFailsOutsideRepositoryRoot(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	restore := mustChdir(t, t.TempDir())
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"doctor"})
	if err == nil {
		t.Fatal("expected doctor to fail outside a repository root")
	}
	if !strings.Contains(stderr.String(), "fail lazyops_yaml: repository root was not found") {
		t.Fatalf("expected repository root failure, got stderr %q", stderr.String())
	}
}

func TestDoctorFailsWhenLazyopsYAMLIsMissing(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "config"), "[remote \"origin\"]\n\turl = git@github.com:lazyops/acme-shop.git\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "HEAD"), "ref: refs/heads/main\n")
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"doctor"})
	if err == nil {
		t.Fatal("expected doctor to fail when lazyops.yaml is missing")
	}
	if !strings.Contains(stderr.String(), "fail lazyops_yaml: lazyops.yaml is missing at the repository root") {
		t.Fatalf("expected lazyops.yaml missing failure, got stderr %q", stderr.String())
	}
}

func TestDoctorFailsWhenRuntimeModeMismatchesBinding(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepoWithYAML(t, ""+
		"project_slug: acme-shop\n"+
		"runtime_mode: distributed-mesh\n\n"+
		"deployment_binding:\n"+
		"  target_ref: prod-solo-1\n\n"+
		"services:\n"+
		"  - name: api\n"+
		"    path: apps/api\n\n"+
		"compatibility_policy:\n"+
		"  env_injection: true\n"+
		"  managed_credentials: true\n"+
		"  localhost_rescue: true\n")
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"doctor"})
	if err == nil {
		t.Fatal("expected doctor to fail when runtime mode mismatches the binding")
	}
	if !strings.Contains(stderr.String(), "fail backend_validation: binding \"prod-solo-1\" uses runtime mode \"standalone\"") {
		t.Fatalf("expected backend validation runtime mismatch failure, got stderr %q", stderr.String())
	}
}

func TestDoctorFailsWhenDependencyBindingIsInvalid(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepoWithYAML(t, ""+
		"project_slug: acme-shop\n"+
		"runtime_mode: standalone\n\n"+
		"deployment_binding:\n"+
		"  target_ref: prod-solo-1\n\n"+
		"services:\n"+
		"  - name: api\n"+
		"    path: apps/api\n\n"+
		"dependency_bindings:\n"+
		"  - service: worker\n"+
		"    alias: api\n"+
		"    target_service: api\n"+
		"    protocol: http\n\n"+
		"compatibility_policy:\n"+
		"  env_injection: true\n"+
		"  managed_credentials: true\n"+
		"  localhost_rescue: true\n")
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"doctor"})
	if err == nil {
		t.Fatal("expected doctor to fail when dependency bindings are invalid")
	}
	if !strings.Contains(stderr.String(), `fail lazyops_yaml: lazyops.yaml dependency_bindings[0]: service "worker" is not declared in services`) {
		t.Fatalf("expected dependency binding validation failure, got stderr %q", stderr.String())
	}
}

func TestStatusHappyPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"status"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"status summary",
		"source: existing-api-composition/v1",
		"project: Acme Shop (acme-shop)",
		"control-plane: validated (binding prod-solo-binding and target instance prod-solo-1 [online] validated)",
		"binding state: attached (prod-solo-binding -> prod-solo-1)",
		"topology state: healthy (instance prod-solo-1, status=online)",
		"deployment state: ready (deploy contract, binding, and topology are aligned)",
		"rollout: idle",
		"next: push or open a pull request to trigger deployment through GitHub",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected status output to contain %q, got %q", expected, output)
		}
	}
	if stderr.String() != "" {
		t.Fatalf("expected no warnings or errors, got stderr %q", stderr.String())
	}
}

func TestStatusReportsMissingBindingAsBlocked(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepoWithYAML(t, ""+
		"project_slug: acme-shop\n"+
		"runtime_mode: standalone\n\n"+
		"deployment_binding:\n"+
		"  target_ref: missing-binding\n\n"+
		"services:\n"+
		"  - name: api\n"+
		"    path: apps/api\n\n"+
		"compatibility_policy:\n"+
		"  env_injection: true\n"+
		"  managed_credentials: true\n"+
		"  localhost_rescue: true\n")
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"status"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stderr.String(), `control-plane: failed (deployment_binding.target_ref "missing-binding" is not registered for project "acme-shop"`) {
		t.Fatalf("expected control-plane validation failure, got stderr %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "binding state: missing target_ref missing-binding") {
		t.Fatalf("expected missing binding output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `deployment state: blocked (deployment_binding.target_ref "missing-binding" is not registered for project "acme-shop"`) {
		t.Fatalf("expected blocked deployment output, got stderr %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "next: create or reuse a compatible deployment binding and retry validation") {
		t.Fatalf("expected blocked next-step output, got %q", stdout.String())
	}
}

func TestStatusReportsOfflineTargetAsDegraded(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport: &stubTransport{
			mode: "stub",
			do: func(_ context.Context, req transport.Request) (transport.Response, error) {
				switch req.Path {
				case "/api/v1/projects":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "projects-list",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop","default_branch":"main"}]}}`),
					}, nil
				case "/api/v1/projects/prj_demo/deployment-bindings":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "deployment-bindings",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"bind_standalone_demo","project_id":"prj_demo","name":"prod-solo-binding","target_ref":"prod-solo-1","runtime_mode":"standalone","target_kind":"instance","target_id":"inst_demo"}]}}`),
					}, nil
				case "/api/v1/instances":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "instances-list",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"inst_demo","name":"prod-solo-1","status":"offline"}]}}`),
					}, nil
				case "/api/v1/mesh-networks":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "mesh-list",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				case "/api/v1/clusters":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "clusters-list",
						Body:        []byte(`{"success":true,"data":{"items":[]}}`),
					}, nil
				default:
					return transport.Response{}, nil
				}
			},
		},
		Credentials: store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"status"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stderr.String(), "control-plane: unavailable") {
		t.Fatalf("expected control-plane fallback warning, got stderr %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "topology state: degraded (instance prod-solo-1, status=offline)") {
		t.Fatalf("expected degraded topology output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "deployment state: degraded (deploy contract is attached, but the target is not ready)") {
		t.Fatalf("expected degraded deployment output, got stderr %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "rollout: paused") {
		t.Fatalf("expected paused rollout output, got %q", stdout.String())
	}
}

func TestStatusBlocksWhenControlPlaneValidationDetectsRuntimeMismatch(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepoWithYAML(t, ""+
		"project_slug: acme-shop\n"+
		"runtime_mode: distributed-mesh\n\n"+
		"deployment_binding:\n"+
		"  target_ref: prod-solo-1\n\n"+
		"services:\n"+
		"  - name: api\n"+
		"    path: apps/api\n\n"+
		"compatibility_policy:\n"+
		"  env_injection: true\n"+
		"  managed_credentials: true\n"+
		"  localhost_rescue: true\n")
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"status"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stderr.String(), `control-plane: failed (binding "prod-solo-1" uses runtime mode "standalone"`) {
		t.Fatalf("expected control-plane runtime mismatch failure, got stderr %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), `deployment state: blocked (binding "prod-solo-1" uses runtime mode "standalone"`) {
		t.Fatalf("expected blocked deployment on runtime mismatch, got stderr %q", stderr.String())
	}
}

func TestLogsHappyPathWithFilters(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"logs", "api", "--level", "error", "--node", "edge-ap-2", "--limit", "1"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"logs stream",
		"project: Acme Shop (acme-shop)",
		"service: api",
		"filters: level=error, node=edge-ap-2, limit=1",
		"cursor: cursor_demo_002",
		"ERROR [edge-ap-2] upstream timeout while contacting postgres",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected logs output to contain %q, got %q", expected, output)
		}
	}
	if strings.Contains(output, "GET /health 200 1.2ms") {
		t.Fatalf("expected filtered output to omit non-matching lines, got %q", output)
	}
	if stderr.String() != "" {
		t.Fatalf("expected no warnings or errors, got stderr %q", stderr.String())
	}
}

func TestLogsRejectsUnknownServiceFromDeployContract(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"logs", "worker"})
	if err == nil {
		t.Fatal("expected service validation error, got nil")
	}
	if !strings.Contains(err.Error(), `service "worker" is not declared in lazyops.yaml for project "acme-shop"`) {
		t.Fatalf("expected service-specific error, got %v", err)
	}
}

func TestLogsRedactsSecretsFromRenderedLines(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	capture := &stubTransport{
		mode: "stub",
		do: func(_ context.Context, req transport.Request) (transport.Response, error) {
			switch req.Path {
			case "/api/v1/projects":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "projects-list",
					Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop","default_branch":"main"}]}}`),
				}, nil
			case "/ws/logs/stream":
				return transport.Response{
					StatusCode:  200,
					FixtureName: "logs-stream",
					Body:        []byte(`{"success":true,"data":{"service":"api","cursor":"cursor_demo","lines":[{"timestamp":"2026-03-31T12:00:00Z","level":"info","message":"Authorization: Bearer lazyops_pat_mock_secret_value","node":"edge-ap-1"}]}}`),
				}, nil
			default:
				return transport.Response{}, nil
			}
		},
	}

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      capture,
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"logs", "api", "--contains", "authorization", "--limit", "10"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if strings.Contains(output, "lazyops_pat_mock_secret_value") {
		t.Fatalf("expected secret token to be redacted, got %q", output)
	}
	if !strings.Contains(output, "[redacted]") {
		t.Fatalf("expected redaction marker in logs output, got %q", output)
	}
	if len(capture.requests) != 2 {
		t.Fatalf("expected project lookup and logs request, got %d requests", len(capture.requests))
	}
	logsRequest := capture.requests[1]
	if logsRequest.Path != "/ws/logs/stream" {
		t.Fatalf("expected logs request path, got %+v", logsRequest)
	}
	if logsRequest.Query["project"] != "prj_demo" || logsRequest.Query["service"] != "api" {
		t.Fatalf("expected logs query to include project and service, got %+v", logsRequest.Query)
	}
	if logsRequest.Query["contains"] != "authorization" || logsRequest.Query["limit"] != "10" {
		t.Fatalf("expected logs filters to be forwarded, got %+v", logsRequest.Query)
	}
}

func TestLogsCancellationReturnsCleanly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareDoctorRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport: &stubTransport{
			mode: "stub",
			do: func(ctx context.Context, req transport.Request) (transport.Response, error) {
				switch req.Path {
				case "/api/v1/projects":
					return transport.Response{
						StatusCode:  200,
						FixtureName: "projects-list",
						Body:        []byte(`{"success":true,"data":{"items":[{"id":"prj_demo","name":"Acme Shop","slug":"acme-shop","default_branch":"main"}]}}`),
					}, nil
				case "/ws/logs/stream":
					<-ctx.Done()
					return transport.Response{}, ctx.Err()
				default:
					return transport.Response{}, nil
				}
			},
		},
		Credentials: store,
	}

	time.AfterFunc(10*time.Millisecond, cancel)

	root := NewRootCommand()
	if err := root.Execute(ctx, runtime, []string{"logs", "api"}); err != nil {
		t.Fatalf("expected clean cancellation, got %v", err)
	}

	if !strings.Contains(stderr.String(), "log stream cancelled for service api in project acme-shop") {
		t.Fatalf("expected cancellation warning, got stderr %q", stderr.String())
	}
}

func TestProtectedCommandsRequireLogin(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "init", args: []string{"init"}},
		{name: "link", args: []string{"link"}},
		{name: "status", args: []string{"status"}},
		{name: "bindings", args: []string{"bindings"}},
		{name: "logs", args: []string{"logs", "api"}},
		{name: "traces", args: []string{"traces", "corr-demo"}},
		{name: "tunnel-db", args: []string{"tunnel", "db"}},
		{name: "tunnel-tcp", args: []string{"tunnel", "tcp"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			runtime := &Runtime{
				Output:         ui.NewConsoleOutput(&stdout, &stderr),
				SpinnerFactory: ui.NewSpinnerFactory(&stderr),
				Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
				Credentials:    mustTestStore(t),
			}

			root := NewRootCommand()
			err := root.Execute(context.Background(), runtime, tc.args)
			if err == nil {
				t.Fatal("expected auth guard error, got nil")
			}
			if !strings.Contains(err.Error(), "CLI is not logged in") {
				t.Fatalf("expected login requirement error, got %v", err)
			}
		})
	}
}

func TestAuthorizedCommandInjectsBearerToken(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	capture := &stubTransport{
		mode: "capture",
		do: func(_ context.Context, req transport.Request) (transport.Response, error) {
			return transport.Response{
				StatusCode:  200,
				FixtureName: "trace-summary",
				Body:        []byte(`{"success":true,"data":{"correlation_id":"corr-demo","service_path":["gateway","api"]}}`),
			}, nil
		},
	}

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      capture,
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"traces", "corr-demo"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(capture.requests) != 1 {
		t.Fatalf("expected one authorized request, got %d", len(capture.requests))
	}
	if got := capture.requests[0].Headers["Authorization"]; got != "Bearer lazyops_pat_mock_secret_value" {
		t.Fatalf("expected Authorization header to be injected, got %q", got)
	}
}

func TestTunnelDBFailsWhenProjectNotFound(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: unknown-project\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"tunnel", "db"})
	if err == nil {
		t.Fatal("expected project not found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected project not found error, got %v", err)
	}
}

func TestTunnelDBFailsWhenPortConflict(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"tunnel", "db", "--port", "99999"})
	if err == nil {
		t.Fatal("expected port conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "port") {
		t.Fatalf("expected port conflict error, got %v", err)
	}
	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error with next step, got %v", err)
	}
}

func TestTunnelDBFailsWhenPortIsBusy(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	listener, err := net.Listen("tcp", ":15433")
	if err != nil {
		t.Fatalf("failed to bind test port: %v", err)
	}
	defer listener.Close()

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err = root.Execute(context.Background(), runtime, []string{"tunnel", "db", "--port", "15433"})
	if err == nil {
		t.Fatal("expected busy port error, got nil")
	}
	if !strings.Contains(err.Error(), "not available") && !strings.Contains(err.Error(), "already") {
		t.Fatalf("expected busy port error, got %v", err)
	}
	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error with next step, got %v", err)
	}
}

func TestTunnelStopRequiresSessionID(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"tunnel", "stop"})
	if err == nil {
		t.Fatal("expected missing session id error, got nil")
	}
	if !strings.Contains(err.Error(), "session id is required") {
		t.Fatalf("expected session id required error, got %v", err)
	}
}

func TestTunnelStopHandlesNonexistentSession(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"tunnel", "stop", "tun_nonexistent"})
	if err == nil {
		t.Fatal("expected session not found error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected session not found error, got %v", err)
	}
	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error with next step, got %v", err)
	}
}

func TestTunnelStopSuccess(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"tunnel", "stop", "tun_demo_15432"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "tunnel session tun_demo_15432 stopped") {
		t.Fatalf("expected stop success message, got %q", output)
	}
	if !strings.Contains(output, "local port released") {
		t.Fatalf("expected local port released message, got %q", output)
	}
}

func TestTunnelListShowsNoActiveSessions(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"tunnel", "list"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "no active tunnel sessions") {
		t.Fatalf("expected no active sessions message, got %q", output)
	}
}

func TestTunnelTCPRequiresPort(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"tunnel", "tcp"})
	if err == nil {
		t.Fatal("expected --port required error, got nil")
	}
	if !strings.Contains(err.Error(), "--port is required") {
		t.Fatalf("expected --port required error, got %v", err)
	}
	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error with next step, got %v", err)
	}
}

func TestTunnelTCPSuccessCreatesSession(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	if err := root.Execute(context.Background(), runtime, []string{"tunnel", "tcp", "--port", "19090", "--remote", "localhost:8080"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "tunnel session created") {
		t.Fatalf("expected tunnel session created message, got %q", output)
	}
	if !strings.Contains(output, "type: tcp") {
		t.Fatalf("expected tunnel type tcp, got %q", output)
	}
	if !strings.Contains(output, "local port: 19090") {
		t.Fatalf("expected local port 19090, got %q", output)
	}
	if !strings.Contains(output, "remote: localhost:8080") {
		t.Fatalf("expected remote localhost:8080, got %q", output)
	}
	if !strings.Contains(output, "connect: tcp://127.0.0.1:19090") {
		t.Fatalf("expected connect info, got %q", output)
	}
	if !strings.Contains(output, "stop with:") {
		t.Fatalf("expected stop hint, got %q", output)
	}
	if !strings.Contains(stderr.String(), "debug tunnel") {
		t.Fatalf("expected debug tunnel warning in stderr, got %q", stderr.String())
	}
}

func TestTunnelTCPPortConflict(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err := root.Execute(context.Background(), runtime, []string{"tunnel", "tcp", "--port", "99999", "--remote", "localhost:8080"})
	if err == nil {
		t.Fatal("expected port conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "port") {
		t.Fatalf("expected port conflict error, got %v", err)
	}
	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error with next step, got %v", err)
	}
}

func TestTunnelTCPBusyPort(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	repoRoot := mustPrepareInitRepo(t)
	restore := mustChdir(t, repoRoot)
	defer restore()
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod-solo-1\nservices:\n  - name: api\n    path: apps/api\n")

	store := mustTestStore(t)
	mustSeedCredential(t, store, credentials.Record{
		Token:       "lazyops_pat_mock_secret_value",
		UserID:      "usr_demo",
		DisplayName: "CLI Demo User",
	})

	listener, err := net.Listen("tcp", ":19091")
	if err != nil {
		t.Fatalf("failed to bind test port: %v", err)
	}
	defer listener.Close()

	runtime := &Runtime{
		Output:         ui.NewConsoleOutput(&stdout, &stderr),
		SpinnerFactory: ui.NewSpinnerFactory(&stderr),
		Transport:      transport.NewMockTransport(transport.DefaultFixtures()),
		Credentials:    store,
	}

	root := NewRootCommand()
	err = root.Execute(context.Background(), runtime, []string{"tunnel", "tcp", "--port", "19091", "--remote", "localhost:8080"})
	if err == nil {
		t.Fatal("expected busy port error, got nil")
	}
	if !strings.Contains(err.Error(), "not available") && !strings.Contains(err.Error(), "already") {
		t.Fatalf("expected busy port error, got %v", err)
	}
	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error with next step, got %v", err)
	}
}

func mustTestStore(t *testing.T) *credentials.Store {
	t.Helper()

	store, err := credentials.NewStoreWithKeychain(credentials.StoreConfig{
		Service:         "lazyops-cli",
		Account:         "default",
		CredentialsPath: t.TempDir() + "/credentials.json",
	}, &testKeychain{})
	if err != nil {
		t.Fatalf("NewStoreWithKeychain() error = %v", err)
	}

	return store
}

func mustSeedCredential(t *testing.T, store *credentials.Store, record credentials.Record) {
	t.Helper()

	if _, err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func mustWriteTestFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func mustPrepareInitRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "keep"), "")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "go.mod"), "module api\n\ngo 1.22.2\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "cmd", "server", "main.go"), "package main\nfunc main(){}\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "internal", "api", "routes.go"), `package api; func routes(){ GET("/healthz", nil) }`)
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "api", "internal", "config", "config.go"), "package config\nconst _ = `SERVER_PORT\", \"8080\"`\n")
	return repoRoot
}

func mustPrepareMeshInitRepo(t *testing.T) string {
	t.Helper()

	repoRoot := mustPrepareInitRepo(t)
	mustWriteTestFile(t, filepath.Join(repoRoot, "apps", "web", "package.json"), `{"name":"web","scripts":{"start":"next start"}}`)
	return repoRoot
}

func mustPrepareLinkRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "config"), "[remote \"origin\"]\n\turl = git@github.com:lazyops/acme-shop.git\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "HEAD"), "ref: refs/heads/main\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), "project_slug: acme-shop\nruntime_mode: standalone\n\ndeployment_binding:\n  target_ref: prod-solo-1\n")
	return repoRoot
}

func mustPrepareDoctorRepo(t *testing.T) string {
	t.Helper()

	return mustPrepareDoctorRepoWithYAML(t, ""+
		"project_slug: acme-shop\n"+
		"runtime_mode: standalone\n\n"+
		"deployment_binding:\n"+
		"  target_ref: prod-solo-1\n\n"+
		"services:\n"+
		"  - name: api\n"+
		"    path: apps/api\n\n"+
		"compatibility_policy:\n"+
		"  env_injection: true\n"+
		"  managed_credentials: true\n"+
		"  localhost_rescue: true\n")
}

func mustPrepareDoctorRepoWithYAML(t *testing.T, lazyopsYAML string) string {
	t.Helper()

	repoRoot := mustPrepareInitRepo(t)
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "config"), "[remote \"origin\"]\n\turl = git@github.com:lazyops/acme-shop.git\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "HEAD"), "ref: refs/heads/main\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), lazyopsYAML)
	return repoRoot
}

func mustPrepareDoctorMeshRepoWithYAML(t *testing.T, lazyopsYAML string) string {
	t.Helper()

	repoRoot := mustPrepareMeshInitRepo(t)
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "config"), "[remote \"origin\"]\n\turl = git@github.com:lazyops/acme-shop.git\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, ".git", "HEAD"), "ref: refs/heads/main\n")
	mustWriteTestFile(t, filepath.Join(repoRoot, "lazyops.yaml"), lazyopsYAML)
	return repoRoot
}

func mustChdir(t *testing.T, dir string) func() {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}

	return func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("Chdir() restore error = %v", err)
		}
	}
}

type testKeychain struct {
	payload []byte
}

func (k *testKeychain) Name() string {
	return "keychain"
}

func (k *testKeychain) Save(_ context.Context, _ string, _ string, payload []byte) error {
	k.payload = append([]byte(nil), payload...)
	return nil
}

func (k *testKeychain) Load(_ context.Context, _ string, _ string) ([]byte, error) {
	if len(k.payload) == 0 {
		return nil, credentials.ErrNotFound
	}

	return append([]byte(nil), k.payload...), nil
}

func (k *testKeychain) Delete(_ context.Context, _ string, _ string) error {
	if len(k.payload) == 0 {
		return credentials.ErrNotFound
	}
	k.payload = nil
	return nil
}

type fakeSpinnerFactory struct {
	spinner *fakeSpinner
}

func (f *fakeSpinnerFactory) New() ui.Spinner {
	if f.spinner == nil {
		f.spinner = &fakeSpinner{}
	}
	return f.spinner
}

type fakeSpinner struct {
	startCalls int
	stopCalls  int
	lastStart  string
	lastStop   string
}

func (s *fakeSpinner) Start(message string) {
	s.startCalls++
	s.lastStart = message
}

func (s *fakeSpinner) Update(string) {}

func (s *fakeSpinner) Stop(message string) {
	s.stopCalls++
	s.lastStop = message
}

type stubTransport struct {
	mode     string
	do       func(ctx context.Context, req transport.Request) (transport.Response, error)
	requests []transport.Request
}

func (s *stubTransport) Do(ctx context.Context, req transport.Request) (transport.Response, error) {
	s.requests = append(s.requests, req)
	if s.do == nil {
		return transport.Response{}, nil
	}
	return s.do(ctx, req)
}

func (s *stubTransport) Mode() string {
	if strings.TrimSpace(s.mode) == "" {
		return "stub"
	}
	return s.mode
}

var _ credentials.Keychain = (*testKeychain)(nil)
var _ ui.SpinnerFactory = (*fakeSpinnerFactory)(nil)
var _ ui.Spinner = (*fakeSpinner)(nil)
var _ transport.Transport = (*stubTransport)(nil)
