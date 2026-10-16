# Comment history: `internal/observerengine`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/observerengine/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf (ADR-0103 unit 12 §2): stdlib
// plus internal/recovery (the StallPolicy strategy) and internal/signalcenter
// (the module's producer) — never core, adapters, bridge or policy (the
// failurelearning/importgraph_test.go idiom).
```

### `go/internal/observerengine/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 12 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/observerengine/observerengine.go:130` — above `Policy recovery.StallPolicy`

```text
// Policy maps a stall incident to an action; nil = the legacy Enforce
// branch with an unenriched INCIDENT envelope (Null Object).
```

### `go/internal/observerengine/observerengine_test.go:3` — above `import (`

```text
// observerengine_test.go — the leaf's fixtures and the construction, layout,
// reporter and registry tests (ADR-0103 unit 12 §6 tests 16-17, 33-35).
// Every test uses t.TempDir() only, a fixed or stepped clock, fake ports and
// a recording Center: no syscall, no inbox, no tmux, no ticker.
```

### `go/internal/observerengine/respond.go:11` — above `type incident struct {`

```text
// incident is one stall INCIDENT: the envelope's type and payload, and the
// typed event the StallPolicy decides on.
```

### `go/internal/observerengine/respond.go:25` — above `func (e *Engine) decide(in incident) (action, reason string, kill bool) {`

```text
// decide is the pure decision (ADR-0044 C4): a nil policy is the legacy
// branch (kill iff Enforce and a pgid); a policy's verdict outranks Enforce,
// and the record reflects what will ACTUALLY happen — a kill_retry with no
// pgid to kill must not claim a kill.
```

### `go/internal/observerengine/respond.go:70` — above `func (e *Engine) killFields(in incident, action, reason string) map[string]string {`

```text
// killFields names the incident, the decision and the idle numbers when the
// incident carries them (fields.kind goes to the Center only, never into the
// envelope).
```

### `go/internal/observerengine/respond_test.go:3` — above `import (`

```text
// respond_test.go — §6 tests 24-26: the pure decision (ADR-0044's
// record-reflects-reality rule as a table) and the act order
// enrich → emit → signal → kill, with the kill error surfaced.
```

### `go/internal/observerengine/tick_test.go:33` — above `func TestTick_ProbeRunsBeforeIngest(t *testing.T) {`

```text
// TestTick_ProbeRunsBeforeIngest — the ProcessAlive closure observes
// eventCount == 0 while three lines are pending; after the tick they are
// ingested; the incident fires exactly once over three ticks. Kills M1 (in
// the leaf) and M5 (processDeadFired dropped).
```

### `go/internal/observerengine/observerengine.go:1` — above `package observerengine`

```text
// Package observerengine is unit 12 of the component breakdown (ADR-0103):
// the phase-observer engine — the stream-json tail and decoder, the stall
// rules (idle nudge, hard stall, no-progress backstop, dead-process probe),
// the incident responder (the ADR-0044 StallPolicy strategy with the
// record-reflects-reality rule) and the envelope/report sinks the manual
// `evolve phase-observer` subcommand runs. The host (internal/phaseobserver)
// keeps validation, defaults, the real ticker and the select loop and drives
// the engine one Tick at a time; every collaborator is an injected function,
// so the leaf is testable with a stepped clock and no syscall, inbox or tmux.
// The engine reports its own faults as observer.warning under module observer
// through ONE Reporter, which the live core.Observer adapter shares for its
// sink-open and watcher-leak faults. Design:
// docs/architecture/decomposition/12-phaseobserver.md.
```
