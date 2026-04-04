package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func testK3sDetector() *K3sEnvironmentDetector {
	d := NewK3sEnvironmentDetector(nil)
	d.now = func() time.Time {
		return time.Date(2026, 4, 4, 14, 0, 0, 0, time.UTC)
	}
	return d
}

func TestK3sEnvironmentDetectorDefaultMode(t *testing.T) {
	d := testK3sDetector()
	if d.Mode() != contracts.RuntimeModeStandalone {
		t.Fatalf("expected default mode standalone, got %q", d.Mode())
	}
}

func TestK3sEnvironmentDetectorNotDetected(t *testing.T) {
	d := testK3sDetector()
	err := d.Detect()
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if d.IsK3s() {
		t.Fatal("expected not detected in clean environment")
	}
	if d.Mode() != contracts.RuntimeModeStandalone {
		t.Fatalf("expected mode standalone, got %q", d.Mode())
	}
}

func TestK3sEnvironmentDetectorDetectedViaK3sBinary(t *testing.T) {
	tmpDir := t.TempDir()
	k3sPath := filepath.Join(tmpDir, "k3s")
	if err := os.WriteFile(k3sPath, []byte{}, 0o755); err != nil {
		t.Fatalf("create k3s binary: %v", err)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	d := testK3sDetector()
	d.Detect()

	if d.IsK3s() {
		t.Fatal("expected not detected via PATH (detector checks /usr/local/bin/k3s)")
	}
}

func TestK3sEnvironmentDetectorDetectedViaNodeName(t *testing.T) {
	os.Setenv("NODE_NAME", "test-node-1")
	defer os.Unsetenv("NODE_NAME")

	d := testK3sDetector()
	err := d.Detect()
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !d.IsK3s() {
		t.Fatal("expected detected via NODE_NAME")
	}
	if d.NodeName() != "test-node-1" {
		t.Fatalf("expected node name test-node-1, got %q", d.NodeName())
	}
	if d.Mode() != contracts.RuntimeModeDistributedK3s {
		t.Fatalf("expected mode distributed-k3s, got %q", d.Mode())
	}
}

func TestK3sEnvironmentDetectorDetectedViaK3sURL(t *testing.T) {
	os.Setenv("K3S_URL", "https://k3s-server:6443")
	defer os.Unsetenv("K3S_URL")

	d := testK3sDetector()
	err := d.Detect()
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !d.IsK3s() {
		t.Fatal("expected detected via K3S_URL")
	}
	if d.Mode() != contracts.RuntimeModeDistributedK3s {
		t.Fatalf("expected mode distributed-k3s, got %q", d.Mode())
	}
}

func TestK3sEnvironmentDetectorDetectedViaK3sToken(t *testing.T) {
	os.Setenv("K3S_TOKEN", "test-token-123")
	defer os.Unsetenv("K3S_TOKEN")

	d := testK3sDetector()
	err := d.Detect()
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !d.IsK3s() {
		t.Fatal("expected detected via K3S_TOKEN")
	}
}

func TestK3sEnvironmentDetectorAssertNotK3s(t *testing.T) {
	d := testK3sDetector()
	err := d.AssertNotK3s("prepare_release_workspace")
	if err != nil {
		t.Fatalf("expected no error when not k3s, got %v", err)
	}
}

func TestK3sEnvironmentDetectorAssertNotK3sRejected(t *testing.T) {
	os.Setenv("NODE_NAME", "test-node-1")
	defer os.Unsetenv("NODE_NAME")

	d := testK3sDetector()
	d.Detect()

	err := d.AssertNotK3s("prepare_release_workspace")
	if err == nil {
		t.Fatal("expected error when k3s detected")
	}
	k3sErr, ok := err.(*K3sOperationNotAllowedError)
	if !ok {
		t.Fatalf("expected K3sOperationNotAllowedError, got %T", err)
	}
	if k3sErr.Operation != "prepare_release_workspace" {
		t.Fatalf("expected operation prepare_release_workspace, got %q", k3sErr.Operation)
	}
}

func TestK3sEnvironmentDetectorPersistDetection(t *testing.T) {
	os.Setenv("NODE_NAME", "test-node-1")
	defer os.Unsetenv("NODE_NAME")

	d := testK3sDetector()
	d.Detect()

	root := filepath.Join(t.TempDir(), "runtime-root")
	detectPath, err := d.PersistDetection(root)
	if err != nil {
		t.Fatalf("persist detection: %v", err)
	}

	if _, err := os.Stat(detectPath); err != nil {
		t.Fatalf("expected detection file to exist: %v", err)
	}
}

func TestK3sEnvironmentDetectorDetectedAt(t *testing.T) {
	d := testK3sDetector()
	if !d.DetectedAt().IsZero() {
		t.Fatal("expected zero detected_at before detection")
	}

	os.Setenv("NODE_NAME", "test-node-1")
	defer os.Unsetenv("NODE_NAME")

	d.Detect()
	if d.DetectedAt().IsZero() {
		t.Fatal("expected non-zero detected_at after detection")
	}
}
