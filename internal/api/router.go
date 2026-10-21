package api

import (
	"database/sql"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"zpush/internal/audit"
	"zpush/internal/delivery"
	"zpush/internal/destination"
	"zpush/internal/event"
	"zpush/internal/ingest"
	"zpush/internal/mapping"
	"zpush/internal/notify"
	"zpush/internal/observability"
	"zpush/internal/rules"
	"zpush/internal/sender"
	"zpush/internal/secret"
	"zpush/internal/source"
	"zpush/internal/templates"
)

// NewRouter creates the main HTTP router with all routes and middleware.
func NewRouter(
	db *sql.DB,
	secretSvc *secret.Service,
	sourceRepo *source.Repository,
	mappingRepo *mapping.Repository,
	ruleRepo *rules.Repository,
	templateRepo *templates.Repository,
	destRepo *destination.Repository,
	eventRepo *event.Repository,
	deliveryRepo *delivery.Repository,
	ingester *ingest.Ingester,
	notifier *notify.Notifier,
	auditRepo *audit.Repository,
	loginGuard *loginGuard,
	senderRegistry *sender.Registry,
) chi.Router {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RealIP)
	r.Use(observability.RequestIDMiddleware)
	r.Use(RecoveryMiddleware)
	r.Use(LoggingMiddleware)

	// Root endpoint — welcome / version info
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{
			"name":    "zPush",
			"version": "0.1.0",
			"status":  "running",
		})
	})

	// Health endpoints (no auth)
	r.Get("/healthz", Healthz)
	r.Get("/readyz", Readyz(db))

	// Notification API (data plane) — untrusted webhook traffic, isolated
	// from the admin API namespace so proxies and logs can treat it
	// separately. Source-level auth is enforced inside the handler.
	r.Post("/ingest/{source_slug}", ingester.HandleIngest)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints (no auth required)
		r.Post("/auth/login", LoginHandler(db, auditRepo, loginGuard))

		// Protected endpoints (require admin auth)
		r.Group(func(r chi.Router) {
			r.Use(AdminAuthMiddleware(db))

			// Configuration CRUD
			r.Route("/sources", func(r chi.Router) {
				r.Get("/", ListSources(db))
				r.Post("/", CreateSource(db, secretSvc, auditRepo))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetSource(db))
					r.Put("/", UpdateSource(db, secretSvc, auditRepo))
					r.Delete("/", DeleteSource(db, auditRepo))
				})
			})

			r.Route("/mappings", func(r chi.Router) {
				r.Get("/", ListMappings(db))
				r.Post("/", CreateMapping(db, auditRepo))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetMapping(db))
					r.Put("/", UpdateMapping(db, auditRepo))
					r.Delete("/", DeleteMapping(db, auditRepo))
				})
			})

			r.Route("/rules", func(r chi.Router) {
				r.Get("/", ListRules(db))
				r.Post("/", CreateRule(db, auditRepo))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetRule(db))
					r.Put("/", UpdateRule(db, auditRepo))
					r.Delete("/", DeleteRule(db, auditRepo))
				})
			})

			r.Route("/templates", func(r chi.Router) {
				r.Get("/", ListTemplates(db))
				r.Post("/", CreateTemplate(db, auditRepo))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetTemplate(db))
					r.Put("/", UpdateTemplate(db, auditRepo))
					r.Delete("/", DeleteTemplate(db, auditRepo))
				})
			})

			r.Route("/destinations", func(r chi.Router) {
				r.Get("/", ListDestinations(db, secretSvc))
				r.Post("/", CreateDestination(db, secretSvc, auditRepo))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetDestination(db, secretSvc))
					r.Put("/", UpdateDestination(db, secretSvc, auditRepo))
					r.Delete("/", DeleteDestination(db, auditRepo))
					r.Post("/test", TestDestination(db, secretSvc, senderRegistry, auditRepo))
				})
			})

			// Events and Deliveries
			r.Route("/events", func(r chi.Router) {
				r.Get("/", ListEvents(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetEvent(db))
					r.Post("/replay", ReplayEvent(db, notifier, auditRepo))
				})
			})

			r.Route("/deliveries", func(r chi.Router) {
				r.Get("/", ListDeliveries(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetDelivery(db))
					r.Post("/retry", RetryDelivery(db, notifier, auditRepo))
				})
				r.Post("/bulk-retry", BulkRetry(db, notifier, auditRepo))
			})

			// Audit logs
			r.Get("/audit", ListAuditLogs(db))

			// Dashboard
			r.Get("/dashboard/stats", DashboardStats(db))

			// Preview / debug
			r.Post("/preview/parse", PreviewParse())
			r.Post("/preview/mapping", PreviewMapping())
			r.Post("/preview/template", PreviewTemplate(db))
			r.Post("/preview/delivery", PreviewDelivery())
		})
	})

	// Serve frontend static files (if web/dist exists)
	serveFrontend(r)

	return r
}

// serveFrontend registers routes to serve the React SPA static files.
// In development, the frontend is served by Vite dev server on port 3000.
// In production, the built files are served by the Go server.
func serveFrontend(r chi.Router) {
	distDir := filepath.Join("web", "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return // no frontend build, skip
	}

	// Serve static assets
	fileServer := http.FileServer(http.Dir(distDir))
	r.Handle("/assets/*", http.StripPrefix("/", fileServer))

	indexPath := filepath.Join(distDir, "index.html")

	// SPA fallback: any non-API route serves a real file from the dist root
	// when one exists (favicon.svg etc.), otherwise index.html.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ingest/") {
			WriteError(w, http.StatusNotFound, "CONFIG_001", "endpoint not found")
			return
		}
		// path.Clean anchored at "/" collapses "..", so rel never escapes distDir
		rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if rel != "" && isRegularFile(filepath.Join(distDir, filepath.FromSlash(rel))) {
			http.ServeFile(w, r, filepath.Join(distDir, filepath.FromSlash(rel)))
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

// isRegularFile reports whether p exists and is a regular file (not a directory).
func isRegularFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

