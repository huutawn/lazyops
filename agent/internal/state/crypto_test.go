package state

import "testing"

func TestEncryptAndDecryptSecretRoundTrip(t *testing.T) {
	encrypted, err := EncryptSecret("agt-secret-standalone", "state-key-123")
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	if encrypted == "agt-secret-standalone" {
		t.Fatal("expected encrypted payload to differ from plaintext")
	}

	decrypted, err := DecryptSecret(encrypted, "state-key-123")
	if err != nil {
		t.Fatalf("decrypt secret: %v", err)
	}
	if decrypted != "agt-secret-standalone" {
		t.Fatalf("unexpected decrypted secret %q", decrypted)
	}
}
