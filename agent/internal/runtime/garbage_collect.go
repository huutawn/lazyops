package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (d *FilesystemDriver) GarbageCollectRuntime(_ context.Context, runtimeCtx RuntimeContext) (GarbageCollectRuntimeResult, error) {
	bindingRoot := filepath.Join(
		d.root,
		"projects",
		runtimeCtx.Project.ProjectID,
		"bindings",
		runtimeCtx.Binding.BindingID,
	)
	gcRoot := filepath.Join(bindingRoot, "runtime-gc")
	if err := os.MkdirAll(gcRoot, 0o755); err != nil {
		return GarbageCollectRuntimeResult{}, err
	}

	protectedRevisions := protectedRevisionIDs(runtimeCtx)
	removedRevisions, removedGatewayVersions, removedSidecarVersions, err := removeUnprotectedRevisionRoots(filepath.Join(bindingRoot, "revisions"), toSet(protectedRevisions))
	if err != nil {
		return GarbageCollectRuntimeResult{}, err
	}

	collectedAt := d.now()
	result := GarbageCollectRuntimeResult{
		ProjectID:              runtimeCtx.Project.ProjectID,
		BindingID:              runtimeCtx.Binding.BindingID,
		ProtectedRevisionIDs:   protectedRevisions,
		RemovedRevisionRoots:   removedRevisions,
		RemovedGatewayVersions: removedGatewayVersions,
		RemovedSidecarVersions: removedSidecarVersions,
		Summary: fmt.Sprintf(
			"runtime garbage collection removed %d revision roots, %d gateway versions, and %d sidecar versions",
			len(removedRevisions),
			len(removedGatewayVersions),
			len(removedSidecarVersions),
		),
		ReportPath:  filepath.Join(gcRoot, "last-run.json"),
		CollectedAt: collectedAt,
	}
	if err := writeJSON(result.ReportPath, result); err != nil {
		return GarbageCollectRuntimeResult{}, err
	}

	if d.logger != nil {
		d.logger.Info("runtime garbage collection removed stale runtime state",
			"binding_id", runtimeCtx.Binding.BindingID,
			"protected_revisions", len(protectedRevisions),
			"removed_revisions", len(removedRevisions),
			"removed_gateway_versions", len(removedGatewayVersions),
			"removed_sidecar_versions", len(removedSidecarVersions),
		)
	}

	return result, nil
}

func protectedRevisionIDs(runtimeCtx RuntimeContext) []string {
	unique := make(map[string]struct{})
	for _, revisionID := range []string{
		runtimeCtx.Revision.RevisionID,
		runtimeCtx.Rollout.CurrentRevisionID,
		runtimeCtx.Rollout.StableRevisionID,
		runtimeCtx.Rollout.PreviousStableRevisionID,
		runtimeCtx.Rollout.PendingRevisionID,
		runtimeCtx.Rollout.CandidateRevisionID,
		runtimeCtx.Rollout.DrainingRevisionID,
	} {
		if strings.TrimSpace(revisionID) == "" {
			continue
		}
		unique[revisionID] = struct{}{}
	}
	protected := make([]string, 0, len(unique))
	for revisionID := range unique {
		protected = append(protected, revisionID)
	}
	sort.Strings(protected)
	return protected
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		set[value] = struct{}{}
	}
	return set
}

func removeUnprotectedRevisionRoots(root string, protected map[string]struct{}) ([]string, []string, []string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, err
	}

	removedRevisions := make([]string, 0)
	removedGatewayVersions := make([]string, 0)
	removedSidecarVersions := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, keep := protected[name]; keep {
			continue
		}

		revisionRoot := filepath.Join(root, name)
		removedGatewayVersions = append(removedGatewayVersions, collectNestedVersionDirs(filepath.Join(revisionRoot, "gateway", "versions"))...)
		removedSidecarVersions = append(removedSidecarVersions, collectNestedVersionDirs(filepath.Join(revisionRoot, "sidecars", "versions"))...)
		if err := os.RemoveAll(revisionRoot); err != nil {
			return nil, nil, nil, err
		}
		removedRevisions = append(removedRevisions, revisionRoot)
	}

	sort.Strings(removedRevisions)
	sort.Strings(removedGatewayVersions)
	sort.Strings(removedSidecarVersions)
	return removedRevisions, removedGatewayVersions, removedSidecarVersions, nil
}

func collectNestedVersionDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	removed := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		removed = append(removed, filepath.Join(root, entry.Name()))
	}
	return removed
}
