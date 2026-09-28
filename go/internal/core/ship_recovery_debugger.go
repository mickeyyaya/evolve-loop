package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// CodeRebaseReentryAborted marks a debugger-resolved fleet rebase the host could not finish; the cycle ends there.
const CodeRebaseReentryAborted signalcenter.Code = "ORCHESTRATOR_REBASE_REENTRY_ABORTED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeRebaseReentryAborted, "after a debugger resolved a fleet-rebase conflict, the host could not carry, rebase, pend or route the resolved tree (the reason names which), or the recovery budget was spent; the cycle ended instead of a reship on the base the ship diverged from")
}

// resumeFleetRebaseAfterDebugger re-enters the fleet rebase once the debugger has resolved its conflict: the
// worktree still sits on the base the ship diverged from, so the debugger's target would only rediscover the
// divergence one phase later. Inside the ship-recovery budget and after the contention backoff, the resolved
// tree is carried as a commit, rebased onto main, pended again, and routed as any rebased change is (carry,
// rebind, or a Build that re-authors); a re-run the debugger asked for upstream of that route still happens, on
// the rebased tree. Every other recovery is left to the debugger's decision. A conflict that survives the
// resolution, a rebase the host cannot finish, or a spent budget ends the cycle, on stderr and in the Signal
// Center. The sequence has no checkpoint of its own; an interrupted re-entry leaves a carrier commit that keeps
// the change intact and is the resume heal's to finish.
func (o *Orchestrator) resumeFleetRebaseAfterDebugger(ctx context.Context, projectRoot string, cycle int, cs *CycleState, decided Phase, depth, fleetWidth int) (Phase, bool) {
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
	if err := carryResolvedTree(ctx, cs.ActiveWorktree, gitCapture, fmt.Sprintf("cycle-%d/%s", cycle, cs.RunID)); err != nil {
		return o.abortRebaseReentry(cycle, *cs, "carry the resolved tree: "+err.Error())
	}
	ok, conflict := rebaseRecordingConflicts(ctx, projectRoot, cs)
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

// fleetRebaseRecovery reports whether the recovery the debugger served began as a fleet rebase: the state keeps
// the ship's own code, and the conflict is the rebase's reclassification of it.
func fleetRebaseRecovery(code string) bool {
	return code == string(CodeGitFleetRebaseNeeded) || code == string(CodeGitFleetRebaseConflict)
}

func worktreeWritablePaths(next Phase, cs CycleState) []string {
	if next != PhaseDebugger || !fleetRebaseRecovery(cs.ShipRecoveryCode) {
		return nil
	}
	return cs.ShipRecoveryConflicts
}

// earliestPhase keeps the debugger's own target when it lies upstream of the routed phase, so a Build or Audit
// re-run it asked for still happens — on the rebased tree. A tests-first request cannot follow a rebase (the
// explanation contract refreshes after TDD against a binding the route has just moved), so it yields to the route.
func earliestPhase(decided, routed Phase) Phase {
	order := map[Phase]int{PhaseBuild: 0, PhaseAudit: 1, PhaseShip: 2}
	d, dok := order[decided]
	r, rok := order[routed]
	if dok && rok && d < r {
		return decided
	}
	return routed
}

// carryResolvedTree commits the worktree's tracked state — the debugger's resolution included — on the fork
// point, so the fleet rebase has a commit to replay; untracked files never ride (a resolution edits tracked
// paths, and the ship stages by manifest). The branch's earlier commits stay reachable from the reflog only;
// every downstream check recomputes the fork point and the tree instead of naming them.
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
	carrier, err := gitStdout(ctx, git, worktree, "-c", "commit.gpgsign=false", "commit-tree", tree, "-p", base,
		"-m", "evolve: the change with the debugger's conflict resolution", "-m", "Evolve-Carrier: "+label)
	if err != nil {
		return err
	}
	_, err = gitStdout(ctx, git, worktree, "reset", "--soft", carrier)
	return err
}
