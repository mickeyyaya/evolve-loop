# Comment history: `acs/cycle1241`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1241/predicates_test.go:3` — above `package cycle1241`

```text
// Package cycle1241 materialises the cycle-1241 acceptance criteria for the
// fleet-scoped todo `tdd-structural-test-reachability-probe`, committed by
// triage as two tasks:
//
//   - acs-current-cycle-scope-reachability-probe — a new
//     `go/internal/acssuite/reachability_test.go` whose
//     `TestCurrentCycleScopeReachable` proves the current-cycle ACS scope is
//     reachable END TO END: `goLanePatterns(moduleDir, cycle)` selects it AND a
//     real `go test` against that pattern produces real test events. It closes
//     the gap between `TestAllACSPredicatesAreTagged` (tagging only) and the
//     existing `TestGoLanePatterns_*` unit cases (pattern strings in isolation).
//   - acs-scope-cyclenum-drift-adversarial-case — a new
//     `TestGoLanePatterns_CycleNumberDrift` in
//     `go/internal/acssuite/acssuite_adversarial_test.go` asserting that a
//     cycle-number/dir-name mismatch EXCLUDES the scope rather than silently
//     matching it loosely.
//
// Provenance: cycle-1222 was a prior attempt at this same todo. It left a RED
// predicate suite (`go/acs/cycle1222/predicates_test.go`) and an EGPS failure
// recorded at `.evolve/runs/cycle-1222/audit-fail-reason.json`
// (`red_count=3`). That file is the authoritative acceptance spec and must go
// GREEN **unmodified** — predicate 005 below is what proves that, because the
// ACS gate runs only `./acs/cycle1241` this cycle and would never otherwise
// execute the cycle-1222 scope.
//
// Predicate strategy — every predicate drives the REAL `go` toolchain against
// the REAL package and asserts on observed `go test -v` events. None asserts
// that a string is present in a source file (the cycle-85 degenerate-predicate
// ban): a no-op could satisfy that by pasting a func name into a comment.
//
//   - 001 the reachability guard must RUN and PASS.
//   - 002 the drift guard must RUN and PASS.
//   - 003 NEGATIVE / anti-vacuity control: a test name that must never exist
//     has to produce "no tests to run" and zero PASS events. If 003 is RED the
//     PASS-line assertions in 001/002/004/005 are not evidence of anything.
//   - 004 WIRING proof: both guards must be reached by the plain, untagged
//     `go test ./internal/acssuite` that CI actually runs — no `-tags`, no
//     `-run`. A guard that inherits `//go:build acs` from the surrounding ACS
//     convention would pass 001/002 under an explicit `-run` while being
//     invisible to every CI run. A guard nothing reaches is dead code.
//   - 005 the cycle-1222 predicate suite goes fully GREEN under `-tags acs`,
//     with its own file unmodified (asserted via `git status --porcelain`).
```

### `go/acs/cycle1241/predicates_test.go:70` — above `const priorPredicatePkg = "./acs/cycle1222"`

```text
// priorPredicatePkg is the cycle-1222 predicate scope that must go GREEN.
```

### `go/acs/cycle1241/predicates_test.go:245` — above `func TestC1241_005_PriorCycle1222PredicatesGoGreenUnmodified(t *testing.T) {`

```text
// TestC1241_005_PriorCycle1222PredicatesGoGreenUnmodified closes the cycle's
// stated acceptance bar: the surviving cycle-1222 predicate suite — the
// authoritative spec for this todo, which recorded red_count=3 — must go fully
// GREEN, and it must do so WITHOUT being edited (editing the spec to match the
// implementation is the classic way to fake this).
//
// The ACS gate runs only `./acs/cycle1241` this cycle, so nothing else executes
// the cycle-1222 scope; without this predicate the "cycle-1222 goes GREEN"
// criterion would ship unverified.
```
