package checkpoint

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestPhaseBoundaryCheckpointer_ConcurrentEscalation_NotClobbered is the N17
// (ADR-0049) regression. PhaseBoundaryCheckpointer used to read the escalation
// guard UNLOCKED (hasEscalationCheckpoint) and then write phase-complete under a
// SEPARATE lock (ApplyToStateFile) — a check-and-act straddling the lock
// boundary. In fleet mode concurrent cycles share the host-global
// cycle-state.json, so in the TOCTOU window a peer could write an escalation
// checkpoint (quota-likely / batch-cap-near / operator-requested / stall) that
// this lowest-priority phase-complete write then clobbered — a lost escalation
// the consumer (e.g. detectQuotaPause after RunCycle) never sees.
//
// Folding the check+act under ONE flock.WithPathLock makes BOTH serialized
// orders converge on the escalation:
//   - peer writes escalation first -> our guard reads it under the lock -> yields
//   - we write phase-complete first -> peer's escalation lands after -> survives
//
// so the final reason is ALWAYS the escalation. A start barrier forces the two
// to overlap, so the pre-fix lost-update trips across the iterations.
func TestPhaseBoundaryCheckpointer_ConcurrentEscalation_NotClobbered(t *testing.T) {
	if core.PhaseBoundaryCheckpointer == nil {
		t.Fatal("core.PhaseBoundaryCheckpointer not registered (init() wiring missing)")
	}
	const iters = 300
	now := time.Unix(1770000000, 0)
	for i := 0; i < iters; i++ {
		root := t.TempDir()
		seedCycleState(t, root, "") // no checkpoint yet — both writers race from clean state
		path := filepath.Join(root, ".evolve", "cycle-state.json")
		cs := core.CycleState{CycleID: 234}

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		var phaseErr, escErr error
		// Writer A: the phase-boundary checkpoint (phase-complete, lowest priority).
		go func() {
			defer wg.Done()
			<-start
			phaseErr = core.PhaseBoundaryCheckpointer(cs, root, now)
		}()
		// Writer B: a concurrent peer escalation checkpoint (quota-likely).
		go func() {
			defer wg.Done()
			<-start
			escErr = ApplyToStateFile(path, Compose(cs, ReasonQuotaLikely, 0, "", now))
		}()
		close(start)
		wg.Wait()

		if phaseErr != nil {
			t.Fatalf("iter %d phase checkpoint: %v", i, phaseErr)
		}
		if escErr != nil {
			t.Fatalf("iter %d escalation checkpoint: %v", i, escErr)
		}
		if got := readCheckpointReason(t, root); got != string(ReasonQuotaLikely) {
			t.Fatalf("iter %d lost escalation: reason=%q want %q (phase-complete clobbered a concurrent escalation through the TOCTOU window)",
				i, got, ReasonQuotaLikely)
		}
	}
}
