package verifylock

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
	release()
	release2, err := AcquireAt(context.Background(), lockPath, nil)
	if err != nil {
		t.Fatalf("lock not reacquirable after release: %v", err)
	}
	release2()
}

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

func TestLockPath_IsTheOneFileAcquireLocks(t *testing.T) {
	t.Parallel()
	hubRoot, plainRoot := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(hubRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hubRoot, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		root, want string
		hub        bool
	}{
		{hubRoot, filepath.Join(hubRoot, ".git", "evolve-verify.lock"), true},
		{plainRoot, filepath.Join(plainRoot, ".evolve", "evolve-verify.lock"), false},
	} {
		path, hub := LockPath(c.root)
		release, err := Acquire(context.Background(), c.root, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, statErr := os.Stat(path)
		release()
		if path != c.want || hub != c.hub || statErr != nil {
			t.Errorf("LockPath(%s) = %q, %t (stat %v); want %q, %t and the file Acquire locks", c.root, path, hub, statErr, c.want, c.hub)
		}
	}
}
