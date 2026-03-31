package logger

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

const redactedValue = "[REDACTED]"

var sensitiveKeyParts = []string{
	"token",
	"secret",
	"password",
	"authorization",
	"bootstrap",
	"mesh_key",
	"private_key",
	"api_key",
}

type RedactingHandler struct {
	next slog.Handler
}

func NewRedactingHandler(next slog.Handler) slog.Handler {
	return RedactingHandler{next: next}
}

func (h RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func (h RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return RedactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h RedactingHandler) WithGroup(name string) slog.Handler {
	return RedactingHandler{next: h.next.WithGroup(name)}
}

func RedactField(key string, value any) any {
	switch typed := value.(type) {
	case string:
		if isSensitiveKey(key) {
			if strings.HasPrefix(typed, "Bearer ") {
				return "Bearer " + redactedValue
			}
			return redactedValue
		}
		return redactString(key, typed)
	case []string:
		if isSensitiveKey(key) {
			out := make([]string, 0, len(typed))
			for range typed {
				out = append(out, redactedValue)
			}
			return out
		}
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, fmt.Sprint(RedactField(key, item)))
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(typed))
		for childKey, childValue := range typed {
			out[childKey] = fmt.Sprint(RedactField(childKey, childValue))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			out[childKey] = RedactField(childKey, childValue)
		}
		return out
	default:
		if isSensitiveKey(key) {
			return redactedValue
		}
		return value
	}
}

func redactAttr(attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		redacted := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			redacted = append(redacted, redactAttr(child))
		}
		return slog.Attr{
			Key:   attr.Key,
			Value: slog.GroupValue(redacted...),
		}
	}

	return slog.Any(attr.Key, RedactField(attr.Key, attr.Value.Any()))
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, part := range sensitiveKeyParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func redactString(key, value string) string {
	if strings.Contains(strings.ToLower(key), "authorization") && strings.HasPrefix(value, "Bearer ") {
		return "Bearer " + redactedValue
	}
	if strings.HasPrefix(value, "Bearer ") {
		return "Bearer " + redactedValue
	}
	return value
}
