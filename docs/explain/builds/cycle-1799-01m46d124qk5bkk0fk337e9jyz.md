# Build Explanation — Cycle 1799

## Build Binding
- Cycle: 1799
- Base SHA: 838158dbbb616d5f7c868efaf194b44cb78b16fb

## Summary
`evolve worktree cleanup --stale` now lists sealed cycle worktrees that nothing holds and removes them only with `--apply`, `evolve branches audit` reports a lane with a fresh run lease as live, and `evolve worktree create|cleanup --dev` manage console dev trees with refusals for dirty, unmerged and reused-name branches.

## Rationale
`cleanup --dev` must delete a dev branch after a squash merge, which git's own `branch -d` cannot recognize, without ever deleting a commit that is not in `origin/main`. Four alternatives were weighed:
- Merge proof: comparing the branch's content with `origin/main` (tree or patch-id equality) was rejected. A squash commit on `main` can differ from the branch tip after conflict resolution or later commits, so content checks give false negatives and need a heuristic match. The build instead takes ancestry in the fetched `origin/main`, or else a merged PR reported by `gh`. The cost is that a squash-merged branch depends on `gh`.
- PR matching: taking only the first PR (`.[0]`) or requiring exactly one PR per branch was rejected. A reused branch name lists several merged PRs, and `.[0]` would pick one by list order. The build accepts the head when any merged PR's `headRefOid` equals it exactly (`mergedPRHeadIs`). A merged PR at any other commit proves nothing, so a post-merge commit or new work under a reused name is refused.
- Without `gh`: guessing merged from content or from the branch name was rejected. With no `gh`, a `gh` failure or the 30s timeout, there is no proof and the command refuses with exit 1. The tradeoff is a kept tree that the operator removes by hand. No commit is lost.
- Delete: `git branch -D` was rejected because it deletes whatever the ref points at when it runs. The build uses the compare-and-delete `git update-ref -d refs/heads/<branch> <head>`. That delete succeeds only while the ref still points at the head that was proven merged.

This build adopts the cycle 1797 continuation, whose implementation already carried these choices. That handoff failed only because the `./cmd/evolve` unit tests ran past the shared floor budget, and the budget fix has since merged.

## Changed Areas
- `go/cmd/evolve/cmd_worktree_stale.go` — adds the stale-lane classifier that keeps bound, freshly leased, unsealed and dirty lanes with a reason.
- `go/cmd/evolve/cmd_worktree_dev.go` — adds `create --dev` and `cleanup --dev`; a squash merge counts only when gh reports a merged PR whose `headRefOid` equals the local head, and the branch is deleted by a compare-and-delete `git update-ref -d` bound to that head.
- `go/cmd/evolve/cmd_worktree.go` — routes the new flags and exit codes 1 and 2.
- `go/cmd/evolve/cmd_branches.go` — audit reports a leased lane as live and never superseded.
- `go/cmd/evolve/cmd_worktree_dev_test.go` — unit coverage for the dev commands.
- `go/cmd/evolve/cmd_worktree_stale_test.go` — unit coverage for the stale classifier.
- `go/internal/gc/worktrees.go` — shares the leaf-to-cycle parser with the CLI.
- `go/internal/gc/processes.go` — follows the shared parser.
- `go/internal/gc/leafcycle_test.go` — tests the exported parser.
- `go/acs/cycle1799/helpers_test.go` — fixtures and a fake gh for the acceptance predicates.
- `go/acs/cycle1799/predicates_test.go` — fourteen predicates driving the built binary.
- `docs/operations/workspace-layout.md` — documents the dev-tree commands in place of raw git steps.
- `docs/operations/runtime-reference.md` — documents the stale cleanup and the dev commands.
- `docs/architecture/packages/internal-gc.md` — notes the exported leaf parser.
- `.evolve/evals/cli-worktree-dev.md` — evidence commands point at cycle1799.
- `.evolve/evals/worktree-gc-respects-continuations.md` — evidence commands point at cycle1799.

## Design Decisions
- `cleanup --stale` is a dry run by default, and `--apply` is explicit. A lane is removed only when it is sealed, no continuation binding names it, it holds no fresh run lease (re-checked just before removal) and it is clean. Removal uses a non-force `git worktree remove` and then `git branch -d`, so an unmerged lane branch stays in place.
- `cleanup --dev` first refuses a dirty or detached tree (exit 1). It then fetches `origin/main` and proves the local head merged by ancestry (`git merge-base --is-ancestor`) or by an exact `headRefOid` match. Next it removes the tree with a non-force `git worktree remove`. Last, it deletes the branch with `git update-ref -d refs/heads/<branch> <head>`, passing the proven head as the expected old value. This is the race guard: if a commit lands on the branch between the proof and the delete, the ref no longer matches, git refuses the delete and the command exits 2 with the branch and its new commit intact.
- `create --dev` branches from the fetched `origin/main` with `--no-track`, so a push from the dev branch never targets `main`.

## Verification
`go test -count=1 ./cmd/evolve` passes, the fourteen cycle1799 predicates pass, and `evolve acs suite --cycle 1799` reports green=181 red=0.

## Compatibility
Existing `worktree create|list|cleanup` and `branches audit|prune` invocations are unchanged.

## Limitations
Without `gh`, a squash-merged branch is refused rather than guessed merged.
