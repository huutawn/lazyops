package buildworker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lazyops-server/internal/config"
	"lazyops-server/internal/models"

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

	// Build
	imageRef, imageDigest, services, err := w.buildAndPush(ctx, input)

	status := "succeeded"
	if err != nil {
		status = "failed"
		slog.Error("build job failed", "job_id", job.ID, "error", err)
	}

	// Callback
	if err := w.callback(ctx, input, status, imageRef, imageDigest, services); err != nil {
		slog.Error("build callback failed", "job_id", job.ID, "error", err)
		w.failJob(job.ID, fmt.Sprintf("callback failed: %v", err))
		return
	}

	slog.Info("build job completed", "job_id", job.ID, "status", status, "image", imageRef)
}

type BuildWorkerInput struct {
	BuildJobID           string `json:"build_job_id"`
	ProjectID            string `json:"project_id"`
	ProjectRepoLinkID    string `json:"project_repo_link_id"`
	GitHubDeliveryID     string `json:"github_delivery_id"`
	GitHubInstallationID int64  `json:"github_installation_id"`
	GitHubRepoID         int64  `json:"github_repo_id"`
	RepoOwner            string `json:"repo_owner"`
	RepoName             string `json:"repo_name"`
	RepoFullName         string `json:"repo_full_name"`
	TrackedBranch        string `json:"tracked_branch"`
	CommitSHA            string `json:"commit_sha"`
	TriggerKind          string `json:"trigger_kind"`
	PullRequestNumber    int    `json:"pull_request_number"`
	PreviewEnabled       bool   `json:"preview_enabled"`
}

func (w *Worker) buildAndPush(ctx context.Context, input BuildWorkerInput) (imageRef, imageDigest string, services []string, err error) {
	// Clone repo
	repoDir, err := w.cloneRepo(ctx, input)
	if err != nil {
		return "", "", nil, fmt.Errorf("clone repo: %w", err)
	}
	defer os.RemoveAll(repoDir)

	// Build image tag
	tag := input.CommitSHA
	if len(tag) > 12 {
		tag = tag[:12]
	}
	imageName := w.imageName(input, tag)

	// Login to registry
	if err := w.dockerLogin(ctx); err != nil {
		return "", "", nil, fmt.Errorf("docker login: %w", err)
	}

	// Build with nixpacks when available; fallback to docker build if Dockerfile exists.
	if err := w.buildImage(ctx, repoDir, imageName); err != nil {
		return "", "", nil, err
	}

	// Push image
	slog.Info("pushing docker image", "image", imageName)
	pushCmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "push", imageName)
	pushOutput, err := pushCmd.CombinedOutput()
	if err != nil {
		return "", "", nil, fmt.Errorf("docker push: %s: %w", string(pushOutput), err)
	}

	// Get image digest
	digest, _ := w.getImageDigest(ctx, imageName)

	// Detect services from nixpacks detection
	services = w.detectServices(repoDir)
	if len(services) == 0 {
		services = []string{"app"}
	}

	return imageName, digest, services, nil
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

func (w *Worker) buildImage(ctx context.Context, repoDir, imageName string) error {
	if _, err := exec.LookPath(w.cfg.BuildWorker.NixpacksBin); err == nil {
		slog.Info("running nixpacks build", "dir", repoDir, "image", imageName)
		cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.NixpacksBin, "build", repoDir, "-t", imageName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("nixpacks build: %w", runErr)
		}
		return nil
	}

	dockerfilePath := filepath.Join(repoDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		slog.Warn("nixpacks not found; falling back to docker build", "image", imageName)
		cmd := exec.CommandContext(ctx, w.cfg.BuildWorker.DockerBin, "build", "-t", imageName, repoDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("docker build fallback: %w", runErr)
		}
		return nil
	}

	return fmt.Errorf("nixpacks not found and Dockerfile missing at repository root")
}

func (w *Worker) cloneRepo(ctx context.Context, input BuildWorkerInput) (string, error) {
	dir, err := os.MkdirTemp(w.cfg.BuildWorker.WorkspaceDir, fmt.Sprintf("build-%s-*", input.BuildJobID))
	if err != nil {
		return "", err
	}

	// Use public HTTPS clone (no auth needed for public repos)
	// For private repos, we'd need a GitHub App installation token
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", input.RepoOwner, input.RepoName)

	slog.Info("cloning repo", "url", repoURL, "commit", input.CommitSHA)

	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, dir)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone: %s: %w", string(output), err)
	}

	return dir, nil
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

func (w *Worker) callback(ctx context.Context, input BuildWorkerInput, status, imageRef, imageDigest string, services []string) error {
	if services == nil {
		services = []string{}
	}

	body := map[string]any{
		"build_job_id": input.BuildJobID,
		"project_id":   input.ProjectID,
		"commit_sha":   input.CommitSHA,
		"status":       status,
	}

	if status == "succeeded" {
		body["image_ref"] = imageRef
		body["image_digest"] = imageDigest
		body["metadata"] = map[string]any{
			"detected_services": services,
		}
	}

	payload, _ := json.Marshal(body)

	// Call the callback endpoint.
	// Prefer explicit BUILD_WORKER_CALLBACK_BASE_URL; otherwise use an internal-safe default.
	url := w.callbackURL()

	slog.Info("sending build callback", "url", url, "status", status, "image", imageRef)

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

func (w *Worker) failJob(jobID string, reason string) {
	now := time.Now().UTC()
	w.db.Model(&models.BuildJob{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":       "failed",
		"started_at":   now,
		"completed_at": now,
		"updated_at":   now,
	})
	slog.Error("build job marked as failed", "job_id", jobID, "reason", reason)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
