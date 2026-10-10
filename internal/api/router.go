package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"zPush/internal/observability"
)

// NewRouter creates the main HTTP router with all routes and middleware.
func NewRouter(db *sql.DB) chi.Router {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RealIP)
	r.Use(observability.RequestIDMiddleware)
	r.Use(RecoveryMiddleware)
	r.Use(LoggingMiddleware)

	// Health endpoints (no auth)
	r.Get("/healthz", Healthz)
	r.Get("/readyz", Readyz(db))

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints (no auth required)
		r.Post("/auth/login", placeholderHandler("auth.login"))

		// Protected endpoints (require admin auth)
		r.Group(func(r chi.Router) {
			r.Use(AdminAuthMiddleware(db))

			// Configuration CRUD
			r.Route("/sources", func(r chi.Router) {
				r.Get("/", placeholderHandler("source.list"))
				r.Post("/", placeholderHandler("source.create"))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", placeholderHandler("source.get"))
					r.Put("/", placeholderHandler("source.update"))
					r.Delete("/", placeholderHandler("source.delete"))
				})
			})

			r.Route("/mappings", func(r chi.Router) {
				r.Get("/", placeholderHandler("mapping.list"))
				r.Post("/", placeholderHandler("mapping.create"))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", placeholderHandler("mapping.get"))
					r.Put("/", placeholderHandler("mapping.update"))
					r.Delete("/", placeholderHandler("mapping.delete"))
				})
			})

			r.Route("/rules", func(r chi.Router) {
				r.Get("/", placeholderHandler("rule.list"))
				r.Post("/", placeholderHandler("rule.create"))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", placeholderHandler("rule.get"))
					r.Put("/", placeholderHandler("rule.update"))
					r.Delete("/", placeholderHandler("rule.delete"))
				})
			})

			r.Route("/templates", func(r chi.Router) {
				r.Get("/", placeholderHandler("template.list"))
				r.Post("/", placeholderHandler("template.create"))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", placeholderHandler("template.get"))
					r.Put("/", placeholderHandler("template.update"))
					r.Delete("/", placeholderHandler("template.delete"))
				})
			})

			r.Route("/destinations", func(r chi.Router) {
				r.Get("/", placeholderHandler("destination.list"))
				r.Post("/", placeholderHandler("destination.create"))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", placeholderHandler("destination.get"))
					r.Put("/", placeholderHandler("destination.update"))
					r.Delete("/", placeholderHandler("destination.delete"))
					r.Post("/test", placeholderHandler("destination.test"))
				})
			})

			// Events and Deliveries
			r.Route("/events", func(r chi.Router) {
				r.Get("/", placeholderHandler("event.list"))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", placeholderHandler("event.get"))
					r.Post("/replay", placeholderHandler("event.replay"))
				})
			})

			r.Route("/deliveries", func(r chi.Router) {
				r.Get("/", placeholderHandler("delivery.list"))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", placeholderHandler("delivery.get"))
					r.Post("/retry", placeholderHandler("delivery.retry"))
				})
				r.Post("/bulk-retry", placeholderHandler("delivery.bulk-retry"))
			})

			// Preview / debug
			r.Post("/preview/parse", placeholderHandler("preview.parse"))
			r.Post("/preview/mapping", placeholderHandler("preview.mapping"))
			r.Post("/preview/rule", placeholderHandler("preview.rule"))
			r.Post("/preview/template", placeholderHandler("preview.template"))
			r.Post("/preview/delivery", placeholderHandler("preview.delivery"))
		})

		// Webhook ingest (source-level auth, not admin auth)
		r.Post("/ingest/{source_slug}", placeholderHandler("ingest"))
	})

	return r
}

// placeholderHandler returns a temporary handler that returns 501 Not Implemented.
// This will be replaced with real implementations in Phase 1.
func placeholderHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusNotImplemented, "SYSTEM_000", name+": not yet implemented")
	}
}
