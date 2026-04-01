package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func candidateManifestPath(layout WorkspaceLayout) string {
	return filepath.Join(layout.Root, "candidate.json")
}

func healthGateReportPath(layout WorkspaceLayout) string {
	return filepath.Join(layout.Root, "health-gate.json")
}

func rolloutSummaryPath(layout WorkspaceLayout) string {
	return filepath.Join(layout.Root, "rollout-summary.json")
}

func loadWorkspaceManifest(layout WorkspaceLayout) (WorkspaceManifest, error) {
	payload, err := os.ReadFile(filepath.Join(layout.Root, "workspace.json"))
	if err != nil {
		return WorkspaceManifest{}, err
	}

	var manifest WorkspaceManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return WorkspaceManifest{}, err
	}
	return manifest, nil
}

func loadCandidateRecord(layout WorkspaceLayout) (CandidateRecord, error) {
	payload, err := os.ReadFile(candidateManifestPath(layout))
	if err != nil {
		return CandidateRecord{}, err
	}

	var record CandidateRecord
	if err := json.Unmarshal(payload, &record); err != nil {
		return CandidateRecord{}, err
	}
	if strings.TrimSpace(record.ManifestPath) == "" {
		record.ManifestPath = candidateManifestPath(layout)
	}
	if strings.TrimSpace(record.WorkspaceRoot) == "" {
		record.WorkspaceRoot = layout.Root
	}
	return record, nil
}

func saveCandidateRecord(record CandidateRecord) error {
	if strings.TrimSpace(record.ManifestPath) == "" {
		return fmt.Errorf("candidate manifest path is required")
	}
	return writeJSON(record.ManifestPath, record)
}

func seedCandidateFromWorkspace(layout WorkspaceLayout, manifest WorkspaceManifest, startedAt time.Time) (CandidateRecord, error) {
	preparedAt := manifest.PreparedAt
	if preparedAt.IsZero() {
		preparedAt = startedAt
	}

	record := CandidateRecord{
		RevisionID:       manifest.Revision.RevisionID,
		WorkspaceRoot:    layout.Root,
		State:            CandidateStatePrepared,
		StartedAt:        startedAt,
		ManifestPath:     candidateManifestPath(layout),
		LastTransitionAt: preparedAt,
		History: []CandidateTransition{
			{
				To:         CandidateStatePrepared,
				Reason:     "release workspace prepared",
				OccurredAt: preparedAt,
			},
		},
	}
	if err := transitionCandidateState(&record, CandidateStateStarting, "candidate workload starting", startedAt); err != nil {
		return CandidateRecord{}, err
	}
	return record, nil
}

func transitionCandidateState(record *CandidateRecord, next CandidateState, reason string, at time.Time) error {
	if record == nil {
		return fmt.Errorf("candidate record is required")
	}
	if record.State == next {
		if record.LastTransitionAt.IsZero() {
			record.LastTransitionAt = at
		}
		return nil
	}
	if !isAllowedCandidateTransition(record.State, next) {
		return fmt.Errorf("invalid candidate state transition from %q to %q", record.State, next)
	}

	record.History = append(record.History, CandidateTransition{
		From:       record.State,
		To:         next,
		Reason:     reason,
		OccurredAt: at,
	})
	record.State = next
	record.LastTransitionAt = at
	if next == CandidateStateStarting && record.StartedAt.IsZero() {
		record.StartedAt = at
	}
	return nil
}

func isAllowedCandidateTransition(from, to CandidateState) bool {
	switch from {
	case "":
		return to == CandidateStatePrepared
	case CandidateStatePrepared:
		return to == CandidateStateStarting
	case CandidateStateStarting:
		return to == CandidateStateHealthy || to == CandidateStateUnhealthy || to == CandidateStateFailed
	case CandidateStateHealthy:
		return to == CandidateStatePromotable || to == CandidateStateUnhealthy
	case CandidateStateUnhealthy:
		return to == CandidateStateHealthy || to == CandidateStateFailed
	case CandidateStatePromotable:
		return to == CandidateStateUnhealthy || to == CandidateStateFailed
	default:
		return false
	}
}
