# Signal Center — design

> **Status:** design (S0) for [ADR-0101](adr/0101-signal-center.md); revised after the architect's
> design review (2026-09-13: APPROVE-WITH-FIXES — every blocking and should-fix finding is folded in
> below and marked *(review N)*). Evidence:
> [signal-center-inventory-2026-09-13.md](../research/signal-center-inventory-2026-09-13.md).
> Severity contract: [observer-severity.md](observer-severity.md) (schema 1.0, adopted unchanged).
> Envelope compatibility: [phase-observer.md](phase-observer.md) § Unified envelope.

## Table of contents

1. [Purpose, goals, non-goals](#1-purpose-goals-non-goals)
2. [Today](#2-today)
3. [Target architecture](#3-target-architecture)
4. [Event schema](#4-event-schema)
5. [Vocabularies: module, kind, severity, code](#5-vocabularies)
6. [Center API and semantics](#6-center-api-and-semantics)
7. [Wiring at the composition root](#7-wiring)
8. [The orchestrator as listener](#8-the-orchestrator-as-listener)
9. [Logging: one line format, and how triage reads it](#9-logging-one-line-format)
10. [Migration slices and acceptance criteria](#10-migration-slices)
11. [Test plan and engineering bar](#11-test-plan-and-engineering-bar)
12. [Decomposition order it enables](#12-decomposition-order)
13. [Why this is better than today](#13-why-this-is-better-than-today)
14. [Risks and mitigations](#14-risks-and-mitigations)
15. [S1 implementation checklist](#15-s1-implementation-checklist)

---

## 1. Purpose, goals, non-goals

The operator's goal, verbatim: *once we find an issue, because each component is small enough to
isolate it, we can find the root cause by reading the logs within a few review steps; the logic is
easier to observe and modify.* Concretely, after this design:

| Goal | Measure |
|---|---|
| One place to read first when a cycle misbehaves | `.evolve/runs/cycle-N/signals.ndjson` exists for every cycle and carries every terminal phase disposition, every gate rejection, every system failure, every bridge warning |
| Every log line names its module and origin | the stderr sink's one format; hand-written `[x]` prefixes → 0 per module as each migrates (one repo-wide test with a shrinking allowlist) |
| One severity vocabulary, one code registry | `INFO/WARN/INCIDENT` only; every WARN/INCIDENT carries a registered code (an unregistered or missing one is itself a WARN signal) |
| The orchestrator hears every phase's facts as they happen | it is a registered listener; its per-cycle `SignalSummary` is on the cycle result |
| Each module is independently testable and observable | new package at 100 % API + line coverage; every producer has a composed proof; every wiring has a root proof |

**Non-goals and the agent seam** *(review 1)*. The Center is a host-side, in-process stream. A
*phase agent* (the LLM CLI subprocess) never gets an `Emit` API: an untrusted producer must not write
the stream the orchestrator reads its own state from — the same rule that keeps agent-authored fields
from widening agent authority at the typed control-plane boundary ([ADR-0074](adr/0074-typed-signal-contracts-routing-authority.md)).
Agent-authored facts enter the Center **only through the phase deliverable**
(`cyclestate.Diagnostic` on the `PhaseResponse`, `phaseio.ErrorContext` for ship recovery), adapted
by the host at the C1 chokepoint. S1 covers the agent's terminal facts (its diagnostics, PR #577);
in-flight agent progress (the observer's heartbeats, `<phase>-observer-events.ndjson`) enters in S4
through the observer adapter, not through the agent. Also out of scope: replacing the hash-chained
ledger; replacing the ADR-0072 halt path; changing any persona; any network transport.

## 2. Today

Summary of the inventory (§1–§6 there):

- `panestream.SignalCenter`: liveness only, per-dispatch, zero subscribers, no cycle/phase on its event.
- ≥ 15 unwired cycle-level sinks; the richest (`abnormal-events.jsonl`, four writers) has no Go reader.
- Three severity vocabularies; two classification vocabularies (`failureadapter` 11, `failurelog` 11 + 3).
- 361 hand-written stderr writes (350 with a bracketed module prefix; `[orchestrator]` 284) plus prefixes through other writers (`[loop]` 132, `[ship]` 130 literal occurrences); 23 uses of the shared console; no module-tagged logger.
- The only fan-out type in `go/internal` is the pane center's handler list; every other seam is a
  single-slot callback or a file. `bridge.Deps.LivenessCenter` — an optional, nil-defaulting injection
  point — has never been set in production: **optional wiring is how the dead center happened**.
- The decision surfaces (`backfillFailReasons`, `cyclehealth.ClassifyOutcome`, `cmd_loop` breakers,
  `SystemFailureSignal`) each re-derive facts from files or return values after the fact.

What this cost (cycles 1630, 1634, 1636 in one batch): facts existed in one component and had no
channel to the component that decided or to the operator reading the log.

## 3. Target architecture

```
  PRODUCERS (existing chokepoints, adapted in place)          LISTENERS (registered at the root; observe, never decide)
  ─────────────────────────────────────────────────            ────────────────────────────────────────────────────────
  core.recordPhaseOutcome  ── phase.outcome ──┐                ┌── core.Orchestrator.observeSignal
  core SystemFailureSignal ── system.failure ─┤                │     → per-cycle SignalSummary → CycleResult.Signals
  shiperr / ship phase     ── ship.error ─────┤   signalcenter │── NDJSONSink → runs/cycle-N/signals.ndjson  (everything)
  deliverable (gate)       ── gate.rejected ──┼──►  Center  ───┼── Filter(StderrSink, WARN) → "[module] kind SEV CODE …"
  bridge engine            ── bridge.warning ─┤   (sync,       │── cmd_loop batch report (counts)                 (S4)
  panestream LivenessCenter── pane.liveness ──┤    ordered,    └── dashboard SSE push                              (S4)
  ledger decorator         ── ledger.appended ┤    panic-
  dispatchevents/observer  ── loop.* / obs.* ─┘    isolated)
```

- The Center is **one object per process** (`evolve cycle`, `evolve loop`), constructed once at the
  composition root **before the bridge engine**, injected everywhere (`core.WithSignalCenter`,
  `bridge.Deps.Signals` at construction, `orchDeps.Signals`).
- **Producers never format log lines.** They emit a typed event; the stderr sink renders it.
- **Listeners observe; they never decide** *(review 10)*: no consumer may treat a signal as
  authoritative for a verdict, a state transition or a halt. The ledger, the cycle state and the
  ADR-0072 artifacts remain the deciding evidence; the Center is the monitoring stream beside them.

Design patterns, named: **Observer / publish–subscribe** (the Center); **Null Object** (a nil
`*Center` is safe — a *test* affordance, never the production default); **Adapter** (each existing
chokepoint maps its native record onto `Event`); **Registry** (module → codes, with docs); **Decorator**
(the ledger port wrapped to emit); **Facade** (the stderr sink is the one module-tagged logger).

## 4. Event schema

```go
// go/internal/signalcenter/event.go — the ONE schema.
// Leaf: stdlib + internal/log (itself stdlib-only), for the ONE bounded-field sanitizer (review 4).
type Event struct {
    SchemaVersion string            `json:"schema_version"`      // "signal/1.0" (distinct from the observer envelope's "1.0") (review 19)
    Seq           uint64            `json:"seq"`                 // monotonic per Center, stamped by Emit — the provable order (review 7)
    PID           int               `json:"pid"`                 // the emitting process — a resumed cycle appends from a second process (review 7)
    TS            string            `json:"ts"`                  // RFC3339Nano UTC, stamped by Emit
    Cycle         int               `json:"cycle,omitempty"`     // 0 = process-level (no cycle yet)
    RunID         string            `json:"run_id,omitempty"`    // the CA.5 run identity (plays the envelope's trace_id)
    Phase         string            `json:"phase,omitempty"`     // "" = not phase-scoped
    Attempt       int               `json:"attempt,omitempty"`   // dispatch attempt, 1-based
    Module        Module            `json:"module"`              // closed vocabulary (§5.1)
    Origin        string            `json:"origin"`              // Type.Method or Func that emitted (§4 rules) (review 20)
    Kind          Kind              `json:"kind"`                // closed vocabulary, dotted (§5.2)
    Code          Code              `json:"code,omitempty"`      // MODULE_SNAKE_CASE, registered; REQUIRED for WARN/INCIDENT (§5.4) (review 18)
    Severity      Severity          `json:"severity"`            // INFO | WARN | INCIDENT (§5.3)
    Reason        string            `json:"reason"`              // one human line — the "why"
    Fields        map[string]string `json:"fields,omitempty"`    // bounded extras (rules below) (review 15)
}
```

Rules (each is a named test):

- Required: `Module`, `Origin`, `Kind`, `Severity`, `Reason`; `Code` required when `Severity ≥ WARN`.
  `Emit` stamps `SchemaVersion`, `Seq`, `PID`, `TS`.
- `Origin` is `Type.Method` for methods and `Func` for functions, exactly as Go spells them
  (`Orchestrator.recordPhaseOutcome`, `finalizeOutcome`); a regex test pins `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`.
- `Fields`: keys match `^[a-z][a-z0-9_]{0,31}$`; at most 12 keys; values pass through
  `log.DiagnosticField` (UTF-8 clean, rune-capped, no line breaks); over-cap keys are dropped and
  `fields.truncated=<n>` is set. Rendering (JSON and stderr) sorts keys, so golden lines are stable.
- The rendered line is capped at 4 KiB (a single `O_APPEND` write stays atomic); a longer event drops
  fields largest-first and sets `fields.truncated`.
- No nested structs: a signal is greppable as one line; structured detail belongs in the artifact the
  event names in `fields.path`.
- Compatibility with the observer envelope: `source.{component,cycle,phase}` ↔ `module,cycle,phase`;
  `type` ↔ `kind`; `severity` identical; `run_id` plays `trace_id`.

Examples (what cycles 1636 and 1630 would have produced):

```json
{"schema_version":"signal/1.0","seq":41,"pid":9055,"ts":"2026-09-13T17:46:02.114Z","cycle":1636,"run_id":"01J…","phase":"triage","attempt":1,
 "module":"orchestrator","origin":"Orchestrator.recordPhaseOutcome","kind":"phase.outcome","code":"ORCHESTRATOR_PHASE_VERDICT_FAIL",
 "severity":"WARN","reason":"triage verdict=FAIL: top_n card \"phase-stub-shape-rule-at-ship-staging\" names protected surface \"go/internal/phases/ship/gitops.go\" — control-plane changes go through the console route",
 "fields":{"archetype":"plan","duration_ms":"121183","verdict":"FAIL"}}
{"schema_version":"signal/1.0","seq":58,"pid":30870,"ts":"2026-09-12T11:11:40.002Z","cycle":1630,"run_id":"01J…","phase":"triage",
 "module":"orchestrator","origin":"Orchestrator.finalizeCycle","kind":"cycle.sealed","code":"",
 "severity":"INFO","reason":"final verdict SKIPPED_UNKNOWN (triage-empty-commitment)",
 "fields":{"final_verdict":"SKIPPED_UNKNOWN","shipped":"false","termination_reason":"triage-empty-commitment"}}
```

## 5. Vocabularies

### 5.1 Module (closed)

`orchestrator`, `advisor`, `runner`, `bridge`, `liveness` (the renamed pane center), `ship`, `audit`,
`triage`, `scout`, `build`, `tdd`, `gate.contract`, `gate.eval`, `gate.repo`, `inbox`, `config`,
`loop`, `watchdog`, `observer`, `dashboard`, `signalcenter` (self-reports). A module is added by
editing the closed set and its test; an unknown module is stamped `SIGNALCENTER_UNKNOWN_MODULE` and
raised to WARN — never dropped. *(review 21: `advisor` and `config` added to match §12.)*

### 5.2 Kind (closed, dotted `<subject>.<event>`)

| Kind | Meaning | Severity default | Terminal? |
|---|---|---|---|
| `phase.dispatched` | a phase attempt started | INFO | |
| `phase.outcome` | the C1 terminal disposition (verdict recorded) | INFO on PASS, WARN on WARN/FAIL | ✓ on FAIL |
| `phase.aborted` | the cycle aborted after a phase (review reject, guard, persistence) | WARN | ✓ |
| `gate.rejected` / `gate.corrected` | a deliverable/contract gate refused / a correction was re-dispatched | WARN / INFO | ✓ / |
| `ship.landed` / `ship.error` | a landing / a `shiperr` code | INFO / WARN (INCIDENT for the `integrity` class) | / ✓ |
| `system.failure` | ADR-0072 signal (halt-class) | INCIDENT | ✓ |
| `quota.paused` | all families exhausted, cycle paused | WARN | ✓ |
| `bridge.warning` / `bridge.tripwire` | engine telemetry warnings, tripwires | WARN | |
| `pane.liveness` | liveness edge from the LivenessCenter | INFO (hung/stagnant → WARN) | |
| `ledger.appended` | a ledger entry was appended (decorator) | INFO | |
| `cycle.sealed` | final verdict decided (after `finalizeOutcome`) | INFO (FAIL → WARN) | ✓ on FAIL |
| `loop.wave` / `loop.halt` / `loop.escalation` | batch-level events (today's `dispatchevents`) | INFO / INCIDENT / WARN | / ✓ / |
| `signalcenter.listener_panicked` / `signalcenter.registry_drift` / `signalcenter.registry_conflict` | self-reports | INCIDENT / WARN / WARN | |

"Terminal" marks the kinds that end or change a cycle's course — the triage entry point (§9).

### 5.3 Severity — the schema-1.0 contract, unchanged

`INFO` (10) log only · `WARN` (20) note, continue, flag for review · `INCIDENT` (30) act. Mapping of the
existing vocabularies inside adapters (the only place a mapping lives):

| Source vocabulary | → Center severity |
|---|---|
| `observer` info/warn/incident | identical |
| `dispatchevents` INFO/WARN/**ERROR** | INFO/WARN/**INCIDENT** (its ERROR events are stall kills and breaker trips — "act") |
| `cyclestate.Diagnostic` `warning` / `error` on a phase response | rides in `reason`/`fields`; the event's severity comes from the verdict (`phase.outcome`: FAIL → WARN) |
| `shiperr.ShipErrorClass` transient/precondition/config/**integrity** | WARN / WARN / WARN / **INCIDENT** |
| `SystemFailureSignal.Halt` | INCIDENT (Halt=true) / WARN (Halt=false) |

No fourth tier. The triage identity is `kind` + `code`; severity is the response tier; severity is
therefore **not** the triage entry point (§9) *(review 8)*.

### 5.4 Code — namespaced, registered, never silent

- Format: `MODULE_SNAKE_CASE`, module prefix = the emitting module upper-cased (`ORCHESTRATOR_…`,
  `SHIP_…`, `GATE_CONTRACT_…`, `BRIDGE_…`). Required on every WARN/INCIDENT *(review 18)*.
- Registration: `signalcenter.RegisterCode(module, code, doc)` at package init of the owning module
  (or a table in the module). Duplicate registration with an identical doc is a no-op; a conflicting
  registration is recorded in `RegistryConflicts()` and emitted as `signalcenter.registry_conflict`
  (WARN) — never a panic: a duplicate code is a review defect, not an availability event *(review 9)*.
  A named repo test asserts `RegistryConflicts()` is empty. `RegisteredCodes()` is exported so
  `docs/architecture/signal-codes.md` is generated from it (S2).
- Existing vocabularies map by projection, not by copy: `shiperr.ShipErrorCode` (42) → `SHIP_<code>`
  (one function `shiperr.SignalCode(code)`); bridge exit codes → `BRIDGE_EXIT_81`/`_85`/`_86`;
  `failureadapter.Classification` → `LOOP_CLASSIFICATION_<value>` — and `failurelog.Classification`
  is folded into `failureadapter`'s in S2 (one vocabulary, one home); `dispatchevents.EventType` →
  `LOOP_<TYPE>`; the contract-gate codes (`missing_effect`, `unbound_effect`, `MISSING_SECONDARY`,
  **landing in PR #575** — S2 depends on it *(review 12)*) → `GATE_CONTRACT_<CODE>`.
- Validation on `Emit` (§6.2): an unregistered code → `SIGNALCENTER_UNREGISTERED_CODE`; a missing
  code on WARN/INCIDENT → `SIGNALCENTER_MISSING_CODE`; unknown module/kind → `SIGNALCENTER_UNKNOWN_MODULE`
  / `_KIND`. In every case the raw values ride in `fields` (`raw_code`, `raw_kind`, …), severity is
  raised to at least WARN, and the event is delivered — the drift is visible in the same file the
  operator reads.

## 6. Center API and semantics

```go
package signalcenter // leaf: stdlib + internal/log; imported by core, bridge, cmd

type Listener func(Event)

func New(opts ...Option) *Center                     // WithClock(func() time.Time); WithRecentLimit(n); WithPID(int)
func (c *Center) Subscribe(l Listener) (unsubscribe func())
func (c *Center) Emit(e Event)                       // sync, ordered, validating, never drops; re-entrancy-safe; nil-safe
func (c *Center) Recent() []Event                    // bounded snapshot (newest last), cross-cycle; nil-safe
func RegisterCode(m Module, code Code, doc string)   // package registry; conflicts recorded, never panic
func RegisteredCodes() map[Module][]CodeDoc
func RegistryConflicts() []Conflict

// Listeners shipped with the package
func NDJSONSink(pathFor func(cycle int) string) Listener  // append-only; one open handle per cycle; resolver from the root
func StderrSink(w io.Writer) Listener                     // the ONE line format (§9)
func Filter(l Listener, at Severity) Listener             // only events ≥ at — used at the root for stderr (§7)
```

Semantics (each is a named test):

1. **Synchronous total order, provable.** `Emit` takes the emit mutex, validates and stamps
   (`Seq` increments under that mutex), snapshots the listener list under the read lock, releases the
   read lock, then calls listeners in registration order. Concurrent emitters (the evaluate batch
   dispatches phases in goroutines and each reaches the C1 chokepoint) serialize; every listener sees
   the same order and `Seq` proves it in the durable file *(review 7)*. The order mutant to kill is
   "listeners observe different orders", not "registration order reversed".
2. **Never drops.** Validation failures rewrite the event with a `SIGNALCENTER_*` code, keep the
   originals in `fields`, and raise severity to at least WARN (§5.4).
3. **Panic isolation without self-deadlock** *(review 2)*. A panicking listener is recovered; the
   Center reports `signalcenter.listener_panicked` (INCIDENT) to the remaining listeners through an
   internal `dispatchLocked` that reuses the already-held emit mutex — never through the public
   `Emit`. A panic while reporting a panic is written to stderr and swallowed.
4. **Re-entrancy is safe, not forbidden** *(review 2)*. A listener that calls the public `Emit`
   (nested emit) does not deadlock: the Center keeps a per-emit depth flag; a nested event is
   appended to a pending queue and delivered, in order, after the outer fan-out completes. A test
   subscribes a re-entrant listener and asserts completion within a timeout and that the nested
   event follows the outer one by `Seq`.
5. **Null Object for tests, never for production.** `var c *Center = nil`: `Emit` is a no-op,
   `Subscribe` returns a no-op unsubscribe, `Recent` returns nil. Producers never check for nil.
   Production roots must wire a real Center (§7) *(review 3)*.
6. **Unsubscribe is idempotent** and safe during an emit (the snapshot semantics above).
7. **Bounded memory.** `Recent` keeps the last N (default 256) events across cycles; the durable
   record is the sink; the orchestrator's per-cycle summary is reset on cycle start (§8).
8. **The NDJSON sink holds one open handle per cycle** (opened `O_APPEND|O_CREATE` on the first event
   for that cycle, closed when the resolver returns a different cycle or on process exit), writes one
   line per `Write` (≤ 4 KiB, §4), and never re-emits through the public `Emit`: a resolver returning
   `""` (process-level events before a cycle is allocated) keeps the event in `Recent` only and the
   drop count is reported once, via `dispatchLocked`, on the next successful write. (`dispatchevents`
   opens per line today; that is the pattern not to copy at bridge-telemetry rates.)
9. **The stderr sink is the only module-tagged logger.** Producers do not print. At the root it is
   wrapped in `Filter(…, WARN)`: INFO goes to the durable file only, as the severity contract says
   ("log only, do not surface") *(review 6)*.

## 7. Wiring

Composition root `go/cmd/evolve/cmd_cycle.go` (`wireOrchestratorDeps`, which the loop shares via
`wireOrchestratorDepsFn`): the Center is constructed **first**, before the bridge engine, because
`bridge.NewEngine` normalizes `Deps` at construction and a field set afterwards is not observed
*(review 5)*:

```go
signals := signalcenter.New(signalcenter.WithPID(os.Getpid()))
signals.Subscribe(signalcenter.NDJSONSink(func(cycle int) string {
    if cycle == 0 { return "" }
    return filepath.Join(core.RunWorkspacePath(projectRoot, cycle), "signals.ndjson")
}))
signals.Subscribe(signalcenter.Filter(signalcenter.StderrSink(os.Stderr), signalcenter.SeverityWarn))
br := newBridge(gobridge.Deps{Signals: signals, …})     // S3: the engine receives it at construction
opts = append(opts, core.WithSignalCenter(signals))     // S1
deps.Signals = signals                                  // orchDeps gains the field in S1, so cmd_loop reads it in S4 (review 16)
```

**Non-optional and loud** *(review 3)*: `wireOrchestratorDeps` always constructs a Center; the only
nil-Center construction sites are `cmd_cycle_simulate.go` and `internal/routingtest`, and a repo test
pins that list. Wiring proofs (the `DeclaredDeliverablesGateWired` precedent): `Orchestrator.SignalCenterWired()`
asserted by a `cmd/evolve` composition test at both roots (cycle and loop); a `core` test that a
constructed orchestrator with a Center is subscribed (its `observeSignal` receives what its own
chokepoint emits); `engine.SignalsWired()` asserted on the bridge side (S3); an import-graph test that
`signalcenter` imports nothing under `internal/` but `log` *(review 4)*.

## 8. The orchestrator as listener

```go
// core
func WithSignalCenter(c *signalcenter.Center) Option   // NewOrchestrator subscribes o.observeSignal when c != nil
func (o *Orchestrator) SignalCenterWired() bool
func (o *Orchestrator) SignalSummary() SignalSummary   // the CURRENT cycle: counts by severity and kind, last INCIDENT
```

- `observeSignal` is O(1): it updates the current cycle's summary under the orchestrator's summary
  mutex. **Lock order** *(review 17)*: emit mutex → summary mutex, never the reverse; no orchestrator
  path holds the summary mutex across an `Emit`. This is the first production mutex in
  `internal/core`; a `-race` test with concurrent evaluate-batch emitters pins it.
- **Lifetime** *(review 16)*: the summary is reset when a cycle starts (`planCycle`); only the current
  cycle's summary is retained in the orchestrator (a loop process runs many cycles); `Recent` on the
  Center is the cross-cycle window.
- **Observe, never decide** *(review 10)*: `observeSignal` never emits (no feedback loops) and never
  decides — deciding stays with the existing floors; ADR-0072's `SystemFailureSignal` is produced
  *into* the Center in S2 and its halt path is unchanged. The summary is reporting evidence.
- `CycleResult.Signals` (S1) carries the summary out to `cmd_loop`, which prints the counts in the
  batch report without re-reading files (S4) — it does not gate on them.
- The orchestrator's summary is what "closely monitor the loop execution" means in code: at any
  point, `SignalSummary()` says how many WARN/INCIDENT signals the current cycle has raised and the
  last INCIDENT's code and reason.

## 9. Logging: one line format

```
[<module>] <kind> <SEVERITY> <CODE> cycle=<N> phase=<p> attempt=<k> seq=<s> origin=<Type.Method> — <reason> [k=v …sorted]
[orchestrator] phase.outcome WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=1636 phase=triage attempt=1 seq=41 origin=Orchestrator.recordPhaseOutcome — triage verdict=FAIL: top_n card "…" names protected surface "go/internal/phases/ship/gitops.go" archetype=plan verdict=FAIL
[ship] ship.error WARN SHIP_GIT_FLEET_REBASE_NEEDED cycle=1632 phase=ship attempt=1 seq=77 origin=Landing.Land — main moved during the landing; recovering via build (attempt 1/2) class=transient
[signalcenter] signalcenter.registry_drift WARN SIGNALCENTER_UNREGISTERED_CODE cycle=1640 seq=12 origin=Center.Emit — code "SHIP_NEW_THING" is not registered module=ship raw_code=SHIP_NEW_THING
```

**Triage in a few review steps, kind-first** *(review 8)*: (1) `grep -E '"kind":"(system\.failure|phase\.aborted|gate\.rejected|ship\.error|quota\.paused)"' signals.ndjson`
— the terminal kinds (§5.2), or `grep INCIDENT` when something halted; (2) the `code` names the rule and
`module`/`origin` name the function; (3) `fields.path` names the artifact. A WARN budget keeps this
honest: a healthy PASS cycle emits ≤ N WARN signals (N measured from a real green cycle in S1 and
asserted by a composed test); if a green cycle cannot stay under budget, the ladder is decoration.
Hand-written `[x]` prose disappears one module at a time (§10 S5).

## 10. Migration slices

| Slice | Content | Acceptance (measured by tests, not prose) | Depends on |
|---|---|---|---|
| **S0** | this design, ADR-0101, the inventory | docs-only PR; links resolve; architect review folded in | — |
| **S1** | `internal/signalcenter` (schema, Center, registry, sinks, filter); `core.WithSignalCenter` + listener + summary + `CycleResult.Signals`; `orchDeps.Signals`; the C1 chokepoint emits `phase.outcome`/`phase.aborted` on both roots (its PR #577 hand-written line replaced by the sink line); `.apicover-enforce` entry; CI line-coverage gate | package 100 % API coverage (apicover) + 100 % line coverage (CI/make gate, §11), `-race`; composed RunCycle test: every dispatched phase (incl. the ADR-0044 abort-path table) yields exactly one `phase.outcome` observed by the orchestrator listener and one `signals.ndjson` line with monotonic `seq`; WARN-budget test; re-entrancy test; mutants: emit removed, listener not subscribed, sink not attached, validation bypassed, recover removed, same-order-for-all-listeners — each killed by name | PR #577 (lands after the #575/#576/#577 merge order) |
| **S2** | producers: `SystemFailureSignal` → `system.failure`; `shiperr` → `ship.error` (+ `shiperr.SignalCode`); contract gate → `gate.rejected/corrected`; quota pause → `quota.paused`; `cycle.sealed` after `finalizeOutcome`; `failurelog.Classification` folded into `failureadapter`'s; generated `docs/architecture/signal-codes.md` | each producer has a composed proof; the classification registry test fails if the two vocabularies diverge again | PR #575 (gate codes) |
| **S3** | bridge: `Deps.Signals` at construction + `engine.SignalsWired()`; engine WARN/TRIPWIRE/CONTEXT-FILL → `bridge.warning/tripwire`; **commit 1:** the pure rename `panestream.SignalCenter` → `LivenessCenter` (ADR-0068/0070 amended by ADR-0101; `bridge.Deps.LivenessCenter` already carries the target name); **commit 2:** `pane.liveness` production; hand-written `[engine]` lines removed | the repo-wide prefix test's allowlist shrinks by `[engine]`/`[bridge]` | S1 |
| **S4** | ledger decorator (`ledger.appended`); `dispatchevents` writers and the `observer` adapter emit through the Center (their files stay as sink outputs until readers migrate); `cmd_loop` **reports** signal counts in the batch report; the dashboard SSE subscribes | `abnormal-events.jsonl` **field-equal modulo timestamp precision** before/after, asserted by a decoding golden *(review 11)*; dashboard shows a signal within one SSE tick; no breaker gates on a signal (a shadow comparison test may log disagreement) | S2 |
| **S5** | per-module log migration riding each decomposition slice (§12): prefix-literal occurrences per module (any writer) `[orchestrator]` 287 → 0, `[loop]` 132 → 0, `[ship]` 130 → 0, … | one repo-wide grep test with a shrinking allowlist (a migrated module cannot be forgotten); the inventory's prefix table regenerated | S1 |

## 11. Test plan and engineering bar

- **TDD, red first, every slice.** For each behavior in §6 a table-driven test is written and watched
  fail before the implementation. Bug-shaped tests reproduce the live incidents: cycle 1636's triage
  rejection produces the exact `phase.outcome` above; cycle 1630's closeout produces `cycle.sealed`
  with `shipped=false`.
- **Coverage** *(review 13)*. `internal/signalcenter` joins `.apicover-enforce` (0 uncovered exports,
  0 false-greens, godoc on every export — the in-test API gate stays as today). The **100 % line
  coverage is enforced in CI/`make`** (`go test -count=1 -coverprofile` + a threshold step beside
  the apicover gate), never by a test that reads its own profile — that would skip silently.
- **Race.** `go test -race ./internal/signalcenter/` with concurrent emitters and (un)subscribers; a
  `core` `-race` test with evaluate-batch goroutines emitting into the orchestrator's summary.
- **Named tests added on review:** re-entrant listener completes (no deadlock) and its nested event
  follows by `seq`; WARN budget on a green cycle; two-process append (a resumed cycle appends with a
  different `pid`, both orders reconstructible by `pid`+`seq`); registry conflicts empty; import
  graph (`signalcenter` imports only `internal/log`); nil-Center construction sites pinned; the
  repo-wide `[x]` prefix allowlist.
- **Mutation proof** per slice, each mutant confirmed to build: emit removed at a producer; listener
  not subscribed; sink not attached; validation short-circuited; recover removed; "listeners observe
  different orders" (not "registration order reversed" — the invariant is sameness).
- **Wiring proof** at the composition root for every injection (`SignalCenterWired`, `SignalsWired`,
  `orchDeps.Signals`), at both roots.
- **Clean-code limits enforced by tests:** every file in the package < 800 lines (the apicover DoD
  already checks this), every function < 50 lines (a `go/ast` test over the package), nesting ≤ 4.
- **Reviewer fleet** on every slice: simplifier → architecture-reviewer ∥ go-reviewer; CRITICAL blocks;
  the S1 PR is re-reviewed specifically against review findings 2, 3, 5, 7 and 17 (the ones that turn
  into live deadlocks, dead wiring or unfalsifiable claims if they stay prose).

## 12. Decomposition order

The Center is the enabler: an extracted unit gets its `Module` tag, its code namespace, its
producer tests and its place on the prefix allowlist on day one. **Cut criterion** *(review 21)*:
`core` first (the orchestrator is where every fact converges and where the mutex now lives), then
by size, with the audit package deferred to a later wave because its files are gate logic under
ADR-0072's coherence floor and need their own campaign note.

| Order | Unit to extract (from) | Size today | Module tag | Notes |
|---|---|---|---|---|
| 1 | phase-outcome recording + failure learning → `internal/core/outcome` (`failure_learning.go` 1070) | 1070 | `orchestrator` | already the C1 chokepoint; S1 touches it — extract after S1 lands |
| 2 | phase advisor → `internal/advisor` (`phase_advisor.go` 1144) | 1144 | `advisor` | pure decision logic; strong test seam |
| 3 | `orchestrator.go` (1158): composition (`New`, options) vs `RunCycle` engine | 1158 | `orchestrator` | Facade over the extracted units |
| 4 | `cyclerun.go` (904): finalize/closeout → `cycleclose` | 904 | `orchestrator` | `finalizeCycle` + `finalizeOutcome` + dossier |
| 5 | `inboxmover.go` (1006) | 1006 | `inbox` | claim/release/promote as three units |
| 6 | `phases/ship/gitops.go` (989) | 989 | `ship` | landing vs binding writer vs staging guard |
| 7 | `config/config.go` (962) | 962 | `config` | typed policy structs; retire env flags as a by-product |
| 8 | `bridge/engine.go` (791) + `autorespond.go` (787) | 1578 | `bridge` | rides S3 |
| later wave | `phases/audit/defect_ledger.go` (881), `acssuite.go` (866), `phases/audit/ciparity.go` (801) | 2548 | `audit`, `acs` | gate logic under the ADR-0072 floor; own campaign note |

Method per unit (named): **Strangler Fig** (new unit beside the old, callers moved one by one, old
deleted), **Extract Class / Extract Function** (Fowler), **Facade** for the remaining public surface,
**Dependency Injection** through the existing `Option` pattern. Static feature flags met in a unit are
retired in that slice (typed policy resolved at the root — `docs/architecture/control-flags.md`
shrinks per slice; no separate flag pass). Each unit ships with its own error-code table, its module
tag, 100 % API + line coverage, and a design note "why this boundary" appended to this document.

## 13. Why this is better than today

| Dimension | Today | After |
|---|---|---|
| Where to look first | 15+ files/streams per cycle, different shapes | `signals.ndjson` (one file, `seq`-ordered), stderr (one format, WARN and up) |
| Severity | 3 vocabularies | 1 (the existing contract) |
| Error identity | 42 ship codes, 2 classification vocabularies, exit codes, free strings, prose | 1 registry, module-namespaced, unregistered/missing = visible drift |
| Module in the log | 361 hand-typed stderr writes across several writers, inconsistent | every line `[module] … origin=Type.Method` from one sink |
| Orchestrator's view | post-hoc: return values, files re-read by the seal, cyclehealth, breakers | live: a listener with a per-cycle summary |
| Wiring | optional nil-default injection nobody set (`Deps.LivenessCenter`) | non-optional at the root, proven at both roots, nil sites pinned |
| Adding a producer | a new file + a new reader + a new prefix | `Emit` one typed event; sinks, summary and docs follow |
| Testing a signal | grep a log or parse a file | subscribe a test listener; assert the event and its `seq` |
| The 1634/1636 class | needed PR #577 to carry one fact one step | every phase's diagnostics already ride `phase.outcome` |

## 14. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Double emission during migration (hand-written line + sink line) | the same slice that adds `Emit` deletes the `Fprintf`; the repo-wide prefix allowlist shrinks in the same PR |
| Emit re-entrancy / self-deadlock | `dispatchLocked` for self-reports; nested emits queued and drained; a timeout-guarded re-entrancy test |
| Synchronous fan-out latency at hot sites | listeners are O(1) or append-only with an open handle; a slow consumer buffers internally; `signalcenter.slow_listener` WARN when a listener exceeds a budget (S4) |
| `signals.ndjson` growth | one line per event ≤ 4 KiB, ≤ 12 fields; gc retention already prunes run dirs |
| Name clash with the bridge center | S3 renames it `LivenessCenter` in its own commit; ADR-0101 amends ADR-0068/0070 |
| A listener panic taking the orchestrator down | recover + self-report; tested |
| Vocabulary sprawl (kinds/modules added ad hoc) | closed sets with tests; adding one is a reviewed edit |
| Registry conflicts | recorded, signalled, asserted empty by a repo test; never a panic |
| Monitoring becoming a control input | the observe-never-decide invariant (§3, §8); S4 reports, does not gate |

## 15. S1 implementation checklist

1. RED: `signalcenter` tests for §6 semantics 1–9, the §4 rules (required fields, `Origin` regex,
   `Fields` bounds, line cap), the JSON golden line, the stderr golden line, registry conflicts, the
   import graph.
2. GREEN: `event.go`, `center.go`, `registry.go`, `sinks.go` (< 800 lines each, functions < 50).
3. RED: `core` — `WithSignalCenter`, `SignalCenterWired`, `SignalSummary`, `observeSignal` (lock
   order, reset on cycle start), `CycleResult.Signals`; the C1 chokepoint emits on every terminal path
   (reuse `orchestrator_phaseoutcome_test.go`'s abort table); the WARN budget on a green cycle.
4. GREEN: `recordPhaseOutcome` → `Emit`; delete its PR #577 `Fprintf` line (the sink renders it);
   update `TestRecordPhaseOutcome_CarriesThePhaseDiagnosticsAndNamesAReasonedFail` to assert the
   sink line format.
5. RED/GREEN: composition root — Center constructed before the bridge engine, `orchDeps.Signals`,
   NDJSON sink + `Filter(StderrSink, WARN)`, `SignalCenterWired` at both roots, nil-site pin test;
   `.apicover-enforce` entry; the CI line-coverage gate.
6. Mutants (each confirmed to build): emit removed; subscribe removed; sink not attached; validation
   bypassed; recover removed; listeners-see-different-orders.
7. Floors: gofmt/vet/`-race`; whole-module; integration; ACS; apicover.
8. Fleet: simplifier → architecture-reviewer ∥ go-reviewer (re-review against review findings 2, 3,
   5, 7, 17); commit-gate; ship; PR (after #575/#576/#577).
