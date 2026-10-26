package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"notygo/internal/audit"
	"notygo/internal/config"
	"notygo/internal/channel"
	"notygo/internal/delivery"
	"notygo/internal/endpoint"
	"notygo/internal/event"
	"notygo/internal/ingest"
	"notygo/internal/mapping"
	"notygo/internal/observability"
	"notygo/internal/queue"
	"notygo/internal/rules"
	"notygo/internal/secret"
	"notygo/internal/sender"
	"notygo/internal/templates"
)

// NewRouter creates the main HTTP router with all routes and middleware.
func NewRouter(
	cfg *config.Config,
	db *pgxpool.Pool,
	secretSvc *secret.Service,
	endpointRepo *endpoint.Repository,
	mappingRepo *mapping.Repository,
	ruleRepo *rules.Repository,
	templateRepo *templates.Repository,
	destRepo *channel.Repository,
	eventRepo *event.Repository,
	deliveryRepo *delivery.Repository,
	ingester *ingest.Ingester,
	q *queue.Manager,
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
			"name":    AppName,
			"version": Version,
			"status":  "running",
		})
	})

	// Health endpoints (no auth). HEAD is served for busybox wget --spider
	// (Docker healthchecks), which probes with HEAD instead of GET.
	r.Get("/healthz", Healthz)
	r.Head("/healthz", Healthz)
	r.Get("/readyz", Readyz(db))
	r.Head("/readyz", Readyz(db))

	// Notification API (data plane) — untrusted webhook traffic, isolated
	// from the admin API namespace so proxies and logs can treat it
	// separately. Singular "/endpoint/" is deliberate: it never collides
	// with the SPA page route "/endpoints" or the admin API
	// "/api/v1/endpoints/". Endpoint-level auth is enforced inside the
	// handler.
	r.Post("/endpoint/{slug}", ingester.HandleIngest)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints (no auth required)
		r.Post("/auth/login", LoginHandler(db, auditRepo, loginGuard))

		// Protected endpoints (require admin auth)
		r.Group(func(r chi.Router) {
			r.Use(AdminAuthMiddleware(db))

			// Configuration CRUD
			r.Route("/endpoints", func(r chi.Router) {
				r.Get("/", ListEndpoints(db, secretSvc))
				r.Post("/", CreateEndpoint(db, secretSvc, auditRepo))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetEndpoint(db, secretSvc))
					r.Put("/", UpdateEndpoint(db, secretSvc, auditRepo))
					r.Delete("/", DeleteEndpoint(db, auditRepo))
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

			r.Route("/channels", func(r chi.Router) {
				r.Get("/", ListChannels(db, secretSvc))
				r.Post("/", CreateChannel(db, secretSvc, auditRepo))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetChannel(db, secretSvc))
					r.Put("/", UpdateChannel(db, secretSvc, auditRepo))
					r.Delete("/", DeleteChannel(db, auditRepo))
					r.Post("/test", TestChannel(db, secretSvc, senderRegistry, auditRepo))
				})
			})

			// Events and Deliveries
			r.Route("/events", func(r chi.Router) {
				r.Get("/", ListEvents(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetEvent(db))
					r.Post("/replay", ReplayEvent(db, q, auditRepo))
				})
			})

			r.Route("/deliveries", func(r chi.Router) {
				r.Get("/", ListDeliveries(db))
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", GetDelivery(db))
					r.Post("/retry", RetryDelivery(db, q, auditRepo))
				})
				r.Post("/bulk-retry", BulkRetry(db, q, auditRepo))
			})

			// Queue (River) health
			r.Get("/queue/stats", GetQueueStats(q))

			// System runtime info (version, concurrency, limits)
			r.Get("/system", SystemInfo(cfg))

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
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/endpoint/") {
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
