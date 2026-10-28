package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/api"
	"notygo/internal/audit"
	"notygo/internal/backoff"
	"notygo/internal/channel"
	"notygo/internal/config"
	"notygo/internal/delivery"
	"notygo/internal/endpoint"
	"notygo/internal/event"
	"notygo/internal/ingest"
	"notygo/internal/mapping"
	"notygo/internal/observability"
	"notygo/internal/processor"
	"notygo/internal/queue"
	"notygo/internal/rule"
	"notygo/internal/secret"
	"notygo/internal/sender"
	"notygo/internal/storage"
	"notygo/internal/template"
)

func main() {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	observability.SetupLogger(cfg.Log.Level)
	slog.Info("starting Notygo", "listen_addr", cfg.Server.ListenAddr)

	// Open database (native pgx pool)
	db, err := storage.Open(rootCtx, cfg.Database.DSN)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations (goose only accepts *sql.DB, derived from the same DSN)
	migDB, err := storage.OpenMigrationsDB(cfg.Database.DSN)
	if err != nil {
		slog.Error("open migrations db", "error", err)
		os.Exit(1)
	}
	if err := storage.RunMigrations(migDB); err != nil {
		migDB.Close()
		slog.Error("run migrations", "error", err)
		os.Exit(1)
	}
	migDB.Close()
	slog.Info("migrations applied")

	// Initialize secret service
	secretSvc, err := secret.NewService(cfg.Secret.MasterKeyEnv)
	if err != nil {
		slog.Error("initialize secret service", "error", err)
		os.Exit(1)
	}
	slog.Info("secret service initialized", "active_key_version", secretSvc.ActiveVersion())

	// Initialize admin user if needed
	if err := ensureAdminUser(rootCtx, db, cfg); err != nil {
		slog.Error("ensure admin user", "error", err)
		os.Exit(1)
	}

	// Create repositories
	endpointRepo := endpoint.NewRepository(db)
	mappingRepo := mapping.NewRepository(db)
	ruleRepo := rule.NewRepository(db)
	templateRepo := template.NewRepository(db)
	destRepo := channel.NewRepository(db)
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

	// Create sender registry and register adapters
	senderRegistry := sender.NewRegistry()
	senderRegistry.Register("dingtalk", sender.NewDingTalkSender())
	senderRegistry.Register("feishu", sender.NewFeishuSender())
	senderRegistry.Register("wecom", sender.NewWeComSender())
	senderRegistry.Register("email", sender.NewEmailSender())
	senderRegistry.Register("http", sender.NewHTTPSender())

	// Create the River queue: services, workers, then the client. The
	// processor enqueues delivery jobs via the manager; the back-reference is
	// completed before the queue starts.
	proc := processor.NewService(db, endpointRepo, mappingRepo, ruleRepo, templateRepo,
		destRepo, eventRepo, deliveryRepo, nil)
	queueManager, err := queue.NewManager(db, cfg.Queue, queue.Deps{
		EventRepo:    eventRepo,
		DeliveryRepo: deliveryRepo,
		Processor:    proc,
		SenderService: delivery.NewSendService(db, destRepo, deliveryRepo,
			senderRegistry, secretSvc, backoff.Config{
				Base:          cfg.Queue.RetryBaseDelay,
				Max:           cfg.Queue.RetryMaxDelay,
				JitterPercent: cfg.Queue.RetryJitterPercent,
			}),
	})
	if err != nil {
		slog.Error("create queue manager", "error", err)
		os.Exit(1)
	}
	proc.SetEnqueuer(queueManager)

	if err := queueManager.Migrate(rootCtx); err != nil {
		slog.Error("migrate river schema", "error", err)
		os.Exit(1)
	}
	slog.Info("river queue migrations applied")

	// Create ingester (ingest inserts the event and its job in one transaction)
	ingester := ingest.NewIngester(db, endpointRepo, secretSvc, rateLimiter, cfg.Ingest.MaxPayloadBytes, queueManager)

	// Start the queue workers
	if err := queueManager.Start(rootCtx); err != nil {
		slog.Error("start river queue", "error", err)
		os.Exit(1)
	}
	slog.Info("river queue started",
		"event_concurrency", cfg.Queue.EventConcurrency,
		"delivery_concurrency", cfg.Queue.DeliveryConcurrency)

	// Create login guard (brute-force lockout, in-memory)
	loginGuard := api.NewLoginGuard(
		cfg.Admin.LoginMaxFailures,
		time.Duration(cfg.Admin.LoginLockoutSeconds)*time.Second,
	)
	slog.Info("login guard initialized",
		"max_failures", cfg.Admin.LoginMaxFailures,
		"lockout_seconds", cfg.Admin.LoginLockoutSeconds)

	// Create router
	router := api.NewRouter(cfg, db, secretSvc, endpointRepo, mappingRepo, ruleRepo, templateRepo, destRepo, eventRepo, deliveryRepo, ingester, queueManager, auditRepo, loginGuard, senderRegistry)

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
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutdown signal received", "signal", sig.String())

	// A second signal force-exits: one straggler delivery with a long sender
	// timeout should not hold the process hostage through the full drain
	// (30s queue + 10s HTTP) — the operator pressing Ctrl-C again has said so.
	go func() {
		<-quit
		slog.Warn("second shutdown signal, forcing exit")
		os.Exit(130)
	}()

	// Stop the queue first so in-flight jobs finish and their business
	// writes land before the HTTP server goes away.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()
	if err := queueManager.Stop(stopCtx); err != nil {
		slog.Error("queue stop error", "error", err)
	}
	slog.Info("queue stopped")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("Notygo stopped")
}

// ensureAdminUser creates the initial admin user on first boot if ADMIN_INITIAL_PASSWORD is set.
// The username is configurable via ADMIN_INITIAL_USER (default "admin"); both
// only take effect when no admin user exists yet.
func ensureAdminUser(ctx context.Context, db *pgxpool.Pool, cfg *config.Config) error {
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
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM notygo_user`).Scan(&count)
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

	_, err = db.Exec(ctx,
		`INSERT INTO notygo_user (id, username, password) VALUES ($1, $2, $3)`,
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
