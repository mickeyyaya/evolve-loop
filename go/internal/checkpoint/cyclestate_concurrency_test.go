package checkpoint_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestCycleState_ConcurrentWriters_BothUpdatesSurvive is the G7 (ADR-0049)
// regression. Two writers read-modify-write the SAME cycle-state.json:
//   - the phase writer (storage.WriteCycleState) owns "phase" and preserves the
//     "checkpoint" block it reads;
//   - the checkpoint writer (checkpoint.ApplyToStateFile) owns "checkpoint" and
//     preserves the "phase" it reads.
//
// Under fleet mode the whole-cycle project lock is skipped, so the two run
// concurrently. Without a SHARED per-file lock, one writer renames over a stale
// read and the peer's update is silently reverted (lost update). Both serialized
// orders yield the SAME outcome — phase=audit AND checkpoint.gitHead=CP1 (both
// updates applied) — so any other result is a lost update. A start barrier makes
// the two RMWs overlap, so the bug trips reliably across the iterations.
func TestCycleState_ConcurrentWriters_BothUpdatesSurvive(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(0, 0).UTC()
	const iters = 200
	for i := 0; i < iters; i++ {
		dir := t.TempDir()
		st := storage.New(dir)
		path := filepath.Join(dir, "cycle-state.json")
		seed := core.CycleState{CycleID: 1, Phase: "build"}
		if err := st.WriteCycleState(ctx, seed); err != nil {
			t.Fatalf("seed phase: %v", err)
		}
		if err := checkpoint.ApplyToStateFile(path,
			checkpoint.Compose(seed, checkpoint.ReasonPhaseComplete, 0, "CP0", now)); err != nil {
			t.Fatalf("seed checkpoint: %v", err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		// Writer A: advance phase build->audit (preserves the checkpoint block).
		go func() {
			defer wg.Done()
			<-start
			_ = st.WriteCycleState(ctx, core.CycleState{CycleID: 1, Phase: "audit"})
		}()
		// Writer B: advance checkpoint CP0->CP1 (preserves the phase).
		go func() {
			defer wg.Done()
			<-start
			_ = checkpoint.ApplyToStateFile(path,
				checkpoint.Compose(seed, checkpoint.ReasonPhaseComplete, 0, "CP1", now))
		}()
		close(start)
		wg.Wait()

		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("iter %d read: %v", i, err)
		}
		var got struct {
			Phase      string `json:"phase"`
			Checkpoint struct {
				GitHead string `json:"gitHead"`
			} `json:"checkpoint"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("iter %d: torn/invalid cycle-state.json: %v\n%s", i, err, raw)
		}
		if got.Phase != "audit" || got.Checkpoint.GitHead != "CP1" {
			t.Fatalf("iter %d lost update: phase=%q checkpoint.gitHead=%q want audit/CP1",
				i, got.Phase, got.Checkpoint.GitHead)
		}
	}
}
