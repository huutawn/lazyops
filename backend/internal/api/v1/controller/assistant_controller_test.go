package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/middleware"
)

func TestAssistantControllerProtectedRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestID())
	ctl := NewAssistantController(nil)
	protected := router.Group("/api/v1")
	protected.Use(middleware.Authenticate(nil))
	protected.POST("/assistant/sessions", ctl.CreateSession)
	protected.GET("/assistant/sessions", ctl.ListSessions)
	protected.GET("/assistant/sessions/:id", ctl.GetSession)
	protected.POST("/assistant/sessions/:id/messages", ctl.PostMessage)
	protected.POST("/assistant/action-plans/:id/confirm", ctl.ConfirmPlan)
	protected.POST("/assistant/action-plans/:id/cancel", ctl.CancelPlan)

	tests := []struct {
		method string
		target string
		body   string
	}{
		{method: http.MethodPost, target: "/api/v1/assistant/sessions", body: `{}`},
		{method: http.MethodGet, target: "/api/v1/assistant/sessions"},
		{method: http.MethodGet, target: "/api/v1/assistant/sessions/asst_123"},
		{method: http.MethodPost, target: "/api/v1/assistant/sessions/asst_123/messages", body: `{"content":"deploy branch main to production"}`},
		{method: http.MethodPost, target: "/api/v1/assistant/action-plans/plan_123/confirm", body: `{}`},
		{method: http.MethodPost, target: "/api/v1/assistant/action-plans/plan_123/cancel", body: `{}`},
	}

	for _, tc := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.target, bytes.NewBufferString(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.target, rec.Code)
		}
	}
}
