package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStorePrefersKeychainWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lazyops", "credentials.json")
	keychain := &fakeKeychain{}

	store, err := NewStoreWithKeychain(StoreConfig{
		Service:         "lazyops-cli",
		Account:         "default",
		CredentialsPath: path,
	}, keychain)
	if err != nil {
		t.Fatalf("NewStoreWithKeychain() error = %v", err)
	}

	result, err := store.Save(context.Background(), Record{Token: "plain-token"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if result.Backend != keychain.Name() {
		t.Fatalf("expected backend %q, got %q", keychain.Name(), result.Backend)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected fallback file not to exist, got stat err %v", err)
	}

	record, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if record.Token != "plain-token" {
		t.Fatalf("expected token from keychain, got %q", record.Token)
	}
}

func TestStoreFallsBackToProtectedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lazyops", "credentials.json")

	store, err := NewStoreWithKeychain(StoreConfig{
		Service:         "lazyops-cli",
		Account:         "default",
		CredentialsPath: path,
	}, unavailableKeychain{})
	if err != nil {
		t.Fatalf("NewStoreWithKeychain() error = %v", err)
	}

	result, err := store.Save(context.Background(), Record{Token: "plain-token"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if result.Backend != "file" {
		t.Fatalf("expected file backend, got %q", result.Backend)
	}

	record, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if record.Token != "plain-token" {
		t.Fatalf("expected token from fallback file, got %q", record.Token)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("expected file mode 0600, got %o", fileInfo.Mode().Perm())
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("expected dir mode 0700, got %o", dirInfo.Mode().Perm())
	}
}

func TestStoreClearRemovesFallbackFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lazyops", "credentials.json")

	store, err := NewStoreWithKeychain(StoreConfig{
		Service:         "lazyops-cli",
		Account:         "default",
		CredentialsPath: path,
	}, unavailableKeychain{})
	if err != nil {
		t.Fatalf("NewStoreWithKeychain() error = %v", err)
	}

	if _, err := store.Save(context.Background(), Record{Token: "plain-token"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.Clear(context.Background()); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected credential file to be removed, got stat err %v", err)
	}
}

type fakeKeychain struct {
	payload []byte
}

func (f *fakeKeychain) Name() string {
	return "keychain"
}

func (f *fakeKeychain) Save(_ context.Context, _ string, _ string, payload []byte) error {
	f.payload = append([]byte(nil), payload...)
	return nil
}

func (f *fakeKeychain) Load(_ context.Context, _ string, _ string) ([]byte, error) {
	if len(f.payload) == 0 {
		return nil, ErrNotFound
	}

	return append([]byte(nil), f.payload...), nil
}

func (f *fakeKeychain) Delete(_ context.Context, _ string, _ string) error {
	f.payload = nil
	return nil
}
