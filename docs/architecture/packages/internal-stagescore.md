# Package `internal/stagescore`

`internal/stagescore` tallies shadow-stage decisions against their observed outcomes.

## Exports

- `Decision` (`WouldStop`, `WouldSkip`, `WouldHold`) names the shadow-stage verdict.
- `Outcome` (`Passed`, `Red`) names the real run outcome.
- `Record` pairs a `Decision`, an `Outcome`, and `LaneRouted`.
- `Tally` holds `Decisions`, `FalseStops`, `Escapes`, and `Conflicts`.
- `Score` aggregates a slice of `Record` and rejects any unknown `Decision`.

## Rules

- `WouldStop` with `Passed` increments `FalseStops`.
- `WouldSkip` with `Red` increments `Escapes`.
- `WouldHold` with `LaneRouted` increments `Conflicts`.
- Unknown decisions return an error so callers fail closed.
