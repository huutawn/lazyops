package response

import (
	"github.com/gin-gonic/gin"
)

type ErrorBody struct {
	Code      string            `json:"code"`
	Details   any               `json:"details,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
	Retryable bool              `json:"retryable,omitempty"`
}

type Envelope struct {
	Success       bool       `json:"success"`
	Message       string     `json:"message"`
	RequestID     string     `json:"request_id,omitempty"`
	CorrelationID string     `json:"correlation_id,omitempty"`
	Data          any        `json:"data,omitempty"`
	Error         *ErrorBody `json:"error,omitempty"`
	Meta          any        `json:"meta,omitempty"`
}

func JSON(c *gin.Context, status int, message string, data any) {
	JSONWithMeta(c, status, message, data, nil)
}

func JSONWithMeta(c *gin.Context, status int, message string, data any, meta any) {
	c.JSON(status, Envelope{
		Success:       true,
		Message:       message,
		RequestID:     c.Writer.Header().Get("X-Request-ID"),
		CorrelationID: c.Writer.Header().Get("X-Correlation-ID"),
		Data:          data,
		Meta:          meta,
	})
}

func Error(c *gin.Context, status int, message, code string, details any) {
	ErrorWithFields(c, status, message, code, details, nil)
}

func ValidationError(c *gin.Context, status int, message, code string, fields map[string]string, details any) {
	ErrorWithFields(c, status, message, code, details, fields)
}

func ErrorWithFields(c *gin.Context, status int, message, code string, details any, fields map[string]string) {
	c.AbortWithStatusJSON(status, Envelope{
		Success:       false,
		Message:       message,
		RequestID:     c.Writer.Header().Get("X-Request-ID"),
		CorrelationID: c.Writer.Header().Get("X-Correlation-ID"),
		Error: &ErrorBody{
			Code:    code,
			Details: details,
			Fields:  fields,
		},
	})
}
