# Build Explanation — Cycle 1879

## Build Binding
- Cycle: 1879
- Base SHA: 26838170cd0a90a9de528b45d9ae6bfe10a4b71b

## Summary
`evolve cycle scope-delta --cycle N [--json]` is a new read-only subcommand. It classifies each path that a lane changed with `scopedelta.Classify`, and it reports out-of-scope and cross-lane paths. It is the first production caller of `internal/scopedelta`.

## Rationale
Before this change, `internal/scopedelta` had no caller in `go/cmd/evolve` or `go/internal/core` (scout report, Key Findings). Operators found scope drift by hand: they compared `git diff` with the declared files of the inbox item. The cycle 1718/1719 doc conflict and the undeclared `offenders.json` edits of size-ratchet lanes went undetected that way. A command that reuses the classifier gives one rule set for operator inspection and for a later build gate.

## Changed Areas
- `go/cmd/evolve/cmd_cycle_scope_delta.go` — adds `runCycleScopeDelta`. It reads the lane's `cycle-state.json`, its claimed inbox records and its sibling lanes. It diffs the worktree against `worktree_base_sha`, then classifies and prints each changed path.
- `go/cmd/evolve/cmd_cycle.go` — adds the `scope-delta` case to the `runCycle` dispatch, so the subcommand is reachable from `evolve cycle`.
- `go/cmd/evolve/cmd_cycle_scope_delta_test.go` — adds unit tests for the cycle argument rules, the missing-worktree error and the same-wave sibling filter. The ACS predicates do not reach these branches.
- `docs/operations/runtime-reference.md` — documents the command, its scope sources, its JSON shape and its exit codes in the read-only operator commands list.
- `go/acs/cycle1879/predicates_test.go` — the TDD phase's acceptance predicates for the command. The build did not edit this file.
- `go/acs/cycle1879/helpers_test.go` — the TDD phase's fixture and built-binary helpers. The build did not edit this file.
- `.evolve/evals/cli-cycle-scope-delta.md` — the TDD phase's eval, with its score caps. The build did not edit this file.

## Design Decisions
- The declared scope is the `files` of the claimed records in `inbox/processing/cycle-N/` whose id is in `lane-scope.json` `todo_ids`. A lane with no pin uses every claimed record. Rejected: the Task Contract inbox path, because a claim moves the record away from it. The cycle 1879 Task Contract did not find its own record.
- A sibling lane is another `runs/cycle-<int>` lane whose `cycle-state.json` `goal_hash` equals this lane's hash. An empty hash matches no sibling. Rejected: every run directory, because lanes of earlier waves then mark unrelated paths cross-lane (predicate `TestC1879_003`).
- The protected list is the subset of changed paths for which `guards.IsProtectedSurface` is true. The manifest matches fragments, and `scopedelta.Scope.Protected` matches prefixes. Thus the subset is the exact projection, with no translation of the manifest.
- The JSON entries carry only `path`, `class` and `disposition`, and `summary` is `scopedelta.Summary` unchanged. The fixture decoder rejects unknown fields (test-report.md, Fixture Contract).
- An unreadable sibling is skipped with a WARN. An unreadable target lane exits 2. The sibling list is advisory, but the target lane is the subject of the report.

## Verification
- `cd go && go test -tags acs -count=1 ./acs/cycle1879` passes all ten predicates through the built binary and the real `main → runCycle` dispatch.
- `cd go && go test -count=1 ./cmd/evolve -run 'TestScopeDelta|TestLaneChangedPaths|TestSiblingLaneFiles'` passes the three unit tests.
- `go vet ./...` and `gofmt -l` report nothing.

## Compatibility
The command is additive and read-only: it writes no state and changes no existing subcommand. `internal/scopedelta` stays unchanged.

## Limitations
- `git diff --name-only <base>` sees committed and unstaged tracked changes, but not untracked files.
- A lane whose worktree was pruned after ship has no diff source, so the command exits 2 for it.
- The command passes an empty `Declaration` to `Classify`, so an out-of-scope path is always `opportunistic`, never `discovered` or `misunderstood`.
