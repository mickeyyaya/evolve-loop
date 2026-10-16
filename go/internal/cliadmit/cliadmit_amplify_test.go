package cliadmit_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliadmit"
)

func setAmpXDG(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
}

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
			releases[i] = rel
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

func TestAcquireAmplify_DoubleRelease(t *testing.T) {
	setAmpXDG(t)

	rel, err := cliadmit.Acquire(context.Background(), "amp-double-rel", 1, cliadmit.DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	rel()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("second release() panicked: %v", r)
			}
		}()
		rel()
	}()

	rel2, err := cliadmit.Acquire(context.Background(), "amp-double-rel", 1, cliadmit.DefaultTTL)
	if err != nil {
		t.Errorf("Acquire after double-release must succeed: %v", err)
		return
	}
	rel2()
}

func TestAcquireAmplify_PreCancelledContextBlocked(t *testing.T) {
	setAmpXDG(t)

	holder, err := cliadmit.Acquire(context.Background(), "amp-precancel", 1, cliadmit.DefaultTTL)
	if err != nil {
		t.Fatalf("setup Acquire: %v", err)
	}
	defer holder()

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
	case <-time.After(2 * time.Second):
		t.Error("Acquire with pre-cancelled context blocked >2s (must detect dead ctx and return)")
	}
}

func TestAcquireAmplify_CorruptedSlotFile(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdgDir)

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
	}

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
