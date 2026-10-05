# Build Explanation — Cycle 1797

## Build Binding
- Cycle: 1797
- Base SHA: 63d60776ae1f1afcdc27240f80278e440837e2d0

## Summary
`evolve worktree cleanup --stale` lists the sealed cycle worktrees that nothing holds. It is a dry run unless `--apply` is given, which removes those worktrees and their merged branches. A lane that is unsealed, named by a continuation binding, held by a fresh run lease or dirty is reported as kept, with the reason. `evolve branches audit` now reports a lane branch whose cycle holds a fresh run lease as `live superseded=false`, and `branches prune` never deletes one. `evolve worktree create --dev <task> --branch <b>` and `evolve worktree cleanup --dev <task>` replace the hand-run hub dev-tree steps. A squash-merged PR counts as merged only when `gh` reports a merged PR whose head commit (`headRefOid`) is the dev tree's local head.

## Rationale
The console could not clear 95 legacy worktrees safely. `cleanup --cycle N` force-removed with no liveness or binding check, and `branches audit` called every live lane's still-empty branch superseded. The new mode reuses the existing evidence sources rather than inventing any: `dossier.ClosedOut` for sealed, `continuation.ListRegistryEntries` for bindings and `runlease.LiveOwner` for a fresh lease. `internal/gc`'s sweep does not consult continuation bindings and is policy-driven (age and keep-recent), so it was not reused for an explicit operator verb. The dev verbs encode the workspace-layout rules (branch from the fetched `origin/main`; remove only after the merge), including the squash-merge case that git cannot see.

## Changed Areas
- `go/cmd/evolve/cmd_worktree_stale.go` — new. Holds the `--stale` planner and applier (`planStaleLanes`, `staleKeepReason`, `removeStaleLane`), the binding index (`readLaneBindings`, matched by branch or by worktree leaf, because a binding's path can name a retired hub), and `liveLaneRef`, which the branches commands share.
- `go/cmd/evolve/cmd_worktree_dev.go` — new. `resolveDevHub` finds the hub as the parent of the bare common git dir (via `internal/plane`) and refuses a plain checkout. `runWorktreeCreateDev` refuses an existing task dir or branch with exit 1, then fetches `origin/main` and adds the tree with `--no-track`. Any git failure exits 2. `runWorktreeCleanupDev` refuses a dirty tree, a detached tree or an unmerged branch with exit 1. Merged means the head is in the fetched `origin/main`, or one of the merged PRs that `gh pr list --state merged` lists for the branch has the local head as its `headRefOid` (`mergedPRHeadIs`). A merged PR under the same branch name at any other commit proves nothing about the local head. A merged tree is removed with a non-force `git worktree remove`. Its branch is deleted with `git update-ref -d refs/heads/<b> <head>`, which deletes it only while it still points at the head that was proven merged.
- `go/cmd/evolve/cmd_worktree.go` — adds the `--dev`/`--branch` flags to `create` and the `--dev`/`--stale`/`--apply` flags to `cleanup`, with exclusive modes as usage exit 10. The `--cycle` removal and the prune were split into `removeCycleWorktree` and `pruneWorktrees` to keep each function within the 50-line size ratchet. Their two inline comments were dropped under the no-comments convention.
- `go/cmd/evolve/cmd_branches.go` — audit prints `live superseded=false` for a leased lane. Prune wraps `hasOpenPR` with `keepLive`, so a live lane's branch is kept the way an open PR keeps one, and is printed as `live ... kept`.
- `go/internal/gc/worktrees.go` — `leafCycleNumber` is exported as `LeafCycleNumber` so the CLI parses lane leaves the same way the sweep does.
- `go/internal/gc/processes.go` — the call site follows the rename.
- `go/internal/gc/leafcycle_test.go` — a table test that names and runs `LeafCycleNumber`. This is the apicover requirement for the new export.
- `go/cmd/evolve/cmd_worktree_stale_test.go` — in-process tests of every keep reason, a dry run that removes nothing, an `--apply` that removes only the free lane, exclusive-flag usage errors, and audit/prune of a leased branch.
- `go/cmd/evolve/cmd_worktree_dev_test.go` — an in-process hub fixture covering create at the fetched origin tip, a duplicate-task refusal, dirty and unmerged refusals (the unmerged one through an injected PR lookup reporting an older head), a merged cleanup, and a plain checkout refused as not a hub. A table test of `mergedPRHeadIs` covers no PR, an exact head match, a match in a later PR, a PR at an older head and a short-SHA prefix.
- `go/acs/cycle1797/helpers_test.go` — TDD-authored binary-driving harness and fake `gh`.
- `go/acs/cycle1797/predicates_test.go` — the fourteen TDD-authored acceptance predicates for both items, including the post-merge-commit and reused-branch-name refusals.
- `.evolve/evals/worktree-gc-respects-continuations.md` — the eval for the stale-cleanup item (TDD-authored, tracked with the build).
- `.evolve/evals/cli-worktree-dev.md` — the eval for the dev-worktree item (TDD-authored, tracked with the build).
- `docs/operations/workspace-layout.md` — the dev-tree rule and the tree diagram now name the two `evolve worktree ... --dev` verbs, their refusal causes and exit codes 1 and 2, in place of the raw `git worktree add/remove` and `git branch -D` steps.
- `docs/operations/runtime-reference.md` — the runtime-constraints paragraph documents `cleanup --stale [--apply]`, the dev verbs and the live-lane audit and prune behavior.
- `docs/architecture/packages/internal-gc.md` — records why `LeafCycleNumber` is exported.

## Design Decisions
- A lane is removable only when all of these hold: it is sealed (a closeout dossier exists), no continuation binding names its branch or leaf, it has no fresh run lease (`runlease.LiveOwner`: a fresh heartbeat and a live pid), and it is clean. Keeping is the safe direction, so the checks run in that order and the first one that holds becomes the reported reason.
- `--apply` re-checks the lease immediately before each removal, uses a non-force `git worktree remove`, and deletes the branch with `git branch -d`. A branch that is not merged into HEAD therefore stays in place and is reported, and a failed removal exits 1 after the rest of the batch has been processed.
- Dev cleanup force-deletes the branch only after its own merge proof, because git's `branch -d` cannot see a squash merge. The proof is bound to the commit being deleted: either the local head is an ancestor of the fetched `origin/main`, or a merged PR's `headRefOid` equals the local head exactly. So every commit the branch holds was in `origin/main` or in the merged PR. The first build accepted any merged PR named by the branch, which let `-D` delete a commit made after the merge and new work under a reused branch name (cycle 1797 audit H1). Exact equality was chosen over an ancestry check against the PR head, because that commit need not exist in the local store. The delete passes the proven head as the ref's expected old value, so a commit made between the proof and the delete makes the delete fail (exit 2) instead of losing that commit.
- The merged-PR lookup is passed into `runWorktreeCleanupDev` as a function (`ghMergedPRHeads` in production), so unit tests never start a real `gh`. The production call is bounded by a 30s context timeout. A timeout or any other `gh` failure counts as no proof, so cleanup refuses with exit 1 rather than hanging or deleting.
- A dev branch is created with `--no-track` so that a `git push` from it can never target `main`.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1797/`: 14/14 predicates PASS, driving the real binary against hub and lane fixtures.
- `go test -count=1 ./cmd/evolve -run 'Worktree|Branches|Ratchet'` PASS, with the existing worktree and branches tests unchanged.
- `go test -count=1 ./internal/gc ./internal/sizeratchet` PASS.
- `gofmt -l` is clean and `go vet ./...` is clean.

## Compatibility
Existing invocations behave as before: `create --cycle`, `cleanup`, `cleanup --cycle`, and audit/prune output for non-live branches. The only output change is for a branch whose cycle holds a fresh run lease, which was previously misreported as superseded.

## Limitations
- `--stale` considers only the registered worktrees under `--base` whose leaf is `cycle-*`, so orphan branches with no worktree stay with `evolve branches prune`.
- The dev verbs assume the hub's base branch is `main` on `origin`.
- The squash-merge proof needs an authenticated `gh`. Without one, cleanup refuses rather than guessing.
- A squash-merged branch whose local head is behind its PR's final head (the PR got commits pushed from elsewhere) is refused even though nothing would be lost. Pull, then clean up.
