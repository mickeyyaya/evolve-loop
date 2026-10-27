# Build Explanation — Cycle 2057

## Build Binding
- Cycle: 2057
- Base SHA: b350d132b0b438285703e74a9f891a74f1970ed0

## Summary
One writer factory, `NewSelectionLogger`, now builds the loop logger. Each record carries the keys `ts`, `run`, `cycle`, `phase`, `dispatch`, `cat`, `component`, `level` and `code`. The stderr loop-log sink writes through the factory, and the text of each line is the same as before.

## Rationale
A reader needs the same keys in every record to select records by run, phase or code. One factory gives those keys to the JSON handler and the text handler from one call. This avoids a second logger setup in each package.

## Changed Areas
- `go/internal/log/selection.go` — adds the factory, the `Selection` value, `Sinks` and `WithSelection`. A fan-out handler gives one record to both handlers.
- `go/internal/log/selection_test.go` — tests each key, the override rule, the level filter and the nil writer.
- `go/internal/log/apicover_named_test.go` — names the new exports for the apicover gate.
- `go/internal/signalcenter/sinks.go` — `StderrSink` builds its writer with the factory and keeps the line text.
- `docs/architecture/event-channels.md` — adds the section "Selection keys" with the nine keys and the factory.
- `docs/architecture/packages/internal-log.md` — names the factory and the keys in the package note.
- `.evolve/evals/logging-hygiene-h3-selection-keys.md` — eval graders for the task.
- `docs/private/research/archived-2026-10-27/superseded-predicate-packages/cycle2051/predicates_test.go` — archived predicates of an unshipped cycle.
- `docs/private/research/archived-2026-10-27/unshipped-build-explanations/cycle-2051-01m5xwqdjqyc9mhva9bxdvs52e.md` — archived explanation of an unshipped cycle.

## Design Decisions
A key in the record wins over the same key from the context, and it appears once. A record without a selection gets no invented values. A render function lets the text handler keep the existing line text. Rejected: a second logger type for text, because it duplicates the level and key logic.

## Verification
The twelve ACS predicates of cycle 2057 pass. The `internal/log` and `internal/signalcenter` package tests pass. `gofmt` and `go vet` report nothing.

## Compatibility
The text of each loop-log line does not change. Existing `NewJSONLogger` callers are untouched.

## Limitations
Only the stderr loop-log sink moves to the factory in this cycle. Other writers move in later cycles.
