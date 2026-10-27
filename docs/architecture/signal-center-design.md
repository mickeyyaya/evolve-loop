# Signal Center — design

> **Status:** design (S0) for [ADR-0101](adr/0101-signal-center.md), revised after the architect's design review (2026-09-13: APPROVE-WITH-FIXES). The design folds in every blocking and `should-fix` finding below and marks it *(review N)*.
> Evidence: [signal-center-inventory-2026-09-13.md](../research/signal-center-inventory-2026-09-13.md).
> Severity contract: [observer-severity.md](observer-severity.md) (schema 1.0, adopted unchanged); envelope compatibility: [phase-observer.md](phase-observer.md) § Unified envelope.
> External subscribers (2026-10-09): a `Publisher` listener at each root appends every event to channel logs. Other programs subscribe with `evolve events` ([event-channels.md](event-channels.md), [ADR-0127](adr/0127-push-only-event-channels.md)).

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

The operator's goal, verbatim:

> "once we find an issue, because each component is small enough to isolate it, we can find the root cause by reading the logs within a few review steps; the logic is easier to observe and modify."

Concretely, after this design:

| Goal | Measure |
|---|---|
| One place to read first when a cycle misbehaves | `.evolve/runs/cycle-N/signals.ndjson` exists for every cycle and carries every terminal phase disposition, every gate rejection, every system failure, every bridge warning |
| Every log line names its module and origin | the stderr sink's one format; hand-written `[x]` prefixes → 0 per module as each migrates (one repo-wide test with a shrinking allowlist) |
| One severity vocabulary, one code registry | `INFO/WARN/INCIDENT` only; every WARN/INCIDENT carries a registered code (an unregistered or missing one is itself a WARN signal) |
| The orchestrator hears every phase's facts as they happen | it is a registered listener; its per-cycle `SignalSummary` is on the cycle result |
| Each module is independently testable and observable | new package at 100 % API + line coverage; every producer has a composed proof; every wiring has a root proof |

**Non-goals and the agent seam** *(review 1)*. The Center is a host-side, in-process stream. A
*phase agent* (the LLM CLI subprocess) never gets an `Emit` API. An untrusted producer must not write
the stream that the orchestrator reads its own state from. This is the same rule that keeps agent-authored fields
from widening agent authority at the typed control-plane boundary ([ADR-0074](adr/0074-typed-signal-contracts-routing-authority.md)).

Agent-authored facts enter the Center **only through the phase deliverable**
(`cyclestate.Diagnostic` on the `PhaseResponse`, `phaseio.ErrorContext` for ship recovery). The host adapts
them at the C1 chokepoint. S1 covers the terminal facts of the agent (its diagnostics, PR #577).
In-flight agent progress (the heartbeats of the observer, `<phase>-observer-events.ndjson`) enters in S4
through the observer adapter, not through the agent.

Four things are also out of scope:

- replacement of the hash-chained ledger
- replacement of the ADR-0072 halt path
- a change to any persona
- any network transport

## 2. Today

Summary of the inventory (§1–§6 there):

- `panestream.SignalCenter`: liveness only, per-dispatch, zero subscribers, no cycle/phase on its event.
- ≥ 15 unwired cycle-level sinks; the richest (`abnormal-events.jsonl`, four writers) has no Go reader.
- Three severity vocabularies; two classification vocabularies (`failureadapter` 11, `failurelog` 11 + 3).
- 361 hand-written stderr writes (350 with a bracketed module prefix; `[orchestrator]` 284) plus prefixes through other writers (`[loop]` 132, `[ship]` 130 literal occurrences). 23 uses of the shared console. No module-tagged logger.
- The only fan-out type in `go/internal` is the pane center's handler list; every other seam is a
  single-slot callback or a file. `bridge.Deps.LivenessCenter` — an optional, nil-defaulting injection
  point — has never been set in production: **optional wiring is how the dead center happened**.
- The decision surfaces (`backfillFailReasons`, `cyclehealth.ClassifyOutcome`, `cmd_loop` breakers,
  `SystemFailureSignal`) each re-derive facts from files or return values after the fact.

What this cost (cycles 1630, 1634, 1636 in one batch): facts existed in one component. They had no
channel to the component that decided, or to the operator who read the log.

## 3. Target architecture

```
  PRODUCERS (existing chokepoints, adapted in place)          LISTENERS (registered at the root; observe, never decide)
  ─────────────────────────────────────────────────            ────────────────────────────────────────────────────────
  core.recordPhaseOutcome  ── phase.outcome ──┐                ┌── core.Orchestrator.observeSignal
  core SystemFailureSignal ── system.failure ─┤                │     → per-cycle Summary (Orchestrator.SignalSummary())
  shiperr / ship phase     ── ship.error ─────┤   signalcenter │── NDJSONSink → runs/cycle-N/signals.ndjson  (everything)
  deliverable (gate)       ── gate.rejected ──┼──►  Center  ───┼── Filter(StderrSink, WARN) → "[module] kind SEV CODE …"
  bridge engine            ── bridge.warning ─┤   (sync,       │── cmd_loop batch report (counts)                 (S4)
  panestream LivenessCenter── pane.liveness ──┤    ordered,    └── dashboard SSE push                              (S4)
  ledger append observer   ── ledger.appended ┤    panic-
  dispatchevents/observer  ── loop.* / obs.* ─┘    isolated)
```

- The Center is **one object per process** (`evolve cycle`, `evolve loop`). The composition root constructs it once
  **before the bridge engine** and injects it everywhere (`core.WithSignalCenter`,
  `bridge.Deps.Signals` at construction, `orchDeps.Signals`).
- **Producers never format log lines.** They emit a typed event, and the stderr sink renders it.
- **Listeners observe; they never decide** *(review 10)*. No consumer may treat a signal as
  authoritative for a verdict, a state transition or a halt. The ledger, the cycle state and the
  ADR-0072 artifacts remain the deciding evidence. The Center is the monitoring stream beside them.

Design patterns, named:

- **Observer / publish–subscribe** (the Center)
- **Null Object** (a nil `*Center` is safe — a *test* affordance, never the production default)
- **Adapter** (each existing chokepoint maps its native record onto `Event`)
- **Registry** (module → codes, with docs)
- **Observer at the write chokepoint** (the ledger port wrapped to emit)
- **Facade** (the stderr sink is the one module-tagged logger)

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

- Required: `Module`, `Origin`, `Kind`, `Severity`, `Reason`. `Code` is required when `Severity ≥ WARN`.
  `Emit` stamps `SchemaVersion`, `Seq`, `PID`, `TS`.
- `Origin` is `Type.Method` for methods and `Func` for functions, exactly as Go spells them
  (`Orchestrator.recordPhaseOutcome`, `finalizeOutcome`). A regex test pins `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`.
- `Fields`: keys match `^[a-z][a-z0-9_]{0,31}$`. At most 12 keys are allowed.
  - Values (and `Reason`) pass through `log.SanitizeField` (UTF-8 clean, control characters and U+2028/2029 folded to spaces, 512-rune cap).
  - Over-cap keys are dropped, and `fields.truncated=<n>` is set.
  - Rendering (JSON and stderr) sorts keys, so golden lines are stable.
  - `log.DiagnosticField` stays the quoted rendering for hand-written diagnostics until S5 retires them.
- The rendered line is capped at 4 KiB (a single `O_APPEND` write stays atomic) **for any event**.
  - A longer event drops fields largest-first and sets `fields.truncated`.
  - If the line is still over, `Reason` is cut to fit with a trailing `…`. JSON escaping can inflate a rune-capped `Reason` to six bytes a rune.
  - `Origin`, `Phase` and `RunID` are identifiers bounded to 128 runes (`MaxIdentRunes`). An empty `Reason` therefore always fits, and the cap provably terminates (`TestNormalize_LineCapHoldsForAnyEvent`).
- No nested structs: a signal is greppable as one line. Structured detail belongs in the artifact that the
  event names in `fields.path`.
- Compatibility with the observer envelope: `source.{component,cycle,phase}` ↔ `module,cycle,phase`;
  `type` ↔ `kind`; `severity` identical; `run_id` plays `trace_id`.

Examples (the lines that the Center produces for cycles 1636 and 1630 if it had run):

```json
{"schema_version":"signal/1.0","seq":41,"pid":9055,"ts":"2026-09-13T17:46:02.114Z","cycle":1636,"run_id":"01J…","phase":"triage","attempt":1,
 "module":"orchestrator","origin":"Orchestrator.recordPhaseOutcome","kind":"phase.outcome","code":"ORCHESTRATOR_PHASE_VERDICT_FAIL",
 "severity":"WARN","reason":"triage verdict=FAIL: top_n card \"phase-stub-shape-rule-at-ship-staging\" names protected surface \"go/internal/phases/ship/gitops.go\" — control-plane changes go through the console route",
 "fields":{"archetype":"plan","duration_ms":"121183","verdict":"FAIL"}}
{"schema_version":"signal/1.0","seq":58,"pid":30870,"ts":"2026-09-12T11:11:40.002Z","cycle":1630,"run_id":"01J…",
 "module":"orchestrator","origin":"cycleRun.completeCycle","kind":"cycle.sealed",
 "severity":"INFO","reason":"final verdict SKIPPED_UNKNOWN",
 "fields":{"final_verdict":"SKIPPED_UNKNOWN","phases_run":"2","termination_reason":"triage-empty-commitment"}}
```

## 5. Vocabularies

### 5.1 Module (closed)

The closed set of modules:

- `orchestrator`
- `advisor`
- `runner`
- `bridge` (the engine, and since breakdown unit 16 the `BRIDGE_SUBAGENT_*` codes of the `evolve subagent run` execution path). One module has two producers. `origin` and the sub-prefix tell them apart.
- `liveness` (the renamed pane center)
- `ship`. It also carries the warnings of the landing: the ff-merge, the push with its inline repair, and the ship-binding witness (breakdown unit 07 — `internal/phases/ship/landing`, `ship.warning`).
- `audit` (the defect-ledger gate, breakdown unit 09). The audit package's later units share the tag. The `AUDIT_LEDGER_` family prefix and `origin=Ledger.<Method>` name the producer.
- `triage`
- `scout`
- `build`
- `tdd`
- `gate.contract`
- `gate.eval`
- `gate.repo`
- `inbox`
- `config` (the routing-config loader — registry, env and policy-stage resolution; breakdown unit 08)
- `loop`
- `watchdog`
- `observer` (the own faults of the phase observer — the engine `internal/observerengine` of the manual subcommand and the sink of the live adapter; breakdown unit 12)
- `dashboard`
- `signalcenter` (self-reports)
- `ledger` (the append observer of the file ledger, S4a)
- `outcome` (the phase-outcome recorder, breakdown unit 01)
- `failurediag` (the failure-diag sidecar writer and the delivery-failure classifier, breakdown unit 02)
- `carryover` (the carryover-todo lifecycle, breakdown unit 03)
- `failurelearning` (the failure-learning engine — the failed-approach recorder, the deterministic floor, the recurrence closure; breakdown unit 03b)
- `review` (the kernel half of the code-review phase, `internal/codereview`, ADR-0124: `REVIEW_FINDINGS`)
- `repair` (the fixability check before a repair round, ADR-0158: `REPAIR_HANDOFF`)

A module is added by
editing the closed set and its test. The Center stamps an unknown module `SIGNALCENTER_UNKNOWN_MODULE` and
raises it to WARN — never dropped. *(review 21: `advisor` and `config` added to match §12.)*

### 5.2 Kind (closed, dotted `<subject>.<event>`)

| Kind | Meaning | Severity default | Terminal? |
|---|---|---|---|
| `phase.dispatched` | a phase starts: one event for each dispatch, not for each retry (ADR-0127 E11) | INFO | |
| `phase.outcome` | the C1 terminal disposition (verdict recorded). It is also the routing disposition that the orchestrator takes after a phase. Two cases exist: an audit FAIL's repair grant or decline (`ORCHESTRATOR_AUDIT_REPAIR_*`), and a post-Build writer's stale explanation routed back to Build (`ORCHESTRATOR_EXPLANATION_REAUTHOR_ROUTED`, WARN). A routing disposition carries no verdict and is not a phase run. The dashboard counts a `phase.outcome` as a run only when `fields.verdict` is set | INFO on PASS, WARN on WARN/FAIL | ✓ on FAIL |
| `phase.aborted` | the cycle aborted after a phase (review reject, guard, persistence) | WARN | ✓ |
| `gate.passed` | the contract gate let the phase advance. Cases: verified, salvaged, or advanced with a WARN that it is recommended not to hide (`would-block` under a shadow stage, breaker demotion, fail-open) — S2b. For a verified phase (`GATE_CONTRACT_VERIFIED`), the fields name the artifact, its size, the owed files and effects, and where the gate searched | INFO; WARN for `would-block` / demoted / fail-open | |
| `gate.rejected` / `gate.corrected` | `gate.rejected`: the contract gate refused at enforce. The reason IS the correction directive (`fields.codes`, `blocks`). `gate.corrected`: the ladder of the orchestrator re-dispatched a correction (`fields.correction`, `max`, `rung`, `cli`, `escalated`) — S2b, both roots | WARN / INFO | ✓ / |
| `ship.landed` / `ship.error` | a landing / a `shiperr` code | INFO / WARN (INCIDENT for the `integrity` class) | / ✓ |
| `ship.warning` | the landing failed to reset the tracked binary before the ff-merge or declined the inline push-race repair. Or it failed to read HEAD after the push, or failed to write ship-binding.json. The commit/push stands. The own error of the ship (if any) is the `ship.error` that follows, and its `fields.step` names the landing step (unit 07) | WARN | |
| `system.failure` | ADR-0072 signal (halt-class) | INCIDENT | ✓ |
| `quota.paused` | all families exhausted, cycle paused | WARN | ✓ |
| `bridge.warning` / `bridge.tripwire` | engine telemetry warnings, tripwires. They cover the launch-exit classification (`BRIDGE_EXIT_*`, one per non-zero `Engine.Launch` exit, origin `Engine.Launch`, `fields.step=classify`). They also cover the four Launch step failures (`BRIDGE_BOOT_STRIKE_CLEAR_FAILED` / `_RECORD_FAILED`, `BRIDGE_LAUNCH_ERROR_PERSIST_FAILED`, `BRIDGE_RESULT_READ_FAILED`; origin `Engine.<step>`) — breakdown unit 10 | WARN | |
| `bridge.warning` / `bridge.tripwire` | engine telemetry warnings, tripwires. They also cover the admission, resolution, preparation, execution, verification and ledger faults of the `evolve subagent run` execution path (`BRIDGE_SUBAGENT_*`, `origin=Dispatcher.Dispatch`, breakdown unit 16) | WARN | |
| `pane.liveness` | liveness edge from the LivenessCenter | INFO; WARN with a `LIVENESS_PANE_*` code — the registry (rendered in `signal-codes.md`) is the one list of which states warn | |
| `ledger.appended` | a ledger entry was appended (the file ledger's append observer; `fields.entry_seq` names the line) | INFO | |
| `outcome.warning` | the phase-outcome recorder failed to persist a record (sidecar or timing log skipped or failed). The in-memory record stands (unit 01) | WARN | |
| `failurediag.warning` | the failure-diag writer failed to land `<phase>-failure-diag.json` (temp write or rename). The phase abort proceeds unchanged. The diagnosis is lost from disk (unit 02) | WARN | |
| `carryover.warning` | the carryover lifecycle failed to read or decode a cycle-workspace document (`carryover-todos.json`, `defect-ledger.json`), or failed to persist the failure-learning arrays to state.json. The closeout or the caller proceeds (unit 03) | WARN | |
| `failurelearning.warning` | the failure-learning engine failed to load policy, write the deterministic floor artifacts or update the recurrence ledger, or truncated a self-reported defect list. The FailedRecord and P0 todo stand. The cycle proceeds (unit 03b) | non-terminal |
| `config.warning` | the routing-config loader took a fail-safe default. A dial is outside its vocabulary (registry, env or policy.json). Or the spine is weak or misordered. Or an enable is inert. Or a phase registry exists but the loader failed to read or parse it, so the loader degrades to the compiled baseline, which omits triage. The cycle proceeds on the resolved value (unit 08) | WARN | non-terminal |
| `observer.warning` | the phase observer failed to append an event, tail the stdout log, append the nudge or write the report. It also failed to open the live events sink or reclaim its watcher. Or it sent a stall SIGTERM (INCIDENT `OBSERVER_STALL_KILL_SENT`). The phase proceeds unobserved or as decided (unit 12) | WARN / INCIDENT for the kill | |
| `audit.warning` | the defect-ledger gate recorded a rejection that it failed to persist, or recorded an overflow. Or it failed to arm/read/write-back the lineage of a continuation, or found inherited defects unaccounted. In the INFO case, it failed to tell the auditor its inherited ids. The diagnostics stay the graded wire. The audit phase decides the verdict (unit 09) | WARN; INFO for `AUDIT_LEDGER_PROMPT_DEGRADED` | |
| `inbox.warning` | the inbox mover failed to claim, promote, park, release or recover an item, or an item-record rewrite / the continuation manifest read failed. The item stays where the lifecycle guarantees (never lost). The ledger line of the transition says what moved. The contract of the caller (sentinel / NoOp) is unchanged. The cycle proceeds (unit 06) | WARN | See the note below the table. |
| `runner.warning` | the verdict engine of the phase runner reconciled a bridge infra teardown to the own deliverable of the agent (through Verify or the ACS floor). Or it failed a teardown with forensics or degraded an optional phase. Or it found a contracted deliverable unverified after the settle window (and downgraded a clean-ship verdict). Or it failed to write the clean-stdout companion. The PhaseResponse that it returns is authoritative. The orchestrator disposes (unit 11) | WARN | |
| `advisor.warning` | the phase advisor failed to dispatch or parse a routing decision (the caller degrades to the static spine or keeps the first plan). Or it dropped a minted phase that assumed a control-plane identity. Or it failed to read `router.json`, failed to run git for the pre-plan recon, or failed to persist a capture artifact. The cycle proceeds (unit 04). See the second note below the table for the `ORCHESTRATOR_PLAN_PHASE_GATED` code | WARN | |
| `audit.warning` | a CI-parity gate failed to run (fail-open), skipped an env-exclusive scope or absorbed a contention flake. Or it hit its retake deadline without a verdict, lost its log or lock, or deferred a graduation obligation. Or it FAILED the audit on offenders. The diagnostics list and the verdict are unchanged. The cycle routes to retro on FAIL (unit 14) | WARN | |
| `cycle.sealed` | final verdict decided (after `finalizeOutcome`) | INFO (FAIL → WARN) | ✓ on FAIL |
| `loop.wave` / `loop.halt` / `loop.escalation` | batch-level events (today's `dispatchevents`). `loop.wave` carries `fields.wave` on every event. It is INFO for the summary of the coordinator. It is WARN with a code for the conditions of the wave engine (`LOOP_MIN_WIDTH_REPAIR`, `LOOP_WAVE_DISPATCH_FAILED`, `LOOP_WAVE_EMPTY_PLAN`, `LOOP_WAVE_ALL_LANES_STALE`; unit 13). `loop.halt` also carries the two non-zero-exit stops of the chain (`LOOP_CHAIN_INBOX_UNREADABLE`, `LOOP_CHAIN_BATCH_ERROR`) | INFO or WARN / INCIDENT / WARN | / ✓ / |
| `loop.started` / `loop.exit` / `loop.lost` | the life of a loop process (ADR-0127 E11). `announceLoop` emits `loop.started` with `fields.proc_start` and `loop.exit` with `fields.exit`. A re-exec of the loop emits a new `loop.started` with the same `proc_start`, and only the last image emits `loop.exit`. A reader makes `loop.lost` with the code `LOOP_LOST`, and no log holds it. | INFO / INFO / INCIDENT | |
| `loop.warning` | a batch- or chain-boundary degradation with no wave index (unit 13). The cases are a boundary binary refresh that degraded to the current binary (`LOOP_BOUNDARY_REFRESH_SKIPPED` with `fields.step`, `LOOP_BOUNDARY_REFRESH_AUDIT_FAILED`). Others are an invalid inbox item (`LOOP_CHAIN_INBOX_ITEM_INVALID`) and a quota defer (`LOOP_CHAIN_QUOTA_DEFER`). The written rule: wave-indexed → `loop.wave`; other non-terminal → `loop.warning`; a chain stop with a non-zero exit → `loop.halt` | WARN | |
| `signalcenter.listener_panicked` / `signalcenter.sink_dropped` | self-reports: a panicking listener was dropped / the NDJSON sink failed to write (count in `fields.dropped`) | INCIDENT / WARN | |

Note on `inbox.warning`: since 2026-09-14 (docs/incidents/2026-09-14-triage-refusal-poison-loop.md) the kind also covers the per-item breaker of the FAIL closeout.

- `INBOX_ITEM_ROUTED_CONSOLE`: a `TRIAGE_PROTECTED_SURFACE` refusal routed the card console-manual in place (fields task_id, path, reason, step=route).
- `INBOX_ROUTE_NOT_FOUND`: the closeout failed to find the item to route, and it WILL be re-picked.

Since 2026-09-26 (F36), `Orchestrator.performEffectsAndReview` (module `orchestrator`) also raises `ORCHESTRATOR_HOST_EFFECT_FAILED` when a host-performed declared effect fails before the review. An example is a claim whose move failed. The reason names the effect. The effects gate judges the effect either way.

Second note: since 2026-10-01 the `advisor.warning` kind also carries the `ORCHESTRATOR_PLAN_PHASE_GATED` code of the orchestrator (module `orchestrator`, origin `Orchestrator.recordRoutingDecision`). The phase registry removed a phase that the plan of the advisor runs. The fields are `fields.phase`, `rule` (`skip-when-gates-plan` / `insert-when-gates-plan`) and `next_phase`. The walk continues (ADR-0099 amendment 2026-10-01).

"Terminal" marks the kinds that end or change a cycle's course — the triage entry point (§9).

Vocabulary drift (unknown module or kind, unregistered or missing code, bad origin, missing reason)
is **not** a separate event. `Normalize` stamps the drift code onto the offending event itself (§5.4),
so the drift is one line, next to the own reason of the producer.

`signalcenter.registry_drift` is the kind that line carries only when the own kind of the producer is unknown. An unknown kind cannot be
rendered as itself, so `fields.raw_kind` keeps it. `signalcenter` is likewise the module of a line
whose module was unknown (`fields.raw_module`).

Registry conflicts are recorded at package init — before any Center exists — and asserted empty by a named test. They are a review defect, not a
runtime event.

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
  (or a table in the module).
  - Duplicate registration with an identical doc is a no-op.
  - A conflicting registration is recorded in `RegistryConflicts()` — never a panic and never an event. A conflicting registration has a different doc, a malformed code, or a code whose prefix belongs to another module.
  - Registration runs at package init, before any Center exists. A duplicate code is therefore a review defect, not an availability
    event *(review 9)*.
  - `TestRegistryConflicts_RealRegistryIsClean` asserts that the real registry is clean.
  - `IsRegistered(code)` answers which module owns a code.
  - `RegisteredCodes()` is exported, so `docs/architecture/signal-codes.md` is generated from it (S2).
- Existing vocabularies map by projection, not by copy:
  - `shiperr.ShipErrorCode` (42) → `SHIP_<code>` (one function `shiperr.SignalCode(code)`).
  - Bridge exit codes → `BRIDGE_EXIT_<class>` as `launchoutcome.Classify(code, …).Signal` (a column of the one Outcome). The name is the class name (`BRIDGE_EXIT_ARTIFACT_TIMEOUT`, `_UNKNOWN_PROMPT`, `_RESPOND_LOOP_GUARD`, …). `fields.exit_code` carries the number. This landed as breakdown unit 10.
  - Bridge exit codes → `BRIDGE_EXIT_81`/`_85`/`_86` (owner: the bridge-engine unit). The run path stamps the number as `fields.exit_code` on `BRIDGE_SUBAGENT_VERDICT_FAIL` / `_ADAPTER_EXEC_FAILED` — breakdown unit 16.
  - `failureadapter.Classification` → `LOOP_CLASSIFICATION_<value>`. `failurelog.Classification`
    is folded into `failureadapter`'s in S2 (one vocabulary, one home).
  - `dispatchevents.EventType` → `LOOP_<TYPE>`.
  - The contract-gate codes (`missing_effect`, `unbound_effect`, `MISSING_SECONDARY`,
    **landing in PR #575** — S2 depends on it *(review 12)*) → `GATE_CONTRACT_<CODE>`.
- Validation on `Emit` (§6.2):
  - An unregistered code → `SIGNALCENTER_UNREGISTERED_CODE`.
  - A missing code on WARN/INCIDENT → `SIGNALCENTER_MISSING_CODE`.
  - Unknown module/kind → `SIGNALCENTER_UNKNOWN_MODULE` / `_KIND`.
  - In every case the raw values ride in `fields` (`raw_code`, `raw_kind`, …), severity is
    raised to at least WARN, and the event is delivered. The drift is visible in the same file that the
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
func IsRegistered(code Code) (Module, bool)
func RegisteredCodes() map[Module][]CodeDoc
func RegistryConflicts() []Conflict
func Normalize(e Event) (Event, []Code)              // what Emit applies: field bounds, vocabulary, code, text, line cap; returns the drift

// Listeners shipped with the package
func (c *Center) NDJSONSink(pathFor func(cycle int) string) Listener // a Center METHOD: it reports its own drops through c
func StderrSink(w io.Writer) Listener                // writes FormatLine(e) — the ONE line format (§9)
func FormatLine(e Event) string                      // exported: golden tests and other sinks share the format
func Filter(l Listener, at Severity) Listener        // only events ≥ at — used at the root for stderr (§7)
func ConsoleSink(w io.Writer) Listener               // Filter(StderrSink(w), WARN): the console half of EVERY root — the one home of the threshold (§7; ADR-0103 unit 12 review fold)

// One listener's per-cycle view (§8); the orchestrator keeps one
type Summary struct { Cycle, Total int; BySeverity map[Severity]int; ByKind map[Kind]int; LastIncident *Event }
func NewSummary() *Summary
func (s *Summary) Observe(e Event)                   // follows the current cycle by itself: a newer cycle resets, an older one is ignored, cycle 0 counts
func (s *Summary) Snapshot() Summary                 // deep copy
```

Semantics (each is a named test):

1. **Total order, provable: one queue, one drainer.** `Emit` normalizes the event and stamps it
   (`Seq`, `PID`, `TS`) under the mutex of the Center. Then it appends the event to a queue. The
   first emitter that finds no drain in progress becomes the drainer. The drainer pops the events in
   `Seq` order. It delivers each event, in registration order, to a snapshot of the listener list.

   The Center takes the snapshot under the mutex. **The mutex is never held while a listener runs.**
   An emitter that arrives while a drain is in progress enqueues its event and returns. Such an
   emitter is a concurrent evaluate-batch goroutine, or a listener that emits again. The running
   drainer delivers the queued events before it returns. Thus the file order equals the `Seq` order,
   and every listener sees the same sequence *(review 7)*.

   The order mutant to kill is "listeners observe different orders". It is not "registration order
   reversed".

   **Consequence for producers on exit paths** (S1 review). An emitter that meets a drain in progress
   returns before its event is delivered. The sole S1 producer runs on the draining goroutine, so
   delivery is synchronous in practice. `Flush` (S2a) blocks until the queue is empty and no drain
   is running.

   Every root defers its closer, which flushes first, after it wires the Center
   (`TestRootSignalCenter_EveryRootDefersItsCloser`). Thus an `os.Exit` path leaves no gap in `seq`.
   Such a path is `loop.halt` or a watchdog goroutine (S3, S4). Never call `Flush` from inside a
   listener. The drain that runs the listener is the one that `Flush` waits for.
2. **Never drops.** Validation failures rewrite the event with a `SIGNALCENTER_*` code.

   The Center keeps the originals in `fields`. It raises severity to at least WARN (§5.4).
3. **Panic isolation without self-deadlock** *(review 2)*. The Center recovers a panicking listener and unsubscribes it.

   The Center then reports `signalcenter.listener_panicked` (INCIDENT, `fields.listener` = the registration id). It enqueues the report like any other event. The drain in progress delivers it to the remaining listeners after the current event.

   There is no internal dispatch path: one queue and one way in. A report can therefore neither deadlock nor reorder.
4. **Re-entrancy is safe, not forbidden** *(review 2)*. A listener can call the public `Emit` (nested emit) without a deadlock.

   The call enqueues and returns (semantics 1). The drainer delivers the nested event after the outer one. `TestEmit_ReentrantListenerIsQueuedNotDeadlocked` asserts completion within a timeout. It also asserts that the nested event follows the outer one by `Seq`.
5. **Null Object for tests, never for production.** `var c *Center = nil`: `Emit` is a no-op.

   `Subscribe` returns a no-op unsubscribe, and `Recent` returns nil. Producers never check for nil. Production roots must wire a real Center (§7) *(review 3)*.
6. **Unsubscribe is idempotent** and safe during an emit (the snapshot semantics above).
7. **Bounded memory.** `Recent` keeps the last N (default 256) events across cycles.

   The durable record is the sink. The `Summary` of the orchestrator keeps only the current cycle. It resets itself when the first event of a newer cycle arrives (§8).
8. **The NDJSON sink holds one open handle per cycle.**

   The sink opens it with `O_APPEND|O_CREATE` on the first event for that cycle. It closes the handle when the resolver returns a different path, or on process exit. The sink writes one line per `Write` (≤ 4 KiB, §4).

   These cases count as a drop: a resolver that returns `""` (process-level events before a cycle is allocated), an open failure, or a short write. The sink reports the count once, as `signalcenter.sink_dropped` WARN (`fields.dropped=<n>`), on the next successful write. It reports it through the `Emit` of the Center itself, which is why the sink is a Center method.

   A resumed cycle appends to the same file from a new process. `pid`+`seq` reconstruct both orders (`TestNDJSONSink_TwoProcessesAppendToTheSameCycleFile`). `dispatchevents` opens per line today. Do not copy that pattern at bridge-telemetry rates.
9. **The stderr sink is the only module-tagged logger.** Producers do not print.

   At the root, `Filter(…, WARN)` wraps the sink. INFO goes to the durable file only, as the severity contract says ("log only, do not surface") *(review 6)*.

   That wrapping is `ConsoleSink(w)` — the ONE home of the console threshold. The orchestrator roots and the `evolve phase-observer` subprocess (ADR-0103 unit 12) consume it. A module-wide scan (`TestConsoleSinkThresholdHasOneHome`) keeps every other production source from a re-spelling of it.

## 7. Wiring

Composition root `go/cmd/evolve/cmd_cycle.go` (`wireOrchestratorDeps`, which the loop shares through
`wireOrchestratorDepsFn`): the Center is constructed **first**, before the bridge engine. `bridge.NewEngine` normalizes `Deps` at construction, and the engine does not observe a field that the root sets afterwards
*(review 5)*:

```go
signals := newRootSignalCenter(projectRoot, evolveDir, console) // S4a: the ONE sink topology (below)
ld := ledger.New(evolveDir, ledger.WithSignals(signals))        // S4a: the ledger's append observer
br := newBridge(gobridge.Deps{Signals: signals, …})             // S3: the engine receives it at construction
opts = append(opts, core.WithSignalCenter(signals))             // S1
orchDeps.Signals = signals                                      // S1: the root keeps the handle; cmd_loop reads the runner's SignalSummary() (S4a)

// newRootSignalCenter — production and the loop tests' stub root build the same topology:
func newRootSignalCenter(projectRoot, evolveDir string, console io.Writer) *signalcenter.Center {
    signals := signalcenter.New(signalcenter.WithPID(os.Getpid()))
    signals.Subscribe(signals.NDJSONSink(func(cycle int) string {
        if cycle == 0 { return filepath.Join(evolveDir, "signals.ndjson") } // batch-level (cycle-less) signals
        return filepath.Join(core.RunWorkspacePath(projectRoot, cycle), "signals.ndjson")
    }))
    signals.Subscribe(signalcenter.ConsoleSink(console)) // Filter(StderrSink(console), WARN) — its one home
    return signals
}
```

Batch-level signals carry no cycle and are durable in `<evolveDir>/signals.ndjson` (S4a). They are the halts and wave summaries of the loop, and a bridge warning raised before any
cycle. Before S4a the root
returned no path for cycle 0. This was harmless while no regular producer was cycle-less.

Suppose the root still returned no path for cycle 0 when the wave summary of the loop arrived. Then the sink reports a drop after every wave. The stub root caught this danger, because it builds the production topology (§15.4).

**E12 retires this file** ([ADR-0127](adr/0127-push-only-event-channels.md)). The project-level `<evolveDir>/signals.ndjson` retires into the `signals` channel. The root topology then wraps the per-cycle sink in a cycle filter, so cycle 0 events never reach it. The lossless `loop` and `errors` channels keep those events ([event-channels.md](event-channels.md) §2).

**Non-optional and loud** *(review 3)*: `wireOrchestratorDeps` always constructs a Center. The `--simulate` root does the same (both build `newRootSignalCenter`, ONE topology). The architecture review of unit 01
found that the Center-less simulate root had silenced the warnings of the recorder. The only nil-Center
construction site is the test-only `internal/routingtest` engine, and a repo test pins that list.

Wiring proofs (the `DeclaredDeliverablesGateWired` precedent):

- `Orchestrator.SignalCenterWired()` is asserted by a `cmd/evolve` composition test at both roots (cycle and loop).
- A `core` test shows that a constructed orchestrator with a Center is subscribed (its `observeSignal` receives what its own chokepoint emits).
- `engine.SignalsWired()` is asserted on the bridge side (S3).
- An import-graph test shows that `signalcenter` imports nothing under `internal/` but `log` *(review 4)*.

## 8. The orchestrator as listener

```go
// core
func WithSignalCenter(c *signalcenter.Center) Option   // NewOrchestrator subscribes o.observeSignal when c != nil
func (o *Orchestrator) SignalCenterWired() bool
func (o *Orchestrator) SignalSummary() signalcenter.Summary // a snapshot of the CURRENT cycle: counts by severity and kind, last INCIDENT
```

- `observeSignal` is O(1): it updates the summary of the current cycle under the summary
  mutex of the orchestrator.
  - **Lock order** *(review 17)*: emit mutex → summary mutex, never the reverse.
  - No orchestrator path holds the summary mutex across an `Emit`.
  - This is the first production mutex in `internal/core`. A `-race` test with concurrent evaluate-batch emitters pins it.
- **Lifetime** *(review 16)*: the summary follows the current cycle by itself.
  - The first event of a newer cycle resets it.
  - Events of older cycles are ignored.
  - Process-level events (cycle 0) count toward the current cycle.
  - No orchestrator path therefore has to remember to reset it (`Summary.Observe` is tested in isolation).
  - `Recent` on the Center is the cross-cycle window.
- **Observe, never decide** *(review 10)*: `observeSignal` never emits (no feedback loops) and never
  decides. Deciding stays with the existing floors. ADR-0072's `SystemFailureSignal` is produced
  *into* the Center in S2, and its halt path is unchanged. The summary is reporting evidence.
- The orchestrator is the contact point: `cmd_loop` reads `Orchestrator.SignalSummary()` (S4) to
  print the counts in the batch report without a re-read of files. It does not gate on them. The
  summary is deliberately **not** copied onto `CycleResult` (S1 review): one owner, one snapshot
  method, no second copy to keep in step.
- The summary of the orchestrator is what "closely monitor the loop execution" means in code. At any
  point, `SignalSummary()` says how many WARN/INCIDENT signals the current cycle has raised. It also gives the
  code and reason of the last INCIDENT.

## 9. Logging: one line format

```
[<module>] <kind> <SEVERITY> <CODE> cycle=<N> phase=<p> attempt=<k> seq=<s> origin=<Type.Method> — <reason> [k=v …sorted]
[orchestrator] phase.outcome WARN ORCHESTRATOR_PHASE_VERDICT_FAIL cycle=1636 phase=triage attempt=1 seq=41 origin=Orchestrator.recordPhaseOutcome — triage verdict=FAIL: top_n card "…" names protected surface "go/internal/phases/ship/gitops.go" archetype=plan duration_ms=121183 verdict=FAIL
[ship] ship.warning WARN SHIP_LANDING_PUSH_REPAIR_DECLINED cycle=1641 phase=ship seq=76 origin=Landing.Push — push rejected; inline fetch + ff-retry declined at fetch branch=main probe=fetch step=push
[ship] ship.error WARN SHIP_GIT_PUSH_REJECTED cycle=1641 phase=ship seq=77 origin=Orchestrator.recordShipError — ship: git push failed (rc=1); main is at abc123: <nil> branch=main class=transient git_rc=1 path=…/ship-error.json repair_outcome=declined stage=atomic-ship step=push
[ship] ship.error WARN SIGNALCENTER_UNREGISTERED_CODE cycle=1640 phase=ship seq=12 origin=Orchestrator.recordShipError — main moved during the landing class=transient drift=SIGNALCENTER_UNREGISTERED_CODE raw_code=SHIP_NEW_THING stage=atomic-ship
[orchestrator] system.failure INCIDENT ORCHESTRATOR_SYSTEM_FAILURE cycle=862 seq=88 origin=cycleRun.completeCycle — verdict-incoherence: recorded FAIL but on-disk audit=PASS, acs=PASS category=verdict-incoherence halt=true level=system
[orchestrator] cycle.sealed WARN ORCHESTRATOR_CYCLE_FAILED cycle=862 seq=89 origin=cycleRun.completeCycle — final verdict FAIL final_verdict=FAIL phases_run=5 termination_reason=audit-fail-floor
```

The last line shows drift stamped in place. The producer's module, kind, origin and reason stay. The
drift code replaces the unregistered code, and `fields.drift` / `fields.raw_code` say what was
wrong. This is one line, where the operator is already looking.

**Triage in a few review steps, kind-first** *(review 8)*:

1. Run `grep -E '"kind":"(system\.failure|phase\.aborted|gate\.rejected|ship\.error|quota\.paused)"' signals.ndjson` — the terminal kinds (§5.2). Run `grep INCIDENT` when something halted.
2. The `code` names the rule, and `module`/`origin` name the function.
3. `fields.path` names the artifact.

A WARN budget keeps this honest. A healthy PASS cycle emits ≤ N WARN signals. N = 0 on the unit-level green cycle (`TestRunCycle_EveryDispatchedPhaseEmitsOnePhaseOutcome_GreenCycleStaysUnderTheWarnBudget`). The live
budget is measured from the first green runtime cycle after S1 deploys and pinned in S2. If a green
cycle cannot stay under budget, the ladder is decoration.

Hand-written `[x]` prose disappears one module at a time (§10 S5).

## 10. Migration slices

| Slice | Depends on |
|---|---|
| **S0** | — |
| **S1** | PR #577 (lands after the #575/#576/#577 merge order) |
| **S2a** | S1 |
| **S2b** (contract-gate producers landed, see §15.5; the rest stays in S2c) | S2a, PR #575, PR #576 |
| **S3** (landed, see §15.3) | S1 |
| **S4a** (landed, see §15.4) | S2a, S3 |
| **S4b** | S4a |
| **S5** | S1 |

**S0** content: this design, ADR-0101, the inventory.

**S0** acceptance (measured by tests, not prose): a docs-only PR; links resolve; the architect review is folded in.

**S1** content:

- `internal/signalcenter` (schema, Center, registry, sinks, filter).
- `core.WithSignalCenter` + listener + `Orchestrator.SignalSummary()`.
- `orchDeps.Signals`.
- The C1 chokepoint emits `phase.outcome`/`phase.aborted` on both roots (the sink line replaces its PR #577 hand-written line).
- `.apicover-enforce` entry.
- CI line-coverage gate.

**S1** acceptance (measured by tests, not prose):

- Package 100 % API coverage (apicover) + 100 % line coverage (CI/make gate, §11), `-race`.
- Composed RunCycle test: every dispatched phase (with the ADR-0044 abort-path table) yields exactly one `phase.outcome` that the orchestrator listener observes. It also yields one `signals.ndjson` line with monotonic `seq`.
- WARN-budget test.
- Re-entrancy test.
- Mutants, each killed by name: emit removed, listener not subscribed, sink not attached, validation bypassed, recover removed, same-order-for-all-listeners.

**S2a** content (producers that need only S1):

- `system.failure` + `cycle.sealed` at `cycleRun.completeCycle` (the closeout both roots share; the hand-written SYSTEM-FAILURE HALT / LANDING LOST lines deleted).
- `ship.error` at `Orchestrator.recordShipError` with `shiperr.SignalCode`. Every ship code is registered under module `ship` with a doc each, and a source-parsed test proves completeness. `ShipErrorClass.SignalSeverity` is the one home of the class → severity rule.
- `quota.paused` at `cycleRun.pauseForQuota` (the seam both roots reach; its hand-written WARN line deleted).
- An abnormal exit seals the cycle FAIL from `cycleRun.abnormalEpilogue`.
- `Center.Flush` deferred at both roots.
- `evolve signals codes generate|check` projects the registry into `docs/architecture/signal-codes.md`.

**S2a** acceptance (measured by tests, not prose):

- Composed `RunCycle` test: `cycle.sealed` is the LAST orchestrator event.
- Each producer has a direct proof (INCIDENT on halt / integrity, WARN otherwise).
- `signal-codes.md` currency is a `cmd/evolve` test (CI).
- Mutants: each emit removed, INCIDENT not raised, prefix wrong, Flush broadcast removed, drift check disabled.

**S2b** content:

- Contract gate → `gate.passed/rejected` + the `gate.corrected` of the ladder on both roots (the `GATE_CONTRACT_*` codes; PR #575 landed).
- `fields.shipped` on `cycle.sealed` (needs `CycleState.Shipped` of PR #576).
- `failurelog.Classification` folded into `failureadapter`'s.
- The live WARN budget pinned from the first green runtime cycle.

**S2b** acceptance (measured by tests, not prose):

- Each producer has a composed proof.
- The classification registry test fails if the two vocabularies diverge again.

**S3** content:

- Bridge: `Deps.Signals` at construction + `engine.SignalsWired()`.
- The production Adapter takes the Center as a constructor argument (`adapters/bridge.NewDefault(projectRoot, signals)`). Every other call site passes an explicit `nil`, pinned. The Adapter threads the Center into every engine.
- Engine telemetry warnings → `bridge.warning` (`BRIDGE_TOKEN_RESOLVER_MISSING/_FAILED`, `BRIDGE_TOKEN_USAGE_WARNING`, `BRIDGE_CONTEXT_FILL_HIGH`, `BRIDGE_TELEMETRY_APPEND_FAILED`).
- The tripwire → `bridge.tripwire` (`BRIDGE_TELEMETRY_TRIPWIRE`).
- **Commit 1:** the pure rename `panestream.SignalCenter` → `LivenessCenter` (ADR-0068/0070 amended).
- **Commit 2:** `pane.liveness` from a `LivenessHandler` that the tmux driver registers per dispatch (module `liveness`, `LIVENESS_PANE_STAGNANT/_HUNG/_EXHAUSTED`).
- The hand-written `[engine] WARN` / `[engine] TRIPWIRE` lines removed.

**S3** acceptance (measured by tests, not prose):

- Wiring proofs at the engine, the Adapter (deps + real factory) and the root.
- Every producer has a direct proof.
- The telemetry suites assert what the WARN-filtered sink of the root renders.
- Mutants: signals not threaded, wired-always-true, handler not registered, hung not WARN, tripwire/warn/engine-warning not emitted, root passes nil, state name lost, format characters surviving.

**S4a** content:

- The loop module's producers at their seams: ONE `loop.halt` INCIDENT per batch halt. Its code is the rule of the caller.
  - `haltOnSystemFailure` is the one chokepoint: `LOOP_SYSTEM_FAILURE_HALT` for a halt that the cycle signalled, `LOOP_PIPELINE_BLOCKER_HALT` for the blocker breaker.
  - A fleet lane's halt code → `LOOP_FLEET_LANE_HALT`.
  - A wave-boundary halt → `LOOP_HALT`.
- `loop.wave` (INFO for the wave summary that the report also prints; WARN `LOOP_MIN_WIDTH_REPAIR`) and `loop.escalation` WARN `LOOP_ESCALATION_BOUNDARY`.
- The hand-written `[loop] … HALT` and escalation lines deleted.
- The append observer of the file ledger (`ledger.New(evolveDir, ledger.WithSignals(signals))`, module `ledger`) at the root:
  - `ledger.appended` INFO per entry through `Append` (the records of the orchestrator, the stop_review of the bridge, the inbox lifecycle lines, the seal anchor).
  - WARN `LEDGER_APPEND_FAILED`.
- The ledger of the root is threaded into the failed-cycle inbox walk.
- `cmd_loop` **reports** the per-cycle `SignalSummary` of the driven runner in the batch report (`[loop] cycle N signals: …`, a report line, never a gate) through the `loopCycleRunner` seam.
- ONE sink topology (`newRootSignalCenter(root, evolveDir, console)`) for production and the stub root, with a durable batch-level file for cycle-less signals.

**S4a** acceptance (measured by tests, not prose):

- Every producer has a direct proof.
- The window/escalation/min-width suites assert the sink-rendered line through the stub root (the production topology).
- A go/ast guard inventories every ledger line writer.
- Mutants:
  - each emit removed
  - INCIDENT demoted
  - a second breaker INCIDENT
  - ledger not observed
  - lifecycle not observed
  - ledger failure not WARN
  - root ledger unobserved
  - inbox walk on a self-built ledger
  - report silent / read off the seam
  - console writer ignored
  - cycle-less signals dropped

**S4b** content:

- `dispatchevents` writers and the `observer` adapter emit through the Center.
  - The Center accessor of the adapter and its two fault codes `OBSERVER_EVENTS_SINK_OPEN_FAILED` / `OBSERVER_WATCHER_LEAKED` landed in breakdown unit 12.
  - The detection stream (`stall_no_output`, `stall_probe_active`, started/stopped) still enters here.
- Their files stay as sink outputs until readers migrate (`subagent.AppendAbnormalEvent` folded).
- The dashboard SSE subscribes.

**S4b** acceptance (measured by tests, not prose):

- `abnormal-events.jsonl` **field-equal modulo timestamp precision** before/after, asserted by a decoding golden *(review 11)*.
- The dashboard shows a signal within one SSE tick.
- No breaker gates on a signal (a shadow comparison test may log disagreement).

**S5** content:

- Per-module log migration that rides each decomposition slice (§12). Prefix-literal occurrences per module (any writer): `[orchestrator]` 287 → 0, `[loop]` 132 → 0, `[ship]` 130 → 0, …
- Unit 13 removed four `[loop]` literals and sixteen `[chain]` lines from the two loop schedulers. The two `[loop]` literals of fleet are its F2.

**S5** acceptance (measured by tests, not prose):

- One repo-wide grep test with a shrinking allowlist (a migrated module cannot be forgotten).
- The inventory's prefix table regenerated.

## 11. Test plan and engineering bar

- **TDD, red first, every slice.** For each behavior in §6, write a table-driven test and watch it
  fail before the implementation. Bug-shaped tests reproduce the live incidents. The triage
  rejection of cycle 1636 produces the exact `phase.outcome` above. The closeout of cycle 1630 produces `cycle.sealed`
  with `shipped=false`.
- **Coverage** *(review 13)*. `internal/signalcenter` joins `.apicover-enforce` (0 uncovered exports,
  0 false-greens, godoc on every export — the in-test API gate stays as today). The **100 % line
  coverage is enforced in CI/`make`** (`go test -count=1 -coverprofile` + a threshold step beside
  the apicover gate). The 100 % line coverage is never enforced by a test that reads its own profile. Such a test skips silently.
- **Race.** `go test -race ./internal/signalcenter/` with concurrent emitters and (un)subscribers. A
  `core` `-race` test with evaluate-batch goroutines emitting into the summary of the orchestrator.
- **Named tests added on review:**
  - re-entrant listener completes (no deadlock) and its nested event follows by `seq`
  - WARN budget on a green cycle
  - two-process append (a resumed cycle appends with a different `pid`, both orders reconstructible by `pid`+`seq`)
  - registry conflicts empty
  - import graph (`signalcenter` imports only `internal/log`)
  - nil-Center construction sites pinned
  - the repo-wide `[x]` prefix allowlist
- **Mutation proof** per slice, each mutant confirmed to build:
  - emit removed at a producer
  - listener not subscribed
  - sink not attached
  - validation short-circuited
  - recover removed
  - "listeners observe different orders" (not "registration order reversed" — the invariant is sameness)
- **Wiring proof** at the composition root for every injection (`SignalCenterWired`, `SignalsWired`,
  `orchDeps.Signals`), at both roots.
- **Clean-code limits enforced by tests:**
  - every file in the package < 800 lines (the apicover DoD already checks this)
  - every function < 50 lines (a `go/ast` test over the package)
  - nesting ≤ 4
- **Reviewer fleet** on every slice: simplifier → architecture-reviewer ∥ go-reviewer. CRITICAL blocks.
  The S1 PR is re-reviewed specifically against review findings 2, 3, 5, 7 and 17. These findings turn
  into live deadlocks, dead wiring or unfalsifiable claims if they stay prose.

## 12. Decomposition order

The Center is the enabler. On day one, an extracted unit gets these things:

- its `Module` tag
- its code namespace
- its producer tests
- its place on the prefix allowlist

**Cut criterion** *(review 21)*: `core` first (the orchestrator is where every fact converges and where the mutex now lives), then
by size. The audit package waits for a later wave. Its files are gate logic under
the coherence floor of ADR-0072 and need their own campaign note. The exception is the defect ledger, pulled
forward as unit 09 (its schema was spelled in three homes and the `audit` tag had no producer).

| Order | Size today | Module tag |
|---|---|---|
| 1 | 1070 | `orchestrator` → `outcome` |
| 2 | 1144 | `advisor` |
| 3 | 1158 | `orchestrator` |
| 4 | 904 | `orchestrator` |
| 5 | 1006 | `inbox` |
| 6 | 989 | `ship` |
| 7 | 962 | `config` |
| 8 | 1578 | `bridge` |
| 9 | 882 | `audit` |
| 12 | 601 | `observer` |

Unit to extract (from) and notes, by order:

- **Order 1:**
  - Unit 01 landed 2026-09-13 ([decomposition/01-outcome-recorder.md](decomposition/01-outcome-recorder.md)): the C1 recorder → `internal/core/outcome`.
  - Unit 02 landed 2026-09-13 ([decomposition/02-failure-diagnostics.md](decomposition/02-failure-diagnostics.md)): the failure-diag sidecar writer and the delivery-failure classifier → `internal/core/failurediag`.
  - Unit 03 landed 2026-09-13 ([decomposition/03-carryover-lifecycle.md](decomposition/03-carryover-lifecycle.md)): the carryover-todo lifecycle → `internal/core/carryover`, three sibling files deleted.
  - Unit 03b landed 2026-09-13 ([decomposition/03b-failure-learning-engine.md](decomposition/03b-failure-learning-engine.md)): the failure-learning engine → `internal/core/failurelearning`. `recordFailureLearning` split into a pure gate + four steps (`failure_learning.go` 823 → 566 → 318).
  - Unit 11 built 2026-09-14, out of §12 order ([decomposition/11-phaserunner.md](decomposition/11-phaserunner.md)): the verdict engine of the phase runner → `internal/phases/runner/verdict`. It is the first `runner` producer (`runner.warning`, five codes; `runner.go` 719 → 500, `reconciliation.go` + `classification.go` deleted). It jumped ahead of the advisor and orchestrator rows on the cycles-1634/1636 F4 triage class.
  - Notes: already the C1 chokepoint; S1 touches it — extract after S1 lands.
- **Order 2:**
  - Unit 04 landed 2026-09-14 ([decomposition/04-advisor.md](decomposition/04-advisor.md)): phase advisor → `internal/core/advisor` (`phase_advisor.go` 1133 → 373, the seam). The brain, with its dispatch shell, has its I/O behind injected seams. The "pure decision logic" note was false: four I/O sites exist.
  - Notes: the two rune-cap text rules hoisted into `internal/textcap`; six `ADVISOR_*` codes on `advisor.warning`.
- **Order 3:** `orchestrator.go` (1158): composition (`New`, options) vs `RunCycle` engine. Notes: Facade over the extracted units.
- **Order 4:** `cyclerun.go` (904): finalize/closeout → `cycleclose`. Notes: `finalizeCycle` + `finalizeOutcome` + dossier.
- **Order 5:**
  - Unit 06 landed 2026-09-14 ([decomposition/06-inboxmover.md](decomposition/06-inboxmover.md)): the inbox lifecycle mover → `internal/inboxmover/lifecycle` as ONE leaf of eight files. The "three units" became three files of one package that share the ledger line and the item primitives.
  - `inboxmover.go` 993 → 372 (the seam: Options, the resolved defaults, the one construction, the facades).
  - Notes: claim/release/promote as three units.
- **Order 6:**
  - Unit 07 landed 2026-09-14 ([decomposition/07-shipgitops.md](decomposition/07-shipgitops.md)): the landing → `internal/phases/ship/landing` behind the seam `gitops_landing.go`. The landing is the ff-merge, the push with its inline repair and the post-push head read, the shared git probes, and the binding writer.
  - `gitops.go` 989 → 935, `repair.go` 474 → 402, `worktree_ship.go` 205 → 184. The staging guard and the run-scope policy remain (07b / 07c).
  - Notes: landing vs binding writer vs staging guard.
- **Order 7:**
  - Unit 08 landed 2026-09-14 ([decomposition/08-config.md](decomposition/08-config.md)): `config/config.go` (962 → 229) decomposed in place — `Loader` with an injected reader and Center, `config.warning`, six `CONFIG_*` codes.
  - Notes: typed policy structs (`PolicyStages`). The "retire env flags" by-product is OUT of scope (the nine keys are prose-contracted in the protected flagregistry table — its own slice).
- **Order 8:**
  - `bridge/engine.go` (791) + `autorespond.go` (787) — unit 10 landed 2026-09-14 ([decomposition/10-bridgeengine.md](decomposition/10-bridgeengine.md)): the launch-outcome classifier → `internal/bridge/launchoutcome` (the exit table, the cause miners).
  - `Launch` split in place. The request gauntlet stays in the host as `bridge.ValidateRequest`. `autorespond.go` and the remaining `[bridge]` lines are unit 10b.
  - Notes: rides S3.
- **Order 9:**
  - Unit 09 landed 2026-09-14 ([decomposition/09-defectledger.md](decomposition/09-defectledger.md)): `phases/audit/defect_ledger.go` (882) → `internal/core/defectledger` (882 → 392 — the seam and the citation resolver stay).
  - Notes: pulled forward from the later wave. The three schema homes of unit 03 F13 collapse onto one leaf. 13 `AUDIT_LEDGER_*` codes on `audit.warning`.
- **Order 12:**
  - Unit 12 landed 2026-09-14 ([decomposition/12-phaseobserver.md](decomposition/12-phaseobserver.md)): the phase observer — `internal/phaseobserver/phaseobserver.go` (601) → the clock-stepped engine `internal/observerengine`. The engine has a tail/decoder, stall rules, an incident responder, and envelope/report sinks. The host is a Strangler seam.
  - The live `adapters/observer.CoreAdapter` is wired to the Center for its own two faults. The layout is projected from `observerengine.PathsFor`.
  - Notes: eight `OBSERVER_*` codes; the adapter's detection stream stays S4b's.

Method per unit (named):

- **Strangler Fig** (new unit beside the old, callers moved one by one, old deleted)
- **Extract Class / Extract Function** (Fowler)
- **Facade** for the remaining public surface
- **Dependency Injection** through the existing `Option` pattern

Static feature flags met in a unit are retired in that slice. Typed policy resolves at the root, and `docs/architecture/control-flags.md`
shrinks per slice. No separate flag pass exists. Each unit ships with its own error-code table, its module
tag, 100 % API + line coverage, and a design note "why this boundary" appended to this document.

## 13. Why this is better than today

| Dimension | Today | After |
|---|---|---|
| Where to look first | 15+ files/streams per cycle, different shapes | `signals.ndjson` (one file, `seq`-ordered), stderr (one format, WARN and up) |
| Severity | 3 vocabularies | 1 (the existing contract) |
| Error identity | the ship error codes, 2 classification vocabularies, exit codes, free strings, prose | 1 registry, module-namespaced, unregistered/missing = visible drift |
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
| Emit re-entrancy / self-deadlock | one queue, one drainer, no lock held during delivery; self-reports and nested emits enqueue like any other event; a timeout-guarded re-entrancy test |
| Synchronous fan-out latency at hot sites | listeners are O(1) or append-only with an open handle; a slow consumer buffers internally; `signalcenter.slow_listener` WARN when a listener exceeds a budget (S4) |
| `signals.ndjson` growth | one line per event ≤ 4 KiB, ≤ 12 fields; gc retention already prunes run dirs |
| Name clash with the bridge center | S3 renames it `LivenessCenter` in its own commit; ADR-0101 amends ADR-0068/0070 |
| A listener panic taking the orchestrator down | recover + self-report; tested |
| Vocabulary sprawl (kinds/modules added ad hoc) | closed sets with tests; adding one is a reviewed edit |
| Registry conflicts | recorded at package init, asserted empty by a named test; never a panic, never an event |
| Monitoring becoming a control input | the observe-never-decide invariant (§3, §8); S4 reports, does not gate |

## 15. S1 implementation checklist

1. RED: `signalcenter` tests for §6 semantics 1–9 and the §4 rules (required fields, `Origin` regex, `Fields` bounds, line cap).

   Write tests also for the JSON golden line, the stderr golden line, registry conflicts and the import graph.
2. GREEN: `event.go`, `center.go`, `registry.go`, `sinks.go` (< 800 lines each, functions < 50).
3. RED: `core` — `WithSignalCenter`, `SignalCenterWired`, `SignalSummary`, `observeSignal` (lock order; the summary follows the cycle by itself).

   The C1 chokepoint emits on every terminal path (reuse the abort table of `orchestrator_phaseoutcome_test.go`). Test the WARN budget on a green cycle.
4. GREEN: `recordPhaseOutcome` → `Emit`.

   Delete its PR #577 `Fprintf` line (the sink renders it). Update `TestRecordPhaseOutcome_CarriesThePhaseDiagnosticsAndNamesAReasonedFail` to assert the sink line format.
5. RED/GREEN: composition root. Construct the Center before the bridge engine.

   Add `orchDeps.Signals`, the NDJSON sink + `Filter(StderrSink, WARN)`, `SignalCenterWired` at both roots, and the nil-site pin test. Add the `.apicover-enforce` entry and the CI line-coverage gate.
6. Mutants (each confirmed to build): emit removed, subscribe removed, sink not attached, validation bypassed, recover removed, listeners-see-different-orders.
7. Floors: gofmt/vet/`-race`; whole-module; integration; ACS; apicover.
8. Fleet: simplifier → architecture-reviewer ∥ go-reviewer (re-review against review findings 2, 3, 5, 7, 17).

   Then run the commit-gate and ship. Open the PR after #575/#576/#577.

### 15.1 Landed — S1 as implemented (2026-09-13)

Every item above is done. These are the deltas from the plan, each chosen during implementation
and folded back into §4–§9, so this document describes the code that exists:

| Planned | Landed | Why |
|---|---|---|
| Emit-mutex fan-out + an internal `dispatchLocked` for self-reports | one queue, one drainer, no lock held during delivery. Self-reports enqueue like any event (§6.1, §6.3) | one way in means one order and no second dispatch path to get wrong. `TestEmit_AllListenersObserveTheSameOrderUnderConcurrency` kills the mutant "listeners observe different orders" |
| package-level `NDJSONSink` | `(*Center).NDJSONSink` | the sink reports its own drops through the Center it serves |
| separate `signalcenter.registry_drift` / `registry_conflict` events | the drift is stamped onto the offending event (`registry_drift` is only the replacement kind for an unknown kind). Conflicts are recorded only, with no `registry_conflict` kind (§5.2, §5.4) | drift is one line next to the reason of the producer. Conflicts happen at package init, before a Center exists |
| summary reset in `planCycle`; `CycleResult.Signals` | `signalcenter.Summary` follows the cycle by itself. `Orchestrator.SignalSummary()` is the only reader (§8) | no path can forget the reset. One owner, one snapshot, no copy on the result |
| `log.DiagnosticField` for field values | `log.SanitizeField` for values and `Reason`. `FormatLine` is exported (§4, §9) | a signal line is never quoted twice. Sinks share one format |
| WARN budget N measured live | N = 0 on the unit green cycle; live N pinned in S2 | S1 has no runtime cycle yet |

Folded from the S1 architecture review (Approve; four MEDIUM):

- The durable sink releases its own lock before it reports drops. A `Listener` is a plain func and can run outside a drain. `TestNDJSONSink_DropReportIsEmittedOutsideTheSinkLock` reproduced the self-deadlock first.
- The `listener_panicked` INCIDENT names the listener **function** (`fields.listener`, for example `core.(*Orchestrator).observeSignal-fm`) beside its id.
- `verdictReason(verdict, diags)` is the one rendering of "verdict=<V>[: reasons]" for the floor error, the seal and the FAIL **and** WARN events.
- The registry-clean assertion lives in `cmd/evolve` (`TestSignalCenterRegistry_EveryLinkedModuleRegistersCleanly`), the binary that links every producer module. The leaf package's own test cannot see them.

Folded from the S1 Go review (Approve; one MEDIUM): the ≤ 4 KiB line cap only looked at `Fields`. Before the fix, a regex-valid but over-long `Origin` or a hostile `Reason` broke the atomic-append promise. Identifiers are now bounded and `Reason` is cut to fit (§4).

Proof carried by the S1 PR:

- `internal/signalcenter` 100 % line coverage (`make cover-strict`, in CI) and 98/98 exports API-covered.
- `core` 343/343.
- `-race` ×3.
- Nineteen mutants killed by name:
  - emit removed
  - subscribe removed
  - NDJSON sink not attached
  - orchestrator not wired
  - WARN branch unreachable
  - FAIL kept INFO
  - cycle not stamped
  - abort not distinguished
  - summary lock removed
  - zero-summary init removed
  - stderr filter dropped
  - stderr sink not attached
  - sink truncates instead of appending
  - leaf imports core
  - core registers a conflicting code
  - listener name blank
  - sink lock held across Emit
  - reason cut removed
  - identifier bound removed
- Named wiring tests: `TestWireOrchestratorDeps_SignalCenterWired`, `TestWireOrchestratorDeps_SignalCenterConsoleSinkIsFilteredAtWarn`, `TestNilSignalCenterRootsArePinned`, `TestImportGraph_LeafPackageImportsOnlyInternalLog`, `TestNDJSONSink_TwoProcessesAppendToTheSameCycleFile`.

### 15.4 Landed — S4a (2026-09-13)

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `loop.halt` | `haltOnSystemFailure` — the ONE shared halt+escalate action. The sequential path, the cycle-run root and the blocker breaker call it. It emits ONE INCIDENT whose code is the `loopHaltRule` of the caller | INCIDENT `LOOP_SYSTEM_FAILURE_HALT` (a halt the cycle signalled) or `LOOP_PIPELINE_BLOCKER_HALT` (the breaker's rule, + `rule`, `fingerprint`) | `category`, `level`, `next` (the dossier's own `next_action` — its one home), `escalation`, `inbox_item` (what the halt wrote) |
| `loop.halt` | `loopBatchCoordinator.fleetHaltDecision` — a fleet lane exited with the halt code; the reason points at the lane's own `LOOP_SYSTEM_FAILURE_HALT` instead of restating its dossier | INCIDENT `LOOP_FLEET_LANE_HALT` | `kind`, `iteration`, `rc` |
| `loop.halt` | `loopBatchCoordinator.probeSyncAndPublish` (origin corrected by unit 13, was the stale `runWaveIteration`; moved from `prepareIteration` with the pre-wave work on 2026-09-29) — the wave-boundary sync refused (plane diverged) | INCIDENT `LOOP_HALT` | `stop_reason` |
| `loop.halt` | `prepareFreshBatch` (loop boot, before the readiness gate) and `loopBatchCoordinator.updateCLIsAtBoundary` (each later wave boundary, after the plane sync) call `haltOnCLISmokeFailure` (2026-10-05, [model currency C4](../plans/model-currency-2026-10.md)). A boundary CLI update left a family whose `doctor live` smoke failed | INCIDENT `LOOP_HALT` | `stop_reason` (`cli_update_smoke_halt`), `families`, `versions` (`family=new`) |
| `loop.wave` | `loopBatchCoordinator.completeWave` (summary; the `[loop] wave N: x/y lanes ok` report line stays — INFO never prints). The other producers are the wave engine's `Engine.RepairMinWidth`, `Engine.Dispatch` and `gatedLauncher.Run`. Unit 13 covers `LOOP_MIN_WIDTH_REPAIR` (re-homed), `LOOP_WAVE_DISPATCH_FAILED{path, step}`, `LOOP_WAVE_EMPTY_PLAN{cause}` and `LOOP_WAVE_ALL_LANES_STALE`. The producer is `loopwave.EmitWave`, and `emitLoopWave` is its projection | INFO / WARN | `wave`, `lanes_ok`, `lanes` / `desired`, `realized`, `path`, `step`, `error`, `cause`, `planned`, `skipped` |
| `loop.warning` / `loop.halt` | the chain engine (unit 13): `Refresher.Refresh` (`LOOP_BOUNDARY_REFRESH_SKIPPED{step}`, `LOOP_BOUNDARY_REFRESH_AUDIT_FAILED`) and `Driver.Run` (`LOOP_CHAIN_INBOX_ITEM_INVALID`, `LOOP_CHAIN_QUOTA_DEFER` on `loop.warning`; `LOOP_CHAIN_INBOX_UNREADABLE`, `LOOP_CHAIN_BATCH_ERROR` on `loop.halt`). The chain root builds its own batch-level Center (`runLoopChain`) and flushes it before every batch. A chained batch can re-exec at a wave boundary, and it flushes only its own Center. The refresh flushes the Center that it received before every re-exec | WARN / INCIDENT | `batch`, `step`, `commit`, `error`, `name`, `path`, `rc`, `stop_reason`, `exit`, `cycle`, `wake_at`, `source` |
| `loop.escalation` | `applyEscalationBoundary` | WARN `LOOP_ESCALATION_BOUNDARY` | `stage`, `bumped`, `filed`, `skipped`, `planned`, `refused` (since 2026-10-06: an autofile intent with no `priority_class`, each also printed as `[loop] WARN escalation boundary: refused <id>: no priority_class`) |
| `ledger.appended` | `FileLedger.Append` — the append observer of the file ledger. The construction option `ledger.WithSignals(signals)` installs it, and the root passes the option to `ledger.New`. Every entry writer reaches this chokepoint (`AppendLifecycle`, the segment anchor of the seal, the stop_review of the bridge, the records of the orchestrator). The file ledger still chains and locks | INFO; WARN `LEDGER_APPEND_FAILED` when the append errors (the error still returns) | `role`, `kind`, `exit_code`, `path`, `entry_seq` (the line the signal names) |

The batch report reads the view of the driven runner. After each sequential cycle `cmd_loop` prints
`[loop] cycle N signals: T total, W WARN, I INCIDENT (last INCIDENT CODE — reason)` from
`SignalSummary()` on the `loopCycleRunner` seam. The seam is the real orchestrator in production and a scripted
runner in tests: one seam, no second identity for the collaborator. The line is a report line (the loop
reports, never gates; §8).

Wiring: `newRootSignalCenter(root, evolveDir, console)` is the ONE sink
topology. Production (`wireOrchestratorDeps`) and the stub root of the loop tests call the same
constructor. A test that asserts a rendered INCIDENT line therefore proves the topology of production. The
console sink renders into the stderr writer of the command (production passes `os.Stderr`).

Cycle-less signals are durable in `<evolveDir>/signals.ndjson` (see §6). The production topology of the stub root
caught the danger of a drop report from the wave summary after every wave.

Deleted
1:1: the three-line `[loop] SYSTEM-FAILURE HALT` message, `[loop] PIPELINE-BLOCKER HALT`, the
fleet-lane halt line, `[loop] HALT: …`, and the escalation-boundary line. The min-width repair
line became the reason of the WARN.

`fields.next` on a halt IS the `next_action` of the dossier
(`writePipelineEscalation` returns what it wrote; the prose is stated once). The fleet-lane halt
points at the own INCIDENT of the lane. Under `--simulate` the root builds the same topology since unit 01
(ADR-0103). A system-failure halt therefore renders on the console there exactly as in a real cycle.

E12 of [ADR-0127](adr/0127-push-only-event-channels.md) retires the project-level `<evolveDir>/signals.ndjson` (see §7).

**The ledger, after the architecture review (HIGH-1).** The first cut was a Decorator that embedded
`*FileLedger` and overrode `Append`. Go embedding is delegation without virtual dispatch. The
promoted `AppendLifecycle` called `FileLedger.Append`, never the override. Every inbox
lifecycle line, and the segment anchor of the seal, was therefore appended unobserved. The inbox mover
also builds its own `FileLedger` over the same file whenever no `Ledger` is supplied, and no
production caller supplied one.

Landed instead: an **append observer at the write chokepoint**.

- The construction option `ledger.WithSignals(signals)` installs it (`ledger.New(evolveDir, opts...)`,
  functional options). `FileLedger.Append` is the ONE path every entry writer reaches, so
  promotion cannot route around it.
- The failed-cycle inbox walk (`cycleoutcome.ApplyFailure`) receives the ledger of the root. The cycle-run root and both sequential loop paths reach it through the one
  `applyCycleFailureOutcome`. `orchDeps.Ledger` is the `rootLedger`
  interface: `core.Ledger` + the `LedgerAppender` of the mover, one object, one identity.
- A go/ast
  guard (`TestFileLedger_EveryLineWriterReachesTheAppendChokepointOrIsInventoried`) pins every line
  writer inside the package to the chokepoint or to
  its inventory with a reason. It follows method and plain-helper calls alike. The inventory holds `Rebaseline`, an operator repair root, and `WriteCompositionVerdict`, a
  composition record by a self-constructed ledger.
- `TestUnobservedLedgerRootsArePinned`
  (cmd/evolve, the twin of the pinned nil-Center roots) pins every `ledger.New(` outside the package that
  passes no `WithSignals`, with its count and reason. These are the `evolve cycle reset` seal, the
  `evolve ledger` repair commands, the read-only ledger of the guard chain, and the
  fallback of the inbox mover. The post-ship mover of the ship phase and the operator inbox commands
  still reach that fallback.

Both inventories are the code-homed list of ledger lines that the Center does not see.
S4b's DI work shrinks them. A cycle-signalled halt is two records by design. The first is the
detection of the orchestrator (`system.failure`, S2a). The second is the halt action of the loop (`loop.halt`, carrying
the dossier fields).

Not in this slice: the wave-summary INFO path and the plane-diverged halt have no fleet fixture
(the wave dispatch needs subprocess lanes). The direct producer tests cover their producers, and the S4b/S5 fleet
fixture is the follow-up.

### 15.3 Landed — S3 (2026-09-13)

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `bridge.warning` | `NewEngine` — a missing token resolver at construction (cycle 0) | WARN `BRIDGE_TOKEN_RESOLVER_MISSING` | — |
| `bridge.warning` | `attemptLogContext.warn` — the ONE telemetry-warning writer the engine had (resolver failed, usage caveat, context fill over the threshold, ledger append failed) | WARN `BRIDGE_TOKEN_RESOLVER_FAILED` / `BRIDGE_TOKEN_USAGE_WARNING` / `BRIDGE_CONTEXT_FILL_HIGH` / `BRIDGE_TELEMETRY_APPEND_FAILED`; the old line's detail is the reason | `call_id`, `cli`, `agent` (+ `cycle`/`run`/`phase`/`attempt` on the event) |
| `bridge.tripwire` | `attemptLogContext.tripwire` from `emitTokenWarnings` — a successful try of a family with its own collector (claude, agy) past the threshold with no measurable usage | WARN `BRIDGE_TELEMETRY_TRIPWIRE` | `call_id`, `cli`, `agent`, `duration_ms` |
| `pane.liveness` | `paneLivenessHandler`, registered on the dispatch's `LivenessCenter` in `newReplWaitState` — one event per liveness EDGE (the center is edge-triggered) | INFO, or WARN with the `LIVENESS_PANE_*` code the registry lists (`signal-codes.md` is the one list of which states warn) | `session`, `state` (`LivenessState.String`, the vocabulary's one spelling — the timeout summary's snake_case word is a projection of it) |
| `bridge.warning` | `Engine.Launch` — the unit-10 chokepoint. It emits exactly ONE event per non-zero launch exit. The event follows the launch-error persist and the boot-strike record (breakdown unit 10, 2026-09-14) | WARN `BRIDGE_EXIT_<class>` — the signal column of the exit table (`launchoutcome.Outcome.Signal`). The classes are `_SAFETY_GATE` (2), `_COST_LEAK` (3), `_BAD_FLAGS` (10), `_REPL_BOOT_TIMEOUT` (80), `_ARTIFACT_TIMEOUT` (81, one code whatever the sub-cause), `_UNKNOWN_PROMPT` (85), `_RESPOND_LOOP_GUARD` (86). More classes are `_REQUIRED_TIER_UNAVAILABLE` (99), `_COMMAND_TIMEOUT` (124), `_MISSING_BINARY` (127), `_SIGNAL_DEATH` (-1) and `_DRIVER_ERROR` (unknown). The reason is the classified error string that the outcome record and the retry backoff parse | `call_id`, `cli`, `agent`, `step=classify`, `exit_code`, `cause_code` (the llm-calls.ndjson value, with an 81 sub-cause), `transient`, `ctx_cancelled`, `launch_error` (the persisted `<agent>-launch-error.txt` when written) |
| `bridge.warning` | `Engine.clearBootStrike` / `Engine.recordBootStrike` / `Engine.persistLaunchError` / `Engine.readResult` — the four best-effort steps of the Launch spine (unit 10). The first two replaced the last two `[engine]` stderr lines of the module 1:1. The last two were silent `_ =` / `err == nil` sites | WARN `BRIDGE_BOOT_STRIKE_CLEAR_FAILED` / `BRIDGE_BOOT_STRIKE_RECORD_FAILED` / `BRIDGE_LAUNCH_ERROR_PERSIST_FAILED` / `BRIDGE_RESULT_READ_FAILED`. None of them changes the classified error of the launch | `call_id`, `cli`, `agent`, `step` (`clear_boot_strike` / `record_boot_strike` / `persist_launch_error` / `read_result`), `path` (persist, read), `completion` (read) |

Wiring: `bridge.Deps.Signals` (nil = Null Object, tests only) → `Engine.SignalsWired()`.

The production Adapter is `adapters/bridge.NewDefault(projectRoot, signals)`: explicit DI at construction, no setter to forget. `productionEngineDeps` threads it into every engine it builds (`Adapter.SignalsWired`, proven on the deps AND through the real `engineFactory`). The root passes its Center (`TestWireOrchestratorDeps_SignalCenterReachesTheBridge`, through `orchDeps.Bridge`). Every other `bridge.NewDefault` call site — the eight phase-registry defaults, `cmd_campaign`, tests — passes an explicit `nil` (`TestNilSignalBridgeRootsAreExplicit`).

The dispatch identity that every bridge event carries comes from `BridgeRequest` (`Cycle`, `RunID`, `Agent` as the phase). There is one `dispatchIdentity` rule, and the Config of the driver carries the same values. There is no fallback: a request without a `Cycle` is an operator probe whose signals stay at cycle 0.

Deleted 1:1: `[engine] WARN: Deps.TokenResolver is nil`, the `[engine] WARN: <event> call_id=…` writer, `[engine] TRIPWIRE: …`. The telemetry test suites (tripwire, context fill, resolver warnings) now assert what the WARN-filtered stderr sink of the root renders. This is the one line format the operator reads. The suites no longer assert hand-written text.

`log.SanitizeField` also folds Unicode format characters (bidi overrides, zero-width joiners, the BOM). The old quoted field escaped them, and the sink renders plain. A resolver error can therefore no longer reorder a terminal line.

The rename (commit 1) is mechanical and reviewed on its own: `panestream.SignalCenter` → `LivenessCenter` (`NewLivenessCenter`, `LivenessEvent`, `LivenessHandler`, `RegisterLivenessHandler`), files moved, ADR-0068/0070 carry an "Amended by ADR-0101" note.

Remaining hand-written lines in the module are unit 10b's: `[bridge]` (sandbox, launch validation, dry-run, wall corroboration). The launch-validation and driver lines are the DATA that the unit-10 classifier mines, so they stay verbatim. The two `[engine]` lines (boot-strike clear failed, boot-timeout bench record failed) landed as `BRIDGE_BOOT_STRIKE_CLEAR_FAILED` / `BRIDGE_BOOT_STRIKE_RECORD_FAILED` of unit 10 (2026-09-14). None of them was a fact that S3 signalled.

Folded from the S3 architecture review (Block → fixed):

- The §5.2 severity cell above now defers to the registry instead of a restatement of which states warn (the copy had already diverged).
- Every bridge signal of one dispatch carries ONE derived `dispatchIdentity` (cycle, run, phase = the agent role) with no workspace-path fallback. A request without a `Cycle` is an operator probe whose signals stay at cycle 0.
- `event` takes its origin explicitly.
- `Deps.LivenessCenter` documents that an injected center must be per-dispatch (handlers accumulate; there is no unregister).

From the re-review: the ADR and this section no longer describe the deleted fallback. The `livenessOrUnknown` of the timeout summary is a projection of `LivenessState.String` (it had grown its own switch, missing `exhausted`).

Follow-up outside this slice: CI compiles only `acs/regression/...` under `-tags acs`. Widen it to `./acs/...` behind a shrinking allowlist of the four fossil packages, so a signature change can never break a frozen predicate silently again.

### 15.2 Landed — S2a (2026-09-13)

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `cycle.sealed` | `cycleRun.completeCycle` — the closeout both dispatch roots share, after `finalizeCycle`, so the FINAL verdict is known. It is the last orchestrator event of every completed cycle. An abnormal exit (the cycle died mid-phase) seals from `cycleRun.abnormalEpilogue` instead: FAIL, with the abort reason as `termination_reason`. "how did cycle N end" therefore has one answer on every path | INFO; WARN `ORCHESTRATOR_CYCLE_FAILED` on FAIL | `final_verdict`, `phases_run`, `termination_reason`, `retro_decision` (`shipped` arrives in S2b with `CycleState.Shipped`) |
| `system.failure` | `cycleRun.completeCycle`, before the seal, when any floor attached a `SystemFailureSignal` (incoherence, lost landing, audit-fail floor, decision branches) | INCIDENT `ORCHESTRATOR_SYSTEM_FAILURE` when `Halt`, WARN otherwise | `category`, `level`, `halt` |
| `ship.error` | `Orchestrator.recordShipError` — where core records the phase's typed error (`ship-error.json` + ledger) | WARN `SHIP_<code>` (`shiperr.SignalCode`); INCIDENT for the `integrity` class — `ShipErrorClass.SignalSeverity`, the rule's one home, beside the vocabulary | `class`, `stage`, `path` |
| `quota.paused` | `cycleRun.pauseForQuota` — the seam BOTH dispatch roots reach. The deferred guard of the resume root never calls the epilogue for a pause, so the epilogue was the wrong seam (S2a architecture review). Its hand-written `[orchestrator] WARN` line deleted | WARN `ORCHESTRATOR_QUOTA_PAUSED`; the reason is the one error text that the ledger and the phase diag carry | `phase` |

Design choices: every producer is an Adapter from a value that the pipeline already owns (`CycleResult`, `SystemFailureSignal`, `ShipError`, the `ErrAllFamiliesExhausted` sentinel). No new state and no new decision exists. Each chokepoint has one call line, and the body is in `signal_cycle.go` (`verdictReason`-style: one rendering per fact).

The ship vocabulary is projected, not copied (`SignalCode`, `AllCodes`, one `codeDocs` table registered at init). `TestCodeDocs_CoverEveryDeclaredShipErrorCode` parses `shiperr.go`, so a new constant cannot slip in undocumented. No hand-maintained count exists anywhere: the generated doc is the only place a count lives.

`Center.Flush` (`sync.Cond` on drain end) is deferred at both roots. The code catalogue is a generated projection of `RegisteredCodes()` (`RenderCodes` → `evolve signals codes generate|check`, `docs/architecture/signal-codes.md`). `TestSignalsCodes_RepoDocIsInSyncWithTheLinkedRegistry` in `cmd/evolve` — the binary that links every module — gates it.

The hand-written `[orchestrator] SYSTEM-FAILURE HALT`, `LANDING LOST` and quota-pause `WARN` lines were deleted in the same slice. The sink renders them in the one line format.

Folded from the S2a architecture review (Block → fixed):

- The quota producer moved from the epilogue (unreachable on the resume root) to `pauseForQuota`, with fresh-root, resume-root (integration) and direct proofs.
- The class → severity rule moved beside the vocabulary.
- The completeness proof is parsed from source.
- `emitCycleClose` is a `cycleRun` method, so every producer stamps the one run id (`cs.RunID`).
- An abnormal exit now seals the cycle.

From the re-review (Approve): `origin` is a parameter of the seal producer. The abnormal-path seal therefore names `cycleRun.abnormalEpilogue` (a callee never asserts the identity of its caller — `origin` is vocabulary). The ship-class severity table walks the declared classes from source like the codes do.

Accepted as-is: the textual Flush-wiring pin (convention-consistent; the behavioural proof is the Flush test). Also accepted: the `emit…` producer naming beside `emitQuotaPause` of the loop. The Signal Center producers keep one prefix, and S4 renames the report emitter of the loop.

### 15.5 Landed — S2b (2026-09-13): the contract gate reports, the ladder reports, the prompt states the gate's criteria

Operator direction (2026-09-13): *"orchestrator … should check if each phase deliverables match to
the policy through the signal center, it knows where to search and check if all the output docs /
codes are ready (without looking into the context, it just checked if files are legit and existed)
and by-pass to the next phase"*, and *"the pass criterions for gating the output should be sent to
phase agent as part of input … please verify"*.

**Finding.** The check existed and was wired on both roots (ADR-0100: `reviewAndGuard` →
`reviewWithCorrections` → `reviewDeliverable` → `deliverable.Reviewer.Review` → `VerifyWithStage`;
resume: `reviewResumedDeliverable`). It is deterministic Go, and no model reads the files. But its
decisions never reached the Center. `gate.contract`, `gate.rejected` and `gate.corrected` were
declared in the closed sets with **zero producers**, and the production streams held no gate event.

The prompt carried the contract block (artifact path, required sections or JSON keys, verdict
sentinel, the `evolve phase verify` self-check). But two ADR-0100 additions that the gate enforces lived only in persona prose:
the agent-owed secondaries and the declared effects. A registry
change was therefore a risk: a gate move without a prompt move was possible.

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `gate.passed` | `deliverable.Reviewer.Review` through `gatesignal.Reporter` (module `gate.contract`; origin `Reviewer.Review`) | INFO `GATE_CONTRACT_VERIFIED` (the reason names the artifact and its size, the owed files, the effects). INFO `GATE_CONTRACT_SALVAGED`. WARN `GATE_CONTRACT_WOULD_BLOCK` (shadow/advisory stage, or the report-size gate's). WARN `GATE_CONTRACT_DEMOTED` (breaker open: advanced UNVERIFIED). WARN `GATE_CONTRACT_FAIL_OPEN` | `artifact`, `bytes`, `owed`, `effects`, `stage` · `pattern` · `codes`, `salvage` · `blocks` |
| `gate.rejected` | same | WARN `GATE_CONTRACT_REJECTED` — the reason IS the correction directive (one `[code] message` per violation, from the ONE `summarize()`) | `codes`, `blocks`, `threshold` |
| `gate.corrected` | `Orchestrator.emitGateCorrection` (module `orchestrator`) from `cycleRun.reviewWithCorrections` (fresh root) and `Orchestrator.reviewResumedDeliverable` (resume root) — one per correction re-dispatch | INFO `ORCHESTRATOR_GATE_CORRECTION` | `correction`, `max`, `rung`, `cli`, `escalated`, `salvage_retry` |

**The record per phase boundary.** Three paths exist:

- `gate.passed` (verified: what was found where) → `phase.outcome` (the advance).
- `gate.rejected` → `gate.corrected` (1/2, redispatch, cli) → `gate.passed` → `phase.outcome`.
- `phase.aborted` after the ladder (its `abort_reason` names the file).

The batch report line adds `· gates: P passed, R rejected, C corrected` (`formatSignalReport`). The check
itself is unchanged: verdicts, transitions and the ladder stay with the floors. The Center only
carries the evidence (§8).

**Wiring.** `deliverable.WithSignals(signals)` — a functional option that the four reviewer constructors
accept — is set at `wireOrchestratorDeps`. The proof `Orchestrator.ContractGateSignalsWired()` asks the
chain for the capability (`VerifiesDeclaredDeliverables` + `SignalsWired`), the
`DeclaredDeliverablesGateWired` precedent, because core cannot name the deliverable type.

**The prompt.** `RenderContractBlockStage` (the cache-safe block) does two things:

- It names the agent-owed files and points at the tail for their paths ("Also write these agent-owed files at the EXACT paths listed under <owed-files> at the END of this prompt: … the gate verifies each exists, is non-empty and parses").
- It names the declared effects ("verified at the phase boundary; your instructions say how").

`RenderContractTail(c, artifactPath, workspace)` renders each owed file as `<owed-file>/abs/path</owed-file>` through `phasecontract.OwedPath(workspace, name)`. This is the ONE join that `verifySecondaries` of the gate reads through (basenames only; a declared separator never steers a read outside the workspace). It renders the effects as `<effects>`.

The bridge passes `req.Workspace`, not the directory of the artifact (a dispatched-artifact override can move the deliverable elsewhere). Names come from `phasecontract.Contract` (filled from the registry by `FromSpec`), and locations come from the one join. The gate and the prompt therefore cannot drift on either. Contracts without owed files or effects render byte-identical prompts (pinned).

**Patterns.**

- Observer at the ONE decision point. The gate stays the one verifier, and the reporter only reports (`internal/deliverable/gatesignal`, a leaf over `signalcenter`).
- Null Object (nil Center).
- Functional options.
- Single-source-with-projection: `summarize()` feeds the log line, the signal reason and the correction directive. `signal-codes.md` was regenerated — 84 codes.

**Kept / not done.**

- The `[contract-gate]` log lines of the gate and the `[orchestrator]` lines of the ladder stay (S5 retires them per module). One `summarize()` source feeds them, so they cannot drift from the signal.
- `gate.eval` / `gate.repo` producers — the other gate modules of the closed set — have the same shape. They are the next slice.
- A salvage rung emits no `gate.corrected` (only a re-dispatch is a correction).
- The rest of the original S2b row (`fields.shipped` on `cycle.sealed`, the classification fold, the WARN budget pin) stays in S2c.

**Tests.**

- `gatesignal`: 100 % lines, and every export named (apicover).
- The reviewer: one test per decision (verified, rejected, shadow `would-block`, report-size `would-block`, demoted, fail-open, salvaged, `would-salvage`, off and unwired silent). Another test covers the capability through the core interface.
- Core: one re-dispatch → one `gate.corrected` on the fresh root. The escalated second correction says so with its CLI. The ladder of the resume root.
- Cmd: the root wiring proof and the report clause.
- `phasecontract`: owed files and effects rendered in block and tail, no clause otherwise.
- The kind set closed at 22.
- Fourteen build-confirmed mutants killed by name:
  - the rejected, verified, demoted, fail-open and report-size `would-block` emits removed
  - the owed files and effects not projected into the verified event
  - the fresh-root and the resume-root ladder emits removed
  - the `escalated` field hard-coded
  - the root not passing its Center to the gate
  - the report clause never rendering
  - the owed-files clause never rendering
  - empty-valued fields kept
  - a rejection emitted as `gate.passed`

**Review folds (fleet: code-simplifier → architecture-reviewer ∥ go-reviewer; both Warning, no CRITICAL).**

- HIGH — the owed-file location was a belief with two homes (the `filepath.Join` of the gate, the "SAME directory" prose of the prompt). `phasecontract.OwedPath` is the one join, the tail renders the exact paths, and the block points at the tail.
- MEDIUM — the verified event re-resolved the contract instead of a report of what the gate checked. The verifier now fills `Result.Owed` / `Result.Effects` (json:"-", the single-read seam `Result.Content` set), and the signal projects them.
- MEDIUM — the ladder producer took ten positional parameters with two adjacent bools. The `gateCorrection` parameter object replaces them.
- MEDIUM — the report-size constructor assigned its settings after the options. `withReportSize` is now an option, applied at the one point.
- LOW / Go review — `Reporter.emit` filters into its own map. The correction ordinal has one home (`fields.correction`, no `Attempt`).

Seven more build-confirmed mutants (twenty-one in all) were killed:

- the join not stripping a directory
- the tail rendering the bare name
- the owed files or effects not recorded by the verifier
- the options applied before the report-size setting
- the caller's map mutated
- the gate reading a path that the tail did not render
