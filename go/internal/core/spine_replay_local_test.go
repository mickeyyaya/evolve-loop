package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestSpineReplay_LocalHistory(t *testing.T) {
	t.Parallel()

	root := os.Getenv("EVOLVE_SPINE_REPLAY_DIR")
	if root == "" {
		t.Skip("replay harness: set EVOLVE_SPINE_REPLAY_DIR to run")
	}
	runs, err := filepath.Glob(filepath.Join(root, ".evolve", "runs", "cycle-*"))
	if err != nil || len(runs) == 0 {
		t.Fatalf("no run dirs under %s: %v", root, err)
	}
	sort.Strings(runs)

	cfg, _ := config.Load(filepath.Join(root, "docs", "architecture", "phase-registry.json"), map[string]string{})
	sm := NewStateMachine()

	type miss struct{ dir, next, why string }
	var misses []miss
	replayed := 0
	for _, dir := range runs {
		var rj struct {
			CycleID         int      `json:"cycle_id"`
			CompletedPhases []string `json:"completed_phases"`
		}
		b, rerr := os.ReadFile(filepath.Join(dir, "run.json"))
		if rerr != nil {
			continue
		}
		if json.Unmarshal(b, &rj) != nil || len(rj.CompletedPhases) == 0 {
			continue
		}
		replayed++
		_, derr := router.Digest(dir, rj.CompletedPhases)
		if derr != nil {
			misses = append(misses, miss{dir, "-", "digest error: " + derr.Error()})
			continue
		}
		for i := 1; i < len(rj.CompletedPhases); i++ {
			next := Phase(rj.CompletedPhases[i])
			prefixSig, _ := router.Digest(dir, rj.CompletedPhases[:i])
			if !sm.SpineSatisfiedUpTo(next, prefixSig, cfg) {
				clean := len(prefixSig.DigestDegraded) == 0
				misses = append(misses, miss{filepath.Base(dir), string(next),
					fmt.Sprintf("would-block (cleanAbsence=%v, degraded=%v)", clean, prefixSig.DigestDegraded)})
			}
		}
	}
	t.Logf("replayed %d run dirs; %d would-block transitions", replayed, len(misses))
	for _, m := range misses {
		t.Logf("  MISS %s next=%s %s", m.dir, m.next, m.why)
	}
	if len(misses) > 0 {
		t.Errorf("%d would-block transitions remain after the fallbacks — enforce flip NOT safe; see log", len(misses))
	}
}
