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

func TestTextRedactsSSHKeys(t *testing.T) {
	input := "config: -----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----"
	output := Text(input)

	if strings.Contains(output, "MIIEowIBAAKCAQEA") {
		t.Fatalf("expected SSH key material to be redacted, got %q", output)
	}
	if !strings.Contains(output, "BEGIN RSA PRIVATE KEY") {
		t.Fatalf("expected BEGIN marker to remain, got %q", output)
	}
	if !strings.Contains(output, maskedValue) {
		t.Fatalf("expected masked value, got %q", output)
	}
}

func TestTextRedactsSSHCertificates(t *testing.T) {
	input := "pub: ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ user@host"
	output := Text(input)

	if strings.Contains(output, "AAAAB3NzaC1yc2EAAAADAQABAAABAQ") {
		t.Fatalf("expected SSH cert content to be redacted, got %q", output)
	}
	if !strings.Contains(output, "ssh-rsa ") {
		t.Fatalf("expected ssh-rsa prefix to remain, got %q", output)
	}
	if !strings.Contains(output, maskedValue) {
		t.Fatalf("expected masked value, got %q", output)
	}
}

func TestTextRedactsKubeconfigValues(t *testing.T) {
	input := "kubeconfig: /home/user/.kube/config password=mysecret123"
	output := Text(input)

	if strings.Contains(output, "/home/user/.kube/config") {
		t.Fatalf("expected kubeconfig path to be redacted, got %q", output)
	}
	if strings.Contains(output, "mysecret123") {
		t.Fatalf("expected password to be redacted, got %q", output)
	}
}

func TestTextRedactsSSHCredentials(t *testing.T) {
	input := "ssh_key: sk_live_abc123"
	output := Text(input)

	if strings.Contains(output, "sk_live_abc123") {
		t.Fatalf("expected ssh_key value to be redacted, got %q", output)
	}
}
