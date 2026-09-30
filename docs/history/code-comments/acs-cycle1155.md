# Comment history: `acs/cycle1155`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1155/predicates_test.go:3` — above `package cycle1155`

```text
// Package cycle1155 materialises the cycle-1155 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	replan-rejections-telemetry — wire router.ValidatePlan into the post-scout
//	re-plan path (internal/core/cyclerun_replan.go) so unknown-phase rejections in
//	a RE-PLAN are recorded, and fix recordPlanRejections
//	(internal/core/decision_branch.go) so a second call in the same cycle
//	ACCUMULATES per plan-kind instead of overwriting the upfront record.
//
// Why this is the cycle's bar. The enforcement half already shipped: the
// integrity-floor clamp drops unknown-phase entries on BOTH the upfront and the
// re-plan path (router/floor.go dropUnknownPhases). The telemetry half did not —
// router.ValidatePlan has exactly one call site (core/cyclerun.go, upfront plan),
// so an advisor-hallucinated phase in a re-plan is dropped SILENTLY with zero
// forensic trail, which is the exact failure dropUnknownPhases' doc comment cites
// (cycles 1151, 1152). And recordPlanRejections writes advisor-rejections.json
// unconditionally, so naively adding the second call site would DESTROY the
// upfront record — the accumulation half is not optional polish, it is what makes
// the wiring safe.
//
// Predicate strategy — every predicate EXERCISES the system (cycle-85
// degenerate-predicate ban): each shells `go test` on a behavioural test in
// internal/core that drives the real cycleRun.postScoutReplan() against a real
// workspace and reads the real emitted telemetry artifacts. None of them greps
// production source for a magic string, so adding the string `ValidatePlan` to
// cyclerun_replan.go without a working record cannot pass any of them.
//
// Asserting on the `--- PASS: <name>` line rather than the exit code is load
// bearing: `go test -run` on a pattern matching NOTHING exits 0 with "no tests to
// run", so a deleted or renamed binding test would otherwise false-GREEN.
```
