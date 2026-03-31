package repo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestScanDetectsRepoRootFromNestedPath(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, ".git"))
	mustWriteFile(t, filepath.Join(repoRoot, "go.mod"), "module lazyops-cli\n\ngo 1.22.2\n")
	mustMkdir(t, filepath.Join(repoRoot, "apps", "api"))

	result, err := Scan(filepath.Join(repoRoot, "apps", "api"))
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.RepoRoot != repoRoot {
		t.Fatalf("expected repo root %q, got %q", repoRoot, result.RepoRoot)
	}
	if result.Monorepo {
		t.Fatal("expected root-only service scan to be single-service")
	}
	if len(result.Services) != 1 {
		t.Fatalf("expected one detected service, got %d", len(result.Services))
	}
	if result.Services[0].Path != "." {
		t.Fatalf("expected root service path '.', got %q", result.Services[0].Path)
	}
	if !slices.Equal(result.Services[0].SignalNames(), []string{"go.mod"}) {
		t.Fatalf("expected go.mod signal, got %v", result.Services[0].SignalNames())
	}
}

func TestScanDetectsMonorepoServiceMarkers(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, filepath.Join(repoRoot, ".git"))
	mustWriteFile(t, filepath.Join(repoRoot, "apps", "web", "package.json"), `{"name":"web"}`)
	mustWriteFile(t, filepath.Join(repoRoot, "apps", "web", "Dockerfile"), "FROM node:20-alpine\n")
	mustWriteFile(t, filepath.Join(repoRoot, "apps", "api", "go.mod"), "module api\n\ngo 1.22.2\n")
	mustWriteFile(t, filepath.Join(repoRoot, "workers", "jobs", "requirements.txt"), "fastapi==0.110.0\n")
	mustWriteFile(t, filepath.Join(repoRoot, "vendor", "ignored", "go.mod"), "module ignored\n")

	result, err := Scan(repoRoot)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if !result.Monorepo {
		t.Fatal("expected monorepo layout when multiple services are detected")
	}
	if len(result.Services) != 3 {
		t.Fatalf("expected three detected services, got %d", len(result.Services))
	}

	assertService(t, result, "api", "apps/api", []string{"go.mod"})
	assertService(t, result, "web", "apps/web", []string{"Dockerfile", "package.json"})
	assertService(t, result, "jobs", "workers/jobs", []string{"requirements.txt"})
}

func TestFindRepoRootReturnsErrorWhenGitMetadataMissing(t *testing.T) {
	_, err := FindRepoRoot(t.TempDir())
	if err == nil {
		t.Fatal("expected missing repo root error, got nil")
	}
	if !errors.Is(err, ErrRepoRootNotFound) {
		t.Fatalf("expected ErrRepoRootNotFound, got %v", err)
	}
}

func assertService(t *testing.T, result RepoScanResult, expectedName string, expectedPath string, expectedSignals []string) {
	t.Helper()

	for _, service := range result.Services {
		if service.Name != expectedName || service.Path != expectedPath {
			continue
		}

		signals := service.SignalNames()
		if !slices.Equal(signals, expectedSignals) {
			t.Fatalf("expected signals %v for service %s, got %v", expectedSignals, expectedName, signals)
		}
		return
	}

	t.Fatalf("service %s at %s was not found in scan result", expectedName, expectedPath)
}

func mustWriteFile(t *testing.T, path string, contents string) {
	t.Helper()

	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}
