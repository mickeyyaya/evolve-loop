package core

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const auditFixReasonPrefix = "audit-fix: "

func (cr *cycleRun) auditFixRound(branch Phase, reason string, sysFail *SystemFailureSignal) (Phase, string) {
	if sysFail != nil || strings.HasPrefix(reason, repairHandoffReasonPrefix) {
		return branch, reason
	}
	in := RungInput{Rung: retryladder.RungAuditFix, Cycle: cr.cycle, Workspace: cr.cs.WorkspacePath, Worktree: cr.cs.ActiveWorktree}
	if repairSeededPhase(branch) {
		_, fb := cr.o.auditFailEnvelope(cr.cs, "")
		var mark string
		mark, in.Choice = cr.recordAuditFix(branch, fb)
		in.Failure = *fb
		cr.cs.Ladder = withAuditFix(cr.cs.Ladder, mark)
	}
	h, registered := cr.o.rungHandler(retryladder.RungAuditFix)
	if !registered || cr.cs.Ladder == nil || cr.cs.Ladder.AuditFix == "" {
		return branch, reason
	}
	res, err := h.Handle(cr.ctx, in)
	if err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d: audit_fix handler: %v; the repair round of today runs\n", cr.cycle, err)
		return branch, reason
	}
	next, why := auditFixBranch(res, branch, reason, cr.cs.ActiveWorktree)
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d: audit_fix %s; next %s\n", cr.cycle, res.Reason, next)
	return next, why
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
