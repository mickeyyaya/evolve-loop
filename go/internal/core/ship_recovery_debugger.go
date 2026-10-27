package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeRebaseReentryAborted signalcenter.Code = "ORCHESTRATOR_REBASE_REENTRY_ABORTED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeRebaseReentryAborted, "after a debugger resolved a fleet-rebase conflict, the host could not carry, rebase, pend or route the resolved tree (the reason names which), or the recovery budget was spent; the cycle ended instead of a reship on the base the ship diverged from")
}

func (o *Orchestrator) resumeFleetRebaseAfterDebugger(ctx context.Context, projectRoot string, cycle int, cs *CycleState, decided Phase, depth, fleetWidth int) (Phase, bool) {
	return o.keepShipWindowOnlyForShip(o.resumeFleetRebaseRoute(ctx, projectRoot, cycle, cs, decided, depth, fleetWidth))
}

func (o *Orchestrator) resumeFleetRebaseRoute(ctx context.Context, projectRoot string, cycle int, cs *CycleState, decided Phase, depth, fleetWidth int) (Phase, bool) {
	if !fleetRebaseRecovery(cs.ShipRecoveryCode) || cs.ActiveWorktree == "" || inPlaceWorktree(cs.ActiveWorktree, projectRoot) {
		return "", false
	}
	if budget := shipRecoveryBudget(CodeGitFleetRebaseNeeded, fleetWidth); depth >= budget {
		return o.abortRebaseReentry(cycle, *cs, fmt.Sprintf("ship recovery exhausted after %d attempt(s) (budget %d, fleet width %d)", depth, budget, fleetWidth))
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d debugger resolved the fleet-rebase conflict; re-entering the fleet rebase on the resolved tree (recovery attempt %d) instead of a %s on the base the ship diverged from\n", cycle, depth+1, decided)
	if decided == PhaseTDD {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d the debugger's tests-first re-run is superseded: after a rebase the explanation contract admits Build, Audit or Ship, and Build re-authors on the rebased tree\n", cycle)
	}
	backoffSleep(contentionBackoff(depth))
	o.holdShipWindow(ctx, projectRoot, cycle, cs.RunID)
	if err := carryResolvedTree(ctx, cs.ActiveWorktree, gitCapture, carrierLabel(cycle, cs.RunID)); err != nil {
		return o.abortRebaseReentry(cycle, *cs, "carry the resolved tree: "+err.Error())
	}
	ok, conflict := reauditRebase(ctx, projectRoot, cs)
	switch {
	case ok:
	case conflict:
		return o.abortRebaseReentry(cycle, *cs, "the conflict survived the debugger's resolution")
	default:
		return o.abortRebaseReentry(cycle, *cs, "the fleet rebase failed (infra)")
	}
	if err := pendRebasedChange(ctx, cs.ActiveWorktree, gitCapture); err != nil {
		return o.abortRebaseReentry(cycle, *cs, "pend the resolved change: "+err.Error())
	}
	routed := PhaseAudit
	if cs.ExplanationDocumentationVersion > 0 {
		next, ok := o.routeRebasedExplanation(ctx, projectRoot, cycle, cs)
		if !ok {
			return o.abortRebaseReentry(cycle, *cs, "the rebased explanation could not be routed")
		}
		routed = next
	}
	return earliestPhase(decided, routed), true
}

func (o *Orchestrator) abortRebaseReentry(cycle int, cs CycleState, reason string) (Phase, bool) {
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet rebase re-entry aborted: %s; the cycle ends\n", cycle, reason)
	o.signals.Emit(signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Phase: string(PhaseDebugger),
		Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.resumeFleetRebaseAfterDebugger", Kind: signalcenter.KindPhaseAborted,
		Severity: signalcenter.SeverityWarn, Code: CodeRebaseReentryAborted, Reason: reason,
	})
	return PhaseEnd, true
}

func fleetRebaseRecovery(code string) bool {
	return code == string(CodeGitFleetRebaseNeeded) || code == string(CodeGitFleetRebaseConflict)
}

func worktreeWritablePaths(next Phase, cs CycleState) []string {
	if next != PhaseDebugger || !fleetRebaseRecovery(cs.ShipRecoveryCode) {
		return nil
	}
	return cs.ShipRecoveryConflicts
}

func earliestPhase(decided, routed Phase) Phase {
	order := map[Phase]int{PhaseBuild: 0, PhaseAudit: 1, PhaseShip: 2}
	d, dok := order[decided]
	r, rok := order[routed]
	if dok && rok && d < r {
		return decided
	}
	return routed
}

func carryResolvedTree(ctx context.Context, worktree string, git gitFn, label string) error {
	if _, err := gitStdout(ctx, git, worktree, "add", "-u"); err != nil {
		return err
	}
	if untracked, err := gitStdout(ctx, git, worktree, "ls-files", "--others", "--exclude-standard"); err == nil && untracked != "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN the carrier leaves untracked file(s) behind (a resolution edits tracked paths): %s\n", strings.Join(strings.Fields(untracked), ", "))
	}
	tree, err := gitStdout(ctx, git, worktree, "write-tree")
	if err != nil {
		return err
	}
	base, err := forkPoint(ctx, git, worktree)
	if err != nil {
		return err
	}
	carrier, err := commitCarrier(ctx, git, worktree, AuditedChange{Base: base, Tree: tree, Label: label}, "evolve: the change with the debugger's conflict resolution")
	if err != nil {
		return err
	}
	_, err = gitStdout(ctx, git, worktree, "reset", "--soft", carrier)
	return err
}
