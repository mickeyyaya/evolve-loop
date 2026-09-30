# Comment history: `acs/cycle976`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle976/predicates_test.go:3` — above `package cycle976`

```text
// Package cycle976 materialises the cycle-976 acceptance criteria for the two
// triage-committed top_n tasks (see scout-report.md / triage-report.md), both
// tracing to ONE wiring defect: Orchestrator.profileForModelRouting is a
// permanent nil-stub (cyclerun.go:711-713), so router.ClampPlanModelRouting's
// model-tier-envelope guard — floor, ceiling, AND the documented "universal
// floor" — never fires in the composed production path.
//
//   - wire-real-profiles-into-model-tier-envelope-guard: replace the nil-stub
//     with a real per-phase profile lookup so an out-of-envelope advisor tier
//     actually clamps end-to-end (TestC976_001, 003).
//   - universal-floor-wiring-proof-regression-test: prove the compiled
//     universalTierFloor fires through the real dispatch path, not just the
//     router unit boundary (TestC976_002).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574 precedent).
// Each predicate shells `go test -run` over the RED integration tests authored
// this cycle in internal/core (model_routing_envelope_wiring_test.go). None is a
// source-grep — every one exercises the system under test (a full RunCycle over
// a real Orchestrator with on-disk .evolve/profiles fixtures) and asserts on the
// tier the build phase is actually dispatched with. RED now: the nil-stub leaves
// the advisor tier unclamped. GREEN once Builder wires the real lookup.
```
