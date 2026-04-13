package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/state"
)

func TestRollbackReleaseHandlerHydratesContextFromMinimalPayload(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)
	service.now = func() time.Time {
		return time.Date(2026, 4, 1, 10, 35, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.CurrentRevisionID = "rev_122"
		local.RevisionCache.StableRevisionID = "rev_122"
		return nil
	}); err != nil {
		t.Fatalf("seed stable/current revision: %v", err)
	}

	payload := samplePreparePayload(contracts.RuntimeModeStandalone)
	configureServiceHealthChecks(t, &payload, server, tcpListener)
	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	stableCtx, err := ContextFromPreparePayload(stablePayload)
	if err != nil {
		t.Fatalf("build stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), stableCtx); err != nil {
		t.Fatalf("prepare stable release workspace: %v", err)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	runPromotionReadySetup(t, service, raw)
	if result := service.handlePromoteRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPromoteRelease,
		RequestID:     "req_promote_for_minimal_rollback",
		CorrelationID: "corr_promote_for_minimal_rollback",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       raw,
	}); result.Error != nil {
		t.Fatalf("promote release failed: %#v", result.Error)
	}

	rollbackRaw, err := json.Marshal(rollbackReleasePayload{
		DeploymentID: "dep_test",
		RevisionID:   payload.Revision.RevisionID,
	})
	if err != nil {
		t.Fatalf("marshal rollback payload: %v", err)
	}

	result := service.handleRollbackRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRollbackRelease,
		RequestID:     "req_minimal_rollback",
		CorrelationID: "corr_minimal_rollback",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       rollbackRaw,
	})
	if result.Error != nil {
		t.Fatalf("expected rollback release to succeed from minimal payload, got %#v", result.Error)
	}

	local, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if local.RevisionCache.CurrentRevisionID != "rev_122" {
		t.Fatalf("expected current revision rev_122, got %q", local.RevisionCache.CurrentRevisionID)
	}
	if local.RevisionCache.StableRevisionID != "rev_122" {
		t.Fatalf("expected stable revision rev_122, got %q", local.RevisionCache.StableRevisionID)
	}
	if local.RevisionCache.LastRollbackFromRevision != payload.Revision.RevisionID {
		t.Fatalf("expected rollback from revision %q, got %q", payload.Revision.RevisionID, local.RevisionCache.LastRollbackFromRevision)
	}
}
