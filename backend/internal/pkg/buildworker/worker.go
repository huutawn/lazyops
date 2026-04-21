package buildworker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Worker struct {
	cfg      config.Config
	db       *gorm.DB
	running  bool
	mu       sync.Mutex
	shutdown chan struct{}
}

func New(cfg config.Config) (*Worker, error) {
	if !cfg.BuildWorker.Enabled {
		return nil, fmt.Errorf("build worker is not enabled (set BUILD_WORKER_ENABLED=true)")
	}
	if cfg.BuildWorker.RegistryHost == "" {
		return nil, fmt.Errorf("BUILD_REGISTRY_HOST is required")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.Name, cfg.Database.SSLMode, cfg.Database.TimeZone,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	if err := os.MkdirAll(cfg.BuildWorker.WorkspaceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}

	return &Worker{
		cfg:      cfg,
		db:       db,
		shutdown: make(chan struct{}),
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	w.mu.Lock()
	w.running = true
	w.mu.Unlock()

	slog.Info("build worker started polling",
		"interval", w.cfg.BuildWorker.PollInterval,
		"max_concurrency", w.cfg.BuildWorker.MaxConcurrency,
	)

	ticker := time.NewTicker(w.cfg.BuildWorker.PollInterval)
	defer ticker.Stop()

	sem := make(chan struct{}, w.cfg.BuildWorker.MaxConcurrency)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			var jobs []models.BuildJob
			if err := w.db.Where("status = ?", "queued").
				Order("created_at ASC").
				Limit(w.cfg.BuildWorker.MaxConcurrency).
				Find(&jobs).Error; err != nil {
				slog.Error("failed to poll build jobs", "error", err)
				continue
			}
			if len(jobs) == 0 {
				continue
			}
			for _, job := range jobs {
				select {
				case sem <- struct{}{}:
					go func(j models.BuildJob) {
						defer func() { <-sem }()
						w.processJob(context.Background(), j)
					}(job)
				default:
					// All workers busy, will pick up on next tick
				}
			}
		}
	}
}

func (w *Worker) Shutdown() {
	w.mu.Lock()
	w.running = false
	w.mu.Unlock()
	select {
	case <-w.shutdown:
	default:
		close(w.shutdown)
	}
}

func (w *Worker) processJob(ctx context.Context, job models.BuildJob) {
	slog.Info("processing build job",
		"job_id", job.ID,
		"repo", job.RepoFullName,
		"commit", job.CommitSHA,
		"branch", job.TrackedBranch,
	)

	// Mark as running
	now := time.Now().UTC()
	if err := w.db.Model(&models.BuildJob{}).
		Where("id = ? AND status = ?", job.ID, "queued").
		Updates(map[string]any{
			"status":     "running",
			"started_at": now,
			"updated_at": now,
		}).Error; err != nil {
		slog.Error("failed to update job status to running", "job_id", job.ID, "error", err)
		return
	}

	// Parse worker input
	var input BuildWorkerInput
	if err := json.Unmarshal([]byte(job.WorkerInputJSON), &input); err != nil {
		w.failJob(job.ID, fmt.Sprintf("parse worker input: %v", err))
		return
	}
	slog.Info("build job worker input parsed",
		"job_id", job.ID,
		"project_id", input.ProjectID,
		"service_target_count", len(input.ServiceTargets),
		"service_targets", normalizeBuildTargets(input.ServiceTargets),
	)

	// Build
	result, err := w.buildAndPush(ctx, input)

	status := "succeeded"
	if err != nil {
		status = "failed"
		slog.Error("build job failed", "job_id", job.ID, "error", err)
	}
	if strings.TrimSpace(result.CommitSHA) != "" {
		input.CommitSHA = strings.TrimSpace(result.CommitSHA)
	} else if strings.TrimSpace(input.CommitSHA) == "" {
		input.CommitSHA = fallbackBuildCallbackCommit(job, input)
	}

	// Callback
	if err := w.callback(ctx, input, status, result); err != nil {
		slog.Error("build callback failed", "job_id", job.ID, "error", err)
		artifactMetadataJSON := marshalBuildArtifactMetadata(result, strings.TrimSpace(input.CommitSHA), err.Error())
		w.failJobWithArtifact(job.ID, fmt.Sprintf("callback failed: %v", err), artifactMetadataJSON)
		return
	}

	slog.Info("build job completed", "job_id", job.ID, "status", status, "image", result.ImageRef)
}

type BuildWorkerInput struct {
	BuildJobID           string                       `json:"build_job_id"`
	ProjectID            string                       `json:"project_id"`
	ProjectRepoLinkID    string                       `json:"project_repo_link_id"`
	GitHubDeliveryID     string                       `json:"github_delivery_id"`
	GitHubInstallationID int64                        `json:"github_installation_id"`
	GitHubRepoID         int64                        `json:"github_repo_id"`
	RepoOwner            string                       `json:"repo_owner"`
	RepoName             string                       `json:"repo_name"`
	RepoFullName         string                       `json:"repo_full_name"`
	TrackedBranch        string                       `json:"tracked_branch"`
	CommitSHA            string                       `json:"commit_sha"`
	TriggerKind          string                       `json:"trigger_kind"`
	PullRequestNumber    int                          `json:"pull_request_number"`
	PreviewEnabled       bool                         `json:"preview_enabled"`
	ServiceTargets       []BuildTargetServiceMetadata `json:"service_targets,omitempty"`
}

type BuildTargetServiceMetadata struct {
	ServiceName         string         `json:"service_name"`
	ServicePath         string         `json:"service_path"`
	RuntimeProfile      string         `json:"runtime_profile,omitempty"`
	Public              bool           `json:"public,omitempty"`
	StartHint           string         `json:"start_hint,omitempty"`
	DeclaredTargetPort  int            `json:"declared_target_port,omitempty"`
	DeclaredServicePort int            `json:"declared_service_port,omitempty"`
	DeclaredHealthcheck map[string]any `json:"declared_healthcheck,omitempty"`
}

type BuildServiceArtifactMetadata struct {
	ServiceName             string                             `json:"service_name"`
	ServicePath             string                             `json:"service_path"`
	ImageRef                string                             `json:"image_ref"`
	ImageDigest             string                             `json:"image_digest"`
	DetectedPorts           []BuildDetectedPortMetadata        `json:"detected_ports,omitempty"`
	PortDetectionSource     string                             `json:"port_detection_source,omitempty"`
	PortDetectionConfidence string                             `json:"port_detection_confidence,omitempty"`
	SuggestedTargetPort     int                                `json:"suggested_target_port,omitempty"`
	DetectedFramework       string                             `json:"detected_framework,omitempty"`
	SuggestedHealthcheck    *BuildSuggestedHealthcheckMetadata `json:"suggested_healthcheck,omitempty"`
	PortResolutionStatus    string                             `json:"port_resolution_status,omitempty"`
	PortResolutionSource    string                             `json:"port_resolution_source,omitempty"`
	PortResolutionReason    string                             `json:"port_resolution_reason,omitempty"`
	CandidatePorts          []int                              `json:"candidate_ports,omitempty"`
}

type buildResult struct {
	CommitSHA               string
	ImageRef                string
	ImageDigest             string
	ServiceArtifacts        []BuildServiceArtifactMetadata
	Services                []string
	DetectedPorts           []BuildDetectedPortMetadata
	PortDetectionSource     string
	PortDetectionConfidence string
	SuggestedTargetPort     int
	DetectedFramework       string
	SuggestedHealthcheck    *BuildSuggestedHealthcheckMetadata
	PortResolutionStatus    string
	PortResolutionSource    string
	PortResolutionReason    string
	CandidatePorts          []int
}

type buildPortResolution struct {
	Status                  string
	Source                  string
	Reason                  string
	CandidatePorts          []int
	SuggestedTargetPort     int
	SuggestedHealthcheck    *BuildSuggestedHealthcheckMetadata
	PortDetectionConfidence string
}

type buildInvocationMetadata struct {
	UsedNixpacks      bool
	NixpacksPlanStart string
	NixpacksPlanError string
}

const (
	buildPortResolutionStatusResolved   = "resolved"
	buildPortResolutionStatusAmbiguous  = "ambiguous"
	buildPortResolutionStatusUnresolved = "unresolved"

	buildPortResolutionSourceExplicit      = "explicit"
	buildPortResolutionSourceNixpacksPlan  = "nixpacks_plan"
	buildPortResolutionSourceDockerInspect = "docker_inspect"
	buildPortResolutionSourceFrameworkHint = "framework_hint"
	buildPortResolutionSourceSmokeRun      = "smoke_run"
	buildPortResolutionSourceStartHint     = "start_hint"
	buildPortResolutionSourceMixed         = "mixed"
	buildPortResolutionSourceInternal      = "internal_default"
)

func (w *Worker) buildAndPush(
	ctx context.Context,
	input BuildWorkerInput,
) (buildResult, error) {
	// Clone repo
	repoDir, resolvedCommitSHA, err := w.cloneRepo(ctx, input)
	if err != nil {
		return buildResult{}, fmt.Errorf("clone repo: %w", err)
	}
	defer os.RemoveAll(repoDir)
	if strings.TrimSpace(resolvedCommitSHA) != "" {
		input.CommitSHA = strings.TrimSpace(resolvedCommitSHA)
	}

	// Login to registry
	if err := w.dockerLogin(ctx); err != nil {
		return buildResult{}, fmt.Errorf("docker login: %w", err)
	}

	repoServiceTargets := normalizeBuildTargets(input.ServiceTargets)
	if len(repoServiceTargets) > 0 {
		result := buildResult{
			CommitSHA:        input.CommitSHA,
			Services:         make([]string, 0, len(repoServiceTargets)),
			ServiceArtifacts: make([]BuildServiceArtifactMetadata, 0, len(repoServiceTargets)),
		}
		for _, target := range repoServiceTargets {
			serviceDir, err := resolveServiceBuildDir(repoDir, target.ServicePath)
			if err != nil {
				return buildResult{}, err
			}
			imageName := w.imageNameForService(input, target.ServiceName)
			buildMeta, err := w.buildImage(ctx, serviceDir, imageName)
			if err != nil {
				return buildResult{}, err
			}
			slog.Info("pushing docker image", "image", imageName, "service", target.ServiceName)
			pushCmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "push", imageName)
			pushOutput, err := pushCmd.CombinedOutput()
			if err != nil {
				return buildResult{}, fmt.Errorf("docker push %s: %s: %w", target.ServiceName, string(pushOutput), err)
			}

			digest, _ := w.getImageDigest(ctx, imageName)
			detectedFramework, suggestedHealthcheck := w.detectFrontendMetadata(serviceDir)
			exposedPorts, _, _ := w.inspectImagePorts(ctx, imageName)
			smokePorts, _ := w.smokeRunImagePorts(ctx, imageName)
			detectedPorts, portDetectionSource := mergeDetectedPorts(exposedPorts, smokePorts)
			resolution := resolveRepoServiceBuildPort(target, detectedPorts, suggestedHealthcheck, buildMeta)
			result.Services = append(result.Services, target.ServiceName)
			result.ServiceArtifacts = append(result.ServiceArtifacts, BuildServiceArtifactMetadata{
				ServiceName:             target.ServiceName,
				ServicePath:             target.ServicePath,
				ImageRef:                imageName,
				ImageDigest:             digest,
				DetectedPorts:           detectedPorts,
				PortDetectionSource:     portDetectionSource,
				PortDetectionConfidence: resolution.PortDetectionConfidence,
				SuggestedTargetPort:     resolution.SuggestedTargetPort,
				DetectedFramework:       detectedFramework,
				SuggestedHealthcheck:    resolution.SuggestedHealthcheck,
				PortResolutionStatus:    resolution.Status,
				PortResolutionSource:    resolution.Source,
				PortResolutionReason:    resolution.Reason,
				CandidatePorts:          cloneIntSlice(resolution.CandidatePorts),
			})
		}
		return result, nil
	}

	imageName := w.imageName(input, shortCommitTag(input.CommitSHA))
	if _, err := w.buildImage(ctx, repoDir, imageName); err != nil {
		return buildResult{}, err
	}

	slog.Info("pushing docker image", "image", imageName)
	pushCmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "push", imageName)
	pushOutput, err := pushCmd.CombinedOutput()
	if err != nil {
		return buildResult{}, fmt.Errorf("docker push: %s: %w", string(pushOutput), err)
	}

	digest, _ := w.getImageDigest(ctx, imageName)
	services := w.detectServices(repoDir)
	if len(services) == 0 {
		services = []string{"app"}
	}
	detectedFramework, suggestedHealthcheck := w.detectFrontendMetadata(repoDir)
	exposedPorts, _, _ := w.inspectImagePorts(ctx, imageName)
	smokePorts, smokeReason := w.smokeRunImagePorts(ctx, imageName)
	detectedPorts, portDetectionSource := mergeDetectedPorts(exposedPorts, smokePorts)
	resolution := resolveBuildPort(BuildTargetServiceMetadata{}, services, detectedPorts, smokePorts, detectedFramework, suggestedHealthcheck, smokeReason, false)

	return buildResult{
		CommitSHA:               input.CommitSHA,
		ImageRef:                imageName,
		ImageDigest:             digest,
		Services:                services,
		DetectedPorts:           detectedPorts,
		PortDetectionSource:     portDetectionSource,
		PortDetectionConfidence: resolution.PortDetectionConfidence,
		SuggestedTargetPort:     resolution.SuggestedTargetPort,
		DetectedFramework:       detectedFramework,
		SuggestedHealthcheck:    resolution.SuggestedHealthcheck,
		PortResolutionStatus:    resolution.Status,
		PortResolutionSource:    resolution.Source,
		PortResolutionReason:    resolution.Reason,
		CandidatePorts:          cloneIntSlice(resolution.CandidatePorts),
	}, nil
}

func (w *Worker) imageName(input BuildWorkerInput, tag string) string {
	registryHost := strings.TrimSpace(w.cfg.BuildWorker.RegistryHost)
	namespace := strings.TrimSpace(w.cfg.BuildWorker.RegistryUser)
	if namespace == "" {
		namespace = strings.TrimSpace(input.RepoOwner)
	}
	if namespace == "" {
		namespace = "lazyops"
	}

	repoName := strings.TrimSpace(input.ProjectID)
	if repoName == "" {
		repoName = strings.TrimSpace(input.RepoName)
	}
	if repoName == "" {
		repoName = "app"
	}

	return fmt.Sprintf("%s/%s/%s:%s",
		strings.ToLower(registryHost),
		strings.ToLower(namespace),
		strings.ToLower(repoName),
		strings.ToLower(strings.TrimSpace(tag)),
	)
}

func (w *Worker) imageNameForService(input BuildWorkerInput, serviceName string) string {
	repoName := strings.TrimSpace(input.ProjectID)
	if repoName == "" {
		repoName = strings.TrimSpace(input.RepoName)
	}
	if repoName == "" {
		repoName = "app"
	}
	serviceName = strings.ToLower(strings.TrimSpace(serviceName))
	serviceName = strings.ReplaceAll(serviceName, "_", "-")
	serviceName = strings.ReplaceAll(serviceName, ".", "-")
	serviceName = strings.ReplaceAll(serviceName, "/", "-")
	if serviceName == "" {
		serviceName = "app"
	}
	return w.imageName(BuildWorkerInput{
		ProjectID: "",
		RepoOwner: input.RepoOwner,
		RepoName:  repoName + "-" + serviceName,
	}, shortCommitTag(input.CommitSHA))
}

func (w *Worker) buildImage(ctx context.Context, repoDir, imageName string) (buildInvocationMetadata, error) {
	if _, err := exec.LookPath(w.cfg.BuildWorker.NixpacksBin); err == nil {
		planStart, planErr := w.inspectNixpacksPlan(ctx, repoDir)
		slog.Info("running nixpacks build", "dir", repoDir, "image", imageName)
		cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.NixpacksBin, "build", repoDir, "-t", imageName)
		cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			return buildInvocationMetadata{}, fmt.Errorf("nixpacks build: %w", runErr)
		}
		return buildInvocationMetadata{
			UsedNixpacks:      true,
			NixpacksPlanStart: planStart,
			NixpacksPlanError: planErr,
		}, nil
	}

	dockerfilePath := filepath.Join(repoDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		slog.Warn("nixpacks not found; falling back to docker build", "image", imageName)
		cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "build", "-t", imageName, repoDir)
		cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			return buildInvocationMetadata{}, fmt.Errorf("docker build fallback: %w", runErr)
		}
		return buildInvocationMetadata{}, nil
	}

	return buildInvocationMetadata{}, fmt.Errorf("nixpacks not found and Dockerfile missing at repository root")
}

func (w *Worker) inspectNixpacksPlan(ctx context.Context, repoDir string) (string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.NixpacksBin, "plan", repoDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", formatNixpacksPlanError(stdout.String(), stderr.String(), err)
	}

	planJSON, err := extractJSONDocument(stdout.Bytes())
	if err != nil {
		return "", fmt.Sprintf("nixpacks plan output was not parseable JSON: %v", err)
	}

	var plan struct {
		Start struct {
			Cmd string `json:"cmd"`
		} `json:"start"`
	}
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return "", fmt.Sprintf("nixpacks plan output was not parseable JSON: %v", err)
	}

	startCmd := strings.TrimSpace(plan.Start.Cmd)
	if startCmd == "" {
		return "", "nixpacks plan did not define start.cmd"
	}
	return startCmd, ""
}

func extractJSONDocument(payload []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty output")
	}
	if json.Valid(trimmed) {
		return trimmed, nil
	}

	start := bytes.IndexByte(trimmed, '{')
	end := bytes.LastIndexByte(trimmed, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("JSON object not found in output")
	}

	candidate := bytes.TrimSpace(trimmed[start : end+1])
	if !json.Valid(candidate) {
		return nil, fmt.Errorf("JSON object not found in output")
	}
	return candidate, nil
}

func formatNixpacksPlanError(stdout, stderr string, err error) string {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = strings.TrimSpace(stdout)
	}
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Sprintf("nixpacks plan failed: %s", detail)
}

func (w *Worker) cloneRepo(ctx context.Context, input BuildWorkerInput) (string, string, error) {
	dir, err := os.MkdirTemp(w.cfg.BuildWorker.WorkspaceDir, fmt.Sprintf("build-%s-*", input.BuildJobID))
	if err != nil {
		return "", "", err
	}

	repoURL, err := w.repoCloneURL(ctx, input)
	if err != nil {
		return "", "", err
	}

	slog.Info("cloning repo",
		"repo_full_name", strings.TrimSpace(input.RepoFullName),
		"tracked_branch", strings.TrimSpace(input.TrackedBranch),
		"commit", input.CommitSHA,
	)

	cloneArgs := []string{"clone", "--depth", "1"}
	if strings.TrimSpace(input.CommitSHA) == "" && strings.TrimSpace(input.TrackedBranch) != "" {
		cloneArgs = append(cloneArgs, "--branch", strings.TrimSpace(input.TrackedBranch))
	}
	cloneArgs = append(cloneArgs, repoURL, dir)
	cloneCmd := exec.CommandContext(ctx, "git", cloneArgs...)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git clone: %s: %w", string(output), err)
	}
	if commitSHA := strings.TrimSpace(input.CommitSHA); commitSHA != "" {
		fetchCmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin", commitSHA)
		if output, err := fetchCmd.CombinedOutput(); err != nil {
			return "", "", fmt.Errorf("git fetch %s: %s: %w", commitSHA, string(output), err)
		}
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", dir, "checkout", "--detach", "FETCH_HEAD")
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			return "", "", fmt.Errorf("git checkout %s: %s: %w", commitSHA, string(output), err)
		}
	}

	resolvedCommitSHA, err := resolveGitHeadCommit(ctx, dir)
	if err != nil {
		return "", "", err
	}

	return dir, resolvedCommitSHA, nil
}

func (w *Worker) repoCloneURL(ctx context.Context, input BuildWorkerInput) (string, error) {
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", input.RepoOwner, input.RepoName)
	if input.GitHubInstallationID <= 0 {
		return repoURL, nil
	}
	if strings.TrimSpace(w.cfg.GitHubApp.AppID) == "" || strings.TrimSpace(w.cfg.GitHubApp.PrivateKey) == "" {
		return repoURL, nil
	}

	token, err := w.createGitHubInstallationAccessToken(ctx, input.GitHubInstallationID)
	if err != nil {
		return "", fmt.Errorf("github installation token: %w", err)
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	parsed.User = url.UserPassword("x-access-token", token)
	return parsed.String(), nil
}

func (w *Worker) createGitHubInstallationAccessToken(ctx context.Context, installationID int64) (string, error) {
	appID := strings.TrimSpace(w.cfg.GitHubApp.AppID)
	privateKey := strings.TrimSpace(w.cfg.GitHubApp.PrivateKey)
	if appID == "" || privateKey == "" || installationID <= 0 {
		return "", fmt.Errorf("github app credentials unavailable")
	}

	privateKey = strings.ReplaceAll(privateKey, "\\n", "\n")
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    appID,
		IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Minute)),
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKey))
	if err != nil {
		return "", err
	}
	signed, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+signed)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("github api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Token) == "" {
		return "", fmt.Errorf("github installation token missing from response")
	}
	return strings.TrimSpace(payload.Token), nil
}

func resolveGitHeadCommit(ctx context.Context, repoDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %s: %w", string(output), err)
	}
	commitSHA := strings.TrimSpace(string(output))
	if commitSHA == "" {
		return "", fmt.Errorf("git rev-parse HEAD returned empty commit")
	}
	return commitSHA, nil
}

func fallbackBuildCallbackCommit(job models.BuildJob, input BuildWorkerInput) string {
	if commitSHA := strings.TrimSpace(job.CommitSHA); commitSHA != "" {
		return commitSHA
	}
	if commitSHA := strings.TrimSpace(input.CommitSHA); commitSHA != "" {
		return commitSHA
	}
	if branch := strings.TrimSpace(input.TrackedBranch); branch != "" {
		return branch
	}
	return "unknown"
}

func (w *Worker) dockerLogin(ctx context.Context) error {
	if w.cfg.BuildWorker.RegistryUser == "" || w.cfg.BuildWorker.RegistryPass == "" {
		return nil // No credentials, skip login
	}
	cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "login",
		w.cfg.BuildWorker.RegistryHost,
		"-u", w.cfg.BuildWorker.RegistryUser,
		"--password-stdin",
	)
	cmd.Stdin = strings.NewReader(w.cfg.BuildWorker.RegistryPass)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker login: %s", string(output))
	}
	return nil
}

func (w *Worker) getImageDigest(ctx context.Context, imageName string) (string, error) {
	cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "inspect", imageName, "--format", "{{index .RepoDigests 0}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	digest := strings.TrimSpace(string(output))
	// Extract just the sha256 part
	if idx := strings.Index(digest, "@sha256:"); idx >= 0 {
		return "sha256:" + digest[idx+len("@sha256:"):], nil
	}
	return digest, nil
}

func (w *Worker) detectServices(repoDir string) []string {
	var services []string

	// Check for common service indicators
	indicators := map[string]string{
		"package.json":     "node",
		"requirements.txt": "python",
		"go.mod":           "go",
		"Cargo.toml":       "rust",
		"pom.xml":          "java",
		"build.gradle":     "java",
		"Gemfile":          "ruby",
		"composer.json":    "php",
		"Dockerfile":       "docker",
	}

	for file, svcType := range indicators {
		if _, err := os.Stat(filepath.Join(repoDir, file)); err == nil {
			services = append(services, svcType)
		}
	}

	if len(services) == 0 {
		// Check nixpacks detection output
		planPath := filepath.Join(repoDir, ".nixpacks", "plan.json")
		if data, err := os.ReadFile(planPath); err == nil {
			var plan struct {
				Phases map[string]struct {
					Cmds []string `json:"cmds"`
				} `json:"phases"`
			}
			if json.Unmarshal(data, &plan) == nil {
				for _, phase := range plan.Phases {
					for _, cmd := range phase.Cmds {
						if strings.Contains(cmd, "npm") || strings.Contains(cmd, "yarn") {
							services = append(services, "node")
						} else if strings.Contains(cmd, "pip") || strings.Contains(cmd, "python") {
							services = append(services, "python")
						}
					}
				}
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, s := range services {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	return unique
}

type BuildSuggestedHealthcheckMetadata struct {
	Path string `json:"path"`
	Port int    `json:"port"`
}

type BuildDetectedPortMetadata struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Name     string `json:"name,omitempty"`
	Exposed  bool   `json:"exposed,omitempty"`
}

func (w *Worker) detectFrontendMetadata(repoDir string) (string, *BuildSuggestedHealthcheckMetadata) {
	packageJSONPath := filepath.Join(repoDir, "package.json")
	payload, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return "", nil
	}

	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return "", nil
	}

	hasDep := func(name string) bool {
		if manifest.Dependencies != nil {
			if _, ok := manifest.Dependencies[name]; ok {
				return true
			}
		}
		if manifest.DevDependencies != nil {
			if _, ok := manifest.DevDependencies[name]; ok {
				return true
			}
		}
		return false
	}

	switch {
	case hasDep("next"):
		return "next", &BuildSuggestedHealthcheckMetadata{Path: "/", Port: 3000}
	case hasDep("vite"):
		return "vite", &BuildSuggestedHealthcheckMetadata{Path: "/", Port: 3000}
	case hasDep("react-scripts"):
		return "react-scripts", &BuildSuggestedHealthcheckMetadata{Path: "/", Port: 3000}
	default:
		return "", nil
	}
}

func (w *Worker) callback(
	ctx context.Context,
	input BuildWorkerInput,
	status string,
	result buildResult,
) error {
	if result.Services == nil {
		result.Services = []string{}
	}

	body := map[string]any{
		"build_job_id": input.BuildJobID,
		"project_id":   input.ProjectID,
		"commit_sha":   input.CommitSHA,
		"status":       status,
	}

	if status == "succeeded" {
		metadata := map[string]any{
			"detected_services": result.Services,
		}
		if len(result.ServiceArtifacts) > 0 {
			metadata["service_artifacts"] = result.ServiceArtifacts
		}
		if result.ImageRef != "" {
			body["image_ref"] = result.ImageRef
		}
		if result.ImageDigest != "" {
			body["image_digest"] = result.ImageDigest
		}
		if len(result.DetectedPorts) > 0 {
			metadata["detected_ports"] = result.DetectedPorts
		}
		if strings.TrimSpace(result.PortDetectionSource) != "" {
			metadata["port_detection_source"] = strings.TrimSpace(result.PortDetectionSource)
		}
		if strings.TrimSpace(result.PortDetectionConfidence) != "" {
			metadata["port_detection_confidence"] = strings.TrimSpace(result.PortDetectionConfidence)
		}
		if result.SuggestedTargetPort > 0 {
			metadata["suggested_target_port"] = result.SuggestedTargetPort
		}
		if strings.TrimSpace(result.DetectedFramework) != "" {
			metadata["detected_framework"] = strings.TrimSpace(result.DetectedFramework)
		}
		if result.SuggestedHealthcheck != nil {
			metadata["suggested_healthcheck"] = result.SuggestedHealthcheck
		}
		if strings.TrimSpace(result.PortResolutionStatus) != "" {
			metadata["port_resolution_status"] = strings.TrimSpace(result.PortResolutionStatus)
		}
		if strings.TrimSpace(result.PortResolutionSource) != "" {
			metadata["port_resolution_source"] = strings.TrimSpace(result.PortResolutionSource)
		}
		if strings.TrimSpace(result.PortResolutionReason) != "" {
			metadata["port_resolution_reason"] = strings.TrimSpace(result.PortResolutionReason)
		}
		if len(result.CandidatePorts) > 0 {
			metadata["candidate_ports"] = cloneIntSlice(result.CandidatePorts)
		}
		body["metadata"] = metadata
	}

	payload, _ := json.Marshal(body)

	// Call the callback endpoint.
	// Prefer explicit BUILD_WORKER_CALLBACK_BASE_URL; otherwise use an internal-safe default.
	url := w.callbackURL()

	slog.Info("sending build callback",
		"url", url,
		"status", status,
		"image", result.ImageRef,
		"service_artifact_count", len(result.ServiceArtifacts),
		"detected_services", result.Services,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("create callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("callback request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("callback returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func marshalBuildArtifactMetadata(result buildResult, commitSHA string, failureReason string) string {
	payload := struct {
		CommitSHA               string                             `json:"commit_sha"`
		ArtifactRef             string                             `json:"artifact_ref,omitempty"`
		ImageRef                string                             `json:"image_ref,omitempty"`
		ImageDigest             string                             `json:"image_digest,omitempty"`
		ServiceArtifacts        []BuildServiceArtifactMetadata     `json:"service_artifacts,omitempty"`
		DetectedServices        []string                           `json:"detected_services,omitempty"`
		DetectedPorts           []BuildDetectedPortMetadata        `json:"detected_ports,omitempty"`
		PortDetectionSource     string                             `json:"port_detection_source,omitempty"`
		PortDetectionConfidence string                             `json:"port_detection_confidence,omitempty"`
		SuggestedTargetPort     int                                `json:"suggested_target_port,omitempty"`
		DetectedFramework       string                             `json:"detected_framework,omitempty"`
		SuggestedHealthcheck    *BuildSuggestedHealthcheckMetadata `json:"suggested_healthcheck,omitempty"`
		PortResolutionStatus    string                             `json:"port_resolution_status,omitempty"`
		PortResolutionSource    string                             `json:"port_resolution_source,omitempty"`
		PortResolutionReason    string                             `json:"port_resolution_reason,omitempty"`
		CandidatePorts          []int                              `json:"candidate_ports,omitempty"`
	}{
		CommitSHA:               strings.TrimSpace(commitSHA),
		ArtifactRef:             deriveBuildArtifactRef(result.ImageRef, result.ImageDigest),
		ImageRef:                strings.TrimSpace(result.ImageRef),
		ImageDigest:             strings.TrimSpace(result.ImageDigest),
		ServiceArtifacts:        result.ServiceArtifacts,
		DetectedServices:        result.Services,
		DetectedPorts:           result.DetectedPorts,
		PortDetectionSource:     strings.TrimSpace(result.PortDetectionSource),
		PortDetectionConfidence: strings.TrimSpace(result.PortDetectionConfidence),
		SuggestedTargetPort:     result.SuggestedTargetPort,
		DetectedFramework:       strings.TrimSpace(result.DetectedFramework),
		SuggestedHealthcheck:    cloneSuggestedHealthcheck(result.SuggestedHealthcheck),
		PortResolutionStatus:    strings.TrimSpace(result.PortResolutionStatus),
		PortResolutionSource:    strings.TrimSpace(result.PortResolutionSource),
		PortResolutionReason:    strings.TrimSpace(result.PortResolutionReason),
		CandidatePorts:          cloneIntSlice(result.CandidatePorts),
	}
	if payload.PortResolutionReason == "" {
		payload.PortResolutionReason = strings.TrimSpace(failureReason)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func deriveBuildArtifactRef(imageRef, imageDigest string) string {
	imageRef = strings.TrimSpace(imageRef)
	imageDigest = strings.TrimSpace(imageDigest)
	switch {
	case imageRef != "" && imageDigest != "":
		return imageRef + "@" + imageDigest
	case imageRef != "":
		return imageRef
	default:
		return ""
	}
}

func (w *Worker) inspectImagePorts(ctx context.Context, imageName string) ([]BuildDetectedPortMetadata, string, error) {
	cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "inspect", imageName, "--format", "{{json .Config.ExposedPorts}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, "", err
	}

	var exposed map[string]any
	if err := json.Unmarshal(output, &exposed); err != nil {
		return nil, "", err
	}

	ports := make([]BuildDetectedPortMetadata, 0, len(exposed))
	for key := range exposed {
		parts := strings.SplitN(strings.TrimSpace(key), "/", 2)
		if len(parts) == 0 {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || port <= 0 {
			continue
		}
		protocol := "tcp"
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			protocol = strings.ToLower(strings.TrimSpace(parts[1]))
		}
		ports = append(ports, BuildDetectedPortMetadata{
			Port:     port,
			Protocol: protocol,
			Name:     fmt.Sprintf("%d-%s", port, protocol),
			Exposed:  true,
		})
	}

	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Port == ports[j].Port {
			return ports[i].Protocol < ports[j].Protocol
		}
		return ports[i].Port < ports[j].Port
	})
	return ports, "docker_inspect", nil
}

func (w *Worker) smokeRunImagePorts(ctx context.Context, imageName string) ([]int, string) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	containerName := "lazyops-port-scan-" + randomHex(6)
	runCmd := exec.CommandContext(timeoutCtx, w.cfg.BuildWorker.DockerBin, "run", "-d", "--name", containerName, imageName)
	output, err := runCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Sprintf("smoke run failed: %s", strings.TrimSpace(string(output)))
	}

	containerID := strings.TrimSpace(string(output))
	defer w.removeContainer(context.Background(), containerID)

	deadline := time.Now().Add(15 * time.Second)
	stableUntil := time.Time{}
	found := make(map[int]struct{})
	lastReason := ""

	for {
		state, err := w.inspectContainerState(timeoutCtx, containerID)
		if err != nil {
			lastReason = fmt.Sprintf("inspect smoke-run container: %v", err)
			break
		}
		if state.Pid > 0 {
			ports, err := readListeningPortsForPID(state.Pid)
			if err == nil {
				for _, port := range ports {
					found[port] = struct{}{}
				}
				if len(found) > 0 && stableUntil.IsZero() {
					stableUntil = time.Now().Add(1 * time.Second)
				}
			} else {
				lastReason = fmt.Sprintf("inspect listening ports: %v", err)
			}
		}

		if len(found) > 1 {
			break
		}
		if len(found) > 0 && !stableUntil.IsZero() && time.Now().After(stableUntil) {
			break
		}
		if !state.Running {
			if len(found) == 0 {
				reason := strings.TrimSpace(state.Error)
				if reason == "" {
					reason = fmt.Sprintf("container exited before binding a listening port (status=%s exit_code=%d)", strings.TrimSpace(state.Status), state.ExitCode)
				}
				lastReason = reason
			}
			break
		}
		if time.Now().After(deadline) {
			if len(found) == 0 && lastReason == "" {
				lastReason = "timed out waiting for the container to expose a listening port"
			}
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	return normalizeCandidatePorts(mapKeysToInts(found)), strings.TrimSpace(lastReason)
}

type dockerContainerState struct {
	Status   string `json:"Status"`
	Running  bool   `json:"Running"`
	Pid      int    `json:"Pid"`
	ExitCode int    `json:"ExitCode"`
	Error    string `json:"Error"`
}

func (w *Worker) inspectContainerState(ctx context.Context, containerID string) (dockerContainerState, error) {
	cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "inspect", containerID, "--format", "{{json .State}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return dockerContainerState{}, fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	var state dockerContainerState
	if err := json.Unmarshal(output, &state); err != nil {
		return dockerContainerState{}, err
	}
	return state, nil
}

func (w *Worker) removeContainer(ctx context.Context, containerID string) {
	if strings.TrimSpace(containerID) == "" {
		return
	}
	cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "rm", "-f", containerID)
	_, _ = cmd.CombinedOutput()
}

func readListeningPortsForPID(pid int) ([]int, error) {
	if pid <= 0 {
		return nil, nil
	}
	ports := make(map[int]struct{})
	for _, path := range []string{
		filepath.Join("/proc", strconv.Itoa(pid), "net", "tcp"),
		filepath.Join("/proc", strconv.Itoa(pid), "net", "tcp6"),
	} {
		payload, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, port := range parseProcNetListeningPorts(string(payload)) {
			ports[port] = struct{}{}
		}
	}
	return normalizeCandidatePorts(mapKeysToInts(ports)), nil
}

func parseProcNetListeningPorts(payload string) []int {
	lines := strings.Split(strings.TrimSpace(payload), "\n")
	if len(lines) <= 1 {
		return nil
	}
	ports := make(map[int]struct{})
	for _, line := range lines[1:] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 || fields[3] != "0A" {
			continue
		}
		addressParts := strings.SplitN(fields[1], ":", 2)
		if len(addressParts) != 2 {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(addressParts[1]), 16, 32)
		if err != nil || value <= 0 {
			continue
		}
		ports[int(value)] = struct{}{}
	}
	return normalizeCandidatePorts(mapKeysToInts(ports))
}

func resolveRepoServiceBuildPort(
	target BuildTargetServiceMetadata,
	detectedPorts []BuildDetectedPortMetadata,
	suggestedHealthcheck *BuildSuggestedHealthcheckMetadata,
	buildMeta buildInvocationMetadata,
) buildPortResolution {
	if declaredPort, declaredReason, declaredHealthcheck := declaredPortHint(target); declaredPort > 0 {
		reason := declaredReason
		if reason == "" {
			reason = fmt.Sprintf("using declared service port %d", declaredPort)
		}
		return buildPortResolution{
			Status:                  buildPortResolutionStatusResolved,
			Source:                  buildPortResolutionSourceExplicit,
			Reason:                  strings.TrimSpace(reason),
			CandidatePorts:          []int{declaredPort},
			SuggestedTargetPort:     declaredPort,
			SuggestedHealthcheck:    cloneSuggestedHealthcheck(declaredHealthcheck),
			PortDetectionConfidence: "high",
		}
	}

	if buildMeta.UsedNixpacks {
		return resolveBuildPortFromNixpacksPlan(buildMeta.NixpacksPlanStart, buildMeta.NixpacksPlanError, suggestedHealthcheck)
	}

	return resolveBuildPortFromExpose(detectedPorts, suggestedHealthcheck)
}

func resolveBuildPort(
	target BuildTargetServiceMetadata,
	services []string,
	detectedPorts []BuildDetectedPortMetadata,
	smokePorts []int,
	detectedFramework string,
	suggestedHealthcheck *BuildSuggestedHealthcheckMetadata,
	smokeReason string,
	exposeFirst bool,
) buildPortResolution {
	if declaredPort, declaredReason, declaredHealthcheck := declaredPortHint(target); declaredPort > 0 {
		reason := declaredReason
		if reason == "" {
			reason = fmt.Sprintf("using declared service port %d", declaredPort)
		}
		return buildPortResolution{
			Status:                  buildPortResolutionStatusResolved,
			Source:                  buildPortResolutionSourceExplicit,
			Reason:                  strings.TrimSpace(reason),
			CandidatePorts:          []int{declaredPort},
			SuggestedTargetPort:     declaredPort,
			SuggestedHealthcheck:    cloneSuggestedHealthcheck(declaredHealthcheck),
			PortDetectionConfidence: "high",
		}
	}

	if exposeFirst {
		return resolveBuildPortFromExpose(detectedPorts, suggestedHealthcheck)
	}

	candidateSources := map[int]map[string]struct{}{}
	addCandidate := func(port int, source string) {
		if port <= 0 || strings.TrimSpace(source) == "" {
			return
		}
		if _, ok := candidateSources[port]; !ok {
			candidateSources[port] = make(map[string]struct{})
		}
		candidateSources[port][source] = struct{}{}
	}

	if port := selectInternalServicePort(services, detectedPorts); port > 0 {
		addCandidate(port, buildPortResolutionSourceInternal)
	}
	for _, port := range parseStartHintPorts(target.StartHint) {
		addCandidate(port, buildPortResolutionSourceStartHint)
	}
	if suggestedHealthcheck != nil && suggestedHealthcheck.Port > 0 {
		addCandidate(suggestedHealthcheck.Port, buildPortResolutionSourceFrameworkHint)
	}
	for _, item := range detectedPorts {
		if item.Port > 0 && item.Exposed {
			addCandidate(item.Port, buildPortResolutionSourceDockerInspect)
		}
	}
	for _, port := range smokePorts {
		addCandidate(port, buildPortResolutionSourceSmokeRun)
	}

	candidatePorts := make([]int, 0, len(candidateSources))
	for port := range candidateSources {
		candidatePorts = append(candidatePorts, port)
	}
	candidatePorts = normalizeCandidatePorts(candidatePorts)

	if len(candidatePorts) == 0 {
		reason := strings.TrimSpace(smokeReason)
		if reason == "" {
			reason = "no candidate port could be resolved from docker inspect, start hint, framework hint, or smoke run"
		}
		return buildPortResolution{
			Status:         buildPortResolutionStatusUnresolved,
			Source:         "",
			Reason:         reason,
			CandidatePorts: nil,
		}
	}
	if len(candidatePorts) > 1 {
		sourceSet := make(map[string]struct{})
		for _, sources := range candidateSources {
			for source := range sources {
				sourceSet[source] = struct{}{}
			}
		}
		reason := fmt.Sprintf(
			"multiple candidate ports detected (%s); declare target_port/service_port or set a precise healthcheck.port",
			intsToCSV(candidatePorts),
		)
		if extra := strings.TrimSpace(smokeReason); extra != "" {
			reason += "; " + extra
		}
		return buildPortResolution{
			Status:         buildPortResolutionStatusAmbiguous,
			Source:         summarizeSources(sourceSet),
			Reason:         reason,
			CandidatePorts: candidatePorts,
		}
	}

	port := candidatePorts[0]
	portSources := candidateSources[port]
	source := summarizeSources(portSources)
	reason := fmt.Sprintf("resolved port %d from %s", port, strings.ReplaceAll(source, "_", " "))
	if source == "" {
		source = buildPortResolutionSourceMixed
	}

	var healthcheck *BuildSuggestedHealthcheckMetadata
	if suggestedHealthcheck != nil && suggestedHealthcheck.Port == port {
		healthcheck = cloneSuggestedHealthcheck(suggestedHealthcheck)
	}
	return buildPortResolution{
		Status:                  buildPortResolutionStatusResolved,
		Source:                  source,
		Reason:                  reason,
		CandidatePorts:          candidatePorts,
		SuggestedTargetPort:     port,
		SuggestedHealthcheck:    healthcheck,
		PortDetectionConfidence: resolutionConfidenceForSources(portSources),
	}
}

func resolveBuildPortFromNixpacksPlan(
	startCmd string,
	planErr string,
	suggestedHealthcheck *BuildSuggestedHealthcheckMetadata,
) buildPortResolution {
	startCmd = strings.TrimSpace(startCmd)
	if startCmd == "" {
		reason := "nixpacks plan did not expose a single numeric start port"
		if detail := strings.TrimSpace(planErr); detail != "" {
			reason += "; " + detail
		}
		return buildPortResolution{
			Status:         buildPortResolutionStatusUnresolved,
			Reason:         reason,
			CandidatePorts: nil,
		}
	}

	candidatePorts := parseStartHintPorts(startCmd)
	switch len(candidatePorts) {
	case 0:
		return buildPortResolution{
			Status:         buildPortResolutionStatusUnresolved,
			Source:         buildPortResolutionSourceNixpacksPlan,
			Reason:         "nixpacks start command contains no numeric port",
			CandidatePorts: nil,
		}
	case 1:
		port := candidatePorts[0]
		var healthcheck *BuildSuggestedHealthcheckMetadata
		if suggestedHealthcheck != nil && strings.TrimSpace(suggestedHealthcheck.Path) != "" {
			healthcheck = cloneSuggestedHealthcheck(&BuildSuggestedHealthcheckMetadata{
				Path: suggestedHealthcheck.Path,
				Port: port,
			})
		}
		return buildPortResolution{
			Status:                  buildPortResolutionStatusResolved,
			Source:                  buildPortResolutionSourceNixpacksPlan,
			Reason:                  fmt.Sprintf("resolved port %d from nixpacks plan start.cmd", port),
			CandidatePorts:          candidatePorts,
			SuggestedTargetPort:     port,
			SuggestedHealthcheck:    healthcheck,
			PortDetectionConfidence: "high",
		}
	default:
		return buildPortResolution{
			Status:         buildPortResolutionStatusAmbiguous,
			Source:         buildPortResolutionSourceNixpacksPlan,
			Reason:         fmt.Sprintf("nixpacks start command contains multiple candidate ports: %s", intsToCSV(candidatePorts)),
			CandidatePorts: candidatePorts,
		}
	}
}

func resolveBuildPortFromExpose(
	detectedPorts []BuildDetectedPortMetadata,
	suggestedHealthcheck *BuildSuggestedHealthcheckMetadata,
) buildPortResolution {
	exposedTCPPorts := make([]int, 0, len(detectedPorts))
	for _, item := range detectedPorts {
		if item.Port <= 0 || !item.Exposed {
			continue
		}
		if protocol := strings.ToLower(strings.TrimSpace(item.Protocol)); protocol != "" && protocol != "tcp" {
			continue
		}
		exposedTCPPorts = append(exposedTCPPorts, item.Port)
	}
	exposedTCPPorts = normalizeCandidatePorts(exposedTCPPorts)

	switch len(exposedTCPPorts) {
	case 0:
		return buildPortResolution{
			Status:         buildPortResolutionStatusUnresolved,
			Reason:         "image exposes no TCP ports via EXPOSE",
			CandidatePorts: nil,
		}
	case 1:
		port := exposedTCPPorts[0]
		var healthcheck *BuildSuggestedHealthcheckMetadata
		if suggestedHealthcheck != nil && strings.TrimSpace(suggestedHealthcheck.Path) != "" {
			healthcheck = cloneSuggestedHealthcheck(&BuildSuggestedHealthcheckMetadata{
				Path: suggestedHealthcheck.Path,
				Port: port,
			})
		}
		return buildPortResolution{
			Status:                  buildPortResolutionStatusResolved,
			Source:                  buildPortResolutionSourceDockerInspect,
			Reason:                  fmt.Sprintf("resolved port %d from docker inspect EXPOSE metadata", port),
			CandidatePorts:          exposedTCPPorts,
			SuggestedTargetPort:     port,
			SuggestedHealthcheck:    healthcheck,
			PortDetectionConfidence: "high",
		}
	default:
		return buildPortResolution{
			Status:         buildPortResolutionStatusAmbiguous,
			Source:         buildPortResolutionSourceDockerInspect,
			Reason:         fmt.Sprintf("image exposes multiple TCP ports: %s", intsToCSV(exposedTCPPorts)),
			CandidatePorts: exposedTCPPorts,
		}
	}
}

func declaredPortHint(target BuildTargetServiceMetadata) (int, string, *BuildSuggestedHealthcheckMetadata) {
	if target.DeclaredTargetPort > 0 {
		return target.DeclaredTargetPort, fmt.Sprintf("using declared target_port=%d", target.DeclaredTargetPort), declaredHealthcheckHint(target.DeclaredHealthcheck)
	}
	if target.DeclaredServicePort > 0 {
		return target.DeclaredServicePort, fmt.Sprintf("using declared service_port=%d", target.DeclaredServicePort), declaredHealthcheckHint(target.DeclaredHealthcheck)
	}
	if port := intValue(target.DeclaredHealthcheck["port"]); port > 0 {
		healthcheck := declaredHealthcheckHint(target.DeclaredHealthcheck)
		return port, fmt.Sprintf("using declared healthcheck.port=%d", port), healthcheck
	}
	return 0, "", nil
}

func declaredHealthcheckHint(raw map[string]any) *BuildSuggestedHealthcheckMetadata {
	if len(raw) == 0 {
		return nil
	}
	port := intValue(raw["port"])
	if port <= 0 {
		return nil
	}
	path := strings.TrimSpace(stringValue(raw["path"]))
	if path == "" {
		return nil
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return &BuildSuggestedHealthcheckMetadata{
		Path: path,
		Port: port,
	}
}

func resolutionConfidenceForSources(sources map[string]struct{}) string {
	if len(sources) == 0 {
		return ""
	}
	if len(sources) > 1 {
		return "high"
	}
	switch summarizeSources(sources) {
	case buildPortResolutionSourceExplicit, buildPortResolutionSourceSmokeRun, buildPortResolutionSourceInternal:
		return "high"
	case buildPortResolutionSourceDockerInspect, buildPortResolutionSourceFrameworkHint, buildPortResolutionSourceStartHint:
		return "medium"
	default:
		return "low"
	}
}

func selectInternalServicePort(services []string, detectedPorts []BuildDetectedPortMetadata) int {
	lower := make([]string, 0, len(services))
	for _, item := range services {
		lower = append(lower, strings.ToLower(strings.TrimSpace(item)))
	}
	for _, candidate := range []struct {
		Name string
		Port int
	}{
		{Name: "postgres", Port: 5432},
		{Name: "mysql", Port: 3306},
		{Name: "redis", Port: 6379},
		{Name: "rabbitmq", Port: 5672},
	} {
		for _, item := range lower {
			if item == candidate.Name {
				if containsDetectedPort(detectedPorts, candidate.Port) || len(detectedPorts) == 0 {
					return candidate.Port
				}
			}
		}
	}
	return 0
}

func containsDetectedPort(detectedPorts []BuildDetectedPortMetadata, port int) bool {
	for _, item := range detectedPorts {
		if item.Port == port {
			return true
		}
	}
	return false
}

func mergeDetectedPorts(exposedPorts []BuildDetectedPortMetadata, smokePorts []int) ([]BuildDetectedPortMetadata, string) {
	ports := make(map[string]BuildDetectedPortMetadata, len(exposedPorts)+len(smokePorts))
	for _, item := range exposedPorts {
		if item.Port <= 0 {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(item.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		key := fmt.Sprintf("%d/%s", item.Port, protocol)
		item.Protocol = protocol
		item.Exposed = true
		if strings.TrimSpace(item.Name) == "" {
			item.Name = fmt.Sprintf("%d-%s", item.Port, protocol)
		}
		ports[key] = item
	}
	for _, port := range smokePorts {
		if port <= 0 {
			continue
		}
		key := fmt.Sprintf("%d/tcp", port)
		existing, ok := ports[key]
		if ok {
			existing.Exposed = true
			ports[key] = existing
			continue
		}
		ports[key] = BuildDetectedPortMetadata{
			Port:     port,
			Protocol: "tcp",
			Name:     fmt.Sprintf("%d-tcp", port),
			Exposed:  false,
		}
	}

	out := make([]BuildDetectedPortMetadata, 0, len(ports))
	for _, item := range ports {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Port == out[j].Port {
			return out[i].Protocol < out[j].Protocol
		}
		return out[i].Port < out[j].Port
	})

	switch {
	case len(exposedPorts) > 0 && len(smokePorts) > 0:
		return out, buildPortResolutionSourceMixed
	case len(smokePorts) > 0:
		return out, buildPortResolutionSourceSmokeRun
	case len(exposedPorts) > 0:
		return out, buildPortResolutionSourceDockerInspect
	default:
		return out, ""
	}
}

var startHintPortPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|[\s"'` + "`" + `])PORT=(\d{2,5})(?:$|[\s"'` + "`" + `])`),
	regexp.MustCompile(`(?i)--port(?:=|\s+)(\d{2,5})`),
	regexp.MustCompile(`(?i)(?:^|\s)-p\s+(\d{2,5})(?:$|\s)`),
	regexp.MustCompile(`(?i)(?:listen|addr|bind|serve)\S*\s+0\.0\.0\.0:(\d{2,5})`),
}

func parseStartHintPorts(startHint string) []int {
	startHint = strings.TrimSpace(startHint)
	if startHint == "" {
		return nil
	}
	ports := make([]int, 0, 2)
	for _, pattern := range startHintPortPatterns {
		matches := pattern.FindAllStringSubmatch(startHint, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			port, err := strconv.Atoi(strings.TrimSpace(match[1]))
			if err != nil || port <= 0 {
				continue
			}
			ports = append(ports, port)
		}
	}
	return normalizeCandidatePorts(ports)
}

func summarizeSources(sources map[string]struct{}) string {
	if len(sources) == 0 {
		return ""
	}
	if len(sources) == 1 {
		for source := range sources {
			return source
		}
	}
	return buildPortResolutionSourceMixed
}

func normalizeCandidatePorts(items []int) []int {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(items))
	out := make([]int, 0, len(items))
	for _, item := range items {
		if item <= 0 {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Ints(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneIntSlice(items []int) []int {
	if len(items) == 0 {
		return nil
	}
	out := make([]int, len(items))
	copy(out, items)
	return out
}

func cloneSuggestedHealthcheck(item *BuildSuggestedHealthcheckMetadata) *BuildSuggestedHealthcheckMetadata {
	if item == nil {
		return nil
	}
	cloned := *item
	return &cloned
}

func mapKeysToInts(items map[int]struct{}) []int {
	if len(items) == 0 {
		return nil
	}
	out := make([]int, 0, len(items))
	for item := range items {
		out = append(out, item)
	}
	return out
}

func intsToCSV(items []int) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, strconv.Itoa(item))
	}
	return strings.Join(parts, ", ")
}

func intValue(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func stringValue(raw any) string {
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func (w *Worker) callbackURL() string {
	base := strings.TrimSpace(w.cfg.BuildWorker.CallbackBaseURL)
	if base != "" {
		return strings.TrimRight(base, "/") + "/api/v1/builds/callback"
	}

	host := strings.TrimSpace(w.cfg.Server.Host)
	port := strings.TrimSpace(w.cfg.Server.Port)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "127.0.0.1" || host == "localhost" {
		host = "backend"
	}
	if port == "" {
		port = "8080"
	}
	return fmt.Sprintf("http://%s:%s/api/v1/builds/callback", host, port)
}

func shortCommitTag(commitSHA string) string {
	tag := strings.TrimSpace(commitSHA)
	if len(tag) > 12 {
		tag = tag[:12]
	}
	if tag == "" {
		return "latest"
	}
	return tag
}

func normalizeBuildTargets(items []BuildTargetServiceMetadata) []BuildTargetServiceMetadata {
	if len(items) == 0 {
		return nil
	}
	out := make([]BuildTargetServiceMetadata, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.ServiceName)
		path := strings.TrimSpace(item.ServicePath)
		if name == "" || path == "" {
			continue
		}
		key := name + "|" + path
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, BuildTargetServiceMetadata{
			ServiceName:         name,
			ServicePath:         path,
			RuntimeProfile:      strings.TrimSpace(item.RuntimeProfile),
			Public:              item.Public,
			StartHint:           strings.TrimSpace(item.StartHint),
			DeclaredTargetPort:  item.DeclaredTargetPort,
			DeclaredServicePort: item.DeclaredServicePort,
			DeclaredHealthcheck: cloneAnyMap(item.DeclaredHealthcheck),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ServicePath != out[j].ServicePath {
			return out[i].ServicePath < out[j].ServicePath
		}
		return out[i].ServiceName < out[j].ServiceName
	})
	return out
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func resolveServiceBuildDir(repoDir, servicePath string) (string, error) {
	trimmed := strings.TrimSpace(servicePath)
	if trimmed == "" {
		return "", fmt.Errorf("service path is required for monorepo build")
	}
	cleaned := filepath.Clean(trimmed)
	candidate := repoDir
	if cleaned != "." {
		candidate = filepath.Join(repoDir, cleaned)
	}
	relative, err := filepath.Rel(repoDir, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve service path %q: %w", servicePath, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("service path %q escapes repository root", servicePath)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("service path %q not found in repository", servicePath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("service path %q must point to a directory", servicePath)
	}
	return candidate, nil
}

func (w *Worker) failJob(jobID string, reason string) {
	w.failJobWithArtifact(jobID, reason, "")
}

func (w *Worker) failJobWithArtifact(jobID string, reason string, artifactMetadataJSON string) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":       "failed",
		"started_at":   now,
		"completed_at": now,
		"updated_at":   now,
	}
	if strings.TrimSpace(artifactMetadataJSON) != "" {
		updates["artifact_metadata_json"] = artifactMetadataJSON
	}
	w.db.Model(&models.BuildJob{}).Where("id = ?", jobID).Updates(updates)
	slog.Error("build job marked as failed", "job_id", jobID, "reason", reason)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
