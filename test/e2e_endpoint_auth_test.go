// TestEndpointAuth covers the endpoint auth feature end to end:
//
// Admin plane: CRUD with all three auth types, auth_type validation (400
// instead of the historical misleading 404/500), not-found error mapping,
// decrypted-secret responses.
//
// Data plane: the HMAC signature matrix (valid / prefix / wrong key /
// missing / non-hex / tampered), bearer and none regression, body-size
// boundary values (2KB), token-bucket rate limiting (rate=2 burst=4), and
// X-Delivery-Id dedup.
//
// The 002 migration is exercised implicitly: creating an auth_type=hmac
// endpoint INSERTs code 2, which the pre-002 CHECK constraint would reject.
//
// Run with the same gate as TestE2E:
//
//	TEST_DATABASE_DSN=postgres://notygo:notygo@master:5432/notygo?sslmode=disable \
//	  go test ./test/ -run TestEndpointAuth -v
package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testHMACSecret   = "e2e-hmac-secret"
	testBearerSecret = "e2e-bearer-secret"
)

func signBodyHex(key string, body string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// ingest posts a raw body to the data plane with arbitrary headers and
// asserts only the status code; returns the body for error-code checks.
func (s *testStack) ingest(t *testing.T, slug, body string, headers map[string]string, wantStatus int) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/endpoint/"+slug, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build ingest request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ingest %s: %v", slug, err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode != wantStatus {
		t.Fatalf("ingest %s: status = %d, want %d\nbody: %s", slug, resp.StatusCode, wantStatus, buf.String())
	}
	return buf.Bytes()
}

// mustIngestCode asserts both the HTTP status and the structured error code.
func (s *testStack) mustIngestCode(t *testing.T, slug, body string, headers map[string]string, wantStatus int, wantCode string) {
	t.Helper()
	raw := s.ingest(t, slug, body, headers, wantStatus)
	var e struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("ingest %s: decode error body: %v\nbody: %s", slug, err, raw)
	}
	if e.Error.Code != wantCode {
		t.Fatalf("ingest %s: code = %q, want %q\nbody: %s", slug, e.Error.Code, wantCode, raw)
	}
}

// createEndpoint creates an endpoint via the admin API and returns its parsed
// representation.
func (s *testStack) createEndpoint(t *testing.T, body string) map[string]any {
	t.Helper()
	var ep map[string]any
	s.mustJSON(t, "POST", "/api/v1/endpoints", &ep, body, http.StatusCreated)
	return ep
}

func TestEndpointAuth(t *testing.T) {
	if os.Getenv("TEST_DATABASE_DSN") == "" {
		t.Skip("TEST_DATABASE_DSN not set; skipping integration tests")
	}
	s := newTestStack(t)
	s.login(t)

	var hmacSlug, bearerSlug, noneSlug, boundarySlug, rateSlug string
	var bearerID, noneID string

	// ── Admin plane ──────────────────────────────────────────

	t.Run("CreateHMAC", func(t *testing.T) {
		ep := s.createEndpoint(t, fmt.Sprintf(
			`{"name":"auth-e2e-hmac","slug":"auth-e2e-hmac","auth_type":"hmac","secret":%q,"max_body_bytes":1048576}`,
			testHMACSecret))
		hmacSlug = ep["slug"].(string)
		if ep["auth_type"] != "hmac" {
			t.Fatalf("auth_type = %v, want hmac", ep["auth_type"])
		}
		// Since the secret-plaintext change, the API returns the decrypted value.
		if ep["secret"] != testHMACSecret {
			t.Fatalf("secret = %v, want %q (decrypted plaintext)", ep["secret"], testHMACSecret)
		}
	})

	t.Run("CreateBearerAndNone", func(t *testing.T) {
		bearer := s.createEndpoint(t, fmt.Sprintf(
			`{"name":"auth-e2e-bearer","slug":"auth-e2e-bearer","auth_type":"bearer","secret":%q}`,
			testBearerSecret))
		bearerSlug = bearer["slug"].(string)
		bearerID = bearer["id"].(string)
		if bearer["secret"] != testBearerSecret {
			t.Fatalf("bearer secret = %v, want %q", bearer["secret"], testBearerSecret)
		}
		none := s.createEndpoint(t, `{"name":"auth-e2e-none","slug":"auth-e2e-none","auth_type":"none"}`)
		noneSlug = none["slug"].(string)
		noneID = none["id"].(string)
	})

	t.Run("InvalidAuthTypeRejected", func(t *testing.T) {
		// Historical bug: create returned 500, update returned a misleading
		// 404 "endpoint not found". Both must be 400 now.
		raw := s.mustDo(t, "POST", "/api/v1/endpoints",
			`{"name":"bad","auth_type":"basic"}`, http.StatusBadRequest)
		if !strings.Contains(string(raw), "invalid auth_type") {
			t.Fatalf("create with bad auth_type: body = %s", raw)
		}
		raw = s.mustDo(t, "PUT", "/api/v1/endpoints/"+hmacSlug,
			`{"auth_type":"oauth"}`, http.StatusBadRequest)
		if !strings.Contains(string(raw), "invalid auth_type") {
			t.Fatalf("update with bad auth_type: body = %s", raw)
		}
	})

	t.Run("SwitchBearerToHMACKeepsSecret", func(t *testing.T) {
		// The exact scenario the user hit: edit → auth_type=hmac, no secret
		// submitted. The stored secret survives and becomes the HMAC key.
		var updated map[string]any
		s.mustJSON(t, "PUT", "/api/v1/endpoints/"+bearerID, &updated,
			`{"auth_type":"hmac"}`, http.StatusOK)
		if updated["auth_type"] != "hmac" {
			t.Fatalf("auth_type = %v, want hmac", updated["auth_type"])
		}
		if updated["secret"] != testBearerSecret {
			t.Fatalf("secret = %v, want %q (preserved through the switch)", updated["secret"], testBearerSecret)
		}
		// Old bearer token no longer authenticates (auth is hmac now).
		s.mustIngestCode(t, bearerSlug, `{"a":1}`,
			map[string]string{"Authorization": "Bearer " + testBearerSecret},
			http.StatusUnauthorized, "INGEST_001")
		// But signing with the preserved secret does.
		s.ingest(t, bearerSlug, `{"a":1}`,
			map[string]string{"X-Signature": signBodyHex(testBearerSecret, `{"a":1}`)},
			http.StatusAccepted)
	})

	t.Run("NotFoundMapping", func(t *testing.T) {
		// Genuine not-found stays 404; the repo's ErrNotFound must not swallow
		// other errors as 404 (historical bug).
		s.mustDo(t, "GET", "/api/v1/endpoints/missing-id", "", http.StatusNotFound)
		s.mustDo(t, "PUT", "/api/v1/endpoints/missing-id", `{"name":"x"}`, http.StatusNotFound)
		s.mustDo(t, "DELETE", "/api/v1/endpoints/missing-id", "", http.StatusNotFound)
	})

	t.Run("ListShowsHMAC", func(t *testing.T) {
		var page struct {
			Data []struct {
				AuthType string `json:"auth_type"`
			} `json:"data"`
		}
		s.mustJSON(t, "GET", "/api/v1/endpoints?page=1&size=50", &page, "", http.StatusOK)
		kinds := map[string]int{}
		for _, row := range page.Data {
			kinds[row.AuthType]++
		}
		if kinds["hmac"] < 2 || kinds["none"] < 1 {
			t.Fatalf("list auth_type counts = %v, want >=2 hmac and >=1 none", kinds)
		}
	})

	// ── Data plane: HMAC matrix ──────────────────────────────

	t.Run("HMACMatrix", func(t *testing.T) {
		body := `{"event":"deploy"}`
		s.ingest(t, hmacSlug, body,
			map[string]string{"X-Signature": signBodyHex(testHMACSecret, body)},
			http.StatusAccepted)
		s.ingest(t, hmacSlug, body,
			map[string]string{"X-Signature": "sha256=" + signBodyHex(testHMACSecret, body)},
			http.StatusAccepted)
		s.mustIngestCode(t, hmacSlug, body,
			map[string]string{"X-Signature": signBodyHex("wrong-key", body)},
			http.StatusUnauthorized, "INGEST_001")
		s.mustIngestCode(t, hmacSlug, body, nil, http.StatusUnauthorized, "INGEST_001")
		s.mustIngestCode(t, hmacSlug, body,
			map[string]string{"X-Signature": "zzzz-not-hex"},
			http.StatusUnauthorized, "INGEST_001")
		// Signature over a different body must not authenticate this one.
		s.mustIngestCode(t, hmacSlug, body,
			map[string]string{"X-Signature": signBodyHex(testHMACSecret, `{"event":"other"}`)},
			http.StatusUnauthorized, "INGEST_001")
	})

	t.Run("HMACWithoutSecretConfigured", func(t *testing.T) {
		ep := s.createEndpoint(t, `{"name":"auth-e2e-nosec","slug":"auth-e2e-nosec","auth_type":"hmac"}`)
		s.mustIngestCode(t, ep["slug"].(string), `{"a":1}`,
			map[string]string{"X-Signature": signBodyHex("anything", `{"a":1}`)},
			http.StatusUnauthorized, "INGEST_001")
	})

	t.Run("NoneAndBearerRegression", func(t *testing.T) {
		s.ingest(t, noneSlug, `{"a":1}`, nil, http.StatusAccepted)
		// bearerSlug was switched to hmac above; exercise plain bearer on a
		// fresh endpoint.
		ep := s.createEndpoint(t, fmt.Sprintf(
			`{"name":"auth-e2e-bearer2","slug":"auth-e2e-bearer2","auth_type":"bearer","secret":%q}`,
			testBearerSecret))
		slug := ep["slug"].(string)
		s.ingest(t, slug, `{"a":1}`,
			map[string]string{"Authorization": "Bearer " + testBearerSecret},
			http.StatusAccepted)
		s.mustIngestCode(t, slug, `{"a":1}`,
			map[string]string{"Authorization": "Bearer wrong-token"},
			http.StatusUnauthorized, "INGEST_001")
	})

	t.Run("DisabledAndUnknownSlug", func(t *testing.T) {
		s.mustJSON(t, "PUT", "/api/v1/endpoints/"+noneID, &struct{}{},
			`{"enabled":false}`, http.StatusOK)
		s.mustIngestCode(t, noneSlug, `{"a":1}`, nil, http.StatusForbidden, "INGEST_005")
		s.mustJSON(t, "PUT", "/api/v1/endpoints/"+noneID, &struct{}{},
			`{"enabled":true}`, http.StatusOK)
		s.mustIngestCode(t, "no-such-slug", `{"a":1}`, nil, http.StatusNotFound, "INGEST_005")
	})

	// ── Body-size boundary: 2KB ──────────────────────────────

	t.Run("BodyBoundary2KB", func(t *testing.T) {
		ep := s.createEndpoint(t, `{"name":"auth-e2e-b2k","slug":"auth-e2e-b2k","auth_type":"none","max_body_bytes":2048}`)
		boundarySlug = ep["slug"].(string)
		// `{"pad":"<n×x>"}` has total length 10+n.
		exact := func(total int) string {
			return `{"pad":"` + strings.Repeat("x", total-10) + `"}`
		}
		s.ingest(t, boundarySlug, exact(2047), nil, http.StatusAccepted) // below
		s.ingest(t, boundarySlug, exact(2048), nil, http.StatusAccepted) // at the limit (inclusive)
		s.mustIngestCode(t, boundarySlug, exact(2049), nil,
			http.StatusRequestEntityTooLarge, "INGEST_002") // one byte over
		// Empty body: fails JSON validation, not auth.
		s.mustIngestCode(t, boundarySlug, "", nil, http.StatusBadRequest, "INGEST_003")
	})

	t.Run("OversizeBeatsBadSignature", func(t *testing.T) {
		// Order after the reorder: read body + size check BEFORE auth, so an
		// oversized body 413s even with a garbage signature.
		ep := s.createEndpoint(t, fmt.Sprintf(
			`{"name":"auth-e2e-b2kh","slug":"auth-e2e-b2kh","auth_type":"hmac","secret":%q,"max_body_bytes":2048}`,
			testHMACSecret))
		over := `{"pad":"` + strings.Repeat("x", 2049-10) + `"}`
		s.mustIngestCode(t, ep["slug"].(string), over,
			map[string]string{"X-Signature": "deadbeef"},
			http.StatusRequestEntityTooLarge, "INGEST_002")
	})

	// ── Rate limit: rate=2/s, burst=4 ────────────────────────

	t.Run("TokenBucketRateLimit", func(t *testing.T) {
		ep := s.createEndpoint(t, `{"name":"auth-e2e-rl","slug":"auth-e2e-rl","auth_type":"none","rate_limit":"{\"rate\":2,\"burst\":4}"}`)
		rateSlug = ep["slug"].(string)
		body := `{"i":0}`

		// Burst: 4 back-to-back all pass (bucket starts full).
		for i := range 4 {
			s.ingest(t, rateSlug, fmt.Sprintf(`{"i":%d}`, i), nil, http.StatusAccepted)
		}
		// 5th immediately after: rejected, and rejection does not consume a
		// token — the 6th is still rejected.
		s.mustIngestCode(t, rateSlug, body, nil, http.StatusTooManyRequests, "INGEST_004")
		s.mustIngestCode(t, rateSlug, body, nil, http.StatusTooManyRequests, "INGEST_004")

		// ~1.2s of refill at 2/s ≈ 2.4 tokens: two pass, the third is denied.
		time.Sleep(1200 * time.Millisecond)
		s.ingest(t, rateSlug, `{"i":10}`, nil, http.StatusAccepted)
		s.ingest(t, rateSlug, `{"i":11}`, nil, http.StatusAccepted)
		s.mustIngestCode(t, rateSlug, body, nil, http.StatusTooManyRequests, "INGEST_004")

		// 2s idle refills the bucket back to capacity 4.
		time.Sleep(2 * time.Second)
		for i := range 4 {
			s.ingest(t, rateSlug, fmt.Sprintf(`{"j":%d}`, i), nil, http.StatusAccepted)
		}
	})

	// ── Dedup ────────────────────────────────────────────────

	t.Run("DuplicateDeliveryID", func(t *testing.T) {
		headers := map[string]string{"X-Delivery-Id": "e2e-dup-1"}
		s.ingest(t, noneSlug, `{"dup":1}`, headers, http.StatusAccepted)
		s.mustIngestCode(t, noneSlug, `{"dup":1}`, headers, http.StatusConflict, "INGEST_007")
	})

	// ── Cleanup: delete every endpoint created above ─────────

	t.Run("DeleteAll", func(t *testing.T) {
		for _, slug := range []string{hmacSlug, bearerSlug, noneSlug, boundarySlug, rateSlug} {
			var list struct {
				Data []struct {
					ID   string
					Slug string
				} `json:"data"`
			}
			s.mustJSON(t, "GET", "/api/v1/endpoints?page=1&size=100", &list, "", http.StatusOK)
			for _, row := range list.Data {
				if row.Slug == slug {
					s.mustDo(t, "DELETE", "/api/v1/endpoints/"+row.ID, "", http.StatusNoContent)
				}
			}
		}
		// Events survive endpoint deletion (history is not cascaded).
		var events struct {
			Total int64 `json:"total"`
		}
		s.mustJSON(t, "GET", "/api/v1/events?page=1&size=1", &events, "", http.StatusOK)
		if events.Total == 0 {
			t.Fatal("events should survive endpoint deletion")
		}
	})
}
