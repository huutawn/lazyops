package buildworker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFrontendMetadataNext(t *testing.T) {
	worker := &Worker{}
	repoDir := t.TempDir()

	writePackageJSON(t, repoDir, `{"dependencies":{"next":"14.0.0"}}`)
	framework, suggested := worker.detectFrontendMetadata(repoDir)

	if framework != "next" {
		t.Fatalf("expected framework next, got %q", framework)
	}
	if suggested == nil || suggested.Path != "/" || suggested.Port != 3000 {
		t.Fatalf("expected suggested healthcheck /:3000, got %#v", suggested)
	}
}

func TestDetectFrontendMetadataViteAndReactScripts(t *testing.T) {
	worker := &Worker{}

	viteDir := t.TempDir()
	writePackageJSON(t, viteDir, `{"devDependencies":{"vite":"5.0.0"}}`)
	framework, suggested := worker.detectFrontendMetadata(viteDir)
	if framework != "vite" {
		t.Fatalf("expected framework vite, got %q", framework)
	}
	if suggested == nil || suggested.Path != "/" || suggested.Port != 3000 {
		t.Fatalf("expected suggested healthcheck /:3000 for vite, got %#v", suggested)
	}

	reactScriptsDir := t.TempDir()
	writePackageJSON(t, reactScriptsDir, `{"dependencies":{"react-scripts":"5.0.1"}}`)
	framework, suggested = worker.detectFrontendMetadata(reactScriptsDir)
	if framework != "react-scripts" {
		t.Fatalf("expected framework react-scripts, got %q", framework)
	}
	if suggested == nil || suggested.Path != "/" || suggested.Port != 3000 {
		t.Fatalf("expected suggested healthcheck /:3000 for react-scripts, got %#v", suggested)
	}
}

func TestDetectFrontendMetadataOmitWhenUnknown(t *testing.T) {
	worker := &Worker{}
	repoDir := t.TempDir()

	writePackageJSON(t, repoDir, `{"dependencies":{"express":"4.0.0"}}`)
	framework, suggested := worker.detectFrontendMetadata(repoDir)
	if framework != "" {
		t.Fatalf("expected empty framework, got %q", framework)
	}
	if suggested != nil {
		t.Fatalf("expected nil suggestion, got %#v", suggested)
	}
}

func writePackageJSON(t *testing.T, repoDir, content string) {
	t.Helper()
	path := filepath.Join(repoDir, "package.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}
