package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"lazyops-cli/internal/contracts"
)

type MockTransport struct {
	fixtures map[string]Response
	latency  time.Duration
}

func NewMockTransport(fixtures map[string]Response) *MockTransport {
	return NewMockTransportWithLatency(fixtures, 50*time.Millisecond)
}

func NewMockTransportWithLatency(fixtures map[string]Response, latency time.Duration) *MockTransport {
	return &MockTransport{
		fixtures: fixtures,
		latency:  latency,
	}
}

func (t *MockTransport) Mode() string {
	return "mock"
}

func (t *MockTransport) Do(ctx context.Context, req Request) (Response, error) {
	select {
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-time.After(t.latency):
	}

	if strings.EqualFold(req.Method, "POST") && req.Path == "/api/v1/auth/cli-login" {
		return mockCLILogin(req)
	}
	if strings.EqualFold(req.Method, "POST") && req.Path == "/api/v1/auth/pat/revoke" {
		return mockPATRevoke(req)
	}
	if response, handled := mockAuthorize(req); handled {
		return response, nil
	}
	if strings.EqualFold(req.Method, "POST") && isProjectRepoLinkPath(req.Path) {
		return mockLinkRepository(req)
	}
	if strings.EqualFold(req.Method, "POST") && isProjectBindingsPath(req.Path) {
		return mockCreateDeploymentBinding(req)
	}
	if strings.EqualFold(req.Method, "GET") && req.Path == "/ws/logs/stream" {
		return mockLogsStream(req)
	}
	if strings.EqualFold(req.Method, "GET") && isTracePath(req.Path) {
		return mockTrace(req)
	}
	if strings.EqualFold(req.Method, "POST") && isTunnelSessionPath(req.Path) {
		return mockCreateTunnelSession(req)
	}
	if strings.EqualFold(req.Method, "DELETE") && isTunnelStopPath(req.Path) {
		return mockStopTunnelSession(req)
	}

	response, ok := t.fixtures[req.Key()]
	if !ok {
		return Response{}, fmt.Errorf("mock fixture not found for %s", req.Key())
	}

	return response, nil
}

func mockAuthorize(req Request) (Response, bool) {
	if !requiresMockAuth(req.Path) {
		return Response{}, false
	}

	authHeader := strings.TrimSpace(req.Headers["Authorization"])
	if authHeader == "" {
		return Response{
			StatusCode:  401,
			FixtureName: "mock-missing-auth",
			Body: mustJSON(map[string]any{
				"error":     "missing_auth",
				"message":   "CLI session is missing or expired.",
				"next_step": "run `lazyops login` again and retry the command",
			}),
		}, true
	}
	if authHeader != "Bearer lazyops_pat_mock_secret_value" {
		return Response{
			StatusCode:  401,
			FixtureName: "mock-invalid-auth",
			Body: mustJSON(map[string]any{
				"error":     "invalid_auth",
				"message":   "CLI PAT is invalid or revoked.",
				"next_step": "run `lazyops login` again to issue a fresh PAT",
			}),
		}, true
	}

	return Response{}, false
}

func requiresMockAuth(path string) bool {
	switch {
	case path == "/api/v1/auth/cli-login":
		return false
	case path == "/api/v1/auth/pat/revoke":
		return false
	case strings.HasPrefix(path, "/api/v1/"):
		return true
	case strings.HasPrefix(path, "/ws/"):
		return true
	case strings.HasPrefix(path, "/mock/v1/"):
		return true
	default:
		return false
	}
}

func mockCLILogin(req Request) (Response, error) {
	var payload struct {
		Method   string `json:"method"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Provider string `json:"provider"`
	}

	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return Response{
			StatusCode:  400,
			FixtureName: "cli-login-bad-json",
			Body: mustJSON(map[string]any{
				"error":     "invalid_request",
				"message":   "CLI login payload is invalid.",
				"next_step": "retry the login command with valid arguments",
			}),
		}, nil
	}

	switch strings.ToLower(strings.TrimSpace(payload.Method)) {
	case "password":
		if payload.Email == "demo@lazyops.local" && payload.Password == "demo-password" {
			return Response{
				StatusCode:  200,
				FixtureName: "cli-login-password",
				Body: envelope(map[string]any{
					"token": "lazyops_pat_mock_secret_value",
					"user": map[string]any{
						"id":           "usr_demo",
						"display_name": "CLI Demo User",
					},
					"meta": map[string]any{
						"storage_hint": "keychain",
						"auth_method":  "password",
					},
				}),
			}, nil
		}

		return Response{
			StatusCode:  401,
			FixtureName: "cli-login-invalid-credentials",
			Body: mustJSON(map[string]any{
				"error":     "invalid_credentials",
				"message":   "Email or password is incorrect.",
				"next_step": "retry `lazyops login --email <email> --password <password>` or switch to `--provider github` or `--provider google`",
			}),
		}, nil
	case "browser":
		provider := strings.ToLower(strings.TrimSpace(payload.Provider))
		if provider == "github" || provider == "google" {
			return Response{
				StatusCode:  200,
				FixtureName: "cli-login-browser-" + provider,
				Body: envelope(map[string]any{
					"token": "lazyops_pat_mock_secret_value",
					"user": map[string]any{
						"id":           "usr_demo",
						"display_name": "CLI Demo User",
					},
					"meta": map[string]any{
						"storage_hint": "keychain",
						"auth_method":  "browser",
						"provider":     provider,
					},
				}),
			}, nil
		}

		return Response{
			StatusCode:  400,
			FixtureName: "cli-login-invalid-provider",
			Body: mustJSON(map[string]any{
				"error":     "invalid_provider",
				"message":   "Browser login provider is not supported.",
				"next_step": "use `--provider github` or `--provider google`",
			}),
		}, nil
	default:
		return Response{
			StatusCode:  400,
			FixtureName: "cli-login-invalid-method",
			Body: mustJSON(map[string]any{
				"error":     "invalid_method",
				"message":   "CLI login method is not supported.",
				"next_step": "use email/password or `--provider github` or `--provider google`",
			}),
		}, nil
	}
}

func mockPATRevoke(req Request) (Response, error) {
	authHeader := strings.TrimSpace(req.Headers["Authorization"])
	if authHeader == "" {
		return Response{
			StatusCode:  401,
			FixtureName: "pat-revoke-missing-auth",
			Body: mustJSON(map[string]any{
				"error":     "missing_auth",
				"message":   "CLI session is missing a valid PAT.",
				"next_step": "run `lazyops login` again and retry `lazyops logout`",
			}),
		}, nil
	}

	if authHeader != "Bearer lazyops_pat_mock_secret_value" {
		return Response{
			StatusCode:  401,
			FixtureName: "pat-revoke-invalid-auth",
			Body: mustJSON(map[string]any{
				"error":     "invalid_auth",
				"message":   "CLI PAT is invalid or already revoked.",
				"next_step": "run `lazyops login` again before retrying `lazyops logout`",
			}),
		}, nil
	}

	return Response{
		StatusCode:  200,
		FixtureName: "pat-revoke",
		Body: envelope(map[string]any{
			"revoked": true,
		}),
	}, nil
}

func mockCreateDeploymentBinding(req Request) (Response, error) {
	projectID, ok := projectIDFromBindingsPath(req.Path)
	if !ok {
		return Response{
			StatusCode:  400,
			FixtureName: "deployment-binding-invalid-path",
			Body: mustJSON(map[string]any{
				"error":     "invalid_path",
				"message":   "Deployment binding path is invalid.",
				"next_step": "retry the command with a valid project id",
			}),
		}, nil
	}

	var payload struct {
		Name        string `json:"name"`
		TargetRef   string `json:"target_ref"`
		RuntimeMode string `json:"runtime_mode"`
		TargetKind  string `json:"target_kind"`
		TargetID    string `json:"target_id"`
	}
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return Response{
			StatusCode:  400,
			FixtureName: "deployment-binding-bad-json",
			Body: mustJSON(map[string]any{
				"error":     "invalid_request",
				"message":   "Deployment binding payload is invalid.",
				"next_step": "retry the init command with valid binding arguments",
			}),
		}, nil
	}

	mode := strings.TrimSpace(payload.RuntimeMode)
	kind := strings.TrimSpace(payload.TargetKind)
	if !bindingModeMatchesKind(mode, kind) {
		return Response{
			StatusCode:  400,
			FixtureName: "deployment-binding-incompatible-mode",
			Body: mustJSON(map[string]any{
				"error":     "incompatible_mode",
				"message":   "Deployment binding runtime mode is incompatible with the selected target kind.",
				"next_step": "choose a target that matches the runtime mode or change `--runtime-mode`",
			}),
		}, nil
	}

	binding := contracts.DeploymentBinding{
		ID:          "bind_" + sanitizeBindingID(payload.Name),
		ProjectID:   projectID,
		Name:        strings.TrimSpace(payload.Name),
		TargetRef:   strings.TrimSpace(payload.TargetRef),
		RuntimeMode: mode,
		TargetKind:  kind,
		TargetID:    strings.TrimSpace(payload.TargetID),
		PlacementPolicy: map[string]any{
			"strategy": "balanced",
		},
		DomainPolicy: map[string]any{
			"provider": "sslip.io",
		},
		CompatibilityPolicy: map[string]any{
			"env_injection":       true,
			"managed_credentials": true,
			"localhost_rescue":    true,
		},
		ScaleToZeroPolicy: map[string]any{
			"enabled": false,
		},
		CreatedAt: time.Date(2026, time.April, 1, 9, 0, 0, 0, time.UTC),
	}

	return Response{
		StatusCode:  201,
		FixtureName: "deployment-binding-created",
		Body:        envelope(binding),
	}, nil
}

func isProjectBindingsPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/projects/") && strings.HasSuffix(path, "/deployment-bindings")
}

func isProjectRepoLinkPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/projects/") && strings.HasSuffix(path, "/repo-link")
}

func projectIDFromBindingsPath(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "projects" || parts[4] != "deployment-bindings" {
		return "", false
	}
	return parts[3], true
}

func bindingModeMatchesKind(mode string, kind string) bool {
	switch mode {
	case "standalone":
		return kind == "instance"
	case "distributed-mesh":
		return kind == "mesh"
	case "distributed-k3s":
		return kind == "cluster"
	default:
		return false
	}
}

func mockLinkRepository(req Request) (Response, error) {
	projectID, ok := projectIDFromRepoLinkPath(req.Path)
	if !ok {
		return Response{
			StatusCode:  400,
			FixtureName: "repo-link-invalid-path",
			Body: mustJSON(map[string]any{
				"error":     "invalid_path",
				"message":   "Repo link path is invalid.",
				"next_step": "retry the command with a valid project id",
			}),
		}, nil
	}

	var payload struct {
		InstallationID int64  `json:"installation_id"`
		RepoID         int64  `json:"repo_id"`
		TrackedBranch  string `json:"tracked_branch"`
	}
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return Response{
			StatusCode:  400,
			FixtureName: "repo-link-bad-json",
			Body: mustJSON(map[string]any{
				"error":     "invalid_request",
				"message":   "Repo link payload is invalid.",
				"next_step": "retry `lazyops link` after rechecking the project, installation, repo, and branch selections",
			}),
		}, nil
	}

	if payload.InstallationID != 48151623 || payload.RepoID != 1001 {
		return Response{
			StatusCode:  403,
			FixtureName: "repo-link-repo-access-denied",
			Body: mustJSON(map[string]any{
				"error":     "repo_access_denied",
				"message":   "The selected GitHub App installation does not grant access to the repo.",
				"next_step": "install the GitHub App on the repo or choose a different installation",
			}),
		}, nil
	}
	if strings.TrimSpace(payload.TrackedBranch) == "" {
		return Response{
			StatusCode:  400,
			FixtureName: "repo-link-missing-branch",
			Body: mustJSON(map[string]any{
				"error":     "missing_branch",
				"message":   "Tracked branch is required for repo link.",
				"next_step": "rerun `lazyops link --branch <tracked-branch>`",
			}),
		}, nil
	}

	link := contracts.ProjectRepoLink{
		ID:                   "prl_demo",
		ProjectID:            projectID,
		GitHubInstallationID: payload.InstallationID,
		GitHubRepoID:         payload.RepoID,
		RepoOwner:            "lazyops",
		RepoName:             "acme-shop",
		TrackedBranch:        strings.TrimSpace(payload.TrackedBranch),
		PreviewEnabled:       true,
		CreatedAt:            time.Date(2026, time.April, 2, 8, 0, 0, 0, time.UTC),
	}

	return Response{
		StatusCode:  201,
		FixtureName: "repo-link",
		Body:        envelope(link),
	}, nil
}

func projectIDFromRepoLinkPath(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "v1" || parts[2] != "projects" || parts[4] != "repo-link" {
		return "", false
	}
	return parts[3], true
}

func sanitizeBindingID(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if normalized == "" {
		return "generated"
	}
	return normalized
}

func isTracePath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/traces/")
}

func mockTrace(req Request) (Response, error) {
	parts := strings.Split(strings.TrimPrefix(req.Path, "/api/v1/traces/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return Response{
			StatusCode:  400,
			FixtureName: "trace-missing-correlation-id",
			Body: mustJSON(map[string]any{
				"error":     "missing_correlation_id",
				"message":   "Correlation id is required for trace lookup.",
				"next_step": "rerun `lazyops traces <correlation-id>` with a valid correlation id",
			}),
		}, nil
	}

	correlationID := parts[0]

	datasets := map[string]contracts.TraceSummary{
		"corr-demo": {
			CorrelationID:  "corr-demo",
			ServicePath:    []string{"gateway", "web", "api", "postgres"},
			NodeHops:       []string{"edge-ap-1", "mesh-ap-2", "db-ap-1"},
			LatencyHotspot: "api -> postgres",
			TotalLatencyMS: 182,
		},
		"corr-error-demo": {
			CorrelationID:  "corr-error-demo",
			ServicePath:    []string{"gateway", "api", "external-service"},
			NodeHops:       []string{"edge-ap-1", "mesh-ap-2"},
			LatencyHotspot: "api -> external-service",
			TotalLatencyMS: 5230,
		},
	}

	summary, ok := datasets[correlationID]
	if !ok {
		summary = contracts.TraceSummary{
			CorrelationID:  correlationID,
			ServicePath:    []string{"gateway", "web", "api"},
			NodeHops:       []string{"edge-ap-1", "mesh-ap-1"},
			LatencyHotspot: "unknown",
			TotalLatencyMS: 95,
		}
	}

	return Response{
		StatusCode:  200,
		FixtureName: "trace-summary",
		Body:        envelope(summary),
	}, nil
}

func isTunnelSessionPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/tunnels/db/sessions") || strings.HasPrefix(path, "/api/v1/tunnels/tcp/sessions")
}

func isTunnelStopPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/tunnels/sessions/")
}

func mockCreateTunnelSession(req Request) (Response, error) {
	var payload struct {
		ProjectID string `json:"project_id"`
		LocalPort int    `json:"local_port"`
		Remote    string `json:"remote"`
		Timeout   string `json:"timeout"`
		TargetRef string `json:"target_ref"`
	}
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return Response{
			StatusCode:  400,
			FixtureName: "tunnel-bad-json",
			Body: mustJSON(map[string]any{
				"error":     "invalid_request",
				"message":   "Tunnel session payload is invalid.",
				"next_step": "retry the tunnel command with valid arguments",
			}),
		}, nil
	}

	if payload.ProjectID == "" {
		return Response{
			StatusCode:  400,
			FixtureName: "tunnel-missing-project",
			Body: mustJSON(map[string]any{
				"error":     "missing_project",
				"message":   "Project id is required to create a tunnel session.",
				"next_step": "run `lazyops init` before opening a debug tunnel",
			}),
		}, nil
	}

	if payload.ProjectID != "prj_demo" {
		return Response{
			StatusCode:  404,
			FixtureName: "tunnel-project-not-found",
			Body: mustJSON(map[string]any{
				"error":     "project_not_found",
				"message":   fmt.Sprintf("Project %q does not have tunnel support enabled.", payload.ProjectID),
				"next_step": "verify the project has an online target and retry",
			}),
		}, nil
	}

	if payload.LocalPort == 99999 {
		return Response{
			StatusCode:  409,
			FixtureName: "tunnel-port-conflict",
			Body: mustJSON(map[string]any{
				"error":     "port_conflict",
				"message":   fmt.Sprintf("Local port %d is already in use by another process.", payload.LocalPort),
				"next_step": "choose a different port with `--port` or stop the process using that port",
			}),
		}, nil
	}

	sessionID := "tun_demo_" + fmt.Sprintf("%d", payload.LocalPort)
	tunnelType := "db"
	if strings.Contains(req.Path, "/tcp/") {
		tunnelType = "tcp"
	}

	return Response{
		StatusCode:  201,
		FixtureName: "tunnel-session-created",
		Body: envelope(map[string]any{
			"session_id": sessionID,
			"local_port": payload.LocalPort,
			"remote":     payload.Remote,
			"status":     "ready",
			"expires_at": time.Now().Add(30 * time.Minute).Format(time.RFC3339),
			"type":       tunnelType,
		}),
	}, nil
}

func mockStopTunnelSession(req Request) (Response, error) {
	parts := strings.Split(strings.TrimPrefix(req.Path, "/api/v1/tunnels/sessions/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return Response{
			StatusCode:  400,
			FixtureName: "tunnel-stop-missing-id",
			Body: mustJSON(map[string]any{
				"error":     "missing_session_id",
				"message":   "Session id is required to stop a tunnel.",
				"next_step": "rerun `lazyops tunnel stop <session-id>`",
			}),
		}, nil
	}

	sessionID := parts[0]
	if sessionID == "tun_nonexistent" {
		return Response{
			StatusCode:  404,
			FixtureName: "tunnel-stop-not-found",
			Body: mustJSON(map[string]any{
				"error":     "session_not_found",
				"message":   fmt.Sprintf("Tunnel session %q was not found.", sessionID),
				"next_step": "list active sessions with `lazyops tunnel list` or start a new tunnel",
			}),
		}, nil
	}

	return Response{
		StatusCode:  200,
		FixtureName: "tunnel-session-stopped",
		Body: envelope(map[string]any{
			"session_id": sessionID,
			"status":     "stopped",
		}),
	}, nil
}

func mockLogsStream(req Request) (Response, error) {
	projectID := strings.TrimSpace(req.Query["project"])
	if projectID != "" && projectID != "prj_demo" {
		return Response{
			StatusCode:  404,
			FixtureName: "logs-project-not-found",
			Body: mustJSON(map[string]any{
				"error":     "project_not_found",
				"message":   fmt.Sprintf("Project %q was not found for the logs stream.", projectID),
				"next_step": "rerun `lazyops logs <service>` from a repo linked to a valid project",
			}),
		}, nil
	}

	service := strings.TrimSpace(req.Query["service"])
	if service == "" {
		return Response{
			StatusCode:  400,
			FixtureName: "logs-missing-service",
			Body: mustJSON(map[string]any{
				"error":     "missing_service",
				"message":   "Service filter is required for the logs stream.",
				"next_step": "rerun `lazyops logs <service>` with a declared service name",
			}),
		}, nil
	}

	baseTimestamp := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	datasets := map[string]contracts.LogsStreamPreview{
		"api": {
			Service: "api",
			Cursor:  "cursor_demo_002",
			Lines: []contracts.LogLine{
				{
					Timestamp: baseTimestamp,
					Level:     "info",
					Message:   "api listening on :8080",
					Node:      "edge-ap-1",
				},
				{
					Timestamp: baseTimestamp.Add(2 * time.Second),
					Level:     "info",
					Message:   "GET /health 200 1.2ms",
					Node:      "edge-ap-1",
				},
				{
					Timestamp: baseTimestamp.Add(4 * time.Second),
					Level:     "error",
					Message:   "upstream timeout while contacting postgres",
					Node:      "edge-ap-2",
				},
			},
		},
		"web": {
			Service: "web",
			Cursor:  "cursor_web_001",
			Lines: []contracts.LogLine{
				{
					Timestamp: baseTimestamp,
					Level:     "info",
					Message:   "web listening on :3000",
					Node:      "edge-ap-1",
				},
			},
		},
	}

	preview, ok := datasets[service]
	if !ok {
		return Response{
			StatusCode:  404,
			FixtureName: "logs-service-not-found",
			Body: mustJSON(map[string]any{
				"error":     "service_not_found",
				"message":   fmt.Sprintf("Service %q has no log stream for the selected project.", service),
				"next_step": "choose a service declared in lazyops.yaml or rerun `lazyops init` if the repo layout changed",
			}),
		}, nil
	}

	level := strings.ToLower(strings.TrimSpace(req.Query["level"]))
	if level != "" && level != "debug" && level != "info" && level != "warn" && level != "error" {
		return Response{
			StatusCode:  400,
			FixtureName: "logs-invalid-level",
			Body: mustJSON(map[string]any{
				"error":     "invalid_filter",
				"message":   fmt.Sprintf("Log level filter %q is invalid.", level),
				"next_step": "use level filters debug, info, warn, or error",
			}),
		}, nil
	}

	cursor := strings.TrimSpace(req.Query["cursor"])
	if cursor != "" && cursor != "cursor_demo_001" && cursor != "cursor_demo_002" && cursor != "cursor_web_001" {
		return Response{
			StatusCode:  400,
			FixtureName: "logs-invalid-cursor",
			Body: mustJSON(map[string]any{
				"error":     "invalid_cursor",
				"message":   fmt.Sprintf("Cursor %q is invalid for the logs stream.", cursor),
				"next_step": "drop `--cursor` or use a cursor returned by the previous logs response",
			}),
		}, nil
	}

	limit := len(preview.Lines)
	if rawLimit := strings.TrimSpace(req.Query["limit"]); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return Response{
				StatusCode:  400,
				FixtureName: "logs-invalid-limit",
				Body: mustJSON(map[string]any{
					"error":     "invalid_limit",
					"message":   fmt.Sprintf("Log limit %q is invalid.", rawLimit),
					"next_step": "use a positive integer for the log limit filter",
				}),
			}, nil
		}
		limit = parsed
	}

	if cursor == "cursor_demo_001" && service == "api" && len(preview.Lines) > 1 {
		preview.Lines = preview.Lines[1:]
	}

	contains := strings.ToLower(strings.TrimSpace(req.Query["contains"]))
	node := strings.TrimSpace(req.Query["node"])

	filtered := make([]contracts.LogLine, 0, len(preview.Lines))
	for _, line := range preview.Lines {
		if level != "" && !strings.EqualFold(line.Level, level) {
			continue
		}
		if node != "" && !strings.EqualFold(line.Node, node) {
			continue
		}
		if contains != "" && !strings.Contains(strings.ToLower(line.Message), contains) {
			continue
		}
		filtered = append(filtered, line)
		if len(filtered) >= limit {
			break
		}
	}
	preview.Lines = filtered

	return Response{
		StatusCode:  200,
		FixtureName: "logs-stream",
		Body:        envelope(preview),
	}, nil
}

func DefaultFixtures() map[string]Response {
	projectCreatedAt := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	installationInstalledAt := projectCreatedAt.Add(-24 * time.Hour)
	instanceCreatedAt := projectCreatedAt.Add(-72 * time.Hour)
	bindingCreatedAt := projectCreatedAt.Add(-6 * time.Hour)

	projectList := contracts.ProjectsResponse{
		Projects: []contracts.Project{
			{
				ID:            "prj_demo",
				Name:          "Acme Shop",
				Slug:          "acme-shop",
				DefaultBranch: "main",
				CreatedAt:     projectCreatedAt,
			},
		},
	}

	installations := contracts.GitHubInstallationsResponse{
		Installations: []contracts.GitHubInstallation{
			{
				ID:                   "ghi_demo",
				GitHubInstallationID: 48151623,
				AccountLogin:         "lazyops",
				AccountType:          "Organization",
				ScopeJSON: map[string]any{
					"repositories": []map[string]any{
						{
							"id":             1001,
							"name":           "acme-shop",
							"owner":          "lazyops",
							"default_branch": "main",
						},
					},
					"permissions": map[string]any{
						"contents": "read",
						"metadata": "read",
					},
				},
				InstalledAt: installationInstalledAt,
			},
		},
	}

	instances := contracts.InstancesResponse{
		Instances: []contracts.Instance{
			{
				ID:        "inst_demo",
				Name:      "prod-solo-1",
				PublicIP:  "203.0.113.10",
				PrivateIP: "10.10.0.10",
				AgentID:   "agt_inst_demo",
				Status:    "online",
				Labels: map[string]any{
					"region": "ap-southeast-1",
					"tier":   "prod",
				},
				RuntimeCapabilities: map[string]any{
					"docker":     true,
					"scale_zero": true,
				},
				CreatedAt: instanceCreatedAt,
			},
		},
	}

	meshNetworks := contracts.MeshNetworksResponse{
		MeshNetworks: []contracts.MeshNetwork{
			{
				ID:        "mesh_demo",
				Name:      "prod-ap",
				Provider:  "wireguard",
				CIDR:      "10.42.0.0/16",
				Status:    "online",
				CreatedAt: instanceCreatedAt,
			},
		},
	}

	clusters := contracts.ClustersResponse{
		Clusters: []contracts.Cluster{
			{
				ID:                  "cls_demo",
				Name:                "prod-k3s-ap",
				Provider:            "k3s",
				KubeconfigSecretRef: "secret://clusters/cls_demo/kubeconfig",
				Status:              "registered",
				CreatedAt:           instanceCreatedAt,
			},
		},
	}

	binding := contracts.DeploymentBinding{
		ID:          "bind_demo",
		ProjectID:   "prj_demo",
		Name:        "prod-ap-mesh",
		TargetRef:   "prod-ap",
		RuntimeMode: "distributed-mesh",
		TargetKind:  "mesh",
		TargetID:    "mesh_demo",
		PlacementPolicy: map[string]any{
			"strategy": "balanced",
		},
		DomainPolicy: map[string]any{
			"provider": "sslip.io",
		},
		CompatibilityPolicy: map[string]any{
			"env_injection":    true,
			"localhost_rescue": true,
		},
		ScaleToZeroPolicy: map[string]any{
			"enabled": false,
		},
		CreatedAt: bindingCreatedAt,
	}

	standaloneBinding := contracts.DeploymentBinding{
		ID:          "bind_standalone_demo",
		ProjectID:   "prj_demo",
		Name:        "prod-solo-binding",
		TargetRef:   "prod-solo-1",
		RuntimeMode: "standalone",
		TargetKind:  "instance",
		TargetID:    "inst_demo",
		PlacementPolicy: map[string]any{
			"strategy": "balanced",
		},
		DomainPolicy: map[string]any{
			"provider": "sslip.io",
		},
		CompatibilityPolicy: map[string]any{
			"env_injection":       true,
			"managed_credentials": true,
			"localhost_rescue":    true,
		},
		ScaleToZeroPolicy: map[string]any{
			"enabled": false,
		},
		CreatedAt: bindingCreatedAt.Add(-30 * time.Minute),
	}

	k3sBinding := contracts.DeploymentBinding{
		ID:          "bind_k3s_demo",
		ProjectID:   "prj_demo",
		Name:        "prod-k3s-binding",
		TargetRef:   "prod-k3s-ap",
		RuntimeMode: "distributed-k3s",
		TargetKind:  "cluster",
		TargetID:    "cls_demo",
		PlacementPolicy: map[string]any{
			"strategy": "cluster-native",
		},
		DomainPolicy: map[string]any{
			"provider": "sslip.io",
		},
		CompatibilityPolicy: map[string]any{
			"env_injection":       true,
			"managed_credentials": true,
			"localhost_rescue":    true,
		},
		ScaleToZeroPolicy: map[string]any{
			"enabled": false,
		},
		CreatedAt: bindingCreatedAt.Add(-15 * time.Minute),
	}

	bindingList := contracts.DeploymentBindingsResponse{
		Bindings: []contracts.DeploymentBinding{binding, standaloneBinding, k3sBinding},
	}

	traceSummary := contracts.TraceSummary{
		CorrelationID:  "corr-demo",
		ServicePath:    []string{"gateway", "web", "api", "postgres"},
		NodeHops:       []string{"edge-ap-1", "mesh-ap-2", "db-ap-1"},
		LatencyHotspot: "api -> postgres",
		TotalLatencyMS: 182,
	}

	return map[string]Response{
		Request{Method: "GET", Path: "/api/v1/projects"}.Key(): {
			StatusCode:  200,
			FixtureName: "projects-list",
			Body:        envelope(projectList),
		},
		Request{Method: "POST", Path: "/api/v1/github/app/installations/sync"}.Key(): {
			StatusCode:  200,
			FixtureName: "github-installations",
			Body:        envelope(installations),
		},
		Request{Method: "GET", Path: "/api/v1/instances"}.Key(): {
			StatusCode:  200,
			FixtureName: "instances-list",
			Body:        envelope(instances),
		},
		Request{Method: "GET", Path: "/api/v1/mesh-networks"}.Key(): {
			StatusCode:  200,
			FixtureName: "mesh-list",
			Body:        envelope(meshNetworks),
		},
		Request{Method: "GET", Path: "/api/v1/clusters"}.Key(): {
			StatusCode:  200,
			FixtureName: "clusters-list",
			Body:        envelope(clusters),
		},
		Request{Method: "POST", Path: "/api/v1/projects/prj_demo/repo-link"}.Key(): {
			StatusCode:  200,
			FixtureName: "repo-link",
			Body: envelope(map[string]any{
				"project_id": "prj_demo",
				"repo_owner": "lazyops",
				"repo_name":  "acme-shop",
				"linked":     true,
			}),
		},
		Request{Method: "GET", Path: "/api/v1/projects/prj_demo/deployment-bindings"}.Key(): {
			StatusCode:  200,
			FixtureName: "deployment-bindings",
			Body:        envelope(bindingList),
		},
		Request{Method: "POST", Path: "/api/v1/projects/prj_demo/deployment-bindings"}.Key(): {
			StatusCode:  201,
			FixtureName: "deployment-binding-created",
			Body:        envelope(binding),
		},
		Request{Method: "GET", Path: "/mock/v1/doctor", Query: map[string]string{"project": "prj_demo"}}.Key(): {
			StatusCode:  200,
			FixtureName: "doctor-preview",
			Body: envelope(map[string]any{
				"checks": []map[string]any{
					{
						"name":      "auth",
						"status":    "pass",
						"summary":   "CLI auth preview is healthy.",
						"next_step": "",
					},
					{
						"name":      "repo_link",
						"status":    "pass",
						"summary":   "Repo link preview is healthy.",
						"next_step": "",
					},
					{
						"name":      "webhook_health",
						"status":    "pass",
						"summary":   "Deploy webhook is registered and reachable.",
						"next_step": "",
					},
				},
			}),
		},
		Request{Method: "GET", Path: "/mock/v1/status", Query: map[string]string{"project": "prj_demo"}}.Key(): {
			StatusCode:  200,
			FixtureName: "status-preview",
			Body: envelope(map[string]any{
				"project_id": "prj_demo",
				"rollout":    "idle",
				"topology":   "mock-preview",
			}),
		},
		Request{Method: "GET", Path: "/api/v1/traces/corr-demo"}.Key(): {
			StatusCode:  200,
			FixtureName: "trace-summary",
			Body:        envelope(traceSummary),
		},
		Request{Method: "POST", Path: "/api/v1/tunnels/db/sessions"}.Key(): {
			StatusCode:  201,
			FixtureName: "db-tunnel",
			Body: envelope(map[string]any{
				"session_id": "tun_db_demo",
				"local_port": 15432,
				"status":     "ready",
			}),
		},
		Request{Method: "POST", Path: "/api/v1/tunnels/tcp/sessions"}.Key(): {
			StatusCode:  201,
			FixtureName: "tcp-tunnel",
			Body: envelope(map[string]any{
				"session_id": "tun_tcp_demo",
				"local_port": 19090,
				"status":     "ready",
			}),
		},
	}
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func envelope(data any) []byte {
	return mustJSON(map[string]any{
		"success": true,
		"data":    data,
	})
}
