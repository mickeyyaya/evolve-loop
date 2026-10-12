# Build Explanation — Cycle 1859

## Build Binding
- Cycle: 1859
- Base SHA: ba30cf5c2553d86834db0a087a8b2a79d53af79b

## Summary
The function `runWorktreeCreateDev` deletes the branch that git created when `git worktree add -b` fails.

## Rationale
Git creates the branch before it creates the worktree directory. When directory creation fails, git leaves the branch behind in the repository. The leftover branch blocks subsequent retry attempts with a branch-already-exists refusal. Deleting the branch on failure restores repository state.

## Changed Areas
- `go/cmd/evolve/cmd_worktree_dev.go` — deletes the branch with `git branch -D` when `git worktree add` fails.
- `go/cmd/evolve/cmd_worktree_dev_backfill_e3_test.go` — expects no leftover branch for the uncreatable dev directory case.
- `go/cmd/evolve/cmd_worktree_dev_test.go` — verifies that a failed dev worktree creation leaves no branch and retry succeeds.

## Design Decisions
The code deletes the branch immediately inside the worktree add failure handling path before returning exit code 2. It reuses the existing `store` executor without adding extra helper abstractions.

## Verification
Unit test `TestWorktreeDev_FailedCreateLeavesNoBranch` and backfill test `TestRunWorktreeCreateDev_RefusalsAndGitFailures` pass. The two ACS predicates for cycle 1859 pass.

## Compatibility
The command line interface, exit codes, and output streams remain unchanged.

## Limitations
Branch deletion on failure applies specifically to dev worktree creation in `runWorktreeCreateDev`.
