# Build Explanation — Cycle 1905

## Build Binding
- Cycle: 1905
- Base SHA: 4f363b8901b8d1dcd9267f98772e4449cc921de7

## Summary
`evolve doctor plane` now inspects hub worktree checkouts for broken gitdir links and supports `--repair` to restore them with `git worktree repair`.

## Rationale
Moving a hub directory or renaming a `dev/*` worktree breaks the `.git` pointer in the checkout or the store record. Previously, operators had to run raw `git --git-dir=.repo.git worktree repair` by hand. Adding inspection and repair to `evolve doctor plane` gives operators one CLI command with clear exit codes.

## Changed Areas
- `go/internal/cli/opscmd/doctor_plane.go` — parses `--repair` with `flag.FlagSet` and `cmdutil.ReorderArgs`. It locates `.repo.git`, reads each store record under `.repo.git/worktrees/*/gitdir`, finds the broken worktrees, and runs `git worktree repair` when `--repair` is set.
- `go/internal/cli/opscmd/doctor_plane_test.go` — covers a renamed worktree, a worktree under `.evolve/worktrees`, a foreign primary checkout, and a deleted worktree.
- `docs/operations/workspace-layout.md` — tells the operator to run `evolve doctor plane --repair` after a hub move.
- `docs/architecture/packages/internal-cli-opscmd.md` — records the new exit-code invariants of the plane report.
- `.evolve/evals/cli-doctor-plane-repair.md` — lists the eval graders of the task.
- `go/acs/cycle1905/predicates_test.go` — the TDD predicates of the cycle.

## Design Decisions
Flag parsing uses `cmdutil.ReorderArgs` so `--repair` works before or after the root path. Hub discovery walks up to `.repo.git`.

Worktree discovery reads the store records, not the file tree. Each record gives the recorded `.git` path. When that path is gone, the command tries the same relative path under the new hub root, then the siblings of the recorded directory. A candidate counts only when its `.git` file names the same record in a store called `.repo.git`.

Thus worktrees under dot-directories such as `runtime/.evolve/worktrees` and worktrees outside the hub tree are inspected. Submodules and checkouts of other repositories are never reported or repaired. Inspection no longer walks the hub: on the real hub it takes 0.5 s instead of 19.3 s. The rejected option was `git worktree list --porcelain`: after a hub move it prints the old paths, so it cannot find the moved checkouts.

## Verification
The cycle 1905 ACS predicates (`go/acs/cycle1905`) and unit tests in `go/internal/cli/opscmd/doctor_plane_test.go` test moved hubs, renamed worktrees, healthy hubs, bad flags, and git errors. Predicates 006 to 008 test a lane under `.evolve/worktrees`, foreign checkouts in the hub, and a worktree outside the hub.

## Compatibility
Existing `evolve doctor plane [root]` output lines (`plane:`, `branch:`, `gitdir:`) and the primary-checkout warning (exit 2) do not change.

## Limitations
Hub worktree discovery and repair apply only to checkouts inside a hub layout with `.repo.git`. Standalone checkouts keep single-checkout inspection. A store record without a `gitdir` file is skipped, because it names no checkout path. A worktree that moved to a different parent directory without a hub move is not found; run `git worktree repair <path>` for it. A record whose checkout was deleted is skipped; `git worktree prune` removes it.
