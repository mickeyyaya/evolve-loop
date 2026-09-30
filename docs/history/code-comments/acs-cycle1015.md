# Comment history: `acs/cycle1015`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/acs/cycle1015/predicates_test.go:3` — above `package cycle1015`

```text
// Package cycle1015 materialises the cycle-1015 acceptance criteria for the sole
// fleet-scoped item of this lane, surface-tripwire-in-tokens-report (inbox item
// telemetry-coverage-tripwire-nonclaude-success, weight 0.93). fleet_scope pins
// this lane to that one id, so per R9.3 no predicate binds to any other lane's
// work.
//
// Where cycle-1005 landed the ENGINE side (recordTokenUsage escalates a
// non-claude exit-0 >60s source=none launch to a "tripwire":true record in
// llm-calls.ndjson), cycle-1015 is the REPORT side: `evolve tokens report` must
// SURFACE that record to the operator. The load-bearing crux is the cycle-1007
// render-order regression — renderTripwires runs UNCONDITIONALLY before the
// empty-phases early return, so a cycle with tripwire hits but no phase-timing
// rows still prints the WARN instead of silently early-returning.
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban):
//
//   - 001–003 run the in-package behavioral tests (package main, go/cmd/evolve,
//     which drives runTokensReport end-to-end over a real temp .evolve/runs tree)
//     as a SUBPROCESS and require an explicit "--- PASS:" marker for each named
//     test. A bare exit-0 is rejected, so a renamed/skipped/deleted test cannot
//     green the gate.
//
// Adversarial axes (SKILL §6):
//   - SEMANTIC — 001 (surfaces + names CLI/agent/cycle in text AND json), 002
//     (the render-order crux: still surfaces when Phases is empty), 003 (F1
//     control-byte sanitisation) are distinct behaviors, not one restated.
//   - NEGATIVE — 004 requires the zero-tripwire cycle (claude baseline + a
//     quota-abort exit-85 short non-claude launch) to stay SILENT: an
//     always-on/no-op render that unconditionally prints TRIPWIRE fails it. This
//     is the strongest anti-no-op signal (a report that always warns "passes"
//     the surfacing tests but fails here).
//
// The in-package tests (cmd_tokens_test.go) are the RED contract for this
// behavioral surface; the subprocess makes them the cycle's audit-gating
// predicate so the surfacing contract survives beyond this cycle.
```
