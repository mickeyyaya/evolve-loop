package cyclestate

// Verdict constants are the per-phase outcomes the EGPS gate matches verbatim; SKIPPED marks a phase that opted out.
const (
	VerdictPASS    = "PASS"
	VerdictFAIL    = "FAIL"
	VerdictWARN    = "WARN"
	VerdictSKIPPED = "SKIPPED"
)

// ClassificationMidExecutionFail is the supervisor's default class for a phase that failed mid-cycle with no self-report.
// It stays outside failurelog's taxonomy on purpose, so records carrying it age out on the one-day legacy TTL.
const ClassificationMidExecutionFail = "cycle-mid-execution-fail"

// CycleTerminationTriageNoWork marks a cycle whose Triage committed zero tasks and ended before any implementation phase.
const CycleTerminationTriageNoWork = "triage-empty-commitment"

// CycleOutcome constants are the cycle-level FinalVerdict labels, distinct from the per-phase verdicts.
// SHIPPED_VIA_BUILD needs this cycle's own ship PASS; main HEAD movement is never evidence, since sibling lanes move it.
const (
	CycleOutcomeShippedViaBuild      = "SHIPPED_VIA_BUILD"
	CycleOutcomeSkippedAuditAdvisory = "SKIPPED_AUDIT_ADVISORY"
	CycleOutcomeSkippedUnknown       = "SKIPPED_UNKNOWN"
)

// IsVerdict reports whether s is exactly one of the canonical verdict strings (case- and whitespace-sensitive).
func IsVerdict(s string) bool {
	switch s {
	case VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED:
		return true
	}
	return false
}
