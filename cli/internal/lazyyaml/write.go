package lazyyaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
