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

func TestResolveBuildPortUsesSmokeRunWhenImageDoesNotExposePort(t *testing.T) {
	resolution := resolveBuildPort(
		BuildTargetServiceMetadata{},
		[]string{"api"},
		nil,
		[]int{4321},
		"",
		nil,
		"",
		false,
	)

	if resolution.Status != buildPortResolutionStatusResolved {
		t.Fatalf("expected resolved status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceSmokeRun {
		t.Fatalf("expected smoke_run source, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 4321 {
		t.Fatalf("expected suggested target port 4321, got %#v", resolution)
	}
	if len(resolution.CandidatePorts) != 1 || resolution.CandidatePorts[0] != 4321 {
		t.Fatalf("expected candidate ports [4321], got %#v", resolution.CandidatePorts)
	}
}

func TestResolveBuildPortMarksAmbiguousWhenMultiplePortsCompete(t *testing.T) {
	resolution := resolveBuildPort(
		BuildTargetServiceMetadata{},
		[]string{"web"},
		[]BuildDetectedPortMetadata{{Port: 9229, Protocol: "tcp", Exposed: true}},
		[]int{3000},
		"next",
		&BuildSuggestedHealthcheckMetadata{Port: 3000, Path: "/"},
		"",
		false,
	)

	if resolution.Status != buildPortResolutionStatusAmbiguous {
		t.Fatalf("expected ambiguous status, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 0 {
		t.Fatalf("expected no suggested target port, got %#v", resolution)
	}
	if got := resolution.CandidatePorts; len(got) != 2 || got[0] != 3000 || got[1] != 9229 {
		t.Fatalf("expected candidate ports [3000 9229], got %#v", got)
	}
}

func TestResolveBuildPortMarksUnresolvedWhenNoCandidateExists(t *testing.T) {
	resolution := resolveBuildPort(
		BuildTargetServiceMetadata{},
		[]string{"api"},
		nil,
		nil,
		"",
		nil,
		"container exited before binding a port",
		false,
	)

	if resolution.Status != buildPortResolutionStatusUnresolved {
		t.Fatalf("expected unresolved status, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 0 {
		t.Fatalf("expected unresolved result to omit suggested port, got %#v", resolution)
	}
	if resolution.Reason == "" {
		t.Fatalf("expected unresolved reason to be preserved, got %#v", resolution)
	}
}

func TestResolveBuildPortRespectsExplicitDeclaredPort(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			DeclaredTargetPort: 7000,
			DeclaredHealthcheck: map[string]any{
				"path": "/healthz",
				"port": 7000,
			},
		},
		[]string{"go"},
		"",
		[]BuildDetectedPortMetadata{{Port: 3000, Protocol: "tcp", Exposed: true}},
		&BuildSuggestedHealthcheckMetadata{Port: 3000, Path: "/"},
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "PORT=3000 ./out",
		},
	)

	if resolution.Status != buildPortResolutionStatusResolved {
		t.Fatalf("expected resolved status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceExplicit {
		t.Fatalf("expected explicit source, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 7000 {
		t.Fatalf("expected explicit port 7000 to win, got %#v", resolution)
	}
	if resolution.SuggestedHealthcheck == nil || resolution.SuggestedHealthcheck.Port != 7000 {
		t.Fatalf("expected declared healthcheck to be preserved, got %#v", resolution.SuggestedHealthcheck)
	}
}

func TestResolveRepoServiceBuildPortUsesNixpacksPlan(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "api",
			ServicePath:    "backend",
			RuntimeProfile: "service",
			Public:         true,
		},
		[]string{"go"},
		"",
		[]BuildDetectedPortMetadata{{Port: 8080, Protocol: "tcp", Exposed: true}},
		nil,
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "PORT=3000 ./out",
		},
	)

	if resolution.Status != buildPortResolutionStatusResolved {
		t.Fatalf("expected resolved status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceNixpacksPlan {
		t.Fatalf("expected nixpacks_plan source, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 3000 {
		t.Fatalf("expected nixpacks port 3000, got %#v", resolution)
	}
	if got := resolution.CandidatePorts; len(got) != 1 || got[0] != 3000 {
		t.Fatalf("expected candidate ports [3000], got %#v", got)
	}
}

func TestResolveRepoServiceBuildPortRewritesSuggestedHealthcheckPortToNixpacksPort(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "web",
			ServicePath:    "fe",
			RuntimeProfile: "web",
			Public:         true,
		},
		[]string{"node"},
		"vite",
		[]BuildDetectedPortMetadata{{Port: 8080, Protocol: "tcp", Exposed: true}},
		&BuildSuggestedHealthcheckMetadata{Path: "/", Port: 3000},
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "node server.js --port 8080",
		},
	)

	if resolution.SuggestedHealthcheck == nil {
		t.Fatalf("expected suggested healthcheck to be preserved, got %#v", resolution)
	}
	if resolution.SuggestedHealthcheck.Path != "/" || resolution.SuggestedHealthcheck.Port != 8080 {
		t.Fatalf("expected suggested healthcheck /:8080, got %#v", resolution.SuggestedHealthcheck)
	}
}

func TestResolveRepoServiceBuildPortRejectsNixpacksPlanWithoutNumericPort(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "api",
			ServicePath:    "backend",
			RuntimeProfile: "service",
			Public:         true,
		},
		[]string{"unknown"},
		"",
		nil,
		nil,
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "PORT=$PORT ./out",
		},
	)

	if resolution.Status != buildPortResolutionStatusUnresolved {
		t.Fatalf("expected unresolved status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceNixpacksPlan {
		t.Fatalf("expected nixpacks_plan source when no language fallback exists, got %#v", resolution)
	}
	if resolution.Reason != "nixpacks start command contains no numeric port" {
		t.Fatalf("expected unresolved reason to preserve nixpacks detail, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 0 {
		t.Fatalf("expected unresolved Nixpacks result to omit suggested port, got %#v", resolution)
	}
}

func TestResolveRepoServiceBuildPortRejectsMissingNixpacksPlanPort(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "api",
			ServicePath:    "backend",
			RuntimeProfile: "service",
			Public:         true,
		},
		[]string{"unknown"},
		"",
		nil,
		nil,
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanError: "nixpacks plan did not define start.cmd",
		},
	)

	if resolution.Status != buildPortResolutionStatusUnresolved {
		t.Fatalf("expected unresolved status, got %#v", resolution)
	}
	if resolution.Source != "" {
		t.Fatalf("expected empty source when plan has no usable start command and no language fallback exists, got %#v", resolution)
	}
	if resolution.Reason != "nixpacks plan did not expose a single numeric start port; nixpacks plan did not define start.cmd" {
		t.Fatalf("expected nixpacks-plan-specific unresolved reason, got %#v", resolution)
	}
}

func TestResolveRepoServiceBuildPortRejectsNixpacksPlanWithMultiplePorts(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "api",
			ServicePath:    "backend",
			RuntimeProfile: "service",
			Public:         true,
		},
		[]string{"go"},
		"",
		nil,
		nil,
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "PORT=3000 ./out --port 9229",
		},
	)

	if resolution.Status != buildPortResolutionStatusAmbiguous {
		t.Fatalf("expected ambiguous status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceNixpacksPlan {
		t.Fatalf("expected nixpacks_plan source, got %#v", resolution)
	}
	if resolution.Reason != "nixpacks start command contains multiple candidate ports: 3000, 9229" {
		t.Fatalf("expected Nixpacks-specific ambiguous reason, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 0 {
		t.Fatalf("expected ambiguous Nixpacks result to omit suggested port, got %#v", resolution)
	}
}

func TestResolveRepoServiceBuildPortFallsBackToDefaultGoPort(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "api",
			ServicePath:    "backend",
			RuntimeProfile: "service",
			Public:         true,
		},
		[]string{"go"},
		"",
		nil,
		nil,
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "./out",
		},
	)

	if resolution.Status != buildPortResolutionStatusResolved {
		t.Fatalf("expected resolved status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceLanguageDefault {
		t.Fatalf("expected language_default source, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 8080 {
		t.Fatalf("expected go fallback port 8080, got %#v", resolution)
	}
	if got := resolution.CandidatePorts; len(got) != 1 || got[0] != 8080 {
		t.Fatalf("expected candidate ports [8080], got %#v", got)
	}
	if resolution.Reason != "nixpacks start command contains no numeric port; falling back to default go port 8080" {
		t.Fatalf("expected fallback reason to include nixpacks failure and go default, got %#v", resolution.Reason)
	}
}

func TestResolveRepoServiceBuildPortFallsBackToFrameworkDefaultPort(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "web",
			ServicePath:    "fe",
			RuntimeProfile: "web",
			Public:         true,
		},
		[]string{"node"},
		"next",
		nil,
		&BuildSuggestedHealthcheckMetadata{Path: "/", Port: 3000},
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "npm run start",
		},
	)

	if resolution.Status != buildPortResolutionStatusResolved {
		t.Fatalf("expected resolved status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceLanguageDefault {
		t.Fatalf("expected language_default source, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 3000 {
		t.Fatalf("expected frontend fallback port 3000, got %#v", resolution)
	}
	if resolution.SuggestedHealthcheck == nil || resolution.SuggestedHealthcheck.Path != "/" || resolution.SuggestedHealthcheck.Port != 3000 {
		t.Fatalf("expected frontend healthcheck /:3000, got %#v", resolution.SuggestedHealthcheck)
	}
}

func TestResolveRepoServiceBuildPortDoesNotFallbackWhenNixpacksPlanIsAmbiguous(t *testing.T) {
	resolution := resolveRepoServiceBuildPort(
		BuildTargetServiceMetadata{
			ServiceName:    "api",
			ServicePath:    "backend",
			RuntimeProfile: "service",
			Public:         true,
		},
		[]string{"go"},
		"",
		nil,
		nil,
		buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: "PORT=3000 ./out --port 9229",
		},
	)

	if resolution.Status != buildPortResolutionStatusAmbiguous {
		t.Fatalf("expected ambiguous status, got %#v", resolution)
	}
	if resolution.Source != buildPortResolutionSourceNixpacksPlan {
		t.Fatalf("expected nixpacks_plan source, got %#v", resolution)
	}
	if resolution.SuggestedTargetPort != 0 {
		t.Fatalf("expected ambiguous result to omit suggested port, got %#v", resolution)
	}
}

func TestExtractJSONDocumentHandlesPrefixedNixpacksOutput(t *testing.T) {
	payload := []byte("Final Nixpacks plan: {\n  \"start\": {\"cmd\": \"node server.js --port 3000\"}\n}\n")

	document, err := extractJSONDocument(payload)
	if err != nil {
		t.Fatalf("expected JSON extraction to succeed, got %v", err)
	}
	if string(document) != "{\n  \"start\": {\"cmd\": \"node server.js --port 3000\"}\n}" {
		t.Fatalf("unexpected extracted JSON document: %q", string(document))
	}
}

func TestParseProcNetListeningPorts(t *testing.T) {
	payload := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 00000000:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000        0 1 0000000000000000 100 0 0 10 0\n" +
		"   1: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000        0 1 0000000000000000 100 0 0 10 0\n"

	ports := parseProcNetListeningPorts(payload)
	if len(ports) != 2 || ports[0] != 3000 || ports[1] != 8080 {
		t.Fatalf("expected listening ports [3000 8080], got %#v", ports)
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
