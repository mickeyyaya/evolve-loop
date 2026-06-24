package swarm

import (
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// registry_concurrency_test.go — Phase 2 / S2.5 (modularization campaign,
// ADR-0050). SessionRegistry.mu guards r.m.Sessions, which the dispatcher
// mutates from N worker goroutines concurrently: dispatcher.go spawns one
// Register per worker when Concurrency>=2 (the default = len(plan.Workers)).
// registry_test.go had ZERO concurrency tests, and the one incidental
// concurrent path (Concurrency:2) trips -race only ~1/20 runs because the
// workers' other work staggers the two single Register goroutines — so a
// stripped mutex would pass CI ~95% of the time.
//
// This drives real contention via fixtures.StressN and asserts the no-lost-
// update invariant under -race. RED-check: delete r.mu.Lock()/Unlock() from
// Register — concurrent upsertLocked appends to r.m.Sessions race (go test
// -race trips reliably) and the final Snapshot count drops below n*k. That
// proves the test asserts the lock, not merely "did not panic".
func TestSessionRegistry_ConcurrentRegister_NoRaceNoLostUpdate(t *testing.T) {
	// In-memory mode (manifestPath=="") isolates the in-memory slice-mutation
	// race the mutex guards, without per-call manifest I/O.
	r := NewSessionRegistry("", 7, "build", 1234)

	const n, k = 8, 40
	fixtures.StressN(t, n, k, func(g, i int) {
		id := fmt.Sprintf("w-%d-%d", g, i)
		if err := r.Register(SessionHandle{WorkerID: id, Agent: id, Status: StatusLive}); err != nil {
			t.Errorf("register %s: %v", id, err)
			return
		}
		_ = r.Snapshot() // read contention against the concurrent writers
		if i > 0 {
			// Flip a prior entry — a write that must serialize with the appends.
			if err := r.MarkReaped(fmt.Sprintf("w-%d-%d", g, i-1)); err != nil {
				t.Errorf("mark-reaped: %v", err)
			}
		}
	})

	// INVARIANT: every one of the n*k DISTINCT workers registered exactly once
	// survives — none lost to a concurrent-append data race on r.m.Sessions.
	got := r.Snapshot()
	if len(got) != n*k {
		t.Errorf("registered sessions = %d, want %d (lost-update: concurrent Register dropped entries)", len(got), n*k)
	}
	seen := make(map[string]bool, len(got))
	for _, h := range got {
		if seen[h.WorkerID] {
			t.Errorf("duplicate WorkerID %s — upsert under the lock must be exactly-once", h.WorkerID)
		}
		seen[h.WorkerID] = true
	}
}
