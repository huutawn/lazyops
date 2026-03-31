package logger

import "testing"

func TestRedactFieldBySensitiveKey(t *testing.T) {
	if got := RedactField("agent_token", "agt-secret"); got != redactedValue {
		t.Fatalf("expected redacted token, got %v", got)
	}
}

func TestRedactBearerValue(t *testing.T) {
	got := RedactField("authorization", "Bearer super-secret-token")
	if got != "Bearer "+redactedValue {
		t.Fatalf("expected bearer token to be redacted, got %v", got)
	}
}

func TestRedactNestedMap(t *testing.T) {
	got := RedactField("payload", map[string]any{
		"mesh_key": "mesh-secret",
		"safe":     "visible",
	})

	typed, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", got)
	}
	if typed["mesh_key"] != redactedValue {
		t.Fatalf("expected nested mesh key to be redacted, got %v", typed["mesh_key"])
	}
	if typed["safe"] != "visible" {
		t.Fatalf("expected safe field to remain visible, got %v", typed["safe"])
	}
}
