package core

// spine_replay_local_test.go — env-gated OPERATOR replay harness (skipped in
// CI/normal runs). Replays a project's REAL .evolve/runs history through
// router.Digest + StateMachine.SpineSatisfiedUpTo to answer empirically:
// would any historical cycle trip the spine floor at enforce? This is the
// soak-evidence tool behind the R8.5 flip (2026-07-16: 536 dirs replayed,
// 0 would-block on every cycle shape since ~cycle-480 after the scout/audit
// digest fallbacks; the only misses were pre-convention dirs, cycles
// 361-479). Re-run it before widening the floor (new anchors, new gates):
//
//	EVOLVE_SPINE_REPLAY_DIR=<project-root> go test -v \
//	    -run TestSpineReplay_LocalHistory ./internal/core/

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
	root := os.Getenv("EVOLVE_SPINE_REPLAY_DIR")
	if root == "" {
		t.Skip("replay harness: set EVOLVE_SPINE_REPLAY_DIR to run")
	}
	runs, err := filepath.Glob(filepath.Join(root, ".evolve", "runs", "cycle-*"))
	if err != nil || len(runs) == 0 {
		t.Fatalf("no run dirs under %s: %v", root, err)
	}
	sort.Strings(runs)

	// Production registry resolution (cmd_cycle.go:300).
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
			continue // no run.json: pre-marker dir, not replayable
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
		// Replay the gate for each transition the live loop actually took:
		// after each completed prefix, the NEXT phase was gated.
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
