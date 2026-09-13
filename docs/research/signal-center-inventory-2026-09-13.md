# Signal Center inventory — every event, signal and recorder seam (2026-09-13)

> **Purpose.** The evidence base for [ADR-0101 — Signal Center](../architecture/adr/0101-signal-center.md)
> and [signal-center-design.md](../architecture/signal-center-design.md). Read-only survey of the
> `go/` module at origin/main `f786617e` plus the pipeline fixes of 2026-09-13 (PR #576, PR #577).
> Counts are from `rg` over non-test files unless stated. Paths are relative to the repo root.
>
> **Operator question answered:** *"Let me know if the centralized info service still exists or has
> been bypassed."* — It exists in name only. `panestream.SignalCenter` is tmux-pane liveness, built
> privately per REPL dispatch, with **zero production subscribers**; every cycle-level fact bypasses it.

## 1. `panestream.SignalCenter` today

| Aspect | Evidence |
|---|---|
| Definition | `go/internal/bridge/panestream/signalcenter.go:26` — `SignalCenter{sessions, registry, handlers []SignalHandler}` |
| Constructor | `NewSignalCenter()` — `signalcenter.go:66` (only constructor) |
| Event type | `SignalEvent{SessionKey string; State LivenessState}` — `signalcenter.go:36`. **No cycle, phase, module, code, severity or attempt.** |
| Subscribe | `RegisterSignalHandler(h)` — `signalcenter.go:94`; `RegisterHandler(name, factory)` `:79` is a probe registry, not a subscriber |
| Publish | `Observe(sessionKey, rendered, profile)` — `signalcenter.go:112`; edge-triggered dispatch `:151-156` |
| Who constructs | one production site: `go/internal/bridge/driver_tmux_wait_state.go:90-92` (`w.deps.LivenessCenter`, else `NewSignalCenter()`) |
| Injection seam | `bridge.Deps.LivenessCenter` — `go/internal/bridge/engine.go:188`; **never set by any non-test caller** |
| Lifetime | per REPL dispatch (`newReplWaitState`, `driver_tmux_wait_state.go:74`) — dies with the phase's wait loop |
| Production subscribers | **zero** (`rg RegisterSignalHandler --glob '!*_test.go'` → declaration + doc only) |
| Publishers | `driver_tmux_wait_checkpoint.go:35` (`Observe`) — the only caller; read-only consumers in `driver_tmux_wait_*.go`, `driver_tmux_channel.go:86`, `autorespond.go:427,482` |
| Reach | `core` and `cmd/evolve` never import `panestream` |

To subscribe today the orchestrator would need: a process/cycle-scoped owner, `bridge.Deps.LivenessCenter`
populated at the composition root, a `core.Option`, and a schema that carries cycle + phase. That is a
rebuild, not a re-wire.

## 2. Inventory of every event / signal / recorder seam

Transport: **mem** in-memory func/option field · **ws** file under `.evolve/runs/cycle-N/` · **ldg** `.evolve/ledger.jsonl` · **err** stderr line · **st** `.evolve/state.json`.

| # | Seam | file:line | Producer(s) | Consumer(s) | Transport | Schema | Wired at the composition root? | Orchestrator can listen today? |
|---|---|---|---|---|---|---|---|---|
| 1 | `recordPhaseOutcome` (ADR-0044 C1 chokepoint) | `go/internal/core/failure_learning.go:119` | 13 sites (`quota_pause.go:30`, `phase_completion.go:60`, `evaluate_batch.go:167`, `cyclerun_select.go:154`, `cyclerun_remediate.go:88,132`, `cyclerun_correction.go:278,290,298,325,330`, `retry_opts.go:149`, `cyclerun.go:212`, `cyclerun_dispatch.go:321,371`) | phase-timing.json, usage sidecar | ws+mem+err | `recovery.PhaseOutcome` → `phasetiming.Entry` | it IS the orchestrator | internal, **no observer hook** — the natural producer #1 |
| 2 | `phasetiming.Entry` | `go/internal/phasetiming/phasetiming.go:29` | `writePhaseTimings` `failure_learning.go:236` | dossier `dossier_producer.go:75`, dashboard `dashboard/model.go:103`, `gc/discover.go:20`, cyclecost, cyclehealth | ws | 16 fields incl. `diagnostics` (PR #577) | UNWIRED (file) | file poll only |
| 3 | `LedgerEntry` appends | `go/internal/core/ports.go:89`; writer `adapters/ledger/ledger.go:93` | **26 `o.ledger.Append` sites** in core | ship gate, `evolve ledger verify`, gc, campaign | ldg | `core.LedgerEntry` (23 fields; no severity/code/attempt) | **WIRED** (`cmd/evolve/cmd_cycle.go:750`) | yes in principle — a real port; a decorating listener could wrap it (closest thing to a bus) |
| 4 | `emitPhaseBindings` | `go/internal/core/phase_bindings.go:19` | `cyclerun_record.go:51`, `resume_execution.go:265` | ship's audit-binding verifier | ldg | `LedgerEntry` | inherits #3 | via ledger |
| 5 | Observer seam `o.observer.Start` | iface `go/internal/core/observer.go:34`; noop `:52` | `cyclerun_dispatch.go:225`, `cyclerun_remediate.go:121,168`, `cyclerun_correction.go:269`, `retry_opts.go:172`, `failure_learning.go:499`, `resume.go:448` | `adapters/observer.CoreAdapter.Start` `core_adapter.go:68` | ws `<phase>-observer-events.ndjson` + `Sink io.Writer` | `observer.Event{TS,Type,Severity,Cycle,Phase,Agent,Reason}` | **WIRED** `cmd_cycle.go:601` (`WithObserver`, guarded by `Autospawn`) | closest existing listener shape; `Sink` unset in production |
| 6 | Chronicle digest | `cyclerun_chronicle.go:33`; `cyclerun.go:795` | orchestrator | phase agents (`recent-outcomes.md`) | ws | markdown, unstructured | `WithChronicleConfig` | no |
| 7 | `recordFailureLearning` | `failure_learning.go:435` | retro path | retro agent, KB | mem+ws+ldg(:517)+st | unexported request | internal | no |
| 8 | `writeFailureLearningState` | `failure_learning.go:473,480,507` | same | state.json | st | `core.State` | internal | no |
| 9 | `failurelog.Record` | `go/internal/failurelog/record.go:67` | `cmd_loop_sequential_dispatch.go:66`, `cmd_loop_sequential_verify.go:76,94`, `cmd_loop_control.go:120`, `cmd_loop_outcome.go:435` | maintenance pruner | st | `failurelog.Recorded` | UNWIRED (direct from cmd) | no |
| 10 | `faillearn.FailureEvent` | `go/internal/faillearn/faillearn.go:43` | `core/failure_learning.go:558`, `core/reset.go:216`, `cmd_loop_outcome.go:443` | lessons corpus, inbox | file | richest existing shape (Cycle, FailedPhase, Scope, Classification, Verdict, Summary, Defects, EvidencePaths, GitHead) | UNWIRED | no |
| 11 | `SystemFailureSignal` | `go/internal/cyclestate/result.go:71` | `audit_fail_decision.go:68`, `decision_branch.go:118,166`, `system_failure.go:190`, `lost_landing_floor.go:70`, `cmd_loop_blockerbreaker.go:65` | `cmd_loop_systemfailure_halt.go:45`, `cmd_loop_escalation.go:34` | mem (`CycleResult.SystemFailure`) | `{Category, Level, Evidence, Halt}` | returned by value | post-hoc only |
| 12 | `recordJudgmentLesson` | `judgment_lesson.go:66` | orchestrator | carryover todos | st | `[]Diagnostic` | internal | no |
| 13 | `throughputRecorder` | `throughput_hook.go:16` | orchestrator | `triagecap.Recorder` | st | mutator callback, no payload | **WIRED** `cmd_cycle.go:571` | no |
| 14–16 | `PhaseBoundary/Resume/QuotaBoundaryCheckpointer` | `orchestrator.go:30,37`, `resume_policy.go:10` | `phase_completion.go:42`, `failure_learning.go:531`, `cyclerun_epilogue.go:31`, `resume_execution.go:156`, `quota_pause.go:15` | `checkpoint.go:188,205,213` | ws/st | func vars | blank-import `init()` (`cmd_loop.go:20`) | no |
| 17 | `interaction.Event/Outcome/Recorder` | `go/internal/interaction/interaction.go:113,141,163` | bridge + correction ladder | rollup | ws `<phase>-interactions.ndjson` | has Kind, Phase, Cycle, Trigger, Rung | UNWIRED (ad hoc) | no |
| 18 | `interaction.Summary` | `rollup.go:22`; written `core/orchestrator.go:982` | finalize defer | cyclehealth, soakreport | ws `interaction-summary.json` | counters | in-orchestrator | no |
| 19 | `phaseobserver` (out-of-proc) | `go/internal/phaseobserver/` (601 lines) | separate process | operator | err+ws | `Config` | separate process | no |
| 20 | `phasewatchdog` stall kill | `phasewatchdog.go:282,199` | watchdog | abnormal-events readers | ws | untyped map, uses `dispatchevents` vocab | UNWIRED | no |
| 21 | **`dispatchevents.Event`** ⭐ | `go/internal/dispatchevents/events.go:83`; `Emit` `:132` | `cmd_loop_sequential_observe.go:21,49`, `cmd_loop_sequential_verify.go:41`, `cmd_loop_goalstall.go:288`, `core/phaseoutputs_signal.go:53` | **no Go reader of the file** (operator `jq`; the vocabulary alone has one reader, `phasewatchdog.go:296`) | ws `abnormal-events.jsonl` | `{EventType(closed), Timestamp, SourcePhase, Severity(INFO/WARN/ERROR), Details, RemediationHint, Cycle, Classification}` | UNWIRED (inline `NewWriter`) | no — strongest schema + vocabulary candidate |
| 22 | `log.SidecarWriter/EmitAbnormal` | `go/internal/log/events.go:48,62` | `deliverable/salvage_*.go` | none | file | `log.Event{EventType,Timestamp,Severity,Fields}` | UNWIRED | no |
| 23 | `cyclehealth.ClassifyOutcome` | `cyclehealth/outcome.go:67` | — | `cmd_loop_outcome.go:84`, `soakreport.go:70` | reads ws | 5-value `Outcome` | reader | consumer |
| 24 | `writeCycleDossier` | `dossier_producer.go:66` | `cycle_closeout.go:34`, `cyclerun_epilogue.go:97` | `cmd_loop_outcome.go:302` | committed dossier | `dossier.Dossier` | in-orchestrator | no |
| 25 | `Diagnostic` on `PhaseResponse` | `cyclestate/result.go:116`; projection `ErrorMessages` `:135` | every runner | chokepoint, `verdict.go`, cyclehealth, seal | mem→ws | `{Severity, Message}` — no code, no module | flows through response | in-proc, no hook |
| 26–28 | bridge `[engine] WARN / TRIPWIRE / TokenResolver nil` | `bridge/attempt_telemetry.go:30,189`, `engine.go:383` | engine | scrollback; `cmd_tokens.go:287` reads the ndjson twin | err | unstructured k=v text | UNWIRED | no |
| 29 | `log.Diag()` console | `go/internal/log/console.go:34` | **23 call sites** (runner 20, releasepipeline 2, adapters/bridge 1) | stderr | err | formatted strings, no fields | UNWIRED | no |
| 30 | `llm-calls.ndjson` | `llmcalls/record.go:81`; `store.go:45` | `attempt_telemetry.go:41` | `cmd_tokens.go`, `cycleclassify:459`, dashboard | ws | ~25 fields incl. `Attempt`, `CauseCode`, `Tripwire` | UNWIRED | no |
| 31 | `<phase>-usage.json` | `failure_learning.go:35,175` | chokepoint | `cyclecost:263-286`, `phaseoutputs.go:118` | ws | unexported sidecar | in-orchestrator | no |
| 32 | `token-usage.json` | `bridge/tmux_repl_report.go:20` | tmux driver | `bridge/report.go:102` | ws | snapshot | UNWIRED | no |
| 33 | `router.RoutingSignals` | `router/signals.go:65`; `digest.go:26` | `Digest` reads handoffs | router, conditions, policy | reads ws | pull-only digest | via `WithRouting` | pull only |
| 35 | `failuregrade.Grade` | `failuregrade.go:25,~85` | chokepoint `failure_learning.go:163` | stderr only | err | `Tier` + `Evidence` | UNWIRED (shadow) | no |
| 36–37 | `[graduated-enforcement SHADOW]`, `[verdict-cache SHADOW]` | `failure_learning.go:164`, `orchestrator.go:1021` | chokepoint, RunCycle | operator | err | unstructured; `WithVerdictCacheLookupHook` `:802` exists but **nothing registers** | UNWIRED | hook exists, unused |
| 38 | `catalog_refresh` ledger stamp | `cyclerun.go:679,695` | cycle start | ledger readers | ldg | `LedgerEntry` | `WithCatalogRefreshStage` `cmd_cycle.go:682` | via ledger |
| 39 | `dashboard` | `go/internal/dashboard/` (2059 lines) | — | HTTP/SSE `server.go:191` | polls files | `dashboard.Model` | `cmd_dashboard.go:34` | polls, no push |
| 41 | `soakreport` | `soakreport.go:61,160` | — | operator | reads ndjson | `Report` | reader | no |
| 42 | `subagent.AppendAbnormalEvent` | `subagent/helpers.go:32` | subagent-run, fan-out | none | ws | hand-rolled `fmt.Sprintf` JSON (`:44`) | UNWIRED | no |
| 43–48 | `guardslog.Append`, `sessionrecord.Append`, `clihealth.RecordBootStrike`, `recurrence.RecordClosure`, `checkpoint.RecordPhaseIntegrity`, `inboxmover.RecordRootTaskFailure` | see agent report | various | various | file | ad hoc | UNWIRED | no |
| 50 | `contractResolverSink` | `cmd_cycle_catalog_publisher.go:12` | `WithCatalogPublisher` | bridge resolver | mem | `func(Catalog)` | **WIRED** `cmd_cycle.go:583` | fan-out of one, catalog only |
| 51 | `panestream.SignalCenter` | §1 | tmux driver | none | mem | `SignalEvent{SessionKey, State}` | UNWIRED | no |

**Unwired cycle-level sinks (confirmed): ≥ 15** — rows 2, 9, 10, 17, 20, 21, 22, 26–28, 30, 35, 36, 37, 42, 43–48, 51. The earlier "≥ 8" was an undercount. Row numbers follow the survey's numbering; 34, 40 and 49 were "not found in this tree" items and are listed in the paragraph below the table instead of in it.

Not found in this tree (either renamed or outside it): `dispatchSignals` (closest: `router.RoutingSignals`), `E_PUSH` as a constant (live code is `shiperr.CodeGitPushRejected = "GIT_PUSH_REJECTED"`), an `evolve loop status` Go subcommand (the operator surface is `evolve dashboard` + the `/loop-status` slash command).

## 3. Existing registration / DI patterns

- `type Option func(*Orchestrator)` — `go/internal/core/orchestrator.go:503`. 33 `With*` options (`orchestrator.go:509–802`, `throughput_hook.go:16`, `failure_hook.go:44`, `composition_carryforward.go:68,75,83,165`) plus 7 sub-option types (`phase_advisor.go`, `failure_advisor.go`).
- Composition root: **`go/cmd/evolve/cmd_cycle.go`** — `opts := []core.Option{…}` from `:557`, applied at `:750` (`core.NewOrchestrator(st, ld, runners, opts...)`); helper `cmd_composition_wiring.go`. Secondary roots: `cmd_cycle_simulate.go:85` (option-less), `internal/routingtest/engine.go:171` (tests).
- Package-var DI: the three `*BoundaryCheckpointer` vars set by `internal/checkpoint`'s `init()` via the blank import at `cmd_loop.go:20`.
- Pub/sub anywhere in `go/internal`: exactly one — `panestream.SignalCenter.handlers`. Everything else is a single-slot callback. No `Subscribe/Broadcast/Publish/Notify` type exists; channels are all semaphores/worker pools except the dashboard's SSE fan-out (`dashboard/sse.go:62`).

## 4. Existing schema candidates

| Struct | file:line | Fields |
|---|---|---|
| **`dispatchevents.Event`** | `dispatchevents/events.go:83` | EventType (closed, 7), Timestamp, SourcePhase, Severity (INFO/WARN/ERROR), Details, RemediationHint, Cycle, Classification |
| `observer.Event` | `adapters/observer/observer.go:58` | TS, Type, Severity (info/warn/incident), Cycle, Phase, Agent, Reason |
| `log.Event` | `log/events.go:29` | EventType, Timestamp, Severity, Fields map |
| `cyclestate.Diagnostic` | `cyclestate/result.go:116` | Severity, Message (+ `SeverityError/Warning`, `ErrorMessages`) |
| `core.LedgerEntry` | `core/ports.go:89` | 23 fields (TS, Cycle, Role, Kind, ExitCode, Message, Action, RunID, hash chain…) |
| `phasetiming.Entry` / `recovery.PhaseOutcome` | `phasetiming.go:29` / `recovery/outcome.go:41` | phase, verdict, timing, tokens, abort_reason, diagnostics… |
| `cyclestate.SystemFailureSignal` | `result.go:71` | Category, Level, Evidence, Halt |
| `core.VerdictReason` / `Taxonomy` | `core/verdict.go:22,40` | Status, Summary, Taxonomy{Source, FailureMode, Consequence} — "deliberately parallel to ShipError.{Stage,Code,Class}" |
| `faillearn.FailureEvent` | `faillearn.go:43` | Cycle, FailedPhase, Scope, Classification, Verdict, Summary, Defects, EvidencePaths, GitHead |
| `interaction.Event/Outcome` | `interaction.go:113,141` | Kind, Phase, Cycle, Trigger, Rung, DecisionID, Payload / Result, LatencyMS, CostUSD |
| `llmcalls.Record` | `llmcalls/record.go:81` | CallID, Phase, CLI, Model, Attempt, Tokens, UsageStatus, ExitCode, Tripwire, FillPct, CauseCode… |
| `shiperr.ShipError` / `phaseio.ErrorContext` | `shiperr.go:29` / `phaseio.go:24` | Code, Class, Stage, Message/Debug |
| `subagent.AbnormalEvent` | `subagent/helpers.go:17` | EventType, Severity, Details, RemediationHint, SourcePhase |
| `panestream.SignalEvent` | `signalcenter.go:36` | SessionKey, State |

**No struct carries the operator's full tuple** (module, class/function, code, reason, severity, cycle, phase, attempt). Closest: `dispatchevents.Event` (no module/origin/code/attempt), `llmcalls.Record` (no severity/module), `phasetiming.Entry` (no severity/code/module).

## 5. Error-code vocabularies

| Vocabulary | file:line | Values | Readers |
|---|---|---|---|
| `shiperr.ShipErrorCode` | `shiperr/shiperr.go:69`; `doc.go:29` | **42** codes (EXPLANATION_DOCUMENTATION, SELF_SHA_*, AUDIT_BINDING_* ×12, EGPS_RED_COUNT, CONTROL_PLANE_VIOLATION, COMMIT_GATE_*, TRIVIAL_*, GIT_* ×7, COMMIT_PREFIX_GATE, MANIFEST_GATE, REPO_CONTRACT_*, WORKTREE_RESOLVE, INTEGRITY_TREE_DRIFT, ARGS, GIT_IO, STATE_IO, UNKNOWN) | `router/recovery.go:59,111`, `core.recoverFromShipError`, `phases/ship/verify.go:170`, `failuregrade.go:~77` |
| `shiperr.ShipErrorClass` | `shiperr.go:40` | transient, precondition, integrity, config | ship recovery ladder |
| bridge exit codes | `bridge/exitcodes.go:31,32` | 81 = ExitArtifactTimeout, 85 = ExitUnknownPrompt (86 at `recipe_adapter.go:137`) | `core/failure_learning.go:352`, `core/quota_exhaustion.go:30`, `bridge/autorespond.go:478` |
| `failuregrade.Tier` | `failuregrade.go:25-45` | abort, correct, quarantine, repair (+ abort-reason signatures `:72-76`) | shadow log only |
| `failureadapter.Classification` | `failureadapter.go:22-35` | 11 values | `core.Taxonomy.Consequence`, disposition router |
| `failurelog.Classification` | `failurelog/classifications.go:32-53` | the same 11 **+ 3** (operator-reset, loop-fatal, unknown-classification) — **duplicate vocabulary** | `cmd_loop_maintenance.go:50-55`, `cmd_loop_outcome.go:437` |
| `cyclehealth.Outcome` | `cyclehealth/outcome.go:25-43` | 5 values | `cmd_loop_outcome.go:84`, `soakreport.go:70` |
| `dispatchevents.EventType` | `events.go:36-72` | 7 values + `IsKnownEventType` | `phasewatchdog.go:296` only |
| `phaseio.ErrorContext.Code` | `phaseio.go:25` | free string | recovery prompt |
| `llmcalls.UsageStatus/CauseCode` | `record.go:102,107` | enum / free string | `cmd_tokens.go`, `cycleclassify` |

Three severity vocabularies coexist: `cyclestate` (`error`/`warning`), `dispatchevents` (`INFO/WARN/ERROR`), `observer` (`info/warn/incident` — the schema-1.0 contract in `docs/architecture/observer-severity.md`).

## 6. Module-tagged logging today

**No shared module-tagged logger.** `internal/log.Console` (`console.go:18`, `Diag()` `:34`) has 23 call sites. Two units, measured with `rg` over non-test Go under `internal/` + `cmd/`: **361** hand-written `fmt.Fprint*(os.Stderr, …)` sites, **350** of them opening with a bracketed prefix literal (`[orchestrator]` alone: 284). Modules that write through another writer (`[loop]` via `b.stderr`, `[ship]` via an injected `io.Writer`, `[engine]`/`[runner]` via `log.Diag`) do not appear in that unit, so the table below counts **occurrences of each prefix literal in any writer** — the unit the S5 migration targets.

| Prefix | Occurrences (any writer) | Heaviest file |
|---|---|---|
| `[orchestrator]` | 287 (284 are `Fprint*(os.Stderr, …)` sites) | `core/failure_learning.go` (31) |
| `[loop]` | 132 | `cmd/evolve/cmd_loop*.go` |
| `[ship]` | 130 | `phases/ship/gitops.go` (989 lines) |
| `[bridge]` | 40 · `[chain]` 24 · `[runner]` 23 · `[skills]` 20 · `[gc]` 16 · `[changelog-gen]` 16 · `[ledger]` 14 · `[doctor]` 14 · `[codex]` 13 · `[subagent-run]` 12 · `[contract-gate]` 12 · `[release-pipeline]` 11 · `[marketplace-poll]` 11 · `[agy]` 10 · `[claude-p]` 9 · `[inbox-mover]` 8 · `[commit-prefix-gate]` 8 · `[build-floor]` 8 · ~30 more prefixes at ≤ 7 each · `[engine]` 5 · `[graduated-enforcement SHADOW]` 1 · `[verdict-cache SHADOW]` 1 | | |

Bounded-field helpers that could seed a structured encoder: `log.DiagnosticField` (`log/field.go:13`) and its bridge alias `diagnosticField` (`bridge/attempt_telemetry.go:37`).

## 7. Sizes (decomposition order)

Top non-test files under `go/internal`: `core/orchestrator.go` 1158 · `core/phase_advisor.go` 1144 · `core/failure_learning.go` 1070 · `inboxmover/inboxmover.go` 1006 · `phases/ship/gitops.go` 989 · `config/config.go` 962 · `core/cyclerun.go` 904 · `phases/audit/defect_ledger.go` 881 · `acssuite/acssuite.go` 866 · `phases/audit/ciparity.go` 801 · `bridge/engine.go` 791 · `bridge/autorespond.go` 787 · `router/router.go` 755 · `explanationdocs/explanationdocs.go` 745 · `phases/runner/runner.go` 719.

Top packages by non-test lines (with tests): `core` 23,763 (78,284) · `bridge` 11,233 (41,116) · `phases/ship` 6,106 (21,860) · `phases/audit` 3,292 (13,320) · `policy` 2,985 · `subagent` 2,924 · `router` 2,828 · `deliverable` 2,247 · then `inboxmover` 2,089 · `dashboard` 2,059 · `triagecap` 1,968 · `phases/runner` 1,846. `go/cmd/evolve` is 17,602 non-test lines; its largest file is the composition root `cmd_cycle.go` (1,000).

## 8. Ambiguities

1. `MISSING_SECONDARY:<name>`, `missing_effect`, `unbound_effect` — zero hits in this tree (they live in PR #575, not yet merged).
2. `o.chronicle` is config (`policy.ChronicleConfig`), not a sink; the chronicle is markdown at `cyclerun_chronicle.go:33`.
3. Tests were not run for this survey: `panestream` has 11 SignalCenter test files (~78 KB) for a type with zero production subscribers; `dispatchevents` has 6 test files and no reader test because no Go reader exists.
