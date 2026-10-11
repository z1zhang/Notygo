package api

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"zPush/internal/delivery"
	"zPush/internal/destination"
	"zPush/internal/event"
	"zPush/internal/ingest"
	"zPush/internal/mapping"
	"zPush/internal/observability"
	"zPush/internal/rules"
	"zPush/internal/secret"
	"zPush/internal/source"
	"zPush/internal/templates"
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

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints (no auth required)
		r.Post("/auth/login", LoginHandler(db))

		// Protected endpoints (require admin auth)
		r.Group(func(r chi.Router) {
			r.Use(AdminAuthMiddleware(db))

			// Configuration CRUD
			r.Route("/sources", func(r chi.Router) {
				r.Get("/", ListSources(db))
				r.Post("/", CreateSource(db, secretSvc))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetSource(db))
					r.Put("/", UpdateSource(db, secretSvc))
					r.Delete("/", DeleteSource(db))
				})
			})

			r.Route("/mappings", func(r chi.Router) {
				r.Get("/", ListMappings(db))
				r.Post("/", CreateMapping(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetMapping(db))
					r.Put("/", UpdateMapping(db))
					r.Delete("/", DeleteMapping(db))
				})
			})

			r.Route("/rules", func(r chi.Router) {
				r.Get("/", ListRules(db))
				r.Post("/", CreateRule(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetRule(db))
					r.Put("/", UpdateRule(db))
					r.Delete("/", DeleteRule(db))
				})
			})

			r.Route("/templates", func(r chi.Router) {
				r.Get("/", ListTemplates(db))
				r.Post("/", CreateTemplate(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetTemplate(db))
					r.Put("/", UpdateTemplate(db))
					r.Delete("/", DeleteTemplate(db))
				})
			})

			r.Route("/destinations", func(r chi.Router) {
				r.Get("/", ListDestinations(db))
				r.Post("/", CreateDestination(db, secretSvc))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetDestination(db))
					r.Put("/", UpdateDestination(db, secretSvc))
					r.Delete("/", DeleteDestination(db))
					r.Post("/test", placeholderHandler("destination.test"))
				})
			})

			// Events and Deliveries
			r.Route("/events", func(r chi.Router) {
				r.Get("/", ListEvents(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetEvent(db))
					r.Post("/replay", ReplayEvent(db))
				})
			})

			r.Route("/deliveries", func(r chi.Router) {
				r.Get("/", ListDeliveries(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetDelivery(db))
					r.Post("/retry", RetryDelivery(db))
				})
				r.Post("/bulk-retry", BulkRetry(db))
			})

			// Audit logs
			r.Get("/audit", ListAuditLogs(db))

			// Preview / debug
			r.Post("/preview/parse", placeholderHandler("preview.parse"))
			r.Post("/preview/mapping", placeholderHandler("preview.mapping"))
			r.Post("/preview/rule", placeholderHandler("preview.rule"))
			r.Post("/preview/template", PreviewTemplate(db))
			r.Post("/preview/delivery", placeholderHandler("preview.delivery"))
		})

		// Webhook ingest (source-level auth, not admin auth)
		r.Post("/ingest/{source_slug}", ingester.HandleIngest)
	})

	// Serve frontend static files (if web/dist exists)
	serveFrontend(r)

	return r
}

// serveFrontend registers routes to serve the Vue SPA static files.
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

	// SPA fallback: any non-API, non-health route serves index.html
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// Only serve index.html for non-API routes
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			WriteError(w, http.StatusNotFound, "CONFIG_001", "endpoint not found")
			return
		}
		indexPath := filepath.Join(distDir, "index.html")
		http.ServeFile(w, r, indexPath)
	})
}

// placeholderHandler returns a temporary handler that returns 501 Not Implemented.
func placeholderHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusNotImplemented, "SYSTEM_000", name+": not yet implemented")
	}
}
