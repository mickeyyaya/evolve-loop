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
		go func() {
			defer wg.Done()
			<-start
			_ = st.WriteCycleState(ctx, core.CycleState{CycleID: 1, Phase: "audit"})
		}()
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
