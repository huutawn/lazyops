package lazyyaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type WriteResult struct {
	Path       string
	BackupPath string
	Overwrote  bool
}

func DefaultPath(repoRoot string) string {
	return filepath.Join(repoRoot, "lazyops.yaml")
}

func WriteFile(repoRoot string, payload []byte, overwrite bool) (WriteResult, error) {
	return writeFileWithClock(repoRoot, payload, overwrite, time.Now)
}

func writeFileWithClock(repoRoot string, payload []byte, overwrite bool, now func() time.Time) (WriteResult, error) {
	if repoRoot == "" {
		return WriteResult{}, fmt.Errorf("repo root is required to write lazyops.yaml")
	}

	if err := validatePayloadSecurity(payload); err != nil {
		return WriteResult{}, err
	}

	configPath := DefaultPath(repoRoot)
	info, err := os.Stat(configPath)
	switch {
	case err == nil:
		if !overwrite {
			return WriteResult{}, fmt.Errorf("lazyops.yaml already exists at %s. next: rerun `lazyops init --write --overwrite` to confirm replacement", configPath)
		}

		existing, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return WriteResult{}, fmt.Errorf("could not read the existing lazyops.yaml before backup: %w", readErr)
		}

		backupPath := configPath + ".bak." + now().UTC().Format("20060102-150405")
		if writeErr := os.WriteFile(backupPath, existing, info.Mode().Perm()); writeErr != nil {
			return WriteResult{}, fmt.Errorf("could not create a backup of the existing lazyops.yaml: %w", writeErr)
		}
		if writeErr := os.WriteFile(configPath, payload, info.Mode().Perm()); writeErr != nil {
			return WriteResult{}, fmt.Errorf("could not overwrite lazyops.yaml after backup: %w", writeErr)
		}

		return WriteResult{
			Path:       configPath,
			BackupPath: backupPath,
			Overwrote:  true,
		}, nil
	case errors.Is(err, os.ErrNotExist):
		if writeErr := os.WriteFile(configPath, payload, 0o644); writeErr != nil {
			return WriteResult{}, fmt.Errorf("could not write lazyops.yaml: %w", writeErr)
		}
		return WriteResult{Path: configPath}, nil
	default:
		return WriteResult{}, fmt.Errorf("could not inspect lazyops.yaml path: %w", err)
	}
}

var (
	secretFieldPattern      = regexp.MustCompile(`(?mi)^\s*(ssh_key|ssh|private_key|password|pat|token|agent_token|github_token|secret|kubeconfig|public_ip|private_ip|server_ip|project_id|deployment_binding_id|target_id|target_kind|instance_id|mesh_network_id|cluster_id|deploy_command)\s*:`)
	forbiddenContentMarkers = []string{
		"secret://",
		"-----begin private key-----",
		"-----begin rsa private key-----",
		"-----begin openssh private key-----",
		"ssh-rsa ",
		"github_pat_",
		"ghp_",
		"glpat-",
		"bearer ",
	}
	writeIPv4Pattern = regexp.MustCompile(`\b(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\b`)
)

func validatePayloadSecurity(payload []byte) error {
	content := string(payload)
	lowerContent := strings.ToLower(content)

	for _, marker := range forbiddenContentMarkers {
		if strings.Contains(lowerContent, strings.ToLower(marker)) {
			return fmt.Errorf("lazyops.yaml must not contain secrets, kubeconfig material, or raw credentials. next: verify the deploy contract stays logical and retry `lazyops init --write`")
		}
	}

	if secretFieldPattern.MatchString(content) {
		return fmt.Errorf("lazyops.yaml contains forbidden field names. next: remove SSH keys, passwords, tokens, IPs, or backend IDs from the deploy contract and retry `lazyops init --write`")
	}

	if writeIPv4Pattern.MatchString(content) {
		return fmt.Errorf("lazyops.yaml must not contain raw infrastructure IP addresses. next: use logical target references instead of server IPs and retry `lazyops init --write`")
	}

	return nil
}
