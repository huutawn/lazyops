package lazyyaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteFileCreatesLazyopsYAMLWhenMissing(t *testing.T) {
	repoRoot := t.TempDir()

	result, err := writeFileWithClock(repoRoot, []byte("project_slug: acme-shop\n"), false, fixedNow)
	if err != nil {
		t.Fatalf("writeFileWithClock() error = %v", err)
	}

	if result.Overwrote {
		t.Fatal("expected new file write, got overwrite result")
	}

	rendered, err := os.ReadFile(filepath.Join(repoRoot, "lazyops.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(rendered) != "project_slug: acme-shop\n" {
		t.Fatalf("expected written lazyops.yaml contents, got %q", string(rendered))
	}
}

func TestWriteFileRejectsOverwriteWithoutConfirmation(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "lazyops.yaml")
	if err := os.WriteFile(configPath, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() setup error = %v", err)
	}

	_, err := writeFileWithClock(repoRoot, []byte("new\n"), false, fixedNow)
	if err == nil {
		t.Fatal("expected overwrite confirmation error, got nil")
	}
	if !strings.Contains(err.Error(), "--overwrite") {
		t.Fatalf("expected overwrite guidance, got %v", err)
	}
}

func TestWriteFileCreatesBackupWhenOverwriting(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "lazyops.yaml")
	if err := os.WriteFile(configPath, []byte("old\n"), 0o640); err != nil {
		t.Fatalf("WriteFile() setup error = %v", err)
	}

	result, err := writeFileWithClock(repoRoot, []byte("new\n"), true, fixedNow)
	if err != nil {
		t.Fatalf("writeFileWithClock() error = %v", err)
	}

	if !result.Overwrote {
		t.Fatal("expected overwrite result, got non-overwrite")
	}
	if !strings.HasSuffix(result.BackupPath, "lazyops.yaml.bak.20260401-100203") {
		t.Fatalf("expected timestamped backup path, got %q", result.BackupPath)
	}

	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("ReadFile(backup) error = %v", err)
	}
	if string(backup) != "old\n" {
		t.Fatalf("expected backup to keep old contents, got %q", string(backup))
	}

	rendered, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(rendered) != "new\n" {
		t.Fatalf("expected overwritten lazyops.yaml contents, got %q", string(rendered))
	}
}

func fixedNow() time.Time {
	return time.Date(2026, time.April, 1, 10, 2, 3, 0, time.UTC)
}

func TestWriteFileSetsSecurePermissions(t *testing.T) {
	repoRoot := t.TempDir()

	_, err := writeFileWithClock(repoRoot, []byte("project_slug: acme-shop\n"), false, fixedNow)
	if err != nil {
		t.Fatalf("writeFileWithClock() error = %v", err)
	}

	info, err := os.Stat(filepath.Join(repoRoot, "lazyops.yaml"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o644 {
		t.Fatalf("expected file mode 0644, got %o", perm)
	}
}

func TestLazyopsYAMLNeverContainsSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"ssh_key", "project_slug: test\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod\nservices:\n  - name: api\n    path: apps/api\nssh_key: sk_live_abc123\n"},
		{"password", "project_slug: test\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod\nservices:\n  - name: api\n    path: apps/api\npassword: mysecret\n"},
		{"kubeconfig", "project_slug: test\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod\nservices:\n  - name: api\n    path: apps/api\nkubeconfig: |\n  apiVersion: v1\n"},
		{"private_key", "project_slug: test\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod\nservices:\n  - name: api\n    path: apps/api\nprivate_key: |\n  -----BEGIN RSA PRIVATE KEY-----\n"},
		{"github_pat", "project_slug: test\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod\nservices:\n  - name: api\n    path: apps/api\npat: ghp_abc123\n"},
		{"server_ip", "project_slug: test\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod\nservices:\n  - name: api\n    path: apps/api\nserver_ip: 203.0.113.10\n"},
		{"secret_ref", "project_slug: test\nruntime_mode: standalone\ndeployment_binding:\n  target_ref: prod\nservices:\n  - name: api\n    path: apps/api\nsecret: secret://clusters/cls_demo/kubeconfig\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			_, err := writeFileWithClock(repoRoot, []byte(tt.input), false, fixedNow)
			if err == nil {
				t.Fatalf("expected security rejection for %s, got nil", tt.name)
			}
			if !strings.Contains(err.Error(), "must not") && !strings.Contains(err.Error(), "must stay") && !strings.Contains(err.Error(), "forbidden") {
				t.Fatalf("expected security rejection message, got %v", err)
			}
		})
	}
}
