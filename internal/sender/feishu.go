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

// FeishuConfig holds the configuration for a Feishu/Lark bot webhook sender.
type FeishuConfig struct {
	WebhookURL string `json:"webhook_url"`
	MsgType    string `json:"msg_type"` // "interactive", "text", "post"
}

// FeishuSender sends messages via Feishu/Lark robot webhooks.
type FeishuSender struct{}

// NewFeishuSender creates a new FeishuSender.
func NewFeishuSender() *FeishuSender {
	return &FeishuSender{}
}

// Send delivers a message to Feishu via the configured webhook.
func (s *FeishuSender) Send(ctx context.Context, destConfig ChannelConfig, msg RenderedMessage) (*SendResult, error) {
	var cfg FeishuConfig
	if err := json.Unmarshal(destConfig.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid feishu config: %w", err)
	}

	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	msgType := cfg.MsgType
	if msgType == "" {
		msgType = "interactive"
	}

	// Build request body based on message type
	var payload any
	switch msgType {
	case "text":
		payload = map[string]any{
			"msg_type": "text",
			"content": map[string]string{
				"text": msg.Body,
			},
		}
	case "post":
		payload = map[string]any{
			"msg_type": "post",
			"content": map[string]any{
				"post": map[string]any{
					"zh_cn": map[string]any{
						"title": msg.Title,
						"content": [][]map[string]any{
							{
								{"tag": "text", "text": msg.Body},
							},
						},
					},
				},
			},
		}
	default: // interactive (card)
		payload = map[string]any{
			"msg_type": "interactive",
			"card": map[string]any{
				"header": map[string]any{
					"title": map[string]any{
						"tag":     "plain_text",
						"content": msg.Title,
					},
				},
				"elements": []map[string]any{
					{
						"tag":     "markdown",
						"content": msg.Body,
					},
				},
			},
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal feishu payload: %w", err)
	}

	timeout := destConfig.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.WebhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("feishu request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read feishu response: %w", err)
	}

	result := &SendResult{
		StatusCode:     resp.StatusCode,
		Body:           string(respBody),
		RequestPayload: string(bodyBytes),
	}

	// Parse Feishu response for error code
	var fsResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &fsResp); err == nil {
		result.BusinessCode = fmt.Sprintf("%d", fsResp.Code)
		// Feishu rate limit or server error → retryable
		if fsResp.Code == 99991400 || fsResp.Code == 99991401 {
			result.IsRetryable = true
		}
	}

	return result, nil
}
