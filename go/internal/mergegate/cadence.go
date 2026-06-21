package mergegate

import "fmt"

// Cadence is the advisor's scaling decision — how often the merge-to-main gate
// promotes accumulated milestone work.
type Cadence string

const (
	// CadenceDefer holds promotion: either a safety floor is unmet, or the batch
	// is still accumulating toward its threshold.
	CadenceDefer Cadence = "defer"
	// CadencePerWave promotes after every completed wave (smallest batch — the
	// trunk-based / DORA optimum; the default when BatchWaveCount <= 1).
	CadencePerWave Cadence = "per-wave"
	// CadenceBatched promotes after a batch of waves accumulates (or a churn
	// ceiling / anti-starvation bound is hit).
	CadenceBatched Cadence = "batched"
	// CadenceFeatureComplete promotes the final milestone once every campaign
	// wave is complete.
	CadenceFeatureComplete Cadence = "feature-complete"
)

// DecisionInput is the objective progress + quality snapshot the advisor reads.
// Every field is measured by the caller (campaign progress + handoff signals);
// the advisor itself does no I/O. Safety predicates are pre-reduced to
// booleans/counts so this package stays a stdlib-only leaf (SeverityBlocks is the
// caller's router.Severity >= policy threshold comparison — no duplication of
// router.ParseSeverity here).
type DecisionInput struct {
	WavesDone       int  // campaign waves fully shipped so far
	WavesTotal      int  // total waves in the campaign plan (0 ⇒ unknown/non-campaign)
	PendingWaves    int  // completed-but-not-yet-promoted waves accumulated this batch
	ChurnLOC        int  // accumulated changed LOC since the last promotion
	CarryoverAgeMax int  // oldest unpicked P0/P1 carryover age, in cycles (anti-starvation)
	AuditPassed     bool // audit verdict == PASS on the integrated tree
	CIGreen         bool // main/integration CI is green
	LedgerVerified  bool // evolve ledger verify passed (hash chain intact)
	SeverityBlocks  bool // build max severity >= policy block threshold
	Conflicts       int  // unresolved merge-train conflicts
}

// Thresholds are the resolved cadence-scaling knobs (projected from
// policy.MergeGateConfig at the composition root).
type Thresholds struct {
	BatchWaveCount       int // waves to accumulate before firing (<=1 ⇒ per-wave)
	BatchChurnLOC        int // churn ceiling above which a batch fires early
	CarryoverStallCycles int // anti-starvation bound (0 ⇒ disabled)
}

// Decision is the advisor's output: whether to fire the gate at this boundary,
// the chosen cadence, and a human-readable reason (always populated, for the
// ledger/dossier record).
type Decision struct {
	Fire    bool
	Cadence Cadence
	Reason  string
}

// DecideCadence is the merge-cadence advisor — pure and deterministic. Precedence
// (defer-wins): a safety violation always defers; otherwise feature-complete >
// anti-starvation flush > per-wave > batch-threshold > keep accumulating.
func DecideCadence(in DecisionInput, th Thresholds) Decision {
	if d, blocked := safetyDefer(in); blocked {
		return d
	}
	// All waves complete ⇒ promote the final milestone, even past a high batch.
	if in.WavesTotal > 0 && in.WavesDone >= in.WavesTotal {
		return Decision{Fire: true, Cadence: CadenceFeatureComplete, Reason: "all waves complete"}
	}
	// Anti-starvation: accumulated work has waited too long ⇒ flush a batch.
	if th.CarryoverStallCycles > 0 && in.CarryoverAgeMax >= th.CarryoverStallCycles {
		return Decision{Fire: true, Cadence: CadenceBatched,
			Reason: fmt.Sprintf("carryover stall (%d >= %d cycles) forces a promotion", in.CarryoverAgeMax, th.CarryoverStallCycles)}
	}
	// Smallest batch: promote every wave.
	if th.BatchWaveCount <= 1 {
		return Decision{Fire: true, Cadence: CadencePerWave, Reason: "per-wave cadence"}
	}
	// Batch: fire once enough waves accumulate, or churn exceeds the ceiling
	// (small-batch guard — never let a batch grow unboundedly large).
	if in.PendingWaves >= th.BatchWaveCount {
		return Decision{Fire: true, Cadence: CadenceBatched,
			Reason: fmt.Sprintf("batch of %d waves reached", th.BatchWaveCount)}
	}
	if th.BatchChurnLOC > 0 && in.ChurnLOC > th.BatchChurnLOC {
		return Decision{Fire: true, Cadence: CadenceBatched,
			Reason: fmt.Sprintf("batch churn ceiling exceeded (%d > %d LOC)", in.ChurnLOC, th.BatchChurnLOC)}
	}
	return Decision{Fire: false, Cadence: CadenceDefer,
		Reason: fmt.Sprintf("accumulating batch (%d/%d waves)", in.PendingWaves, th.BatchWaveCount)}
}

// safetyDefer returns a defer Decision (and blocked=true) when any safety floor
// predicate is unmet. The order is fixed so the reason is deterministic. This is
// the kernel floor the LLM gate can only ever tighten, never loosen.
func safetyDefer(in DecisionInput) (Decision, bool) {
	switch {
	case !in.AuditPassed:
		return deferFor("audit did not pass"), true
	case !in.CIGreen:
		return deferFor("CI is not green"), true
	case !in.LedgerVerified:
		return deferFor("ledger verification failed"), true
	case in.Conflicts > 0:
		return deferFor(fmt.Sprintf("%d unresolved merge conflict(s)", in.Conflicts)), true
	case in.SeverityBlocks:
		return deferFor("build severity at/above the block threshold"), true
	}
	return Decision{}, false
}

func deferFor(reason string) Decision {
	return Decision{Fire: false, Cadence: CadenceDefer, Reason: reason}
}
