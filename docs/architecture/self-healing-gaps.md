# Self-Healing Gaps & Hardening (cycle-164 / multi-CLI run)

Status: living analysis. Source: the multi-CLI validation run (cycles 154-164) + a
full trace of the recovery control-flow in `go/internal/core/orchestrator.go`.

## Why this exists

The pipeline already has recovery machinery — audit-FAIL → `retro` → failure-adapter
(PROCEED/RETRY/BLOCK); ship-error → a 4-link recovery chain (`router/recovery.go`) →
`debugger`/re-audit/retry-ship. But a `RunCycle` hard error **stops the whole batch**
(this is why a single phase failure aborted runs 154-162). So every place RunCycle
returns an error instead of routing to recovery is a potential pipeline-blocker.

Research alignment (2025 self-healing / self-debugging agent literature): the resilient
pattern is *analyze context → decide retry / reroute / escalate*. The workflow adapts
in-flight and does not abort. The agent feeds corrective feedback back to condition
the retry. Outcomes update the knowledge base (lessons). The caveat: an imprecise
evaluator/retry can reinforce bad patterns and **burn tokens**. Retries must therefore be
narrowly scoped to genuinely transient failures, never a real FAIL of a model.

## Ranked hard-abort gaps (where a blocker escapes self-healing)

| # | Site (orchestrator.go unless noted) | Trigger | Recoverable? | Fix posture |
|---|---|---|---|---|
| 1 | `attempt` loop `return ... "phase %s: %w"` | any non-ArtifactTimeout bridge error on a mandatory phase | yes (transient class) | **DONE** — cycle-173: transient bridge failures (exits 80/85/86) are classified and retried up to phaseMaxAttempts |
| 9 | `retro.go` bridge-fail returns error | retro's OWN bridge dies | yes | **DONE** — return FAIL verdict + nil error → routes through `decideAfterRetro` |
| 5 | `return ... "non-canonical verdict"` | runner returns verdict ∉ {PASS,FAIL,WARN,SKIPPED} (parse blip) | yes | **DONE** — cycle-173: non-canonical verdicts are classified and retried up to phaseMaxAttempts |
| 3 | `router/recovery.go` integrity-block | integrity-class ShipError (for example, INTEGRITY_TREE_DRIFT false positive) | sometimes | route through `debugger` deep-dive before BLOCK (deferred/by-design) |
| 2 | `phaseMaxAttempts=2`, no backoff | repeated ArtifactTimeout | partly | **DONE** — cycle-180: configurable exponential backoff (`EVOLVE_RETRY_BACKOFF_BASE_S`, default 5s) |
| 4 | `maxRecoveryDepth=2` then abort | persistent ship blocker | by design | keep cap; escalate to operator notice (deferred/by-design) |
| 6 | tree-diff guard / `recoverBuildLeak` false | unrecoverable leak | correctness guard | keep (bugs #5/#6 already hardened recoverBuildLeak) |
| 7 | reviewer reject (noop default) | future reviewer trips | n/a today | add retry budget before any real reviewer ships (deferred/by-design) |
| 8 | state-machine transition error | unknown verdict edge | programmer error | keep (guards a bug, not runtime) (deferred/by-design) |
| 10 | Artifact timeout exhaustion | `ErrArtifactTimeout` occurs on the last try, and the work is lost | yes | **DONE** — cycle-171/179: [artifact-backfill](artifact-backfill.md) default-on, extracts artifact from raw stdout to avoid hard cycle aborts |
| 11 | Retries forensically invisible | difficulty auditing retries | yes | **DONE** — cycle-171: `attempt_count` logged to [phase timing](phase-timing-and-diagnostics.md) and failure diagnostics |
| 12 | Latency anomaly detection | phase runs excessively slow without crashing | yes | **DONE** — cycle-180: signal 12 `phase_latency` in `cyclehealth.go` raises warning on slow phases |
| 14 | `backfillArtifactPath` / `phaseHeaders` | retro & build-planner backfill coverage incomplete | yes | **DONE** — cycle-187: add retro/build-planner to backfill phaseHeaders and align retro runner polling path |
| 16 | StopReviewer Pause Escalation | pause verdict triggers hard timeout without investigation evidence | yes | **DONE** — cycle-189: Distinct pause semantics write a detailed `<workspace>/<phase>-escalation-report.json` to preserve investigation evidence before pausing |

## Principle for fixes

A failure in the **failure-handler** (retro, debugger) must never be fatal (GAP 9).
A **transient** infra/bridge failure is recommended to retry or reroute, bounded (GAP 1/5). A
**genuine** model FAIL or correctness-guard breach must still stop. Recovery is for
transient/infra faults. It does not mask real failures (token-optimization + the
"imprecise-evaluator" caveat).

## Completed as of cycle-189

Over successive self-evolution cycles, we have systematically addressed the primary gaps identified in this living document:

1. **Transient Bridge Retries (GAP 1 & 5)**:
   - Implemented in cycle-173. Transient bridge errors (exit codes 80, 85, 86) and non-canonical phase verdicts now trigger bounded retries. They do not abort at once.

2. **Artifact Backfill (GAP 10)**:
   - Added robust fallback in cycle-171 (refined to default-on in cycle-179). If a write-tool artifact timeout (`ErrArtifactTimeout`) occurs on the last try, the orchestrator tries to extract the phase artifact directly from the clean stdout file. It writes a `WARN` verdict ledger entry and lets the cycle continue. Detailed in [artifact-backfill.md](artifact-backfill.md).

3. **Configurable Exponential Backoff (GAP 2)**:
   - Implemented in cycle-180. The retry loop now sleep-paces relaunch tries with the `EVOLVE_RETRY_BACKOFF_BASE_S` env-var (defaulting to 5 seconds). The sleep duration scales as `base * 2^(attempt-2)` clamped to a ceiling of `max(base, 30)` seconds. This prevents consecutive collision errors under resource contention.

4. **Forensic Try Tracking (GAP 11)**:
   - Unified timing, latency and try counters. The `attempt_count` field is explicitly recorded in `phase-timing.json` and in transient failure diagnostics. This gives transparent visibility into self-heal events. Detailed in [phase-timing-and-diagnostics.md](phase-timing-and-diagnostics.md).

5. **Phase Latency Monitoring (GAP 12)**:
   - Added signal 12 `phase_latency` to `go/internal/cyclehealth/cyclehealth.go` in cycle-180. It reads `phase-timing.json` and evaluates each phase's duration against `EVOLVE_PHASE_LATENCY_CEILING_S` (default 900s / 15 minutes), raising warning anomalies when a phase executes abnormally slowly.

6. **Self-Heal Event Signal (GAP 13)**:
   - Added signal 13 `self_heal_events` to `go/internal/cyclehealth/cyclehealth.go` in cycle-186. It reads `ledger.jsonl` and filters for any phase relaunch (`phase_retry`) or recovery (`backfill`) events tagging the current cycle, surfacing them as `WARN` severity anomalies. Detailed in [phase-timing-and-diagnostics.md](phase-timing-and-diagnostics.md).

7. **Progress Disambiguation (ADR-0026 Stage 1 #4)**:
   - Implemented in cycle-186. The progressed detection of the tmux REPL driver now uses `PaneHasSubstantiveChange` in `stopreview.go`. It strips Unicode and ASCII spinners, deliberating time indicators and token counters before it compares snapshots. This prevents animated spinners from fooling the stall detector. Detailed in [ADR-0026](adr/0026-self-healing-review-layer.md).

8. **Retro & Build-Planner Backfill (GAP 14)**:
   - Implemented in cycle-187 to extend backfill recovery coverage to the retrospective and build-planner phases, aligning the retro runner to poll the correct file path. Detailed in [artifact-backfill.md](artifact-backfill.md).

9. **Stop-Review Ledger Trail (GAP 15 / ADR-0026 Stage 1 #5)**:
   - Implemented in cycle-188. Stop-review verdicts (extend AND pause) are now emitted to the ledger as `kind=stop_review` entries carrying the `action` (extend/pause) and `message` (reviewer justification). `Deps.OnStopReview` callback in `engine.go`; driver calls it nil-safely; orchestrator wires it to `ledger.Append`; `checkSelfHealEvents` in `cyclehealth.go` flags `action=pause` as `SeverityWarn`. ADR-0026 Stage 1 #5 closed.

10. **Distinct Pause Semantics / Escalation Report (GAP 16 / ADR-0026 Stage 1 #3)**:
    - Implemented in cycle-189. When a stop-reviewer issues a `ReviewPause` verdict, the bridge writes a `<workspace>/<phase>-escalation-report.json` before it returns `ExitArtifactTimeout`. The file holds detailed investigation evidence (phase, cycle, elapsed, `attempt`, stop kind, final pane tail and verdict justification). This closes ADR-0026 Stage 1 #3.

## Multi-CLI note

The per-phase CLI resolves in this order: `EVOLVE_<AGENT>_CLI` > `EVOLVE_CLI` > profile.cli >
`claude-tmux`. A `cli_fallback` chain runs on trigger exits `[80 81 124 127]`
(`runner/cli_chain.go`). A non-Claude CLI works on any phase when three conditions hold:

- (a) its manifest/flags are correct (bugs at 154);
- (b) its writes are confined or recovered (bugs #4/#5/#6 — `recoverBuildLeak`);
- (c) the fallback chain is populated.
