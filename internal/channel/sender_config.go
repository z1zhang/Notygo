package channel

import (
	"encoding/json"
	"time"

	"notygo/internal/secret"
	"notygo/internal/sender"
)

// BuildSenderConfig builds a sender.ChannelConfig from a channel model,
// decrypting the stored secret. Shared by the delivery worker and the
// channel test-send endpoint so both exercise the exact same code path.
func BuildSenderConfig(dest *Channel, secretSvc *secret.Service) sender.ChannelConfig {
	cfg := sender.ChannelConfig{
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

	// Decrypt secret if available
	if dest.EncryptedSecret != nil && *dest.EncryptedSecret != "" && secretSvc != nil {
		decrypted, err := secretSvc.Decrypt(*dest.EncryptedSecret)
		if err == nil {
			cfg.Secret = decrypted
		}
	}

	return cfg
}
