# Comment history: `acs/cycle1271`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1271/predicates_test.go:3` — above `package cycle1271`

```text
// Package cycle1271 materialises the cycle-1271 acceptance criteria for the two
// triage-committed top_n tasks (see scout-report.md / triage-report.md). Both
// close the FIRST of the follow-ups internal/contextfill's own header comment
// defers: the pure fill-ratio derivation exists but nothing imports it, so no
// cycle has ever recorded how full a phase's context window got.
//
//   - wire-contextfill-into-phasetiming-entry: phasetiming.Entry gains
//     ContextFillRatio/ContextWindowHot, populated at the ADR-0044 C1 chokepoint
//     (core.recordPhaseOutcome) from the tier the phase actually ran at, and
//     degrading to zero — never a fabricated ratio — when no tier is resolvable
//     (TestC1271_001, 002).
//   - contextfill-rollup-hot-phase-summary: phasetiming.Summary gains
//     HotPhaseCount/HotPhases, the cycle-level twin of the existing token rollup
//     (TestC1271_003).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-976 precedent). Each
// predicate shells `go test -run` over the RED tests authored this cycle in the
// two production packages; none is a source-grep (the cycle-85 degenerate-
// predicate ban). 002 is the WIRING PROOF — it drives the real production
// chokepoint (*Orchestrator).recordPhaseOutcome, the sole writer of
// phase-timing.json, so a derivation that exists only in a helper nobody calls
// stays RED. 005 asserts the scope constraint on the real build graph.
//
// RED now: Entry/Summary carry no such fields, so both target packages fail to
// COMPILE — the intended RED signal. GREEN once Builder adds the fields and the
// chokepoint derivation.
```

### `go/acs/cycle1271/predicates_test.go:78` — above `func TestC1271_002_ChokepointDerivesFillFromRealDispatch(t *testing.T) {`

```text
// TestC1271_002_ChokepointDerivesFillFromRealDispatch — task-1 WIRING PROOF.
// The derivation must run inside the real ADR-0044 C1 chokepoint,
// (*Orchestrator).recordPhaseOutcome — the sole writer of phase-timing.json —
// producing exactly contextfill.FillRatio for a tier-resolvable phase, NOT
// flagging a cold phase hot, and leaving both fields zero when no canonical tier
// is resolvable (concrete model id, empty provenance, unknown tier). A
// contextfill call reachable only from a test would leave this RED.
```
