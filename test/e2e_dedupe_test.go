package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"notygo/internal/delivery"
	"os"
)

// TestDedupeConcurrentFirstHitWins hammers CheckAndInsertDedupeKey with
// concurrent identical (rule, hash) pairs — the exact interleaving that used
// to double-insert under the two-step version. The unique index must
// arbitrate: exactly one caller sees first hit, every other caller is told
// duplicate, and only one row lands in the table.
func TestDedupeConcurrentFirstHitWins(t *testing.T) {
	if os.Getenv("TEST_DATABASE_DSN") == "" {
		t.Skip("TEST_DATABASE_DSN not set; skipping integration tests")
	}
	ts := newTestStack(t)
	repo := delivery.NewRepository(ts.pool)
	ctx := context.Background()

	const callers = 20
	now := time.Now().UTC()
	var wg sync.WaitGroup
	results := make([]*delivery.DedupeCheckResult, callers)
	errs := make([]error, callers)

	wg.Add(callers)
	for i := range callers {
		go func() {
			defer wg.Done()
			results[i], errs[i] = repo.CheckAndInsertDedupeKey(ctx, "rule-race", "hash-race", 60, now)
		}()
	}
	wg.Wait()

	firstHits, duplicates := 0, 0
	for i := range callers {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		switch {
		case !results[i].IsDuplicate:
			firstHits++
		case results[i].IsDuplicate:
			duplicates++
		}
	}
	if firstHits != 1 || duplicates != callers-1 {
		t.Fatalf("got %d first hits / %d duplicates, want exactly 1 / %d", firstHits, duplicates, callers-1)
	}

	var rows int
	if err := ts.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notygo_dedupe_key WHERE rule_id = 'rule-race'`,
	).Scan(&rows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("got %d rows for (rule-race, hash-race), want 1", rows)
	}
}

// TestDedupeWindowSemantics pins the window contract against a controlled
// clock (CheckAndInsertDedupeKey takes `now` as a parameter precisely so
// tests never wait on real time):
//   - the window is fixed from the first hit — later hits do not extend it;
//   - once expired, the next hit re-arms a fresh window instead of being
//     suppressed forever by the stale row.
func TestDedupeWindowSemantics(t *testing.T) {
	if os.Getenv("TEST_DATABASE_DSN") == "" {
		t.Skip("TEST_DATABASE_DSN not set; skipping integration tests")
	}
	ts := newTestStack(t)
	repo := delivery.NewRepository(ts.pool)
	ctx := context.Background()

	const window = 60
	t0 := time.Now().UTC().Truncate(time.Microsecond)

	steps := []struct {
		name string
		at   time.Time
		want bool // IsDuplicate
	}{
		{"first hit arms the window", t0, false},
		{"hit inside window is duplicate", t0.Add(30 * time.Second), true},
		{"duplicate does not extend window", t0.Add(50 * time.Second), true},
		{"hit after expiry re-arms", t0.Add(61 * time.Second), false},
		{"re-armed window suppresses", t0.Add(70 * time.Second), true},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			res, err := repo.CheckAndInsertDedupeKey(ctx, "rule-window", "hash-window", window, step.at)
			if err != nil {
				t.Fatalf("check at %s: %v", step.at.Format(time.RFC3339), err)
			}
			if res.IsDuplicate != step.want {
				t.Errorf("at %s: IsDuplicate = %v, want %v", step.at.Format(time.RFC3339), res.IsDuplicate, step.want)
			}
		})
	}

	// The re-arm at t0+61s must have set expires_at = t0+121s: the fixed
	// window from step 3 (which would have been t0+110s had it slid) must
	// not exist, proving duplicates never moved the deadline.
	var expiresAt time.Time
	if err := ts.pool.QueryRow(ctx,
		`SELECT expires_at FROM notygo_dedupe_key WHERE rule_id = 'rule-window'`,
	).Scan(&expiresAt); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if want := t0.Add(121 * time.Second); !expiresAt.Equal(want) {
		t.Errorf("final expires_at = %s, want %s (re-armed at t0+61s, not slid to t0+110s)",
			expiresAt.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}
