package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const laneDeferralsRule = "lane-deferrals"

type LaneDeferralRecord struct {
	Cycle int
	Cause string
}

func CollectBatchLaneDeferrals(evolveDir string, fromCycle int) []LaneDeferralRecord {
	entries, err := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if err != nil {
		return nil
	}
	var out []LaneDeferralRecord
	for _, e := range entries {
		n, inBatch := batchCycleNumber(e, fromCycle)
		if !inBatch {
			continue
		}
		outcome, detail := cyclehealth.ClassifyOutcome(filepath.Join(evolveDir, "runs", e.Name()))
		if outcome == cyclehealth.OutcomeDeferred && strings.HasPrefix(detail, cyclestate.CycleTerminationLaneWorktreeDeferred) {
			cause := strings.TrimPrefix(detail, cyclestate.CycleTerminationLaneWorktreeDeferred+": ")
			out = append(out, LaneDeferralRecord{Cycle: n, Cause: cause})
		}
	}
	return out
}

func EvaluateLaneDeferrals(deferrals []LaneDeferralRecord, ceiling int) BlockerVerdict {
	if ceiling <= 0 {
		return BlockerVerdict{}
	}
	byCycle := make(map[int]LaneDeferralRecord, len(deferrals))
	cycles := make([]int, 0, len(deferrals))
	for _, d := range deferrals {
		byCycle[d.Cycle] = d
		cycles = append(cycles, d.Cycle)
	}
	sort.Ints(cycles)
	at, run, ok := consecutiveRun(cycles, ceiling, nil)
	if !ok {
		return BlockerVerdict{}
	}
	last := byCycle[cycles[at]]
	return BlockerVerdict{
		Halt: true, Rule: laneDeferralsRule, Count: run,
		Reason: fmt.Sprintf("%d consecutive fleet lanes deferred because their worktree could not be provisioned, ending at cycle %d (ceiling %d); last cause: %s — a fault outside the items (a stale ref lock in the shared store, the network, credentials) defers every lane until it is fixed",
			run, last.Cycle, ceiling, last.Cause),
	}
}

func LaneDeferredCycleSet(deferrals []LaneDeferralRecord) map[int]bool {
	set := make(map[int]bool, len(deferrals))
	for _, d := range deferrals {
		set[d.Cycle] = true
	}
	return set
}
