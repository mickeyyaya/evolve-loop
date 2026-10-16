# Comment history: `internal/contextfill`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/contextfill/apicover_named_test.go:11` — above `func TestAPICoverNamedExports(t *testing.T) {`

```text
// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) through the one path a consumer will
// actually take: resolve a tier's window, derive the fill ratio for a phase's
// recorded TokenUsage, classify it, and prove an unknown tier degrades to
// ErrInvalidWindow instead of a fabricated number.
```

### `go/internal/contextfill/contextfill.go:1` — above `package contextfill`

```text
// Package contextfill derives how full a phase's model context window got, from
// the token counts the loop ALREADY persists. Every phase records a
// cyclestate.TokenUsage (input / output / cache_read / cache_write) into
// <workspace>/phase-timing.json, but nothing today turns those absolute counts
// into the one number that says whether the phase was running out of room:
// occupancy over the model's context-window size.
//
// This package is the pure derivation only — a stdlib+cyclestate leaf. As of
// cycle-1271 it is wired into the durable telemetry record and NOWHERE else:
// internal/core's recordPhaseOutcome (the ADR-0044 C1 chokepoint) derives the
// ratio onto phasetiming.Entry, and phasetiming.Rollup summarises the hot
// phases through IsHot. Those two packages are the whole importer set — keep it
// that way. The off/advisory/enforce Stage dial and the advisory prompt hint
// remain deliberately DEFERRED to follow-up cycles.
```
