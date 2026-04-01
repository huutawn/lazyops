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

	protected := router.Group("/api/v1")
	protected.Use(middleware.Authenticate(nil))
	protected.GET("/projects", projectController.List)
	protected.POST("/projects", projectController.Create)
	protected.POST("/projects/:id/repo-link", projectController.LinkRepo)
	protected.GET("/github/repos", githubController.ListRepos)

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "list projects", method: http.MethodGet, target: "/api/v1/projects"},
		{name: "create project", method: http.MethodPost, target: "/api/v1/projects", body: `{"name":"Acme"}`},
		{name: "link repo", method: http.MethodPost, target: "/api/v1/projects/prj_123/repo-link", body: `{"github_installation_id":1,"github_repo_id":2}`},
		{name: "list github repos", method: http.MethodGet, target: "/api/v1/github/repos"},
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
