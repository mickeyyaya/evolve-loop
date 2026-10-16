package checkpoint

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhaseBoundaryCheckpointer_ConcurrentEscalation_NotClobbered(t *testing.T) {
	if core.PhaseBoundaryCheckpointer == nil {
		t.Fatal("core.PhaseBoundaryCheckpointer not registered (init() wiring missing)")
	}
	const iters = 300
	now := time.Unix(1770000000, 0)
	for i := 0; i < iters; i++ {
		root := t.TempDir()
		seedCycleState(t, root, "")
		path := filepath.Join(root, ".evolve", "cycle-state.json")
		cs := core.CycleState{CycleID: 234}

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		var phaseErr, escErr error
		go func() {
			defer wg.Done()
			<-start
			phaseErr = core.PhaseBoundaryCheckpointer(cs, root, now)
		}()
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
