package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// backfillFailReasons reads only orchestrator-authored phase-timing state,
// never an agent-writable workspace file, so a failure identity cannot be
// forged from the workspace.
func backfillFailReasons(result *CycleResult, timings []phaseTimingEntry) {
	if result == nil || result.FinalVerdict != VerdictFAIL || len(result.FailReasons) > 0 {
		return
	}
	named := map[string]bool{}
	for _, t := range timings {
		if t.AbortReason != "" {
			result.FailReasons = append(result.FailReasons, fmt.Sprintf("phase %s: %s", t.Phase, t.AbortReason))
			named[t.Phase] = true
		}
	}
	for _, t := range timings {
		if t.AbortReason == "" && t.Verdict == VerdictFAIL && !named[t.Phase] {
			named[t.Phase] = true
			if len(cyclestate.ErrorMessages(t.Diagnostics)) > 0 {
				result.FailReasons = append(result.FailReasons, fmt.Sprintf("phase %s: %s", t.Phase, verdictReason(VerdictFAIL, t.Diagnostics)))
				continue
			}
			result.FailReasons = append(result.FailReasons, fmt.Sprintf("phase %s: verdict FAIL with no recorded abort reason (phase-infra class)", t.Phase))
		}
	}
	if len(result.FailReasons) == 0 {
		result.FailReasons = []string{"unexplained cycle FAIL: no phase recorded an abort reason or FAIL verdict (infra death before/around dispatch)"}
	}
}
