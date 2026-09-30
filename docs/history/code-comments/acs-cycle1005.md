# Comment history: `acs/cycle1005`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1005/predicates_test.go:3` — above `package cycle1005`

```text
// Package cycle1005 materialises the cycle-1005 acceptance criteria for the sole
// fleet-scoped item of this lane, telemetry-coverage-tripwire-nonclaude-success.
// fleet_scope pins this lane to that one id, so per R9.3 no predicate binds to any
// other lane's work. Scout split the item into two triage-committed (top_n) tasks
// sharing one file (go/internal/bridge/engine.go) and one behavioral surface
// (recordTokenUsage): Task 1 telemetry-tripwire-nonclaude-exit0-warn and Task 2
// telemetry-tripwire-llm-calls-record.
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001–004 run the in-package behavioral tests (package bridge, which can reach
//     the deliberately-UNEXPORTED recordTokenUsage method) as a SUBPROCESS and
//     require an explicit "--- PASS:" marker for each named test. A bare exit-0 is
//     rejected, so a renamed/skipped/deleted test cannot green the gate. The
//     in-package tests (engine_tripwire_test.go) are the RED contract authored this
//     TDD phase; the subprocess makes them the cycle's audit-gating predicate.
//
// Adversarial axes (SKILL §6): NEGATIVE — 002 requires the quota-abort (exit 85,
// short), exit-0 sub-threshold, claude-baseline, and covered launches to stay
// silent; each is an input that must NOT trip. EDGE — 003 requires the tripwire to
// fire fail-open when the cycle is not derivable from the workspace path. SEMANTIC
// — 001 (fires on unmeasured success), 002 (four distinct silence reasons), 003
// (fail-open), and 004 (queryable ndjson field) are distinct behaviors, not one
// restated.
```
