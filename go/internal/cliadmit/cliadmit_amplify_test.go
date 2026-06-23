package cliadmit_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/cliadmit"
)

// setAmpXDG isolates slot files to a per-test temp dir so amplification tests
// cannot collide with each other or with real slot files.
func setAmpXDG(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
}

// TestAcquireAmplify_DistinctCLIsNoInterference asserts that two goroutines
// each acquiring a DISTINCT CLI name (both at max=1) do not block each other.
// Slot files are keyed by CLI name; no cross-name interference is the invariant.
func TestAcquireAmplify_DistinctCLIsNoInterference(t *testing.T) {
	setAmpXDG(t)

	type result struct {
		rel func()
		err error
	}
	results := make([]result, 2)
	var wg sync.WaitGroup

	for i, cli := range []string{"amp-alpha", "amp-beta"} {
		i, cli := i, cli
		wg.Add(1)
		go func() {
			defer wg.Done()
			rel, err := cliadmit.Acquire(context.Background(), cli, 1, cliadmit.DefaultTTL)
			results[i] = result{rel: rel, err: err}
		}()
	}
	wg.Wait()

	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d: Acquire failed — distinct CLIs must not interfere: %v", i, r.err)
			continue
		}
		r.rel()
	}
}

// TestAcquireAmplify_BurstNAtMaxN asserts that N concurrent callers with max=N
// all succeed immediately — none should block or be denied.
func TestAcquireAmplify_BurstNAtMaxN(t *testing.T) {
	setAmpXDG(t)

	const N = 5
	var failures int64
	releases := make([]func(), N)
	var wg sync.WaitGroup

	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			rel, err := cliadmit.Acquire(context.Background(), "amp-burst", N, cliadmit.DefaultTTL)
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			releases[i] = rel // wg.Done() happens-before wg.Wait(), no data race
		}()
	}
	wg.Wait()

	if f := atomic.LoadInt64(&failures); f != 0 {
		t.Errorf("BurstNAtMaxN: %d/%d goroutines failed (max=%d should admit all)", f, N, N)
	}
	for _, rel := range releases {
		if rel != nil {
			rel()
		}
	}
}

// TestAcquireAmplify_DoubleRelease asserts that calling release() twice is safe:
// the second call must not panic, and a subsequent Acquire on the same slot must
// still succeed (proving the slot file is not permanently corrupted).
func TestAcquireAmplify_DoubleRelease(t *testing.T) {
	setAmpXDG(t)

	rel, err := cliadmit.Acquire(context.Background(), "amp-double-rel", 1, cliadmit.DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	rel() // nominal release

	// Second release must not panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("second release() panicked: %v", r)
			}
		}()
		rel()
	}()

	// Slot must be free for a subsequent caller.
	rel2, err := cliadmit.Acquire(context.Background(), "amp-double-rel", 1, cliadmit.DefaultTTL)
	if err != nil {
		t.Errorf("Acquire after double-release must succeed: %v", err)
		return
	}
	rel2()
}

// TestAcquireAmplify_PreCancelledContextBlocked asserts that Acquire returns an
// error immediately when the context is already cancelled AND all slots are full.
// The implementation must not hang in the backoff loop with a dead context.
func TestAcquireAmplify_PreCancelledContextBlocked(t *testing.T) {
	setAmpXDG(t)

	// Hold the only slot so the second Acquire must enter the backoff path.
	holder, err := cliadmit.Acquire(context.Background(), "amp-precancel", 1, cliadmit.DefaultTTL)
	if err != nil {
		t.Fatalf("setup Acquire: %v", err)
	}
	defer holder()

	// Cancel the context BEFORE calling Acquire (so it arrives dead at the backoff loop).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := cliadmit.Acquire(ctx, "amp-precancel", 1, cliadmit.DefaultTTL)
		if err == nil {
			t.Errorf("Acquire with pre-cancelled context + full slots should return error, got nil")
		}
	}()

	select {
	case <-done:
		// good — returned promptly without hanging
	case <-time.After(2 * time.Second):
		t.Error("Acquire with pre-cancelled context blocked >2s (must detect dead ctx and return)")
	}
}

// TestAcquireAmplify_CorruptedSlotFile asserts that Acquire self-heals when the
// slot file contains malformed JSON and still admits the caller (readHolders
// self-healing path noted in the build-report).
func TestAcquireAmplify_CorruptedSlotFile(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdgDir)

	// Pre-populate the expected slot file with invalid JSON.
	slotDir := filepath.Join(xdgDir, "evolve")
	if err := os.MkdirAll(slotDir, 0o700); err != nil {
		t.Fatalf("mkdir evolve: %v", err)
	}
	slotPath := filepath.Join(slotDir, "cli-amp-corrupt.slots")
	if err := os.WriteFile(slotPath, []byte("{not valid json!!"), 0o600); err != nil {
		t.Fatalf("write corrupt slot: %v", err)
	}

	rel, err := cliadmit.Acquire(context.Background(), "amp-corrupt", 1, cliadmit.DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire with corrupted slot file must self-heal and succeed, got: %v", err)
	}
	rel()
}

// TestAcquireAmplify_MaxTwoThirdBlocking asserts the blocking path with max=2:
// first two callers admit immediately; the third blocks until one slot is freed.
func TestAcquireAmplify_MaxTwoThirdBlocking(t *testing.T) {
	setAmpXDG(t)

	ctx := context.Background()
	const max = 2

	rel1, err := cliadmit.Acquire(ctx, "amp-max2", max, cliadmit.DefaultTTL)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	rel2, err := cliadmit.Acquire(ctx, "amp-max2", max, cliadmit.DefaultTTL)
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}

	type acqResult struct {
		rel func()
		err error
	}
	thirdCh := make(chan acqResult, 1)
	go func() {
		rel, err := cliadmit.Acquire(ctx, "amp-max2", max, cliadmit.DefaultTTL)
		thirdCh <- acqResult{rel, err}
	}()

	// Allow the third goroutine time to enter the backoff loop.
	time.Sleep(100 * time.Millisecond)

	select {
	case res := <-thirdCh:
		if res.err == nil {
			t.Errorf("third Acquire returned immediately — should be blocking while both max=%d slots are held", max)
			if res.rel != nil {
				res.rel()
			}
		}
	default:
		// expected: third goroutine is still blocking
	}

	// Free one slot — the third caller must unblock and succeed.
	rel1()

	select {
	case res := <-thirdCh:
		if res.err != nil {
			t.Errorf("third Acquire must succeed after slot freed, got: %v", res.err)
		} else if res.rel != nil {
			res.rel()
		}
	case <-time.After(5 * time.Second):
		t.Error("third Acquire did not unblock within 5s after slot freed")
	}

	rel2()
}
