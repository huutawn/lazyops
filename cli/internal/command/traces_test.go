package command

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/transport"
	"lazyops-cli/internal/ui"
)

func TestTracesCommandRequiresCorrelationID(t *testing.T) {
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
	err := root.Execute(context.Background(), runtime, []string{"traces"})
	if err == nil {
		t.Fatal("expected missing correlation id error, got nil")
	}
	if !strings.Contains(err.Error(), "correlation id is required") {
		t.Fatalf("expected correlation id required error, got %v", err)
	}
	if !strings.Contains(err.Error(), "next:") {
		t.Fatalf("expected actionable error, got %v", err)
	}
}

func TestTracesCommandRendersTraceSummary(t *testing.T) {
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
	if err := root.Execute(context.Background(), runtime, []string{"traces", "corr-demo"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "correlation id: corr-demo") {
		t.Fatalf("expected correlation id in output, got %q", output)
	}
	if !strings.Contains(output, "service path:") {
		t.Fatalf("expected service path header, got %q", output)
	}
	if !strings.Contains(output, "gateway") {
		t.Fatalf("expected gateway service in path, got %q", output)
	}
	if !strings.Contains(output, "node hops:") {
		t.Fatalf("expected node hops header, got %q", output)
	}
	if !strings.Contains(output, "edge-ap-1") {
		t.Fatalf("expected edge-ap-1 node hop, got %q", output)
	}
	if !strings.Contains(output, "total latency: 182ms") {
		t.Fatalf("expected total latency, got %q", output)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "latency hotspot: api -> postgres") {
		t.Fatalf("expected latency hotspot in stderr, got %q", errOutput)
	}
}

func TestTracesCommandHandlesUnknownCorrelationID(t *testing.T) {
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
	if err := root.Execute(context.Background(), runtime, []string{"traces", "corr-unknown-xyz"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "correlation id: corr-unknown-xyz") {
		t.Fatalf("expected unknown correlation id in output, got %q", output)
	}
	if !strings.Contains(output, "service path:") {
		t.Fatalf("expected service path header for unknown id, got %q", output)
	}
}

func TestTracesCommandHandlesErrorDemoTrace(t *testing.T) {
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
	if err := root.Execute(context.Background(), runtime, []string{"traces", "corr-error-demo"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "correlation id: corr-error-demo") {
		t.Fatalf("expected correlation id in output, got %q", output)
	}
	if !strings.Contains(output, "external-service") {
		t.Fatalf("expected external-service in path, got %q", output)
	}
	if !strings.Contains(output, "total latency: 5230ms") {
		t.Fatalf("expected high latency, got %q", output)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "latency hotspot: api -> external-service") {
		t.Fatalf("expected latency hotspot in stderr, got %q", errOutput)
	}
}

func TestTracesCommandRequiresAuth(t *testing.T) {
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
	err := root.Execute(context.Background(), runtime, []string{"traces", "corr-demo"})
	if err == nil {
		t.Fatal("expected auth required error, got nil")
	}
	if !strings.Contains(err.Error(), "not logged in") && !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("expected auth required error, got %v", err)
	}
}

func TestParseTracesArgsRejectsFlagAsCorrelationID(t *testing.T) {
	_, err := parseTracesArgs([]string{"--help"})
	if err == nil {
		t.Fatal("expected error for flag as correlation id, got nil")
	}
	if !strings.Contains(err.Error(), "correlation id is required") {
		t.Fatalf("expected correlation id required error, got %v", err)
	}
}
