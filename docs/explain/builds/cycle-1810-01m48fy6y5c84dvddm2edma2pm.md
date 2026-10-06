# Build Explanation — Cycle 1810

## Build Binding
- Cycle: 1810
- Base SHA: 493ff848fda83021dca736b49d91e1afffa83743

## Summary
This build adds two operator verbs. `evolve land --branch B (--patch F | --salvage <leaf>)` lands a lane patch or a salvaged cycle, uncommitted, on a new console dev worktree at the freshly fetched `origin/main`. `evolve boundary run [--merge n,…] --goal-text-file F [--max-cycles N] [--dry-run]` runs the wave boundary as one verb: `loop-stop --wait`, `pr merge`, `sync-main`, `gc`, `loop-stop --release`, `loop --detach --log <plane>/.evolve/boundary-loop.log`, stopping at the first failed step with that step's exit code.

## Rationale
Both procedures were sequences the operator typed by hand. A patch landed with raw `git apply` on a stale local `main`, or a partial apply, went unnoticed until ship; a skipped or failed boundary step (a merge while a lane still ran, a launch on an unsynced plane) was found later. Each verb reuses the existing code instead of copying it: `land` creates the worktree through `runWorktreeCreateDev`, the function behind `evolve worktree create --dev`, and `boundary` calls the existing verb handlers in-process, so every refusal stays the underlying verb's own.

## Changed Areas
- `go/cmd/evolve/cmd_land.go` — the `land` verb: argument parsing with the package's `cliFlags`, input checks (patch file or salvage leaf) before any side effect, worktree creation through `runWorktreeCreateDev`, `git apply --3way`, naming conflicted files from `git diff --diff-filter=U`, and a restore of the leaf's `untracked.tgz` that refuses entries escaping the worktree or overwriting a file.
- `go/cmd/evolve/cmd_land_test.go` — unit tests for the usage refusals and the archive restore's escape and overwrite refusals.
- `go/cmd/evolve/cmd_boundary.go` — the `boundary run` verb: argument validation, the six-step plan built from the existing verbs with `--project-root` pinned to the resolved plane, the dry-run printer, and the sequencer that stops at the first nonzero step and returns its code.
- `go/cmd/evolve/cmd_boundary_test.go` — the TDD-authored fake-dispatch tests for step order, early stop, dry run and bad arguments.
- `go/cmd/evolve/cmd_boundary_launch_test.go` — the TDD-authored repair tests that feed the launch step's argv to `evolve loop`'s real argument parser and check a refused launch returns the loop's own exit code after the release.
- `go/cmd/evolve/registry.go` — registers `land` and `boundary`.
- `go/cmd/evolve/main.go` — lists both verbs in the top-level usage.
- `docs/operations/runtime-reference.md` — documents both verbs, their steps and exit codes beside the wave-boundary procedure.
- `go/acs/cycle1810/predicates_test.go` — the TDD-authored predicates that drive the real binary against a hub fixture.
- `go/acs/cycle1810/helpers_test.go` — the TDD-authored hub, salvage-leaf and fake-`gh` fixtures for those predicates.
- `.evolve/evals/cli-boundary-run.md` — pins the boundary acceptance to the cycle 1810 predicates and frozen tests.
- `.evolve/evals/cli-land-patch.md` — pins the land acceptance to the cycle 1810 predicates.

## Design Decisions
`boundary` dispatches through an explicit five-entry handler map rather than the registry table, because the registry already references `runBoundary` and a lookup through it would be an initialization cycle; the map also makes the set of verbs it may run visible. The release step is placed after `sync-main` and `gc`, so any failure before it leaves the brake engaged. `land` takes no task name: the dev directory is the branch name with `/` replaced by `-`, which `validDevTask` accepts. The launch step passes `--log <plane>/.evolve/boundary-loop.log` because `evolve loop` refuses `--detach` without `--log` (exit 10); an absolute path under the plane keeps the log out of the operator's cwd, and one fixed name means successive boundaries append to the same file instead of scattering logs. An input check failure exits 2 before `runWorktreeCreateDev` runs, so a missing patch or leaf never creates a branch. A conflicting apply still restores the leaf's untracked files, then exits 1, so the operator resolves one complete tree.

## Verification
The cycle 1810 predicates run the built binary against a hub fixture whose origin moves after the store is cloned: a land on the fetched tip with the patch uncommitted and nothing pushed, a salvage with nested untracked paths, a `--3way` conflict exiting 1 with markers, an existing branch refused with no worktree created, missing inputs exiting 2 with no side effect, a dry run that engages no brake and calls no `gh`, and a real `pr merge` refusal that stops the boundary before `sync-main` with the brake kept. The fake-dispatch tests cover each step's failure code and the full six-step order, and the launch repair tests parse the dispatched launch argv with `parseLoopArgs`, the parser `evolve loop` itself uses, so a launch it would refuse fails the test.

## Compatibility
Both verbs are additive; no existing command, flag or exit code changes.

## Limitations
`land` restores regular files from `untracked.tgz` only; a symlink or other entry type is refused by name for a manual restore. `boundary` does not run `evolve worktree cleanup --dev --all`, which stays an operator step, and exposes no `--wait` timeout of its own, using `loop-stop`'s default.
