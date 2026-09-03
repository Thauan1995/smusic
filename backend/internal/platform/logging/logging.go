// Package logging configures structured logging (backend-go.md §5: "slog
// ... com formato JSON em produção, texto legível em dev"). Never log
// passwords, tokens, or (per security.md's cross-cutting rule, relevant
// once presence ships in Fatia 2) exact location coordinates.
package logging

import (
	"log/slog"
	"os"
)

// New returns a slog.Logger: JSON handler when ENV=production, a
// human-readable text handler otherwise.
func New() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}

	var handler slog.Handler
	if os.Getenv("ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler)
}
