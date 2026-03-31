package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lazyops-server/internal/api/response"
	"lazyops-server/pkg/logger"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error("panic recovered", "panic", recovered)
		response.Error(c, http.StatusInternalServerError, "internal server error", "internal_error", nil)
	})
}
