# Comment history: `acs/cycle1555`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1555/predicates_test.go:3` — above `package cycle1555`

```text
// Package cycle1555 materialises the cycle-1555 acceptance criteria for the
// two fleet-scoped tasks pinned to this lane (live inbox record
// `red-first-deliverable-reds-main`):
//
//   - ship-added-test-red-gate          → a newly added Go test that fails in
//     the lane worktree must block ship with CodeRepoContractGate; selection
//     is bounded to newly-added `_test.go` files (modified/non-test excluded).
//   - ship-added-test-production-proof  → the block fires through the real
//     production caller (Phase.runNative), before any git/ship action, while
//     an honest `t.Skip` reproducer stays non-blocking.
//
// The live defect. Three lanes reached main with an intentionally red
// reproduction test because the existing repo-contract scanner
// (go/internal/phases/ship/repocontract.go) only runs FOUR fixed guard
// packages — it has no consumer for an arbitrary new test file added by the
// shipping diff itself. `runRepoContractGate`/`Phase.runNative` are the sole
// pre-push boundary (repocontract.go, ship.go:144); a detector that only
// exists as a helper and is never reached from there is the exact class of
// unconsumed signal this cycle's inbox record calls out.
//
// Predicate strategy — behavioral, never source-grep (the cycle-85
// degenerate-predicate ban). Each predicate DRIVES the system under test by
// running the named Go unit test in ONE named package with `-run` narrowing
// (per the flaky-predicate-shape rules: no `./...` sweeps, no wall-clock
// bounds, no literal PIDs, cmd.Dir always set explicitly) and asserting the
// `--- PASS: <TestName>` line is present. The PASS-line assertion is the
// anti-vacuous guard: `go test -run '^TestDoesNotExist$'` exits 0 with
// "no tests to run", so exit-code-only checking would pass on an EMPTY repo.
//
// Predicate map:
//
//	001 — a newly added failing test blocks ship w/ CodeRepoContractGate (T1 AC1)
//	002 — selection ignores modified tests, non-test files, empty diffs (T1 AC2/AC3)
//	003 — the four pre-existing gate tests are still GREEN (anti-weakening)
//	004 — WIRING PROOF: Phase.runNative stops before git/ship on a real added red test (T2 AC1)
//	005 — a `t.Skip`-ped newly added test does NOT block at the gate (T2 AC2)
```
