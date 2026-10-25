package sender

import (
	"encoding/json"
	"testing"
)

func TestBuildBodyFieldsFlat(t *testing.T) {
	fields := []BodyField{
		{Key: "chat_id", ValueSource: "literal", Value: "12345"},
		{Key: "text", ValueSource: "body"},
		{Key: "backup", ValueSource: "body"}, // same value source twice is allowed
	}
	v, err := buildBodyFields(fields, RenderedMessage{Title: "标题", Body: "内容"})
	if err != nil {
		t.Fatalf("buildBodyFields: %v", err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("want object, got %T", v)
	}
	if m["chat_id"] != "12345" || m["text"] != "内容" || m["backup"] != "内容" {
		t.Fatalf("m = %v", m)
	}
}

func TestBuildBodyFieldsNested(t *testing.T) {
	// Discord-style embeds: array of objects
	fields := []BodyField{
		{Key: "username", ValueSource: "literal", Value: "Notygo"},
		{Key: "embeds", Type: "array", Children: []BodyField{
			{Type: "object", Children: []BodyField{
				{Key: "title", ValueSource: "title"},
				{Key: "description", ValueSource: "body"},
			}},
		}},
	}
	v, err := buildBodyFields(fields, RenderedMessage{Title: "镜像更新", Body: "nginx 1.27"})
	if err != nil {
		t.Fatalf("buildBodyFields: %v", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal %q: %v", b, err)
	}
	if got["username"] != "Notygo" {
		t.Fatalf("username = %v", got["username"])
	}
	embeds, ok := got["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("embeds = %v", got["embeds"])
	}
	embed := embeds[0].(map[string]any)
	if embed["title"] != "镜像更新" || embed["description"] != "nginx 1.27" {
		t.Fatalf("embed = %v", embed)
	}
}

func TestBuildBodyFieldsEscapesContent(t *testing.T) {
	fields := []BodyField{{Key: "text", ValueSource: "body"}}
	v, err := buildBodyFields(fields, RenderedMessage{Body: "line1\nline2 \"quoted\""})
	if err != nil {
		t.Fatalf("buildBodyFields: %v", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("content with quotes/newlines broke JSON: %v (body %q)", err, b)
	}
}

func TestBuildBodyFieldsArrayScalars(t *testing.T) {
	// Array of scalars: keys ignored, value sources can repeat
	fields := []BodyField{
		{Key: "tags", Type: "array", Children: []BodyField{
			{ValueSource: "title"},
			{ValueSource: "body"},
		}},
	}
	v, err := buildBodyFields(fields, RenderedMessage{Title: "a", Body: "b"})
	if err != nil {
		t.Fatalf("buildBodyFields: %v", err)
	}
	m := v.(map[string]any)
	arr, ok := m["tags"].([]any)
	if !ok || len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Fatalf("tags = %v", m["tags"])
	}
}

func TestBuildBodyFieldsErrors(t *testing.T) {
	cases := []struct {
		name   string
		fields []BodyField
	}{
		{"missing value source", []BodyField{{Key: "text"}}},
		{"invalid value source", []BodyField{{Key: "text", ValueSource: "unknown"}}},
		{"empty top key", []BodyField{{Key: "", ValueSource: "body"}}},
		{"duplicate key", []BodyField{
			{Key: "a", ValueSource: "body"},
			{Key: "a", ValueSource: "body"},
		}},
		{"empty child key in object", []BodyField{
			{Key: "g", Type: "object", Children: []BodyField{{Key: "", ValueSource: "body"}}},
		}},
	}
	for _, c := range cases {
		if _, err := buildBodyFields(c.fields, RenderedMessage{}); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}
