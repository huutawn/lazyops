package repo

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type ServiceCandidate struct {
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	Signals     []ServiceSignal `json:"signals"`
	StartHint   string          `json:"start_hint,omitempty"`
	Healthcheck HealthcheckHint `json:"healthcheck,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
}

type HealthcheckHint struct {
	Path string `json:"path,omitempty"`
	Port int    `json:"port,omitempty"`
}

type DetectionResult struct {
	RepoRoot   string             `json:"repo_root"`
	Monorepo   bool               `json:"monorepo"`
	Candidates []ServiceCandidate `json:"candidates"`
}

type packageJSONManifest struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

type healthPathMatch struct {
	path string
	file string
}

func DetectServices(scanResult RepoScanResult) (DetectionResult, error) {
	candidates := make([]ServiceCandidate, 0, len(scanResult.Services))

	for _, service := range scanResult.Services {
		absPath := scanResult.RepoRoot
		if service.Path != "." {
			absPath = filepath.Join(scanResult.RepoRoot, service.Path)
		}

		candidate, err := detectServiceCandidate(absPath, service)
		if err != nil {
			return DetectionResult{}, err
		}
		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Path < candidates[j].Path
	})

	if err := validateCandidates(candidates); err != nil {
		return DetectionResult{}, err
	}

	return DetectionResult{
		RepoRoot:   scanResult.RepoRoot,
		Monorepo:   scanResult.Monorepo,
		Candidates: candidates,
	}, nil
}

func detectServiceCandidate(absPath string, service DetectedService) (ServiceCandidate, error) {
	candidate := ServiceCandidate{
		Name:    service.Name,
		Path:    service.Path,
		Signals: append([]ServiceSignal(nil), service.Signals...),
	}

	startHint, startWarnings, err := inferStartHint(absPath, candidate.Signals)
	if err != nil {
		return ServiceCandidate{}, fmt.Errorf("infer start hint for service %s: %w", candidate.Name, err)
	}
	candidate.StartHint = startHint
	candidate.Warnings = append(candidate.Warnings, startWarnings...)

	healthcheck, healthWarnings, err := inferHealthcheck(absPath)
	if err != nil {
		return ServiceCandidate{}, fmt.Errorf("infer healthcheck for service %s: %w", candidate.Name, err)
	}
	candidate.Healthcheck = healthcheck
	candidate.Warnings = append(candidate.Warnings, healthWarnings...)

	if candidate.StartHint == "" {
		candidate.Warnings = append(candidate.Warnings, "no start hint inferred yet; review this service before writing lazyops.yaml")
	}
	if candidate.Healthcheck.Path == "" {
		candidate.Warnings = append(candidate.Warnings, "no health hint inferred yet; add an explicit health check during init review")
	}

	return candidate, nil
}

func validateCandidates(candidates []ServiceCandidate) error {
	paths := map[string]struct{}{}
	names := map[string][]string{}

	for _, candidate := range candidates {
		if candidate.Path == "" {
			return fmt.Errorf("service candidate path is required")
		}
		if _, exists := paths[candidate.Path]; exists {
			return fmt.Errorf("duplicate service path %q detected. next: keep one service marker per path before rerunning `lazyops init`", candidate.Path)
		}
		paths[candidate.Path] = struct{}{}

		names[candidate.Name] = append(names[candidate.Name], candidate.Path)
	}

	for name, paths := range names {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		return fmt.Errorf("duplicate service name %q detected at %s. next: rename one service directory or refine detection before rerunning `lazyops init`", name, strings.Join(paths, ", "))
	}

	return nil
}

func inferStartHint(absPath string, signals []ServiceSignal) (string, []string, error) {
	warnings := []string{}

	if hasSignal(signals, SignalPackageJSON) {
		hint, packageWarnings, err := inferNodeStartHint(absPath)
		if err != nil {
			return "", nil, err
		}
		if hint != "" {
			return hint, packageWarnings, nil
		}
		warnings = append(warnings, packageWarnings...)
	}

	if hasSignal(signals, SignalGoMod) {
		hint, goWarnings, err := inferGoStartHint(absPath)
		if err != nil {
			return "", nil, err
		}
		if hint != "" {
			return hint, append(warnings, goWarnings...), nil
		}
		warnings = append(warnings, goWarnings...)
	}

	if hasSignal(signals, SignalRequirements) {
		hint, pythonWarnings := inferPythonStartHint(absPath)
		if hint != "" {
			return hint, append(warnings, pythonWarnings...), nil
		}
		warnings = append(warnings, pythonWarnings...)
	}

	if hasSignal(signals, SignalDockerfile) {
		hint, dockerWarnings, err := inferDockerStartHint(absPath)
		if err != nil {
			return "", nil, err
		}
		if hint != "" {
			return hint, append(warnings, dockerWarnings...), nil
		}
		warnings = append(warnings, dockerWarnings...)
	}

	return "", uniqueWarnings(warnings), nil
}

func inferNodeStartHint(absPath string) (string, []string, error) {
	manifestPath := filepath.Join(absPath, string(SignalPackageJSON))
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", nil, err
	}

	var manifest packageJSONManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return "", nil, fmt.Errorf("parse package.json: %w", err)
	}

	scriptPriority := []string{"start", "dev", "serve", "preview"}
	available := make([]string, 0, len(scriptPriority))
	for _, script := range scriptPriority {
		if strings.TrimSpace(manifest.Scripts[script]) != "" {
			available = append(available, script)
		}
	}

	if len(available) == 0 {
		return "", []string{"package.json was found but no start/dev/serve/preview script is available"}, nil
	}

	chosen := available[0]
	warnings := []string{}
	if chosen != "start" {
		warnings = append(warnings, fmt.Sprintf("using `npm run %s` as the best available start hint from package.json", chosen))
	}

	return "npm run " + chosen, warnings, nil
}

func inferGoStartHint(absPath string) (string, []string, error) {
	rootMain := filepath.Join(absPath, "main.go")
	if exists(rootMain) {
		return "go run .", nil, nil
	}

	cmdMainPaths, err := filepath.Glob(filepath.Join(absPath, "cmd", "*", "main.go"))
	if err != nil {
		return "", nil, err
	}
	sort.Strings(cmdMainPaths)

	switch len(cmdMainPaths) {
	case 0:
		return "", []string{"go.mod was found but no runnable Go entrypoint was inferred from main.go or cmd/*/main.go"}, nil
	case 1:
		rel, err := filepath.Rel(absPath, filepath.Dir(cmdMainPaths[0]))
		if err != nil {
			return "", nil, err
		}
		return "go run ./" + filepath.ToSlash(rel), nil, nil
	default:
		entrypoints := make([]string, 0, len(cmdMainPaths))
		for _, mainPath := range cmdMainPaths {
			rel, err := filepath.Rel(absPath, filepath.Dir(mainPath))
			if err != nil {
				return "", nil, err
			}
			entrypoints = append(entrypoints, "./"+filepath.ToSlash(rel))
		}
		return "", []string{fmt.Sprintf("multiple Go entrypoints were found (%s); choose one during init review", strings.Join(entrypoints, ", "))}, nil
	}
}

func inferPythonStartHint(absPath string) (string, []string) {
	type candidate struct {
		filename string
		hint     string
	}

	candidates := []candidate{
		{filename: "manage.py", hint: "python manage.py runserver"},
		{filename: "main.py", hint: "python main.py"},
		{filename: "app.py", hint: "python app.py"},
	}

	found := make([]candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if exists(filepath.Join(absPath, candidate.filename)) {
			found = append(found, candidate)
		}
	}

	switch len(found) {
	case 0:
		return "", []string{"requirements.txt was found but no obvious Python entrypoint was inferred from manage.py, main.py, or app.py"}
	case 1:
		return found[0].hint, nil
	default:
		names := make([]string, 0, len(found))
		for _, candidate := range found {
			names = append(names, candidate.filename)
		}
		return "", []string{fmt.Sprintf("multiple Python entrypoints were found (%s); choose one during init review", strings.Join(names, ", "))}
	}
}

func inferDockerStartHint(absPath string) (string, []string, error) {
	dockerfilePath := filepath.Join(absPath, string(SignalDockerfile))
	payload, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return "", nil, err
	}

	lines := strings.Split(string(payload), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "CMD "):
			return strings.TrimSpace(trimmed[4:]), []string{"using Dockerfile CMD as the best available start hint"}, nil
		case strings.HasPrefix(upper, "ENTRYPOINT "):
			return strings.TrimSpace(trimmed[len("ENTRYPOINT "):]), []string{"using Dockerfile ENTRYPOINT as the best available start hint"}, nil
		}
	}

	return "", []string{"Dockerfile was found but no CMD or ENTRYPOINT line was inferred for a start hint"}, nil
}

func inferHealthcheck(absPath string) (HealthcheckHint, []string, error) {
	healthPath, pathWarnings, err := inferHealthPath(absPath)
	if err != nil {
		return HealthcheckHint{}, nil, err
	}

	port, portWarnings, err := inferServicePort(absPath)
	if err != nil {
		return HealthcheckHint{}, nil, err
	}

	warnings := append(pathWarnings, portWarnings...)
	if healthPath == "" {
		if port != 0 {
			warnings = append(warnings, fmt.Sprintf("found port %d but no health endpoint path was inferred", port))
		}
		return HealthcheckHint{}, uniqueWarnings(warnings), nil
	}

	hint := HealthcheckHint{Path: healthPath}
	if port != 0 {
		hint.Port = port
	}

	return hint, uniqueWarnings(warnings), nil
}

func inferHealthPath(absPath string) (string, []string, error) {
	files, err := collectTextFiles(absPath)
	if err != nil {
		return "", nil, err
	}

	routePatterns := []struct {
		path    string
		regexps []*regexp.Regexp
	}{
		{
			path: "/healthz",
			regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)\b(GET|POST|PUT|DELETE)\s*\(\s*"/healthz"`),
				regexp.MustCompile(`(?i)\bHandleFunc\s*\(\s*"/healthz"`),
				regexp.MustCompile(`(?i)\b(app|router)\.(get|post|put|delete)\s*\(\s*"/healthz"`),
				regexp.MustCompile(`(?i)@(?:app|router)\.(?:get|route)\s*\(\s*"/healthz"`),
			},
		},
		{
			path: "/health",
			regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)\b(GET|POST|PUT|DELETE)\s*\(\s*"/health"`),
				regexp.MustCompile(`(?i)\bHandleFunc\s*\(\s*"/health"`),
				regexp.MustCompile(`(?i)\b(app|router)\.(get|post|put|delete)\s*\(\s*"/health"`),
				regexp.MustCompile(`(?i)@(?:app|router)\.(?:get|route)\s*\(\s*"/health"`),
			},
		},
		{
			path: "/readyz",
			regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)\b(GET|POST|PUT|DELETE)\s*\(\s*"/readyz"`),
				regexp.MustCompile(`(?i)\bHandleFunc\s*\(\s*"/readyz"`),
				regexp.MustCompile(`(?i)\b(app|router)\.(get|post|put|delete)\s*\(\s*"/readyz"`),
				regexp.MustCompile(`(?i)@(?:app|router)\.(?:get|route)\s*\(\s*"/readyz"`),
			},
		},
		{
			path: "/ready",
			regexps: []*regexp.Regexp{
				regexp.MustCompile(`(?i)\b(GET|POST|PUT|DELETE)\s*\(\s*"/ready"`),
				regexp.MustCompile(`(?i)\bHandleFunc\s*\(\s*"/ready"`),
				regexp.MustCompile(`(?i)\b(app|router)\.(get|post|put|delete)\s*\(\s*"/ready"`),
				regexp.MustCompile(`(?i)@(?:app|router)\.(?:get|route)\s*\(\s*"/ready"`),
			},
		},
	}

	matches := []healthPathMatch{}
	seen := map[string]struct{}{}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return "", nil, err
		}

		for _, pattern := range routePatterns {
			if _, exists := seen[pattern.path]; exists {
				continue
			}
			for _, regex := range pattern.regexps {
				if !regex.Match(content) {
					continue
				}
				seen[pattern.path] = struct{}{}
				matches = append(matches, healthPathMatch{path: pattern.path, file: file})
				break
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", nil, nil
	case 1:
		return matches[0].path, nil, nil
	default:
		best := chooseHealthPath(matches)
		options := make([]string, 0, len(matches))
		for _, match := range matches {
			options = append(options, match.path)
		}
		sort.Strings(options)
		return best, []string{fmt.Sprintf("multiple health endpoints were inferred (%s); using %s as the best match", strings.Join(uniqueStrings(options), ", "), best)}, nil
	}
}

func chooseHealthPath(matches []healthPathMatch) string {
	priority := map[string]int{
		"/healthz": 0,
		"/health":  1,
		"/readyz":  2,
		"/ready":   3,
	}

	best := matches[0].path
	bestPriority := priority[best]
	for _, match := range matches[1:] {
		if priority[match.path] < bestPriority {
			best = match.path
			bestPriority = priority[match.path]
		}
	}
	return best
}

func inferServicePort(absPath string) (int, []string, error) {
	serverPort, _, err := inferPortFromPatterns(absPath, []*regexp.Regexp{
		regexp.MustCompile(`\bSERVER_PORT"\s*,\s*"(\d{2,5})"`),
		regexp.MustCompile(`\bHTTP_PORT"\s*,\s*"(\d{2,5})"`),
		regexp.MustCompile(`\bAPP_PORT"\s*,\s*"(\d{2,5})"`),
		regexp.MustCompile(`\bPORT"\s*,\s*"(\d{2,5})"`),
		regexp.MustCompile(`ListenAndServe\(\s*":(\d{2,5})"`),
	})
	if err != nil {
		return 0, nil, err
	}
	if serverPort != 0 {
		return serverPort, nil, nil
	}

	dockerfilePort, err := inferDockerfilePort(absPath)
	if err != nil {
		return 0, nil, err
	}
	if dockerfilePort != 0 {
		return dockerfilePort, nil, nil
	}

	port, warnings, err := inferPortFromPatterns(absPath, []*regexp.Regexp{
		regexp.MustCompile(`:(\d{2,5})"`),
	})
	if err != nil {
		return 0, nil, err
	}
	return port, warnings, nil
}

func inferPortFromPatterns(absPath string, portPatterns []*regexp.Regexp) (int, []string, error) {
	files, err := collectTextFiles(absPath)
	if err != nil {
		return 0, nil, err
	}

	ports := []int{}
	seen := map[int]struct{}{}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return 0, nil, err
		}
		for _, pattern := range portPatterns {
			matches := pattern.FindAllStringSubmatch(string(content), -1)
			for _, match := range matches {
				port, convErr := strconv.Atoi(match[1])
				if convErr != nil {
					continue
				}
				if _, exists := seen[port]; exists {
					continue
				}
				seen[port] = struct{}{}
				ports = append(ports, port)
			}
		}
	}

	switch len(ports) {
	case 0:
		return 0, nil, nil
	case 1:
		return ports[0], nil, nil
	default:
		sort.Ints(ports)
		return ports[0], []string{fmt.Sprintf("multiple candidate ports were inferred (%s); using %d as the lowest stable guess", intsToString(ports), ports[0])}, nil
	}
}

func inferDockerfilePort(absPath string) (int, error) {
	dockerfilePath := filepath.Join(absPath, string(SignalDockerfile))
	if !exists(dockerfilePath) {
		return 0, nil
	}

	payload, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return 0, err
	}

	exposePattern := regexp.MustCompile(`(?i)^EXPOSE\s+(\d{2,5})`)
	lines := strings.Split(string(payload), "\n")
	for _, line := range lines {
		match := exposePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) < 2 {
			continue
		}
		port, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		return port, nil
	}

	return 0, nil
}

func collectTextFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if shouldSkipFile(entry.Name()) {
			return nil
		}
		if !isLikelyTextFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func isLikelyTextFile(path string) bool {
	name := filepath.Base(path)
	switch name {
	case "Dockerfile", "package.json", "requirements.txt", "go.mod":
		return true
	}

	switch filepath.Ext(path) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".java":
		return true
	default:
		return false
	}
}

func shouldSkipFile(name string) bool {
	return strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.py") || strings.HasSuffix(name, ".snap")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasSignal(signals []ServiceSignal, expected ServiceSignal) bool {
	for _, signal := range signals {
		if signal == expected {
			return true
		}
	}
	return false
}

func uniqueWarnings(warnings []string) []string {
	return uniqueStrings(filterNonEmpty(warnings))
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func filterNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func intsToString(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}
