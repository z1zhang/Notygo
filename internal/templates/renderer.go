package templates

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"
)

// EventMeta contains event metadata accessible in templates.
type EventMeta struct {
	ID           string
	ReceivedAt   string
	EndpointName string
}

// RenderedMessage holds the output of template rendering.
type RenderedMessage struct {
	Title  string
	Body   string
	Format string
}

// safeFuncMap returns a restricted FuncMap that only allows safe operations.
// No os/exec, file, or network functions are permitted.
func safeFuncMap() template.FuncMap {
	return template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"trim":  strings.TrimSpace,
		"title": func(s string) string {
			// Simple title-case: capitalize first letter of each word
			words := strings.Fields(s)
			for i, w := range words {
				if len(w) > 0 {
					words[i] = strings.ToUpper(w[:1]) + w[1:]
				}
			}
			return strings.Join(words, " ")
		},
		"len": func(v any) (int, error) {
			switch x := v.(type) {
			case string:
				return len(x), nil
			case []any:
				return len(x), nil
			case map[string]any:
				return len(x), nil
			default:
				return 0, fmt.Errorf("len: unsupported type %T", v)
			}
		},
		"default": func(def any, val any) any {
			if val == nil || val == "" {
				return def
			}
			if s, ok := val.(string); ok && s == "" {
				return def
			}
			return val
		},
	}
}

// templateCache caches parsed templates by content hash to avoid re-parsing.
var templateCache sync.Map

// templateHash computes a SHA-256 hash of title + body content for cache keying.
func templateHash(titleTmpl, bodyTmpl string) string {
	h := sha256.New()
	h.Write([]byte(titleTmpl))
	h.Write([]byte(bodyTmpl))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// getOrParseTemplate retrieves a cached template or parses and caches a new one.
func getOrParseTemplate(key, content string) (*template.Template, error) {
	if cached, ok := templateCache.Load(key); ok {
		return cached.(*template.Template), nil
	}

	tmpl, err := template.New(key).Funcs(safeFuncMap()).Parse(content)
	if err != nil {
		return nil, fmt.Errorf("template parse error: %w", err)
	}

	// Store in cache; if there's a race, the last one wins which is fine
	// since they produce equivalent templates.
	templateCache.Store(key, tmpl)
	return tmpl, nil
}

// Render executes a Template with the given data and returns the rendered message.
// The data map is passed directly to the Go template engine, so template authors
// can access any key directly: {{ .tag_values }}, {{ .Event.ID }}, etc.
// Template execution is limited to 5 seconds via context timeout.
func Render(tmpl *Template, data map[string]any) (*RenderedMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a channel to get the result with timeout support
	type renderResult struct {
		msg *RenderedMessage
		err error
	}
	resultCh := make(chan renderResult, 1)

	go func() {
		titleKey := templateHash(tmpl.TitleTemplate, "") + "_title"
		bodyKey := templateHash("", tmpl.BodyTemplate) + "_body"

		titleTmpl, err := getOrParseTemplate(titleKey, tmpl.TitleTemplate)
		if err != nil {
			resultCh <- renderResult{nil, fmt.Errorf("title template error: %w", err)}
			return
		}

		bodyTmpl, err := getOrParseTemplate(bodyKey, tmpl.BodyTemplate)
		if err != nil {
			resultCh <- renderResult{nil, fmt.Errorf("body template error: %w", err)}
			return
		}

		var titleBuf, bodyBuf strings.Builder
		if err := titleTmpl.Execute(&titleBuf, data); err != nil {
			resultCh <- renderResult{nil, fmt.Errorf("title render error: %w", err)}
			return
		}
		if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
			resultCh <- renderResult{nil, fmt.Errorf("body render error: %w", err)}
			return
		}

		resultCh <- renderResult{
			msg: &RenderedMessage{
				Title:  titleBuf.String(),
				Body:   bodyBuf.String(),
				Format: tmpl.Format,
			},
			err: nil,
		}
	}()

	select {
	case result := <-resultCh:
		return result.msg, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("template rendering timed out after 5 seconds")
	}
}
