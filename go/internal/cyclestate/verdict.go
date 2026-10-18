package cyclestate

const (
	VerdictPASS    = "PASS"
	VerdictFAIL    = "FAIL"
	VerdictWARN    = "WARN"
	VerdictSKIPPED = "SKIPPED"
)

const ClassificationMidExecutionFail = "cycle-mid-execution-fail"

const CycleTerminationTriageNoWork = "triage-empty-commitment"

const CycleTerminationTddProtectedRoute = "tdd-protected-predicate-route"

const CycleTerminationTriageClaimFailed = "triage-empty-commitment-claimable-work"

const CycleTerminationLaneWorktreeDeferred = "lane-worktree-deferred"

const (
	CycleOutcomeShippedViaBuild      = "SHIPPED_VIA_BUILD"
	CycleOutcomeSkippedAuditAdvisory = "SKIPPED_AUDIT_ADVISORY"
	CycleOutcomeSkippedUnknown       = "SKIPPED_UNKNOWN"
)

func IsVerdict(s string) bool {
	switch s {
	case VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED:
		return true
	}
	return false
}
