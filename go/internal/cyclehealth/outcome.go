// outcome.go — R6: the cycle-outcome SLO classifier (concurrency-factory
// plan), the measurement instrument for the EVOLVE_PHASE_RECOVERY soak.
//
// "Every cycle delivers a result" is only auditable if every ending is
// classified. The inputs are the ADR-0044 C1 records every terminal path
// already writes — phase-timing.json (the append-merge dispatch log) and
// interaction-summary.json (the I1 rollup) — so the classifier needs no new
// instrumentation and works on any historical run dir.
package cyclehealth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Outcome is the cycle-ending taxonomy. Batch targets: SHIPPED ≥ 60%;
// SHIPPED+SALVAGED+FAILED_EXPLAINED = 100%; FAILED_UNEXPLAINED = 0, alarmed
// (it means a terminal path escaped the C1 chokepoint — file a defect).
type Outcome string

const (
	// OutcomeShipped: a ship dispatch recorded verdict PASS — the change
	// landed through the integrity floor.
	OutcomeShipped Outcome = "SHIPPED"
	// OutcomeSalvaged: no ship, but the correction ladder's salvage rung
	// produced the deliverable (typed recovery preserved the work).
	OutcomeSalvaged Outcome = "SALVAGED"
	// OutcomeFailedExplained: no ship, but some dispatch recorded an
	// abort_reason — the cycle failed loudly with a stated cause.
	OutcomeFailedExplained Outcome = "FAILED_EXPLAINED"
	// OutcomeFailedUnexplained: nothing above matched. The alarm bucket.
	OutcomeFailedUnexplained Outcome = "FAILED_UNEXPLAINED"
	// OutcomeDeferred: the dispatch seam detected all-CLI-families quota
	// exhaustion (every attempt exit=85, cycle-656), wrote a quota-likely
	// checkpoint, and aborted resumable. Not a failure of the change under
	// test — the work is preserved for `evolve loop --resume`.
	OutcomeDeferred Outcome = "DEFERRED"
)

// abortReasonDeferredPrefix mirrors core's abortReasonAllFamiliesExhausted via
// the C1 JSON record (the cross-package contract — cyclehealth must not import
// core).
const abortReasonDeferredPrefix = "all-families-exhausted"

// outcomeTimingEntry mirrors core.phaseTimingEntry's JSON (the C1 record);
// only the fields the classifier reads. Kept local: cyclehealth must not
// import core (it is a leaf the cmd layer composes).
type outcomeTimingEntry struct {
	Phase       string `json:"phase"`
	Verdict     string `json:"verdict"`
	AbortReason string `json:"abort_reason"`
}

// outcomeRollup mirrors interaction.Summary's counters (schema v1).
type outcomeRollup struct {
	ByRung   map[string]int `json:"by_rung"`
	ByResult map[string]int `json:"by_result"`
}

// ClassifyOutcome classifies one cycle workspace (a .evolve/runs/cycle-N
// dir). The detail string carries the evidence (the abort reason, the
// salvage counters) for the batch report. Read failures degrade toward
// FAILED_UNEXPLAINED — an unreadable record cannot explain anything.
func ClassifyOutcome(workspace string) (Outcome, string) {
	var timing []outcomeTimingEntry
	timingPresent := false
	if raw, err := os.ReadFile(filepath.Join(workspace, "phase-timing.json")); err == nil {
		timingPresent = true
		_ = json.Unmarshal(raw, &timing)
	}

	// SHIPPED: any ship dispatch with verdict PASS. Scanned over ALL
	// entries (the timing file is an append-merge log: a failed attempt
	// followed by a repaired PASS is reality — the PASS wins).
	for _, e := range timing {
		if e.Phase == "ship" && e.Verdict == "PASS" {
			return OutcomeShipped, "ship dispatch recorded verdict PASS"
		}
	}

	// SALVAGED: the I2 ladder's salvage rung ran AND an artifact appeared.
	// The v1 rollup has independent counters (no rung×result join), so this
	// is a conjunction heuristic — good enough for the soak instrument;
	// tighten when the rollup grows a joined counter.
	if raw, err := os.ReadFile(filepath.Join(workspace, "interaction-summary.json")); err == nil {
		var r outcomeRollup
		if json.Unmarshal(raw, &r) == nil &&
			r.ByRung["salvage"] > 0 && r.ByResult["artifact_appeared"] > 0 {
			return OutcomeSalvaged, fmt.Sprintf("correction-ladder salvage produced the artifact (salvage=%d, artifact_appeared=%d)",
				r.ByRung["salvage"], r.ByResult["artifact_appeared"])
		}
	}

	// DEFERRED: the abort reason carries the all-families quota-exhaustion
	// prefix (cycle-656) — checked BEFORE the generic explained-failure arm
	// so a quota defer is never paged as a failed cycle.
	for _, e := range timing {
		if strings.HasPrefix(e.AbortReason, abortReasonDeferredPrefix) {
			return OutcomeDeferred, e.AbortReason
		}
	}

	// FAILED_EXPLAINED: the C1 chokepoint recorded an abort reason.
	for _, e := range timing {
		if e.AbortReason != "" {
			return OutcomeFailedExplained, e.AbortReason
		}
	}

	// FAILED_EXPLAINED: a phase recorded verdict FAIL. The audit-FAIL →
	// retro → end terminal is a normal COMPLETION, not an abort — the C1
	// chokepoint never fires and no abort_reason exists — but the recorded
	// verdict is the explanation (cycle-306, soak #3: a legitimate audit
	// FAIL paged as UNEXPLAINED). Scanned in timing order so the detail
	// names the first failing phase, which is the causal one.
	for _, e := range timing {
		if e.Verdict == "FAIL" {
			return OutcomeFailedExplained, fmt.Sprintf("phase %s recorded verdict FAIL (no abort — cycle completed through its failure path)", e.Phase)
		}
	}

	if !timingPresent {
		return OutcomeFailedExplained, "cycle initialization failed before phase timing was recorded"
	}

	return OutcomeFailedUnexplained, "no ship PASS, no salvage, no recorded abort_reason — a terminal path escaped the C1 chokepoint"
}
