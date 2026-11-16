# Build Explanation — Cycle 2444

## Build Binding
- Cycle: 2444
- Base SHA: b681e55abb42ae6940bb3dafc721149c3c1e13dd

## Summary
The oversight now publishes one WARN signal when a landing or a seal names a cycle that no live lane holds.

## Rationale
The oversight returned without a trace in this case. The operator did not see a landing that the pool cannot attribute. A registered code and one emit call fix that with the smallest change.

## Changed Areas
- `go/internal/orchestrator/orchestrator.go` — registers the code LOOP_ORCH_LANDING_UNLOCATED.
- `go/internal/orchestrator/oversight.go` — the landing and seal handlers call a new function that emits the signal with the fields cycle and event.
- `docs/architecture/signal-codes.md` — the generated code table gains the row for the new code.

## Design Decisions
Both handlers share one emit function. The landing-state cleanup stays before the lookup, so it is unchanged.

## Verification
The oversight tests cover both events at both stages, the kept cleanup and the quiet located lane. The signal-codes sync test passes.

## Compatibility
No existing code, field or action changes. A located lane gets no new signal.

## Limitations
The signal does not recover the lane. It only makes the gap visible.
