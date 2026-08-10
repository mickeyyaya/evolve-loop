package core

// failreasons_backfill.go — "no FAIL without a reason" (inbox
// null-failreasons-capture; 2026-08-06, three instances in one night). A
// cycle failing at phase-infra level (launch refusal, timeout, abort) sealed
// FinalVerdict FAIL with FailReasons null: retros fingerprinted a
// content-free identity (the bc2e3236 gate-block class in task-FAIL form)
// and the identical-fingerprint breaker could not see genuine recurrence.
//
// The seal backfills from the phase-timing record — orchestrator memory
// written at the recordPhaseOutcome chokepoint, never an agent-writable
// workspace file, so a failure identity cannot be forged from the workspace.

import "fmt"

// backfillFailReasons guarantees a sealed FAIL carries at least one reason.
// Already-explained FAILs (audit/ship gate reasons) and non-FAIL verdicts are
// untouched. Priority: recorded abort reasons (the real infra error, with its
// originating phase); else the failing phases by name; else one explicit
// unexplained marker — never null.
func backfillFailReasons(result *CycleResult, timings []phaseTimingEntry) {
	if result == nil || result.FinalVerdict != VerdictFAIL || len(result.FailReasons) > 0 {
		return
	}
	// An AbortReason is NOT proof the cycle died there: the ship-recovery path
	// records the transient it recovered FROM and continues (retry_opts.go),
	// so abort entries and failing-phase markers are BOTH emitted — an
	// abort-only backfill would fingerprint a recovered transient while
	// silently dropping the phase that actually failed (diff-review MEDIUM).
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
			result.FailReasons = append(result.FailReasons, fmt.Sprintf("phase %s: verdict FAIL with no recorded abort reason (phase-infra class)", t.Phase))
		}
	}
	if len(result.FailReasons) == 0 {
		result.FailReasons = []string{"unexplained cycle FAIL: no phase recorded an abort reason or FAIL verdict (infra death before/around dispatch)"}
	}
}
