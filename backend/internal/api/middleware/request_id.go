package middleware

import (
	"github.com/gin-gonic/gin"

	"lazyops-server/pkg/utils"
)

const requestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = utils.NewRequestID()
		}

		c.Set(requestIDHeader, requestID)
		c.Writer.Header().Set(requestIDHeader, requestID)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	if requestID, ok := c.Get(requestIDHeader); ok {
		if value, ok := requestID.(string); ok {
			return value
		}
	}

	return ""
}
