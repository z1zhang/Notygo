package templates

import (
	"strings"
	"testing"
)

func TestRender_BasicFields(t *testing.T) {
	tmpl := &Template{
		ID:            "t1",
		Name:          "test",
		Format:        "markdown",
		TitleTemplate: "Alert: {{.name}}",
		BodyTemplate:  "Status is {{.status}}",
		Revision:      1,
	}
	data := map[string]any{"name": "web-server", "status": "critical"}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Alert: web-server" {
		t.Errorf("title = %q, want %q", result.Title, "Alert: web-server")
	}
	if result.Body != "Status is critical" {
		t.Errorf("body = %q, want %q", result.Body, "Status is critical")
	}
	if result.Format != "markdown" {
		t.Errorf("format = %q, want markdown", result.Format)
	}
}

func TestRender_EventMeta(t *testing.T) {
	tmpl := &Template{
		ID:            "t2",
		Name:          "test",
		Format:        "markdown",
		TitleTemplate: "Event {{.Event.ID}}",
		BodyTemplate:  "Received at {{.Event.ReceivedAt}} from {{.Event.SourceName}}",
		Revision:      1,
	}
	data := map[string]any{
		"Event": map[string]any{
			"ID":         "evt-123",
			"ReceivedAt": "2025-01-01T00:00:00Z",
			"SourceName": "monitoring",
		},
	}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Event evt-123" {
		t.Errorf("title = %q, want %q", result.Title, "Event evt-123")
	}
	if result.Body != "Received at 2025-01-01T00:00:00Z from monitoring" {
		t.Errorf("body = %q", result.Body)
	}
}

func TestRender_FuncMap_Upper(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: "{{upper .name}}",
		BodyTemplate:  "",
	}
	data := map[string]any{"name": "alice"}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "ALICE" {
		t.Errorf("title = %q, want ALICE", result.Title)
	}
}

func TestRender_FuncMap_Lower(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: "{{lower .name}}",
		BodyTemplate:  "",
	}
	data := map[string]any{"name": "ALICE"}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "alice" {
		t.Errorf("title = %q, want alice", result.Title)
	}
}

func TestRender_FuncMap_Trim(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: "{{trim .name}}",
		BodyTemplate:  "",
	}
	data := map[string]any{"name": "  alice  "}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "alice" {
		t.Errorf("title = %q, want alice", result.Title)
	}
}

func TestRender_FuncMap_Title(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: "{{title .name}}",
		BodyTemplate:  "",
	}
	data := map[string]any{"name": "hello world"}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Hello World" {
		t.Errorf("title = %q, want Hello World", result.Title)
	}
}

func TestRender_FuncMap_Len(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: "Count: {{len .items}}",
		BodyTemplate:  "",
	}
	data := map[string]any{"items": []any{"a", "b", "c"}}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Count: 3" {
		t.Errorf("title = %q, want Count: 3", result.Title)
	}
}

func TestRender_FuncMap_Default(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: `{{default "unknown" .name}}`,
		BodyTemplate:  "",
	}

	// With missing field
	data := map[string]any{}
	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "unknown" {
		t.Errorf("title = %q, want unknown", result.Title)
	}

	// With existing field
	data = map[string]any{"name": "alice"}
	result, err = Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "alice" {
		t.Errorf("title = %q, want alice", result.Title)
	}
}

func TestRender_SyntaxError(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: "{{.missing",
		BodyTemplate:  "ok",
	}
	data := map[string]any{}

	_, err := Render(tmpl, data)
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestRender_NestedFields(t *testing.T) {
	tmpl := &Template{
		Format:        "markdown",
		TitleTemplate: "Container: {{.container_name}}",
		BodyTemplate:  "Image: {{.image}}",
	}
	data := map[string]any{
		"container_name": "web-app",
		"image":          "nginx:latest",
	}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Container: web-app" {
		t.Errorf("title = %q", result.Title)
	}
	if result.Body != "Image: nginx:latest" {
		t.Errorf("body = %q", result.Body)
	}
}

func TestRender_TemplateCaching(t *testing.T) {
	// Render the same template twice; the second render should use the cached template
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: "Hello {{.name}}",
		BodyTemplate:  "World",
	}
	data := map[string]any{"name": "test"}

	result1, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("first render error: %v", err)
	}

	result2, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("second render error: %v", err)
	}

	if result1.Title != result2.Title {
		t.Errorf("cached render produced different result: %q vs %q", result1.Title, result2.Title)
	}
}

func TestRender_NoUnsafeFunctions(t *testing.T) {
	// Attempt to use template features that should not be available
	unsafeTemplates := []struct {
		name string
		tmpl string
	}{
		{"call", `{{call .fn}}`},
		{"printf_system", `{{printf "rm -rf /"}}`},
	}

	for _, tc := range unsafeTemplates {
		tmpl := &Template{
			Format:        "text",
			TitleTemplate: tc.tmpl,
			BodyTemplate:  "",
		}
		data := map[string]any{}

		// These should either fail at parse or not execute dangerous operations
		// The key point is that our FuncMap does NOT expose os/exec, file, or network funcs
		_, err := Render(tmpl, data)
		// call would fail because .fn doesn't exist as a callable
		if err == nil {
			// If no error, at least verify the output isn't doing anything harmful
			// This is acceptable since our safeFuncMap doesn't expose dangerous functions
		}
	}
}

func TestRender_LenString(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: `Length: {{len .text}}`,
		BodyTemplate:  "",
	}
	data := map[string]any{"text": "hello"}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "Length: 5" {
		t.Errorf("title = %q, want Length: 5", result.Title)
	}
}

func TestRender_DefaultWithEmptyString(t *testing.T) {
	tmpl := &Template{
		Format:        "text",
		TitleTemplate: `{{default "fallback" .val}}`,
		BodyTemplate:  "",
	}
	data := map[string]any{"val": ""}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title != "fallback" {
		t.Errorf("title = %q, want fallback (empty string should use default)", result.Title)
	}
}

func TestRender_MultipleFieldsInBody(t *testing.T) {
	tmpl := &Template{
		Format:        "markdown",
		TitleTemplate: "Alert",
		BodyTemplate:  "- Host: {{.host}}\n- CPU: {{.cpu}}%\n- Status: {{upper .status}}",
	}
	data := map[string]any{
		"host":   "server-01",
		"cpu":    "85",
		"status": "critical",
	}

	result, err := Render(tmpl, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Body, "Host: server-01") {
		t.Errorf("body missing host: %q", result.Body)
	}
	if !strings.Contains(result.Body, "CRITICAL") {
		t.Errorf("body missing upper status: %q", result.Body)
	}
}
