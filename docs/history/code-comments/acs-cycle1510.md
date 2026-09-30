# Comment history: `acs/cycle1510`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1510/predicates_test.go:3` — above `package cycle1510`

```text
// Package cycle1510 materialises the cycle-1510 acceptance criteria for the two
// fleet-scoped carryover tasks pinned to this lane, both of which FAILED the
// cycle-1508 audit and were re-scoped by scout against the CURRENT tree:
//
//   - contract-correction-hash-freshness-classification → add the unexported
//     core.contractArtifactDetermined allowlist classifier (no consumer)
//   - contract-correction-verbatim-output-fidelity      → regression-lock
//     composeCorrection's verbatim inclusion + enumerate blocking-gate
//     amendments in build-report.md
//
// Predicate strategy. Both subjects are UNEXPORTED functions in
// internal/core, so a predicate in this package cannot call them directly.
// Each behavioural predicate therefore RUNS the real in-package unit tests as a
// subprocess and asserts on the exit code — the system under test genuinely
// executes, and adding a magic string to a source file cannot make it pass
// (the cycle-85 degenerate-predicate ban). Every invocation is narrowed with
// `-run` to specific test names and pinned to ONE named package via `go -C`,
// per the flaky-predicate-shape rules: no `./...` sweep, no wall-clock bound,
// no literal PID, no bare `git` resolving cwd from the process.
//
// 001/002 carry task 1 (positive allowlist + the fail-closed/negative axis);
// 003/004 carry task 2 (verbatim lock + the "production code UNCHANGED"
// constraint that keeps this task honest about being verification, not a
// change); 005 carries the build-report amendment-enumeration contract that
// closes inst-L1508b's process defect.
```

### `go/acs/cycle1510/predicates_test.go:63` — above `func workspaceDir(t *testing.T) string {`

```text
// workspaceDir resolves this cycle's run workspace. The predicate executes from
// the lane worktree, but phase reports are written to the MAIN tree's
// .evolve/runs/cycle-1510. Both are checked, worktree first, so the predicate
// works under a lane run and a console run alike.
```
