package runtime

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureInternalPostgresAuthenticationFreshVolumeSkipsPasswordSync(t *testing.T) {
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	logPath := installFakeDocker(t)
	hostDataDir := filepath.Join(t.TempDir(), "pgdata")
	if err := os.MkdirAll(hostDataDir, 0o755); err != nil {
		t.Fatalf("mkdir host data dir: %v", err)
	}

	credentialState := internalPostgresCredentialState{
		Database: "app",
		Username: "lazyops_managed",
		Password: "fresh-secret",
	}
	if err := driver.ensureInternalPostgresAuthentication(context.Background(), "lazyops-postgres", hostDataDir, credentialState, true); err != nil {
		t.Fatalf("ensure auth for fresh volume: %v", err)
	}

	logOutput := readFakeDockerLog(t, logPath)
	if !strings.Contains(logOutput, "pg_isready -U lazyops_managed -d app -t 5") {
		t.Fatalf("expected readiness probe to use managed bootstrap role, got %q", logOutput)
	}
	if strings.Contains(logOutput, "ALTER ROLE") {
		t.Fatalf("expected fresh volume path to skip password sync, got %q", logOutput)
	}
	if strings.Contains(logOutput, "pg_reload_conf") {
		t.Fatalf("expected fresh volume path to skip auth reload, got %q", logOutput)
	}
	if _, err := os.Stat(internalPostgresSyncStatePath(hostDataDir)); err != nil {
		t.Fatalf("expected sync state marker to be written for fresh volume: %v", err)
	}
}

func TestEnsureInternalPostgresAuthenticationReusedVolumeUsesManagedBootstrapRole(t *testing.T) {
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	logPath := installFakeDocker(t)
	hostDataDir := filepath.Join(t.TempDir(), "pgdata")
	if err := os.MkdirAll(hostDataDir, 0o755); err != nil {
		t.Fatalf("mkdir host data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostDataDir, "pg_hba.conf"), []byte("host all all all trust\n"), 0o600); err != nil {
		t.Fatalf("write pg_hba.conf: %v", err)
	}

	credentialState := internalPostgresCredentialState{
		Database: "app",
		Username: "lazyops_managed",
		Password: "reused-secret",
	}
	if err := driver.ensureInternalPostgresAuthentication(context.Background(), "lazyops-postgres", hostDataDir, credentialState, false); err != nil {
		t.Fatalf("ensure auth for reused volume: %v", err)
	}

	logOutput := readFakeDockerLog(t, logPath)
	if !strings.Contains(logOutput, "pg_isready -U lazyops_managed -d app -t 5") {
		t.Fatalf("expected readiness probe to use managed bootstrap role, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "ALTER ROLE") {
		t.Fatalf("expected reused volume path to sync password when state is stale, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "SELECT pg_reload_conf();") {
		t.Fatalf("expected reused volume path to reload postgres auth config after rewrite, got %q", logOutput)
	}
	authConfig, err := os.ReadFile(filepath.Join(hostDataDir, "pg_hba.conf"))
	if err != nil {
		t.Fatalf("read pg_hba.conf: %v", err)
	}
	if !strings.Contains(string(authConfig), "password") {
		t.Fatalf("expected pg_hba.conf to be rewritten to password auth, got %q", string(authConfig))
	}
}

func TestEnsureInternalPostgresAuthenticationReusedVolumeSkipsSyncWhenStateMatches(t *testing.T) {
	driver := NewFilesystemDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	logPath := installFakeDocker(t)
	hostDataDir := filepath.Join(t.TempDir(), "pgdata")
	if err := os.MkdirAll(hostDataDir, 0o755); err != nil {
		t.Fatalf("mkdir host data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostDataDir, "pg_hba.conf"), []byte("host all all all password\n"), 0o600); err != nil {
		t.Fatalf("write pg_hba.conf: %v", err)
	}

	credentialState := internalPostgresCredentialState{
		Database: "app",
		Username: "lazyops_managed",
		Password: "stable-secret",
	}
	if err := persistInternalPostgresPasswordSyncState(hostDataDir, fingerprintInternalPostgresCredentialState(credentialState), time.Now().UTC()); err != nil {
		t.Fatalf("persist sync state: %v", err)
	}
	if err := driver.ensureInternalPostgresAuthentication(context.Background(), "lazyops-postgres", hostDataDir, credentialState, false); err != nil {
		t.Fatalf("ensure auth for already-synced reused volume: %v", err)
	}

	logOutput := readFakeDockerLog(t, logPath)
	if !strings.Contains(logOutput, "pg_isready -U lazyops_managed -d app -t 5") {
		t.Fatalf("expected readiness probe to use managed bootstrap role, got %q", logOutput)
	}
	if strings.Contains(logOutput, "ALTER ROLE") {
		t.Fatalf("expected sync marker to suppress password sync, got %q", logOutput)
	}
	if strings.Contains(logOutput, "pg_reload_conf") {
		t.Fatalf("expected unchanged auth config to skip reload, got %q", logOutput)
	}
}

func installFakeDocker(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "docker.log")
	scriptPath := filepath.Join(tmpDir, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$DOCKER_LOG\"\n" +
		"case \"$1\" in\n" +
		"  inspect)\n" +
		"    printf 'status=running exit=0 started=2026-04-13T00:00:00Z finished='\n" +
		"    ;;\n" +
		"  logs)\n" +
		"    printf 'postgres container log tail'\n" +
		"    ;;\n" +
		"esac\n" +
		"exit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker script: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+":"+origPath)
	t.Setenv("DOCKER_LOG", logPath)
	return logPath
}

func readFakeDockerLog(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	return string(payload)
}
