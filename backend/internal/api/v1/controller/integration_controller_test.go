package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
	"lazyops-server/internal/service"
)

func TestGitHubWebhookControllerRejectsInvalidSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.RequestID())
	controller := NewIntegrationController(service.NewGitHubWebhookService("secret", new(controllerRouteResolver)))
	router.POST("/api/v1/integrations/github/webhook", controller.GitHubWebhook)

	payload := `{"ref":"refs/heads/main","after":"abc123","installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/github/webhook", strings.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery_test")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"invalid_signature"`) {
		t.Fatalf("expected invalid_signature, got %s", rec.Body.String())
	}
}

func TestGitHubWebhookControllerAcceptsIgnoredUnlinkedRepo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.RequestID())
	controller := NewIntegrationController(service.NewGitHubWebhookService("secret", new(controllerRouteResolver)))
	router.POST("/api/v1/integrations/github/webhook", controller.GitHubWebhook)

	payload := `{"ref":"refs/heads/main","after":"abc123","installation":{"id":100},"repository":{"id":42,"name":"backend","full_name":"lazyops/backend","owner":{"login":"lazyops"}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/github/webhook", strings.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery_test")
	req.Header.Set("X-Hub-Signature-256", signControllerWebhook("secret", []byte(payload)))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ignored"`) {
		t.Fatalf("expected ignored status, got %s", rec.Body.String())
	}
}

type controllerRouteResolver struct{}

func (r *controllerRouteResolver) LookupWebhookRoute(cmd service.WebhookRouteLookupCommand) (*service.ProjectRepoLinkRecord, error) {
	return nil, service.ErrRepoLinkNotFound
}

func signControllerWebhook(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return fmt.Sprintf("sha256=%x", mac.Sum(nil))
}
