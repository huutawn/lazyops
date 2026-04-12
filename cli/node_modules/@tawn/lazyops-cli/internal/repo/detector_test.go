package repo

import (
	"strings"
	"testing"
)

func TestDetectServicesInfersGoStartAndHealthHints(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, repoRoot+"/.git")
	mustWriteFile(t, repoRoot+"/backend/go.mod", "module backend\n\ngo 1.22.2\n")
	mustWriteFile(t, repoRoot+"/backend/cmd/server/main.go", "package main\nfunc main() {}\n")
	mustWriteFile(t, repoRoot+"/backend/internal/api/routes.go", "package api\nfunc routes(){ GET(\"/health\", nil) }\n")
	mustWriteFile(t, repoRoot+"/backend/internal/config/config.go", "package config\nconst _ = `SERVER_PORT\", \"8080\"`\n")

	scanResult, err := Scan(repoRoot)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	detectionResult, err := DetectServices(scanResult)
	if err != nil {
		t.Fatalf("DetectServices() error = %v", err)
	}

	if len(detectionResult.Candidates) != 1 {
		t.Fatalf("expected one candidate, got %d", len(detectionResult.Candidates))
	}

	candidate := detectionResult.Candidates[0]
	if candidate.StartHint != "go run ./cmd/server" {
		t.Fatalf("expected Go start hint, got %q", candidate.StartHint)
	}
	if candidate.Healthcheck.Path != "/health" {
		t.Fatalf("expected /health hint, got %q", candidate.Healthcheck.Path)
	}
	if candidate.Healthcheck.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", candidate.Healthcheck.Port)
	}
}

func TestDetectServicesInfersNodeHintsWithoutClassification(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, repoRoot+"/.git")
	mustWriteFile(t, repoRoot+"/apps/web/package.json", `{"name":"web","scripts":{"start":"next start","dev":"next dev"}}`)
	mustWriteFile(t, repoRoot+"/apps/web/Dockerfile", "FROM node:20-alpine\nEXPOSE 3000\n")
	mustWriteFile(t, repoRoot+"/apps/web/src/server.ts", `app.get("/healthz", () => "ok")`)

	scanResult, err := Scan(repoRoot)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	detectionResult, err := DetectServices(scanResult)
	if err != nil {
		t.Fatalf("DetectServices() error = %v", err)
	}

	candidate := detectionResult.Candidates[0]
	if candidate.StartHint != "npm run start" {
		t.Fatalf("expected Node start hint, got %q", candidate.StartHint)
	}
	if candidate.Healthcheck.Path != "/healthz" {
		t.Fatalf("expected /healthz hint, got %q", candidate.Healthcheck.Path)
	}
	if candidate.Healthcheck.Port != 3000 {
		t.Fatalf("expected port 3000, got %d", candidate.Healthcheck.Port)
	}
	for _, warning := range candidate.Warnings {
		lower := strings.ToLower(warning)
		if strings.Contains(lower, "frontend") || strings.Contains(lower, "backend") {
			t.Fatalf("warning must stay service-only, got %q", warning)
		}
	}
}

func TestDetectServicesWarnsWhenDetectionIsAmbiguous(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, repoRoot+"/.git")
	mustWriteFile(t, repoRoot+"/workers/jobs/requirements.txt", "fastapi==0.110.0\n")
	mustWriteFile(t, repoRoot+"/workers/jobs/app.py", "print('hi')\n")

	scanResult, err := Scan(repoRoot)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	detectionResult, err := DetectServices(scanResult)
	if err != nil {
		t.Fatalf("DetectServices() error = %v", err)
	}

	candidate := detectionResult.Candidates[0]
	if candidate.StartHint != "python app.py" {
		t.Fatalf("expected Python start hint, got %q", candidate.StartHint)
	}
	if candidate.Healthcheck.Path != "" {
		t.Fatalf("expected no healthcheck hint, got %+v", candidate.Healthcheck)
	}
	if len(candidate.Warnings) == 0 {
		t.Fatal("expected ambiguity warnings when healthcheck cannot be inferred")
	}
	if !containsWarning(candidate.Warnings, "no health hint inferred yet") {
		t.Fatalf("expected no-health warning, got %v", candidate.Warnings)
	}
}

func TestDetectServicesRejectsDuplicateServiceNames(t *testing.T) {
	repoRoot := t.TempDir()
	mustMkdir(t, repoRoot+"/.git")
	mustWriteFile(t, repoRoot+"/apps/api/go.mod", "module api\n\ngo 1.22.2\n")
	mustWriteFile(t, repoRoot+"/services/api/package.json", `{"name":"api","scripts":{"start":"node server.js"}}`)

	scanResult, err := Scan(repoRoot)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	_, err = DetectServices(scanResult)
	if err == nil {
		t.Fatal("expected duplicate service name error, got nil")
	}
	if !strings.Contains(err.Error(), `duplicate service name "api"`) {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
}

func containsWarning(warnings []string, expected string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, expected) {
			return true
		}
	}
	return false
}
