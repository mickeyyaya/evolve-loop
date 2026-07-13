package shipwindow

// shipwindow_test.go — RED test contract for the ship-window lease
// (cycle-778, inbox id ship-window-lease, weight 0.97, operator-boosted).
//
// PROBLEM (evolve tokens report, cycles 767-774): audit ran ~10x for 8 cycles.
// The extras are AUDIT_BINDING_HEAD_MOVED re-audits: a sibling lane lands on
// main between this lane's audit-binding snapshot (core/phase_bindings.go
// recordAuditBinding: `git rev-parse HEAD`) and its ship push, so ship's
// verifyAuditBinding sees a moved HEAD and forces a deep-tier re-audit.
// width-scaled-binding-retry makes lanes SURVIVE that race; this lease makes
// them AVOID it by serializing ONLY the binding-snapshot→push critical section.
//
// CONTRACT the Builder implements in this package (tests are the spec;
// do NOT modify them — implement production code until they pass):
//
//	// PathIn returns the lease file path: <evolveDir>/ship-window.lock.
//	func PathIn(evolveDir string) string
//
//	// DefaultTTL is the stale-break ceiling for a held lease. The window is
//	// binding-snapshot→push (~5-10 min observed), NOT the whole audit; pick
//	// a ceiling comfortably above that (runlease.DefaultTTL is the pattern).
//	const/var DefaultTTL time.Duration
//
//	type Options struct {
//	    TTL      time.Duration      // 0 ⇒ DefaultTTL
//	    Now      func() time.Time   // nil ⇒ time.Now (test seam, runlease-style)
//	    Alive    func(pid int) bool // nil ⇒ real pid probe; holder-death oracle
//	    Poll     time.Duration      // waiter poll interval; 0 ⇒ sane default
//	    OwnerPID int                // 0 ⇒ os.Getpid()
//	}
//
//	// Acquire blocks until the caller holds the ship-window lease for
//	// evolveDir, or ctx is done (then returns ctx's error). A held lease is
//	// respected only while LIVE (runlease semantics: heartbeat Fresh within
//	// TTL AND holder pid alive); a stale lease (TTL expired OR holder dead)
//	// is broken and acquisition proceeds. Waiters acquire in FIFO order of
//	// their Acquire calls (starvation fairness).
//	func Acquire(ctx context.Context, evolveDir string, opts Options) (*Lease, error)
//
//	// Release frees the lease so the next queued waiter proceeds.
//	func (l *Lease) Release() error

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestShipWindowLease_SiblingWaitsInsteadOfReaudit is the headline AC: two
// lanes race the audit→ship critical section against one simulated main HEAD.
// With the lease serializing binding-snapshot→push, the section is mutually
// exclusive, so NO lane ever observes HEAD moving between its snapshot and its
// push (zero AUDIT_BINDING_HEAD_MOVED) — and both lanes still ship.
//
// Anti-no-op teeth: a stub Acquire that returns immediately would let both
// lanes into the section simultaneously — the deliberate 25ms work window
// makes the overlap counter and/or the headMoved counter fire.
func TestShipWindowLease_SiblingWaitsInsteadOfReaudit(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var mainHead int64  // simulated main HEAD (each push advances it)
	var inSection int64 // lanes concurrently inside binding-snapshot→push
	var overlaps int64  // mutual-exclusion violations
	var headMoved int64 // AUDIT_BINDING_HEAD_MOVED analogue

	var wg sync.WaitGroup
	for lane := 0; lane < 2; lane++ {
		wg.Add(1)
		go func(lane int) {
			defer wg.Done()
			l, err := Acquire(ctx, dir, Options{Poll: 2 * time.Millisecond})
			if err != nil {
				t.Errorf("lane %d: Acquire: %v", lane, err)
				return
			}
			if atomic.AddInt64(&inSection, 1) > 1 {
				atomic.AddInt64(&overlaps, 1)
			}
			snapshot := atomic.LoadInt64(&mainHead) // the audit-binding snapshot
			time.Sleep(25 * time.Millisecond)       // audit-bound work window
			if atomic.LoadInt64(&mainHead) != snapshot {
				atomic.AddInt64(&headMoved, 1) // would be a deep-tier re-audit
			}
			atomic.AddInt64(&mainHead, 1) // the push
			atomic.AddInt64(&inSection, -1)
			if err := l.Release(); err != nil {
				t.Errorf("lane %d: Release: %v", lane, err)
			}
		}(lane)
	}
	wg.Wait()

	if got := atomic.LoadInt64(&overlaps); got != 0 {
		t.Errorf("critical section overlapped %d time(s); lease must serialize binding-snapshot→push", got)
	}
	if got := atomic.LoadInt64(&headMoved); got != 0 {
		t.Errorf("%d AUDIT_BINDING_HEAD_MOVED event(s); want 0 — siblings must wait, not re-audit", got)
	}
	if got := atomic.LoadInt64(&mainHead); got != 2 {
		t.Errorf("mainHead = %d, want 2 — queued lanes must still ship, not be dropped", got)
	}
}

// TestShipWindowLease_HeldLeaseBlocksSibling is the NEGATIVE predicate (the
// strongest anti-no-op signal): while a fresh, live-holder lease is held, a
// sibling's Acquire must NOT succeed — it blocks until its context expires and
// returns that context's error.
func TestShipWindowLease_HeldLeaseBlocksSibling(t *testing.T) {
	dir := t.TempDir()

	holder, err := Acquire(context.Background(), dir, Options{})
	if err != nil {
		t.Fatalf("holder Acquire: %v", err)
	}
	defer func() {
		if err := holder.Release(); err != nil {
			t.Errorf("holder Release: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	sibling, err := Acquire(ctx, dir, Options{Poll: 5 * time.Millisecond, OwnerPID: os.Getpid()})
	if err == nil {
		_ = sibling.Release()
		t.Fatalf("sibling acquired the lease while a fresh live-holder lease was held; want block until ctx done")
	}
	if ctx.Err() == nil {
		t.Errorf("sibling Acquire returned error %v before its context was done", err)
	}
}

// TestShipWindowLease_HolderDeathRecovered: the lease is the sole liveness
// oracle (run-lease pattern). Two independent stale-break paths, each of which
// must unblock a waiter WITHOUT operator intervention:
//
//   - dead holder: pid probe says the holder is gone ⇒ break immediately,
//     even though the heartbeat is still within TTL (the post-crash window);
//   - TTL expiry: heartbeat aged past TTL ⇒ break even if the pid still
//     exists (live-but-hung holder must not wedge every sibling ship).
func TestShipWindowLease_HolderDeathRecovered(t *testing.T) {
	t0 := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	t.Run("dead holder broken before TTL", func(t *testing.T) {
		dir := t.TempDir()
		// Holder acquires at t0 and "crashes" (never releases).
		_, err := Acquire(context.Background(), dir, Options{
			Now:      func() time.Time { return t0 },
			OwnerPID: 4194000, // arbitrary; the waiter's Alive oracle declares it dead
		})
		if err != nil {
			t.Fatalf("holder Acquire: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l, err := Acquire(ctx, dir, Options{
			Now:   func() time.Time { return t0.Add(time.Second) }, // heartbeat still Fresh
			Alive: func(int) bool { return false },                 // …but the holder is dead
			Poll:  2 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("waiter failed to break a dead-holder lease within budget: %v", err)
		}
		if err := l.Release(); err != nil {
			t.Errorf("Release after break: %v", err)
		}
	})

	t.Run("TTL-expired lease broken despite live pid", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Acquire(context.Background(), dir, Options{
			Now:      func() time.Time { return t0 },
			OwnerPID: os.Getpid(),
		})
		if err != nil {
			t.Fatalf("holder Acquire: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l, err := Acquire(ctx, dir, Options{
			Now:   func() time.Time { return t0.Add(DefaultTTL + time.Second) }, // aged past TTL
			Alive: func(int) bool { return true },                               // pid "alive" (hung holder)
			Poll:  2 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("waiter failed to break a TTL-expired lease within budget: %v", err)
		}
		if err := l.Release(); err != nil {
			t.Errorf("Release after break: %v", err)
		}
	})
}

// TestShipWindowLease_FIFOFairness: waiters acquire in the order their Acquire
// calls started — a lane queued first cannot be starved by later arrivals.
// Waiter starts are staggered well beyond the poll interval so arrival order
// is unambiguous.
func TestShipWindowLease_FIFOFairness(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	holder, err := Acquire(ctx, dir, Options{})
	if err != nil {
		t.Fatalf("holder Acquire: %v", err)
	}

	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			l, err := Acquire(ctx, dir, Options{Poll: 2 * time.Millisecond})
			if err != nil {
				t.Errorf("waiter %d: Acquire: %v", id, err)
				return
			}
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			if err := l.Release(); err != nil {
				t.Errorf("waiter %d: Release: %v", id, err)
			}
		}(i)
		// Stagger: waiter i is enqueued (many poll cycles deep) before i+1 starts.
		time.Sleep(150 * time.Millisecond)
	}

	if err := holder.Release(); err != nil {
		t.Fatalf("holder Release: %v", err)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 || order[0] != 0 || order[1] != 1 || order[2] != 2 {
		t.Errorf("acquisition order = %v, want [0 1 2] (FIFO by Acquire start)", order)
	}
}

// TestShipWindowLease_PathIn pins the on-disk contract: the lease lives at
// <evolveDir>/ship-window.lock (the path the operator greps and gc sweeps).
func TestShipWindowLease_PathIn(t *testing.T) {
	got := PathIn("/some/root/.evolve")
	want := "/some/root/.evolve/ship-window.lock"
	if got != want {
		t.Errorf("PathIn = %q, want %q", got, want)
	}
}
