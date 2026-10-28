package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// ParseLevel maps a LOG_LEVEL value to a slog level. It returns ok=false for
// unrecognized values so SetupLogger can warn instead of silently ignoring a
// typo like "inof". The empty string maps to info (env var unset).
func ParseLevel(level string) (lvl slog.Level, ok bool) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, true
	case "info", "":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

// SetupLogger configures the global slog logger with JSON output at the given
// LOG_LEVEL value. It announces the effective level — logged at that very
// level, so the line survives even a LOG_LEVEL=error configuration — giving
// operators startup proof the setting took effect.
func SetupLogger(level string) {
	lvl, ok := ParseLevel(level)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(handler))
	if !ok {
		slog.Warn("invalid LOG_LEVEL value, falling back to info", "value", level)
	}
	slog.Log(context.Background(), lvl, "logger initialized", "level", lvl.String())
}
