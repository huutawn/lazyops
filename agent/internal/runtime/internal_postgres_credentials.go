package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazyops-agent/internal/state"
)

type internalPostgresCredentialState struct {
	Kind              string    `json:"kind"`
	ProjectID         string    `json:"project_id"`
	BindingID         string    `json:"binding_id"`
	Database          string    `json:"database"`
	Username          string    `json:"username"`
	PasswordEncrypted string    `json:"password_encrypted,omitempty"`
	PasswordPlaintext string    `json:"password_plaintext,omitempty"`
	CredentialRef     string    `json:"credential_ref"`
	AuditRef          string    `json:"audit_ref"`
	LocalListenerPort int       `json:"local_listener_port,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Password          string    `json:"-"`
}

func internalPostgresCredentialStatePath(runtimeRoot, projectID, bindingID string) string {
	return filepath.Join(
		runtimeRoot,
		"projects",
		projectID,
		"bindings",
		bindingID,
		"internal-services",
		"postgres",
		"credentials.json",
	)
}

func loadOrCreateInternalPostgresCredentialState(runtimeRoot, stateKey, projectID, bindingID string, listenerPort int, now time.Time) (internalPostgresCredentialState, error) {
	statePath := internalPostgresCredentialStatePath(runtimeRoot, projectID, bindingID)
	if payload, err := os.ReadFile(statePath); err == nil {
		var existing internalPostgresCredentialState
		if err := json.Unmarshal(payload, &existing); err == nil {
			record, changed, err := materializeInternalPostgresCredentialState(existing, stateKey, projectID, bindingID, listenerPort, now)
			if err != nil {
				return internalPostgresCredentialState{}, err
			}
			if changed {
				if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
					return internalPostgresCredentialState{}, err
				}
				if err := writeJSON(statePath, record); err != nil {
					return internalPostgresCredentialState{}, err
				}
			}
			return record, nil
		}
	}

	record, _, err := materializeInternalPostgresCredentialState(internalPostgresCredentialState{}, stateKey, projectID, bindingID, listenerPort, now)
	if err != nil {
		return internalPostgresCredentialState{}, err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return internalPostgresCredentialState{}, err
	}
	if err := writeJSON(statePath, record); err != nil {
		return internalPostgresCredentialState{}, err
	}
	return record, nil
}

func materializeInternalPostgresCredentialState(existing internalPostgresCredentialState, stateKey, projectID, bindingID string, listenerPort int, now time.Time) (internalPostgresCredentialState, bool, error) {
	changed := false
	if existing.CreatedAt.IsZero() {
		existing.CreatedAt = now
		changed = true
	}
	if strings.TrimSpace(existing.Kind) == "" {
		existing.Kind = "postgres"
		changed = true
	}
	if strings.TrimSpace(existing.ProjectID) == "" {
		existing.ProjectID = projectID
		changed = true
	}
	if strings.TrimSpace(existing.BindingID) == "" {
		existing.BindingID = bindingID
		changed = true
	}
	if strings.TrimSpace(existing.Database) == "" {
		existing.Database = "app"
		changed = true
	}
	if strings.TrimSpace(existing.Username) == "" {
		existing.Username = "lazyops_managed"
		changed = true
	}
	if strings.TrimSpace(existing.CredentialRef) == "" {
		existing.CredentialRef = fmt.Sprintf("managed://%s/%s/internal/postgres", projectID, bindingID)
		changed = true
	}
	if strings.TrimSpace(existing.AuditRef) == "" {
		existing.AuditRef = fmt.Sprintf("audit://%s/%s/internal/postgres", projectID, bindingID)
		changed = true
	}
	if existing.LocalListenerPort != listenerPort {
		existing.LocalListenerPort = listenerPort
		changed = true
	}

	password := ""
	if decrypted := decryptInternalServicePassword(strings.TrimSpace(existing.PasswordEncrypted), stateKey); decrypted != "" {
		password = decrypted
	} else if plain := strings.TrimSpace(existing.PasswordPlaintext); plain != "" {
		password = plain
	} else {
		generated, err := generateInternalServiceUUIDPassword()
		if err != nil {
			return internalPostgresCredentialState{}, false, err
		}
		password = generated
		changed = true
	}

	if strings.TrimSpace(password) == "" {
		generated, err := generateInternalServiceUUIDPassword()
		if err != nil {
			return internalPostgresCredentialState{}, false, err
		}
		password = generated
		changed = true
	}
	existing.Password = password

	if strings.TrimSpace(stateKey) != "" {
		encrypted, err := state.EncryptSecret(password, stateKey)
		if err != nil {
			return internalPostgresCredentialState{}, false, err
		}
		if encrypted != strings.TrimSpace(existing.PasswordEncrypted) {
			existing.PasswordEncrypted = encrypted
			changed = true
		}
		if strings.TrimSpace(existing.PasswordPlaintext) != "" {
			existing.PasswordPlaintext = ""
			changed = true
		}
	} else if strings.TrimSpace(existing.PasswordPlaintext) != password {
		existing.PasswordPlaintext = password
		changed = true
	}

	existing.UpdatedAt = now
	return existing, changed, nil
}

func decryptInternalServicePassword(ciphertext, stateKey string) string {
	ciphertext = strings.TrimSpace(ciphertext)
	stateKey = strings.TrimSpace(stateKey)
	if ciphertext == "" || stateKey == "" {
		return ""
	}
	plaintext, err := state.DecryptSecret(ciphertext, stateKey)
	if err != nil {
		return ""
	}
	return plaintext
}

func generateInternalServiceUUIDPassword() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(buf)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hexValue[0:8],
		hexValue[8:12],
		hexValue[12:16],
		hexValue[16:20],
		hexValue[20:32],
	), nil
}
