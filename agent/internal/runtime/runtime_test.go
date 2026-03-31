package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/dispatcher"
	"lazyops-agent/internal/state"
)

func TestContextFromPreparePayloadRejectsK3s(t *testing.T) {
	_, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeDistributedK3s))
	if err == nil {
		t.Fatal("expected k3s runtime mode to be rejected by local runtime driver context")
	}
}

func TestFilesystemDriverPrepareReleaseWorkspaceCreatesLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	driver.now = func() time.Time {
		return time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	}

	runtimeCtx, err := ContextFromPreparePayload(samplePreparePayload(contracts.RuntimeModeStandalone))
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}

	prepared, err := driver.PrepareReleaseWorkspace(context.Background(), runtimeCtx)
	if err != nil {
		t.Fatalf("prepare release workspace: %v", err)
	}

	for _, path := range []string{
		prepared.Layout.Root,
		prepared.Layout.Artifacts,
		prepared.Layout.Config,
		prepared.Layout.Sidecars,
		prepared.Layout.Gateway,
		prepared.Layout.Services,
		prepared.ManifestPath,
		prepared.ServiceManifests["api"],
		prepared.ServiceManifests["web"],
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected path %s to exist: %v", path, err)
		}
	}

	manifestRaw, err := os.ReadFile(prepared.ManifestPath)
	if err != nil {
		t.Fatalf("read workspace manifest: %v", err)
	}
	if strings.Contains(strings.ToLower(string(manifestRaw)), "container") {
		t.Fatal("expected workspace manifest to remain service-oriented and not leak container terminology")
	}

	var manifest WorkspaceManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode workspace manifest: %v", err)
	}
	if manifest.ArtifactPlan.Status != "pending_fetch" {
		t.Fatalf("expected artifact plan status pending_fetch, got %q", manifest.ArtifactPlan.Status)
	}
	if got := len(manifest.GatewayPlan.PublicServices); got != 1 {
		t.Fatalf("expected 1 public service in gateway plan, got %d", got)
	}
}

func TestServicePrepareReleaseWorkspaceHandlerUpdatesRevisionCache(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), filepath.Join(t.TempDir(), "runtime-root"))
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)
	service.now = func() time.Time {
		return time.Date(2026, 3, 31, 10, 30, 0, 0, time.UTC)
	}

	payload := samplePreparePayload(contracts.RuntimeModeDistributedMesh)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_prepare",
		CorrelationID: "corr_prepare",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	})
	if result.Error != nil {
		t.Fatalf("expected handler to succeed, got error %#v", result.Error)
	}
	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done status, got %q", result.Status)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.PendingRevisionID != payload.Revision.RevisionID {
		t.Fatalf("expected pending revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.PendingRevisionID)
	}
}

func TestFilesystemDriverStubOperationsStayUnimplemented(t *testing.T) {
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), filepath.Join(t.TempDir(), "runtime-root"))
	runtimeCtx := RuntimeContext{}

	if err := driver.StartReleaseCandidate(context.Background(), runtimeCtx); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected start release candidate to remain unimplemented, got %v", err)
	}
	if err := driver.GarbageCollectRuntime(context.Background(), runtimeCtx); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected garbage collect runtime to remain unimplemented, got %v", err)
	}
}

func TestPrepareReleaseWorkspaceHandlerReturnsValidationErrorForBadPayload(t *testing.T) {
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, NewFilesystemDriver(nil, filepath.Join(t.TempDir(), "runtime-root")))

	result := service.handlePrepareReleaseWorkspace(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPrepareReleaseWorkspace,
		RequestID:     "req_bad",
		CorrelationID: "corr_bad",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       json.RawMessage(`{"project":{},"binding":{},"revision":{}}`),
	})
	if result.Error == nil {
		t.Fatal("expected validation failure for incomplete payload")
	}
	if result.Error.Retryable {
		t.Fatal("expected invalid runtime context to be non-retryable")
	}
}

func TestPrepareReleaseWorkspaceHandlerCanBeRegistered(t *testing.T) {
	registry := dispatcher.NewDefaultRegistry()
	service := NewService(nil, nil, NewFilesystemDriver(nil, filepath.Join(t.TempDir(), "runtime-root")))
	service.Register(registry)

	if _, ok := registry.Resolve(contracts.CommandPrepareReleaseWorkspace); !ok {
		t.Fatal("expected runtime service to register prepare_release_workspace handler")
	}
}

func samplePreparePayload(mode contracts.RuntimeMode) contracts.PrepareReleaseWorkspacePayload {
	return contracts.PrepareReleaseWorkspacePayload{
		Project: contracts.ProjectMetadataPayload{
			ProjectID: "prj_123",
			Name:      "Lazy App",
			Slug:      "lazy-app",
		},
		Binding: contracts.DeploymentBindingPayload{
			BindingID:   "bind_123",
			ProjectID:   "prj_123",
			Name:        "prod",
			TargetRef:   "prod-main",
			RuntimeMode: mode,
			TargetKind:  contracts.TargetKindInstance,
			TargetID:    "inst_123",
			PlacementPolicy: contracts.PlacementPolicy{
				Strategy: "spread",
			},
			DomainPolicy: contracts.DomainPolicy{
				Enabled:  true,
				Provider: "sslip.io",
			},
			CompatibilityPolicy: contracts.CompatibilityPolicy{
				EnvInjection:       true,
				ManagedCredentials: true,
				LocalhostRescue:    true,
			},
			ScaleToZeroPolicy: contracts.ScaleToZeroPolicy{
				Enabled: false,
			},
		},
		Revision: contracts.DesiredRevisionPayload{
			RevisionID:          "rev_123",
			ProjectID:           "prj_123",
			BlueprintID:         "bp_123",
			DeploymentBindingID: "bind_123",
			CommitSHA:           "abc123",
			TriggerKind:         "git_push",
			RuntimeMode:         mode,
			Services: []contracts.ServicePayload{
				{
					Name:   "api",
					Path:   "services/api",
					Public: false,
					HealthCheck: contracts.HealthCheckPayload{
						Protocol: "http",
						Port:     8080,
						Path:     "/health",
					},
				},
				{
					Name:   "web",
					Path:   "services/web",
					Public: true,
					HealthCheck: contracts.HealthCheckPayload{
						Protocol: "http",
						Port:     3000,
						Path:     "/ready",
					},
				},
			},
			DependencyBindings: []contracts.DependencyBindingPayload{
				{
					Service:       "web",
					Alias:         "api",
					TargetService: "api",
					Protocol:      "http",
					LocalEndpoint: "http://localhost:8080",
				},
			},
			CompatibilityPolicy: contracts.CompatibilityPolicy{
				EnvInjection:       true,
				ManagedCredentials: true,
				LocalhostRescue:    true,
			},
			MagicDomainPolicy: contracts.MagicDomainPolicy{
				Enabled:  true,
				Provider: "sslip.io",
			},
			ScaleToZeroPolicy: contracts.ScaleToZeroPolicy{
				Enabled: false,
			},
			PlacementAssignments: []contracts.PlacementAssignment{
				{
					ServiceName: "api",
					TargetID:    "inst_123",
					TargetKind:  contracts.TargetKindInstance,
				},
			},
		},
	}
}
