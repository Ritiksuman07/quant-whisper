package cmd

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

func newLogger(format string) *slog.Logger {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		normalized = "text"
	}

	handlerOptions := &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				attr.Key = "timestamp"
			}
			return attr
		},
	}

	var handler slog.Handler
	if normalized == "json" {
		handler = slog.NewJSONHandler(os.Stderr, handlerOptions)
	} else {
		handler = slog.NewTextHandler(os.Stderr, handlerOptions)
	}
	return slog.New(handler)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
