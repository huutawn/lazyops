package enroll

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/control"
	"lazyops-agent/internal/state"
)

func TestEnrollSuccessPersistsEncryptedToken(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	client := control.NewMockClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	service := New(store, client, slog.New(slog.NewTextHandler(io.Discard, nil)), "state-key-123")

	response, err := service.Enroll(
		context.Background(),
		"bootstrap-valid-standalone",
		contracts.MachineInfo{
			Hostname: "local-dev",
			Labels: map[string]string{
				"target_ref": "local-dev",
			},
		},
		contracts.CapabilityReportPayload{
			AgentKind:   contracts.AgentKindInstance,
			RuntimeMode: contracts.RuntimeModeStandalone,
		},
		contracts.RuntimeModeStandalone,
		contracts.AgentKindInstance,
	)
	if err != nil {
		t.Fatalf("enroll agent: %v", err)
	}

	current, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !current.Enrollment.Enrolled {
		t.Fatal("expected agent to be marked enrolled")
	}
	if current.Enrollment.EncryptedAgentToken == "" {
		t.Fatal("expected encrypted agent token to be persisted")
	}
	if current.Enrollment.EncryptedAgentToken == response.AgentToken {
		t.Fatal("expected stored token to be encrypted, not plaintext")
	}

	decrypted, err := service.LoadAgentToken(context.Background())
	if err != nil {
		t.Fatalf("load agent token: %v", err)
	}
	if decrypted != response.AgentToken {
		t.Fatalf("unexpected decrypted token %q", decrypted)
	}
}

func TestEnrollRejectsExpiredBootstrapToken(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	client := control.NewMockClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	service := New(store, client, slog.New(slog.NewTextHandler(io.Discard, nil)), "state-key-123")

	_, err := service.Enroll(
		context.Background(),
		"bootstrap-expired-standalone",
		contracts.MachineInfo{Hostname: "local-dev", Labels: map[string]string{"target_ref": "local-dev"}},
		contracts.CapabilityReportPayload{},
		contracts.RuntimeModeStandalone,
		contracts.AgentKindInstance,
	)
	if !errors.Is(err, control.ErrBootstrapTokenExpired) {
		t.Fatalf("expected expired token error, got %v", err)
	}
}

func TestEnrollRejectsReusedBootstrapToken(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	client := control.NewMockClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	service := New(store, client, slog.New(slog.NewTextHandler(io.Discard, nil)), "state-key-123")

	_, err := service.Enroll(
		context.Background(),
		"bootstrap-reused-standalone",
		contracts.MachineInfo{Hostname: "local-dev", Labels: map[string]string{"target_ref": "local-dev"}},
		contracts.CapabilityReportPayload{},
		contracts.RuntimeModeStandalone,
		contracts.AgentKindInstance,
	)
	if !errors.Is(err, control.ErrBootstrapTokenReused) {
		t.Fatalf("expected reused token error, got %v", err)
	}
}

func TestEnrollRejectsTargetMismatch(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	client := control.NewMockClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	service := New(store, client, slog.New(slog.NewTextHandler(io.Discard, nil)), "state-key-123")

	_, err := service.Enroll(
		context.Background(),
		"bootstrap-valid-standalone",
		contracts.MachineInfo{Hostname: "local-dev", Labels: map[string]string{"target_ref": "wrong-target"}},
		contracts.CapabilityReportPayload{},
		contracts.RuntimeModeStandalone,
		contracts.AgentKindInstance,
	)
	if !errors.Is(err, control.ErrBootstrapTargetMismatch) {
		t.Fatalf("expected target mismatch error, got %v", err)
	}
}

func TestEnrollReturnsErrorOnPartialPersistenceFailure(t *testing.T) {
	readOnlyDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o500); err != nil {
		t.Fatalf("mkdir readonly dir: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0o700)

	store := state.New(filepath.Join(readOnlyDir, "agent-state.json"))
	client := control.NewMockClient(slog.New(slog.NewTextHandler(io.Discard, nil)))
	service := New(store, client, slog.New(slog.NewTextHandler(io.Discard, nil)), "state-key-123")

	_, err := service.Enroll(
		context.Background(),
		"bootstrap-valid-standalone",
		contracts.MachineInfo{Hostname: "local-dev", Labels: map[string]string{"target_ref": "local-dev"}},
		contracts.CapabilityReportPayload{},
		contracts.RuntimeModeStandalone,
		contracts.AgentKindInstance,
	)
	if err == nil {
		t.Fatal("expected partial persistence failure to return an error")
	}

	current, loadErr := store.Load(context.Background())
	if loadErr != nil {
		t.Fatalf("load state after failed persist: %v", loadErr)
	}
	if current.Enrollment.Enrolled {
		t.Fatal("expected failed persistence to keep enrollment state unset")
	}
}
