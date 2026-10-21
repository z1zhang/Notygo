package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginGuardLockout(t *testing.T) {
	g := NewLoginGuard(3, 10*time.Minute)
	key := "ip:1.2.3.4"

	for i := 0; i < 2; i++ {
		g.recordFailure(key)
	}
	if g.isLocked(key) {
		t.Fatalf("locked after %d failures, threshold is 3", 2)
	}

	g.recordFailure(key)
	if !g.isLocked(key) {
		t.Fatal("expected lock after 3 failures within window")
	}

	// A different key is unaffected
	if g.isLocked("ip:5.6.7.8") {
		t.Fatal("unrelated key should not be locked")
	}

	// Success clears the history
	g.reset(key)
	if g.isLocked(key) {
		t.Fatal("reset should clear the failure history")
	}
}

func TestLoginGuardWindowExpiry(t *testing.T) {
	g := NewLoginGuard(2, 20*time.Millisecond)
	key := "user:admin"

	g.recordFailure(key)
	time.Sleep(25 * time.Millisecond)
	g.recordFailure(key) // first failure is now outside the window and pruned

	if g.isLocked(key) {
		t.Fatal("failures outside the lockout window should not accumulate")
	}
}

func TestLoginGuardMaxFailuresFloor(t *testing.T) {
	g := NewLoginGuard(0, time.Minute)
	g.recordFailure("ip:1.1.1.1")
	if !g.isLocked("ip:1.1.1.1") {
		t.Fatal("maxFailures below 1 should be clamped to 1")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	r.RemoteAddr = "203.0.113.7:51234"
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("clientIP = %q, want 203.0.113.7", got)
	}

	// Bare address without port is returned as-is
	r.RemoteAddr = "203.0.113.7"
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("clientIP = %q, want 203.0.113.7", got)
	}
}
