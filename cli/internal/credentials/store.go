package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	ErrNotFound            = errors.New("credentials not found")
	ErrKeychainUnavailable = errors.New("keychain unavailable")
)

type Record struct {
	Token         string    `json:"token"`
	UserID        string    `json:"user_id,omitempty"`
	DisplayName   string    `json:"display_name,omitempty"`
	StoredAt      time.Time `json:"stored_at"`
	StorageMethod string    `json:"storage_method,omitempty"`
}

type StoreConfig struct {
	Service         string
	Account         string
	CredentialsPath string
}

type SaveResult struct {
	Backend string
}

type Store struct {
	service string
	account string

	keychain Keychain
	file     *FileStore
}

func NewStore(cfg StoreConfig) (*Store, error) {
	service := strings.TrimSpace(cfg.Service)
	if service == "" {
		service = "lazyops-cli"
	}

	account := strings.TrimSpace(cfg.Account)
	if account == "" {
		account = "default"
	}

	credentialsPath := strings.TrimSpace(cfg.CredentialsPath)
	if credentialsPath == "" {
		return nil, fmt.Errorf("credentials path is required")
	}

	return &Store{
		service:  service,
		account:  account,
		keychain: DetectNativeKeychain(),
		file:     NewFileStore(credentialsPath),
	}, nil
}

func NewStoreWithKeychain(cfg StoreConfig, keychain Keychain) (*Store, error) {
	store, err := NewStore(cfg)
	if err != nil {
		return nil, err
	}

	store.keychain = keychain
	return store, nil
}

func (s *Store) Save(ctx context.Context, record Record) (SaveResult, error) {
	record.Token = strings.TrimSpace(record.Token)
	record.UserID = strings.TrimSpace(record.UserID)
	record.DisplayName = strings.TrimSpace(record.DisplayName)

	if record.Token == "" {
		return SaveResult{}, fmt.Errorf("token is required")
	}
	if record.StoredAt.IsZero() {
		record.StoredAt = time.Now().UTC()
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return SaveResult{}, err
	}

	if s.keychain != nil {
		if err := s.keychain.Save(ctx, s.service, s.account, payload); err == nil {
			return SaveResult{Backend: s.keychain.Name()}, nil
		} else if !errors.Is(err, ErrKeychainUnavailable) {
			if fallbackErr := s.file.Save(record); fallbackErr != nil {
				return SaveResult{}, errors.Join(err, fallbackErr)
			}
			return SaveResult{Backend: "file"}, nil
		}
	}

	if err := s.file.Save(record); err != nil {
		return SaveResult{}, err
	}

	return SaveResult{Backend: "file"}, nil
}

func (s *Store) Load(ctx context.Context) (Record, error) {
	if s.keychain != nil {
		payload, err := s.keychain.Load(ctx, s.service, s.account)
		if err == nil {
			record, decodeErr := decodeRecord(payload)
			if decodeErr != nil {
				return Record{}, decodeErr
			}
			record.StorageMethod = s.keychain.Name()
			return record, nil
		}
		if !errors.Is(err, ErrKeychainUnavailable) && !errors.Is(err, ErrNotFound) {
			if record, fallbackErr := s.file.Load(); fallbackErr == nil {
				record.StorageMethod = "file"
				return record, nil
			}
			return Record{}, err
		}
	}

	record, err := s.file.Load()
	if err != nil {
		return Record{}, err
	}
	record.StorageMethod = "file"
	return record, nil
}

func (s *Store) Clear(ctx context.Context) error {
	var errs []error

	if s.keychain != nil {
		if err := s.keychain.Delete(ctx, s.service, s.account); err != nil && !errors.Is(err, ErrKeychainUnavailable) && !errors.Is(err, ErrNotFound) {
			errs = append(errs, err)
		}
	}

	if err := s.file.Delete(); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrNotFound) {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...)
}

func DefaultCredentialsPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return filepath.Join(".lazyops", "credentials.json")
	}

	return filepath.Join(homeDir, ".config", "lazyops", "credentials.json")
}

func decodeRecord(payload []byte) (Record, error) {
	var record Record
	if err := json.Unmarshal(payload, &record); err != nil {
		return Record{}, err
	}
	if strings.TrimSpace(record.Token) == "" {
		return Record{}, fmt.Errorf("stored credential token is empty")
	}
	return record, nil
}

type Keychain interface {
	Name() string
	Save(ctx context.Context, service string, account string, payload []byte) error
	Load(ctx context.Context, service string, account string) ([]byte, error)
	Delete(ctx context.Context, service string, account string) error
}

func DetectNativeKeychain() Keychain {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("security"); err == nil {
			return &macOSKeychain{}
		}
	case "linux":
		if _, err := exec.LookPath("secret-tool"); err == nil {
			return &linuxSecretToolKeychain{}
		}
	}

	return unavailableKeychain{}
}
