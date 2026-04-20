package buildworker

import (
	"os"
	"path/filepath"
	"testing"

	"lazyops-server/internal/config"
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

func TestSelectSuggestedTargetPortPrefersInternalServiceDefaults(t *testing.T) {
	port, confidence := selectSuggestedTargetPort(
		[]string{"postgres"},
		[]BuildDetectedPortMetadata{{Port: 5432, Protocol: "tcp"}},
		"",
		nil,
	)

	if port != 5432 || confidence != "high" {
		t.Fatalf("expected postgres default port with high confidence, got port=%d confidence=%q", port, confidence)
	}
}

func TestSelectSuggestedTargetPortFallsBackToFrameworkHint(t *testing.T) {
	port, confidence := selectSuggestedTargetPort(
		[]string{"web"},
		[]BuildDetectedPortMetadata{{Port: 3000, Protocol: "tcp"}, {Port: 9229, Protocol: "tcp"}},
		"next",
		nil,
	)

	if port != 3000 || confidence != "medium" {
		t.Fatalf("expected next.js hint port 3000 with medium confidence, got port=%d confidence=%q", port, confidence)
	}
}

func TestSelectSuggestedTargetPortChoosesSuggestedHealthcheckEvenWhenNotExposed(t *testing.T) {
	port, confidence := selectSuggestedTargetPort(
		[]string{"api"},
		[]BuildDetectedPortMetadata{{Port: 8080, Protocol: "tcp"}},
		"",
		&BuildSuggestedHealthcheckMetadata{Port: 3000, Path: "/"},
	)

	if port != 3000 || confidence != "medium" {
		t.Fatalf("expected healthcheck port 3000 with medium confidence, got port=%d confidence=%q", port, confidence)
	}
}

func TestImageNameForServiceUsesDistinctRepositoryPerService(t *testing.T) {
	worker := &Worker{
		cfg: config.Config{
			BuildWorker: config.BuildWorkerConfig{
				RegistryHost: "docker.io",
			},
		},
	}
	input := BuildWorkerInput{
		ProjectID: "prj_demo",
		RepoOwner: "tawn",
		CommitSHA: "6918dcb1117a0fad1b343f4e370ec65723de1ad4",
	}

	beImage := worker.imageNameForService(input, "be")
	feImage := worker.imageNameForService(input, "fe")

	if beImage == feImage {
		t.Fatalf("expected distinct image refs per service, got be=%q fe=%q", beImage, feImage)
	}
	if beImage != "docker.io/tawn/prj_demo-be:6918dcb1117a" {
		t.Fatalf("expected backend image ref to include service suffix, got %q", beImage)
	}
	if feImage != "docker.io/tawn/prj_demo-fe:6918dcb1117a" {
		t.Fatalf("expected frontend image ref to include service suffix, got %q", feImage)
	}
}

func writePackageJSON(t *testing.T, repoDir, content string) {
	t.Helper()
	path := filepath.Join(repoDir, "package.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}
