# Comment history: `acs/cycle1188`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1188/predicates_test.go:3` — above `package cycle1188`

```text
// Package cycle1188 materialises the cycle-1188 acceptance criteria for the one
// fleet-scoped task pinned to this lane: close-evaluate-batch-retry-parity-inbox.
//
// What this cycle is (and is NOT). The underlying design defect — the
// evaluate-batch retry loop having silently diverged from the main dispatch loop
// (missing optionalInfraSkip + postShipObserverSkip) — was ALREADY fixed in
// cycle-1166 and closed out at the repo-root inbox layer in cycle-1185. The
// shared retry core (retryPhaseRunner/retryOpts) is live in this worktree and its
// parity tests are green here BEFORE any change this cycle. So this cycle is pure
// state/paperwork reconciliation: this worktree carries its own isolated
// `.evolve/` snapshot whose inbox still holds the item at the OPEN root and whose
// state.json has no `evaluatedTasks` key at all.
//
// Because the code is already correct, the load-bearing predicates are 001–003
// (the applied state transition), and 004 is a REGRESSION PIN, not a RED target.
// Baseline measured at TDD time, in this worktree:
//
//   - `.evolve/inbox/2026-07-08T00-50-00Z-evaluate-batch-retry-parity.json` PRESENT
//   - no `.evolve/inbox/processed/*/` record for the item (only three
//     interactive-2026-06-* dirs exist)
//   - state.json keys: no `evaluatedTasks` at all (verified: key absent)
//   - `go test ./internal/core/... -run 'RetryOpts|RetryParity|DispatchRunnerWithRetry'`
//     → ok (pre-existing GREEN)
//
// Predicate strategy — every predicate reads REAL runtime state or executes the
// real system; none greps production source for a magic string (the cycle-85
// degenerate-predicate ban):
//
//   - 001 parses the emitted processed record as JSON and asserts it is the
//     genuine item that was MOVED (original id/created_at/weight/files survive),
//     not a hollow touch-file bearing the right name.
//   - 002 parses the LIVE state.json and asserts an evaluatedTasks completion
//     record exists with decision=="completed" — and that the rest of the file
//     survived (carryoverTodos intact), so a clobbering rewrite cannot pass.
//   - 003 is the NEGATIVE predicate: the item must be ABSENT from the open-inbox
//     root. A copy-instead-of-move fails this while passing 001.
//   - 004 EXECUTES the retry-parity suite as a subprocess and asserts exit 0 —
//     the paperwork closeout must not disturb the shipped fix it is closing.
//
// Root resolution mirrors the cycle-998 predicates: acsassert.RepoRoot resolves
// to this worktree (where Builder writes, per worktree isolation), so the
// artifacts read here are the ones Builder is required to produce.
```

### `go/acs/cycle1188/predicates_test.go:69` — above `func findProcessedRecord(t *testing.T, root string) string {`

```text
// findProcessedRecord returns the path of the processed record for the item,
// searching every `.evolve/inbox/processed/<dir>/` subdirectory. Builder may
// file it under cycle-1188/ or consumed/ or any sibling; the acceptance bar is
// "filed under processed", not one exact directory name. Returns "" if absent.
//
// The repo-root precedent (cycle-1185) prefixes the basename with a short hash
// (`c4e56157-<basename>`), so the match is a suffix match, not equality.
```
