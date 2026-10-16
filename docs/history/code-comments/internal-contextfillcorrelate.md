# Comment history: `internal/contextfillcorrelate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d8

### `go/internal/contextfillcorrelate/apicover_named_test.go:14` — above `func TestAPICoverNamedExports(t *testing.T) {`

```text
// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) along the one path a consumer takes:
// Load the real corpus shape off disk, Correlate it into a Report, read the
// Bucket / VerdictSplit tallies back out, round-trip the report through the
// Bucket JSON codec the CLI's --json projection depends on, and render Markdown.
```

### `go/internal/contextfillcorrelate/contextfillcorrelate.go:1` — above `package contextfillcorrelate`

```text
// Package contextfillcorrelate joins the per-phase context-window fill ratio
// (internal/contextfill, persisted onto phasetiming.Entry.ContextFillRatio in
// cycle-1271) against the cycle's final verdict from its knowledge-base
// dossier. Parts (1) and (2) of the `context-fill-telemetry-and-cap` inbox item
// derive and persist the number; nothing before this package ASKED the question
// the number exists to answer — do cycles that ran hot fail more often?
//
// The package is a leaf: stdlib plus internal/contextfill (the one definition of
// the hot boundary) and internal/phasetiming (the one reader of the timing log).
// The derivation is not reimplemented here.
//
// Absent evidence is never reported as data. A cycle with no usable fill ratio,
// or with no final verdict, lands in Report.NoData — it is NEVER bucketed as a
// fabricated 0.0, which would manufacture exactly the correlation being
// measured. Conservation holds: CyclesJoined + len(NoData) equals the number of
// dossiers read.
```

### `go/internal/contextfillcorrelate/contextfillcorrelate_test.go:127` — above `body := []byte('{"cycle": 11, "final_verdict": "FAIL"}')`

```text
// Both bodies lie, claiming to be cycle 11.
```
