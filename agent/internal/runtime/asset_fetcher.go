package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LocalCacheFetcher struct {
	root string
	now  func() time.Time
}

func NewLocalCacheFetcher(root string) *LocalCacheFetcher {
	return &LocalCacheFetcher{
		root: root,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (f *LocalCacheFetcher) FetchRevisionAssets(_ context.Context, runtimeCtx RuntimeContext, layout WorkspaceLayout) (ArtifactMaterialization, error) {
	cacheKey := assetCacheKey(runtimeCtx)
	cachePath := filepath.Join(f.root, cacheKey)
	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		return ArtifactMaterialization{}, err
	}

	workspacePath := filepath.Join(layout.Artifacts, "asset.json")
	materialization := ArtifactMaterialization{
		Status:        "cached",
		ArtifactRef:   runtimeCtx.Revision.ArtifactRef,
		ImageRef:      runtimeCtx.Revision.ImageRef,
		CacheKey:      cacheKey,
		CachePath:     cachePath,
		WorkspacePath: workspacePath,
		ResolvedAt:    f.now(),
	}
	if err := writeJSON(filepath.Join(cachePath, "cache-manifest.json"), materialization); err != nil {
		return ArtifactMaterialization{}, err
	}
	if err := writeJSON(workspacePath, materialization); err != nil {
		return ArtifactMaterialization{}, err
	}
	return materialization, nil
}

func assetCacheKey(runtimeCtx RuntimeContext) string {
	base := fmt.Sprintf("%s|%s|%s|%s|%s",
		runtimeCtx.Project.ProjectID,
		runtimeCtx.Binding.BindingID,
		runtimeCtx.Revision.RevisionID,
		runtimeCtx.Revision.ArtifactRef,
		runtimeCtx.Revision.ImageRef,
	)
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:8])
}
