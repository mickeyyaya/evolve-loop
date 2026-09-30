# Comment history: `acs/cycle1014`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1014/predicates_test.go:3` — above `package cycle1014`

```text
// Package cycle1014 materialises the cycle-1014 acceptance criteria for the sole
// fleet-scoped item of this lane, tripwire-regression-lock (inbox
// telemetry-coverage-tripwire-nonclaude-success, weight 0.93). fleet_scope pins
// this lane to that one id, so per R9.3 no predicate binds to any other lane's
// work. Triage committed exactly one top_n task: add an explicitly-AC-named
// positive+negative report-level regression that locks in the already-landed
// telemetry-coverage tripwire (engine.go recordTokenUsage computes it; cycle-1013
// surfaced it in `evolve tokens report`). This cycle adds no production code —
// it consolidates the AC1/AC2/AC3 contract into a single named regression so a
// future hostile edit to the render path (the exact cycle-1007 render-order
// defect) is caught by a test whose name states the contract.
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban). Each
// predicate runs the Builder's in-package behavioral RED tests (package main,
// which reaches the unexported runTokensReport/renderTokensReport entry points)
// as a SUBPROCESS and requires an explicit "--- PASS:" marker per named test. A
// bare exit-0 is rejected, so a renamed/skipped/deleted/never-written test cannot
// green the gate (this is what makes RED real today: neither Builder test exists
// yet, so no PASS marker is emitted). The `-run` pattern is anchored to only this
// task's two tests, so the package's unrelated pre-existing red test
// (TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport) cannot block — and
// cannot mask — this task's own scoped run.
//
// Adversarial axes (adversarial-testing SKILL §6): POSITIVE/SEMANTIC — 001 requires
// the fire case (non-claude cli, exit 0, >60s, source=none) to surface a TRIPWIRE
// line that names the CLI, agent, AND cycle together (AC1+AC2). NEGATIVE — 002
// requires a matrix of three distinct false-positive vectors (a claude-baseline
// launch, a sub-threshold-duration launch, and a quota-abort exit-85 launch) to
// stay silent (AC3), rejecting an always-on impl. The two named tests are distinct
// behaviors, not one restated.
```
