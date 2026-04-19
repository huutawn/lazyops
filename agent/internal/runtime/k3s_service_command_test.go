package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazyops-agent/internal/contracts"
)

func TestHandleRestartK3sServiceDecodesJSONPayload(t *testing.T) {
	driver, logPath := newRecordedKubectlDriver(t)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, driver)

	payload, err := json.Marshal(map[string]string{
		"namespace":    "demo",
		"service_name": "api",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := service.handleRestartK3sService(context.Background(), contracts.CommandEnvelope{
		Type:    contracts.CommandRestartK3sService,
		Payload: payload,
	})

	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done result, got %#v", result)
	}
	logContent := readKubectlLog(t, logPath)
	for _, expected := range []string{
		"-n demo rollout restart deployment/api",
		"-n demo rollout status deployment/api --timeout=120s",
	} {
		if !strings.Contains(logContent, expected) {
			t.Fatalf("expected kubectl log to contain %q, got %q", expected, logContent)
		}
	}
}

func TestHandleLabelK3sNodeDecodesJSONPayload(t *testing.T) {
	driver, logPath := newRecordedKubectlDriver(t)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, driver)

	payload, err := json.Marshal(map[string]string{
		"node_name":   "worker-2",
		"label_key":   "lazyops.io/pinned-service",
		"label_value": "postgres",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	result := service.handleLabelK3sNode(context.Background(), contracts.CommandEnvelope{
		Type:    contracts.CommandLabelK3sNode,
		Payload: payload,
	})

	if result.Status != contracts.CommandAckDone {
		t.Fatalf("expected done result, got %#v", result)
	}
	logContent := readKubectlLog(t, logPath)
	for _, expected := range []string{
		"get node worker-2 -o jsonpath={.status.conditions[?(@.type=='Ready')].status}",
		"label node worker-2 lazyops.io/pinned-service=postgres --overwrite",
	} {
		if !strings.Contains(logContent, expected) {
			t.Fatalf("expected kubectl log to contain %q, got %q", expected, logContent)
		}
	}
}

func newRecordedKubectlDriver(t *testing.T) (*K3sDriver, string) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "kubectl.log")
	scriptPath := filepath.Join(dir, "kubectl")
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf '%s\\n' \"$*\" >> \"" + logPath + "\"",
		"if [ \"$1\" = \"get\" ] && [ \"$2\" = \"node\" ]; then",
		"  printf 'True'",
		"fi",
	}, "\n") + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write kubectl stub: %v", err)
	}

	driver := NewK3sDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), dir, scriptPath, "")
	return driver, logPath
}

func readKubectlLog(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read kubectl log: %v", err)
	}
	return string(content)
}
