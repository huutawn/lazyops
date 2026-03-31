package response

import "github.com/gin-gonic/gin"

type Envelope struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Data      any    `json:"data,omitempty"`
	Error     any    `json:"error,omitempty"`
}

func JSON(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Envelope{
		Success:   true,
		Message:   message,
		RequestID: c.Writer.Header().Get("X-Request-ID"),
		Data:      data,
	})
}

func Error(c *gin.Context, status int, message string, details any) {
	c.AbortWithStatusJSON(status, Envelope{
		Success:   false,
		Message:   message,
		RequestID: c.Writer.Header().Get("X-Request-ID"),
		Error:     details,
	})
}
