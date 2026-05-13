# Per-Phase Observer Service

> The observer is spawned at every phase start, monitors the subagent through its entire phase, and notifies the orchestrator at the end (and during, on incident). Implements layers 1-3 (collect → digest → translate) of the operator-specified observability architecture; layer 4 (execute) is the orchestrator itself. Sibling to `phase-watchdog.sh`; coexists in v1, subsumes in v2.

## Why

Pre-observer, the pipeline produced rich raw signals (NDJSON stream-json events, per-phase sidecars, hash-chained ledger) but no in-process analysis. The orchestrator had to do all reasoning itself from raw artifacts — or, more commonly, the operator had to grep across files to answer "what went wrong in this phase." The observer is the structured intermediate layer that:

1. Watches the subagent's stream in real time
2. Maintains running L2 state (event counts, tool histograms, cumulative cost, cache stats)
3. Runs 5 deterministic detection rules on every poll
4. Emits structured observations (INFO heartbeat / WARN anomaly / INCIDENT accident)
5. Writes a per-phase JSON report at exit for the orchestrator to consume

The minimum bar: detect a phase that is **stuck** (no output for N seconds) or **looping** (repeating identical tool calls). Both fire as INCIDENT observations with a suggested termination action.

## Architecture

```
ORCHESTRATOR (existing LLM)
       ▲ reads
       │
{phase}-observer-report.json   ← phase-end summary
{phase}-observer-events.ndjson ← live stream
       ▲ writes
       │
PHASE-OBSERVER PROCESS
  ├─ L3: 5 detection rules (rule_stuck_no_output, rule_infinite_loop,
  │      rule_error_spike, rule_cost_anomaly, rule_throttled)
  ├─ L2: In-memory state (event counts, tool histogram, cost, cache, ratelimits)
  └─ L1: Byte-offset poll of {phase}-stdout.log; parse NDJSON; update L2
```

One observer per subagent invocation. Bounded lifetime: spawned by `run-cycle.sh` immediately before the subagent, signaled with SIGUSR1 when subagent exits, auto-shuts-down after `EVOLVE_OBSERVER_EOF_GRACE_S` quiet polls if no SIGUSR1 arrives.

## Unified envelope

Every event the observer emits — heartbeats, rule fires, phase-end markers — uses the same envelope:

```json
{
  "id": "obs_...",
  "schema_version": "1.0",
  "ts": "2026-05-13T14:00:00.123Z",
  "trace_id": "cycle-38-build-1715608800",
  "source": {
    "component": "phase-observer",
    "cycle": 38,
    "phase": "build",
    "agent": "builder",
    "observer_pid": 12345
  },
  "type": "observation.heartbeat | observation.warn | observation.incident | observation.phase_end",
  "severity": "INFO | WARN | INCIDENT",
  "data": {
    "metric_type": "stuck | infinite_loop | cost_anomaly | error_spike | throttled | heartbeat | phase_end",
    "evidence": { ... },
    "suggested_action": { ... }
  }
}
```

See `docs/architecture/observer-severity.md` for severity semantics.

## Detection rules (v1)

| Rule | Trigger | Severity | Suggested action_type |
|---|---|---|---|
| `stuck_no_output` | `now - last_event_ts >= EVOLVE_OBSERVER_STALL_S` (default 240s) | INCIDENT | `terminate_phase` (reason `STUCK_NO_OUTPUT`) |
| `infinite_loop` | Same `(tool_name + sha256(args))` repeated >= `EVOLVE_OBSERVER_LOOP_N` (default 6) in `EVOLVE_OBSERVER_LOOP_WINDOW_S` (default 120s) | INCIDENT | `terminate_phase` (reason `INFINITE_LOOP`) |
| `error_spike` | `is_error=true / total_tool_results > EVOLVE_OBSERVER_ERROR_RATE` (default 0.3); requires >= 5 sample | WARN | `continue` (reason `ERROR_SPIKE`) |
| `cost_anomaly` | `(current_cost - baseline_mean) / baseline_stddev >= EVOLVE_OBSERVER_COST_SIGMA` (default 2); requires baseline | WARN | `continue` (reason `COST_ANOMALY`) — not wired in v1; needs rollup baselines |
| `throttled` | `rate_limit_event` count in 60s window >= `EVOLVE_OBSERVER_THROTTLE_N` (default 3) | WARN | `continue` (reason `API_THROTTLED`) |

Future haiku-augmented rule (designed for, not built): `semantic_loop` — agent repeating semantically equivalent but textually distinct tool calls.

## Phase-end report

Written atomically just before the observer exits. Schema:

```json
{
  "schema_version": "1.0",
  "cycle": 38,
  "phase": "build",
  "agent": "builder",
  "observer": {
    "started_at": "...", "ended_at": "...", "phase_duration_ms": 453000,
    "exit_reason": "subagent_exited_normally | observer_eof_grace"
  },
  "summary": {
    "verdict": "NORMAL | DEGRADED | INCIDENT",
    "event_count": 152,
    "tool_call_count": 47,
    "error_count": 1,
    "rate_limit_events": 1,
    "cumulative_cost_usd": 1.84,
    "cache_hit_rate": 0.96
  },
  "incidents": [ ... full INCIDENT envelopes ... ],
  "tool_call_histogram": { "Read": {"count": 23, "errors": 0}, ... }
}
```

The orchestrator's directive is one line: "after each phase, read `{phase}-observer-report.json`; if `verdict` is `INCIDENT` or `incidents[]` non-empty, treat the recommended `suggested_action` as a decision input."

## Spawning & lifecycle

Spawned by `run-cycle.sh` immediately before the subagent — same pattern as `phase-watchdog.sh`:

```bash
bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/phase-observer.sh" \
    "$workspace" "$RUN_PGID" "$cycle" "$phase" "$agent" "$cycle_state_file" &
OBSERVER_PID=$!

# ...spawn subagent...

# On subagent exit:
kill -USR1 "$OBSERVER_PID" 2>/dev/null
wait "$OBSERVER_PID" 2>/dev/null
```

Auto-shutdown via EOF-grace (default 10s of no log growth + at least one event seen). SIGUSR1 forces immediate shutdown. SIGTERM is allowed; observer exits gracefully without writing the final report.

## Watchdog migration ladder (v9.4 → v9.7+)

| Version | phase-watchdog | phase-observer | Operator action |
|---|---|---|---|
| v9.4.0 | runs unchanged; SIGTERMs on stall | runs alongside; emits INCIDENT but does NOT kill | None (shipped) |
| v9.5.0 (this) | runs UNLESS `EVOLVE_OBSERVER_ENFORCE=1` | gains `--enforce` flag; with EVOLVE_OBSERVER_ENFORCE=1 spawned at cycle-scope (replacing watchdog) AND at phase-scope, both with kill authority | Opt in via `EVOLVE_OBSERVER_ENFORCE=1` to test |
| v9.6.0 (candidate) | not spawned by run-cycle.sh | default `EVOLVE_OBSERVER_ENFORCE=1`; watchdog kept on disk but unused | None — observer is now default |
| v9.7.0 (candidate) | DELETED | sole stall-protection mechanism | Remove `EVOLVE_INACTIVITY_*` env vars from any cron / CI configs |

### v9.5.0 `--enforce` flag semantics

When operator sets `EVOLVE_OBSERVER_ENFORCE=1`:
- `run-cycle.sh` spawns `phase-observer.sh --enforce --scope=cycle` instead of `phase-watchdog.sh` (cycle-level safety net is now the observer)
- `subagent-run.sh` passes `--enforce` to its existing phase-level observer spawn
- On `INCIDENT(stuck)` or `INCIDENT(infinite_loop)`: observer writes `stall-progress.json`, calls `cycle-state.sh checkpoint stall-inactivity`, sends SIGTERM to target PGID, sleeps `EVOLVE_OBSERVER_GRACE_S` (default 10s), sends SIGKILL to survivors
- WARN-severity rules (cost_anomaly, error_spike, throttled) never trigger kill — even in `--enforce` mode

When `EVOLVE_OBSERVER_ENFORCE=0` (default in v9.5): identical to v9.4.0 advisory-only behavior. Observer emits observations; watchdog kills.

### Cycle-scope vs phase-scope ruleset (v9.5+)

| Rule | Phase-scope | Cycle-scope |
|---|---|---|
| `stuck_no_output` | ✓ | ✓ |
| `infinite_loop` | ✓ | ✗ (orchestrator doesn't loop on individual tool calls) |
| `error_spike` | ✓ | ✗ |
| `cost_anomaly` | ✓ | ✗ |
| `throttled` | ✓ | ✗ |

Cycle-scope observer is essentially "the watchdog with the unified envelope format" — same liveness signal source, same kill authority, same scope.

### Legacy env-var bridge (v9.5)

These watchdog env vars still work but emit `[phase-observer] DEPRECATED` warning to stderr. Removal target: v9.7+.

| Legacy | Modern |
|---|---|
| `EVOLVE_INACTIVITY_THRESHOLD_S` | `EVOLVE_OBSERVER_STALL_S` |
| `EVOLVE_INACTIVITY_POLL_S` | `EVOLVE_OBSERVER_POLL_S` |
| `EVOLVE_INACTIVITY_GRACE_S` | `EVOLVE_OBSERVER_GRACE_S` |
| `EVOLVE_INACTIVITY_WARN_PCT` | `EVOLVE_OBSERVER_WARN_PCT` |
| `EVOLVE_INACTIVITY_DISABLE=1` | `EVOLVE_OBSERVER_ENABLED=0` |

## Tunables (env vars)

| Var | Default | Purpose |
|---|---|---|
| `EVOLVE_OBSERVER_ENABLED` | `0` (v1) | Master switch in `run-cycle.sh`. Set to `1` to spawn observers. |
| `EVOLVE_OBSERVER_POLL_S` | `5` | Poll interval (seconds) |
| `EVOLVE_OBSERVER_STALL_S` | `240` | Stuck threshold |
| `EVOLVE_OBSERVER_LOOP_N` | `6` | Infinite-loop repeat count |
| `EVOLVE_OBSERVER_LOOP_WINDOW_S` | `120` | Infinite-loop window |
| `EVOLVE_OBSERVER_ERROR_RATE` | `0.3` | Error-spike threshold (decimal 0..1) |
| `EVOLVE_OBSERVER_COST_SIGMA` | `2` | Cost-anomaly z-score threshold |
| `EVOLVE_OBSERVER_THROTTLE_N` | `3` | Rate-limit events per 60s threshold |
| `EVOLVE_OBSERVER_EOF_GRACE_S` | `10` | Auto-shutdown grace period after no growth |
| `EVOLVE_OBSERVER_HEARTBEAT_EVERY` | `12` | Polls between heartbeat observations |

## Operator runbook

**Watch a live phase**:
```bash
tail -F .evolve/runs/cycle-38/builder-observer-events.ndjson | jq -c '{ts, severity, "metric": .data.metric_type}'
```

**Inspect a completed phase**:
```bash
jq '.summary, {incidents: (.incidents | length)}' .evolve/runs/cycle-38/builder-observer-report.json
```

**Tune for a noisy phase** (e.g., long Memo phases):
```bash
EVOLVE_OBSERVER_STALL_S=600 EVOLVE_OBSERVER_ENABLED=1 \
    bash scripts/dispatch/evolve-loop-dispatch.sh --resume
```

**Disable temporarily**:
```bash
EVOLVE_OBSERVER_ENABLED=0   # observer not spawned; watchdog still active
```

## Verification & tests

- Unit tests for rules: `scripts/tests/phase-observer-test.sh` — 6 fixture scenarios, 20 assertions
- Live smoke: run any cycle with `EVOLVE_OBSERVER_ENABLED=1`; tail the events file
- Integration: orchestrator-report.md should reference `observer-report.json` consumption after the v1 rollout

## Out of scope (v1)

- Haiku-augmented semantic-loop detection
- Cross-cycle anomaly aggregation (week-over-week trends)
- Observer subsuming watchdog's kill-sequence
- Auto-execution of suggested actions (deliberately deferred — observer informs; orchestrator acts)
- Integration with external alerting (Slack, PagerDuty)
