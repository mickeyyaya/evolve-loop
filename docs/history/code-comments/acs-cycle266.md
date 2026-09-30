# Comment history: `acs/cycle266`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle266/predicates_test.go:3` — above `package cycle266`

```text
// Package cycle266 materializes the cycle-266 acceptance criteria for the
// `no-duplicate-phase-invariant` task: close the cycle-265 audit M1 WARN by
// turning the misnamed `TestInvariant_DuplicatePhaseRejected` (which only proved
// tolerance) into a real, behavior-grounded kernel-floor invariant.
//
//  1. add a `no-duplicate-phase` entry to `invariantChecks` in invariants.go
//     that calls t.Errorf when in.Plan.Entries repeats a Phase value;
//  2. rename the misleading test to `TestInvariant_DuplicatePhaseTolerated`
//     (its scenario genuinely asserts tolerance/determinism);
//  3. add `TestInvariant_NoDuplicatePhaseEnforcesUniqueness` with a positive
//     (unique plan → invariant silent) and a negative (duplicate plan → fires)
//     sub-case.
//
// These predicates are BEHAVIORAL (cycle-85 lesson): each RUNS the
// system-under-test — the `internal/routingtest` Go suite — as a subprocess and
// asserts on its real `go test -cover -v` output (top-level PASS lines, subtest
// PASS lines, coverage %, absence of FAIL). The load-bearing assertions exercise
// the actual invariant via the suite; any source-file checks are AUXILIARY
// anti-no-op guards, never the sole weight. The builder's job is production code
// (invariants.go) + the routingtest unit tests; these predicates gate it.
```

### `go/acs/cycle266/predicates_test.go:194` — above `func TestC266_005_CoverageAtLeast80(t *testing.T) {`

```text
// NOTE: coverage is 80.6% at the cycle-266 baseline, so this criterion is a
// MAINTAIN/regression guard and may report pre-existing GREEN at RED time.
```
