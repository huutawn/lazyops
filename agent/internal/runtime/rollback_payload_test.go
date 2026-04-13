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

func TestRollbackReleaseHandlerRestoresCurrentStableWhenCandidateFailsBeforePromotion(t *testing.T) {
	store := state.New(filepath.Join(t.TempDir(), "agent-state.json"))
	root := filepath.Join(t.TempDir(), "runtime-root")
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), store, driver)
	service.now = func() time.Time {
		return time.Date(2026, 4, 1, 11, 5, 0, 0, time.UTC)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tcpListener, stopTCP := startTCPHealthListener(t)
	defer stopTCP()

	previousStablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	previousStablePayload.Revision.RevisionID = "rev_001"
	previousStablePayload.Revision.CommitSHA = "001"
	previousStablePayload.Revision.ArtifactRef = "artifact://lazy-app/rev_001.tar.gz"
	previousStablePayload.Revision.ImageRef = "ghcr.io/lazyops/lazy-app:rev_001"
	configureServiceHealthChecks(t, &previousStablePayload, server, tcpListener)
	previousStableCtx, err := ContextFromPreparePayload(previousStablePayload)
	if err != nil {
		t.Fatalf("build previous stable runtime context: %v", err)
	}
	if _, err := driver.PrepareReleaseWorkspace(context.Background(), previousStableCtx); err != nil {
		t.Fatalf("prepare previous stable release workspace: %v", err)
	}

	if _, err := store.Update(context.Background(), func(local *state.AgentLocalState) error {
		local.RevisionCache.CurrentRevisionID = "rev_001"
		local.RevisionCache.StableRevisionID = "rev_001"
		return nil
	}); err != nil {
		t.Fatalf("seed previous stable revision: %v", err)
	}

	stablePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	stablePayload.Revision.RevisionID = "rev_122"
	stablePayload.Revision.CommitSHA = "122"
	stablePayload.Revision.ArtifactRef = "artifact://lazy-app/rev_122.tar.gz"
	stablePayload.Revision.ImageRef = "ghcr.io/lazyops/lazy-app:rev_122"
	configureServiceHealthChecks(t, &stablePayload, server, tcpListener)
	stableRaw, err := json.Marshal(stablePayload)
	if err != nil {
		t.Fatalf("marshal stable payload: %v", err)
	}

	runPromotionReadySetup(t, service, stableRaw)
	if result := service.handlePromoteRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandPromoteRelease,
		RequestID:     "req_promote_current_stable",
		CorrelationID: "corr_promote_current_stable",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload:       stableRaw,
	}); result.Error != nil {
		t.Fatalf("promote current stable failed: %#v", result.Error)
	}

	candidatePayload := samplePreparePayload(contracts.RuntimeModeStandalone)
	candidatePayload.Revision.RevisionID = "rev_999"
	candidatePayload.Revision.CommitSHA = "999"
	candidatePayload.Revision.ArtifactRef = "artifact://lazy-app/rev_999.tar.gz"
	candidatePayload.Revision.ImageRef = "ghcr.io/lazyops/lazy-app:rev_999"
	configureServiceHealthChecks(t, &candidatePayload, server, tcpListener)
	candidateRaw, err := json.Marshal(candidatePayload)
	if err != nil {
		t.Fatalf("marshal candidate payload: %v", err)
	}

	runRuntimeSetup(t, service, candidateRaw)

	result := service.handleRollbackRelease(context.Background(), contracts.CommandEnvelope{
		Type:          contracts.CommandRollbackRelease,
		RequestID:     "req_candidate_failed_rollback",
		CorrelationID: "corr_candidate_failed_rollback",
		AgentID:       "agt_local",
		Source:        contracts.EnvelopeSourceBackend,
		OccurredAt:    time.Now().UTC(),
		Payload: mustMarshalRollbackPayload(t, rollbackReleasePayload{
			DeploymentID: "dep_candidate_failed",
			RevisionID:   candidatePayload.Revision.RevisionID,
		}),
	})
	if result.Error != nil {
		t.Fatalf("expected candidate rollback to succeed, got %#v", result.Error)
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
	if local.RevisionCache.LastRollbackFromRevision != "rev_999" {
		t.Fatalf("expected rollback from candidate revision rev_999, got %q", local.RevisionCache.LastRollbackFromRevision)
	}
	if local.RevisionCache.LastRollbackToRevision != "rev_122" {
		t.Fatalf("expected rollback to current stable revision rev_122, got %q", local.RevisionCache.LastRollbackToRevision)
	}
}

func mustMarshalRollbackPayload(t *testing.T, payload rollbackReleasePayload) []byte {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal rollback payload: %v", err)
	}
	return raw
}
