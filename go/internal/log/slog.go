package log

import (
	"io"
	"log/slog"
	"strings"
)

// NewJSONLogger returns a slog.Logger that writes JSON-encoded records to w.
// level is a case-insensitive string ("debug", "info", "warn", "error").
// Unknown values fall back to info. The returned logger has no source
// (caller info) attached — keeps the line compact for log aggregation.
func NewJSONLogger(w io.Writer, level string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     parseLevel(level),
		AddSource: false,
	})
	return slog.New(h)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
