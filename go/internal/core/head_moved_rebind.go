package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

const CodeShipRecoveryPath signalcenter.Code = "ORCHESTRATOR_SHIP_RECOVERY_PATH"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeShipRecoveryPath, "a ship error naming a moved main (AUDIT_BINDING_HEAD_MOVED or GIT_FLEET_REBASE_NEEDED) took a recovery path. fields.path is rebind (identity carry re-bound audit), reaudit, rebuild, debugger, end, or next phase. Fields: fields.code (routed), fields.original_code (raised), fields.reason (why moved HEAD re-audited)")
}

const (
	FieldRecoveryPath   = "path"
	RecoveryPathReaudit = "reaudit"
)

const (
	recoveryPathRebind   = "rebind"
	recoveryPathRebuild  = "rebuild"
	recoveryPathDebugger = "debugger"
	recoveryPathEnd      = "end"
)

const carrierOverMovedMainSubject = "evolve: the audited change, carried over a moved main"

func (cr *cycleRun) carryAuditedChangeOverMovedMain(se *ShipError) (*ShipError, string) {
	if !NamesMovedMain(se.Code) {
		return se, ""
	}
	fleetRebase := se.Code == CodeGitFleetRebaseNeeded
	if fleetRebase {
		if _, pending, _ := cr.o.pendingAuditedChange(cr.ctx, cr.req.ProjectRoot, cr.cs); !pending {
			return se, ""
		}
	}
	declined := cr.o.commitPendingAuditedChange(cr.ctx, cr.req.ProjectRoot, cr.cycle, cr.cs)
	switch {
	case declined == "":
		return asFleetRebase(se, "main moved since the audit and the lane holds the audited change; rebase it and re-check the merged tree"), ""
	case fleetRebase:
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d main moved before the lane landed; auditing again: %s\n", cr.cycle, declined)
		return asMovedHead(se), declined
	}
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] cycle %d main moved since the audit; auditing again: %s\n", cr.cycle, declined)
	return se, declined
}

func (o *Orchestrator) pendingAuditedChange(ctx context.Context, projectRoot string, cs CycleState) (AuditedChange, bool, string) {
	worktree := cs.ActiveWorktree
	if worktree == "" || inPlaceWorktree(worktree, projectRoot) {
		return AuditedChange{}, false, "the cycle has no worktree of its own"
	}
	revs, err := gitStdout(ctx, gitCapture, worktree, "rev-parse", "HEAD", "HEAD^{tree}")
	if err != nil {
		return AuditedChange{}, false, "read the worktree HEAD: " + err.Error()
	}
	head, baseTree, _ := strings.Cut(revs, "\n")
	if head != cs.WorktreeBaseSHA {
		return AuditedChange{}, false, fmt.Sprintf("the change is not pending on the audited base (HEAD %s, cycle base %s)", head, cs.WorktreeBaseSHA)
	}
	audit, err := o.latestAuditEntry(ctx, cs.RunID)
	if err != nil || audit.WorktreeTreeSHA == "" {
		status, statusErr := gitStdout(ctx, gitCapture, worktree, "status", "--porcelain")
		return AuditedChange{}, statusErr == nil && status != "", fmt.Sprintf("no auditor row names the audited tree (err=%v)", err)
	}
	if audit.AuditedBase() != head {
		return AuditedChange{}, true, fmt.Sprintf("the change is not pending on the audited base %s (HEAD %s, cycle base %s)", audit.AuditedBase(), head, cs.WorktreeBaseSHA)
	}
	if baseTree == audit.WorktreeTreeSHA {
		return AuditedChange{}, false, "the audit judged no change on the lane base"
	}
	return AuditedChange{Base: head, Tree: audit.WorktreeTreeSHA}, true, ""
}

func (o *Orchestrator) commitPendingAuditedChange(ctx context.Context, projectRoot string, cycle int, cs CycleState) string {
	pending, _, declined := o.pendingAuditedChange(ctx, projectRoot, cs)
	if declined != "" {
		return declined
	}
	worktree := cs.ActiveWorktree
	held, err := treefence.Take(ctx, worktree)
	if err != nil {
		return "read the worktree tree: " + err.Error()
	}
	if held.Tree != pending.Tree {
		return fmt.Sprintf("the lane diff changed since the audit (tree %s, audited %s)", held.Tree, pending.Tree)
	}
	pending.Label = carrierLabel(cycle, cs.RunID)
	carrier, err := commitCarrier(ctx, gitCapture, worktree, pending, carrierOverMovedMainSubject)
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
	if !NamesMovedMain(r.routed.Code) {
		return
	}
	path := recoveryPathOf(r.next)
	fields := map[string]string{FieldRecoveryPath: path, "code": string(r.routed.Code), "original_code": string(r.raised.Code)}
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
		return RecoveryPathReaudit
	case PhaseBuild:
		return recoveryPathRebuild
	case PhaseDebugger:
		return recoveryPathDebugger
	}
	return string(next)
}
