package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	apiresponse "lazyops-server/internal/api/response"
)

func resetVisitorsForTest() {
	visitorsMu.Lock()
	defer visitorsMu.Unlock()
	visitors = map[string]*visitor{}
}

func TestScopedRateLimitBlocksCLIAbuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetVisitorsForTest()

	router := gin.New()
	router.Use(RequestID())
	router.POST("/api/v1/auth/cli-login", ScopedRateLimit("auth:cli-login:test", 1, 1), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/cli-login", nil)
	firstReq.RemoteAddr = "127.0.0.1:4000"
	firstRes := httptest.NewRecorder()
	router.ServeHTTP(firstRes, firstReq)

	if firstRes.Code != http.StatusOK {
		t.Fatalf("expected first request to pass, got %d", firstRes.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/cli-login", nil)
	secondReq.RemoteAddr = "127.0.0.1:4000"
	secondRes := httptest.NewRecorder()
	router.ServeHTTP(secondRes, secondReq)

	if secondRes.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be rate limited, got %d", secondRes.Code)
	}

	var envelope apiresponse.Envelope
	if err := json.Unmarshal(secondRes.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if envelope.Error == nil || envelope.Error.Code != "rate_limited" {
		t.Fatalf("expected rate_limited error code, got %+v", envelope.Error)
	}
}
