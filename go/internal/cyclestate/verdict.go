package cyclestate

// Verdict constants — the four outcomes a phase may emit. These match
// the EGPS gate vocabulary (CLAUDE.md env-var table: WARN removed at
// v10.0.0 but still accepted by Audit for the pre-EGPS soft-start
// boundary; SKIPPED used when a phase opted out, e.g. EVOLVE_TRIAGE_DISABLE).
const (
	VerdictPASS    = "PASS"
	VerdictFAIL    = "FAIL"
	VerdictWARN    = "WARN"
	VerdictSKIPPED = "SKIPPED"
)

// ClassificationMidExecutionFail is the supervisor's default class for a phase
// that failed mid-cycle with no self-report. Deliberately OUTSIDE failurelog's
// taxonomy: NormalizeLegacy maps it to UnknownClassification, so a record
// carrying it ages out on the one-day legacy bucket (failurelog.LegacyEffectiveTTL)
// — an operator decision (ADR-0103 unit 03b, F11). Projected by the
// failure-learning engine (the FailedRecord and the lesson event) and by
// recurrence's generic-pattern denylist; every other spelling is data.
const ClassificationMidExecutionFail = "cycle-mid-execution-fail"

// CycleTerminationTriageNoWork identifies a successful Triage transition that
// explicitly committed zero tasks and ended before any implementation phase.
const CycleTerminationTriageNoWork = "triage-empty-commitment"

// CycleOutcome constants — cycle-level FinalVerdict labels emitted by
// finalizeOutcome. Distinct from the per-phase Verdict* set because a
// cycle outcome covers multiple phases plus the cycle's own ship latch.
// They disambiguate the bare "SKIPPED" verdict that previously conflated
// a shipped cycle, a fluent-mode advisory, and a no-signal noop.
// SHIPPED_VIA_BUILD is emitted only when THIS cycle's ship phase PASSed;
// main HEAD movement is never evidence (a sibling lane moves it too).
const (
	CycleOutcomeShippedViaBuild      = "SHIPPED_VIA_BUILD"
	CycleOutcomeSkippedAuditAdvisory = "SKIPPED_AUDIT_ADVISORY"
	CycleOutcomeSkippedUnknown       = "SKIPPED_UNKNOWN"
)

// IsVerdict reports whether s is one of the canonical verdict strings.
// Case- and whitespace-sensitive — guards against silent typos.
func IsVerdict(s string) bool {
	switch s {
	case VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED:
		return true
	}
	return false
}
