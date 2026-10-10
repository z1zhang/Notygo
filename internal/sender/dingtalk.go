package sender

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// DingTalkConfig holds the configuration for a DingTalk robot webhook sender.
type DingTalkConfig struct {
	WebhookURL string `json:"webhook_url"`
	MsgType    string `json:"msg_type"` // "markdown" or "actionCard"
}

// DingTalkSender sends messages via DingTalk robot webhooks.
type DingTalkSender struct{}

// NewDingTalkSender creates a new DingTalkSender.
func NewDingTalkSender() *DingTalkSender {
	return &DingTalkSender{}
}

// Send delivers a message to DingTalk via the configured webhook.
// If destConfig.SecretRef is provided (decrypted secret), signed mode is used.
func (s *DingTalkSender) Send(ctx context.Context, destConfig DestinationConfig, msg RenderedMessage) (*SendResult, error) {
	var cfg DingTalkConfig
	if err := json.Unmarshal(destConfig.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid dingtalk config: %w", err)
	}

	webhookURL := cfg.WebhookURL

	// If a secret is provided, use signed mode
	if destConfig.SecretRef != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sign, err := computeDingTalkSign(timestamp, destConfig.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("compute dingtalk sign: %w", err)
		}
		webhookURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, sign)
	}

	// Determine message type
	msgType := cfg.MsgType
	if msgType == "" {
		msgType = "markdown"
	}

	// Build the request body
	var payload any
	switch msgType {
	case "actionCard":
		payload = map[string]any{
			"msgtype": "actionCard",
			"actionCard": map[string]string{
				"title": msg.Title,
				"text":  msg.Body,
			},
		}
	default: // markdown
		payload = map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": msg.Title,
				"text":  msg.Body,
			},
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal dingtalk payload: %w", err)
	}

	timeout := destConfig.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create dingtalk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dingtalk request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read dingtalk response: %w", err)
	}

	result := &SendResult{
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
	}

	// Parse DingTalk response for errcode
	var dtResp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &dtResp); err == nil {
		result.BusinessCode = strconv.Itoa(dtResp.ErrCode)
		// errcode 45009 = frequency limit → retryable
		if dtResp.ErrCode == 45009 {
			result.IsRetryable = true
		} else if dtResp.ErrCode != 0 {
			result.IsRetryable = false
		}
	}

	return result, nil
}

// computeDingTalkSign computes the HMAC-SHA256 sign for DingTalk signed mode.
// sign = base64(hmac_sha256(secret, timestamp + "\n" + secret))
func computeDingTalkSign(timestamp, secret string) (string, error) {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	signData := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(signData), nil
}
