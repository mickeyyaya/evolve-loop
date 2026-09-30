# Comment history: `acs/cycle1267`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1267/predicates_test.go:3` — above `package cycle1267`

```text
// Package cycle1267 materialises the cycle-1267 acceptance criteria for the two
// tasks triage committed to this lane:
//
//   - scope-test-amplification-context            (inbox test-amplification-context-scope, w=0.89)
//   - verify-infra-teardown-predicate-consolidation (inbox infra-teardown-predicate-single-source, w=0.86)
//
// Scope note (read before judging these predicates). Both items arrived at this
// cycle PARTIALLY LANDED, carried in by the ADR-0076 continuation snapshot
// 71df9088. The TDD phase verified the live tree rather than trusting the scout
// report and pinned only what is genuinely open:
//
//	Task 1 — CoveringTests + its artifact + the phase-spec input + the persona
//	         instruction all exist and are green. Still MISSING: the inbox
//	         item's "+ direct reverse-import test packages" half (no
//	         reverse-dependency seam exists in the module at all — the cycle-1267
//	         fault-localization report cites changedpkgs.ImporterClosure, which
//	         does NOT exist), and the "no silent caps" half (the cap writes a
//	         note into the artifact but nothing reaches the operator's log).
//	Task 2 — verification-only by triage's own decision. The union-uniqueness
//	         proof already exists; the item's acceptance criterion "NO
//	         timeout-only or transient-only site was incorrectly widened" had no
//	         pin for the TIMEOUT-only half. 004/005 are that pin plus the
//	         existing uniqueness scan as a regression guard, and they are
//	         PRE-EXISTING GREEN by design: a behaviour-preserving task's
//	         deliverable is a durable proof, not a diff.
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987/1255
// precedent). Each predicate shells `go test -run '^(names)$' -v -count=1` over
// exactly ONE named package and requires a `--- PASS: <name>` line per test.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN.
//   - A source-grep predicate (FileContains over a .go file) is deliberately
//     avoided — it passes the moment the magic string appears, fix or no fix
//     (the cycle-85 degenerate-predicate ban).
//   - Flaky-predicate-shape rules: every invocation names EXACTLY ONE package,
//     never ./..., and the two that name ./internal/core (a known 40s+ suite)
//     are narrowed with -run, which the rule explicitly permits. No wall-clock
//     bounds, no literal PIDs, no bare `git`, no un-reaped load generators.
```

### `go/acs/cycle1267/predicates_test.go:100` — above `func TestC1267_002_DirectImportersReachableFromProduction(t *testing.T) {`

```text
// TestC1267_002_DirectImportersReachableFromProduction — AC6, the WIRING proof.
// The widening must be called from a real non-test file in the go/ module
// (resolved from the parsed import graph, not a grep). A seam whose only caller
// is a test injects nothing into test-amplification's context and saves zero
// tokens — the exact dead-code shape the cycle-1255 CoveringTests contract had
// to pin for the same reason.
```

### `go/acs/cycle1267/predicates_test.go:156` — above `func TestC1267_006_CoveringTestsContractNotRegressedByWidening(t *testing.T) {`

```text
// TestC1267_006_CoveringTestsContractNotRegressedByWidening — the anti-regression
// AC. The widening is ADDITIVE: CoveringTests keeps its cycle-1255 contract
// (changed packages → their own _test.go files, both pattern forms, deduped,
// fail-open on the module-wide sweep). "Fixing" the missing reverse-import half
// by making CoveringTests itself walk the whole module — the blind-widen shape
// that would re-inflate the very context this task shrinks — turns this RED.
```
