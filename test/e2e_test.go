// Package tests holds end-to-end integration tests that exercise the full
// Notygo stack (HTTP API + River queue + real PostgreSQL) against a throwaway
// schema.
//
// These tests are gated on TEST_DATABASE_DSN — without it they skip, so
// `go test ./...` stays green on machines without a local PostgreSQL:
//
//	TEST_DATABASE_DSN=postgres://notygo:notygo@master:5432/notygo?sslmode=disable go test ./test/ -v
//
// Isolation: each run migrates into a uniquely-named schema via
// search_path (the migrations and all queries are schema-unqualified), and
// drops it on exit. The dev data in the target database is never touched.
package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"

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
	"notygo/internal/processor"
	"notygo/internal/queue"
	"notygo/internal/rule"
	"notygo/internal/secret"
	"notygo/internal/sender"
	"notygo/internal/storage"
	"notygo/internal/template"
)

const (
	adminUser     = "admin"
	adminPassword = "test-admin-password"
	// The HTTP channel targets a port that is guaranteed closed, so every
	// send fails fast (SSRF guard blocks loopback before a dial happens).
	badChannelURL = "http://127.0.0.1:9/hook"
)

type testStack struct {
	ts      *httptest.Server
	pool    *pgxpool.Pool
	schema  string
	baseURL string
	token   string
}

// TestE2E walks the full product flow end to end:
//
//	login → create config (template/channel/rule/mapping/endpoint) →
//	ingest event → mapping → rule match → delivery created → send fails →
//	retry_wait → automatic retry → attempts exhausted → failed →
//	manual retry → bulk retry → event replay → dashboard/audit/lists.
//
// The subtests share state on purpose: they are one narrative, and a
// failure anywhere fails the flow at the exact step that broke.
func TestE2E(t *testing.T) {
	if os.Getenv("TEST_DATABASE_DSN") == "" {
		t.Skip("TEST_DATABASE_DSN not set; skipping integration tests")
	}
	s := newTestStack(t)

	t.Run("Login", func(t *testing.T) {
		s.login(t)
		s.mustDo(t, "POST", "/api/v1/auth/login",
			`{"username":"admin","password":"wrong"}`, http.StatusUnauthorized)
	})

	var tplID, chID, ruleID, mapID, epSlug string
	t.Run("CreateConfig", func(t *testing.T) {
		var tpl struct{ ID string }
		s.mustJSON(t, "POST", "/api/v1/templates", &tpl,
			`{"name":"e2e-tpl","format":"text","title_template":"{{.title}}","body_template":"{{.body}}"}`, 201)
		tplID = tpl.ID

		var ch struct{ ID string }
		s.mustJSON(t, "POST", "/api/v1/channels", &ch,
			fmt.Sprintf(`{"name":"e2e-ch","type":"http","enabled":true,"config":{"url":"%s","method":"POST"},"timeout_ms":1000,"max_attempts":2}`, badChannelURL), 201)
		chID = ch.ID

		var rule struct{ ID string }
		s.mustJSON(t, "POST", "/api/v1/rules", &rule,
			fmt.Sprintf(`{"name":"e2e-rule","enabled":true,"priority":100,"conditions":{"conditions":[{"field":"title","operator":"eq","value":"e2e"}]},"template_id":"%s","channel_ids":["%s"]}`, tplID, chID), 201)
		ruleID = rule.ID

		var mp struct{ ID string }
		s.mustJSON(t, "POST", "/api/v1/mappings", &mp,
			fmt.Sprintf(`{"name":"e2e-map","rule_ids":["%s"],"continue_on_match":false,"fields":[{"payload_path":"title","target_key":"title","value_type":"string","required":true},{"payload_path":"body","target_key":"body","value_type":"string","required":true}]}`, ruleID), 201)
		mapID = mp.ID

		var ep struct{ Slug string }
		s.mustJSON(t, "POST", "/api/v1/endpoints", &ep,
			fmt.Sprintf(`{"name":"e2e-ep","slug":"e2e-ep","enabled":true,"mapping_id":"%s"}`, mapID), 201)
		epSlug = ep.Slug

		if tplID == "" || chID == "" || ruleID == "" || mapID == "" || epSlug == "" {
			t.Fatalf("missing ids: tpl=%q ch=%q rule=%q map=%q slug=%q", tplID, chID, ruleID, mapID, epSlug)
		}
	})

	var eventID string
	t.Run("IngestAndProcess", func(t *testing.T) {
		s.mustDo(t, "POST", "/endpoint/"+epSlug,
			`{"title":"e2e","body":"first event"}`, http.StatusAccepted)

		var ev struct {
			ID     string
			Status string
		}
		s.poll(t, "event completed", 15*time.Second, func() bool {
			var page struct {
				Data []struct {
					ID     string
					Status string
				}
			}
			s.mustJSON(t, "GET", "/api/v1/events?page=1&size=10", &page, "", 200)
			if len(page.Data) == 0 {
				return false
			}
			eventID = page.Data[0].ID
			ev.ID, ev.Status = page.Data[0].ID, page.Data[0].Status
			return ev.Status == "completed"
		})
	})

	t.Run("DeliveryRetryLifecycle", func(t *testing.T) {
		var deliveryID string
		s.poll(t, "delivery failed after retries", 15*time.Second, func() bool {
			var page struct {
				Data []struct {
					ID           string
					Status       string
					AttemptCount int `json:"attempt_count"`
				}
			}
			s.mustJSON(t, "GET", "/api/v1/deliveries?page=1&size=10", &page, "", 200)
			if len(page.Data) == 0 {
				return false
			}
			deliveryID = page.Data[0].ID
			return page.Data[0].Status == "failed" && page.Data[0].AttemptCount >= 2
		})

		// Delivery detail carries its attempts.
		var detail struct {
			Delivery struct {
				Status string
			}
			DeliveryAttempts []struct {
				Attempt int
			} `json:"delivery_attempts"`
		}
		s.mustJSON(t, "GET", "/api/v1/deliveries/"+deliveryID, &detail, "", 200)
		if len(detail.DeliveryAttempts) < 2 {
			t.Fatalf("expected >=2 delivery attempts, got %d", len(detail.DeliveryAttempts))
		}

		// Manual retry resets a failed delivery to pending.
		var retried struct {
			Delivery struct {
				ID     string
				Status string
			}
		}
		s.mustJSON(t, "POST", "/api/v1/deliveries/"+deliveryID+"/retry", &retried, `{}`, 200)
		if retried.Delivery.Status != "pending" {
			t.Fatalf("after retry, status = %q, want pending", retried.Delivery.Status)
		}

		// Bulk retry by status filter (status is translated to its numeric
		// code). The manually retried delivery cycles pending → send →
		// failed again within ~100ms, so poll until the filter finds it.
		var bulk struct {
			ResetCount int64 `json:"reset_count"`
		}
		s.poll(t, "bulk retry resets a failed delivery", 15*time.Second, func() bool {
			s.mustJSON(t, "POST", "/api/v1/deliveries/bulk-retry", &bulk, `{"status":"failed"}`, 200)
			return bulk.ResetCount >= 1
		})
	})

	t.Run("EventReplay", func(t *testing.T) {
		s.mustJSON(t, "POST", "/api/v1/events/"+eventID+"/replay", &struct{}{},
			`{}`, 200)

		// Replay re-processes the event; a second event run and delivery set
		// appear (idempotency keys differ per run).
		var total struct {
			Total int64
		}
		s.poll(t, "replayed deliveries", 15*time.Second, func() bool {
			s.mustJSON(t, "GET", "/api/v1/deliveries?page=1&size=10", &total, "", 200)
			return total.Total >= 2
		})
	})

	t.Run("RuleOperatorsUnifiedEngine", func(t *testing.T) {
		// Rules stored via the API with operators the processor-side
		// evaluator silently dropped (empty, ne) must actually match in the
		// production path now that it evaluates via rule.Evaluate.
		var rule2 struct{ ID string }
		s.mustJSON(t, "POST", "/api/v1/rules", &rule2,
			fmt.Sprintf(`{"name":"e2e-rule-ops","enabled":true,"priority":50,"conditions":{"operator":"and","conditions":[{"field":"note","operator":"empty"},{"field":"status","operator":"ne","value":"skip"}]},"template_id":"%s","channel_ids":["%s"]}`, tplID, chID), 201)

		var mp2 struct{ ID string }
		s.mustJSON(t, "POST", "/api/v1/mappings", &mp2,
			fmt.Sprintf(`{"name":"e2e-map-ops","rule_ids":["%s"],"continue_on_match":false,"fields":[{"payload_path":"title","target_key":"title","value_type":"string","required":true},{"payload_path":"body","target_key":"body","value_type":"string","required":true}]}`, rule2.ID), 201)

		var ep2 struct{ Slug string }
		s.mustJSON(t, "POST", "/api/v1/endpoints", &ep2,
			fmt.Sprintf(`{"name":"e2e-ep-ops","slug":"e2e-ep-ops","enabled":true,"mapping_id":"%s"}`, mp2.ID), 201)

		// note and status are absent from the payload (and unmapped):
		// empty → true, ne "skip" → true, so the rule must match.
		s.mustDo(t, "POST", "/endpoint/"+ep2.Slug,
			`{"title":"ops","body":"unified engine"}`, http.StatusAccepted)

		var opsEventID string
		s.poll(t, "ops event completed", 15*time.Second, func() bool {
			var page struct {
				Data []struct {
					ID     string
					Status string
				}
			}
			s.mustJSON(t, "GET", "/api/v1/events?page=1&size=10", &page, "", 200)
			for _, e := range page.Data {
				if e.ID != eventID && e.Status == "completed" {
					opsEventID = e.ID
					return true
				}
			}
			return false
		})

		var detail struct {
			LatestRun struct {
				MatchedRuleIDs []string `json:"matched_rule_ids"`
			} `json:"latest_run"`
		}
		s.mustJSON(t, "GET", "/api/v1/events/"+opsEventID, &detail, "", 200)
		if len(detail.LatestRun.MatchedRuleIDs) == 0 {
			t.Fatalf("expected empty/ne rule to match via unified engine, matched_rule_ids is empty")
		}
	})

	t.Run("ListsFiltersPagination", func(t *testing.T) {
		// Status filter on events (dynamic WHERE path).
		var evs struct {
			Total int64
		}
		s.mustJSON(t, "GET", "/api/v1/events?page=1&size=10&status=completed", &evs, "", 200)
		if evs.Total < 1 {
			t.Fatalf("filtered events total = %d, want >= 1", evs.Total)
		}
		// Unknown filter matches nothing, not everything.
		s.mustJSON(t, "GET", "/api/v1/events?page=1&size=10&status=garbage", &evs, "", 200)
		if evs.Total != 0 {
			t.Fatalf("garbage status filter total = %d, want 0", evs.Total)
		}
		// Delivery status filter: bulk-retried deliveries cycle back through
		// send → retry → failed, so poll until at least one is failed again.
		var dls struct {
			Total int64
		}
		s.poll(t, "failed deliveries via status filter", 15*time.Second, func() bool {
			s.mustJSON(t, "GET", "/api/v1/deliveries?page=1&size=10&status=failed", &dls, "", 200)
			return dls.Total >= 1
		})
		// Audit trail has entries from login and config changes.
		var audits struct {
			Total int64
		}
		s.mustJSON(t, "GET", "/api/v1/audit?page=1&size=50", &audits, "", 200)
		if audits.Total < 3 {
			t.Fatalf("audit total = %d, want >= 3", audits.Total)
		}
		// Dashboard aggregates.
		var stats struct {
			TotalEvents          int64 `json:"total_events"`
			PendingDeliveries    int64 `json:"pending_deliveries"`
			FailedDeliveries     int64 `json:"failed_deliveries"`
			RecentFailedDelivery []struct {
				ID string
			} `json:"recent_failed_deliveries"`
		}
		s.mustJSON(t, "GET", "/api/v1/dashboard/stats", &stats, "", 200)
		if stats.TotalEvents < 1 {
			t.Fatalf("dashboard total_events = %d, want >= 1", stats.TotalEvents)
		}
		// Queue stats (river_job counts) respond, with the retention window
		// so the dashboard can label the stats' scope.
		var qs struct {
			Queues []struct {
				Queue string
				State string
				Count int64
			} `json:"queues"`
			RetentionSeconds int64 `json:"retention_seconds"`
		}
		s.mustJSON(t, "GET", "/api/v1/queue/stats", &qs, "", 200)
		if qs.RetentionSeconds <= 0 {
			t.Fatalf("queue stats retention_seconds = %d, want > 0", qs.RetentionSeconds)
		}
		// Health endpoints.
		s.mustDo(t, "GET", "/healthz", "", http.StatusOK)
		s.mustDo(t, "GET", "/readyz", "", http.StatusOK)
	})
}

// newTestStack assembles the whole application (pool, migrations, River
// queue, router) exactly like cmd/server/main.go does, but pointed at a
// throwaway schema.
func newTestStack(t *testing.T) *testStack {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	schema := "notygo_test_" + strings.ToLower(ulid.Make().String())

	// Pool with search_path pinned to the test schema.
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	poolCfg.ConnConfig.RuntimeParams["search_path"] = schema
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	// Create the schema from a maintenance connection.
	maintCfg, _ := pgx.ParseConfig(dsn)
	maint, err := pgx.ConnectConfig(ctx, maintCfg)
	if err != nil {
		pool.Close()
		t.Fatalf("open maintenance connection: %v", err)
	}
	if _, err := maint.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		maint.Close(ctx)
		t.Fatalf("create schema: %v", err)
	}
	maint.Close(ctx)

	// Migrations via goose (needs *sql.DB, derived from the same DSN with
	// the same search_path so tables land in the test schema).
	migCfg, _ := pgx.ParseConfig(dsn)
	migCfg.RuntimeParams["search_path"] = schema
	migDB := stdlib.OpenDB(*migCfg)
	if err := storage.RunMigrations(migDB); err != nil {
		migDB.Close()
		pool.Close()
		t.Fatalf("run migrations: %v", err)
	}
	migDB.Close()

	s := &testStack{pool: pool, schema: schema}
	// teardown (drop schema) must run LAST, so it is registered first —
	// t.Cleanup is LIFO and the queue/http cleanup below registers second.
	t.Cleanup(func() { s.teardown(t) })

	// Secret service with a random key — the schema is fresh, so no
	// previously encrypted data needs to decrypt.
	key := base64.StdEncoding.EncodeToString(mustRandomKey(t))
	secretSvc, err := secret.NewService("v1:" + key)
	if err != nil {
		t.Fatalf("secret service: %v", err)
	}

	// Seed the admin user directly (cmd/server does this via env var).
	hash, _ := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.MinCost)
	if _, err := pool.Exec(ctx,
		`INSERT INTO notygo_user (id, username, password) VALUES ($1, $2, $3)`,
		ulid.Make().String(), adminUser, string(hash),
	); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	// Repos and services.
	endpointRepo := endpoint.NewRepository(pool)
	mappingRepo := mapping.NewRepository(pool)
	ruleRepo := rule.NewRepository(pool)
	templateRepo := template.NewRepository(pool)
	destRepo := channel.NewRepository(pool)
	eventRepo := event.NewRepository(pool)
	deliveryRepo := delivery.NewRepository(pool)
	auditRepo := audit.NewRepository(pool)

	senderRegistry := sender.NewRegistry()
	senderRegistry.Register("dingtalk", sender.NewDingTalkSender())
	senderRegistry.Register("feishu", sender.NewFeishuSender())
	senderRegistry.Register("wecom", sender.NewWeComSender())
	senderRegistry.Register("email", sender.NewEmailSender())
	senderRegistry.Register("http", sender.NewHTTPSender())

	// River queue with a fast retry curve so the test finishes in seconds.
	// The processor's job enqueuer is bound after the manager exists.
	proc := processor.NewService(pool, endpointRepo, mappingRepo, ruleRepo,
		templateRepo, destRepo, eventRepo, deliveryRepo, nil)
	queueCfg := config.QueueConfig{
		EventConcurrency:    2,
		DeliveryConcurrency: 2,
		RetryBaseDelay:      200 * time.Millisecond,
		RetryMaxDelay:       time.Second,
		RetryJitterPercent:  20,
		JobRetention:        time.Minute,
	}
	queueManager, err := queue.NewManager(pool, queueCfg, queue.Deps{
		EventRepo:     eventRepo,
		DeliveryRepo:  deliveryRepo,
		Processor:     proc,
		SenderService: delivery.NewSendService(pool, destRepo, deliveryRepo, senderRegistry, secretSvc, backoff.Config{Base: queueCfg.RetryBaseDelay, Max: queueCfg.RetryMaxDelay, JitterPercent: queueCfg.RetryJitterPercent}),
	})
	if err != nil {
		t.Fatalf("queue manager: %v", err)
	}
	proc.SetEnqueuer(queueManager)
	if err := queueManager.Migrate(ctx); err != nil {
		t.Fatalf("river migrations: %v", err)
	}
	if err := queueManager.Start(ctx); err != nil {
		t.Fatalf("start river queue: %v", err)
	}

	ingester := ingest.NewIngester(pool, endpointRepo, secretSvc,
		ingest.NewRateLimiter(100, 200), 1<<20, queueManager)

	router := api.NewRouter(&config.Config{Queue: queueCfg}, pool, secretSvc, endpointRepo, mappingRepo, ruleRepo,
		templateRepo, destRepo, eventRepo, deliveryRepo, ingester, queueManager, auditRepo,
		api.NewLoginGuard(5, 15*time.Minute), senderRegistry)
	ts := httptest.NewServer(router)
	s.ts = ts
	s.baseURL = ts.URL

	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := queueManager.Stop(stopCtx); err != nil {
			t.Logf("stop river queue: %v", err)
		}
		ts.Close()
	})
	return s
}

// teardown drops the throwaway schema. Registered before the queue/http
// cleanup, so it runs last: the River client and HTTP server are already
// stopped (no in-flight queries).
func (s *testStack) teardown(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	maint, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_DSN"))
	if err == nil {
		_, err = maint.Exec(ctx, "DROP SCHEMA IF EXISTS "+s.schema+" CASCADE")
		if err != nil {
			t.Logf("drop schema %s: %v (leftover in test DB)", s.schema, err)
		}
		maint.Close(ctx)
	} else {
		t.Logf("teardown connection failed: %v (schema %s left in test DB)", err, s.schema)
	}
	s.pool.Close()
}

// login exchanges the seeded credentials for a bearer token.
func (s *testStack) login(t *testing.T) {
	t.Helper()
	var resp struct {
		Token string
	}
	s.mustJSON(t, "POST", "/api/v1/auth/login", &resp,
		fmt.Sprintf(`{"username":%q,"password":%q}`, adminUser, adminPassword), 200)
	if resp.Token == "" {
		t.Fatal("login returned no token")
	}
	s.token = resp.Token
}

// mustDo issues a request and asserts the status code. Empty body means no
// body. Returns the response body.
func (s *testStack) mustDo(t *testing.T, method, path, body string, wantStatus int) []byte {
	t.Helper()
	var rd *bytes.Reader
	if body == "" {
		rd = bytes.NewReader(nil)
	} else {
		rd = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, s.baseURL+path, rd)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d\nbody: %s", method, path, resp.StatusCode, wantStatus, buf.String())
	}
	return buf.Bytes()
}

// mustJSON issues a request and decodes the JSON response into out.
func (s *testStack) mustJSON(t *testing.T, method, path string, out any, body string, wantStatus int) {
	t.Helper()
	raw := s.mustDo(t, method, path, body, wantStatus)
	if out == nil {
		return
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s %s: decode response: %v\nbody: %s", method, path, err, raw)
	}
}

// poll retries cond every 200ms until it returns true or the timeout hits.
// The River queue in this stack runs with a fast retry curve (200ms base),
// so polls normally resolve within ~1s.
func (s *testStack) poll(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s after %s", what, timeout)
}

func mustRandomKey(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random key: %v", err)
	}
	return b
}
