package runtime

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazyops-agent/internal/contracts"
)

type K3sEnvironmentDetector struct {
	logger *slog.Logger
	now    func() time.Time

	mu         sync.Mutex
	detected   bool
	k3sVersion string
	nodeName   string
	detectedAt time.Time
	mode       contracts.RuntimeMode
}

func NewK3sEnvironmentDetector(logger *slog.Logger) *K3sEnvironmentDetector {
	return &K3sEnvironmentDetector{
		logger: logger,
		now: func() time.Time {
			return time.Now().UTC()
		},
		mode: contracts.RuntimeModeStandalone,
	}
}

func (d *K3sEnvironmentDetector) Detect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	detected := false
	k3sVersion := ""
	nodeName := ""

	if _, err := os.Stat("/usr/local/bin/k3s"); err == nil {
		detected = true
		k3sVersion = "detected"
	}

	if envNode := os.Getenv("NODE_NAME"); envNode != "" {
		nodeName = envNode
		detected = true
	}

	if envK3s := os.Getenv("K3S_URL"); envK3s != "" {
		detected = true
	}

	if envK3sToken := os.Getenv("K3S_TOKEN"); envK3sToken != "" {
		detected = true
	}

	if _, err := os.Stat("/etc/rancher/k3s/k3s.yaml"); err == nil {
		detected = true
		k3sVersion = "config-present"
	}

	d.detected = detected
	d.k3sVersion = k3sVersion
	d.nodeName = nodeName
	d.detectedAt = d.now()

	if detected {
		d.mode = contracts.RuntimeModeDistributedK3s
	} else {
		d.mode = contracts.RuntimeModeStandalone
	}

	return nil
}

func (d *K3sEnvironmentDetector) IsK3s() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.detected
}

func (d *K3sEnvironmentDetector) Mode() contracts.RuntimeMode {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mode
}

func (d *K3sEnvironmentDetector) K3sVersion() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.k3sVersion
}

func (d *K3sEnvironmentDetector) NodeName() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.nodeName
}

func (d *K3sEnvironmentDetector) DetectedAt() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.detectedAt
}

func (d *K3sEnvironmentDetector) AssertNotK3s(operation string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.detected {
		return &K3sOperationNotAllowedError{
			Operation: operation,
			NodeName:  d.nodeName,
		}
	}

	return nil
}

type K3sOperationNotAllowedError struct {
	Operation string
	NodeName  string
}

func (e *K3sOperationNotAllowedError) Error() string {
	return "k3s operation not allowed: " + e.Operation + " (node: " + e.NodeName + ")"
}

func (d *K3sEnvironmentDetector) PersistDetection(workspaceRoot string) (string, error) {
	d.mu.Lock()
	detected := d.detected
	k3sVersion := d.k3sVersion
	nodeName := d.nodeName
	detectedAt := d.detectedAt
	mode := d.mode
	d.mu.Unlock()

	detectDir := filepath.Join(workspaceRoot, "node-agent")
	if err := os.MkdirAll(detectDir, 0o755); err != nil {
		return "", err
	}

	detectPath := filepath.Join(detectDir, "k3s-detection.json")

	content := map[string]any{
		"detected":    detected,
		"k3s_version": k3sVersion,
		"node_name":   nodeName,
		"detected_at": detectedAt,
		"mode":        string(mode),
	}

	raw, err := marshalJSON(content)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(detectPath, raw, 0o644); err != nil {
		return "", err
	}

	return detectPath, nil
}

func marshalJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
