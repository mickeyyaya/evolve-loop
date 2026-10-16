# Comment history: `internal/cyclecost`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d3

### `go/internal/cyclecost/cyclecost.go:1` — above `package cyclecost`

```text
// Package cyclecost computes per-phase + per-cycle cost from the unified
// phasestream event stream (ADR-0020). Walks <workspace>/*-events.ndjson
// files, extracts the last kind==result envelope from each, and sums
// cost + token counts into a per-phase + per-cycle summary.
//
// Used by cmd_loop after each cycle to accumulate the batch cost total
// for display-only telemetry (`total_cost_usd` in loop output). Cost no
// longer gates anything — the token-budget cost gates were removed.
//
// Source format: each line is one normalized envelope. The result
// envelope carries data.cost_usd plus data.tokens.{in,out,cache_r,
// cache_c} (the shape phasestream.Classifier emits). Raw-log parsing
// quirks (legacy single-blob JSON, mid-stream malformed events) are
// handled upstream by the normalizer, so there is no raw fallback here.
//
// The emitted Summary/PhaseCost JSON shape is frozen — it mirrors what
// show-cycle-cost.sh --json produced, so downstream tooling that grep'd
// that output keeps working byte-for-byte.
//
// Zero-cost paths are not errors. A missing workspace IS an error
// — caller already failed to enter the cycle, summing zero would
// mask the bug.
```

### `go/internal/cyclecost/cyclecost.go:112` — above `for _, pc := range sidecars {`

```text
// S7 (token-telemetry): the per-phase usage sidecar (ADR-0044 C1,
// core.recordPhaseOutcome) is written at the single C1 recording
// chokepoint, so it survives abort paths that never reach the
// tmux-driver's events.ndjson normalizer. It fills in phases missing
// an events.ndjson entirely (the abort case); a phase that DOES have
// an events.ndjson keeps that value — events.ndjson is the richer,
// per-line-verified source when both exist.
```

### `go/internal/cyclecost/cyclecost.go:255` — above `const usageSidecarSuffix = "-usage.json"`

```text
// usageSidecarSuffix is the per-phase filename suffix core.recordPhaseOutcome
// writes at the ADR-0044 C1 chokepoint ("<phase>-usage.json").
```

### `go/internal/cyclecost/cyclecost_test.go:65` — above `func writeUsageSidecar(t *testing.T, workspace, phase string, cost float64, in, out, cacheR, cacheW int64) {`

```text
// writeUsageSidecar seeds a <phase>-usage.json sidecar in workspace, matching
// the shape core.recordPhaseOutcome writes at the ADR-0044 C1 chokepoint.
```

### `go/internal/cyclecost/cyclecost_test.go:90` — above `func TestCycleCost_PrefersSidecarOverEvents(t *testing.T) {`

```text
// TestCycleCost_PrefersSidecarOverEvents is the S7 RED test: the usage
// sidecar (ADR-0044 C1) is written at the recording chokepoint and survives
// abort paths that never reach the tmux normalizer, so it must recover a
// phase whose events.ndjson is entirely missing. A phase that DOES have an
// events.ndjson keeps that value (the richer, per-line-verified source) —
// the sidecar only fills gaps, never overrides an existing events log. The
// bridge's unrelated token-usage.json global snapshot must not be mistaken
// for a phase named "token".
```

### `go/internal/cyclecost/cyclecost_test.go:368` — above `func TestSummarizeCycle_ParityViaProduce(t *testing.T) {`

```text
// TestSummarizeCycle_ParityViaProduce is the ADR-0020 cutover gate: a raw
// <phase>-stdout.log written by the bridge, run through the actual production
// path (phasestream.Produce, exactly as runner.go now calls it), yields an
// events.ndjson from which cyclecost recovers the SAME cost + tokens the raw
// result carried. This is the end-to-end billing-parity guarantee for the
// no-runtime-fallback collapse.
```
