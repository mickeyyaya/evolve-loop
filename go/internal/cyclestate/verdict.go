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

// CycleOutcome constants — cycle-level FinalVerdict labels emitted by
// finalizeOutcome. Distinct from the per-phase Verdict* set because a
// cycle outcome covers multiple phases plus commit-movement signal.
// They disambiguate the bare "SKIPPED" verdict that previously conflated
// an inline build-ship, a fluent-mode advisory, and a no-signal noop.
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
