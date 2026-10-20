package core

func LinkGuardDepsForTesting(worktree, projectRoot string, cycle int) {
	linkGuardDeps(worktree, projectRoot, cycle)
}
