package credentials

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

type unavailableKeychain struct{}

func (unavailableKeychain) Name() string {
	return "keychain"
}

func (unavailableKeychain) Save(context.Context, string, string, []byte) error {
	return ErrKeychainUnavailable
}

func (unavailableKeychain) Load(context.Context, string, string) ([]byte, error) {
	return nil, ErrKeychainUnavailable
}

func (unavailableKeychain) Delete(context.Context, string, string) error {
	return ErrKeychainUnavailable
}

type macOSKeychain struct{}

func (macOSKeychain) Name() string {
	return "keychain"
}

func (macOSKeychain) Save(ctx context.Context, service string, account string, payload []byte) error {
	cmd := exec.CommandContext(ctx, "security", "add-generic-password", "-U", "-a", account, "-s", service, "-w", string(payload))
	if output, err := cmd.CombinedOutput(); err != nil {
		return classifyKeychainError(output, err)
	}
	return nil
}

func (macOSKeychain) Load(ctx context.Context, service string, account string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "security", "find-generic-password", "-w", "-a", account, "-s", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, classifyKeychainError(output, err)
	}
	return bytes.TrimSpace(output), nil
}

func (macOSKeychain) Delete(ctx context.Context, service string, account string) error {
	cmd := exec.CommandContext(ctx, "security", "delete-generic-password", "-a", account, "-s", service)
	if output, err := cmd.CombinedOutput(); err != nil {
		return classifyKeychainError(output, err)
	}
	return nil
}

type linuxSecretToolKeychain struct{}

func (linuxSecretToolKeychain) Name() string {
	return "keychain"
}

func (linuxSecretToolKeychain) Save(ctx context.Context, service string, account string, payload []byte) error {
	cmd := exec.CommandContext(ctx, "secret-tool", "store", "--label=LazyOps CLI", "service", service, "account", account)
	cmd.Stdin = bytes.NewReader(payload)
	if output, err := cmd.CombinedOutput(); err != nil {
		return classifyKeychainError(output, err)
	}
	return nil
}

func (linuxSecretToolKeychain) Load(ctx context.Context, service string, account string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "secret-tool", "lookup", "service", service, "account", account)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, classifyKeychainError(output, err)
	}
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return nil, ErrNotFound
	}
	return trimmed, nil
}

func (linuxSecretToolKeychain) Delete(ctx context.Context, service string, account string) error {
	cmd := exec.CommandContext(ctx, "secret-tool", "clear", "service", service, "account", account)
	if output, err := cmd.CombinedOutput(); err != nil {
		return classifyKeychainError(output, err)
	}
	return nil
}

func classifyKeychainError(output []byte, err error) error {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return err
	}

	lowered := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lowered, "could not be found"):
		return ErrNotFound
	case strings.Contains(lowered, "not found"):
		return ErrNotFound
	case strings.Contains(lowered, "no such secret collection"):
		return ErrKeychainUnavailable
	case strings.Contains(lowered, "no secret service"):
		return ErrKeychainUnavailable
	default:
		return errors.New(trimmed)
	}
}
