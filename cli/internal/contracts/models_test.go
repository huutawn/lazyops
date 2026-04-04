package contracts

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecodeCoreContracts(t *testing.T) {
	timestamp := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	revokedAt := timestamp.Add(2 * time.Hour)

	testCases := []struct {
		name   string
		body   []byte
		decode func([]byte) error
	}{
		{
			name: "projects",
			body: mustMarshal(t, ProjectsResponse{
				Projects: []Project{{
					ID:            "prj_demo",
					UserID:        "usr_demo",
					Name:          "Acme Shop",
					Slug:          "acme-shop",
					DefaultBranch: "main",
					CreatedAt:     timestamp,
				}},
			}),
			decode: func(payload []byte) error {
				_, err := DecodeProjectsResponse(payload)
				return err
			},
		},
		{
			name: "github_installations",
			body: mustMarshal(t, GitHubInstallationsResponse{
				Installations: []GitHubInstallation{{
					ID:                   "ghi_demo",
					UserID:               "usr_demo",
					GitHubInstallationID: 48151623,
					AccountLogin:         "lazyops",
					AccountType:          "Organization",
					ScopeJSON: map[string]any{
						"repositories": []string{"acme-shop"},
					},
					InstalledAt: timestamp,
					RevokedAt:   &revokedAt,
				}},
			}),
			decode: func(payload []byte) error {
				_, err := DecodeGitHubInstallationsResponse(payload)
				return err
			},
		},
		{
			name: "instances",
			body: mustMarshal(t, InstancesResponse{
				Instances: []Instance{{
					ID:        "inst_demo",
					UserID:    "usr_demo",
					Name:      "prod-solo-1",
					PublicIP:  "203.0.113.10",
					PrivateIP: "10.10.0.10",
					AgentID:   "agt_inst_demo",
					Status:    "online",
					CreatedAt: timestamp,
				}},
			}),
			decode: func(payload []byte) error {
				_, err := DecodeInstancesResponse(payload)
				return err
			},
		},
		{
			name: "mesh_networks",
			body: mustMarshal(t, MeshNetworksResponse{
				MeshNetworks: []MeshNetwork{{
					ID:        "mesh_demo",
					UserID:    "usr_demo",
					Name:      "prod-ap",
					Provider:  "wireguard",
					CIDR:      "10.42.0.0/16",
					Status:    "online",
					CreatedAt: timestamp,
				}},
			}),
			decode: func(payload []byte) error {
				_, err := DecodeMeshNetworksResponse(payload)
				return err
			},
		},
		{
			name: "clusters",
			body: mustMarshal(t, ClustersResponse{
				Clusters: []Cluster{{
					ID:                  "cls_demo",
					UserID:              "usr_demo",
					Name:                "prod-k3s-ap",
					Provider:            "k3s",
					KubeconfigSecretRef: "secret://clusters/cls_demo/kubeconfig",
					Status:              "registered",
					CreatedAt:           timestamp,
				}},
			}),
			decode: func(payload []byte) error {
				_, err := DecodeClustersResponse(payload)
				return err
			},
		},
		{
			name: "deployment_bindings",
			body: mustMarshal(t, DeploymentBindingsResponse{
				Bindings: []DeploymentBinding{{
					ID:          "bind_demo",
					ProjectID:   "prj_demo",
					Name:        "prod-ap-mesh",
					TargetRef:   "prod-ap",
					RuntimeMode: "distributed-mesh",
					TargetKind:  "mesh",
					TargetID:    "mesh_demo",
					CreatedAt:   timestamp,
				}},
			}),
			decode: func(payload []byte) error {
				_, err := DecodeDeploymentBindingsResponse(payload)
				return err
			},
		},
		{
			name: "deployment_binding",
			body: mustMarshal(t, DeploymentBinding{
				ID:          "bind_demo",
				ProjectID:   "prj_demo",
				Name:        "prod-ap-mesh",
				TargetRef:   "prod-ap",
				RuntimeMode: "distributed-mesh",
				TargetKind:  "mesh",
				TargetID:    "mesh_demo",
				CreatedAt:   timestamp,
			}),
			decode: func(payload []byte) error {
				_, err := DecodeDeploymentBinding(payload)
				return err
			},
		},
		{
			name: "project_repo_link",
			body: mustMarshal(t, ProjectRepoLink{
				ID:                   "prl_demo",
				ProjectID:            "prj_demo",
				GitHubInstallationID: 48151623,
				GitHubRepoID:         1001,
				RepoOwner:            "lazyops",
				RepoName:             "acme-shop",
				TrackedBranch:        "main",
				PreviewEnabled:       true,
				CreatedAt:            timestamp,
			}),
			decode: func(payload []byte) error {
				_, err := DecodeProjectRepoLink(payload)
				return err
			},
		},
		{
			name: "trace_summary",
			body: mustMarshal(t, TraceSummary{
				CorrelationID:  "corr-demo",
				ServicePath:    []string{"gateway", "web", "api", "postgres"},
				NodeHops:       []string{"edge-ap-1", "mesh-ap-2"},
				LatencyHotspot: "api -> postgres",
				TotalLatencyMS: 182,
			}),
			decode: func(payload []byte) error {
				_, err := DecodeTraceSummary(payload)
				return err
			},
		},
		{
			name: "logs_stream",
			body: mustMarshal(t, LogsStreamPreview{
				Service: "api",
				Cursor:  "cursor_demo",
				Lines: []LogLine{{
					Timestamp: timestamp,
					Level:     "info",
					Message:   "api listening on :8080",
					Node:      "edge-ap-1",
				}},
			}),
			decode: func(payload []byte) error {
				_, err := DecodeLogsStreamPreview(payload)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.decode(tc.body); err != nil {
				t.Fatalf("decode error = %v", err)
			}
		})
	}
}

func TestDeploymentBindingValidateRejectsInvalidRuntimeMode(t *testing.T) {
	binding := DeploymentBinding{
		ID:          "bind_demo",
		ProjectID:   "prj_demo",
		Name:        "prod-ap-mesh",
		TargetRef:   "prod-ap",
		RuntimeMode: "invalid-mode",
		TargetKind:  "mesh",
		TargetID:    "mesh_demo",
	}

	err := binding.Validate()
	if err == nil {
		t.Fatal("expected invalid runtime mode error, got nil")
	}
	if !strings.Contains(err.Error(), "runtime_mode") {
		t.Fatalf("expected runtime_mode validation error, got %v", err)
	}
}

func TestDecodeTraceSummaryRejectsMissingServicePath(t *testing.T) {
	payload := mustMarshal(t, TraceSummary{
		CorrelationID: "corr-demo",
	})

	_, err := DecodeTraceSummary(payload)
	if err == nil {
		t.Fatal("expected missing service path error, got nil")
	}
	if !strings.Contains(err.Error(), "service_path") {
		t.Fatalf("expected service_path validation error, got %v", err)
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	return payload
}
