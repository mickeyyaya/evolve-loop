package shipwindow

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShipWindowLease_SiblingWaitsInsteadOfReaudit(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var mainHead int64
	var inSection int64
	var overlaps int64
	var headMoved int64

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
			snapshot := atomic.LoadInt64(&mainHead)
			time.Sleep(25 * time.Millisecond)
			if atomic.LoadInt64(&mainHead) != snapshot {
				atomic.AddInt64(&headMoved, 1)
			}
			atomic.AddInt64(&mainHead, 1)
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

func TestShipWindowLease_HolderDeathRecovered(t *testing.T) {
	t0 := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)

	t.Run("dead holder broken before TTL", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Acquire(context.Background(), dir, Options{
			Now:      func() time.Time { return t0 },
			OwnerPID: 4194000,
		})
		if err != nil {
			t.Fatalf("holder Acquire: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		l, err := Acquire(ctx, dir, Options{
			Now:   func() time.Time { return t0.Add(time.Second) },
			Alive: func(int) bool { return false },
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
			Now:   func() time.Time { return t0.Add(DefaultTTL + time.Second) },
			Alive: func(int) bool { return true },
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

func TestShipWindowLease_PathIn(t *testing.T) {
	got := PathIn("/some/root/.evolve")
	want := "/some/root/.evolve/ship-window.lock"
	if got != want {
		t.Errorf("PathIn = %q, want %q", got, want)
	}
}
