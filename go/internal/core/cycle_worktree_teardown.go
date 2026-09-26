package core

import (
	"fmt"
	"os"
)

// teardownCycleWorktree is the ONE worktree-disposal rule both entrypoints
// apply at cycle exit, so a resumed cycle disposes its worktree exactly like a
// fresh one.
//
// The decision is deliberately conservative in one direction only — it prunes
// just the spent trees:
//
//   - preserve (finalizeCycle's preserveOnVerdict, or a recorded ship-stage
//     failure) means the tree holds audited, possibly uncommitted work that
//     `evolve loop --resume` or `evolve cycle reset` reclaims BY this path.
//   - !completedNormally means the cycle died before adjudicating its work at
//     all, which is the same situation with less information.
//   - Otherwise the cycle finished and ship has merged the worktree into main,
//     so the tree is spent; pruning it is what keeps the checkout count bounded.
//
// Erring the other way is not symmetric: a missed prune costs disk, while a
// wrong prune destroys real, possibly unmerged, audited work. That asymmetry
// is why both no-prune conditions stay, and why the resume path gets this
// exact rule rather than a resume-specific policy.
//
// This rule decides WHETHER to dispose; the provisioner decides whether a
// path is one it may dispose of. gitWorktree.Cleanup refuses the project root
// (a resume checkpoint can name it) and returns an error, which the cerr
// branch below treats exactly like any other failed prune: the path stays
// named so resume/reset can still reach it.
func (o *Orchestrator) teardownCycleWorktree(projectRoot, wtPath string, preserve, completedNormally bool) {
	if wtPath == "" {
		// No worktree was provisioned (or the resume checkpoint named none):
		// nothing to dispose, and nothing to announce.
		return
	}
	if preserve || !completedNormally {
		if inPlaceWorktree(wtPath, projectRoot) {
			// Nothing to reclaim: the "worktree" is the operator's own tree.
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle ended abnormally in the project root %s (in-place worktree) — nothing to preserve or reclaim\n", wtPath)
			return
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] preserving worktree %s — cycle ended abnormally; recover via `evolve loop --resume` or reclaim with `evolve cycle reset`\n", wtPath)
		return
	}
	if cerr := o.worktree.Cleanup(projectRoot, wtPath); cerr != nil {
		// The tree may still be on disk — leave the path named so
		// resume/reset can still reach it.
		return
	}
	o.clearActiveWorktree(wtPath)
}
