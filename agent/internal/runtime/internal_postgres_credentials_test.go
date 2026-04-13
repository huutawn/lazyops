package runtime

import (
	"testing"
	"time"
)

func TestLoadOrCreateInternalPostgresCredentialStateMigratesPlaintextToEncryptedWithoutRotatingSecret(t *testing.T) {
	root := t.TempDir()
	createdAt := time.Date(2026, 4, 13, 2, 0, 0, 0, time.UTC)

	plainRecord, err := loadOrCreateInternalPostgresCredentialState(root, "", "prj_1", "bind_1", 5432, createdAt)
	if err != nil {
		t.Fatalf("create plaintext credential state: %v", err)
	}
	if plainRecord.Password == "" {
		t.Fatal("expected plaintext credential state to materialize a password")
	}
	if plainRecord.PasswordPlaintext == "" {
		t.Fatal("expected plaintext fallback password to be stored when no state key is configured")
	}
	if plainRecord.PasswordEncrypted != "" {
		t.Fatalf("expected no encrypted password without a state key, got %q", plainRecord.PasswordEncrypted)
	}

	encryptedRecord, err := loadOrCreateInternalPostgresCredentialState(root, "migration-state-key", "prj_1", "bind_1", 5432, createdAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("migrate credential state to encrypted: %v", err)
	}
	if encryptedRecord.Password != plainRecord.Password {
		t.Fatalf("expected migrated password to stay stable, got %q want %q", encryptedRecord.Password, plainRecord.Password)
	}
	if encryptedRecord.PasswordEncrypted == "" {
		t.Fatal("expected encrypted password after migration")
	}
	if encryptedRecord.PasswordPlaintext != "" {
		t.Fatalf("expected plaintext password to be removed after migration, got %q", encryptedRecord.PasswordPlaintext)
	}
}
