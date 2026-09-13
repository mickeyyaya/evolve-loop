package shiperr

// signalcodes.go — ADR-0101 S2: the ship vocabulary in the Signal Center's
// code space. Projection, not copy: SignalCode(c) is "SHIP_" + c, and every
// code is registered under the ship module with a one-line doc so
// docs/architecture/signal-codes.md and the ship.error events share ONE
// vocabulary. AllCodes is the sorted list the registration walks; the vocab
// test pins each constant's wire string, this table pins that each has a doc.

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
	CodeWorktreeResolve:             "atomic-ship: the cycle worktree could not be resolved",
	CodeIntegrityTreeDrift:          "post-ship: the shipped tree drifted from the verified one (integrity)",
	CodeArgs:                        "args: invalid ship arguments",
	CodeGitIO:                       "git I/O failure outside a named stage",
	CodeStateIO:                     "state file I/O failure",
	CodeUnknown:                     "an unclassified ship failure",
}

// AllCodes lists every ship error code, sorted — the one enumeration of the
// vocabulary (the constants cannot be walked at runtime).
func AllCodes() []ShipErrorCode {
	out := make([]ShipErrorCode, 0, len(codeDocs))
	for c := range codeDocs {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// SignalCode is the Signal Center code for a ship error: the literal SHIP_
// prefix, nothing clever, so a grep for either spelling finds the other.
func SignalCode(c ShipErrorCode) signalcenter.Code { return signalcenter.Code("SHIP_" + string(c)) }

func init() {
	for _, c := range AllCodes() {
		signalcenter.RegisterCode(signalcenter.ModuleShip, SignalCode(c), codeDocs[c])
	}
}

// SignalSeverity is the Signal Center severity of a ship error of this class
// (design §5.3): the integrity class is an INCIDENT — act; every other class,
// including one this switch does not name yet, is a WARN — note, continue,
// the router recovers. The rule lives with the vocabulary; the table test
// walks every declared class from source, so a new class gets a row.
func (c ShipErrorClass) SignalSeverity() signalcenter.Severity {
	switch c {
	case ShipClassIntegrity:
		return signalcenter.SeverityIncident
	case ShipClassTransient, ShipClassPrecondition, ShipClassConfig:
		return signalcenter.SeverityWarn
	}
	return signalcenter.SeverityWarn
}
