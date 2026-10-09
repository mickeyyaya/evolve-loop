package flock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const failsafe = 10 * time.Second

func firedTimer() func(time.Duration) (<-chan time.Time, func() bool) {
	return func(time.Duration) (<-chan time.Time, func() bool) {
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c, func() bool { return false }
	}
}

func silentTimer() func(time.Duration) (<-chan time.Time, func() bool) {
	return func(time.Duration) (<-chan time.Time, func() bool) {
		return make(chan time.Time), func() bool { return true }
	}
}

func swapAfter(t *testing.T, fake func(time.Duration) (<-chan time.Time, func() bool)) {
	t.Helper()
	old := after
	t.Cleanup(func() { after = old })
	after = fake
}

func recordUnlocks(t *testing.T) <-chan struct{} {
	t.Helper()
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	unlocked := make(chan struct{}, 16)
	flockFn = func(fd int, how int) error {
		err := syscall.Flock(fd, how)
		if how == syscall.LOCK_UN {
			unlocked <- struct{}{}
		}
		return err
	}
	return unlocked
}

func isHeld(t *testing.T, path string) bool {
	t.Helper()
	release, held, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock(%s) = %v", path, err)
	}
	if !held {
		release()
	}
	return held
}

func mustBeFree(t *testing.T, path string) {
	t.Helper()
	release, held, err := TryLock(path)
	if err != nil || held {
		t.Fatalf("TryLock(%s) = held %v, err %v, want the lock free", path, held, err)
	}
	release()
}

func TestLockWithin_GetsAFreeLockAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ch", "loop.lock")

	release, err := LockWithin(path, time.Hour, func() {})

	if err != nil || release == nil {
		t.Fatalf("LockWithin on a free lock = %v, want the lock", err)
	}
	if !isHeld(t, path) {
		release()
		t.Fatal("the lock is free while LockWithin holds it")
	}
	release()
	mustBeFree(t, path)
}

func TestLockWithin_ReturnsAtTheDeadlineWhileAnotherHolderKeepsTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop.lock")
	holder, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	unlocked := recordUnlocks(t)
	swapAfter(t, firedTimer())

	release, err := LockWithin(path, 250*time.Millisecond, func() {})

	if !errors.Is(err, ErrLockDeadline) || release != nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("LockWithin under a held lock = %v, want ErrLockDeadline naming %s and no release", err, path)
	}
	holder()
	<-unlocked
	select {
	case <-unlocked:
	case <-time.After(failsafe):
		t.Fatal("the abandoned call never released the lock")
	}
}

func TestLockWithin_AnAbandonedCallReleasesTheLockWhenItArrives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop.lock")
	holder, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	unlocked := recordUnlocks(t)
	swapAfter(t, firedTimer())
	if _, err := LockWithin(path, time.Millisecond, func() {}); !errors.Is(err, ErrLockDeadline) {
		t.Fatalf("LockWithin = %v, want ErrLockDeadline", err)
	}

	holder()
	<-unlocked

	select {
	case <-unlocked:
	case <-time.After(failsafe):
		t.Fatal("the abandoned call kept the lock after it arrived")
	}
	mustBeFree(t, path)
}

func TestLockWithin_ADeadlineAtTheGrantInstantHasExactlyOneOwner(t *testing.T) {
	t.Run("the deadline after the grant swap waits for the hand-off", func(t *testing.T) {
		h := newHandoff()
		h.state.Store(stateHanded)
		got := make(chan bool, 1)
		go func() {
			_, owned := h.await(firedChan())
			got <- owned
		}()

		h.granted <- grant{release: func() {}}

		if owned := <-got; !owned {
			t.Fatal("the caller lost the swap but did not take the lock: no one owns it")
		}
	})
	t.Run("a grant after the deadline swap is refused", func(t *testing.T) {
		h := newHandoff()

		_, owned := h.await(firedChan())
		offered := h.offer(grant{release: func() {}}, func() {})

		if owned || offered {
			t.Fatalf("await owned %v, offer won %v, want both false: the caller abandoned first", owned, offered)
		}
	})
	t.Run("a race of both swaps has one winner", func(t *testing.T) {
		for i := 0; i < 2000; i++ {
			h := newHandoff()
			start := make(chan struct{})
			var wg sync.WaitGroup
			var offered, owned bool
			wg.Add(2)
			go func() { defer wg.Done(); <-start; offered = h.offer(grant{release: func() {}}, func() {}) }()
			go func() { defer wg.Done(); <-start; _, owned = h.await(firedChan()) }()
			close(start)
			wg.Wait()
			if offered != owned {
				t.Fatalf("trial %d: offer won %v, caller owns %v, want exactly one owner", i, offered, owned)
			}
		}
	})
}

func TestLockWithin_AGrantBeforeTheDeadlineKeepsTheLockForTheCaller(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loop.lock")
	unlocked := recordUnlocks(t)
	swapAfter(t, silentTimer())

	release, err := LockWithin(path, time.Hour, func() {})

	if err != nil {
		t.Fatalf("LockWithin = %v, want the lock", err)
	}
	if !isHeld(t, path) {
		t.Fatal("the lock is free after LockWithin returned it: the goroutine released the caller's lock")
	}
	release()
	<-unlocked
	if n := len(unlocked); n != 0 {
		t.Fatalf("%d extra unlock(s), want exactly one: the caller's", n)
	}
}

func TestLockWithin_AFlockErrorIsHandedToTheCaller(t *testing.T) {
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	want := errors.New("lock refused")
	flockFn = func(int, int) error { return want }
	swapAfter(t, silentTimer())

	release, err := LockWithin(filepath.Join(t.TempDir(), "loop.lock"), time.Hour, func() {})

	if !errors.Is(err, want) || release != nil {
		t.Fatalf("LockWithin with a flock error = %v, want %v and no release", err, want)
	}
}

func TestLockWithin_AnAbandonedFlockErrorLeavesNothingToRelease(t *testing.T) {
	old := flockFn
	t.Cleanup(func() { flockFn = old })
	var unlocks int
	flockFn = func(_ int, how int) error {
		if how == syscall.LOCK_UN {
			unlocks++
		}
		return errors.New("lock refused")
	}
	h := newHandoff()
	h.await(firedChan())
	f, err := os.CreateTemp(t.TempDir(), "lock")
	if err != nil {
		t.Fatal(err)
	}

	h.lockFor(f, f.Name(), func() {})

	if unlocks != 0 {
		t.Fatalf("an abandoned failed lock called unlock %d time(s), want 0", unlocks)
	}
	if err := f.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("the descriptor is still open after a failed lock: close = %v", err)
	}
}

func TestLockWithin_AnUnopenableLockFileFailsAtOnce(t *testing.T) {
	swapAfter(t, silentTimer())

	release, err := LockWithin(t.TempDir(), time.Hour, func() {})

	if err == nil || errors.Is(err, ErrLockDeadline) || release != nil {
		t.Fatalf("LockWithin on a directory = %v, want an open error", err)
	}
}

func firedChan() <-chan time.Time {
	c := make(chan time.Time, 1)
	c <- time.Time{}
	return c
}

func settleSignal() (func(), <-chan struct{}, *int) {
	settled := make(chan struct{}, 4)
	calls := new(int)
	return func() { *calls++; settled <- struct{}{} }, settled, calls
}

func TestLockWithin_SettlesOnceWhenItsGoroutineIsDone(t *testing.T) {
	t.Run("a granted lock settles before LockWithin returns", func(t *testing.T) {
		onSettled, settled, calls := settleSignal()

		release, err := LockWithin(filepath.Join(t.TempDir(), "loop.lock"), time.Hour, onSettled)

		if err != nil {
			t.Fatal(err)
		}
		if *calls != 1 {
			t.Fatalf("onSettled ran %d time(s) before LockWithin returned the lock, want 1", *calls)
		}
		release()
		<-settled
	})
	t.Run("an abandoned call settles only after it releases the lock", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "loop.lock")
		holder, err := Lock(path)
		if err != nil {
			t.Fatal(err)
		}
		swapAfter(t, firedTimer())
		settled := make(chan bool, 1)
		onSettled := func() {
			release, held, err := TryLock(path)
			if err == nil && !held {
				release()
			}
			settled <- err == nil && !held
		}

		if _, err := LockWithin(path, time.Millisecond, onSettled); !errors.Is(err, ErrLockDeadline) {
			t.Fatalf("LockWithin = %v, want ErrLockDeadline", err)
		}
		select {
		case <-settled:
			t.Fatal("the call settled while its goroutine still waits for the lock")
		default:
		}
		holder()

		select {
		case free := <-settled:
			if !free {
				t.Fatal("the abandoned call settled while it still held the lock")
			}
		case <-time.After(failsafe):
			t.Fatal("the call never settled")
		}
	})
	t.Run("an open error settles before LockWithin returns", func(t *testing.T) {
		onSettled, _, calls := settleSignal()

		_, err := LockWithin(t.TempDir(), time.Hour, onSettled)

		if err == nil || *calls != 1 {
			t.Fatalf("LockWithin on a directory = %v with %d settle(s), want an error and 1", err, *calls)
		}
	})
}

func waitSettled(t *testing.T, settled <-chan struct{}) {
	t.Helper()
	select {
	case <-settled:
	case <-time.After(failsafe):
		t.Fatal("the call never settled")
	}
}
