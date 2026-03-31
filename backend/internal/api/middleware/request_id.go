package middleware

import (
	"github.com/gin-gonic/gin"

	"lazyops-server/pkg/utils"
)

const (
	requestIDHeader     = "X-Request-ID"
	correlationIDHeader = "X-Correlation-ID"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = utils.NewRequestID()
		}

		correlationID := c.GetHeader(correlationIDHeader)
		if correlationID == "" {
			correlationID = utils.NewCorrelationID()
		}

		c.Set(requestIDHeader, requestID)
		c.Set(correlationIDHeader, correlationID)
		c.Writer.Header().Set(requestIDHeader, requestID)
		c.Writer.Header().Set(correlationIDHeader, correlationID)
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

func GetCorrelationID(c *gin.Context) string {
	if correlationID, ok := c.Get(correlationIDHeader); ok {
		if value, ok := correlationID.(string); ok {
			return value
		}
	}

	return ""
}
