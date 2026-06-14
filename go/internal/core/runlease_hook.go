package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// runlease_hook.go — ADR-0049 G16: the per-run .lease PRODUCER.
//
// runlease (the SSOT for the heartbeat file) was already CONSUMED by gc's
// liveness check (a run dir with a fresh lease is LIVE, never collected), but
// nothing WROTE one — so every concurrent fleet run was leaseless and a gc pass
// could reap a sibling's run dir mid-cycle. RunCycle now writes the lease at
// cycle start and refreshes it on a heartbeat for the run's lifetime.
//
// The producer is per-cycle and in-process (not a central fleet pump): each
// cycle owns its own lease, so under `evolve fleet` each child process leases
// its own run dir with no shared writer. The heartbeat is a bare ticker — it
// only sleeps, so a legitimately-busy-but-alive think-heavy phase never stalls
// it (we deliberately do NOT couple liveness to phase progress, which would
// reintroduce the busy-pane false-negative class).

// leaseRefreshInterval is the heartbeat cadence: DefaultTTL/2, so a reader
// always sees a heartbeat strictly less than TTL old while the writer is alive
// (the documented "refresh at least twice per window" contract).
func leaseRefreshInterval() time.Duration { return runlease.DefaultTTL / 2 }

// startRunLease writes the initial per-run lease into workspace and spawns a
// heartbeat goroutine that refreshes it every interval until the returned stop
// is called. An empty workspace (worktree-less / test cycle) is a safe no-op.
//
// The initial write completes BEFORE startRunLease returns — the WRITE-ORDERING
// INVARIANT (CE.3): a gc pass that snapshots liveness after RunCycle has begun
// never observes this run unleased. A failed write WARNs and degrades to the
// run-state liveness fallback rather than aborting the cycle.
func startRunLease(workspace, runID string, nowFn func() time.Time, interval time.Duration) (stop func()) {
	if workspace == "" {
		return func() {}
	}
	// Own the run-dir creation rather than relying on a storage-adapter
	// side-effect: runlease.Write needs the dir to exist to place its tmp file.
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN runlease mkdir %s: %v\n", workspace, err)
	}
	lease := runlease.Lease{RunID: runID, OwnerPID: os.Getpid()}
	if err := runlease.Write(workspace, lease, nowFn()); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN runlease initial write %s: %v\n", workspace, err)
	}
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go runLeaseHeartbeat(workspace, lease, ticker.C, nowFn, done)
	// sync.Once so a second stop() (e.g. a future early-exit caller adding one
	// before the defer) cannot panic on close-of-closed-channel.
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			ticker.Stop()
		})
	}
}

// runLeaseHeartbeat refreshes the lease on every tick until done is closed.
// Split out so tests drive refreshes deterministically via an injected channel.
func runLeaseHeartbeat(workspace string, lease runlease.Lease, tick <-chan time.Time, nowFn func() time.Time, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-tick:
			if err := runlease.Write(workspace, lease, nowFn()); err != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN runlease refresh %s: %v\n", workspace, err)
			}
		}
	}
}
