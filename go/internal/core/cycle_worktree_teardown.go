package core

import (
	"fmt"
	"os"
)

func (o *Orchestrator) teardownCycleWorktree(projectRoot, wtPath string, preserve, completedNormally bool) {
	if wtPath == "" {
		return
	}
	if preserve || !completedNormally {
		if inPlaceWorktree(wtPath, projectRoot) {
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle ended abnormally in the project root %s (in-place worktree) — nothing to preserve or reclaim\n", wtPath)
			return
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] preserving worktree %s — cycle ended abnormally; recover via `evolve loop --resume` or reclaim with `evolve cycle reset`\n", wtPath)
		return
	}
	if cerr := o.worktree.Cleanup(projectRoot, wtPath); cerr != nil {
		return
	}
	o.clearActiveWorktree(wtPath)
}
