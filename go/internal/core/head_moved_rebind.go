package core

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

const CodeShipRecoveryPath signalcenter.Code = "ORCHESTRATOR_SHIP_RECOVERY_PATH"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeShipRecoveryPath, "a ship error that names a moved main (AUDIT_BINDING_HEAD_MOVED or GIT_FLEET_REBASE_NEEDED) took a recovery path; fields.path is rebind (the identity carry re-checked the merged tree and re-bound the audit, with no audit round), reaudit (an LLM audit round), rebuild, debugger, end, or the name of the next phase for any other route; fields.code is the routed code, fields.original_code the code ship raised and fields.reason why a moved HEAD was audited again")
}

const (
	recoveryPathRebind   = "rebind"
	recoveryPathReaudit  = "reaudit"
	recoveryPathRebuild  = "rebuild"
	recoveryPathDebugger = "debugger"
	recoveryPathEnd      = "end"
)

const carrierOverMovedMainSubject = "evolve: the audited change, carried over a moved main"

func (cr *cycleRun) carryAuditedChangeOverMovedMain(se *ShipError) (*ShipError, string) {
	if se.Code != CodeAuditBindingHeadMoved {
		return se, ""
	}
	if declined := cr.o.commitPendingAuditedChange(cr.ctx, cr.req.ProjectRoot, cr.cycle, cr.cs); declined != "" {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d main moved since the audit; auditing again: %s\n", cr.cycle, declined)
		return se, declined
	}
	return asFleetRebase(se, "main moved since the audit and the lane holds the audited change; rebase it and re-check the merged tree"), ""
}

func (o *Orchestrator) commitPendingAuditedChange(ctx context.Context, projectRoot string, cycle int, cs CycleState) string {
	worktree := cs.ActiveWorktree
	if worktree == "" || inPlaceWorktree(worktree, projectRoot) {
		return "the cycle has no worktree of its own"
	}
	audit, err := o.latestAuditEntry(ctx, cs.RunID)
	if err != nil || audit.WorktreeTreeSHA == "" {
		return fmt.Sprintf("no auditor row names the audited tree (err=%v)", err)
	}
	head, err := gitStdout(ctx, gitCapture, worktree, "rev-parse", "HEAD")
	if err != nil {
		return "read the worktree HEAD: " + err.Error()
	}
	if head != cs.WorktreeBaseSHA || audit.AuditedBase() != head {
		return fmt.Sprintf("the change is not pending on the audited base %s (HEAD %s, cycle base %s)", audit.AuditedBase(), head, cs.WorktreeBaseSHA)
	}
	held, err := treefence.Take(ctx, worktree)
	if err != nil {
		return "read the worktree tree: " + err.Error()
	}
	if held.Tree != audit.WorktreeTreeSHA {
		return fmt.Sprintf("the lane diff changed since the audit (tree %s, audited %s)", held.Tree, audit.WorktreeTreeSHA)
	}
	carrier, err := commitCarrier(ctx, gitCapture, worktree, AuditedChange{Base: head, Tree: held.Tree, Label: carrierLabel(cycle, cs.RunID)}, carrierOverMovedMainSubject)
	if err != nil {
		return "commit the audited change: " + err.Error()
	}
	if _, err := gitStdout(ctx, gitCapture, worktree, "reset", "-q", "--mixed", carrier); err != nil {
		return "move the lane to its carrier: " + err.Error()
	}
	return ""
}

func (cr *cycleRun) pendUnusedCarrier() {
	worktree, label := cr.cs.ActiveWorktree, carrierLabel(cr.cycle, cr.cs.RunID)
	carried, err := carrierAtHead(cr.ctx, worktree, label, gitCapture)
	if err == nil && carried {
		_, err = gitStdout(cr.ctx, gitCapture, worktree, "reset", "-q", "--soft", "HEAD^")
	}
	if err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle %d ship recovery ended; the carrier %s stays at the lane HEAD: %v\n", cr.cycle, label, err)
	}
}

type shipRecovery struct {
	raised, routed *ShipError
	next           Phase
	declined       string
}

func (cr *cycleRun) reportRecoveryPath(r shipRecovery) {
	if !namesMovedMain(r.routed.Code) {
		return
	}
	path := recoveryPathOf(r.next)
	fields := map[string]string{"path": path, "code": string(r.routed.Code), "original_code": string(r.raised.Code)}
	if r.declined != "" {
		fields["reason"] = r.declined
	}
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.cs.RunID, Phase: string(PhaseShip), Module: signalcenter.ModuleOrchestrator,
		Origin: "cycleRun.recoverShipError", Kind: signalcenter.KindPhaseOutcome, Severity: signalcenter.SeverityInfo,
		Code: CodeShipRecoveryPath, Reason: fmt.Sprintf("ship error %s recovers by %s", r.raised.Code, path), Fields: fields,
	})
}

func recoveryPathOf(next Phase) string {
	switch next {
	case "":
		return recoveryPathEnd
	case PhaseShip:
		return recoveryPathRebind
	case PhaseAudit:
		return recoveryPathReaudit
	case PhaseBuild:
		return recoveryPathRebuild
	case PhaseDebugger:
		return recoveryPathDebugger
	}
	return string(next)
}
