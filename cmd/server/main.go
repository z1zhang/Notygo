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

	"zPush/internal/api"
	"zPush/internal/config"
	"zPush/internal/observability"
	"zPush/internal/secret"
	"zPush/internal/storage"
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
	db, err := storage.OpenSQLite(cfg.Database.Path)
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

	// Create router
	router := api.NewRouter(db)

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

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("zPush stopped")
}

// ensureAdminUser creates the default admin user on first boot if ADMIN_INITIAL_PASSWORD is set.
func ensureAdminUser(db *sql.DB, cfg *config.Config) error {
	if cfg.Admin.InitialPassword == "" {
		slog.Warn("ADMIN_INITIAL_PASSWORD not set; skipping admin user creation")
		return nil
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
		`INSERT INTO admin_users (id, username, password_hash) VALUES (?, 'admin', ?)`,
		generateID(), string(hash),
	)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}
	slog.Info("admin user created", "username", "admin")
	return nil
}

// generateID creates a random 16-byte hex string for use as entity IDs.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
