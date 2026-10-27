# Build Explanation — Cycle 2051

## Build Binding
- Cycle: 2051
- Base SHA: 66b0bcc7e2f6f09823961034127a333d494162b8

## Summary
`go/internal/log` now has one slog writer factory. A record carries the keys ts, run, cycle, phase, dispatch, cat, component, level and code. One call feeds a JSON handler and a text handler.

## Rationale
A single factory keeps the selection keys in one place. The signal-center stderr sink moves onto it first and keeps its line text through a render hook.

## Changed Areas
- `go/internal/log/selection.go` — adds Selection, Sinks, WithSelection and NewSelectionLogger.
- `go/internal/log/selection_test.go` — tests the nine keys, the override rule, the level filter and the render hook.
- `go/internal/log/apicover_named_test.go` — names each new export for the apicover gate.
- `go/internal/signalcenter/sinks.go` — StderrSink writes through the factory with an unchanged line text.
- `docs/architecture/event-channels.md` — adds the section Selection keys.
- `docs/architecture/packages/internal-log.md` — documents the factory in the package note.
- `go/acs/cycle2051/predicates_test.go` — predicates written by the TDD phase.
- `.evolve/evals/logging-hygiene-h3-selection-keys.md` — eval of the task, written by the Scout phase.

## Design Decisions
The text handler cannot reproduce the FormatLine layout, so `Sinks.Render` replaces it with a line writer. The alternative, a second logger type in signalcenter, keeps the writer out of the factory.

## Verification
The cycle 2051 predicates, the log tests and the signalcenter tests pass.

## Compatibility
The loop-log line text is unchanged. The golden-line predicates pin it.

## Limitations
Only the stderr sink moved. Other loop-log writers stay for later cycles.
