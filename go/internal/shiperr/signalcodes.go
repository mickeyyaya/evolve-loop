package shiperr

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var codeDocs = map[ShipErrorCode]string{
	CodeExplanationDocumentation:    "verify-explanation: the change ships without the explanation documentation its class requires",
	CodeSelfSHATampered:             "verify-self-sha: the ship binary's SHA does not match the pinned one (tamper or unpinned rebuild)",
	CodeSelfSHAIO:                   "verify-self-sha: the SHA pin could not be read or written",
	CodeAuditBindingHeadMoved:       "verify-class: main HEAD moved after the audit bound the tree",
	CodeAuditBindingTreeMismatch:    "verify-class: the audited tree differs from the tree being shipped",
	CodeAuditBindingArtifactSHA:     "verify-class: the audit artifact's SHA does not match the ledger binding",
	CodeAuditBindingArtifactMissing: "verify-class: the audit artifact the ledger binds is missing",
	CodeAuditBindingVerdictFail:     "verify-class: the bound audit verdict is FAIL",
	CodeAuditBindingVerdictWarn:     "verify-class: the bound audit verdict is WARN under strict audit",
	CodeAuditBindingMalformed:       "verify-class: the bound audit report carries no parseable verdict",
	CodeAuditBindingDualVerdict:     "verify-class: the bound audit report carries two contradicting verdicts",
	CodeAuditBindingStale:           "verify-class: the audit binding predates the tree being shipped",
	CodeAuditBindingNoAuditor:       "verify-class: no auditor ledger entry binds this cycle",
	CodeAuditBindingAuditorExit:     "verify-class: the auditor exited abnormally, so its verdict cannot bind",
	CodeAuditBindingNoLedger:        "verify-class: the ledger is unreadable, so no audit binding can be verified",
	CodeEGPSRedCount:                "verify-class: the EGPS gate reports red evals; red_count must be 0 to ship",
	CodeControlPlaneViolation:       "verify-class: a cycle-class commit touches the control-plane integrity surface (ADR-0064); ship it manually",
	CodeInvalidClass:                "verify-class: unknown ship class",
	CodeManualNotTTY:                "verify-class: a manual ship needs an interactive confirmation (or the auto-confirm environment)",
	CodeManualDeclined:              "verify-class: the operator declined the manual ship",
	CodeCommitGateMissing:           "verify-class: no commit-gate attestation exists for the staged tree",
	CodeCommitGateStale:             "verify-class: the commit-gate attestation is for a different tree",
	CodeCommitGateMalformed:         "verify-class: the commit-gate attestation cannot be parsed",
	CodeTrivialNotTrivial:           "verify-class: a trivial-class ship exceeds the trivial diff bound",
	CodeTrivialCriticalPaths:        "verify-class: a trivial-class ship touches critical paths",
	CodeGitDetachedHead:             "atomic-ship: the tree is on a detached HEAD",
	CodeGitStageFailed:              "atomic-ship: staging failed (transient)",
	CodeGitCommitFailed:             "atomic-ship: the commit step failed",
	CodeGitFFMergeDiverged:          "atomic-ship: the fast-forward merge to main diverged",
	CodeGitFleetRebaseNeeded:        "atomic-ship: a peer lane moved main; rebase and re-verify the merged tree (transient, ADR-0049 S5b)",
	CodeGitFleetRebaseConflict:      "atomic-ship: the fleet rebase hit a genuine merge conflict; routed to the debugger (integrity, ADR-0049 G13a)",
	CodeGitPushRejected:             "atomic-ship: the remote rejected the push",
	CodeCommitPrefixGate:            "atomic-ship: the commit-message prefix gate refused the message",
	CodeManifestGate:                "atomic-ship: a staged path was declared by no build/TDD report (cross-lane leak guard)",
	CodeRepoContractGate:            "atomic-ship: a repo-contract guard suite is RED in the lane worktree; pushing would red main",
	CodeRepoContractInfra:           "atomic-ship: the repo-contract scanner toolchain died twice without a test-level failure (re-dispatchable)",
	CodeCommittedDocDeleted:         "atomic-ship: the landing deletes a path under docs/ or knowledge-base/ that the target branch holds; a rename into docs/, or within knowledge-base/, ships",
	CodeForeignPredicatePackage:     "atomic-ship: a cycle landing adds a file under go/acs/cycle<M>/ of a cycle other than the shipping one; archive the superseded package with its explanation record",
	CodeWorktreeResolve:             "atomic-ship: the cycle worktree could not be resolved",
	CodeIntegrityTreeDrift:          "post-ship: the shipped tree drifted from the verified one (integrity)",
	CodeGitLaneNotOnOrigin:          "atomic-ship: nothing to commit, but the lane holds commits that origin does not hold, and no landing intent proves them; ship refuses a PASS with no push (integrity)",
	CodeGitLandingUnwindDeclined:    "atomic-ship: a landing that cannot complete also cannot unwind the lane to the audited shape, so the cycle stops; the journal holds the commit, and main does not move (integrity)",
	CodeGitPushPolicyRefused:        "atomic-ship: origin refused the push by policy (a protected branch, a hook, a rule, a token scope or an access refusal); ship does not retry it, unwinds the lane to the audited shape, and the router ends the cycle with no re-audit (precondition)",
	CodeArgs:                        "args: invalid ship arguments",
	CodeGitIO:                       "git I/O failure outside a named stage",
	CodeStateIO:                     "state file I/O failure",
	CodeUnknown:                     "an unclassified ship failure",
}

func AllCodes() []ShipErrorCode {
	out := make([]ShipErrorCode, 0, len(codeDocs))
	for c := range codeDocs {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func SignalCode(c ShipErrorCode) signalcenter.Code { return signalcenter.Code("SHIP_" + string(c)) }

var landingConflicts = map[signalcenter.Code]bool{
	SignalCode(CodeGitFleetRebaseNeeded):   true,
	SignalCode(CodeGitFleetRebaseConflict): true,
	SignalCode(CodeAuditBindingHeadMoved):  true,
}

func IsLandingConflict(c signalcenter.Code) bool { return landingConflicts[c] }

const (
	StepKey          = "step"
	GitRCKey         = "git_rc"
	WorktreeKey      = "worktree"
	BranchKey        = "branch"
	CycleBranchKey   = "cycle_branch"
	RepairOutcomeKey = "repair_outcome"
	OriginalCodeKey  = "original_code"
)

var SignalDebugKeys = []string{StepKey, GitRCKey, WorktreeKey, BranchKey, CycleBranchKey, RepairOutcomeKey, OriginalCodeKey}

func init() {
	for _, c := range AllCodes() {
		signalcenter.RegisterCode(signalcenter.ModuleShip, SignalCode(c), codeDocs[c])
	}
}

func (c ShipErrorClass) SignalSeverity() signalcenter.Severity {
	switch c {
	case ShipClassIntegrity:
		return signalcenter.SeverityIncident
	case ShipClassTransient, ShipClassPrecondition, ShipClassConfig:
		return signalcenter.SeverityWarn
	}
	return signalcenter.SeverityWarn
}
