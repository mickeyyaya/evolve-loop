# ADR-0101 — Signal Center: one event stream every component produces into and the orchestrator listens to

- **Status:** Proposed (2026-09-13). Slice S0 (this ADR + [signal-center-design.md](../signal-center-design.md)
  + the [inventory](../../research/signal-center-inventory-2026-09-13.md)) lands as a docs-only PR; S1
  (the `signalcenter` package, the orchestrator listener, the C1 chokepoint as first producer, the
  durable + stderr sinks, the composition-root wiring proof) lands after PR #575/#576/#577 merge,
  because it builds on the C1 record's `diagnostics` (PR #577); S2 additionally depends on PR #575 (the
  contract-gate codes it maps). S3–S5 follow as their own slices. **This ADR amends ADR-0068 and
  ADR-0070:** their `SignalCenter` type is renamed `LivenessCenter` (decision 8); their decisions are
  otherwise unchanged. Design review 2026-09-13 (architect agent): APPROVE-WITH-FIXES — all findings
  folded into the [design](../signal-center-design.md) and this ADR before S0 landed.
- **Driving evidence:** the operator's 2026-09-13 directive — *"Orchestrator should be the main contact
  point to listen and receive updates from a centralized information service where all phase agents are
  wired to … there should be a center to receive all updates from all phases … each module needs to
  define a set of unified system-level error codes and reasoning and logging that help issue triage; the
  logging should include the module/class name … the ultimate goal is that once we find an issue we can
  find the root cause by reading the logs within a few review steps."* — and the survey it prompted:
  - The only thing named a signal center, `panestream.SignalCenter` (ADR-0068), is tmux-pane liveness,
    built privately **per REPL dispatch**, with **zero production subscribers** and an event that carries
    neither cycle nor phase (`bridge/panestream/signalcenter.go:26-36`, `driver_tmux_wait_state.go:90`).
  - **≥ 15 unwired cycle-level sinks**: `abnormal-events.jsonl` has four writers and no Go reader;
    `llm-calls.ndjson`, `<phase>-interactions.ndjson`, `failedApproaches`, the lessons corpus, the
    `[engine] WARN/TRIPWIRE` lines, the shadow hooks, guards/session/boot-strike logs — each its own
    file, its own shape, nothing subscribed.
  - **Three severity vocabularies** (`cyclestate` error/warning · `dispatchevents` INFO/WARN/ERROR ·
    `observer` info/warn/incident — the schema-1.0 contract) and **two classification vocabularies**
    (`failureadapter` 11 values, `failurelog` the same 11 + 3).
  - **361 hand-written `fmt.Fprint*(os.Stderr, …)` sites** (350 of them opening with a bracketed module
    prefix — `[orchestrator]` alone 284), plus module prefixes written through other writers
    (`[loop]` 132 and `[ship]` 130 occurrences of the literal), against 23 uses of the shared `log.Diag()`.
  - The cost, twice in one night: cycles 1634 and 1636 each took an operator investigation to learn that
    a triage FAIL was a working-as-designed gate, because the phase's own reason had no durable home
    (fixed narrowly by PR #577); cycle 1630 was labelled shipped because the label read a HEAD delta
    instead of the cycle's own evidence (PR #576). Both are the same disease: facts produced in one place
    with no channel to the place that decides.
- **Related:** [ADR-0044](0044-unified-phase-recovery-protocol.md) (the C1 chokepoint — the first
  producer), [ADR-0068](0068-bridge-signal-center-concurrency.md) (the pane-liveness center this ADR renames),
  [ADR-0072](0072-system-failure-policy-and-halt.md) (system-failure halts — an INCIDENT consumer),
  [observer-severity.md](../observer-severity.md) (the severity contract this ADR adopts unchanged),
  [phase-observer.md](../phase-observer.md) (the unified envelope this schema stays compatible with),
  [abnormal-event-capture.md](../abnormal-event-capture.md) (the stream `dispatchevents` writes today),
  [ADR-0070](0070-signal-center-exhaustion-signal.md) (exhaustion as a first-class signal in that bridge
  center — this ADR is its cycle-level counterpart, not a replacement), [ADR-0074](0074-typed-signal-contracts-routing-authority.md)
  (its principle — *signals that cross a boundary must be typed, enforced contracts, never prose* — is
  the principle behind decisions 1 and 4), [ADR-0039](0039-failure-floor-and-failure-signal-contract.md)
  (the deterministic failure floor; `faillearn.FailureEvent` becomes a producer in S4).

## Decision

1. **One event schema.** `signalcenter.Event` — `schema_version, seq, pid, ts, cycle, run_id, phase, attempt,
   module, origin, kind, code, severity, reason, fields`. It carries the operator's full tuple (module,
   class/function = `origin`, error `code`, `reason`, `severity`, `cycle`, `phase`, `attempt`); no
   existing struct does (inventory §4). It stays compatible with the observer envelope's
   `source{component,cycle,phase}` / `type` / `severity` shape.
2. **One severity vocabulary — the existing schema-1.0 contract.** `INFO` / `WARN` / `INCIDENT`
   (10/20/30) from `observer-severity.md`, adopted verbatim; no new tier. Adapters map the other two
   vocabularies (`dispatchevents` ERROR → INCIDENT; `cyclestate` `error` diagnostics on a FAIL → WARN with
   the reason, because a verdict is a disposition the pipeline already handles, not an accident).
3. **One process-scoped Center (Observer / publish–subscribe), built at the composition root and
   injected.** `signalcenter.New()` in `cmd/evolve/cmd_cycle.go`, passed to the orchestrator via
   `core.WithSignalCenter` and to the bridge via `bridge.Deps`. Ordered fan-out through one queue
   and one drainer (`Emit` stamps `seq` under the Center's mutex and enqueues; the first emitter
   with no drain in progress delivers, in `seq` order, to a listener snapshot with no lock held, so
   every listener sees the same total order); a listener panic is recovered, the listener is
   unsubscribed and the INCIDENT report is enqueued like any other event (there is no second
   dispatch path); a listener that emits enqueues and returns; nothing is ever dropped. A nil
   `*Center` is a Null Object (every method nil-safe) — a
   **test affordance only**: production roots always construct a Center, before the bridge engine
   (`bridge.NewEngine` normalizes `Deps` at construction), and a repo test pins the only nil sites
   (`cmd_cycle_simulate.go`, `routingtest`). Optional nil-default injection is exactly how
   `bridge.Deps.LivenessCenter` stayed unset in production; it is not repeated here.
4. **Closed vocabularies, registered codes, never silent.** `Module` and `Kind` are closed sets declared
   in the package; `Code` is `MODULE_SNAKE_CASE` and every module registers its codes with a one-line
   doc (`RegisterCode`). An event with an unregistered code or unknown kind/module is not dropped: it
   is stamped `SIGNALCENTER_UNREGISTERED_CODE` / `SIGNALCENTER_UNKNOWN_KIND`, keeps the raw value in
   `fields`, and is raised to at least WARN — the drift itself becomes a signal.
5. **The orchestrator registers as a listener at construction** and keeps a per-cycle
   `SignalSummary` (counts by severity and kind, the last INCIDENT) exposed through the orchestrator
   only — `Orchestrator.SignalSummary()`, not a copy on the cycle result — so the loop and the dashboard read the orchestrator's view rather than
   re-deriving it from files. INCIDENT-driven halts stay with ADR-0072's `SystemFailureSignal`, which
   becomes a producer (S2); the listener does not add a second halt path. **Listeners observe, never
   decide:** no consumer may treat a signal as authoritative for a verdict, a state transition or a
   halt — the ledger, the cycle state and the ADR-0072 artifacts remain the deciding evidence.
6. **Two default listeners at the root:** a durable `signals.ndjson` per cycle workspace (the ONE file
   to read first when triaging a cycle; every event, `seq`-ordered, `pid`-stamped so a resumed cycle's
   second process is distinguishable) and a stderr sink filtered to WARN and above (INFO is "log
   only" in the severity contract) with ONE line format —
   `[module] kind SEVERITY CODE cycle=N phase=p attempt=k seq=s origin=Type.Method — reason k=v …` — which is what
   "logging includes the module/class name" means from now on. Hand-written `[x]` prefixes migrate to
   `Emit` module by module during decomposition; a module is "done" when it emits and does not print.
7. **Producers are the existing chokepoints, adapted in place — no second recorder.** Phase agents
   (LLM subprocesses) never get an `Emit` API: agent-authored facts enter only through the phase
   deliverable (`cyclestate.Diagnostic`, `phaseio.ErrorContext`), adapted by the host — an untrusted
   producer must not write the stream the orchestrator reads (ADR-0074's boundary rule). S1: the ADR-0044
   C1 chokepoint (`recordPhaseOutcome`) emits `phase.outcome` for every terminal disposition on both
   dispatch roots (its PR #577 log line becomes the sink's line). S2: `SystemFailureSignal`,
   `shiperr.ShipError` (every code, namespaced `SHIP_*`), contract-gate rejections, quota pauses. S3: the
   bridge engine's WARN/TRIPWIRE/CONTEXT-FILL and pane liveness. S4: the ledger port (a decorator emits
   `ledger.appended`), `dispatchevents` writers, the observer adapter. The ledger's hash chain is NOT
   replaced — provenance stays where it is; the Center is the monitoring stream beside it.
8. **`panestream.SignalCenter` is renamed `LivenessCenter`** (done: S3 commit 1, 2026-09-13, amending
   ADR-0068/0070 — `bridge.Deps.LivenessCenter` already carries the target name, so the rename removes
   an existing name/meaning mismatch) and becomes a producer of `pane.liveness` edges; one name, one
   meaning.
9. **Definition of done per slice:** the new package ships with 100 % exported-API coverage (apicover
   enforced) and 100 % line coverage (a CI/`make` threshold step, never a test reading its own
   profile), `-race` clean, every file < 800 lines, functions < 50, nesting ≤ 4;
   every producer proven by a composed test (the event is observed by a listener when the real path
   runs), every wiring proven at the composition root (`SignalCenterWired`), mutants killed by name.

## Rejected

- **Extend `panestream.SignalCenter`.** Per-dispatch lifetime, liveness-only event, bridge-internal
  reach, zero subscribers; every property the requirement needs is missing. Renaming it is cheaper
  than teaching it a second job.
- **Use the ledger as the bus.** The ledger is a hash-chained provenance record with 23 fields, 26
  append sites and a byte-stability golden; adding severity/code/attempt would widen a security
  contract to carry monitoring chatter, and its append is a file write, not a fan-out.
- **Adopt `dispatchevents.Event` as-is.** Best existing schema, but it lacks module/origin/code/attempt,
  is written inline by four producers with no reader, and its ERROR tier conflicts with the schema-1.0
  contract. Its closed-vocabulary discipline (`IsKnownEventType`) is kept; its file becomes a
  Center-fed output during migration.
- **An asynchronous channel bus.** Ordering and back-pressure become test problems, and a listener
  that lags could miss the very INCIDENT the orchestrator must act on. Synchronous fan-out with
  panic isolation is deterministic and testable; listeners that need to be slow (network, disk
  batching) buffer internally.
- **A fourth severity tier (`ERROR`).** The contract says: no new tiers without a schema RFC. The
  triage identity lives in `code` + `kind`; severity stays the response tier.
- **A structured-logging library as the center.** Logging is one listener; the Center's contract is
  the schema, the registry and the orchestrator's subscription. A logger cannot give the orchestrator
  a per-cycle summary or a halt signal.

## Consequences

- **Positive.** One place to read (`signals.ndjson`), one line format, one severity vocabulary, one
  code registry with module namespaces, the orchestrator sees every phase's facts as they happen, and
  each future decomposition slice gets its module tag, its codes and its producer tests for free.
  The unwired-sink count (≥ 15) and the hand-written stderr count (361 sites; per-module prefix
  occurrences `[orchestrator]` 287, `[loop]` 132, `[ship]` 130, …) become measurable and go to
  zero per module.
- **Negative / risks.** Double emission during migration (a hand-written line and the sink line) —
  mitigated by removing the hand-written line in the same slice that adds the `Emit`, and by a test
  that greps the touched module for `Fprintf(os.Stderr, "[module]`. Synchronous fan-out adds a
  listener's latency to the emit site — bounded by keeping listeners O(1) and by the ndjson sink's
  append-only write. `signals.ndjson` growth — bounded per cycle; the existing gc retention applies.
- **Follow-ups filed.** Unify `failureadapter.Classification` and `failurelog.Classification` (one
  vocabulary, one home) — S2. Fold `subagent.AppendAbnormalEvent`'s hand-rolled JSON into the Center —
  S4. Decide the fate of `abnormal-events.jsonl` once every writer emits through the Center — S4.
  The per-module decomposition order and method live in the design doc §12.

## Implementation notes — S1 (landed 2026-09-13)

`internal/signalcenter` (schema, Center, registry, sinks, filter, `Summary`), `core.WithSignalCenter`
+ the orchestrator listener, the C1 chokepoint as the first producer on both dispatch roots, and the
composition-root wiring (durable `signals.ndjson` sink + WARN-filtered console sink) landed as one PR
with 100 % line coverage enforced in CI (`make cover-strict`). Deltas from the decisions above, each
recorded in the design document (§15.1): delivery is one queue and one drainer with no lock held
during delivery (decision 3's "already-held emit lock" dispatch path does not exist — self-reports
enqueue like any event); vocabulary drift is stamped onto the offending event and registry conflicts
are recorded, never emitted (decision 4 — `signalcenter.registry_drift` survives only as the
replacement kind for an event whose own kind is unknown; there is no `registry_conflict` kind); the
per-cycle summary is `signalcenter.Summary`, follows the cycle by itself
and is read only through `Orchestrator.SignalSummary()` (decision 5). Nothing in the decisions'
intent changed: one schema, one center, closed vocabularies, observe-never-decide.

## Implementation notes — S3 (landed 2026-09-13)

Decision 3's second injection point is real: `bridge.Deps.Signals` reaches every engine the
production Adapter builds because the Adapter takes the Center as a constructor argument
(`adapters/bridge.NewDefault(projectRoot, signals)`), and `Engine.SignalsWired()` / `Adapter.SignalsWired()`
are the proofs at both seams; every Center-less `NewDefault` site passes an explicit `nil` and a test
pins them. The engine's telemetry warnings and tripwire are `bridge.warning` / `bridge.tripwire`
(module `bridge`, six codes); the tmux driver registers a `LivenessHandler` per dispatch that turns
every `LivenessCenter` edge into `pane.liveness` (module `liveness`, three WARN codes) — the design's
"Adapter over the existing hook" rather than a change inside the liveness facade. Decision 8 landed
as its own commit. Deltas from the plan: the dispatch identity comes from `BridgeRequest` with the
driver Config carrying the same values — ONE `dispatchIdentity` rule, no workspace-path fallback
(a request without a `Cycle` is an operator probe whose signals stay at cycle 0); `log.SanitizeField`
folds Unicode format characters
too (the sink renders plain text where the old field was quoted).

## Implementation notes — S2a (landed 2026-09-13)

The producers that need only S1: `cycle.sealed` and `system.failure` at the closeout both dispatch
roots share (`cycleRun.completeCycle`; INCIDENT when the failure halts the loop), `ship.error` at
`Orchestrator.recordShipError` with the ship vocabulary projected as `SHIP_<code>` (every code
registered with a doc, completeness parsed from source — decision 4's "one vocabulary, one home";
`ShipErrorClass.SignalSeverity` keeps the class → severity rule beside the vocabulary), and
`quota.paused` at `pauseForQuota`, the seam both dispatch roots reach (the abnormal epilogue was
the wrong seam: the resume root never calls it for a pause). An abnormal exit seals the cycle FAIL
from the epilogue, so every path ends the cycle's stream with one `cycle.sealed`. `Center.Flush`
closes decision 3's only gap (an emitter that finds a drain in
progress returns before delivery): both roots defer it. The code catalogue
`docs/architecture/signal-codes.md` is generated from the registry (`evolve signals codes
generate|check`) and its currency is a `cmd/evolve` test. Deferred to S2b: `gate.rejected /
gate.corrected` (the contract-gate codes land with PR #575), `fields.shipped` on `cycle.sealed`
(PR #576's `CycleState.Shipped`), the `failurelog` / `failureadapter` classification fold.
