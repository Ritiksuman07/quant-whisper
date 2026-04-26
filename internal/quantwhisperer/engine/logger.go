package engine

import (
	"io"
	"log/slog"
)

func fallbackLogger(component string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil)).With(slog.String("component", component))
}
