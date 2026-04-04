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

func TestRedactManagedCredentialFields(t *testing.T) {
	got := RedactField("managed_credentials", map[string]string{
		"LAZYOPS_MANAGED_API_REF":    "managed://prj_123/web/api",
		"LAZYOPS_MANAGED_API_HANDLE": "mcred_abcdef12",
	})
	typed, ok := got.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string result, got %T", got)
	}
	if typed["LAZYOPS_MANAGED_API_REF"] != redactedValue {
		t.Fatalf("expected managed credential ref to be redacted, got %v", typed["LAZYOPS_MANAGED_API_REF"])
	}
	if typed["LAZYOPS_MANAGED_API_HANDLE"] != redactedValue {
		t.Fatalf("expected managed credential handle to be redacted, got %v", typed["LAZYOPS_MANAGED_API_HANDLE"])
	}
}
