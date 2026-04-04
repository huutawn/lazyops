package gitrepo

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var ErrOriginRemoteNotFound = errors.New("origin remote not found")

type Metadata struct {
	RepoRoot      string
	GitDir        string
	OriginURL     string
	RepoOwner     string
	RepoName      string
	CurrentBranch string
}

func Load(repoRoot string) (Metadata, error) {
	absoluteRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return Metadata{}, fmt.Errorf("resolve repo root: %w", err)
	}

	gitDir, err := resolveGitDir(absoluteRoot)
	if err != nil {
		return Metadata{}, err
	}

	originURL, err := readOriginURL(filepath.Join(gitDir, "config"))
	if err != nil {
		return Metadata{}, err
	}

	repoOwner, repoName, err := ParseGitHubRemote(originURL)
	if err != nil {
		return Metadata{}, err
	}

	currentBranch, err := readCurrentBranch(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		RepoRoot:      absoluteRoot,
		GitDir:        gitDir,
		OriginURL:     originURL,
		RepoOwner:     repoOwner,
		RepoName:      repoName,
		CurrentBranch: currentBranch,
	}, nil
}

func ParseRepoSlug(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repo %q is invalid. next: use `--repo <owner>/<name>`", value)
	}

	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return "", "", fmt.Errorf("repo %q is invalid. next: use `--repo <owner>/<name>`", value)
	}

	return owner, name, nil
}

func ParseGitHubRemote(remoteURL string) (string, string, error) {
	trimmed := strings.TrimSpace(remoteURL)
	if trimmed == "" {
		return "", "", errors.New("git origin remote is empty. next: set `origin` to a GitHub repo or use `--repo <owner>/<name>`")
	}

	if strings.HasPrefix(trimmed, "git@") {
		hostAndPath := strings.TrimPrefix(trimmed, "git@")
		parts := strings.SplitN(hostAndPath, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("git origin remote %q is invalid. next: point `origin` at `github.com/<owner>/<repo>`", remoteURL)
		}
		if !strings.EqualFold(parts[0], "github.com") {
			return "", "", fmt.Errorf("git origin remote %q is not a GitHub remote. next: point `origin` at GitHub or use `--repo <owner>/<name>`", remoteURL)
		}
		return parseRemotePath(parts[1], remoteURL)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("git origin remote %q is invalid. next: point `origin` at `github.com/<owner>/<repo>`", remoteURL)
	}
	if !strings.EqualFold(parsed.Host, "github.com") {
		return "", "", fmt.Errorf("git origin remote %q is not a GitHub remote. next: point `origin` at GitHub or use `--repo <owner>/<name>`", remoteURL)
	}
	return parseRemotePath(parsed.Path, remoteURL)
}

func resolveGitDir(repoRoot string) (string, error) {
	gitPath := filepath.Join(repoRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", fmt.Errorf("could not inspect git metadata. next: run `lazyops link` inside a git repository: %w", err)
	}

	if info.IsDir() {
		return gitPath, nil
	}

	content, err := os.ReadFile(gitPath)
	if err != nil {
		return "", fmt.Errorf("could not read git metadata. next: verify the repository is readable and retry `lazyops link`: %w", err)
	}

	const prefix = "gitdir:"
	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return "", fmt.Errorf("git metadata file %q is invalid. next: verify the repository is readable and retry `lazyops link`", gitPath)
	}

	gitDir := strings.TrimSpace(line[len(prefix):])
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoRoot, gitDir)
	}
	return filepath.Clean(gitDir), nil
}

func readOriginURL(configPath string) (string, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return "", fmt.Errorf("could not read git config. next: verify the repository is readable and retry `lazyops link`: %w", err)
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != `remote "origin"` {
			continue
		}
		if !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "url" && value != "" {
			return value, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("could not parse git config. next: verify `.git/config` and retry `lazyops link`: %w", err)
	}

	return "", ErrOriginRemoteNotFound
}

func readCurrentBranch(headPath string) (string, error) {
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", fmt.Errorf("could not read git HEAD. next: verify the repository is readable and retry `lazyops link`: %w", err)
	}

	line := strings.TrimSpace(string(content))
	const prefix = "ref: refs/heads/"
	if strings.HasPrefix(line, prefix) {
		return strings.TrimSpace(line[len(prefix):]), nil
	}

	return "", nil
}

func parseRemotePath(path string, remoteURL string) (string, string, error) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("git origin remote %q is invalid. next: point `origin` at `github.com/<owner>/<repo>`", remoteURL)
	}

	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return "", "", fmt.Errorf("git origin remote %q is invalid. next: point `origin` at `github.com/<owner>/<repo>`", remoteURL)
	}

	return owner, name, nil
}
