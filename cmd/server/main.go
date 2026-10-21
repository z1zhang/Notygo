package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"database/sql"

	"golang.org/x/crypto/bcrypt"

	"zpush/internal/api"
	"zpush/internal/audit"
	"zpush/internal/config"
	"zpush/internal/delivery"
	"zpush/internal/destination"
	"zpush/internal/event"
	"zpush/internal/ingest"
	"zpush/internal/mapping"
	"zpush/internal/notify"
	"zpush/internal/observability"
	"zpush/internal/processor"
	"zpush/internal/rules"
	"zpush/internal/secret"
	"zpush/internal/sender"
	"zpush/internal/source"
	"zpush/internal/storage"
	"zpush/internal/templates"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	observability.SetupLogger(cfg.Log.Level)
	slog.Info("starting zPush", "listen_addr", cfg.Server.ListenAddr)

	// Open database
	db, err := storage.Open(cfg.Database.DSN)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations
	if err := storage.RunMigrations(db); err != nil {
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	// Initialize secret service
	secretSvc, err := secret.NewService(cfg.Secret.MasterKeyEnv)
	if err != nil {
		slog.Error("initialize secret service", "error", err)
		os.Exit(1)
	}
	slog.Info("secret service initialized", "active_key_version", secretSvc.ActiveVersion())

	// Initialize admin user if needed
	if err := ensureAdminUser(db, cfg); err != nil {
		slog.Error("ensure admin user", "error", err)
		os.Exit(1)
	}

	// Create repositories
	sourceRepo := source.NewRepository(db)
	mappingRepo := mapping.NewRepository(db)
	ruleRepo := rules.NewRepository(db)
	templateRepo := templates.NewRepository(db)
	destRepo := destination.NewRepository(db)
	eventRepo := event.NewRepository(db)
	deliveryRepo := delivery.NewRepository(db)
	auditRepo := audit.NewRepository(db)

	// Create rate limiter
	globalRate, globalBurst, err := config.ParseGlobalRateLimit(cfg.Ingest.GlobalRateLimit)
	if err != nil {
		slog.Warn("invalid global rate limit, using defaults", "error", err)
		globalRate = 100
		globalBurst = 200
	}
	rateLimiter := ingest.NewRateLimiter(globalRate, globalBurst)

	// Create notifier for instant wake-up between ingest → processor → worker
	notifier := notify.NewNotifier()

	// Create ingester
	ingester := ingest.NewIngester(db, sourceRepo, secretSvc, rateLimiter, cfg.Ingest.MaxPayloadBytes, notifier)

	// Create sender registry and register adapters
	senderRegistry := sender.NewRegistry()
	senderRegistry.Register("dingtalk", sender.NewDingTalkSender())
	senderRegistry.Register("feishu", sender.NewFeishuSender())
	senderRegistry.Register("wecom", sender.NewWeComSender())
	senderRegistry.Register("email", sender.NewEmailSender())
	senderRegistry.Register("http_webhook", sender.NewHTTPWebhookSender())

	// Create delivery worker
	deliveryWorker := delivery.NewDeliveryWorker(cfg.Worker, destRepo, deliveryRepo, senderRegistry, secretSvc, notifier)

	// Create event processor
	eventProcessor := processor.NewEventProcessor(cfg.Worker, sourceRepo, mappingRepo, ruleRepo, templateRepo, destRepo, eventRepo, deliveryRepo, notifier)

	// Create worker context that can be canceled
	workerCtx, workerCancel := context.WithCancel(context.Background())

	// Start workers
	go eventProcessor.Run(workerCtx)
	slog.Info("event processor started", "concurrency", cfg.Worker.EventProcessorConcurrency)

	go deliveryWorker.Run(workerCtx)
	slog.Info("delivery worker started", "concurrency", cfg.Worker.DeliveryWorkerConcurrency)

	// Start lease recovery loops
	go processor.LeaseRecoveryLoop(workerCtx, db, cfg.Worker.LeaseRecoveryInterval, notifier)
	go delivery.LeaseRecoveryLoop(workerCtx, db, cfg.Worker.LeaseRecoveryInterval, notifier)

	// Create login guard (brute-force lockout, in-memory)
	loginGuard := api.NewLoginGuard(
		cfg.Admin.LoginMaxFailures,
		time.Duration(cfg.Admin.LoginLockoutSeconds)*time.Second,
	)
	slog.Info("login guard initialized",
		"max_failures", cfg.Admin.LoginMaxFailures,
		"lockout_seconds", cfg.Admin.LoginLockoutSeconds)

	// Create router
	router := api.NewRouter(db, secretSvc, sourceRepo, mappingRepo, ruleRepo, templateRepo, destRepo, eventRepo, deliveryRepo, ingester, notifier, auditRepo, loginGuard, senderRegistry)

	// Start HTTP server
	srv := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("http server listening", "addr", cfg.Server.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	// Stop workers first (cancel their context)
	workerCancel()
	slog.Info("workers stopped, waiting for in-flight tasks...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("zPush stopped")
}

// ensureAdminUser creates the initial admin user on first boot if ADMIN_INITIAL_PASSWORD is set.
// The username is configurable via ADMIN_INITIAL_USER (default "admin"); both
// only take effect when no admin user exists yet.
func ensureAdminUser(db *sql.DB, cfg *config.Config) error {
	if cfg.Admin.InitialPassword == "" {
		slog.Warn("ADMIN_INITIAL_PASSWORD not set; skipping admin user creation")
		return nil
	}
	username := cfg.Admin.InitialUsername
	if username == "" {
		username = "admin"
	}

	// Check if any admin user exists
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count)
	if err != nil {
		return fmt.Errorf("check admin users: %w", err)
	}
	if count > 0 {
		slog.Info("admin user already exists; skipping creation")
		return nil
	}

	// Hash the password with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Admin.InitialPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	_, err = db.Exec(
		`INSERT INTO admin_users (id, username, password_hash) VALUES (?, ?, ?)`,
		generateID(), username, string(hash),
	)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}
	slog.Info("admin user created", "username", username)
	return nil
}

// generateID creates a random 16-byte hex string for use as entity IDs.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
