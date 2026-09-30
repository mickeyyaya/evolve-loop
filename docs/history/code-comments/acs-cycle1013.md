# Comment history: `acs/cycle1013`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/acs/cycle1013/predicates_test.go:3` — above `package cycle1013`

```text
// Package cycle1013 materialises the cycle-1013 acceptance criteria for the sole
// fleet-scoped item of this lane, surface-tripwire-in-tokens-report (inbox
// telemetry-coverage-tripwire-nonclaude-success, weight 0.93). fleet_scope pins
// this lane to that one id, so per R9.3 no predicate binds to any other lane's
// work. Scout committed one top_n task: read the engine's llm-calls.ndjson
// tripwire records inside `evolve tokens report` and surface them in the
// plain-text and --json output (the engine side, engine.go recordTokenUsage, is
// already done and unchanged; the gap is entirely cmd/evolve/cmd_tokens.go).
//
// Predicate strategy — every predicate EXERCISES the system under test, never a
// source-grep of production code (the cycle-85 degenerate-predicate ban). Each
// predicate runs the in-package behavioral RED tests (package main, which reaches
// the unexported runTokensReport/renderTokensReport entry points) as a SUBPROCESS
// and requires an explicit "--- PASS:" marker per named test. A bare exit-0 is
// rejected, so a renamed/skipped/deleted test cannot green the gate. The `-run`
// pattern is anchored to only this task's four tests, so the package's unrelated
// pre-existing red test (TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport)
// cannot block — and cannot mask — this task's own scoped run.
//
// Adversarial axes (adversarial-testing SKILL §6): NEGATIVE — 003 requires a
// cycle whose launches are all tripwire:false (claude baseline + quota-abort
// exit-85 short) to stay silent; the input must NOT trip, rejecting an always-on
// impl. EDGE — 004 requires ANSI/control bytes in the record's CLI/agent/phase
// fields to be escaped/stripped from the TTY (F1 injection defense). SEMANTIC —
// 001 (surface in text+json, naming cli/agent/cycle), 002 (survive the empty-
// phases render-order defect), 003 (silence on non-signals), 004 (sanitize) are
// distinct behaviors, not one restated.
```
