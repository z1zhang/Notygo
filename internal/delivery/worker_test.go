package delivery

import (
	"encoding/json"
	"strings"
	"testing"

	"zpush/internal/sender"
)

func TestBuildRequestSummary(t *testing.T) {
	t.Run("chat sender omits empty method/url, always records body", func(t *testing.T) {
		summary := buildRequestSummary(
			sender.DestinationConfig{Type: "dingtalk"},
			sender.RenderedMessage{},
		)
		var m map[string]any
		if err := json.Unmarshal([]byte(summary), &m); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if _, ok := m["method"]; ok {
			t.Errorf("empty method should be omitted, got %v", m["method"])
		}
		if _, ok := m["url"]; ok {
			t.Errorf("empty url should be omitted, got %v", m["url"])
		}
		if v, _ := m["body_snippet"].(string); v != "" {
			t.Errorf("body_snippet should be present and empty, got %q", v)
		}
		if m["type"] != "dingtalk" {
			t.Errorf("type = %v, want dingtalk", m["type"])
		}
	})

	t.Run("http sender keeps method/url and truncates body", func(t *testing.T) {
		longBody := make([]byte, 600)
		for i := range longBody {
			longBody[i] = 'x'
		}
		summary := buildRequestSummary(
			sender.DestinationConfig{Type: "http", Method: "POST", URL: "https://example.com/hook"},
			sender.RenderedMessage{Title: "标题", Body: string(longBody)},
		)
		var m map[string]any
		if err := json.Unmarshal([]byte(summary), &m); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if m["method"] != "POST" || m["url"] != "https://example.com/hook" {
			t.Errorf("method/url = %v/%v", m["method"], m["url"])
		}
		if m["title_snippet"] != "标题" {
			t.Errorf("title_snippet = %v", m["title_snippet"])
		}
		if v, _ := m["body_snippet"].(string); len(v) != 512+len("...(truncated)") {
			t.Errorf("body_snippet length = %d, want %d", len(v), 512+len("...(truncated)"))
		}
	})
}

func TestAttachRequestPayload(t *testing.T) {
	base := `{"type":"http","method":"POST","url":"https://example.com/hook","body_snippet":"hi"}`

	t.Run("merges payload into summary", func(t *testing.T) {
		out := attachRequestPayload(base, `{"text":"通知内容"}`)
		var m map[string]any
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if m["payload"] != `{"text":"通知内容"}` {
			t.Errorf("payload = %v", m["payload"])
		}
		if m["method"] != "POST" || m["url"] != "https://example.com/hook" {
			t.Errorf("existing fields lost: %v", m)
		}
	})

	t.Run("truncates over 2048 bytes", func(t *testing.T) {
		long := `{"text":"` + strings.Repeat("x", 3000) + `"}`
		out := attachRequestPayload(base, long)
		var m map[string]any
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		v, _ := m["payload"].(string)
		if len(v) != 2048+len("...(truncated)") {
			t.Errorf("payload length = %d", len(v))
		}
	})

	t.Run("invalid summary json returns unchanged", func(t *testing.T) {
		if out := attachRequestPayload("not-json", "x"); out != "not-json" {
			t.Errorf("out = %q", out)
		}
	})
}
