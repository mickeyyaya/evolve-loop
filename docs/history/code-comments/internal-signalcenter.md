# Comment history: `internal/signalcenter`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/signalcenter/center.go:13` — above `type Listener func(Event)`

```text
// Listener receives every event the Center delivers. Listeners observe; they
// never decide (ADR-0101 decision 5). A listener may call Emit (re-entrancy is
// queued, never a deadlock). A listener that panics is unsubscribed and the
// panic is reported to the others as a signalcenter.listener_panicked INCIDENT.
```

### `go/internal/signalcenter/center_test.go:3` — above `import (`

```text
// center_test.go — the Center's delivery contract (ADR-0101 decision 3; design §6).
//
// Delivery model: Emit assigns the sequence number and enqueues under the emit
// lock; the first emitter that finds no drain in progress drains the queue in
// seq order, delivering each event to a snapshot of the listeners, outside the
// lock. A listener that emits (re-entrancy) enqueues and returns; its event is
// delivered by the active drainer right after the current fan-out, before the
// outer Emit returns. A concurrent emitter that finds a drain in progress
// enqueues and returns; the active drainer delivers its event in seq order
// before the drainer's own Emit returns. Every listener therefore observes the
// same total order, and `seq` proves it in the durable record.
```

### `go/internal/signalcenter/event.go:1` — above `package signalcenter`

```text
// Package signalcenter is the ONE event stream every host-side component
// produces into and the orchestrator listens to (ADR-0101, docs/architecture/
// signal-center-design.md). It is a leaf: stdlib plus internal/log for the one
// bounded-field sanitizer. core, bridge and cmd import it; it imports none of
// them.
//
// Vocabulary: Module and Kind are closed sets; Code is MODULE_SNAKE_CASE and
// registered by its owning module; Severity is the schema-1.0 contract of
// docs/architecture/observer-severity.md (INFO/WARN/INCIDENT), adopted
// unchanged. An event that breaks a rule is never dropped — Normalize stamps
// it with a SIGNALCENTER_* code, keeps the raw values in Fields and raises it
// to at least WARN, so drift is visible in the same file the operator reads.
```

### `go/internal/signalcenter/event_test.go:3` — above `import (`

```text
// event_test.go — the ONE schema and its closed vocabularies (ADR-0101 decisions 1, 2, 4;
// design §4–§5). Every rule here is one the Center enforces on Emit: an event that
// breaks one is never dropped — it is stamped with a SIGNALCENTER_* code, keeps the
// raw values in Fields, and is raised to at least WARN, so the drift is visible in the
// same file the operator reads.
```

### `go/internal/signalcenter/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises for this package
// (ADR-0101 decision 9; design §11), enforced by a test rather than by review:
// every function < 50 lines, nesting depth ≤ 4, every file < 800 lines.
```

### `go/internal/signalcenter/registry_test.go:3` — above `import (`

```text
// registry_test.go — the code registry (ADR-0101 decision 4; design §5.4). Codes are
// MODULE_SNAKE_CASE, registered by their owning module with a one-line doc. A
// duplicate registration with the same doc is a no-op; a conflicting one is
// RECORDED (never a panic — a duplicate code is a review defect, not an
// availability event) and a named test asserts the registry is conflict-free.
```

### `go/internal/signalcenter/sinks_test.go:3` — above `import (`

```text
// sinks_test.go — the two default listeners and the filter (ADR-0101 decision 6;
// design §6.8–6.9, §9). The NDJSON sink is the durable record (everything, one line
// per event, one open handle per cycle); the stderr sink is the ONE module-tagged
// log line; Filter is how the root keeps INFO out of the console.
```

### `go/internal/signalcenter/sinks_test.go:89` — above `c.Emit(infoEvent("process-level 1"))`

```text
// cycle 0 → no file yet
```

### `go/internal/signalcenter/sinks_test.go:175` — above `func TestConsoleSink_RendersWarnAndAboveInTheOneLineFormat(t *testing.T) {`

```text
// ConsoleSink is the console half of every composition root — the orchestrator
// roots (newRootSignalCenter) and the `evolve phase-observer` subprocess
// (ADR-0103 unit 12 review fold) — so the WARN threshold has ONE home: INFO
// never renders (the contract's "log only" tier), WARN and INCIDENT render in
// the one line format. Kills a threshold raised to INCIDENT and a dropped Filter.
```

### `go/internal/signalcenter/summary.go:3` — above `type Summary struct {`

```text
// Summary is one listener's per-cycle view (design §8): how many signals the
// current cycle raised by severity and kind, and the last INCIDENT. It follows
// the current cycle by itself — an event for a newer cycle resets the view, an
// event for an older one is ignored, and process-level events (cycle 0) count
// toward the current cycle — so a loop process that runs many cycles needs no
// external reset. Not safe for concurrent use; the owner serializes calls.
```

### `go/internal/signalcenter/summary.go:17` — above `func NewSummary() *Summary { return &Summary{BySeverity: map[Severity]int{}, ByKind: map[Kind]int{}} }`

```text
// NewSummary returns an empty view for cycle 0.
```

### `go/internal/signalcenter/summary.go:41` — above `func (s *Summary) Snapshot() Summary {`

```text
// Snapshot returns an independent copy (maps and the last incident included);
// a zero Summary snapshots to empty, non-nil maps.
```

### `go/internal/signalcenter/summary_test.go:3` — above `import "testing"`

```text
// summary_test.go — the per-cycle view a listener keeps (design §8): counts by
// severity and kind and the last INCIDENT. Process-level events (cycle 0)
// count toward the current cycle; an event for a NEWER cycle resets the view
// (a loop process runs many cycles and keeps only the current one).
```
