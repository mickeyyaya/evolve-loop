# ADR-0122 — Checkpoint uncommitted worktree work as refs in the shared store

- **Status:** Accepted (2026-10-06, console lane `cl-checkpoint`).
- **Supersedes nothing.** Adds the `evolve checkpoint` verb and the `refs/checkpoints/` namespace; the loop's cycle salvage (`evolve gc`'s salvage-remove, ADR-0072's "salvage before requeue") is unchanged.
- **Related:** the [hub layout](../../operations/workspace-layout.md) (one bare store, `dev/` worktrees); the package notes [internal-wtcheckpoint.md](../packages/internal-wtcheckpoint.md); the operator reference [runtime-reference.md, "Operator commands (protect in-progress work)"](../../operations/runtime-reference.md).

## Context

The operator asked: "Could we setup a checkpoint so we don't lose the progress because of accidents?" Console lanes keep their work only as uncommitted, staged changes in `dev/` worktrees until a landing, and they cannot commit (the commit gate and `evolve ship` own commits). An agent dying, a worktree being removed, a `reset --hard`, a checkout or a churn-discard can each destroy hours of work. The loop's cycle salvage covers only cycle worktrees under `runtime/.evolve/worktrees/`, and the project rule is that every control goes through the published `evolve` CLI, so a control the CLI cannot do is a defect.

On 2026-10-06 the console took an interim snapshot of ten dirty dev worktrees with raw git plumbing: copy the worktree's index, `write-tree` it for the staged tree; copy it again, `add -A` (minus `go/evolve` and `.evolve/ledger.*`) and `write-tree` for the full tree; `commit-tree` both, staged on `HEAD` and full on staged; `update-ref refs/checkpoints/<worktree>/<stamp>`; skip a worktree whose newest checkpoint already has its full tree.

## Decision

1. **Checkpoints are git refs in the shared store, `refs/checkpoints/<worktree>/<UTC stamp>`, naming a two-commit chain: the full working tree on the staged tree on `HEAD`.** The verb writes exactly the shape the console's interim snapshot wrote, so those refs are first-class.
2. **A save never touches the worktree's branch, index or working tree.** It writes trees through copies of the index (`gitexec.Git.TreeOfIndexCopy`, `GIT_INDEX_FILE`), never locks the real index, ignores inherited git location variables, and so runs safely beside a lane's own git commands. The scratch-index primitive lives in `internal/gitexec`, where `internal/treefence`'s older private copy should converge.
3. **Restore never overwrites work.** It targets a new path or a clean linked worktree, refuses a dirty one and one holding an ignored file at a path it would write, and brings back the staged, unstaged and untracked split exactly.
4. **Retention is config.** A strict `checkpoint` block in `.evolve/policy.json` (`keep_per_worktree`, compiled default 20, a zero or negative value refused) bounds each worktree's refs; a save prunes beyond it, and a save that cannot read the policy prunes nothing. The excluded paths (`go/evolve`, `.evolve/ledger.*`) are not config: they are a safety rule, which a policy edit must not be able to remove.
5. **Pushing is explicit.** `save --push` sends the saved refs to `origin refs/checkpoints/*`; nothing pushes by default, because pushing work in progress to GitHub has not been approved.
6. **`prune --landed` uses a conservative per-checkpoint rule for now**: a checkpoint is landed when every path it changed already matches `origin/main`. It converges onto the dev-worktree content-landed proof the `cl-gc` lane is adding once that lands.
7. **The hub has one definition.** `plane.ResolveHub` (a bare store, its parent as the hub root, `<root>/dev`, `refs/remotes/origin/main`) serves both `evolve checkpoint` and `evolve worktree create|cleanup --dev`.

## Alternatives considered

- **`git stash create` + `git stash store`.** A stash is an index-and-worktree commit pair too, but `stash create` refreshes and can write the real index, untracked files need `-u` (which `stash create` does not take), and stashes live in one per-repository reflog (`refs/stash`) shared by every worktree, where one lane's `stash drop` or `stash clear` could delete another lane's safety copy.
- **WIP commits on the lane branch.** They change the branch the lane will land, collide with the commit gate and `evolve ship`, and are exactly what a reset would discard.
- **Patch files and tarballs (gc's `operator-salvage` format).** They live in one plane's `.evolve`, are not content-addressed or deduplicated, cannot be listed or restored by git, and a removed worktree's directory takes them with it unless written elsewhere.
- **Locking the real index for a consistent snapshot.** It would make a save fail, or block, a lane's concurrent `git add`; copying the index gives a consistent snapshot of a moment without any lock.
- **Auto-push to origin.** It would survive a lost disk, but publishes unreviewed work to GitHub; kept as the explicit `--push` until the operator approves more.

## Consequences

- Any lane or the console can protect work with one command, and recover it into a fresh worktree with another; the refs survive removal of the worktree and deletion of its branch, since they keep the base commit reachable.
- The shared store grows by the objects of each changed snapshot; identical content is not re-saved, and retention bounds the refs per worktree. Pruned refs' objects go with git's normal garbage collection.
- A same-second second save of different content fails loudly (the ref is created, never moved).
- `evolve gc` on main never removes a dev worktree; it touches only cycle worktrees under the plane's worktree base. When the dev-worktree removal the `cl-gc` lane is adding lands, it should refuse a dirty dev worktree whose content is neither landed nor checkpointed, and should checkpoint a dirty tree before a forced removal (see the package notes' Findings).
