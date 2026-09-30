# Comment history: `acs/cycle1073`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1073/predicates_test.go:3` — above `package cycle1073`

```text
// Package cycle1073 materialises the cycle-1073 acceptance criteria for the
// single fleet-scoped task pinned to this lane:
//
//	tdd-topn-scope-gate → convert tddScopeGate.check's case-2 (non-empty
//	committed ## top_n + an authored slug with zero overlap) from a fatal
//	block to the advisory label-drift pattern topNBindingGate already uses
//	(commit cbd088a1, #348), while KEEPING case-1 (empty top_n + a non-empty
//	authored set) fatal.
//
// Why this cycle exists. #348 converted the build->audit gate after two
// recorded false rejections (cycles 916, 1012) discarded CORRECT work whose
// report merely labelled the committed task differently — two LLM-authored
// strings compared for exact equality. The sibling tddScopeGate, which guards
// the triage->TDD transition one phase EARLIER, was not touched by that fix and
// still hard-blocks on the identical comparison. Case 1 is genuinely different
// (there is no committed item the authored files could be a relabelling of) and
// must stay fatal — so a blanket "never block" rewrite is a regression, not a
// fix, and predicate 002 exists to catch exactly that overcorrection.
//
// Predicate strategy — every predicate EXERCISES the gate by running the real
// unit tests that drive tddScopeGate.check directly (white-box, same package),
// never a source-grep of gate.go (the cycle-85 degenerate-predicate ban): a
// magic string added to the source cannot green any of these, only a change to
// the value check() actually returns can.
//
//   - 001 (crux, RED now) — label drift returns block=false with a reason that
//     names the drift and both slug sets.
//   - 002 (negative axis, GREEN now — anti-overcorrection) — an authored set
//     under an EMPTY top_n still returns block=true.
//   - 003 (edge/fail-open axis + no-regression) — the whole topngate package
//     suite is green, covering the six ambiguity cases (missing reports, empty
//     authored set, unparseable claim) and the untouched build-side gate.
//   - 004 — `go vet` is clean on the touched package (the AC pins the exported
//     signature of check(): only the returned bool and string content change).
```
