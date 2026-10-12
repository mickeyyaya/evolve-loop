# internal/observerengine

## Purpose

`internal/observerengine` is the phase-observer engine. It holds the stream-json tail and decoder, the stall rules, the incident responder, and the envelope and report sinks. The stall rules are idle nudge, hard stall, no-progress backstop and dead-process probe.

The host, `internal/phaseobserver`, keeps validation, defaults, the real ticker and the select loop. It drives the engine one `Tick` at a time. Design: [decomposition unit 12](../decomposition/12-phaseobserver.md).

## Design

- Every collaborator is an injected function (`Deps`). Tests need no syscall, inbox or tmux, and they run under a stepped clock.
- `Engine` runs on one goroutine by contract. The host's poll loop is its only caller, so no field is locked.
- The engine reports its own faults as `observer.warning` through one `Reporter`. The live `core.Observer` adapter shares that reporter for its sink-open and watcher-leak faults.
- `Tick` runs in a fixed order: the process probe, the tail, the stall rules, the heartbeat, and the EOF check. The process probe comes first, so a dead agent's residual lines cannot refresh the liveness clocks.
- `severityOf` is the one place that decides a code's severity. `TestSeverityTable_CompleteAndRegistered` keeps it total.

## Invariants

- The clock is read at fixed sites. Those sites are construction, each valid stream line, each rules tick, each no-progress tick, each emit and the report. The host's count-stepping clocks depend on that count.
- Progress resets only on `tool_use` and `tool_result`. A streaming text-only agent therefore trips the no-progress backstop (`MaxNoProgressS`, opt-in). Dispatching a tool counts as progress, and so does a tool returning.
- Only `message.content[0]` is inspected for an assistant message. A text block followed by a `tool_use` counts no progress. This is a preserved quirk, not a design choice.
- The engine defaults nothing. The host arrives with `Settings` already defaulted.
- The dead-process INCIDENT fires exactly once. The process group cannot come back, so repeated firing each tick will be spam.
- A recurring fault, such as a broken sink or log that fails every poll, is reported once per code and op for the observer's life.
- A stall's recorded action must match the action that actually runs. A `kill_retry` with no pgid to kill must not claim a kill. A policy verdict outranks `Enforce`. A nil policy is the legacy branch: kill when `Enforce` is set and a pgid exists.
- A scope other than the two known scopes runs no stall rule.
- Rotation: a log that shrank below the saved offset restarts from zero.
- The stream line bound is 10 MiB (`maxLineBytes`). A longer line stops the read. The size is still returned, so the offset is kept. The unterminated-line loss at that point is follow-up F4.
- `Reporter` never panics. A nil reporter, accessor or Center is a Null Object. An unregistered code degrades to WARN with the Center's drift stamp.
- The report is best-effort. A failed write is reported as `OBSERVER_REPORT_WRITE_FAILED` and returned, but the host keeps its exit code. The report is written through `.tmp` and a rename.

## Findings

- Follow-up F3 is still open. The watchdog and soak-report globs do not use `PathsFor` yet.
- The events and report files use the schema-1.0 key order. The goldens pin that order through `json.Marshal` of maps.
