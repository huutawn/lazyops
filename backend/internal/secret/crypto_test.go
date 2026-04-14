package secret

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	encrypted, err := Encrypt("hello-secret", "backend-secret-key")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == "hello-secret" {
		t.Fatal("expected ciphertext to differ from plaintext")
	}
	decrypted, err := Decrypt(encrypted, "backend-secret-key")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "hello-secret" {
		t.Fatalf("expected decrypted plaintext to round-trip, got %q", decrypted)
	}
}
