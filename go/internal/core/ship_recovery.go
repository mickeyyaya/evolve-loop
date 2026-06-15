package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) recoverFromShipError(ctx context.Context, cycle int, cs CycleState, se *ShipError, depth int) (Phase, bool) {
	o.recordShipError(ctx, cycle, cs, se)
	if depth >= maxRecoveryDepth {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery exhausted after %d attempt(s) (%s/%s); aborting\n", depth, se.Code, se.Class)
		return "", false
	}
	// ADR-0049 S5b: a fleet ff-merge divergence (a peer cycle moved main) is
	// recovered by rebasing the cycle branch onto the new main BEFORE the
	// re-audit (the router routes this code to audit). A clean rebase replays
	// this cycle's patches onto the peer's changes → re-audit re-binds the merged
	// tree → re-ship fast-forwards. A rebase CONFLICT is genuinely overlapping
	// work the advisor's partition should have kept apart — abort loud rather
	// than auto-resolve.
	// The code/class used for the routing decision; a fleet rebase may reclassify
	// a clean RebaseNeeded into a CONFLICT (G13a) that routes to the debugger.
	recoverCode, recoverClass := se.Code, se.Class
	if se.Code == CodeGitFleetRebaseNeeded {
		ok, conflict := rebaseCycleBranchOntoMain(ctx, cs.ActiveWorktree)
		switch {
		case ok:
			// Clean replay → router routes RebaseNeeded to audit (re-bind merged tree).
		case conflict:
			// Genuine overlapping work — a re-audit cannot resolve it. Reclassify to
			// the integrity-class conflict code so recovery routes to the debugger.
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet rebase CONFLICT (overlapping work the partition should have separated) → debugger\n", cycle)
			recoverCode, recoverClass = CodeGitFleetRebaseConflict, ShipClassIntegrity
		default:
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet rebase onto main failed (infra); aborting\n", cycle)
			return "", false
		}
	}
	// Recovery is deterministic Chain-of-Responsibility (no LLM); both routing
	// strategies just delegate to the pure router.Recover, so call it directly.
	// This keeps recovery available even when no routing Strategy was wired
	// (e.g. Stage:Off) — error handling must not depend on routing being on.
	dec := router.Recover(router.RouteInput{
		Blocker: &router.Blocker{
			Code:  string(recoverCode),
			Class: string(recoverClass),
			Stage: string(se.Stage),
		},
	})
	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == PhaseEnd {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) is unrecoverable (%s); aborting\n", se.Code, se.Class, dec.Reason)
		return "", false
	}
	if !o.sm.CanTransition(PhaseShip, cand) {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery proposed illegal edge ship→%s (%s); aborting\n", cand, dec.Reason)
		return "", false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) → recovery routes to %s (%s)\n", se.Code, se.Class, cand, dec.Reason)
	return cand, true
}

// rebaseCycleBranchOntoMain rebases the cycle's worktree branch onto the current
// main so a fleet cycle whose ff-merge diverged (a peer moved main) can re-audit
// + re-ship the merged tree (ADR-0049 S5b). Returns ok=true on a clean replay.
// On failure it distinguishes (G13a) a genuine merge CONFLICT (unmerged paths —
// overlapping work the partition should have separated, route to the debugger)
// from an infra failure (conflict=false, abort the cycle). Either way it aborts
// the in-progress rebase so the worktree is left clean (no half-applied rebase).
// An empty worktree returns (false, false) — a degraded run never rebases.
func rebaseCycleBranchOntoMain(ctx context.Context, worktree string) (ok bool, conflict bool) {
	if worktree == "" {
		return false, false
	}
	if _, exit, err := gitCapture(ctx, worktree, "rebase", "main"); err != nil || exit != 0 {
		// Detect a real conflict BEFORE aborting (abort clears the unmerged index).
		unmerged, _, _ := gitCapture(ctx, worktree, "diff", "--name-only", "--diff-filter=U")
		conflict = strings.TrimSpace(unmerged) != ""
		_, _, _ = gitCapture(ctx, worktree, "rebase", "--abort")
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s onto main failed (exit=%d, err=%v, conflict=%v); rebase aborted\n", worktree, exit, err, conflict)
		return false, conflict
	}
	return true, false
}

// decideAfterDebugger maps the debugger phase's recovery decision (surfaced on
// PhaseResponse.Signals by the debugger runner) to the next phase, mirroring
// decideAfterRetro. RESHIP→ship; RERUN_PHASE→the named phase (defaulting to
// audit); BLOCK/empty/unknown→end. A malformed decision already safe-defaulted
// to BLOCK in the debugger's Classify, so this conservatively ends on anything
// not explicitly RESHIP/RERUN_PHASE.
