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

// HTTPWebhookConfig holds the configuration for a generic HTTP webhook sender.
type HTTPWebhookConfig struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`        // GET, POST, PUT, etc.
	Headers      map[string]string `json:"headers"`       // Custom headers
	BodyTemplate string            `json:"body_template"` // Optional Go template for body
}

// HTTPWebhookSender sends messages via generic HTTP webhooks with SSRF protection.
type HTTPWebhookSender struct{}

// NewHTTPWebhookSender creates a new HTTPWebhookSender.
func NewHTTPWebhookSender() *HTTPWebhookSender {
	return &HTTPWebhookSender{}
}

// Send delivers a message via HTTP webhook with SSRF protection.
func (s *HTTPWebhookSender) Send(ctx context.Context, destConfig DestinationConfig, msg RenderedMessage) (*SendResult, error) {
	var cfg HTTPWebhookConfig
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

	// Build request body
	var bodyReader io.Reader
	if method != http.MethodGet && method != http.MethodHead {
		// If a body template is provided, use it; otherwise send the rendered message as JSON
		if cfg.BodyTemplate != "" {
			bodyReader = strings.NewReader(cfg.BodyTemplate)
		} else {
			payload := map[string]string{
				"title":  msg.Title,
				"body":   msg.Body,
				"format": msg.Format,
			}
			bodyBytes, err := json.Marshal(payload)
			if err != nil {
				return nil, fmt.Errorf("marshal webhook payload: %w", err)
			}
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
	req.Header.Set("User-Agent", "zPush/1.0")

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
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
	}

	// Determine retryability
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		result.IsRetryable = true
	} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		result.IsRetryable = false
	}

	return result, nil
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
