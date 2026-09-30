# Comment history: `acs/cycle1513`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1513/predicates_test.go:3` — above `package cycle1513`

```text
// Package cycle1513 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `contract-correction-verbatim-output-fidelity`: reland the
// cycle-1510 salvage regression lock for `composeCorrection`
// (go/internal/core/retry_backoff_test.go) with ZERO product-code change.
//
// Honest framing, stated up front (inst-L1508b, the lesson this whole todo
// carries): the property under lock — the rejection reason survives
// `composeCorrection` byte-for-byte — ALREADY HOLDS on unmodified source
// (retry_backoff.go:12-17 concatenates with `+`). The deliverable is therefore
// the LOCK, not a behaviour change, and these predicates are PRE-EXISTING GREEN
// in this worktree: the salvage snapshot commit 893ebcd2 is already an ancestor
// of this lane's branch, so the test file and its eval are already tracked here.
// They are absent from origin/main, so the cycle's ship is what actually lands
// them — which is exactly the state these predicates pin. See test-report.md
// §RED Run Output for the executed evidence.
//
// Predicate strategy — every load-bearing assertion runs a real subprocess and
// asserts on its exit code / output (the cycle-85 grep-only ban):
//
//   - 001 asserts the locked test file is present AND git-TRACKED (disk presence
//     alone passes for a gitignored file silently dropped at ship, cycle-93).
//   - 002 is the crux: it RUNS the three locked tests and asserts each named
//     test — and every subtest of the verbatim table — actually reported PASS.
//     A stub file with the right name, or a `-run` pattern matching nothing,
//     fails this; exit 0 alone is not accepted as evidence.
//   - 003 is the anti-tautology guard: `git diff origin/main` on the PRODUCT
//     file must be empty. A "fix" that edits composeCorrection to satisfy its
//     own lock is precisely the cycle-1508 defect this task exists to close.
//   - 004 asserts the landed file is gofmt-clean.
//
// Roots: RepoRoot is the worktree (where the lock lands and where the ship
// commit is taken from), which is the correct root for all four.
```

### `go/acs/cycle1513/predicates_test.go:114` — above `func TestC1513_003_ProductFileUntouched(t *testing.T) {`

```text
// TestC1513_003_ProductFileUntouched pins AC4, the anti-tautology guard: the
// locked product file must be byte-identical to origin/main. This is the
// cycle-1508 defect in predicate form — a lock is worthless if the same commit
// is free to move the thing it locks.
```
