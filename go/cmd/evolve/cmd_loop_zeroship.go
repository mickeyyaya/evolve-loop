package main

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type zeroShipTally struct {
	seen  int
	waves []int
}

func (b *loopBatchCoordinator) zeroShipHalt(batchStartCycle int) (rc int, halted bool) {
	from := max(b.zeroShip.seen, batchStartCycle) + 1
	cycles := core.CollectCycleShips(b.cfg.EvolveDir, from)
	if len(cycles) == 0 {
		return 0, false
	}
	b.zeroShip.seen = cycles[len(cycles)-1].Cycle
	b.zeroShip.waves = append(b.zeroShip.waves, core.CountShips(cycles))

	ceiling := policy.DefaultSystemFailurePolicy().Thresholds.ZeroShipHalt
	pol := policy.LoadOrDefaults(filepath.Join(b.cfg.EvolveDir, "policy.json"), b.stderr)
	if fp, err := pol.FailurePolicyConfig(); err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: zero-ship breaker: failure policy unreadable (%v) — using the compiled default\n", err)
	} else {
		ceiling = fp.Thresholds.ZeroShipHalt
	}
	v := core.EvaluateZeroShip(b.zeroShip.waves, ceiling)
	if !v.Halt {
		return 0, false
	}
	latest := b.zeroShip.seen
	sf := &cyclestate.SystemFailureSignal{
		Category: "zero-ship",
		Level:    "system",
		Evidence: v.Reason + " (rule=" + v.Rule + ")",
		Halt:     true,
	}
	workspace := filepath.Join(b.cfg.EvolveDir, "runs", "cycle-"+strconv.Itoa(latest))
	rule := loopHaltRule{code: CodeLoopZeroShipHalt, fields: map[string]string{"rule": v.Rule, "waves": strconv.Itoa(v.Count)}}
	return haltOnSystemFailure(b.cfg.EvolveDir, b.cfg.ProjectRoot, latest, workspace, sf, b.stderr, b.deps.Signals, rule), true
}
