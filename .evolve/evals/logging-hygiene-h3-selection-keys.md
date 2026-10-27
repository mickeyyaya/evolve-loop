---
score_cap:
  - criterion: "A record from the writer factory carries the nine selection keys with their values in the JSON output"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/log -run 'SelectionLogger|WriterFactory'"
  - criterion: "One call gives the same record to the text handler and the JSON handler"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2051 -run TestC2051_004"
  - criterion: "The signal-center loop-log line text does not change when the writer moves to the factory"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/signalcenter -run 'StderrSink|ConsoleSink'"
  - criterion: "docs/architecture/event-channels.md names the selection keys and the factory"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2051 -run TestC2051_011"
---

# Eval: Selection keys through one slog writer factory (logging plan H3)

> Pins the contract of the writer factory in `go/internal/log`: each record carries `ts`, `run`, `cycle`, `phase`, `dispatch`, `cat`, `component`, `level` and `code`, and one call feeds a JSON handler and a text handler. The first moved writer is the signal-center stderr sink, whose line text must not change. Source: plan `docs/plans/logging-and-process-hygiene-2026-10.md` section 6.1 and evidence E7 (cycle 2051).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| nine-keys | JSON record carries all nine keys | 6/10 | `go test ./internal/log -run SelectionLogger` |
| parity | Text and JSON get one record | 6/10 | `go test -tags acs ./acs/cycle2051 -run TestC2051_004` |
| line-text | Moved writer keeps its line text | 7/10 | `go test ./internal/signalcenter -run StderrSink` |
| docs | event-channels.md names keys and factory | 5/10 | `go test -tags acs ./acs/cycle2051 -run TestC2051_011` |
