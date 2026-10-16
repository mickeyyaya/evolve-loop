package cyclehealth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

type Outcome string

const (
	OutcomeShipped           Outcome = "SHIPPED"
	OutcomeSalvaged          Outcome = "SALVAGED"
	OutcomeFailedExplained   Outcome = "FAILED_EXPLAINED"
	OutcomeFailedUnexplained Outcome = "FAILED_UNEXPLAINED"
	OutcomeDeferred          Outcome = "DEFERRED"
	OutcomeNoWork            Outcome = "NO_WORK"
)

const abortReasonDeferredPrefix = "all-families-exhausted"

type outcomeRollup struct {
	ByRung   map[string]int `json:"by_rung"`
	ByResult map[string]int `json:"by_result"`
}

func ClassifyOutcome(workspace string) (Outcome, string) {
	timing, timingPresent := readTiming(workspace)

	for _, e := range timing {
		if e.Phase == "ship" && e.Verdict == "PASS" {
			return OutcomeShipped, "ship dispatch recorded verdict PASS"
		}
	}

	if detail, ok := salvageDetail(workspace); ok {
		return OutcomeSalvaged, detail
	}

	for _, e := range timing {
		if strings.HasPrefix(e.AbortReason, abortReasonDeferredPrefix) ||
			strings.HasPrefix(e.AbortReason, cyclestate.CycleTerminationLaneWorktreeDeferred) {
			return OutcomeDeferred, e.AbortReason
		}
	}

	for _, e := range timing {
		if e.AbortReason == cyclestate.CycleTerminationTriageNoWork {
			return OutcomeNoWork, "planned no-work: " + e.AbortReason
		}
	}

	for _, e := range timing {
		if e.AbortReason != "" {
			return OutcomeFailedExplained, e.AbortReason
		}
	}

	if detail, ok := failVerdictDetail(timing); ok {
		return OutcomeFailedExplained, detail
	}

	if !timingPresent {
		return OutcomeFailedExplained, "cycle initialization failed before phase timing was recorded"
	}

	return OutcomeFailedUnexplained, "no ship PASS, no salvage, no recorded abort_reason — a terminal path escaped the C1 chokepoint"
}

func readTiming(workspace string) ([]phasetiming.Entry, bool) {
	entries, err := phasetiming.Read(workspace)
	if err == nil {
		return entries, true
	}
	return nil, !os.IsNotExist(err)
}

func salvageDetail(workspace string) (string, bool) {
	if raw, err := os.ReadFile(filepath.Join(workspace, "interaction-summary.json")); err == nil {
		var r outcomeRollup
		if json.Unmarshal(raw, &r) == nil &&
			r.ByRung["salvage"] > 0 && r.ByResult["artifact_appeared"] > 0 {
			return fmt.Sprintf("correction-ladder salvage produced the artifact (salvage=%d, artifact_appeared=%d)",
				r.ByRung["salvage"], r.ByResult["artifact_appeared"]), true
		}
	}
	return "", false
}

func failVerdictDetail(timing []phasetiming.Entry) (string, bool) {
	for _, e := range timing {
		if e.Verdict == "FAIL" {
			if msgs := cyclestate.ErrorMessages(e.Diagnostics); len(msgs) > 0 {
				return fmt.Sprintf("phase %s recorded verdict FAIL: %s (no abort — cycle completed through its failure path)", e.Phase, strings.Join(msgs, "; ")), true
			}
			return fmt.Sprintf("phase %s recorded verdict FAIL (no abort — cycle completed through its failure path)", e.Phase), true
		}
	}
	return "", false
}
