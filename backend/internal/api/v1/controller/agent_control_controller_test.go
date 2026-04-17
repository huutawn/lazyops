package controller

import (
	"testing"

	"lazyops-server/internal/service"
)

func TestDeriveLogBatchLabelPrefersFirstNonEmptyValue(t *testing.T) {
	value := deriveLogBatchLabel([]service.LogBatchEntry{
		{
			Labels: map[string]string{
				"lazyops.service": "api",
			},
		},
		{
			Labels: map[string]string{
				"service": "web",
			},
		},
	}, "service", "lazyops.service")

	if value != "api" {
		t.Fatalf("expected first non-empty label, got %q", value)
	}
}

func TestDeriveLogBatchLabelReturnsEmptyWhenNoLabelsMatch(t *testing.T) {
	value := deriveLogBatchLabel([]service.LogBatchEntry{
		{
			Labels: map[string]string{
				"source_kind": "k3s",
			},
		},
	}, "service", "lazyops.service")

	if value != "" {
		t.Fatalf("expected empty label value, got %q", value)
	}
}

func TestFirstNonEmptyControl(t *testing.T) {
	value := firstNonEmptyControl("", "  ", "rev_123", "rev_456")
	if value != "rev_123" {
		t.Fatalf("expected first non-empty trimmed value, got %q", value)
	}
}
