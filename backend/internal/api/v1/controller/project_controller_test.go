package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
)

func TestDay9ProtectedRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.RequestID())

	projectController := NewProjectController(nil, nil)
	githubController := NewGitHubController(nil)
	targetController := NewTargetController(nil, nil)
	deploymentBindingController := NewDeploymentBindingController(nil)

	protected := router.Group("/api/v1")
	protected.Use(middleware.Authenticate(nil))
	protected.GET("/projects", projectController.List)
	protected.POST("/projects", projectController.Create)
	protected.POST("/projects/:id/repo-link", projectController.LinkRepo)
	protected.POST("/projects/:id/deployment-bindings", deploymentBindingController.Create)
	protected.GET("/github/repos", githubController.ListRepos)
	protected.GET("/mesh-networks", targetController.ListMeshNetworks)
	protected.POST("/mesh-networks", targetController.CreateMeshNetwork)
	protected.GET("/clusters", targetController.ListClusters)
	protected.POST("/clusters", targetController.CreateCluster)

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "list projects", method: http.MethodGet, target: "/api/v1/projects"},
		{name: "create project", method: http.MethodPost, target: "/api/v1/projects", body: `{"name":"Acme"}`},
		{name: "link repo", method: http.MethodPost, target: "/api/v1/projects/prj_123/repo-link", body: `{"github_installation_id":1,"github_repo_id":2}`},
		{name: "create deployment binding", method: http.MethodPost, target: "/api/v1/projects/prj_123/deployment-bindings", body: `{"name":"prod binding","target_ref":"prod-main","runtime_mode":"standalone","target_kind":"instance","target_id":"inst_123"}`},
		{name: "list github repos", method: http.MethodGet, target: "/api/v1/github/repos"},
		{name: "list mesh networks", method: http.MethodGet, target: "/api/v1/mesh-networks"},
		{name: "create mesh network", method: http.MethodPost, target: "/api/v1/mesh-networks", body: `{"name":"mesh-prod","provider":"wireguard","cidr":"10.20.0.0/24"}`},
		{name: "list clusters", method: http.MethodGet, target: "/api/v1/clusters"},
		{name: "create cluster", method: http.MethodPost, target: "/api/v1/clusters", body: `{"name":"cluster-prod","provider":"k3s","kubeconfig_secret_ref":"secret://clusters/prod"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := bytes.NewBufferString(tc.body)
			req := httptest.NewRequest(tc.method, tc.target, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"code":"missing_bearer_token"`) {
				t.Fatalf("expected missing_bearer_token response, got %s", rec.Body.String())
			}
		})
	}
}
