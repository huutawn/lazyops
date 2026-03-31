package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
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

	response, ok := t.fixtures[req.Key()]
	if !ok {
		return Response{}, fmt.Errorf("mock fixture not found for %s", req.Key())
	}

	return response, nil
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
				Body: mustJSON(map[string]any{
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
				Body: mustJSON(map[string]any{
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
		Body: mustJSON(map[string]any{
			"revoked": true,
		}),
	}, nil
}

func DefaultFixtures() map[string]Response {
	return map[string]Response{
		Request{Method: "GET", Path: "/api/v1/projects"}.Key(): {
			StatusCode:  200,
			FixtureName: "projects-list",
			Body: mustJSON(map[string]any{
				"projects": []map[string]any{
					{
						"id":   "prj_demo",
						"slug": "acme-shop",
						"name": "Acme Shop",
					},
				},
			}),
		},
		Request{Method: "GET", Path: "/api/v1/instances"}.Key(): {
			StatusCode:  200,
			FixtureName: "instances-list",
			Body: mustJSON(map[string]any{
				"instances": []map[string]any{
					{
						"id":     "inst_demo",
						"name":   "prod-solo-1",
						"status": "online",
					},
				},
			}),
		},
		Request{Method: "GET", Path: "/api/v1/mesh-networks"}.Key(): {
			StatusCode:  200,
			FixtureName: "mesh-list",
			Body: mustJSON(map[string]any{
				"mesh_networks": []map[string]any{
					{
						"id":       "mesh_demo",
						"name":     "prod-ap",
						"provider": "wireguard",
						"status":   "online",
					},
				},
			}),
		},
		Request{Method: "GET", Path: "/api/v1/clusters"}.Key(): {
			StatusCode:  200,
			FixtureName: "clusters-list",
			Body: mustJSON(map[string]any{
				"clusters": []map[string]any{
					{
						"id":       "cls_demo",
						"name":     "prod-k3s-ap",
						"provider": "k3s",
						"status":   "registered",
					},
				},
			}),
		},
		Request{Method: "POST", Path: "/api/v1/projects/prj_demo/repo-link"}.Key(): {
			StatusCode:  200,
			FixtureName: "repo-link",
			Body: mustJSON(map[string]any{
				"project_id": "prj_demo",
				"repo_owner": "lazyops",
				"repo_name":  "acme-shop",
				"linked":     true,
			}),
		},
		Request{Method: "GET", Path: "/api/v1/projects/prj_demo/deployment-bindings"}.Key(): {
			StatusCode:  200,
			FixtureName: "deployment-bindings",
			Body: mustJSON(map[string]any{
				"bindings": []map[string]any{
					{
						"id":           "bind_demo",
						"target_ref":   "prod-ap",
						"runtime_mode": "distributed-mesh",
						"target_kind":  "mesh",
						"status":       "selectable",
					},
				},
			}),
		},
		Request{Method: "GET", Path: "/mock/v1/doctor", Query: map[string]string{"project": "prj_demo"}}.Key(): {
			StatusCode:  200,
			FixtureName: "doctor-preview",
			Body: mustJSON(map[string]any{
				"checks": []map[string]any{
					{"name": "auth", "status": "pass"},
					{"name": "repo_link", "status": "pass"},
					{"name": "lazyops_yaml", "status": "warn"},
				},
			}),
		},
		Request{Method: "GET", Path: "/mock/v1/status", Query: map[string]string{"project": "prj_demo"}}.Key(): {
			StatusCode:  200,
			FixtureName: "status-preview",
			Body: mustJSON(map[string]any{
				"project_id": "prj_demo",
				"rollout":    "idle",
				"topology":   "mock-preview",
			}),
		},
		Request{Method: "GET", Path: "/ws/logs/stream", Query: map[string]string{"service": "api"}}.Key(): {
			StatusCode:  200,
			FixtureName: "logs-stream",
			Body: mustJSON(map[string]any{
				"service": "api",
				"lines": []string{
					"2026-03-31T12:00:00Z api listening on :8080",
					"2026-03-31T12:00:02Z GET /health 200 1.2ms",
				},
			}),
		},
		Request{Method: "GET", Path: "/api/v1/traces/corr_demo"}.Key(): {
			StatusCode:  200,
			FixtureName: "trace-summary",
			Body: mustJSON(map[string]any{
				"correlation_id": "corr_demo",
				"path": []string{
					"gateway -> web -> api -> postgres",
				},
				"latency_hotspot": "api -> postgres",
			}),
		},
		Request{Method: "POST", Path: "/api/v1/tunnels/db/sessions"}.Key(): {
			StatusCode:  201,
			FixtureName: "db-tunnel",
			Body: mustJSON(map[string]any{
				"session_id": "tun_db_demo",
				"local_port": 15432,
				"status":     "ready",
			}),
		},
		Request{Method: "POST", Path: "/api/v1/tunnels/tcp/sessions"}.Key(): {
			StatusCode:  201,
			FixtureName: "tcp-tunnel",
			Body: mustJSON(map[string]any{
				"session_id": "tun_tcp_demo",
				"local_port": 19090,
				"status":     "ready",
			}),
		},
	}
}

func mustJSON(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}

	return data
}
