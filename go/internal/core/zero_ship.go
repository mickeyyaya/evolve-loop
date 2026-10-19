package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
)

const zeroShipRule = "zero-ship"

type CycleShip struct {
	Cycle   int
	Shipped bool
}

func CollectCycleShips(evolveDir string, fromCycle int) []CycleShip {
	entries, err := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if err != nil {
		return nil
	}
	var out []CycleShip
	for _, e := range entries {
		n, inRange := batchCycleNumber(e, fromCycle)
		if !inRange {
			continue
		}
		outcome, _ := cyclehealth.ClassifyOutcome(filepath.Join(evolveDir, "runs", e.Name()))
		out = append(out, CycleShip{Cycle: n, Shipped: outcome == cyclehealth.OutcomeShipped || outcome == cyclehealth.OutcomeSalvaged})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cycle < out[j].Cycle })
	return out
}

func CountShips(cycles []CycleShip) int {
	ships := 0
	for _, c := range cycles {
		if c.Shipped {
			ships++
		}
	}
	return ships
}

func EvaluateZeroShip(waveShips []int, ceiling int) BlockerVerdict {
	if ceiling <= 0 {
		return BlockerVerdict{}
	}
	run := 0
	for i := len(waveShips) - 1; i >= 0 && waveShips[i] == 0; i-- {
		run++
	}
	if run < ceiling {
		return BlockerVerdict{}
	}
	return BlockerVerdict{
		Halt: true, Rule: zeroShipRule, Count: run,
		Reason: fmt.Sprintf("%d consecutive waves landed no ship (ceiling %d) — failed and no-work lanes alike are zero-ship; a loop that cannot ship %d waves in a row is pipeline-degraded: find the root cause before another wave (operating-policy.md section 4.1)", run, ceiling, ceiling),
	}
}
