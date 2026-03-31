package command

import (
	"bytes"
	"context"
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
