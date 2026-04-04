package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsOriginAndCurrentBranch(t *testing.T) {
	repoRoot := t.TempDir()
	writeTestFile(t, filepath.Join(repoRoot, ".git", "config"), "[remote \"origin\"]\n\turl = git@github.com:lazyops/acme-shop.git\n")
	writeTestFile(t, filepath.Join(repoRoot, ".git", "HEAD"), "ref: refs/heads/main\n")

	metadata, err := Load(repoRoot)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if metadata.RepoOwner != "lazyops" || metadata.RepoName != "acme-shop" {
		t.Fatalf("expected repo owner/name, got %+v", metadata)
	}
	if metadata.CurrentBranch != "main" {
		t.Fatalf("expected current branch main, got %q", metadata.CurrentBranch)
	}
}

func TestParseRepoSlugRejectsInvalidInput(t *testing.T) {
	_, _, err := ParseRepoSlug("acme-shop")
	if err == nil {
		t.Fatal("expected invalid repo slug error, got nil")
	}
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
