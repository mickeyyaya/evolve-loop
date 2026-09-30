# Comment history: `acs/cycle1222`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1222/predicates_test.go:3` — above `package cycle1222`

```text
// Package cycle1222 encodes the cycle-1222 ACS predicates for the fleet-scoped
// todo `tdd-structural-test-reachability-probe`, materialised by triage as two
// committed tasks:
//
//   - acs-current-cycle-scope-reachability-probe — a new
//     `go/internal/acssuite/reachability_test.go` whose
//     `TestCurrentCycleScopeReachable` proves the current-cycle ACS scope is
//     reachable END TO END (goLanePatterns selects it AND `go test -tags acs`
//     against it produces real test events), closing the gap between
//     `TestAllACSPredicatesAreTagged` (tagging only) and `TestGoLanePatterns_*`
//     (pattern logic in isolation).
//   - acs-scope-cyclenum-drift-adversarial-case — a new
//     `TestGoLanePatterns_CycleNumberDrift` in
//     `go/internal/acssuite/acssuite_adversarial_test.go` asserting a
//     cycle-number/dir-name mismatch EXCLUDES the scope rather than silently
//     matching it loosely.
//
// Both deliverables are tests, so every predicate here drives the real `go`
// toolchain against the real `go/internal/acssuite` package and asserts on the
// observed `go test -v` events — never on the presence of a string in a source
// file (which a no-op could satisfy by pasting the func name into a comment).
//
// Predicate C1222-003 is the anti-vacuity control: it proves this file's own
// harness reports "no such test" as a failure, so the PASS-line assertions in
// 001/002/004 cannot pass vacuously.
```
