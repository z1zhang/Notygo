package api

import (
	"net/http"

	"notygo/internal/config"
)

// AppName is the human-readable service name.
const AppName = "Notygo"

// Version is injected at build time via:
//
//	go build -ldflags "-X notygo/internal/api.Version=1.2.3"
//
// "dev" means it was built without injection (local go run / IDE run).
var Version = "dev"

// SystemInfo exposes non-sensitive runtime configuration (queue concurrency,
// rate limits, retention) for the settings UI. Secrets and DSN parts are
// deliberately excluded.
func SystemInfo(cfg *config.Config) http.HandlerFunc {
	rate, burst, _ := config.ParseGlobalRateLimit(cfg.Ingest.GlobalRateLimit)
	return func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]any{
			"name":    AppName,
			"version": Version,
			"queue": map[string]any{
				"event_concurrency":        cfg.Queue.EventConcurrency,
				"delivery_concurrency":     cfg.Queue.DeliveryConcurrency,
				"job_retention_seconds":    int64(cfg.Queue.JobRetention.Seconds()),
				"retry_base_delay_seconds": int64(cfg.Queue.RetryBaseDelay.Seconds()),
				"retry_max_delay_seconds":  int64(cfg.Queue.RetryMaxDelay.Seconds()),
				"retry_jitter_percent":     cfg.Queue.RetryJitterPercent,
			},
			"ingest": map[string]any{
				"max_payload_bytes": cfg.Ingest.MaxPayloadBytes,
				"rate_limit":        cfg.Ingest.GlobalRateLimit,
				"rate_per_second":   rate,
				"burst":             burst,
			},
			"login": map[string]any{
				"max_failures":    cfg.Admin.LoginMaxFailures,
				"lockout_seconds": cfg.Admin.LoginLockoutSeconds,
			},
			"log_level": cfg.Log.Level,
		})
	}
}
