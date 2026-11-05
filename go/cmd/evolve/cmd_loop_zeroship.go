package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

type zeroShipWave struct {
	cycles []int
	ships  int
}

type zeroShipTally struct {
	from  time.Time
	floor int
	armed bool
	waves []zeroShipWave
}

func (b *loopBatchCoordinator) zeroShipHalt(batchStartCycle int) (rc int, halted bool) {
	t := &b.zeroShip
	if !t.armed {
		t.armed, t.floor = true, batchStartCycle
		return 0, false
	}
	to := b.clock()
	pol := policy.LoadOrDefaults(filepath.Join(b.cfg.EvolveDir, "policy.json"), b.stderr)
	stage, stageWarning := stageOfPolicy(pol)
	if stageWarning != "" {
		fmt.Fprintf(b.stderr, "[loop] WARN: zero-ship: %s\n", stageWarning)
	}
	read := wave.ReadWave(b.cfg.ProjectRoot, wave.WaveWindow{Floor: t.floor, From: t.from, To: to, Stage: stage})
	if len(read.ShipFailures) > 0 {
		for _, f := range read.ShipFailures {
			fmt.Fprintf(b.stderr, "[loop] WARN: zero-ship: wave not counted: cannot read the ship source %s: %v; next: fix the read, then run evolve wave status\n", f.Path, f.Err)
		}
		return 0, false
	}
	for _, f := range read.Failures {
		fmt.Fprintf(b.stderr, "[loop] WARN: zero-ship: cannot read %s: %v; the ship count of this wave does not depend on it\n", f.Path, f.Err)
	}
	t.from = to
	if len(read.Cycles) == 0 {
		return 0, false
	}
	ids := make([]int, 0, len(read.Cycles))
	for _, c := range read.Cycles {
		ids = append(ids, c.ID)
	}
	t.floor = max(t.floor, ids[len(ids)-1])
	t.waves = append(t.waves, zeroShipWave{cycles: ids, ships: len(read.Ships)})
	return b.evaluateZeroShip(pol)
}

func (b *loopBatchCoordinator) evaluateZeroShip(pol policy.Policy) (rc int, halted bool) {
	ceiling := policy.DefaultSystemFailurePolicy().Thresholds.ZeroShipHalt
	if fp, err := pol.FailurePolicyConfig(); err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: zero-ship breaker: failure policy unreadable (%v) — using the compiled default\n", err)
	} else {
		ceiling = fp.Thresholds.ZeroShipHalt
	}
	ships := make([]int, 0, len(b.zeroShip.waves))
	for _, w := range b.zeroShip.waves {
		ships = append(ships, w.ships)
	}
	v := core.EvaluateZeroShip(ships, ceiling)
	if !v.Halt {
		return 0, false
	}
	var named []string
	for _, w := range b.zeroShip.waves[len(b.zeroShip.waves)-v.Count:] {
		for _, id := range w.cycles {
			named = append(named, strconv.Itoa(id))
		}
	}
	lastWave := b.zeroShip.waves[len(b.zeroShip.waves)-1].cycles
	latest := lastWave[len(lastWave)-1]
	sf := &cyclestate.SystemFailureSignal{
		Category: "zero-ship",
		Level:    "system",
		Evidence: fmt.Sprintf("%s over cycles %s; next: evolve dossier show %d, then find the root cause before another wave (operating-policy.md section 4.1)", v.Reason, strings.Join(named, ","), latest),
		Halt:     true,
	}
	workspace := filepath.Join(b.cfg.EvolveDir, "runs", "cycle-"+strconv.Itoa(latest))
	rule := loopHaltRule{code: CodeLoopZeroShipHalt, fields: map[string]string{"rule": v.Rule, "waves": strconv.Itoa(v.Count)}}
	return haltOnSystemFailure(b.cfg.EvolveDir, b.cfg.ProjectRoot, latest, workspace, sf, b.stderr, b.deps.Signals, rule), true
}

func (b *loopBatchCoordinator) clock() time.Time {
	if b.now == nil {
		return time.Now()
	}
	return b.now()
}

func stageOfPolicy(pol policy.Policy) (string, string) {
	stage, err := pol.FleetIntegrationStage()
	if err != nil {
		return policy.FleetIntegrationShadow, fmt.Sprintf("%v: the ship count uses the shadow stage", err)
	}
	return stage, ""
}

func shipStageOf(evolveDir string) (string, string) {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.FleetIntegrationShadow, fmt.Sprintf("policy unreadable (%v): the ship count uses the shadow stage", err)
	}
	return stageOfPolicy(pol)
}
