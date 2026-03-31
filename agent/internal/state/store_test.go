package state

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestStoreSaveAndLoadRoundTrip(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "state.json"))
	ctx := context.Background()

	now := time.Now().UTC().Round(time.Second)
	initial := &AgentLocalState{
		Metadata: AgentMetadata{
			AgentID:       "agt_local_test",
			Hostname:      "local-dev",
			AgentKind:     contracts.AgentKindInstance,
			RuntimeMode:   contracts.RuntimeModeStandalone,
			CurrentState:  contracts.AgentStateConnected,
			LastStartedAt: now,
		},
		Enrollment: EnrollmentState{
			SessionID:       "sess_123",
			LastBootstrapAt: now,
		},
		RevisionCache: RevisionCache{
			CurrentRevisionID: "rev_123",
			StableRevisionID:  "rev_122",
			UpdatedAt:         now,
		},
		CapabilitySnapshot: CapabilitySnapshotState{
			LastComputedAt: now,
			Payload: contracts.CapabilityReportPayload{
				AgentKind:   contracts.AgentKindInstance,
				RuntimeMode: contracts.RuntimeModeStandalone,
			},
		},
	}

	if err := store.Save(ctx, initial); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	if loaded.Metadata.AgentID != initial.Metadata.AgentID {
		t.Fatalf("expected agent ID %q, got %q", initial.Metadata.AgentID, loaded.Metadata.AgentID)
	}
	if loaded.RevisionCache.CurrentRevisionID != initial.RevisionCache.CurrentRevisionID {
		t.Fatalf("expected revision %q, got %q", initial.RevisionCache.CurrentRevisionID, loaded.RevisionCache.CurrentRevisionID)
	}
}
