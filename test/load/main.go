// Command load is a load-test tool for the Notygo ingest data plane.
//
// Usage:
//
//	go run ./test/load -duration 60s -concurrency 10 [-rps 500] [-drain 120s]
//
// It logs in with admin credentials and prepares the test targets: by default
// pipeline mode (-pipeline, the default) creates N templates + N rules (rule i
// matches msg == "notygo-load-i", channel_ids empty so no delivery jobs) and M
// mappings/endpoints, one mapping bound per endpoint; payloads rotate so every
// rule matches 1/N of events and the full rule set is evaluated each time.
// -pipeline=false falls back to a bare endpoint (ingest reception only).
// N workers spread across the M endpoints and hammer POST /endpoint/{slug},
// reporting RPS, latency percentiles and the status-code distribution.
// Afterwards it polls /api/v1/queue/stats until the event_process backlog
// drains, which measures the real consumer throughput end to end.
//
// If you see 429s, the server's global ingest limiter (INGEST_GLOBAL_RATE_LIMIT,
// default 100/200) is throttling you — raise it on the server or cap with -rps.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	base        = flag.String("base", "http://127.0.0.1:9003", "server base URL")
	username    = flag.String("username", "admin", "admin username")
	password    = flag.String("password", "admin", "admin password")
	slug        = flag.String("slug", "loadtest", "first ingest endpoint slug (created if missing)")
	duration    = flag.Duration("duration", 60*time.Second, "load duration")
	concurrency = flag.Int("concurrency", 10, "parallel workers")
	rps         = flag.Int("rps", 0, "cap total requests per second (0 = unlimited)")
	drainWait   = flag.Duration("drain", 120*time.Second, "max wait for queue drain after load (0 = skip)")
	reportEvery = flag.Duration("report", 10*time.Second, "progress report interval")

	// Pipeline mode: full event processing (mapping → rule eval → render).
	// Channels are deliberately excluded — no delivery jobs, no outbound
	// requests — so the numbers reflect the processing pipeline itself.
	pipeline      = flag.Bool("pipeline", true, "create mappings+rules+templates and bind endpoints (full event processing)")
	ruleCount     = flag.Int("rules", 4, "pipeline: number of rules, one template each; payloads rotate so every rule matches")
	endpointCount = flag.Int("endpoints", 3, "pipeline: number of endpoints (one mapping each); workers spread across them")
)

// msgValues are the rotating payload "msg" values; rule i matches
// msg == msgValues[i]. With continue_on_match=false every event evaluates the
// whole rule set in priority order and renders exactly one template.
var msgValues []string

type reqResult struct {
	lat  time.Duration
	code int // -1 = transport error
}

type stats struct {
	mu        sync.Mutex
	latencies []time.Duration
	codes     map[int]int
}

func (s *stats) add(r reqResult) {
	s.mu.Lock()
	s.latencies = append(s.latencies, r.lat)
	s.codes[r.code]++
	s.mu.Unlock()
}

func (s *stats) snapshot() (int, map[int]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int]int, len(s.codes))
	for k, v := range s.codes {
		out[k] = v
	}
	return len(s.latencies), out
}

type apiClient struct {
	base  string
	token string
	http  *http.Client
}

func (c *apiClient) do(method, path string, body []byte) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, b, err
}

func main() {
	flag.Parse()

	transport := &http.Transport{
		MaxIdleConns:        *concurrency * 2,
		MaxIdleConnsPerHost: *concurrency,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &apiClient{
		base: *base,
		http: &http.Client{Timeout: 15 * time.Second, Transport: transport},
	}

	token, err := login(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "login failed:", err)
		os.Exit(1)
	}
	client.token = token
	fmt.Printf("logged in as %s\n", *username)

	slugs, err := setupLoadTest(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup failed:", err)
		os.Exit(1)
	}
	fmt.Printf("target: POST %s/endpoint/{%s} (concurrency=%d duration=%s", *base, strings.Join(slugs, ","), *concurrency, *duration)
	if *rps > 0 {
		fmt.Printf(" rps≤%d", *rps)
	}
	if *pipeline {
		fmt.Printf(" pipeline: rules=%d mappings=%d", *ruleCount, *endpointCount)
	} else {
		fmt.Print(" ingest-only")
	}
	fmt.Println(")")

	var tokens chan struct{}
	if *rps > 0 {
		tokens = startPacer(*rps)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	var total, okCount atomic.Int64
	results := make(chan reqResult, *concurrency*8)
	st := &stats{codes: map[int]int{}}

	// Single collector owns the shared stats so progress reports can lock it.
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for r := range results {
			st.add(r)
		}
	}()

	start := time.Now()
	var workers sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			target := "/endpoint/" + slugs[id%len(slugs)]
			seq := 0
			for {
				if *rps > 0 {
					select {
					case <-tokens:
					case <-ctx.Done():
						return
					}
				} else if ctx.Err() != nil {
					return
				}
				seq++
				total.Add(1)
				var msgVal string
				if len(msgValues) > 0 {
					msgVal = msgValues[seq%len(msgValues)]
				} else {
					msgVal = "notygo-load"
				}
				payload := fmt.Sprintf(`{"msg":%q,"worker":%d,"seq":%d,"ts":%q}`,
					msgVal, id, seq, time.Now().UTC().Format(time.RFC3339Nano))
				reqStart := time.Now()
				code, _, err := client.do("POST", target, []byte(payload))
				lat := time.Since(reqStart)
				if err != nil {
					results <- reqResult{lat: lat, code: -1}
					continue
				}
				if code/100 == 2 {
					okCount.Add(1)
				}
				results <- reqResult{lat: lat, code: code}
			}
		}(i)
	}

	// Progress reports while the load is running.
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		lastTotal := int64(0)
		lastAt := start
		t := time.NewTicker(*reportEvery)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				n, codes := st.snapshot()
				window := float64(int64(n)-lastTotal) / time.Since(lastAt).Seconds()
				lastTotal, lastAt = int64(n), time.Now()
				errs := int64(n) - okCount.Load()
				fmt.Printf("  [%6.1fs] total=%d rps=%.0f ok=%d err=%d codes=%v\n",
					time.Since(start).Seconds(), n, window, okCount.Load(), errs, codes)
			case <-ctx.Done():
				return
			}
		}
	}()

	workers.Wait()
	cancel()
	<-progressDone
	close(results)
	<-collectorDone

	elapsed := time.Since(start)
	n, codes := st.snapshot()
	fmt.Println()
	fmt.Printf("=== load summary ===\n")
	fmt.Printf("requests:   %d in %.1fs (%.0f rps avg)\n", n, elapsed.Seconds(), float64(n)/elapsed.Seconds())
	fmt.Printf("accepted:   %d (2xx)\n", okCount.Load())
	fmt.Printf("transport errors: %d\n", codes[-1])
	printLatencies(st)

	other := map[int]int{}
	for c, cnt := range codes {
		if c != -1 && c/100 != 2 {
			other[c] = cnt
		}
	}
	if len(other) > 0 {
		fmt.Printf("non-2xx:     %v\n", other)
		if other[http.StatusTooManyRequests] > 0 {
			fmt.Printf("hint: 429 = global ingest limiter (INGEST_GLOBAL_RATE_LIMIT, default 100/200); raise it or cap with -rps\n")
		}
	}

	if *drainWait > 0 {
		waitDrain(client, *drainWait, okCount.Load(), start)
	}
}

func printLatencies(st *stats) {
	st.mu.Lock()
	lats := make([]time.Duration, len(st.latencies))
	copy(lats, st.latencies)
	st.mu.Unlock()
	if len(lats) == 0 {
		return
	}
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	p := func(q float64) string {
		i := int(float64(len(lats)-1) * q)
		return lats[i].Truncate(time.Microsecond).String()
	}
	fmt.Printf("latency:    min=%s p50=%s p90=%s p99=%s p99.9=%s max=%s\n",
		lats[0], p(0.50), p(0.90), p(0.99), p(0.999), lats[len(lats)-1])
}

func login(c *apiClient) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": *username, "password": *password})
	code, respBody, err := c.do("POST", "/api/v1/auth/login", body)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", code, string(respBody))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("empty token in response: %s", string(respBody))
	}
	return out.Token, nil
}

// setupLoadTest prepares the ingest targets and (in pipeline mode) the whole
// processing chain: N templates + N rules (rule i matches msg == msgValues[i],
// no channels so no delivery jobs), M mappings each binding all rules, and M
// endpoints (one mapping each). Everything is reused across runs by name.
// Returns the endpoint slugs workers should spread across.
func setupLoadTest(c *apiClient) ([]string, error) {
	if !*pipeline {
		if err := ensureBoundEndpoint(c, *slug, "Load Test", ""); err != nil {
			return nil, err
		}
		return []string{*slug}, nil
	}

	msgValues = make([]string, *ruleCount)
	ruleIDs := make([]string, 0, *ruleCount)
	for i := 0; i < *ruleCount; i++ {
		msgValues[i] = fmt.Sprintf("notygo-load-%d", i)
		tplName := fmt.Sprintf("Load Test Template %d", i)
		tplBody, _ := json.Marshal(map[string]any{
			"name":           tplName,
			"format":         "text",
			"title_template": "Load Test",
			"body_template":  fmt.Sprintf("{{.msg}} (rule %d)", i),
		})
		tplID, err := ensureByName(c, "/api/v1/templates", tplName, tplBody)
		if err != nil {
			return nil, err
		}
		ruleName := fmt.Sprintf("Load Test Rule %d", i)
		ruleBody, _ := json.Marshal(map[string]any{
			"name":     ruleName,
			"enabled":  true,
			"priority": 100 + i,
			"conditions": map[string]any{
				"conditions": []map[string]any{
					{"field": "msg", "operator": "eq", "value": msgValues[i]},
				},
			},
			"template_id": tplID,
			"channel_ids": []string{}, // no deliveries: measure processing only
		})
		ruleID, err := ensureByName(c, "/api/v1/rules", ruleName, ruleBody)
		if err != nil {
			return nil, err
		}
		ruleIDs = append(ruleIDs, ruleID)
	}
	fmt.Printf("prepared %d rules (payload rotates so each rule matches 1/%d of events)\n", *ruleCount, *ruleCount)

	slugs := make([]string, 0, *endpointCount)
	for i := 0; i < *endpointCount; i++ {
		epSlug := *slug
		if i > 0 {
			epSlug = fmt.Sprintf("%s-%d", *slug, i+1)
		}
		mapName := fmt.Sprintf("Load Test Mapping %d", i)
		mapBody, _ := json.Marshal(map[string]any{
			"name":              mapName,
			"rule_ids":          ruleIDs,
			"continue_on_match": false,
			"fields": []map[string]any{
				{"payload_path": "msg", "target_key": "msg", "value_type": "string", "required": true},
			},
		})
		mapID, err := ensureByName(c, "/api/v1/mappings", mapName, mapBody)
		if err != nil {
			return nil, err
		}
		if err := ensureBoundEndpoint(c, epSlug, fmt.Sprintf("Load Test %d", i), mapID); err != nil {
			return nil, err
		}
		slugs = append(slugs, epSlug)
	}
	return slugs, nil
}

// ensureByName finds an object by name via its list endpoint (page size 200)
// or creates it, returning its id. Setup objects are reused across runs.
func ensureByName(c *apiClient, apiPath, name string, createBody []byte) (string, error) {
	code, body, err := c.do("GET", apiPath+"?size=200", nil)
	if err == nil && code == http.StatusOK {
		var list struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &list) == nil {
			for _, it := range list.Data {
				if it.Name == name {
					return it.ID, nil
				}
			}
		}
	}
	code, body, err = c.do("POST", apiPath, createBody)
	if err != nil {
		return "", err
	}
	if code == http.StatusCreated {
		var out struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &out) == nil && out.ID != "" {
			fmt.Printf("created %s\n", name)
			return out.ID, nil
		}
		return "", fmt.Errorf("create %s: no id in response: %s", name, string(body))
	}
	return "", fmt.Errorf("create %s: status %d: %s", name, code, string(body))
}

// ensureBoundEndpoint creates the endpoint if missing (binding mappingID when
// non-empty) or binds the mapping via PUT when an older run left it unbound.
func ensureBoundEndpoint(c *apiClient, slug, name, mappingID string) error {
	code, body, err := c.do("GET", "/api/v1/endpoints?size=200", nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("list endpoints: status %d", code)
	}
	var list struct {
		Data []struct {
			ID        string  `json:"id"`
			Slug      string  `json:"slug"`
			MappingID *string `json:"mapping_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return err
	}
	for _, e := range list.Data {
		if e.Slug != slug {
			continue
		}
		if mappingID != "" && (e.MappingID == nil || *e.MappingID == "") {
			bind, _ := json.Marshal(map[string]string{"mapping_id": mappingID})
			code, body, err = c.do("PUT", "/api/v1/endpoints/"+e.ID, bind)
			if err != nil || code != http.StatusOK {
				return fmt.Errorf("bind mapping to endpoint %s: status %d: %v %s", slug, code, err, string(body))
			}
			fmt.Printf("bound mapping to endpoint %q\n", slug)
		}
		return nil
	}
	create, _ := json.Marshal(map[string]any{
		"name":                 name,
		"slug":                 slug,
		"auth_type":            "none",
		"payload_content_type": "application/json",
		"mapping_id":           mappingID,
	})
	code, body, err = c.do("POST", "/api/v1/endpoints", create)
	if err != nil {
		return err
	}
	if code == http.StatusCreated {
		fmt.Printf("created endpoint %q\n", slug)
		return nil
	}
	return fmt.Errorf("create endpoint %s: status %d: %s", slug, code, string(body))
}

func startPacer(rate int) chan struct{} {
	tokens := make(chan struct{}, rate)
	// ≥10 rps: 100ms ticks carrying rate/10 tokens each (Windows timer
	// resolution makes per-request ticks unreliable above ~1ms).
	interval := 100 * time.Millisecond
	perTick := rate / 10
	if perTick == 0 { // low rates: exact one-token ticks
		perTick = 1
		interval = time.Second / time.Duration(rate)
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			for i := 0; i < perTick; i++ {
				select {
				case tokens <- struct{}{}:
				default:
				}
			}
		}
	}()
	return tokens
}

func waitDrain(c *apiClient, max time.Duration, events int64, loadStart time.Time) {
	fmt.Println()
	fmt.Printf("=== queue drain (max %s) ===\n", max)
	deadline := time.Now().Add(max)
	for {
		eventBacklog, deliveryBacklog, err := queueBacklog(c)
		if err != nil {
			fmt.Println("queue stats error:", err)
			return
		}
		if eventBacklog == 0 && deliveryBacklog == 0 {
			throughput := float64(events) / time.Since(loadStart).Seconds()
			fmt.Printf("drained in %.1fs after load start; consumer throughput ≈ %.0f events/s (accepted %d events total)\n",
				time.Since(loadStart).Seconds(), throughput, events)
			return
		}
		if time.Now().After(deadline) {
			fmt.Printf("drain timeout after %s — event_process=%d delivery_send=%d still queued\n",
				max, eventBacklog, deliveryBacklog)
			return
		}
		fmt.Printf("  backlog: event_process=%d delivery_send=%d\n", eventBacklog, deliveryBacklog)
		time.Sleep(2 * time.Second)
	}
}

func queueBacklog(c *apiClient) (int, int, error) {
	code, body, err := c.do("GET", "/api/v1/queue/stats", nil)
	if err != nil {
		return 0, 0, err
	}
	if code != http.StatusOK {
		return 0, 0, fmt.Errorf("status %d: %s", code, string(body))
	}
	var out struct {
		Queues []struct {
			Queue string `json:"queue"`
			State string `json:"state"`
			Count int    `json:"count"`
		} `json:"queues"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, 0, err
	}
	// Everything that is not a terminal state counts as backlog.
	var eventBacklog, deliveryBacklog int
	for _, q := range out.Queues {
		switch q.State {
		case "available", "running", "retryable", "scheduled", "pending":
			switch q.Queue {
			case "event_process":
				eventBacklog += q.Count
			case "delivery_send":
				deliveryBacklog += q.Count
			}
		}
	}
	return eventBacklog, deliveryBacklog, nil
}
