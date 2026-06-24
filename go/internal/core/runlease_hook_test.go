package core

import (
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// TestStartRunLease_WritesInitialLeaseBeforeReturn pins the WRITE-ORDERING
// INVARIANT (CE.3): the lease exists the instant startRunLease returns — before
// the cycle transitions to a non-terminal phase — so a gc pass that snapshots
// liveness after the cycle has started never sees this run unleased.
func TestStartRunLease_WritesInitialLeaseBeforeReturn(t *testing.T) {
	dir := t.TempDir()
	at := time.Unix(100, 0).UTC()
	stop := startRunLease(dir, "run-abc", func() time.Time { return at }, time.Hour)
	defer stop()

	l, ok, err := runlease.Read(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !ok {
		t.Fatal("no lease written at cycle start")
	}
	if l.RunID != "run-abc" {
		t.Errorf("RunID=%q want run-abc", l.RunID)
	}
	if !runlease.Fresh(l, at, 0) {
		t.Errorf("lease not fresh at write time: %+v", l)
	}
}

// TestStartRunLease_EmptyWorkspace_NoOp — a worktree-less / test cycle has no
// run dir to lease; startRunLease must be a safe no-op (no panic, no write).
func TestStartRunLease_EmptyWorkspace_NoOp(t *testing.T) {
	stop := startRunLease("", "run-abc", time.Now, time.Hour)
	stop() // must not panic
}

// TestRunLeaseHeartbeat_RefreshesOnTick proves the heartbeat refreshes the
// lease's HeartbeatAt on each tick (so a reader sees a heartbeat < TTL old
// while the writer is alive). Deterministic via an injected tick channel.
func TestRunLeaseHeartbeat_RefreshesOnTick(t *testing.T) {
	dir := t.TempDir()
	lease := runlease.Lease{RunID: "r1"}
	if err := runlease.Write(dir, lease, time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	tick := make(chan time.Time)
	done := make(chan struct{})
	refreshAt := time.Unix(300, 0).UTC()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runLeaseHeartbeat(dir, lease, tick, func() time.Time { return refreshAt }, done)
	}()

	tick <- time.Time{} // drive one refresh
	want := refreshAt.Format(time.RFC3339Nano)
	deadline := time.Now().Add(2 * time.Second)
	for {
		l, ok, _ := runlease.Read(dir)
		if ok && l.HeartbeatAt == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("heartbeat not refreshed to %s; got %+v", want, l)
		}
		time.Sleep(2 * time.Millisecond)
	}
	close(done)
	wg.Wait() // goroutine must exit before t.TempDir cleanup
}
