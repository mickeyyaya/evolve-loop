# Comment history: `acs/cycle1455`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1455/predicates_test.go:3` — above `package cycle1455`

```text
// Package cycle1455 materialises the cycle-1455 acceptance criteria for the one
// fleet-scoped todo-id pinned to this lane, `context-fill-telemetry-and-cap` —
// specifically its live follow-on defect `contextfill-ratio-over-100pct`
// (inbox 2026-08-12, weight 0.75, pipeline-repair).
//
// The defect: `tokenusage.ScanConfigRoot` sums EVERY assistant turn's
// Input+CacheRead+CacheWrite into one grand total (scanner.go:147-153), and
// `DefaultResolver` feeds that summed total into `FillPct` against a
// SINGLE-turn 200K effective window (defaultresolver.go:38). Each turn's own
// cache_read_input_tokens already carries that turn's whole prior context, so
// summing turn N with turn N+1 re-counts the same context once per turn. Live
// symptom, twice in one monitored wave: scout 566.9%, triage 114.3%.
//
// Predicate strategy — every predicate below EXERCISES the system (the cycle-85
// degenerate-predicate ban); not one greps source:
//
//   - 001 drives the real production resolver over a real multi-turn transcript
//     fixture and asserts the reading is the terminal/peak turn's, naming the
//     summed (190) and first-turn (35) wrong answers explicitly.
//   - 002 is the anti-overfit half: `Result.Usage` is the COST number and
//     summing turns is CORRECT for it. A fix that greens 001 by making the
//     scanner stop summing reds 002.
//   - 003 is the negative case: an honest single-turn overrun must stay
//     unclamped and legible at 120%, not summed to 175% and not flattened to
//     100% (fillpct.go's own documented promise).
//   - 004 is the anti-false-positive edge: three modest turns sum past the 60%
//     warn line while the real reading is 47% — the WARN must go silent.
//   - 005 is the sentinel edge: zero in-window turns means nothing OBSERVED the
//     context; reading that as a measured 0% makes the launch look permanently
//     empty.
//   - 006 shells ONE named package (never a `./...` sweep, per the
//     flaky-predicate-shape rules) to pin no-regression across the existing
//     scanner/fillpct/defaultresolver/apicover suites.
//
// Every fixture grows monotonically — real transcripts do, context only
// accumulates within a phase — so the terminal turn IS the peak turn and either
// extraction satisfies these predicates. What they rule out is the sum, the
// first turn, and the mean.
```
