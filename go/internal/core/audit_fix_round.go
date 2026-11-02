package core

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const (
	auditFixReasonPrefix = "audit-fix: "

	auditFixSourceAudit      = "audit"
	auditFixSourceBuildFloor = "build_floor"
)

func (cr *cycleRun) auditFixRound(branch Phase, reason string, sysFail *SystemFailureSignal) (Phase, string) {
	if sysFail != nil || strings.HasPrefix(reason, repairHandoffReasonPrefix) {
		return branch, reason
	}
	var fb *phasecontract.FailureBlock
	if repairSeededPhase(branch) {
		_, fb = cr.o.auditFailEnvelope(cr.cs, "")
	}
	res, ran := cr.runAuditFix(branch, fb, auditFixSourceAudit)
	if !ran {
		return branch, reason
	}
	next, why := auditFixBranch(res, branch, reason, cr.cs.ActiveWorktree)
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d: audit_fix %s; next %s\n", cr.cycle, res.Reason, next)
	return next, why
}

func (cr *cycleRun) runAuditFix(phase Phase, fb *phasecontract.FailureBlock, source string) (RungResult, bool) {
	in := RungInput{Rung: retryladder.RungAuditFix, Cycle: cr.cycle, Workspace: cr.cs.WorkspacePath, Worktree: cr.cs.ActiveWorktree}
	if fb != nil {
		var mark string
		mark, in.Choice = cr.recordAuditFix(phase, fb, source)
		in.Failure = *fb
		cr.cs.Ladder = withAuditFix(cr.cs.Ladder, mark)
	}
	h, registered := cr.o.rungHandler(retryladder.RungAuditFix)
	if !registered || cr.cs.Ladder == nil || cr.cs.Ladder.AuditFix == "" {
		return RungResult{}, false
	}
	res, err := h.Handle(cr.ctx, in)
	if err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: audit_fix handler (%s): %v; the flow of today runs\n", cr.cycle, source, err)
		return RungResult{}, false
	}
	return res, true
}

func auditFixBranch(res RungResult, branch Phase, reason, salvage string) (Phase, string) {
	switch {
	case res.Stop:
		return PhaseRetro, repairHandoffReasonPrefix + res.Reason + ": the raised repair round after the audit_fix escalation failed; salvage " + salvage
	case res.Handled && !res.Escalate:
		return PhaseAudit, auditFixReasonPrefix + res.Reason + ": the patch passed its review and the gates; the audit runs again on the patched tree"
	}
	return branch, reason
}
