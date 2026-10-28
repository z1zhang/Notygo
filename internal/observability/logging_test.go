package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   slog.Level
		wantOK bool
	}{
		{name: "debug", input: "debug", want: slog.LevelDebug, wantOK: true},
		{name: "info", input: "info", want: slog.LevelInfo, wantOK: true},
		{name: "warn", input: "warn", want: slog.LevelWarn, wantOK: true},
		{name: "error", input: "error", want: slog.LevelError, wantOK: true},
		{name: "case insensitive", input: "WARN", want: slog.LevelWarn, wantOK: true},
		{name: "unset env var maps to info", input: "", want: slog.LevelInfo, wantOK: true},
		{name: "typo rejected", input: "inof", want: slog.LevelInfo, wantOK: false},
		{name: "level value rejected", input: "3", want: slog.LevelInfo, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseLevel(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ParseLevel(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestJSONOutputLevelAndFiltering pins the two properties SetupLogger relies
// on from slog.NewJSONHandler: levels render as the standard uppercase strings
// in the JSON stream, and records below the configured level are dropped.
func TestJSONOutputLevelAndFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	logger.Debug("hidden")
	logger.Info("hidden too")
	logger.Warn("shown warn")
	logger.Error("shown error")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d log lines, want 2 (debug/info filtered at warn level): %q", len(lines), buf.String())
	}

	for i, wantLevel := range []string{"WARN", "ERROR"} {
		var rec struct {
			Level string `json:"level"`
			Msg   string `json:"msg"`
		}
		if err := json.Unmarshal([]byte(lines[i]), &rec); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", i, err)
		}
		if rec.Level != wantLevel {
			t.Errorf("line %d level = %q, want %q", i, rec.Level, wantLevel)
		}
		if !strings.HasPrefix(rec.Msg, "shown") {
			t.Errorf("line %d msg = %q, want a record emitted at or above warn", i, rec.Msg)
		}
	}
}
