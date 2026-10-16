# Comment history: `internal/core/outcome`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d11

### `go/internal/core/outcome/recorder.go:1` — above `package outcome`

```text
// Package outcome is unit 01 of the component breakdown (ADR-0103): the C1
// recording chokepoint (ADR-0044) as its own small unit. Every terminal
// disposition of a dispatched phase is recorded exactly once — into the cycle
// result's PhasesRun, the phase-timing log and the <phase>-usage.json sidecar
// — with the end-of-dispatch clock, the phase archetype and the context fill
// stamped here, so no call site re-reads the clock or re-derives the taxonomy.
// The orchestrator injects its clock, its live archetype lookup and its
// phase.outcome emission (the producer keeps its module); the unit's own
// failure modes are outcome.warning signals under module outcome. Design:
// docs/architecture/decomposition/01-outcome-recorder.md.
```

### `go/internal/core/outcome/recorder.go:41` — above `type UsageSidecar struct {`

```text
// UsageSidecar is the declared on-disk contract of <phase>-usage.json: the
// terminal attempt's cost, duration, verdict and stamps, beside the timing
// log's entry for the same dispatch (ADR-0044 C1). Legacy sidecars without
// Tokens parse to zero.
```

### `go/internal/core/outcome/recorder.go:225` — above `func contextFillFor(out recovery.PhaseOutcome) (ratio float64, hot bool) {`

```text
// contextFillFor derives the terminal attempt's context-window occupancy.
// ResolvedModel is the tier the attempt ran at, so it is the honest lookup
// key; anything that is not a canonical tier yields an absent fill (0, false)
// — never a guessed window, and the error never propagates. Unexported so the
// fill is derived in Record alone (the ADR-0044 chokepoint rule).
```

### `go/internal/core/outcome/recorder_test.go:3` — above `import (`

```text
// recorder_test.go — unit 01 (ADR-0103, design decomposition/01-outcome-recorder.md):
// the C1 recording chokepoint as its own small unit. Every terminal phase
// disposition is recorded exactly once — into the cycle result, the
// phase-timing log and the <phase>-usage.json sidecar — with EndedAt, the
// archetype and the context fill stamped here, the orchestrator's emission
// hook called at the same point as before, and the unit's own failure modes
// raised as outcome.warning signals under module outcome.
```
