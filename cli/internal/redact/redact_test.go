package redact

import (
	"strings"
	"testing"
)

func TestTextRedactsBearerAndSecretAssignments(t *testing.T) {
	input := `Authorization: Bearer abc123 token=plain-secret password="p@ss"`
	output := Text(input)

	for _, secret := range []string{"abc123", "plain-secret", "p@ss"} {
		if strings.Contains(output, secret) {
			t.Fatalf("expected output to redact %q, got %q", secret, output)
		}
	}

	if !strings.Contains(output, maskedValue) {
		t.Fatalf("expected output to contain mask, got %q", output)
	}
}

func TestPrettyJSONRedactsSensitiveKeys(t *testing.T) {
	input := []byte(`{"token":"plain-token","nested":{"password":"secret"},"safe":"value"}`)
	output := string(PrettyJSON(input))

	for _, secret := range []string{"plain-token", "secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("expected output to redact %q, got %q", secret, output)
		}
	}

	if !strings.Contains(output, `"safe": "value"`) {
		t.Fatalf("expected safe value to remain visible, got %q", output)
	}
}
