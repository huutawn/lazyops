package logger

import (
	"log/slog"
	"os"
)

func New(level slog.Level) *slog.Logger {
	base := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(NewRedactingHandler(base))
}
