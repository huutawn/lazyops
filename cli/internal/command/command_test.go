package command

import (
	"bytes"
	"context"
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
	if !strings.Contains(output, "tunnel db is wired to the mock transport") {
		t.Fatalf("expected tunnel db scaffold message, got %q", output)
	}
	if !strings.Contains(output, "fixture: db-tunnel") {
		t.Fatalf("expected db tunnel fixture output, got %q", output)
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
	if !strings.Contains(output, "binding prod-ap-mesh -> prod-ap (distributed-mesh, mesh)") {
		t.Fatalf("expected typed mesh binding output, got %q", output)
	}
	if !strings.Contains(output, "binding prod-solo-binding -> prod-solo-1 (standalone, instance)") {
		t.Fatalf("expected typed standalone binding output, got %q", output)
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
	if err := root.Execute(context.Background(), runtime, []string{"logout"}); err != nil {
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
	if err := root.Execute(context.Background(), runtime, []string{"logout"}); err != nil {
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

func TestProtectedCommandsRequireLogin(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{name: "init", args: []string{"init"}},
		{name: "link", args: []string{"link"}},
		{name: "doctor", args: []string{"doctor"}},
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
				FixtureName: "status",
				Body:        []byte(`{"ok":true}`),
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
	if err := root.Execute(context.Background(), runtime, []string{"status"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(capture.requests) != 1 {
		t.Fatalf("expected one authorized request, got %d", len(capture.requests))
	}
	if got := capture.requests[0].Headers["Authorization"]; got != "Bearer lazyops_pat_mock_secret_value" {
		t.Fatalf("expected Authorization header to be injected, got %q", got)
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
