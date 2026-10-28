package sender

import (
	"encoding/json"
	"time"

	"notygo/internal/channel"
	"notygo/internal/secret"
)

// BuildChannelConfig assembles a ChannelConfig from a channel domain model,
// decrypting the stored secret. It lives in the sender (transport adapter)
// package so the domain model stays free of transport concerns — the
// dependency direction here is adapter → domain. Shared by the delivery
// worker and the channel test-send endpoint so both exercise the exact
// same code path.
func BuildChannelConfig(dest *channel.Channel, secretSvc *secret.Service) ChannelConfig {
	cfg := ChannelConfig{
		Type:      string(dest.Type),
		Timeout:   time.Duration(dest.TimeoutMs) * time.Millisecond,
		RawConfig: dest.Config,
	}

	// Marshal the config map to json.RawMessage for sender consumption
	if configBytes, err := json.Marshal(dest.Config); err == nil {
		cfg.Config = configBytes
	}

	// Extract URL and headers from config
	if url, ok := dest.Config["url"].(string); ok {
		cfg.URL = url
	}
	if method, ok := dest.Config["method"].(string); ok {
		cfg.Method = method
	}
	if headers, ok := dest.Config["headers"].(map[string]any); ok {
		cfg.Headers = make(map[string]string)
		for k, v := range headers {
			if s, ok := v.(string); ok {
				cfg.Headers[k] = s
			}
		}
	}

	// Decrypt secret if available. A decryption failure intentionally
	// degrades to "no secret": the send attempt will then fail with an
	// auth error, which is visible in the delivery attempt log.
	if dest.EncryptedSecret != nil && *dest.EncryptedSecret != "" && secretSvc != nil {
		if decrypted, err := secretSvc.Decrypt(*dest.EncryptedSecret); err == nil {
			cfg.Secret = decrypted
		}
	}

	return cfg
}
