package sysexec

import "slices"

type GhCall struct {
	Site    string
	Verb    string
	Command string
	Purpose string
}

var ghCalls = []GhCall{
	{"cmd/evolve/cmd_status.go", "status", "gh pr list", "Show the open pull requests and their check state in the status report."},
	{"internal/releasepreflight/releasepreflight.go", "release-preflight", "gh run list", "Read the CI run of the head commit before a release."},
	{"internal/ciwatch/classify_source.go", "ci classify", "gh run view", "Read a failed CI run and its changed files to classify the failure."},
	{"internal/ciwatch/ghfetcher.go", "ci watch", "gh run list", "Poll the required CI workflow run of a commit or branch."},
	{"cmd/evolve/cmd_ship_pr.go", "ship", "gh pr create", "Open or update the ship pull request."},
	{"internal/phases/ship/gitops.go", "ship", "gh release create", "Publish the GitHub release of a shipped tag."},
	{"cmd/evolve/cmd_pr.go", "pr merge", "gh pr merge", "Merge a pull request when its checks pass."},
	{"cmd/evolve/cmd_release_promote.go", "release promote", "gh release edit", "Promote a release and rerun its workflow."},
	{"cmd/evolve/cmd_branches.go", "branches", "gh pr list", "Find if a branch has an open pull request."},
	{"cmd/evolve/cmd_gc_devwt.go", "gc", "gh pr list", "Find open and merged pull request heads to decide which dev worktrees to remove."},
	{"cmd/evolve/cmd_worktree_dev.go", "worktree dev", "gh pr list", "Find if the head of a worktree branch merged."},
	{"cmd/evolve/cmd_boundary_ratchet.go", "boundary ratchet", "gh pr create", "Open and list the pull requests of the boundary ratchet."},
	{"cmd/evolve/cmd_loop_wavesync.go", "loop", "gh api", "Read the check runs of a main commit to find a red main."},
	{"internal/integration/ghci.go", "integration", "gh pr create", "Open, watch and merge the integration batch pull request."},
	{"internal/gc/remotegh.go", "gc", "gh pr list", "List all pull requests to decide which remote branches to delete."},
	{"cmd/evolve/cmd_inbox_curator.go", "inbox curate", "gh pr list", "Find open pull requests that cover an inbox item before the curator applies a proposal."},
	{"internal/rollback/rollback.go", "rollback", "gh release delete", "Delete the GitHub release of a rolled-back tag."},
}

func GhCalls() []GhCall { return slices.Clone(ghCalls) }
