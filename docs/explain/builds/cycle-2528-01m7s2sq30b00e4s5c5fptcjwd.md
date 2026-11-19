# Build Explanation — Cycle 2528

## Build Binding
- Cycle: 2528
- Base SHA: 0718e2d6d1499d0832641953bb9a15258bbc6c5b

## Summary
The `evolve acs suite` command now resolves the plane worktree that holds the cycle state across linked worktrees. It no longer falls back to the parent of the git common directory.

## Rationale
In a hub layout with a bare common repository, `mainProjectRoot` returns the hub directory rather than the plane worktree. Searching the non-bare worktrees from `git worktree list --porcelain` for `.evolve/runs/cycle-N/cycle-state.json` finds the true plane and rejects guessed roots when no state exists.

## Changed Areas
- `go/cmd/evolve/cmd_acs.go` — adds `resolveSuiteProjectRoot`, `hasCycleStateDir`, and `parseNonBareWorktrees`, and wires `runACSSuite` to fail when a git-backed cycle has no worktree state.

## Design Decisions
- Fact F1: In a hub layout, `git rev-parse --git-common-dir` returns `<hub>/.repo.git`, so `mainProjectRoot` resolves `<hub>` (`go/cmd/evolve/cmd_acs.go`).
- Fact F2: `TestSuiteProjectRoot_FallbackToGitDerivation` calls `suiteProjectRoot` with a single `string` return value and expects `mainProjectRoot` when no cycle state exists (`go/cmd/evolve/cmd_acs_projectroot_test.go`).
- Implication R1: `resolveSuiteProjectRoot` must return `(string, error)` for `runACSSuite`, while `suiteProjectRoot` keeps its `string` signature and falls back to `mainProjectRoot` on error.
- Decision D1: `resolveSuiteProjectRoot` checks the explicit `--evolve-dir` first, skips bare and prunable worktrees from `git worktree list --porcelain`, and returns the worktree that contains `.evolve/runs/cycle-N/cycle-state.json`.
- Rejected option O1: Changing `suiteProjectRoot` to return `(string, error)` breaks the frozen unit test signature.
- Rejected option O2: Changing `mainProjectRoot` globally affects unrelated callers outside `acs suite`.

## Verification
All `cmd/evolve` unit tests, hub-root tests, and cycle 2528 ACS predicates run and pass with `go test -count=1`. Non-git roots without a worktree list keep the prior fallback behavior.

## Compatibility
The `suiteProjectRoot` function keeps its `string` return type, no-cycle calls (`cycle <= 0`) keep the `mainProjectRoot` derivation, and non-git `--root` directories keep their prior behavior.

## Limitations
Other callers of `mainProjectRoot` outside `evolve acs suite` stay unchanged in this cycle.
