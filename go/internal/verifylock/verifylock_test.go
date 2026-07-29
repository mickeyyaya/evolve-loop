package verifylock

// verifylock_test.go — ADR-0080 P1: expensive go-test verification runs
// (EGPS suite execution, build-floor coverage) are HOST-wide single-flight.
// Batch-16 halt ground truth: TouchedPackagesStayGreen red-lined three lanes
// (1166/1167/1169) while the same suite is GREEN in the preserved worktree —
// two lanes + EGPS oversubscribing one host turned a 43s suite into false
// reds. Verification correctness outranks wave throughput.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquire_SerializesAcrossHolders(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "verify.lock")
	var inCritical int32
	var maxSeen int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := AcquireAt(context.Background(), lockPath, nil)
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			n := atomic.AddInt32(&inCritical, 1)
			if n > atomic.LoadInt32(&maxSeen) {
				atomic.StoreInt32(&maxSeen, n)
			}
			time.Sleep(30 * time.Millisecond)
			atomic.AddInt32(&inCritical, -1)
			release()
		}()
	}
	wg.Wait()
	if maxSeen != 1 {
		t.Fatalf("lock admitted %d concurrent holders, want 1 — verification single-flight is the whole point", maxSeen)
	}
}

func TestAcquire_CtxCancelUnblocksWaiter(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "verify.lock")
	release, err := AcquireAt(context.Background(), lockPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if _, err := AcquireAt(ctx, lockPath, nil); err == nil {
		t.Fatal("a cancelled waiter must return an error, not block forever")
	}
}

func TestAcquire_ReleaseIsIdempotent(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "verify.lock")
	release, err := AcquireAt(context.Background(), lockPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // second call must be a safe no-op
	release2, err := AcquireAt(context.Background(), lockPath, nil)
	if err != nil {
		t.Fatalf("lock not reacquirable after release: %v", err)
	}
	release2()
}

// TestAcquire_ResolvesTheHubLockPath drives the production entry point: for
// a primary checkout the lock lands inside .git (the hub), shared by every
// worktree of the repo — the property that makes single-flight HOST-wide.
func TestAcquire_ResolvesTheHubLockPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	release, err := Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := os.Stat(filepath.Join(root, ".git", "evolve-verify.lock")); err != nil {
		t.Fatalf("lock file not hub-resident: %v", err)
	}
}
