# Comment history: `acs/cycle1290`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1290/predicates_test.go:3` — above `package cycle1290`

```text
// Package cycle1290 materialises the cycle-1290 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `continuation-defect-ledger`,
// third hop of the 1285 → 1287 → 1290 continuation chain):
//
//   - faillearn-publish-mode-parity              → cycle-1287 audit defects[0] (F1,
//     MEDIUM): the failure floor publishes its own artifacts at 0600 while the rest
//     of the runtime publishes 0644, and nothing pins the mode.
//   - faillearn-inbox-failure-preserves-diagnosis → the residual the 1287 landing
//     note named rather than closed: a disk-level inbox failure yields ZERO
//     artifacts, so the diagnosis dies with the queue write.
//
// Predicate strategy. Predicates 001/002 drive the production entry point
// (faillearn.WriteArtifacts) directly from this package and assert on the emitted
// artifacts' MODE and CONTENT — so they are immune to the two cheapest gaming
// moves at once: deleting the in-package unit tests, and asserting `err == nil`
// without ever stat-ing anything. 003/004 then require those in-package tests to
// be tree-resident and executing (the cycle-1285 lesson: a red reproducer minted
// and abandoned in the same cycle protects nothing), and require the pre-existing
// transactional invariants to still pass UNMODIFIED — greening 002 by weakening
// inbox_transactional_test.go is the fix being wrong, not the contract being met.
// Subprocess predicates run ONE named package under an explicit -run expression
// with per-name PASS accounting, per the flaky-predicate-shape rules.
```

### `go/acs/cycle1290/predicates_test.go:175` — above `func TestC1290_003_TheRegressionPinsAreTreeResidentAndExecuting(t *testing.T) {`

```text
// TestC1290_003_TheRegressionPinsAreTreeResidentAndExecuting requires this cycle's
// reproducers to survive as tests in the package they protect. The cycle-1285
// lesson this closes: a red reproducer minted and abandoned inside the same cycle
// leaves the defect free to return the moment the predicate package ages out of
// the ACS lane (cycle predicates run for one cycle; package tests run forever).
```
