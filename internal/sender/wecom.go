package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WeComConfig holds the configuration for a WeCom (企业微信) bot webhook sender.
type WeComConfig struct {
	WebhookURL string `json:"webhook_url"`
	MsgType    string `json:"msg_type"` // "markdown", "text", "image", "news"
}

// WeComSender sends messages via WeCom robot webhooks.
type WeComSender struct{}

// NewWeComSender creates a new WeComSender.
func NewWeComSender() *WeComSender {
	return &WeComSender{}
}

// Send delivers a message to WeCom via the configured webhook.
func (s *WeComSender) Send(ctx context.Context, destConfig DestinationConfig, msg RenderedMessage) (*SendResult, error) {
	var cfg WeComConfig
	if err := json.Unmarshal(destConfig.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid wecom config: %w", err)
	}

	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	msgType := cfg.MsgType
	if msgType == "" {
		msgType = "markdown"
	}

	// Build request body based on message type
	var payload any
	switch msgType {
	case "text":
		payload = map[string]any{
			"msgtype": "text",
			"text": map[string]any{
				"content": msg.Body,
			},
		}
	case "image":
		// For image type, the body should contain the base64 image data and MD5
		// This is typically not rendered from templates, so we send body as content
		payload = map[string]any{
			"msgtype": "text",
			"text": map[string]any{
				"content": msg.Body,
			},
		}
	case "news":
		payload = map[string]any{
			"msgtype": "news",
			"news": map[string]any{
				"articles": []map[string]string{
					{
						"title":       msg.Title,
						"description": msg.Body,
					},
				},
			},
		}
	default: // markdown
		payload = map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": msg.Body,
			},
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wecom payload: %w", err)
	}

	timeout := destConfig.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create wecom request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wecom request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read wecom response: %w", err)
	}

	result := &SendResult{
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
	}

	// Parse WeCom response for error code
	var wcResp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &wcResp); err == nil {
		result.BusinessCode = fmt.Sprintf("%d", wcResp.ErrCode)
		// WeCom rate limit or system busy → retryable
		if wcResp.ErrCode == 45009 || wcResp.ErrCode == 45001 {
			result.IsRetryable = true
		} else if wcResp.ErrCode != 0 {
			result.IsRetryable = false
		}
	}

	return result, nil
}
