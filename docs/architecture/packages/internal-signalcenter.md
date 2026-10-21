# internal/signalcenter

## Purpose

`internal/signalcenter` is the one event stream that host-side components produce into and the orchestrator listens to. It is a leaf: the standard library plus `internal/log` for the bounded-field sanitizer. `core`, `bridge` and `cmd` import it, and it imports none of them. Design: `docs/architecture/signal-center-design.md` (ADR-0101).

The quality indexes (ADR-0153, proposed) plan to publish through this package as module `quality`. The spec of record is [llm-quality-indexes.md](../llm-quality-indexes.md) §5. Until component Q1 lands, the module, the kinds and the codes do not exist here.

## Design

- `Event` is the one schema. `Emit` stamps `SchemaVersion`, `Seq`, `PID` and `TS`. Producers fill the rest. The schema has no nested structs, so one event is one greppable line. Structured detail goes in the artifact that `fields.path` names.
- `Module` and `Kind` are closed sets. `Code` has the form `MODULE_SNAKE_CASE` and is registered by its owning module. `Severity` follows the schema-1.0 contract: INFO (10) logs only, WARN (20) notes and flags, and INCIDENT (30) acts.
- `Center` is the process-scoped fan-out. A nil `*Center` is a Null Object. Every method is safe and does nothing.
- The sinks are listeners. `NDJSONSink` is the durable record, one file per cycle. `StderrSink` is the one module-tagged line renderer. `ConsoleSink` is the stderr half at WARN and above. The composition roots share that threshold.
- `Summary` is one listener's per-cycle view. It follows the current cycle by itself.

## Invariants

- Listeners observe and never decide (ADR-0101, decision 5).
- Delivery order: `Emit` normalizes, stamps the sequence number and enqueues under the lock. The first emitter that finds no drain in progress drains in `Seq` order. It delivers to a snapshot of the listeners, outside the lock. A re-entrant or concurrent emitter enqueues and returns. Every listener observes one total order, and `Seq` proves it in the durable record.
- A listener may call `Emit`. Re-entrancy is queued, so it never deadlocks.
- `Flush` must never be called from inside a listener. The drain that runs that listener is the one `Flush` waits for.
- A listener that panics is unsubscribed. The panic is reported to the others as a `signalcenter.listener_panicked` INCIDENT. The incident names the function that the runtime reports for the listener, so it names what was dropped.
- `Normalize` never rejects an event. Each violation is recorded in `fields.drift`, and the first one becomes the code. The raw values survive under `raw_*` keys. The severity rises to at least WARN, so drift stays visible in the file the operator reads.
- A rendered line is capped at `MaxLineBytes`, so one `O_APPEND` write stays atomic. Fields go first, largest first. Then `Reason` is cut to fit with an ellipsis. JSON escaping can inflate a rune to six bytes, so the cut re-measures until the line fits. An empty `Reason` always fits, so the loop ends.
- `Origin`, `Phase` and `RunID` are bounded by `MaxIdentRunes`. They are identifiers, never prose.
- The sink's lock is released before a drop report is emitted. A listener can run outside a drain. An emit that drains synchronously re-enters `write`.
- A pending drop count is reported once, on the next successful write. A partial write, such as a disk that fills mid-line, counts as a drop. The bytes already on disk stay. A cycle file may therefore carry one broken line, and readers must tolerate it.
- An event with no workspace yet (cycle path `""`) stays in the recent window and counts as a drop.
- `RegisterCode` records a conflicting registration instead of panicking. A duplicate code is a review defect, not an availability event. `TestRegistryConflicts_RealRegistryIsClean` asserts that the list is empty.
- `Summary` follows the current cycle. A newer cycle resets the view. An older event is ignored. Process-level events (cycle 0) count toward the current cycle. A loop process that runs many cycles therefore needs no external reset.
- The recent window defaults to 256 events (`defaultRecentLimit`).

## Findings

- Each severity tier has one numeric home, `severityLevels`, with the values held in `infoLevel`, `warnLevel` and `incidentLevel`. A new tier needs a schema change, because the contract has no fourth tier.
