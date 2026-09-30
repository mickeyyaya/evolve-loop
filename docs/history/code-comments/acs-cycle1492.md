# Comment history: `acs/cycle1492`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1492/predicates_test.go:3` — above `package cycle1492`

```text
// Package cycle1492 materialises the acceptance criteria for the single
// fleet-scoped item pinned to this lane: `verdict-cache-fresh-base-collision`
// (triage top_n: verdict-cache-fresh-base-guard, verdict-cache-post-build-regression).
//
// State of the tree at RED (measured, not assumed):
//
//   - The fresh-base guard itself is already implemented and behaviourally
//     GREEN. `verdictcache.ProbeEligible` is the single-sourced predicate; the
//     pre-loop ADR-0048 shadow probe (orchestrator.go) and the audit-binding Put
//     (phase_bindings.go) both route through it, and the integration oracles
//     TestVerdictCacheCollisionRegression / TestVerdictCacheProbeEligibilityWiring
//     pass. Predicates 001 and 002 pin that behaviour and are PRE-EXISTING GREEN
//     — they are frozen anti-regression contracts, not work items. Their value is
//     adversarial: 002 is the anti-gaming negative that fails if the Builder
//     "fixes" the collision by disabling verdict-cache reuse wholesale.
//
//   - What is RED is the lane's ability to SHIP the guard. Cycle 1488 produced
//     this exact code and still FAILed with `EGPS: red_count=1`: its own
//     predicate TestC1488_003 asserts the retired inline comparison is gone by
//     calling acsassert.FileContains inside a negation. FileContains reports the
//     miss through tb.Errorf, so the predicate fails precisely BECAUSE the fix
//     landed. Predicate 003 is that RED, and its fix is the one-symbol swap to
//     acsassert.FileNotContains — the API that exists for negative assertions.
//
// Predicate strategy (cycle-85 degenerate-predicate ban): every predicate below
// either runs the system under test as a subprocess and asserts on its exit
// code, or calls the production function over real git-derived tree identities.
// No load-bearing source grep appears anywhere in this file, and no
// acsassert.FileContains call is used in a negated position.
```

### `go/acs/cycle1492/predicates_test.go:126` — above `func TestC1492_002_ChangedWorktreeStaysEligible(t *testing.T) {`

```text
// TestC1492_002_ChangedWorktreeStaysEligible materialises AC-1 of
// verdict-cache-post-build-regression, and is this cycle's anti-gaming negative.
// The cheapest way to make 001 pass is to disable verdict-cache lookups
// outright; that would also destroy the genuine re-land reuse ADR-0048 exists
// for. A changed worktree must therefore remain DISTINGUISHABLE from its base
// and must still reach the advisory lookup (skipped=false, matched=true on a
// seeded hit), and the orchestrator's decision must still be derived from the
// shared predicate rather than a re-introduced local copy.
```

### `go/acs/cycle1492/predicates_test.go:174` — above `func TestC1492_003_LaneACSSuiteIsGreen(t *testing.T) {`

```text
// TestC1492_003_LaneACSSuiteIsGreen is the RED that actually blocks this lane.
//
// The guard shipped in cycle 1488's worktree, yet the cycle FAILed on
// `EGPS: red_count=1` — the lane's own carried predicate suite is red, so the
// fix cannot land, and cycles 1488/1492 keep re-deriving the same code. The red
// predicate is TestC1488_003, which asserts the retired inline comparison
// `worktreeTree == headTree` is absent by calling acsassert.FileContains inside
// an `if` negation. FileContains signals a miss through tb.Errorf, so the
// predicate reports FAIL exactly when the criterion HOLDS — an inverted oracle.
// acsassert.FileNotContains is the assertion for that shape.
//
// This predicate runs the carried suite and requires it GREEN. It is behavioural
// (subprocess exit code of the real suite) and it is the ship precondition for
// both top_n tasks: neither can reach main while red_count > 0.
```
