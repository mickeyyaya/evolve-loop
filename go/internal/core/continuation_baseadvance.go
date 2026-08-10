package core

// continuation_baseadvance.go — the worktree-base limb of the binary-lag
// class (inbox continuation-worktree-base-refresh; sibling of the boot
// binary self-heal, #411). A preserved continuation worktree keeps its
// original base forever, so a chain predating a landed pipeline fix
// deterministically re-fails on defects the repo already fixed (live:
// cycle-1365, GIT_STAGE_FAILED on .evolve/evals twice in the retry loop —
// its base predated #418's .gitignore carve-out; unwinnable in place).
//
// Staleness is healed where the binary heals: at the worktree's boot —
// continuation adoption. The adopt-time Clean screen (validateContinuation →
// ClassifyFleetRebaseCandidate) has JUST proven the snapshot merges Clean
// against main, so the advance merge is conflict-free by construction; a
// raced conflict (main moved between the screen and the merge) degrades
// LOUDLY to the stale-base status quo: abort, WARN naming the conflicting
// paths, adopt anyway. Never a half-merge, never a silent skip.

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// advanceContinuationBase merges current plane main into the adopted lane
// worktree and returns the new base SHA (main's tip) on success — the caller
// stores it as WorktreeBaseSHA so worktree-normalize and the review diff bind
// to the healed base. Returns "" when the base is already current (no-op) or
// when the merge conflicts/fails (degrade to the stale base, loudly).
func advanceContinuationBase(ctx context.Context, worktree string, cycle int) string {
	g := gitexec.Git{Dir: worktree, Exec: gitRunner}
	// Already current: main is an ancestor of the lane HEAD.
	if _, _, code, err := g.Capture(ctx, "merge-base", "--is-ancestor", "main", "HEAD"); err == nil && code == 0 {
		return ""
	}
	// --no-verify for the same reason the salvage snapshot commit carries it:
	// a repo hook must not turn every adoption into a permanent stale-base
	// degrade (misread as a raced conflict with an empty path list).
	args := append(append([]string{}, snapshotIdentity...), "merge", "--no-edit", "--no-verify", "main")
	if _, stderr, code, err := g.Capture(ctx, args...); err != nil || code != 0 {
		conflicts, _, _, _ := g.Capture(ctx, "diff", "--name-only", "--diff-filter=U")
		_, _, _, _ = g.Capture(ctx, "merge", "--abort")
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: base-advance merge failed rc=%d (%v: %s) — conflicting paths [%s]; adopting on the STALE base (the cycle-1365 class stays live for this lane)\n",
			cycle, code, err, strings.TrimSpace(stderr), strings.Join(strings.Fields(conflicts), ", "))
		return ""
	}
	tip, stderr, code, err := g.Capture(ctx, "rev-parse", "main")
	if err != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d continuation: base-advance rev-parse failed rc=%d (%v: %s) — merged but base pin unchanged\n", cycle, code, err, strings.TrimSpace(stderr))
		return ""
	}
	healed := strings.TrimSpace(tip)
	fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d continuation: base ADVANCED to main %s (worktree-base limb of the binary-lag class healed at adoption)\n", cycle, healed[:12])
	return healed
}
