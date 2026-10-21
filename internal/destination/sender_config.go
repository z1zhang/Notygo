package destination

import (
	"encoding/json"
	"time"

	"zpush/internal/secret"
	"zpush/internal/sender"
)

// BuildSenderConfig builds a sender.DestinationConfig from a destination model,
// decrypting the stored secret. Shared by the delivery worker and the
// destination test-send endpoint so both exercise the exact same code path.
func BuildSenderConfig(dest *Destination, secretSvc *secret.Service) sender.DestinationConfig {
	cfg := sender.DestinationConfig{
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
	if dest.SecretRef != nil && *dest.SecretRef != "" && secretSvc != nil {
		decrypted, err := secretSvc.Decrypt(*dest.SecretRef)
		if err == nil {
			cfg.Secret = decrypted
			cfg.SecretRef = decrypted
		}
	}

	return cfg
}
