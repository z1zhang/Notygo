package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Secret   SecretConfig
	Worker   WorkerConfig
	Ingest   IngestConfig
	Admin    AdminConfig
	Log      LogConfig
}

type ServerConfig struct {
	ListenAddr string
}

type DatabaseConfig struct {
	DSN      string // resolved MySQL DSN, assembled from the parts below
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

type SecretConfig struct {
	MasterKeyEnv string // raw ENCRYPTION_MASTER_KEY value
}

type WorkerConfig struct {
	EventProcessorConcurrency int
	DeliveryWorkerConcurrency int
	LeaseTimeout              time.Duration
	RetryBaseDelay            time.Duration
	RetryMaxDelay             time.Duration
	RetryJitterPercent        int
	ClaimPollInterval         time.Duration
	LeaseRecoveryInterval     time.Duration
}

type IngestConfig struct {
	MaxPayloadBytes int
	GlobalRateLimit string
}

type AdminConfig struct {
	InitialUsername     string
	InitialPassword     string
	LoginMaxFailures    int
	LoginLockoutSeconds int
}

type LogConfig struct {
	Level string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			ListenAddr: getEnv("LISTEN_ADDR", ":9003"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnvInt("DB_PORT", 3306),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     getEnv("DB_NAME", "zpush"),
		},
		Secret: SecretConfig{
			MasterKeyEnv: os.Getenv("ENCRYPTION_MASTER_KEY"),
		},
		Worker: WorkerConfig{
			EventProcessorConcurrency: getEnvInt("EVENT_PROCESSOR_CONCURRENCY", 4),
			DeliveryWorkerConcurrency: getEnvInt("DELIVERY_WORKER_CONCURRENCY", 8),
			LeaseTimeout:              getEnvDuration("LEASE_TIMEOUT", 30*time.Second),
			RetryBaseDelay:            30 * time.Second,
			RetryMaxDelay:             1 * time.Hour,
			RetryJitterPercent:        20,
			ClaimPollInterval:         getEnvDuration("CLAIM_POLL_INTERVAL", 2*time.Second),
			LeaseRecoveryInterval:     30 * time.Second,
		},
		Ingest: IngestConfig{
			MaxPayloadBytes: getEnvInt("MAX_PAYLOAD_BYTES", 1048576), // 1MB
			GlobalRateLimit: getEnv("INGEST_GLOBAL_RATE_LIMIT", "100/200"),
		},
		Admin: AdminConfig{
			InitialUsername:     getEnv("ADMIN_INITIAL_USER", "admin"),
			InitialPassword:     os.Getenv("ADMIN_INITIAL_PASSWORD"),
			LoginMaxFailures:    getEnvInt("LOGIN_MAX_FAILURES", 5),
			LoginLockoutSeconds: getEnvInt("LOGIN_LOCKOUT_SECONDS", 900),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
	}

	// The MySQL connection is assembled from the DB_* parts.
	if cfg.Database.User == "" || cfg.Database.Password == "" {
		return nil, fmt.Errorf("DB_USER and DB_PASSWORD are required but not set")
	}
	cfg.Database.DSN = buildMySQLDSN(cfg.Database)

	if cfg.Secret.MasterKeyEnv == "" {
		return nil, fmt.Errorf("ENCRYPTION_MASTER_KEY is required but not set")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		// Also try seconds as integer
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}

// buildMySQLDSN assembles a go-sql-driver DSN from discrete parts.
// mysql.Config.FormatDSN escapes special characters (e.g. passwords
// containing ':' or '@') that would break a hand-written DSN string.
func buildMySQLDSN(cfg DatabaseConfig) string {
	mc := mysql.Config{
		Net:       "tcp",
		Addr:      net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		User:      cfg.User,
		Passwd:    cfg.Password,
		DBName:    cfg.Name,
		ParseTime: true,
	}
	return mc.FormatDSN()
}

// ParseGlobalRateLimit parses "rate/burst" format (e.g. "100/200").
func ParseGlobalRateLimit(s string) (rate float64, burst int, err error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid rate limit format %q, expected rate/burst", s)
	}
	r, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid rate %q: %w", parts[0], err)
	}
	b, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid burst %q: %w", parts[1], err)
	}
	return r, b, nil
}
