package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPConfig holds the configuration for a generic HTTP webhook sender.
type HTTPConfig struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`      // GET, POST, PUT, etc.
	Headers    map[string]string `json:"headers"`     // Custom headers
	BodyFields []BodyField       `json:"body_fields"` // Structured body tree
}

// BodyField is one node of the request-body tree. A leaf maps a JSON key to
// a value source ("title"|"body"|"format"|"literal"); a group (type
// "object"|"array") nests children under a key. Multiple keys may map the
// same endpoint. Inside an array, child keys are ignored (position decides).
type BodyField struct {
	Key         string      `json:"key"`
	ValueSource string      `json:"value_source,omitempty"` // leaf only
	Value       string      `json:"value,omitempty"`        // literal endpoint only
	Type        string      `json:"type,omitempty"`         // group only: "object" | "array"
	Children    []BodyField `json:"children,omitempty"`
}

func (f BodyField) isGroup() bool {
	return f.Type == "object" || f.Type == "array"
}

// HTTPSender sends messages via generic HTTP webhooks with SSRF protection.
type HTTPSender struct{}

// NewHTTPSender creates a new HTTPSender.
func NewHTTPSender() *HTTPSender {
	return &HTTPSender{}
}

// Send delivers a message via HTTP webhook with SSRF protection.
func (s *HTTPSender) Send(ctx context.Context, destConfig ChannelConfig, msg RenderedMessage) (*SendResult, error) {
	var cfg HTTPConfig
	if err := json.Unmarshal(destConfig.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid webhook config: %w", err)
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}

	// SSRF check: resolve the host and reject private/internal IPs
	if err := checkSSRF(cfg.URL); err != nil {
		return nil, fmt.Errorf("SSRF protection: %w", err)
	}

	method := cfg.Method
	if method == "" {
		method = http.MethodPost
	}

	// Build request body; the exact bytes are captured for the delivery attempt log.
	var bodyReader io.Reader
	var wirePayload string
	if method != http.MethodGet && method != http.MethodHead {
		// Priority: body_fields (structured tree) > default {title, body, format}
		switch {
		case len(cfg.BodyFields) > 0:
			bodyValue, err := buildBodyFields(cfg.BodyFields, msg)
			if err != nil {
				return nil, fmt.Errorf("invalid body_fields: %w", err)
			}
			bodyBytes, err := json.Marshal(bodyValue)
			if err != nil {
				return nil, fmt.Errorf("marshal webhook payload: %w", err)
			}
			wirePayload = string(bodyBytes)
			bodyReader = bytes.NewReader(bodyBytes)
		default:
			payload := map[string]string{
				"title":  msg.Title,
				"body":   msg.Body,
				"format": msg.Format,
			}
			bodyBytes, err := json.Marshal(payload)
			if err != nil {
				return nil, fmt.Errorf("marshal webhook payload: %w", err)
			}
			wirePayload = string(bodyBytes)
			bodyReader = bytes.NewReader(bodyBytes)
		}
	}

	timeout := destConfig.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create webhook request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Notygo/1.0")

	// Apply custom headers (can override defaults)
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read webhook response: %w", err)
	}

	result := &SendResult{
		StatusCode:     resp.StatusCode,
		Body:           string(respBody),
		RequestPayload: wirePayload,
	}

	// Determine retryability
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		result.IsRetryable = true
	} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		result.IsRetryable = false
	}

	return result, nil
}

// buildBodyFields assembles the top-level JSON object from the body field tree.
func buildBodyFields(fields []BodyField, msg RenderedMessage) (any, error) {
	root := make(map[string]any, len(fields))
	for _, f := range fields {
		if f.Key == "" {
			return nil, fmt.Errorf("top-level field with empty key")
		}
		if _, dup := root[f.Key]; dup {
			return nil, fmt.Errorf("duplicate key %q", f.Key)
		}
		if f.isGroup() {
			v, err := buildGroupValue(f, msg, "at "+f.Key)
			if err != nil {
				return nil, err
			}
			root[f.Key] = v
			continue
		}
		v, err := resolveValueSource(f, msg)
		if err != nil {
			return nil, err
		}
		root[f.Key] = v
	}
	return root, nil
}

// buildGroupValue assembles an object or array group node.
// Keys of children are ignored inside arrays (position decides).
func buildGroupValue(g BodyField, msg RenderedMessage, where string) (any, error) {
	if g.Type == "array" {
		arr := make([]any, 0, len(g.Children))
		for i, c := range g.Children {
			if c.isGroup() {
				v, err := buildGroupValue(c, msg, fmt.Sprintf("%s[%d]", where, i))
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
				continue
			}
			v, err := resolveValueSource(c, msg)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", where, i, err)
			}
			arr = append(arr, v)
		}
		return arr, nil
	}

	// object
	m := make(map[string]any, len(g.Children))
	for _, c := range g.Children {
		if c.Key == "" {
			return nil, fmt.Errorf("%s: child field with empty key", where)
		}
		if _, dup := m[c.Key]; dup {
			return nil, fmt.Errorf("%s: duplicate key %q", where, c.Key)
		}
		if c.isGroup() {
			v, err := buildGroupValue(c, msg, where+"."+c.Key)
			if err != nil {
				return nil, err
			}
			m[c.Key] = v
			continue
		}
		v, err := resolveValueSource(c, msg)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", where, c.Key, err)
		}
		m[c.Key] = v
	}
	return m, nil
}

// resolveValueSource maps a leaf to its value from the rendered message.
func resolveValueSource(f BodyField, msg RenderedMessage) (string, error) {
	switch f.ValueSource {
	case "title":
		return msg.Title, nil
	case "body":
		return msg.Body, nil
	case "format":
		return msg.Format, nil
	case "literal":
		return f.Value, nil
	default:
		if f.ValueSource == "" {
			return "", fmt.Errorf("field %q has no value source", f.Key)
		}
		return "", fmt.Errorf("field %q has invalid value source %q", f.Key, f.ValueSource)
	}
}

// checkSSRF resolves the host from the URL and rejects private/internal IP addresses.
func checkSSRF(rawURL string) error {
	host := extractHost(rawURL)
	if host == "" {
		return fmt.Errorf("empty host in URL %q", rawURL)
	}

	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}

		if isBlockedIP(ip) {
			return fmt.Errorf("target IP %s is blocked (private/internal/reserved)", ipStr)
		}
	}

	return nil
}

// extractHost extracts the hostname from a URL string.
func extractHost(rawURL string) string {
	// Simple extraction: handle scheme://host:port/path
	s := rawURL

	// Remove scheme
	if idx := strings.Index(s, "://"); idx != -1 {
		s = s[idx+3:]
	}

	// Remove path
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}

	// Remove port
	if idx := strings.LastIndex(s, ":"); idx != -1 {
		// Make sure it's not an IPv6 address bracket
		if s[idx-1:len(s)] != "]" {
			s = s[:idx]
		}
	}

	// Remove brackets for IPv6
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	return s
}

// isBlockedIP returns true if the IP address is in a blocked range.
// Blocked ranges: loopback, private, link-local, cloud metadata.
func isBlockedIP(ip net.IP) bool {
	// Loopback: 127.x.x.x, ::1
	if ip.IsLoopback() {
		return true
	}

	// Unspecified: 0.0.0.0, ::
	if ip.IsUnspecified() {
		return true
	}

	// Private ranges
	if ip.IsPrivate() {
		return true
	}

	// Link-local: 169.254.x.x, fe80::
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Explicit check for cloud metadata endpoint: 169.254.169.254
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}

	return false
}
