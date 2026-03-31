package transport

import (
	"context"
	"strings"
	"testing"

	"lazyops-cli/internal/contracts"
)

func TestMockTransportReturnsFixture(t *testing.T) {
	client := NewMockTransport(DefaultFixtures())

	response, err := client.Do(context.Background(), Request{
		Method: "GET",
		Path:   "/api/v1/projects",
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}

	if response.FixtureName != "projects-list" {
		t.Fatalf("expected fixture %q, got %q", "projects-list", response.FixtureName)
	}
}

func TestMockTransportErrorsWhenFixtureMissing(t *testing.T) {
	client := NewMockTransport(DefaultFixtures())

	_, err := client.Do(context.Background(), Request{
		Method: "GET",
		Path:   "/missing",
	})
	if err == nil {
		t.Fatal("expected missing fixture error, got nil")
	}

	if !strings.Contains(err.Error(), "mock fixture not found") {
		t.Fatalf("expected missing fixture error message, got %v", err)
	}
}

func TestDefaultFixturesCoverDaySixContracts(t *testing.T) {
	client := NewMockTransport(DefaultFixtures())

	testCases := []struct {
		name   string
		req    Request
		decode func([]byte) error
	}{
		{
			name: "projects_list",
			req:  Request{Method: "GET", Path: "/api/v1/projects"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeProjectsResponse(body)
				return err
			},
		},
		{
			name: "github_installations_sync",
			req:  Request{Method: "POST", Path: "/api/v1/github/app/installations/sync"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeGitHubInstallationsResponse(body)
				return err
			},
		},
		{
			name: "instances_list",
			req:  Request{Method: "GET", Path: "/api/v1/instances"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeInstancesResponse(body)
				return err
			},
		},
		{
			name: "mesh_networks_list",
			req:  Request{Method: "GET", Path: "/api/v1/mesh-networks"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeMeshNetworksResponse(body)
				return err
			},
		},
		{
			name: "clusters_list",
			req:  Request{Method: "GET", Path: "/api/v1/clusters"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeClustersResponse(body)
				return err
			},
		},
		{
			name: "deployment_bindings_list",
			req:  Request{Method: "GET", Path: "/api/v1/projects/prj_demo/deployment-bindings"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeDeploymentBindingsResponse(body)
				return err
			},
		},
		{
			name: "deployment_binding_create",
			req:  Request{Method: "POST", Path: "/api/v1/projects/prj_demo/deployment-bindings"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeDeploymentBinding(body)
				return err
			},
		},
		{
			name: "trace_query",
			req:  Request{Method: "GET", Path: "/api/v1/traces/corr-demo"},
			decode: func(body []byte) error {
				_, err := contracts.DecodeTraceSummary(body)
				return err
			},
		},
		{
			name: "logs_stream",
			req:  Request{Method: "GET", Path: "/ws/logs/stream", Query: map[string]string{"service": "api"}},
			decode: func(body []byte) error {
				_, err := contracts.DecodeLogsStreamPreview(body)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.Do(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}

			if err := tc.decode(response.Body); err != nil {
				t.Fatalf("decode fixture error = %v", err)
			}
		})
	}
}
