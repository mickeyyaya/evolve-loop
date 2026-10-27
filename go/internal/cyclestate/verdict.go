package cyclestate

const (
	VerdictPASS    = "PASS"
	VerdictFAIL    = "FAIL"
	VerdictWARN    = "WARN"
	VerdictSKIPPED = "SKIPPED"
)

const SealedVerdictField = "final_verdict"

const SealedTerminationField = "termination_reason"

const ClassificationMidExecutionFail = "cycle-mid-execution-fail"

const CycleTerminationTriageNoWork = "triage-empty-commitment"

const CycleTerminationTddProtectedRoute = "tdd-protected-predicate-route"

const CycleTerminationBlockedNeedsConsole = "blocked-needs-console"

const CycleTerminationHandedOff = "handed-off"

const CycleTerminationTriageClaimFailed = "triage-empty-commitment-claimable-work"

const CycleTerminationLaneWorktreeDeferred = "lane-worktree-deferred"

const CycleTerminationLaneDeadline = "stopped: lane deadline"

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

const (
	RepairHandoffSignal = "REPAIR_HANDOFF"
	RepairDecisionField = "decision"
	RepairReasonField   = "reason"
	RepairBlockersField = "blockers"
	RepairDecisionNone  = "none"
	RepairDecisionWould = "would_stop"
	RepairDecisionStop  = "stop"
)
