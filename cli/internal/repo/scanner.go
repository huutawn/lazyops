package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrRepoRootNotFound = errors.New("repository root not found")

type ServiceSignal string

const (
	SignalPackageJSON  ServiceSignal = "package.json"
	SignalGoMod        ServiceSignal = "go.mod"
	SignalRequirements ServiceSignal = "requirements.txt"
	SignalDockerfile   ServiceSignal = "Dockerfile"
)

type RepoScanResult struct {
	StartPath string            `json:"start_path"`
	RepoRoot  string            `json:"repo_root"`
	Monorepo  bool              `json:"monorepo"`
	Services  []DetectedService `json:"services"`
}

type DetectedService struct {
	Name    string          `json:"name"`
	Path    string          `json:"path"`
	Signals []ServiceSignal `json:"signals"`
}

func Scan(startPath string) (RepoScanResult, error) {
	repoRoot, err := FindRepoRoot(startPath)
	if err != nil {
		return RepoScanResult{}, err
	}

	startAbs, err := filepath.Abs(startPath)
	if err != nil {
		return RepoScanResult{}, fmt.Errorf("resolve scan start path: %w", err)
	}

	serviceSignals, err := collectServiceSignals(repoRoot)
	if err != nil {
		return RepoScanResult{}, err
	}

	services := make([]DetectedService, 0, len(serviceSignals))
	for relPath, signals := range serviceSignals {
		servicePath := relPath
		if servicePath == "" {
			servicePath = "."
		}

		name := filepath.Base(repoRoot)
		if servicePath != "." {
			name = filepath.Base(servicePath)
		}

		services = append(services, DetectedService{
			Name:    name,
			Path:    servicePath,
			Signals: signals,
		})
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Path < services[j].Path
	})

	return RepoScanResult{
		StartPath: startAbs,
		RepoRoot:  repoRoot,
		Monorepo:  len(services) > 1,
		Services:  services,
	}, nil
}

func FindRepoRoot(startPath string) (string, error) {
	current, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}

	info, err := os.Stat(current)
	if err != nil {
		return "", fmt.Errorf("stat scan start path: %w", err)
	}
	if !info.IsDir() {
		current = filepath.Dir(current)
	}

	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrRepoRootNotFound
		}
		current = parent
	}
}

func (result RepoScanResult) LayoutLabel() string {
	if result.Monorepo {
		return "monorepo"
	}
	return "single-service"
}

func (service DetectedService) SignalNames() []string {
	names := make([]string, 0, len(service.Signals))
	for _, signal := range service.Signals {
		names = append(names, string(signal))
	}
	return names
}

func collectServiceSignals(repoRoot string) (map[string][]ServiceSignal, error) {
	serviceSignals := map[string]map[ServiceSignal]struct{}{}

	err := filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			name := entry.Name()
			if name != "." && shouldSkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		signal, ok := detectSignal(entry.Name())
		if !ok {
			return nil
		}

		dir := filepath.Dir(path)
		relPath, err := filepath.Rel(repoRoot, dir)
		if err != nil {
			return err
		}

		if _, exists := serviceSignals[relPath]; !exists {
			serviceSignals[relPath] = map[ServiceSignal]struct{}{}
		}
		serviceSignals[relPath][signal] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan repository markers: %w", err)
	}

	normalized := make(map[string][]ServiceSignal, len(serviceSignals))
	for relPath, signals := range serviceSignals {
		list := make([]ServiceSignal, 0, len(signals))
		for signal := range signals {
			list = append(list, signal)
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i] < list[j]
		})
		normalized[relPath] = list
	}

	return normalized, nil
}

func detectSignal(name string) (ServiceSignal, bool) {
	switch name {
	case string(SignalPackageJSON):
		return SignalPackageJSON, true
	case string(SignalGoMod):
		return SignalGoMod, true
	case string(SignalRequirements):
		return SignalRequirements, true
	case string(SignalDockerfile):
		return SignalDockerfile, true
	default:
		return "", false
	}
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".venv", "venv", "dist", "build":
		return true
	default:
		return strings.HasPrefix(name, ".") && name != "."
	}
}
