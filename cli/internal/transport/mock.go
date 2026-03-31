package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type MockTransport struct {
	fixtures map[string]Response
	latency  time.Duration
}

func NewMockTransport(fixtures map[string]Response) *MockTransport {
	return &MockTransport{
		fixtures: fixtures,
		latency:  50 * time.Millisecond,
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

	response, ok := t.fixtures[req.Key()]
	if !ok {
		return Response{}, fmt.Errorf("mock fixture not found for %s", req.Key())
	}

	return response, nil
}

func DefaultFixtures() map[string]Response {
	return map[string]Response{
		Request{Method: "POST", Path: "/api/v1/auth/cli-login"}.Key(): {
			StatusCode:  200,
			FixtureName: "cli-login",
			Body: mustJSON(map[string]any{
				"token": "lazyops_pat_mock_redacted",
				"user": map[string]any{
					"id":           "usr_demo",
					"display_name": "CLI Demo User",
				},
				"meta": map[string]any{
					"storage_hint": "keychain",
				},
			}),
		},
		Request{Method: "POST", Path: "/api/v1/auth/pat/revoke"}.Key(): {
			StatusCode:  200,
			FixtureName: "pat-revoke",
			Body: mustJSON(map[string]any{
				"revoked": true,
			}),
		},
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
