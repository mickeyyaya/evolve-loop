// Package failuregrade is the ADR-0048 Slice A graduated-enforcement classifier.
//
// A gate/cycle failure is not binary (proceed | abort). It is graded into a
// TIER, so a tiny, provably-recoverable defect is corrected, quarantined, or
// repaired instead of costing a whole cycle (the cycles 243/246/247 killers): a
// missing challenge token in a report (a 60-second re-emit), benign config-file
// tree-churn at the tree-diff guard, or a SELF_SHA mismatch after a verified
// reproducible rebuild.
//
// The vocabulary is CLOSED (ADR-0047 Specification pattern): only three named,
// evidence-anchored failure classes grade below Abort, and a class missing its
// evidence predicate still Aborts. So the failure floor (ADR-0039) can never
// silently downgrade a real failure — an unknown reason, or a tree-churn that is
// NOT benign, or a SELF_SHA mismatch that is NOT a verified rebuild (i.e. real
// tampering), all still abort the cycle.
//
// The matched signatures are OUR OWN control reason-codes, never agent content
// — substring matching here is safe precisely because the input is the
// orchestrator's abort reason, not a captured pane (contrast ADR-0047).
package failuregrade

import "strings"

// Tier is the enforcement response to a graded failure.
type Tier int

const (
	// TierAbort is the floor: whole-cycle abort (the default for everything
	// not positively graded down).
	TierAbort Tier = iota
	// TierCorrect re-dispatches the phase with a correction directive (the
	// ADR-0045 mechanism) instead of failing — e.g. "re-emit the report with
	// challenge token <T>".
	TierCorrect
	// TierQuarantine isolates the offending change and warns, without aborting
	// — e.g. benign config-file churn that matches a revert/known-benign class.
	TierQuarantine
	// TierRepair routes to the typed ship repair ladder (e.g. the TOFU SELF_SHA
	// re-pin) instead of aborting.
	TierRepair
)

// String returns the lower-case tier name ("abort", "correct", "quarantine",
// "repair"); an out-of-range Tier renders as "abort" (the fail-closed floor).
func (t Tier) String() string {
	switch t {
	case TierCorrect:
		return "correct"
	case TierQuarantine:
		return "quarantine"
	case TierRepair:
		return "repair"
	default:
		return "abort"
	}
}

// Evidence carries the predicates that gate the non-Abort tiers. Zero-value
// (all false) is the safe default: a class whose predicate is unmet stays at
// Abort (fail-closed).
type Evidence struct {
	// ChurnIsBenign reports that tree-diff-guard churn content matched a revert
	// or a known-benign signature — required to quarantine rather than abort.
	ChurnIsBenign bool
	// RebuildVerified reports that the ship binary was a verified reproducible
	// rebuild (2× byte-identical) — required to route a SELF_SHA mismatch to
	// repair rather than abort it as real tampering.
	RebuildVerified bool
}

// Recognized abort-reason signatures (closed control vocabulary).
const (
	sigMissingChallengeToken = "missing_challenge_token" // deliverable.CodeMissingChallengeToken
	sigSelfSHATampered       = "SELF_SHA_TAMPERED"       // core.CodeSelfSHATampered
	sigTreeDiffGuard         = "tree-diff guard"         // orchestrator leak-guard abort prefix
)

// Grade maps an abort reason plus evidence to its enforcement tier. Reasons not
// in the closed vocabulary — and graded classes lacking their evidence
// predicate — return TierAbort.
func Grade(reason string, ev Evidence) Tier {
	switch {
	case strings.Contains(reason, sigMissingChallengeToken):
		// Always recoverable: the token is known; the agent re-emits the report.
		return TierCorrect
	case strings.Contains(reason, sigTreeDiffGuard):
		if ev.ChurnIsBenign {
			return TierQuarantine
		}
		return TierAbort
	case strings.Contains(reason, sigSelfSHATampered):
		if ev.RebuildVerified {
			return TierRepair
		}
		return TierAbort
	default:
		return TierAbort
	}
}
