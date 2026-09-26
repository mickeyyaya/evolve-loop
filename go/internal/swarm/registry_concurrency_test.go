package swarm

import (
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestSessionRegistry_ConcurrentRegister_NoRaceNoLostUpdate(t *testing.T) {
	// In-memory mode isolates the slice race the mutex guards from per-call manifest I/O.
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
			if err := r.MarkReaped(fmt.Sprintf("w-%d-%d", g, i-1)); err != nil {
				t.Errorf("mark-reaped: %v", err)
			}
		}
	})

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
