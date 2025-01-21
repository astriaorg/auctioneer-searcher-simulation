package main

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
)

type DetailedLogHandler struct {
	wrapped slog.Handler
}

func NewDetailedLogHandler(handler slog.Handler) DetailedLogHandler {
	return DetailedLogHandler{wrapped: handler}
}

func (h *DetailedLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.wrapped.Enabled(ctx, level)
}

func (h *DetailedLogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Get the caller information
	pc, _, line, ok := runtime.Caller(3)
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			// Extract function name (strip package path)
			parts := strings.Split(fn.Name(), ".")
			functionName := parts[len(parts)-1]

			// Add function name, file, and line to attributes
			record.AddAttrs(
				slog.String("function", functionName),
				slog.Int("line", line),
			)
		}
	}

	// Pass the modified record to the wrapped handler
	return h.wrapped.Handle(ctx, record)
}

func (h *DetailedLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DetailedLogHandler{wrapped: h.wrapped.WithAttrs(attrs)}
}

func (h *DetailedLogHandler) WithGroup(name string) slog.Handler {
	return &DetailedLogHandler{wrapped: h.wrapped.WithGroup(name)}
}
