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
  core SystemFailureSignal ── system.failure ─┤                │     → per-cycle Summary (Orchestrator.SignalSummary())
  shiperr / ship phase     ── ship.error ─────┤   signalcenter │── NDJSONSink → runs/cycle-N/signals.ndjson  (everything)
  deliverable (gate)       ── gate.rejected ──┼──►  Center  ───┼── Filter(StderrSink, WARN) → "[module] kind SEV CODE …"
  bridge engine            ── bridge.warning ─┤   (sync,       │── cmd_loop batch report (counts)                 (S4)
  panestream LivenessCenter── pane.liveness ──┤    ordered,    └── dashboard SSE push                              (S4)
  ledger append observer   ── ledger.appended ┤    panic-
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
chokepoint maps its native record onto `Event`); **Registry** (module → codes, with docs); **Observer at the write chokepoint**
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
- `Fields`: keys match `^[a-z][a-z0-9_]{0,31}$`; at most 12 keys; values (and `Reason`) pass through
  `log.SanitizeField` (UTF-8 clean, control characters and U+2028/2029 folded to spaces, 512-rune
  cap); over-cap keys are dropped and `fields.truncated=<n>` is set. Rendering (JSON and stderr)
  sorts keys, so golden lines are stable. (`log.DiagnosticField` stays the quoted rendering for
  hand-written diagnostics until S5 retires them.)
- The rendered line is capped at 4 KiB (a single `O_APPEND` write stays atomic) **for any event**:
  a longer event drops fields largest-first and sets `fields.truncated`; if it is still over (JSON
  escaping can inflate a rune-capped `Reason` to six bytes a rune) `Reason` is cut to fit with a
  trailing `…`; `Origin`, `Phase` and `RunID` are identifiers bounded to 128 runes
  (`MaxIdentRunes`), so an empty `Reason` always fits and the cap provably terminates
  (`TestNormalize_LineCapHoldsForAnyEvent`).
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
{"schema_version":"signal/1.0","seq":58,"pid":30870,"ts":"2026-09-12T11:11:40.002Z","cycle":1630,"run_id":"01J…",
 "module":"orchestrator","origin":"cycleRun.completeCycle","kind":"cycle.sealed",
 "severity":"INFO","reason":"final verdict SKIPPED_UNKNOWN",
 "fields":{"final_verdict":"SKIPPED_UNKNOWN","phases_run":"2","termination_reason":"triage-empty-commitment"}}
```

## 5. Vocabularies

### 5.1 Module (closed)

`orchestrator`, `advisor`, `runner`, `bridge`, `liveness` (the renamed pane center), `ship`, `audit`,
`triage`, `scout`, `build`, `tdd`, `gate.contract`, `gate.eval`, `gate.repo`, `inbox`, `config` (the routing-config loader — registry, env and policy-stage resolution; breakdown unit 08),
`loop`, `watchdog`, `observer` (the phase observer's own faults — the manual subcommand's engine `internal/observerengine` and the live adapter's sink; breakdown unit 12), `dashboard`, `signalcenter` (self-reports), `ledger` (the file ledger's append observer, S4a), `outcome` (the phase-outcome recorder, breakdown unit 01), `failurediag` (the failure-diag sidecar writer and the delivery-failure classifier, breakdown unit 02), `carryover` (the carryover-todo lifecycle, breakdown unit 03), `failurelearning` (the failure-learning engine — the failed-approach recorder, the deterministic floor, the recurrence closure; breakdown unit 03b); `ship` also carries the landing's warnings (the ff-merge, the push with its inline repair, the ship-binding witness; breakdown unit 07 — `internal/phases/ship/landing`, `ship.warning`). A module is added by
`orchestrator`, `advisor`, `runner`, `bridge`, `liveness` (the renamed pane center), `ship`, `audit`
(the defect-ledger gate, breakdown unit 09 — the tag is shared by the audit package's later units;
the `AUDIT_LEDGER_` family prefix and `origin=Ledger.<Method>` name the producer), `triage`, `scout`, `build`, `tdd`, `gate.contract`, `gate.eval`, `gate.repo`, `inbox`, `config`,
`loop`, `watchdog`, `observer`, `dashboard`, `signalcenter` (self-reports), `ledger` (the file ledger's append observer, S4a), `outcome` (the phase-outcome recorder, breakdown unit 01), `failurediag` (the failure-diag sidecar writer and the delivery-failure classifier, breakdown unit 02), `carryover` (the carryover-todo lifecycle, breakdown unit 03), `failurelearning` (the failure-learning engine — the failed-approach recorder, the deterministic floor, the recurrence closure; breakdown unit 03b). A module is added by
editing the closed set and its test; an unknown module is stamped `SIGNALCENTER_UNKNOWN_MODULE` and
raised to WARN — never dropped. *(review 21: `advisor` and `config` added to match §12.)*

### 5.2 Kind (closed, dotted `<subject>.<event>`)

| Kind | Meaning | Severity default | Terminal? |
|---|---|---|---|
| `phase.dispatched` | a phase attempt started | INFO | |
| `phase.outcome` | the C1 terminal disposition (verdict recorded) | INFO on PASS, WARN on WARN/FAIL | ✓ on FAIL |
| `phase.aborted` | the cycle aborted after a phase (review reject, guard, persistence) | WARN | ✓ |
| `gate.passed` | the contract gate let the phase advance: verified (`GATE_CONTRACT_VERIFIED`, fields name the artifact, its size, the owed files and effects — where it searched), salvaged, or advanced with a WARN it should not hide (would-block under a shadow stage, breaker demotion, fail-open) — S2b | INFO; WARN for would-block / demoted / fail-open | |
| `gate.rejected` / `gate.corrected` | the contract gate refused at enforce (the reason IS the correction directive; `fields.codes`, `blocks`) / the orchestrator's ladder re-dispatched a correction (`fields.correction`, `max`, `rung`, `cli`, `escalated`) — S2b, both roots | WARN / INFO | ✓ / |
| `ship.landed` / `ship.error` | a landing / a `shiperr` code | INFO / WARN (INCIDENT for the `integrity` class) | / ✓ |
| `ship.warning` | the landing could not reset the tracked binary before the ff-merge, declined the inline push-race repair, could not read HEAD after the push, or could not write ship-binding.json; the commit/push stands and the ship's own error (if any) is the `ship.error` that follows, whose `fields.step` names the landing step (unit 07) | WARN | |
| `system.failure` | ADR-0072 signal (halt-class) | INCIDENT | ✓ |
| `quota.paused` | all families exhausted, cycle paused | WARN | ✓ |
| `bridge.warning` / `bridge.tripwire` | engine telemetry warnings, tripwires; the launch-exit classification (`BRIDGE_EXIT_*`, one per non-zero `Engine.Launch` exit, origin `Engine.Launch`, `fields.step=classify`) and the four Launch step failures (`BRIDGE_BOOT_STRIKE_CLEAR_FAILED` / `_RECORD_FAILED`, `BRIDGE_LAUNCH_ERROR_PERSIST_FAILED`, `BRIDGE_RESULT_READ_FAILED`; origin `Engine.<step>`) — breakdown unit 10 | WARN | |
| `pane.liveness` | liveness edge from the LivenessCenter | INFO; WARN with a `LIVENESS_PANE_*` code — the registry (rendered in `signal-codes.md`) is the one list of which states warn | |
| `ledger.appended` | a ledger entry was appended (the file ledger's append observer; `fields.entry_seq` names the line) | INFO | |
| `outcome.warning` | the phase-outcome recorder could not persist a record (sidecar or timing log skipped or failed); the in-memory record stands (unit 01) | WARN | |
| `failurediag.warning` | the failure-diag writer could not land `<phase>-failure-diag.json` (temp write or rename); the phase abort proceeds unchanged and the diagnosis is lost from disk (unit 02) | WARN | |
| `carryover.warning` | the carryover lifecycle could not read or decode a cycle-workspace document (`carryover-todos.json`, `defect-ledger.json`) or could not persist the failure-learning arrays to state.json; the closeout or the caller proceeds (unit 03) | WARN | |
| `failurelearning.warning` | the failure-learning engine could not load policy, write the deterministic floor artifacts or update the recurrence ledger, or truncated a self-reported defect list; the FailedRecord and P0 todo stand, the cycle proceeds (unit 03b) | non-terminal |
| `config.warning` | the routing-config loader took a fail-safe default: a dial outside its vocabulary (registry, env or policy.json), a weak or misordered spine, an inert enable, or a phase registry that exists but could not be read or parsed (the loader degrades to the compiled baseline — which omits triage); the cycle proceeds on the resolved value (unit 08) | WARN | non-terminal |
| `observer.warning` | the phase observer could not append an event, tail the stdout log, append the nudge, write the report, open the live events sink or reclaim its watcher — or it sent a stall SIGTERM (INCIDENT `OBSERVER_STALL_KILL_SENT`); the phase proceeds unobserved or as decided (unit 12) | WARN / INCIDENT for the kill | |
| `audit.warning` | the defect-ledger gate recorded a rejection it could not persist or an overflow, could not arm/read/write-back a continuation's lineage, found inherited defects unaccounted, or (INFO) could not tell the auditor its inherited ids; the diagnostics stay the graded wire, the verdict is decided by the audit phase (unit 09) | WARN; INFO for `AUDIT_LEDGER_PROMPT_DEGRADED` | |
| `inbox.warning` | the inbox mover could not claim, promote, park, release or recover an item, or an item-record rewrite / the continuation manifest read failed; the item stays where the lifecycle guarantees (never lost), the transition's ledger line says what moved, the caller's contract (sentinel / NoOp) is unchanged; the cycle proceeds (unit 06) | WARN | |
| `runner.warning` | the phase runner's verdict engine reconciled a bridge infra teardown to the agent's own deliverable (via Verify or the ACS floor), failed a teardown with forensics, degraded an optional phase, found a contracted deliverable unverified after the settle window (and downgraded a clean-ship verdict), or could not write the clean-stdout companion; the PhaseResponse it returns is authoritative, the orchestrator disposes (unit 11) | WARN | |
| `advisor.warning` | the phase advisor could not dispatch or parse a routing decision (the caller degrades to the static spine or keeps the initial plan), dropped a minted phase that assumed a control-plane identity, could not read `router.json`, could not run git for the pre-plan recon, or could not persist a capture artifact; the cycle proceeds (unit 04) | WARN | |
| `audit.warning` | a CI-parity gate could not run (fail-open), skipped an env-exclusive scope, absorbed a contention flake, hit its retake deadline without a verdict, lost its log or lock, deferred a graduation obligation, or FAILED the audit on offenders; the diagnostics list and the verdict are unchanged, the cycle routes to retro on FAIL (unit 14) | WARN | |
| `cycle.sealed` | final verdict decided (after `finalizeOutcome`) | INFO (FAIL → WARN) | ✓ on FAIL |
| `loop.wave` / `loop.halt` / `loop.escalation` | batch-level events (today's `dispatchevents`); `loop.wave` carries `fields.wave` on every event — INFO for the coordinator's summary, WARN with a code for the wave engine's conditions (`LOOP_MIN_WIDTH_REPAIR`, `LOOP_WAVE_DISPATCH_FAILED`, `LOOP_WAVE_EMPTY_PLAN`, `LOOP_WAVE_ALL_LANES_STALE`; unit 13); `loop.halt` also carries the chain's two non-zero-exit stops (`LOOP_CHAIN_INBOX_UNREADABLE`, `LOOP_CHAIN_BATCH_ERROR`) | INFO or WARN / INCIDENT / WARN | / ✓ / |
| `loop.warning` | a batch- or chain-boundary degradation with no wave index (unit 13): the boundary binary refresh degraded to the current binary (`LOOP_BOUNDARY_REFRESH_SKIPPED` with `fields.step`, `LOOP_BOUNDARY_REFRESH_AUDIT_FAILED`), an invalid inbox item (`LOOP_CHAIN_INBOX_ITEM_INVALID`), a quota defer (`LOOP_CHAIN_QUOTA_DEFER`). The written rule: wave-indexed → `loop.wave`; other non-terminal → `loop.warning`; a chain stop with a non-zero exit → `loop.halt` | WARN | |
| `signalcenter.listener_panicked` / `signalcenter.sink_dropped` | self-reports: a panicking listener was dropped / the NDJSON sink could not write (count in `fields.dropped`) | INCIDENT / WARN | |

"Terminal" marks the kinds that end or change a cycle's course — the triage entry point (§9).

Vocabulary drift (unknown module or kind, unregistered or missing code, bad origin, missing reason)
is **not** a separate event: `Normalize` stamps the drift code onto the offending event itself (§5.4),
so the drift is one line, next to the producer's own reason. `signalcenter.registry_drift` is the
kind that line carries only when the producer's own kind is unknown (an unknown kind cannot be
rendered as itself; `fields.raw_kind` keeps it), and `signalcenter` is likewise the module of a line
whose module was unknown (`fields.raw_module`). Registry conflicts are recorded at package init —
before any Center exists — and asserted empty by a named test; they are a review defect, not a
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
  (or a table in the module). Duplicate registration with an identical doc is a no-op; a conflicting
  registration (a different doc, a malformed code, or a code whose prefix belongs to another module)
  is recorded in `RegistryConflicts()` — never a panic and never an event, because registration runs
  at package init, before any Center exists: a duplicate code is a review defect, not an availability
  event *(review 9)*. `TestRegistryConflicts_RealRegistryIsClean` asserts the real registry is clean;
  `IsRegistered(code)` answers which module owns a code; `RegisteredCodes()` is exported so
  `docs/architecture/signal-codes.md` is generated from it (S2).
- Existing vocabularies map by projection, not by copy: `shiperr.ShipErrorCode` (42) → `SHIP_<code>`
  (one function `shiperr.SignalCode(code)`); bridge exit codes → `BRIDGE_EXIT_<class>` as `launchoutcome.Classify(code, …).Signal` (a column of the one Outcome) — spelled by class name (`BRIDGE_EXIT_ARTIFACT_TIMEOUT`, `_UNKNOWN_PROMPT`, `_RESPOND_LOOP_GUARD`, …; `fields.exit_code` carries the number; landed as breakdown unit 10);
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

1. **Total order, provable — one queue, one drainer.** `Emit` normalizes the event and stamps it
   (`Seq`, `PID`, `TS`) under the Center's mutex, then appends it to a queue. The first emitter that
   finds no drain in progress becomes the drainer: it pops events in `Seq` order and delivers each,
   in registration order, to a snapshot of the listener list taken under the mutex — **the mutex is
   never held while a listener runs**. Emitters that arrive while a drain is in progress (concurrent
   evaluate-batch goroutines, or a listener emitting re-entrantly) enqueue and return; the running
   drainer delivers their events before it returns, so the file order equals `Seq` order and every
   listener sees the same sequence *(review 7)*. The order mutant to kill is "listeners observe
   different orders", not "registration order reversed". **Consequence for producers on exit
   paths** (S1 review): an emitter that arrives while another goroutine drains returns before its
   event is delivered; the sole S1 producer runs on the draining goroutine, so delivery is
   synchronous in practice. `Flush` (S2a) blocks until the queue is empty and no drain is running;
   both production roots defer it after wiring the orchestrator
   (`TestSignalCenterFlush_IsWiredAtBothRoots`), so a producer on an `os.Exit` path (`loop.halt`,
   a watchdog goroutine — S3/S4) can never leave an unexplained gap in the file's `seq` at exit.
   Never call `Flush` from inside a listener: the drain running the listener is the one it waits for.
2. **Never drops.** Validation failures rewrite the event with a `SIGNALCENTER_*` code, keep the
   originals in `fields`, and raise severity to at least WARN (§5.4).
3. **Panic isolation without self-deadlock** *(review 2)*. A panicking listener is recovered and
   unsubscribed; the Center then reports `signalcenter.listener_panicked` (INCIDENT,
   `fields.listener` = the registration id) by enqueueing the report like any other event — the drain
   in progress delivers it to the remaining listeners after the current event. There is no internal
   dispatch path: one queue and one way in, so a report can neither deadlock nor reorder.
4. **Re-entrancy is safe, not forbidden** *(review 2)*. A listener that calls the public `Emit`
   (nested emit) does not deadlock: it enqueues and returns (semantics 1) and the drainer delivers
   the nested event after the outer one. `TestEmit_ReentrantListenerIsQueuedNotDeadlocked` asserts
   completion within a timeout and that the nested event follows the outer one by `Seq`.
5. **Null Object for tests, never for production.** `var c *Center = nil`: `Emit` is a no-op,
   `Subscribe` returns a no-op unsubscribe, `Recent` returns nil. Producers never check for nil.
   Production roots must wire a real Center (§7) *(review 3)*.
6. **Unsubscribe is idempotent** and safe during an emit (the snapshot semantics above).
7. **Bounded memory.** `Recent` keeps the last N (default 256) events across cycles; the durable
   record is the sink; the orchestrator's `Summary` keeps only the current cycle — it resets itself
   when a newer cycle's first event arrives (§8).
8. **The NDJSON sink holds one open handle per cycle** (opened `O_APPEND|O_CREATE` on the first event
   for that cycle, closed when the resolver returns a different path or on process exit) and writes
   one line per `Write` (≤ 4 KiB, §4). A resolver returning `""` (process-level events before a cycle
   is allocated), an open failure or a short write counts as a drop; the count is reported once, as
   `signalcenter.sink_dropped` WARN (`fields.dropped=<n>`), on the next successful write — through the
   Center's own `Emit`, which is why the sink is a Center method. A resumed cycle appends to the same
   file from a new process; `pid`+`seq` reconstruct both orders
   (`TestNDJSONSink_TwoProcessesAppendToTheSameCycleFile`). (`dispatchevents` opens per line today;
   that is the pattern not to copy at bridge-telemetry rates.)
9. **The stderr sink is the only module-tagged logger.** Producers do not print. At the root it is
   wrapped in `Filter(…, WARN)`: INFO goes to the durable file only, as the severity contract says
   ("log only, do not surface") *(review 6)*. That wrapping is `ConsoleSink(w)` — the ONE home of
   the console threshold, consumed by the orchestrator roots and by the `evolve phase-observer`
   subprocess (ADR-0103 unit 12); a module-wide scan (`TestConsoleSinkThresholdHasOneHome`) keeps
   every other production source from re-spelling it.

## 7. Wiring

Composition root `go/cmd/evolve/cmd_cycle.go` (`wireOrchestratorDeps`, which the loop shares via
`wireOrchestratorDepsFn`): the Center is constructed **first**, before the bridge engine, because
`bridge.NewEngine` normalizes `Deps` at construction and a field set afterwards is not observed
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

Batch-level signals — the loop's own halts and wave summaries, a bridge warning raised before any
cycle — carry no cycle and are durable in `<evolveDir>/signals.ndjson` (S4a). Before S4a the root
returned no path for cycle 0, which was harmless while no regular producer was cycle-less; the
loop's wave summary would have made the durable sink report a drop after every wave (caught by the
stub root building the production topology, §15.4).

**Non-optional and loud** *(review 3)*: `wireOrchestratorDeps` always constructs a Center, and so does the
`--simulate` root (both build `newRootSignalCenter`, ONE topology — unit 01's architecture review
found the Center-less simulate root had silenced the recorder's warnings); the only nil-Center
construction site is the test-only `internal/routingtest` engine, and a repo test pins that list. Wiring proofs (the `DeclaredDeliverablesGateWired` precedent): `Orchestrator.SignalCenterWired()`
asserted by a `cmd/evolve` composition test at both roots (cycle and loop); a `core` test that a
constructed orchestrator with a Center is subscribed (its `observeSignal` receives what its own
chokepoint emits); `engine.SignalsWired()` asserted on the bridge side (S3); an import-graph test that
`signalcenter` imports nothing under `internal/` but `log` *(review 4)*.

## 8. The orchestrator as listener

```go
// core
func WithSignalCenter(c *signalcenter.Center) Option   // NewOrchestrator subscribes o.observeSignal when c != nil
func (o *Orchestrator) SignalCenterWired() bool
func (o *Orchestrator) SignalSummary() signalcenter.Summary // a snapshot of the CURRENT cycle: counts by severity and kind, last INCIDENT
```

- `observeSignal` is O(1): it updates the current cycle's summary under the orchestrator's summary
  mutex. **Lock order** *(review 17)*: emit mutex → summary mutex, never the reverse; no orchestrator
  path holds the summary mutex across an `Emit`. This is the first production mutex in
  `internal/core`; a `-race` test with concurrent evaluate-batch emitters pins it.
- **Lifetime** *(review 16)*: the summary follows the current cycle by itself — the first event of a
  newer cycle resets it, events of older cycles are ignored, process-level events (cycle 0) count
  toward the current cycle — so no orchestrator path has to remember to reset it (`Summary.Observe`
  is tested in isolation); `Recent` on the Center is the cross-cycle window.
- **Observe, never decide** *(review 10)*: `observeSignal` never emits (no feedback loops) and never
  decides — deciding stays with the existing floors; ADR-0072's `SystemFailureSignal` is produced
  *into* the Center in S2 and its halt path is unchanged. The summary is reporting evidence.
- The orchestrator is the contact point: `cmd_loop` reads `Orchestrator.SignalSummary()` (S4) to
  print the counts in the batch report without re-reading files — it does not gate on them. The
  summary is deliberately **not** copied onto `CycleResult` (S1 review): one owner, one snapshot
  method, no second copy to keep in step.
- The orchestrator's summary is what "closely monitor the loop execution" means in code: at any
  point, `SignalSummary()` says how many WARN/INCIDENT signals the current cycle has raised and the
  last INCIDENT's code and reason.

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

The last line shows drift stamped in place: the producer's module, kind, origin and reason stay, the
unregistered code is replaced by the drift code, and `fields.drift` / `fields.raw_code` say what was
wrong — one line, where the operator is already looking.

**Triage in a few review steps, kind-first** *(review 8)*: (1) `grep -E '"kind":"(system\.failure|phase\.aborted|gate\.rejected|ship\.error|quota\.paused)"' signals.ndjson`
— the terminal kinds (§5.2), or `grep INCIDENT` when something halted; (2) the `code` names the rule and
`module`/`origin` name the function; (3) `fields.path` names the artifact. A WARN budget keeps this
honest: a healthy PASS cycle emits ≤ N WARN signals (N = 0 on the unit-level green cycle —
`TestRunCycle_EveryDispatchedPhaseEmitsOnePhaseOutcome_GreenCycleStaysUnderTheWarnBudget`; the live
budget is measured from the first green runtime cycle after S1 deploys and pinned in S2); if a green
cycle cannot stay under budget, the ladder is decoration.
Hand-written `[x]` prose disappears one module at a time (§10 S5).

## 10. Migration slices

| Slice | Content | Acceptance (measured by tests, not prose) | Depends on |
|---|---|---|---|
| **S0** | this design, ADR-0101, the inventory | docs-only PR; links resolve; architect review folded in | — |
| **S1** | `internal/signalcenter` (schema, Center, registry, sinks, filter); `core.WithSignalCenter` + listener + `Orchestrator.SignalSummary()`; `orchDeps.Signals`; the C1 chokepoint emits `phase.outcome`/`phase.aborted` on both roots (its PR #577 hand-written line replaced by the sink line); `.apicover-enforce` entry; CI line-coverage gate | package 100 % API coverage (apicover) + 100 % line coverage (CI/make gate, §11), `-race`; composed RunCycle test: every dispatched phase (incl. the ADR-0044 abort-path table) yields exactly one `phase.outcome` observed by the orchestrator listener and one `signals.ndjson` line with monotonic `seq`; WARN-budget test; re-entrancy test; mutants: emit removed, listener not subscribed, sink not attached, validation bypassed, recover removed, same-order-for-all-listeners — each killed by name | PR #577 (lands after the #575/#576/#577 merge order) |
| **S2a** | producers that need only S1: `system.failure` + `cycle.sealed` at `cycleRun.completeCycle` (the closeout both roots share; the hand-written SYSTEM-FAILURE HALT / LANDING LOST lines deleted); `ship.error` at `Orchestrator.recordShipError` with `shiperr.SignalCode` (every ship code registered under module `ship` with a doc each — a source-parsed test proves completeness; `ShipErrorClass.SignalSeverity` is the class → severity rule's one home); `quota.paused` at `cycleRun.pauseForQuota` (the seam both roots reach; its hand-written WARN line deleted); an abnormal exit seals the cycle FAIL from `cycleRun.abnormalEpilogue`; `Center.Flush` deferred at both roots; `evolve signals codes generate\|check` projecting the registry into `docs/architecture/signal-codes.md` | composed `RunCycle` test: `cycle.sealed` is the LAST orchestrator event; each producer has a direct proof (INCIDENT on halt / integrity, WARN otherwise); `signal-codes.md` currency is a `cmd/evolve` test (CI); mutants: each emit removed, INCIDENT not raised, prefix wrong, Flush broadcast removed, drift check disabled | S1 |
| **S2b** (contract-gate producers landed, see §15.5; the rest stays in S2c) | contract gate → `gate.passed/rejected` + the ladder's `gate.corrected` on both roots (the `GATE_CONTRACT_*` codes; PR #575 landed); `fields.shipped` on `cycle.sealed` (needs PR #576's `CycleState.Shipped`); `failurelog.Classification` folded into `failureadapter`'s; the live WARN budget pinned from the first green runtime cycle | each producer has a composed proof; the classification registry test fails if the two vocabularies diverge again | S2a, PR #575, PR #576 |
| **S3** (landed, see §15.3) | bridge: `Deps.Signals` at construction + `engine.SignalsWired()`; the production Adapter takes the Center as a constructor argument (`adapters/bridge.NewDefault(projectRoot, signals)`; every other call site passes an explicit `nil`, pinned) and threads it into every engine; engine telemetry warnings → `bridge.warning` (`BRIDGE_TOKEN_RESOLVER_MISSING/_FAILED`, `BRIDGE_TOKEN_USAGE_WARNING`, `BRIDGE_CONTEXT_FILL_HIGH`, `BRIDGE_TELEMETRY_APPEND_FAILED`), the tripwire → `bridge.tripwire` (`BRIDGE_TELEMETRY_TRIPWIRE`); **commit 1:** the pure rename `panestream.SignalCenter` → `LivenessCenter` (ADR-0068/0070 amended); **commit 2:** `pane.liveness` from a `LivenessHandler` the tmux driver registers per dispatch (module `liveness`, `LIVENESS_PANE_STAGNANT/_HUNG/_EXHAUSTED`); the hand-written `[engine] WARN` / `[engine] TRIPWIRE` lines removed | wiring proofs at the engine, the Adapter (deps + real factory) and the root; every producer a direct proof; the telemetry suites assert what the root's WARN-filtered sink renders; mutants: signals not threaded, wired-always-true, handler not registered, hung not WARN, tripwire/warn/engine-warning not emitted, root passes nil, state name lost, format characters surviving | S1 |
| **S4a** (landed, see §15.4) | the loop module's producers at their seams — ONE `loop.halt` INCIDENT per batch halt, its code the caller's rule (`haltOnSystemFailure` is the one chokepoint: `LOOP_SYSTEM_FAILURE_HALT` for a halt the cycle signalled, `LOOP_PIPELINE_BLOCKER_HALT` for the blocker breaker), a fleet lane's halt code → `LOOP_FLEET_LANE_HALT`, a wave-boundary halt → `LOOP_HALT`; `loop.wave` (INFO for the wave summary the report also prints; WARN `LOOP_MIN_WIDTH_REPAIR`), `loop.escalation` WARN `LOOP_ESCALATION_BOUNDARY`; the hand-written `[loop] … HALT` and escalation lines deleted; the file ledger's append observer (`ledger.New(evolveDir, ledger.WithSignals(signals))`, module `ledger`: `ledger.appended` INFO per entry through `Append` — the orchestrator's records, the bridge's stop_review, the inbox lifecycle lines, the seal anchor — WARN `LEDGER_APPEND_FAILED`) at the root, and the root's ledger threaded into the failed-cycle inbox walk; `cmd_loop` **reports** the driven runner's per-cycle `SignalSummary` in the batch report (`[loop] cycle N signals: …`, a report line, never a gate) through the `loopCycleRunner` seam; ONE sink topology (`newRootSignalCenter(root, evolveDir, console)`) for production and the stub root, with a durable batch-level file for cycle-less signals | every producer a direct proof; the window/escalation/min-width suites assert the sink-rendered line through the stub root (the production topology); a go/ast guard inventories every ledger line writer; mutants: each emit removed, INCIDENT demoted, a second breaker INCIDENT, ledger not observed, lifecycle not observed, ledger failure not WARN, root ledger unobserved, inbox walk on a self-built ledger, report silent / read off the seam, console writer ignored, cycle-less signals dropped | S2a, S3 |
| **S4b** | `dispatchevents` writers and the `observer` adapter emit through the Center (the adapter's Center accessor and its two fault codes `OBSERVER_EVENTS_SINK_OPEN_FAILED` / `OBSERVER_WATCHER_LEAKED` landed in breakdown unit 12; the detection stream — `stall_no_output`, `stall_probe_active`, started/stopped — still enters here) (their files stay as sink outputs until readers migrate; `subagent.AppendAbnormalEvent` folded); the dashboard SSE subscribes | `abnormal-events.jsonl` **field-equal modulo timestamp precision** before/after, asserted by a decoding golden *(review 11)*; dashboard shows a signal within one SSE tick; no breaker gates on a signal (a shadow comparison test may log disagreement) | S4a |
| **S5** | per-module log migration riding each decomposition slice (§12): prefix-literal occurrences per module (any writer) `[orchestrator]` 287 → 0, `[loop]` 132 → 0, `[ship]` 130 → 0, … | one repo-wide grep test with a shrinking allowlist (a migrated module cannot be forgotten); the inventory's prefix table regenerated | S1 |
| **S4b** | `dispatchevents` writers and the `observer` adapter emit through the Center (their files stay as sink outputs until readers migrate; `subagent.AppendAbnormalEvent` folded); the dashboard SSE subscribes | `abnormal-events.jsonl` **field-equal modulo timestamp precision** before/after, asserted by a decoding golden *(review 11)*; dashboard shows a signal within one SSE tick; no breaker gates on a signal (a shadow comparison test may log disagreement) | S4a |
| **S5** | per-module log migration riding each decomposition slice (§12): prefix-literal occurrences per module (any writer) `[orchestrator]` 287 → 0, `[loop]` 132 → 0 (unit 13 removed four `[loop]` literals and sixteen `[chain]` lines from the two loop schedulers; fleet's two `[loop]` literals are its F2), `[ship]` 130 → 0, … | one repo-wide grep test with a shrinking allowlist (a migrated module cannot be forgotten); the inventory's prefix table regenerated | S1 |

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
ADR-0072's coherence floor and need their own campaign note — except the defect ledger, pulled
forward as unit 09 (its schema was spelled in three homes and the `audit` tag had no producer).

| Order | Unit to extract (from) | Size today | Module tag | Notes |
|---|---|---|---|---|
| 1 | **landed 2026-09-13 as unit 01** ([decomposition/01-outcome-recorder.md](decomposition/01-outcome-recorder.md)): the C1 recorder → `internal/core/outcome`; **unit 02 landed 2026-09-13** ([decomposition/02-failure-diagnostics.md](decomposition/02-failure-diagnostics.md)): the failure-diag sidecar writer and the delivery-failure classifier → `internal/core/failurediag`; **unit 03 landed 2026-09-13** ([decomposition/03-carryover-lifecycle.md](decomposition/03-carryover-lifecycle.md)): the carryover-todo lifecycle → `internal/core/carryover`, three sibling files deleted; **unit 03b landed 2026-09-13** ([decomposition/03b-failure-learning-engine.md](decomposition/03b-failure-learning-engine.md)): the failure-learning engine → `internal/core/failurelearning`, `recordFailureLearning` split into a pure gate + four steps (`failure_learning.go` 823 → 566 → 318); **unit 11 built 2026-09-14, out of §12 order** ([decomposition/11-phaserunner.md](decomposition/11-phaserunner.md)): the phase runner's verdict engine → `internal/phases/runner/verdict`, the first `runner` producer (`runner.warning`, five codes; `runner.go` 719 → 500, `reconciliation.go` + `classification.go` deleted) — jumped ahead of the advisor and orchestrator rows on the cycles-1634/1636 F4 triage class | 1070 | `orchestrator` → `outcome` | already the C1 chokepoint; S1 touches it — extract after S1 lands |
| 2 | **landed 2026-09-14 as unit 04** ([decomposition/04-advisor.md](decomposition/04-advisor.md)): phase advisor → `internal/core/advisor` (`phase_advisor.go` 1133 → 373, the seam; the brain incl. its dispatch shell, I/O behind injected seams — the "pure decision logic" note was false: four I/O sites) | 1144 | `advisor` | the two rune-cap text rules hoisted into `internal/textcap`; six `ADVISOR_*` codes on `advisor.warning` |
| 3 | `orchestrator.go` (1158): composition (`New`, options) vs `RunCycle` engine | 1158 | `orchestrator` | Facade over the extracted units |
| 4 | `cyclerun.go` (904): finalize/closeout → `cycleclose` | 904 | `orchestrator` | `finalizeCycle` + `finalizeOutcome` + dossier |
| 5 | **landed 2026-09-14 as unit 06** ([decomposition/06-inboxmover.md](decomposition/06-inboxmover.md)): the inbox lifecycle mover → `internal/inboxmover/lifecycle` as ONE leaf of eight files (the "three units" became three files of one package sharing the ledger line and the item primitives); `inboxmover.go` 993 → 372 (the seam: Options, the resolved defaults, the one construction, the facades) | 1006 | `inbox` | claim/release/promote as three units |
| 6 | **unit 07 landed 2026-09-14** ([decomposition/07-shipgitops.md](decomposition/07-shipgitops.md)): the landing (the ff-merge, the push with its inline repair and the post-push head read, the shared git probes, the binding writer) → `internal/phases/ship/landing` behind the seam `gitops_landing.go`; `gitops.go` 989 → 935, `repair.go` 474 → 402, `worktree_ship.go` 205 → 184; the staging guard and the run-scope policy remain (07b / 07c) | 989 | `ship` | landing vs binding writer vs staging guard |
| 7 | **landed 2026-09-14 as unit 08** ([decomposition/08-config.md](decomposition/08-config.md)): `config/config.go` (962 → 229) decomposed in place — `Loader` with an injected reader and Center, `config.warning`, six `CONFIG_*` codes | 962 | `config` | typed policy structs (`PolicyStages`); the "retire env flags" by-product is OUT of scope (the nine keys are prose-contracted in the protected flagregistry table — its own slice) |
| 8 | `bridge/engine.go` (791) + `autorespond.go` (787) — **unit 10 landed 2026-09-14** ([decomposition/10-bridgeengine.md](decomposition/10-bridgeengine.md)): the launch-outcome classifier → `internal/bridge/launchoutcome` (the exit table, the cause miners), `Launch` split in place, the request gauntlet kept in the host as `bridge.ValidateRequest`; `autorespond.go` and the remaining `[bridge]` lines are unit 10b | 1578 | `bridge` | rides S3 |
| 9 | **landed 2026-09-14 as unit 09** ([decomposition/09-defectledger.md](decomposition/09-defectledger.md)): `phases/audit/defect_ledger.go` (882) → `internal/core/defectledger` (882 → 392 — the seam and the citation resolver stay) | 882 | `audit` | pulled forward from the later wave: unit 03 F13's three schema homes collapse onto one leaf; 13 `AUDIT_LEDGER_*` codes on `audit.warning` |
| 12 | **landed 2026-09-14 as unit 12** ([decomposition/12-phaseobserver.md](decomposition/12-phaseobserver.md)): the phase observer — `internal/phaseobserver/phaseobserver.go` (601) → the clock-stepped engine `internal/observerengine` (tail/decoder, stall rules, incident responder, envelope/report sinks) with the host as a Strangler seam; the live `adapters/observer.CoreAdapter` wired to the Center for its own two faults; the layout projected from `observerengine.PathsFor` | 601 | `observer` | eight `OBSERVER_*` codes; the adapter's detection stream stays S4b's |

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

1. RED: `signalcenter` tests for §6 semantics 1–9, the §4 rules (required fields, `Origin` regex,
   `Fields` bounds, line cap), the JSON golden line, the stderr golden line, registry conflicts, the
   import graph.
2. GREEN: `event.go`, `center.go`, `registry.go`, `sinks.go` (< 800 lines each, functions < 50).
3. RED: `core` — `WithSignalCenter`, `SignalCenterWired`, `SignalSummary`, `observeSignal` (lock
   order; the summary follows the cycle by itself); the C1 chokepoint emits on every terminal path
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

### 15.1 Landed — S1 as implemented (2026-09-13)

Every item above is done; these are the deltas from the plan, each chosen during implementation
and folded back into §4–§9 so this document describes the code that exists:

| Planned | Landed | Why |
|---|---|---|
| Emit-mutex fan-out + an internal `dispatchLocked` for self-reports | one queue, one drainer, no lock held during delivery; self-reports enqueue like any event (§6.1, §6.3) | one way in means one order and no second dispatch path to get wrong; the mutant "listeners observe different orders" is killed by `TestEmit_AllListenersObserveTheSameOrderUnderConcurrency` |
| package-level `NDJSONSink` | `(*Center).NDJSONSink` | the sink reports its own drops through the Center it serves |
| separate `signalcenter.registry_drift` / `registry_conflict` events | drift stamped onto the offending event (`registry_drift` is only the replacement kind for an unknown kind); conflicts recorded only, no `registry_conflict` kind (§5.2, §5.4) | drift is one line next to the producer's reason; conflicts happen at package init, before a Center exists |
| summary reset in `planCycle`; `CycleResult.Signals` | `signalcenter.Summary` follows the cycle by itself; `Orchestrator.SignalSummary()` is the only reader (§8) | no path can forget the reset; one owner, one snapshot, no copy on the result |
| `log.DiagnosticField` for field values | `log.SanitizeField` for values and `Reason`; `FormatLine` exported (§4, §9) | a signal line is never quoted twice; sinks share one format |
| WARN budget N measured live | N = 0 on the unit green cycle; live N pinned in S2 | S1 has no runtime cycle yet |

Folded from the S1 architecture review (Approve; four MEDIUM): the durable sink releases its own
lock before reporting drops (a `Listener` is a plain func and may run outside a drain —
`TestNDJSONSink_DropReportIsEmittedOutsideTheSinkLock` reproduced the self-deadlock first); the
`listener_panicked` INCIDENT names the listener **function** (`fields.listener`, e.g.
`core.(*Orchestrator).observeSignal-fm`) beside its id; `verdictReason(verdict, diags)` is the one
rendering of "verdict=<V>[: reasons]" for the floor error, the seal and the FAIL **and** WARN
events; and the registry-clean assertion lives in `cmd/evolve`
(`TestSignalCenterRegistry_EveryLinkedModuleRegistersCleanly`), the binary that links every
producer module — the leaf package's own test cannot see them.

Folded from the S1 Go review (Approve; one MEDIUM): the ≤ 4 KiB line cap only looked at `Fields`, so
a regex-valid but over-long `Origin` or a hostile `Reason` could break the atomic-append promise —
identifiers are now bounded and `Reason` is cut to fit (§4).

Proof carried by the S1 PR: `internal/signalcenter` 100 % line coverage (`make cover-strict`, in CI)
and 98/98 exports API-covered; `core` 343/343; `-race` ×3; nineteen mutants killed by name (emit
removed, subscribe removed, NDJSON sink not attached, orchestrator not wired, WARN branch
unreachable, FAIL kept INFO, cycle not stamped, abort not distinguished, summary lock removed,
zero-summary init removed, stderr filter dropped, stderr sink not attached, sink truncates instead
of appending, leaf imports core, core registers a conflicting code, listener name blank, sink lock
held across Emit, reason cut removed, identifier bound removed); named wiring tests `TestWireOrchestratorDeps_SignalCenterWired`,
`TestWireOrchestratorDeps_SignalCenterConsoleSinkIsFilteredAtWarn`, `TestNilSignalCenterRootsArePinned`,
`TestImportGraph_LeafPackageImportsOnlyInternalLog`, `TestNDJSONSink_TwoProcessesAppendToTheSameCycleFile`.

### 15.4 Landed — S4a (2026-09-13)

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `loop.halt` | `haltOnSystemFailure` — the ONE shared halt+escalate action the sequential path, the cycle-run root and the blocker breaker call; it emits ONE INCIDENT whose code is the caller's `loopHaltRule` | INCIDENT `LOOP_SYSTEM_FAILURE_HALT` (a halt the cycle signalled) or `LOOP_PIPELINE_BLOCKER_HALT` (the breaker's rule, + `rule`, `fingerprint`) | `category`, `level`, `next` (the dossier's own `next_action` — its one home), `escalation`, `inbox_item` (what the halt wrote) |
| `loop.halt` | `loopBatchCoordinator.fleetHaltDecision` — a fleet lane exited with the halt code; the reason points at the lane's own `LOOP_SYSTEM_FAILURE_HALT` instead of restating its dossier | INCIDENT `LOOP_FLEET_LANE_HALT` | `kind`, `iteration`, `rc` |
| `loop.halt` | `loopBatchCoordinator.prepareIteration` (origin corrected by unit 13; was the stale `runWaveIteration`) — the wave-boundary sync refused (plane diverged) | INCIDENT `LOOP_HALT` | `stop_reason` |
| `loop.wave` | `loopBatchCoordinator.dispatchFleetIteration` (summary; the `[loop] wave N: x/y lanes ok` report line stays — INFO never prints) / the wave engine's `Engine.RepairMinWidth`, `Engine.Dispatch`, `gatedLauncher.Run` (unit 13: `LOOP_MIN_WIDTH_REPAIR` re-homed, `LOOP_WAVE_DISPATCH_FAILED{path, step}`, `LOOP_WAVE_EMPTY_PLAN{cause}`, `LOOP_WAVE_ALL_LANES_STALE`; the producer is `loopwave.EmitWave`, `emitLoopWave` its projection) | INFO / WARN | `wave`, `lanes_ok`, `lanes` / `desired`, `realized`, `path`, `step`, `error`, `cause`, `planned`, `skipped` |
| `loop.warning` / `loop.halt` | the chain engine (unit 13): `Refresher.Refresh` (`LOOP_BOUNDARY_REFRESH_SKIPPED{step}`, `LOOP_BOUNDARY_REFRESH_AUDIT_FAILED`), `Driver.Run` (`LOOP_CHAIN_INBOX_ITEM_INVALID`, `LOOP_CHAIN_QUOTA_DEFER` on `loop.warning`; `LOOP_CHAIN_INBOX_UNREADABLE`, `LOOP_CHAIN_BATCH_ERROR` on `loop.halt`); the chain root builds its own batch-level Center (`runLoopChain`), flushes it before every batch (a chained batch may re-exec at a wave boundary, flushing only its own Center) and the refresh flushes the Center it was handed before every re-exec | WARN / INCIDENT | `batch`, `step`, `commit`, `error`, `name`, `path`, `rc`, `stop_reason`, `exit`, `cycle`, `wake_at`, `source` |
| `loop.escalation` | `applyEscalationBoundary` | WARN `LOOP_ESCALATION_BOUNDARY` | `stage`, `bumped`, `filed`, `skipped`, `planned` |
| `ledger.appended` | `FileLedger.Append` — the file ledger's append observer, installed by the construction option `ledger.WithSignals(signals)` the root passes to `ledger.New`; every entry writer reaches this chokepoint (`AppendLifecycle`, the seal's segment anchor, the bridge's stop_review, the orchestrator's records); the file ledger still chains and locks | INFO; WARN `LEDGER_APPEND_FAILED` when the append errors (the error still returns) | `role`, `kind`, `exit_code`, `path`, `entry_seq` (the line the signal names) |

The batch report reads the driven runner's view: after each sequential cycle `cmd_loop` prints
`[loop] cycle N signals: T total, W WARN, I INCIDENT (last INCIDENT CODE — reason)` from
`SignalSummary()` on the `loopCycleRunner` seam (the real orchestrator in production, a scripted
runner in tests — one seam, no second identity for the collaborator) — a report line (the loop
reports, never gates; §8). Wiring: `newRootSignalCenter(root, evolveDir, console)` is the ONE sink
topology — production (`wireOrchestratorDeps`) and the loop tests' stub root call the same
constructor, so a test that asserts a rendered INCIDENT line proves production's topology; the
console sink renders into the command's stderr writer (production passes `os.Stderr`). Cycle-less
signals are durable in `<evolveDir>/signals.ndjson` (see §6 — the stub root's production topology
caught the drop report the wave summary would otherwise have raised after every wave). Deleted
1:1: the three-line `[loop] SYSTEM-FAILURE HALT` message, `[loop] PIPELINE-BLOCKER HALT`, the
fleet-lane halt line, `[loop] HALT: …`, and the escalation-boundary line; the min-width repair
line became the WARN's reason. `fields.next` on a halt IS the dossier's `next_action`
(`writePipelineEscalation` returns what it wrote; the prose is stated once); the fleet-lane halt
points at the lane's own INCIDENT. Under `--simulate` the root builds the same topology since unit 01
(ADR-0103), so a system-failure halt renders on the console there exactly as in a real cycle.

**The ledger, after the architecture review (HIGH-1).** The first cut was a Decorator embedding
`*FileLedger` and overriding `Append`. Go embedding is delegation without virtual dispatch: the
promoted `AppendLifecycle` called `FileLedger.Append`, never the override, so every inbox
lifecycle line — and the seal's segment anchor — was appended unobserved; and the inbox mover
builds its own `FileLedger` over the same file whenever no `Ledger` is supplied, which no
production caller did. Landed instead: an **append observer at the write chokepoint**, installed
by the construction option `ledger.WithSignals(signals)` (`ledger.New(evolveDir, opts...)`,
functional options) — `FileLedger.Append` is the ONE path every entry writer reaches, so
promotion cannot route around it; the failed-cycle inbox walk (`cycleoutcome.ApplyFailure`,
reached from the cycle-run root and both sequential loop paths through the one
`applyCycleFailureOutcome`) receives the root's ledger (`orchDeps.Ledger` is the `rootLedger`
interface: `core.Ledger` + the mover's `LedgerAppender`, one object, one identity); and a go/ast
guard (`TestFileLedger_EveryLineWriterReachesTheAppendChokepointOrIsInventoried`) pins every line
writer inside the package — through method and plain-helper calls alike — to the chokepoint or to
its inventory with a reason (`Rebaseline`, an operator repair root; `WriteCompositionVerdict`, a
composition record by a self-constructed ledger), and `TestUnobservedLedgerRootsArePinned`
(cmd/evolve, the twin of the pinned nil-Center roots) pins every `ledger.New(` outside it that
passes no `WithSignals`, with its count and reason: the `evolve cycle reset` seal, the
`evolve ledger` repair commands, the guard chain's read-only ledger, and the
inbox mover's fallback — which the ship phase's post-ship mover and the operator inbox commands
still reach. Both inventories are the code-homed list of ledger lines the Center does not see;
shrinking them is S4b's DI work. A cycle-signalled halt is two records by design — the
orchestrator's detection (`system.failure`, S2a) and the loop's halt action (`loop.halt`, carrying
the dossier fields).

Not in this slice: the wave-summary INFO path and the plane-diverged halt have no fleet fixture
(the wave dispatch needs subprocess lanes); their producers are covered by the direct producer
tests and the S4b/S5 fleet fixture is the follow-up.

### 15.3 Landed — S3 (2026-09-13)

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `bridge.warning` | `NewEngine` — a missing token resolver at construction (cycle 0) | WARN `BRIDGE_TOKEN_RESOLVER_MISSING` | — |
| `bridge.warning` | `attemptLogContext.warn` — the ONE telemetry-warning writer the engine had (resolver failed, usage caveat, context fill over the threshold, ledger append failed) | WARN `BRIDGE_TOKEN_RESOLVER_FAILED` / `BRIDGE_TOKEN_USAGE_WARNING` / `BRIDGE_CONTEXT_FILL_HIGH` / `BRIDGE_TELEMETRY_APPEND_FAILED`; the old line's detail is the reason | `call_id`, `cli`, `agent` (+ cycle/run/phase/attempt on the event) |
| `bridge.tripwire` | `attemptLogContext.tripwire` from `emitTokenWarnings` — a successful non-claude attempt past the threshold with no measurable usage | WARN `BRIDGE_TELEMETRY_TRIPWIRE` | `call_id`, `cli`, `agent`, `duration_ms` |
| `pane.liveness` | `paneLivenessHandler`, registered on the dispatch's `LivenessCenter` in `newReplWaitState` — one event per liveness EDGE (the center is edge-triggered) | INFO, or WARN with the `LIVENESS_PANE_*` code the registry lists (`signal-codes.md` is the one list of which states warn) | `session`, `state` (`LivenessState.String`, the vocabulary's one spelling — the timeout summary's snake_case word is a projection of it) |
| `bridge.warning` | `Engine.Launch` — the unit-10 chokepoint: exactly ONE event per non-zero launch exit, after the launch-error persist and the boot-strike record (breakdown unit 10, 2026-09-14) | WARN `BRIDGE_EXIT_<class>` — the exit table's signal column (`launchoutcome.Outcome.Signal`): `_SAFETY_GATE` (2), `_COST_LEAK` (3), `_BAD_FLAGS` (10), `_REPL_BOOT_TIMEOUT` (80), `_ARTIFACT_TIMEOUT` (81, one code whatever the sub-cause), `_UNKNOWN_PROMPT` (85), `_RESPOND_LOOP_GUARD` (86), `_REQUIRED_TIER_UNAVAILABLE` (99), `_COMMAND_TIMEOUT` (124), `_MISSING_BINARY` (127), `_SIGNAL_DEATH` (-1), `_DRIVER_ERROR` (unknown); the reason is the classified error string the outcome record and the retry backoff parse | `call_id`, `cli`, `agent`, `step=classify`, `exit_code`, `cause_code` (the llm-calls.ndjson value, incl. an 81 sub-cause), `transient`, `ctx_cancelled`, `launch_error` (the persisted `<agent>-launch-error.txt` when written) |
| `bridge.warning` | `Engine.clearBootStrike` / `Engine.recordBootStrike` / `Engine.persistLaunchError` / `Engine.readResult` — the Launch spine's four best-effort steps (unit 10); the first two replaced the module's last two `[engine]` stderr lines 1:1, the last two were silent `_ =` / `err == nil` sites | WARN `BRIDGE_BOOT_STRIKE_CLEAR_FAILED` / `BRIDGE_BOOT_STRIKE_RECORD_FAILED` / `BRIDGE_LAUNCH_ERROR_PERSIST_FAILED` / `BRIDGE_RESULT_READ_FAILED`; the launch's classified error is unchanged by any of them | `call_id`, `cli`, `agent`, `step` (`clear_boot_strike` / `record_boot_strike` / `persist_launch_error` / `read_result`), `path` (persist, read), `completion` (read) |

Wiring: `bridge.Deps.Signals` (nil = Null Object, tests only) → `Engine.SignalsWired()`; the production
Adapter is `adapters/bridge.NewDefault(projectRoot, signals)` — explicit DI at construction, no setter
to forget — and `productionEngineDeps` threads it into every engine it builds (`Adapter.SignalsWired`,
proven on the deps AND through the real `engineFactory`); the root passes its Center
(`TestWireOrchestratorDeps_SignalCenterReachesTheBridge`, via `orchDeps.Bridge`) and every other
`bridge.NewDefault` call site — the eight phase-registry defaults, `cmd_campaign`, tests — passes an
explicit `nil` (`TestNilSignalBridgeRootsAreExplicit`). The dispatch identity every bridge event carries
comes from `BridgeRequest` (`Cycle`, `RunID`, `Agent` as the phase) — one `dispatchIdentity` rule, the
driver's Config carrying the same values, no fallback (a request without a `Cycle` is an operator probe
whose signals stay at cycle 0). Deleted 1:1: `[engine] WARN: Deps.TokenResolver is
nil`, the `[engine] WARN: <event> call_id=…` writer, `[engine] TRIPWIRE: …`. The telemetry test suites
(tripwire, context fill, resolver warnings) now assert what the root's WARN-filtered stderr sink renders
— the one line format the operator reads — instead of hand-written text. `log.SanitizeField` also folds
Unicode format characters (bidi overrides, zero-width joiners, the BOM): the old quoted field escaped
them, the sink renders plain, so a resolver error can no longer reorder a terminal line. The rename
(commit 1) is mechanical and reviewed on its own: `panestream.SignalCenter` → `LivenessCenter`
(`NewLivenessCenter`, `LivenessEvent`, `LivenessHandler`, `RegisterLivenessHandler`), files moved,
ADR-0068/0070 carry an "Amended by ADR-0101" note. Remaining hand-written lines in the module —
`[bridge]` (sandbox, launch validation, dry-run, wall corroboration) — are unit 10b's (the launch-validation
and driver lines are the DATA the unit-10 classifier mines, so they stay verbatim); the two `[engine]` lines
(boot-strike clear failed, boot-timeout bench record failed) landed as unit 10's
`BRIDGE_BOOT_STRIKE_CLEAR_FAILED` / `BRIDGE_BOOT_STRIKE_RECORD_FAILED` (2026-09-14); none of them was a fact
S3 signalled. Folded from the S3 architecture review (Block → fixed): the
§5.2 severity cell above now defers to the registry instead of restating which states warn (the
copy had already diverged); every bridge signal of one dispatch carries ONE derived
`dispatchIdentity` (cycle, run, phase = the agent role) with no workspace-path fallback — a request
without a `Cycle` is an operator probe whose signals stay at cycle 0; `event` takes its origin
explicitly; `Deps.LivenessCenter` documents that an injected center must be per-dispatch (handlers
accumulate; there is no unregister). From the re-review: the ADR and this section no longer describe
the deleted fallback, and the timeout summary's `livenessOrUnknown` is a projection of
`LivenessState.String` (it had grown its own switch, missing `exhausted`). Follow-up outside this
slice: CI compiles only `acs/regression/...` under `-tags acs`; widen to `./acs/...` behind a
shrinking allowlist of the four fossil packages so a signature change can never break a frozen
predicate silently again.

### 15.2 Landed — S2a (2026-09-13)

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `cycle.sealed` | `cycleRun.completeCycle` — the closeout both dispatch roots share, after `finalizeCycle` so the FINAL verdict is known; the last orchestrator event of every completed cycle. An abnormal exit (the cycle died mid-phase) seals from `cycleRun.abnormalEpilogue` instead: FAIL, the abort reason as `termination_reason` — so "how did cycle N end" has one answer on every path | INFO; WARN `ORCHESTRATOR_CYCLE_FAILED` on FAIL | `final_verdict`, `phases_run`, `termination_reason`, `retro_decision` (`shipped` arrives in S2b with `CycleState.Shipped`) |
| `system.failure` | `cycleRun.completeCycle`, before the seal, when any floor attached a `SystemFailureSignal` (incoherence, lost landing, audit-fail floor, decision branches) | INCIDENT `ORCHESTRATOR_SYSTEM_FAILURE` when `Halt`, WARN otherwise | `category`, `level`, `halt` |
| `ship.error` | `Orchestrator.recordShipError` — where core records the phase's typed error (`ship-error.json` + ledger) | WARN `SHIP_<code>` (`shiperr.SignalCode`); INCIDENT for the `integrity` class — `ShipErrorClass.SignalSeverity`, the rule's one home, beside the vocabulary | `class`, `stage`, `path` |
| `quota.paused` | `cycleRun.pauseForQuota` — the seam BOTH dispatch roots reach (the resume root's deferred guard never calls the epilogue for a pause, so the epilogue was the wrong seam — S2a architecture review); its hand-written `[orchestrator] WARN` line deleted | WARN `ORCHESTRATOR_QUOTA_PAUSED`; the reason is the one error text the ledger and the phase diag carry | `phase` |

Design choices: every producer is an Adapter from a value the pipeline already owns
(`CycleResult`, `SystemFailureSignal`, `ShipError`, the `ErrAllFamiliesExhausted` sentinel) — no
new state, no new decision, one call line at each chokepoint and the body in `signal_cycle.go`
(`verdictReason`-style: one rendering per fact). The ship vocabulary is projected, not copied
(`SignalCode`, `AllCodes`, one `codeDocs` table registered at init;
`TestCodeDocs_CoverEveryDeclaredShipErrorCode` parses `shiperr.go` so a new constant cannot slip in
undocumented — no hand-maintained count anywhere; the generated doc is the only place a count
lives). `Center.Flush` (`sync.Cond` on drain end) is deferred at both roots. The code catalogue is a
generated projection of `RegisteredCodes()` (`RenderCodes` → `evolve signals codes generate|check`,
`docs/architecture/signal-codes.md`) gated by `TestSignalsCodes_RepoDocIsInSyncWithTheLinkedRegistry`
in `cmd/evolve` — the binary that links every module. The hand-written
`[orchestrator] SYSTEM-FAILURE HALT`, `LANDING LOST` and quota-pause `WARN` lines were deleted in the
same slice; the sink renders them in the one line format. Folded from the S2a architecture review
(Block → fixed): the quota producer moved from the epilogue (unreachable on the resume root) to
`pauseForQuota`, with fresh-root, resume-root (integration) and direct proofs; the class → severity
rule moved beside the vocabulary; the completeness proof is parsed from source; `emitCycleClose` is
a `cycleRun` method so every producer stamps the one run id (`cs.RunID`); an abnormal exit now seals
the cycle. From the re-review (Approve): `origin` is a parameter of the seal producer, so the
abnormal-path seal names `cycleRun.abnormalEpilogue` (a callee never asserts its caller's identity —
`origin` is vocabulary), and the ship-class severity table walks the declared classes from source
like the codes do. Accepted as-is: the textual Flush-wiring pin (convention-consistent; the behavioural
proof is the Flush test) and the `emit…` producer naming beside the loop's `emitQuotaPause` (the
Signal Center producers keep one prefix; the loop's report emitter is S4's to rename).

### 15.5 Landed — S2b (2026-09-13): the contract gate reports, the ladder reports, the prompt states the gate's criteria

Operator direction (2026-09-13): *"orchestrator … should check if each phase deliverables match to
the policy through the signal center, it knows where to search and check if all the output docs /
codes are ready (without looking into the context, it just checked if files are legit and existed)
and by-pass to the next phase"*, and *"the pass criterions for gating the output should be sent to
phase agent as part of input … please verify"*.

**Finding.** The check existed and was wired on both roots (ADR-0100: `reviewAndGuard` →
`reviewWithCorrections` → `reviewDeliverable` → `deliverable.Reviewer.Review` → `VerifyWithStage`;
resume: `reviewResumedDeliverable`), deterministic Go with no model reading the files — but its
decisions never reached the Center: `gate.contract`, `gate.rejected` and `gate.corrected` were
declared in the closed sets with **zero producers**, and the production streams held no gate event.
The prompt carried the contract block (artifact path, required sections or JSON keys, verdict
sentinel, the `evolve phase verify` self-check) but the two ADR-0100 additions the gate enforces —
the agent-owed secondaries and the declared effects — lived only in persona prose, so a registry
change would have moved the gate without moving the prompt.

| Producer | Chokepoint (origin) | Severity / code | Fields |
|---|---|---|---|
| `gate.passed` | `deliverable.Reviewer.Review` through `gatesignal.Reporter` (module `gate.contract`; origin `Reviewer.Review`) | INFO `GATE_CONTRACT_VERIFIED` (the reason names the artifact and its size, the owed files, the effects) · INFO `GATE_CONTRACT_SALVAGED` · WARN `GATE_CONTRACT_WOULD_BLOCK` (shadow/advisory stage, or the report-size gate's) · WARN `GATE_CONTRACT_DEMOTED` (breaker open: advanced UNVERIFIED) · WARN `GATE_CONTRACT_FAIL_OPEN` | `artifact`, `bytes`, `owed`, `effects`, `stage` · `pattern` · `codes`, `salvage` · `blocks` |
| `gate.rejected` | same | WARN `GATE_CONTRACT_REJECTED` — the reason IS the correction directive (one `[code] message` per violation, from the ONE `summarize()`) | `codes`, `blocks`, `threshold` |
| `gate.corrected` | `Orchestrator.emitGateCorrection` (module `orchestrator`) from `cycleRun.reviewWithCorrections` (fresh root) and `Orchestrator.reviewResumedDeliverable` (resume root) — one per correction re-dispatch | INFO `ORCHESTRATOR_GATE_CORRECTION` | `correction`, `max`, `rung`, `cli`, `escalated`, `salvage_retry` |

**The record per phase boundary.** `gate.passed` (verified: what was found where) → `phase.outcome`
(the advance); or `gate.rejected` → `gate.corrected` (1/2, redispatch, cli) → `gate.passed` →
`phase.outcome`; or `phase.aborted` after the ladder (its `abort_reason` names the file). The batch
report line adds `· gates: P passed, R rejected, C corrected` (`formatSignalReport`). The check
itself is unchanged: verdicts, transitions and the ladder stay with the floors; the Center only
carries the evidence (§8).

**Wiring.** `deliverable.WithSignals(signals)` — a functional option the four reviewer constructors
accept — at `wireOrchestratorDeps`; the proof `Orchestrator.ContractGateSignalsWired()` asks the
chain for the capability (`VerifiesDeclaredDeliverables` + `SignalsWired`), the
`DeclaredDeliverablesGateWired` precedent, because core cannot name the deliverable type.

**The prompt.** `RenderContractBlockStage` (the cache-safe block) names the agent-owed files and
points at the tail for their paths ("Also write these agent-owed files at the EXACT paths listed
under <owed-files> at the END of this prompt: … the gate verifies each exists, is non-empty and
parses") and names the declared effects ("verified at the phase boundary; your instructions say
how"); `RenderContractTail(c, artifactPath, workspace)` renders each owed file as
`<owed-file>/abs/path</owed-file>` through `phasecontract.OwedPath(workspace, name)` — the ONE join
the gate's `verifySecondaries` reads through (basenames only; a declared separator never steers a
read outside the workspace) — and the effects as `<effects>`. The bridge passes `req.Workspace`,
not the artifact's directory (a dispatched-artifact override can move the deliverable elsewhere).
Names come from `phasecontract.Contract` (filled from the registry by `FromSpec`), locations from
the one join: the gate and the prompt cannot drift on either. Contracts without owed files or
effects render byte-identical prompts (pinned).

**Patterns.** Observer at the ONE decision point (the gate stays the one verifier; the reporter only
reports — `internal/deliverable/gatesignal`, a leaf over `signalcenter`); Null Object (nil Center);
functional options; single-source-with-projection (`summarize()` feeds the log line, the signal
reason and the correction directive; `signal-codes.md` regenerated — 84 codes).

**Kept / not done.** The gate's `[contract-gate]` log lines and the ladder's `[orchestrator]` lines
stay (S5 retires them per module; one `summarize()` source, so they cannot drift from the signal).
`gate.eval` / `gate.repo` producers — the other gate modules of the closed set — are the same shape,
next slice. A salvage rung emits no `gate.corrected` (only a re-dispatch is a correction). The rest
of the original S2b row (`fields.shipped` on `cycle.sealed`, the classification fold, the WARN
budget pin) stays in S2c.

**Tests.** `gatesignal` 100 % lines + every export named (apicover); the reviewer: one test per
decision (verified, rejected, shadow would-block, report-size would-block, demoted, fail-open,
salvaged, would-salvage, off and unwired silent), the capability through the core interface; core:
one re-dispatch → one `gate.corrected` on the fresh root, the escalated second correction says so
with its CLI, the resume root's ladder; cmd: the root wiring proof and the report clause;
phasecontract: owed files and effects rendered in block and tail, no clause otherwise; the kind set
closed at 22. Fourteen build-confirmed mutants killed by name: the rejected, verified,
demoted, fail-open and report-size would-block emits removed; the owed files and effects not
projected into the verified event; the fresh-root and the resume-root ladder emits removed; the
`escalated` field hard-coded; the root not passing its Center to the gate; the report clause never
rendering; the owed-files clause never rendering; empty-valued fields kept; a rejection emitted as
`gate.passed`.

**Review folds (fleet: code-simplifier → architecture-reviewer ∥ go-reviewer; both Warning, no
CRITICAL).** HIGH — the owed-file location was a belief with two homes (the gate's
`filepath.Join`, the prompt's "SAME directory" prose): `phasecontract.OwedPath` is the one join,
the tail renders the exact paths, the block points at the tail. MEDIUM — the verified event
re-resolved the contract instead of reporting what was checked: `Result.Owed` / `Result.Effects`
(json:"-", the single-read seam `Result.Content` set) are filled by the verifier and the signal
projects them. MEDIUM — the ladder producer took ten positional parameters with two adjacent
bools: `gateCorrection` parameter object. MEDIUM — the report-size constructor assigned its
settings after the options: `withReportSize` is an option applied at the one point. LOW / Go
review — `Reporter.emit` filters into its own map; the correction ordinal has one home
(`fields.correction`, no `Attempt`). Seven more build-confirmed mutants (twenty-one in all): the
join not stripping a directory, the tail rendering the bare name, the owed files or effects not
recorded by the verifier, the options applied before the report-size setting, the caller's map
mutated, the gate reading a path the tail did not render.
