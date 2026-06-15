package core

import (
	"context"
	"fmt"
	"os"
)

// cyclerun.go — methods extracted from the RunCycle engine (orchestrator.go) to
// keep RunCycle a readable coordinator. Each extraction is behavior-preserving;
// the orchestrator's characterization tests are the safety net.

// finalizeCycle runs RunCycle's post-loop finalization (extracted verbatim,
// behavior-preserving): reclassify the final verdict against pre/post HEAD, warn
// loudly on a silent no-ship, record shipped throughput, decide worktree
// preservation, and persist the cycle-end state.
//
// It returns whether the worktree must be preserved — the caller's exit defer
// reads this AFTER finalizeCycle returns, so it MUST be threaded back to
// RunCycle's frame (the R2 late-visibility contract); a persist error preserves
// nothing extra here, the defer's !cycleCompletedNormally clause covers it.
func (o *Orchestrator) finalizeCycle(ctx context.Context, cs CycleState, cycle int, preCycleHEAD string, result *CycleResult, state *State) (preserveWorktree bool, err error) {
	postCycleHEAD, _ := o.gitHEAD()
	result.FinalVerdict = o.finalizeOutcome(result.FinalVerdict, result.RetroDecision, preCycleHEAD, postCycleHEAD)

	// Notice the silent no-ship (Fix C): the cycle ran phases but ended without
	// HEAD advancing and without an audit-advisory "would-have-blocked" record —
	// i.e. work may have been produced and then discarded with the worktree
	// (cycle-148: a genuine PASS mis-graded FAIL routed audit→retro→end). The
	// outcome label alone is advisory and easily missed in a batch summary, so
	// surface it loudly here. Not an error — some cycles legitimately produce no
	// change — but always worth an operator's eyes.
	if result.FinalVerdict == CycleOutcomeSkippedUnknown {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d ended without shipping (%s): phases ran but HEAD did not advance and no audit-advisory block was recorded — any worktree changes were discarded. Inspect %s (audit-report.md verdict + acs-verdict.json red_count).\n", cycle, CycleOutcomeSkippedUnknown, cs.WorkspacePath)
	}

	// R9.1: a shipped cycle's committed floors are observed throughput —
	// record them into the rolling window before the state write below
	// persists it (nil seam ⇒ byte-identical no-op).
	if o.throughputRecorder != nil && shippedOutcome(result.FinalVerdict, preCycleHEAD, postCycleHEAD) {
		o.throughputRecorder(state, cycle, cs.WorkspacePath)
	}

	// A completed cycle that FAILED its verdict keeps its worktree for salvage
	// (inbox preserve-worktree-on-verdict-fail). The exit defer prunes only when
	// !preserveWorktree, so the caller sets the flag from this return before
	// marking completion. This MUST stay AFTER finalizeOutcome above: it reads
	// the FINAL verdict, so a SKIPPED/SHIPPED_VIA_BUILD reclassification has
	// already happened — reading it earlier would preserve on a pre-reclassification
	// raw FAIL. L3 gc (internal/gc) reclaims preserved worktrees on retention;
	// `evolve cycle reset` / `evolve loop --resume` reclaim them explicitly.
	preserveWorktree = preserveOnVerdict(result.FinalVerdict)

	state.LastCycleNumber = cycle
	if perr := o.persistCycleEndState(ctx, *state); perr != nil {
		return preserveWorktree, fmt.Errorf("write state: %w", perr)
	}
	return preserveWorktree, nil
}
