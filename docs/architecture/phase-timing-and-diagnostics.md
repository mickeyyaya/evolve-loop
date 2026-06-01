# Phase Timing and Diagnostics

This document outlines the structured observability artifacts produced during Evolve Loop cycles to enable precise latency tracing, diagnostic analysis of phase failures, and self-healing tracking.

## Phase Timing Observability (`phase-timing.json`)

At the conclusion of each cycle run, a central timing trace is recorded at `<workspace>/phase-timing.json`. This file acts as an accumulator, storing a JSON array of timing entries for each phase that executed during the cycle.

### Format and Schema

The JSON payload is an array of objects, where each object contains the following load-bearing fields:

- `phase` (string): The identifier of the phase (e.g. `"scout"`, `"build"`, `"audit"`, `"ship"`).
- `duration_ms` (integer): The elapsed execution time of the phase in milliseconds.
- `verdict` (string): The canonical verdict resolved for the phase (e.g. `"PASS"`, `"WARN"`, `"FAIL"`, `"SKIPPED"`).
- `cost_usd` (number): The financial cost accrued by LLM calls during this phase.
- `attempt_count` (integer): The number of attempts executed during this phase.

### Example Payload

```json
[
  {
    "phase": "scout",
    "duration_ms": 12450,
    "verdict": "PASS",
    "cost_usd": 0.0452,
    "attempt_count": 1
  },
  {
    "phase": "build",
    "duration_ms": 48720,
    "verdict": "PASS",
    "cost_usd": 0.3812,
    "attempt_count": 2
  }
]
```

By collecting the `duration_ms` field for every phase, operator scripts or dashboard subcommands can aggregate telemetry to spot compile bottle-necks or identify high-latency cycles.

---

## Per-Phase Usage Sidecar (`<phase>-usage.json`)

Immediately after each phase successfully records its timing entry in the orchestrator, a structured usage sidecar file is written to `<workspace>/<phase>-usage.json`. This provides granular metrics for cost, duration, attempts, and verdict for each individual phase run.

### Format and Schema

The JSON object contains the following fields:

- `phase` (string): The identifier of the phase (e.g. `"scout"`, `"build"`, `"audit"`, `"ship"`).
- `cost_usd` (number): The financial cost accrued by LLM calls during this phase.
- `duration_ms` (integer): The execution duration of the phase in milliseconds.
- `attempt_count` (integer): The number of attempts executed before the phase finished.
- `verdict` (string): The canonical verdict resolved for the phase (e.g. `"PASS"`, `"WARN"`, `"FAIL"`, `"SKIPPED"`).

### Example Payload (`build-usage.json`)

```json
{
  "phase": "build",
  "cost_usd": 0.42,
  "duration_ms": 48720,
  "attempt_count": 1,
  "verdict": "PASS"
}
```

---

## Phase Failure Diagnostics (`<phase>-failure-diag.json`)

When a mandatory phase exhausts its retries or encounters a non-recoverable error, the orchestrator writes a failure diagnostic file before returning the error and aborting the cycle. This file is saved to `<workspace>/<phase>-failure-diag.json`.

### Format and Schema

The diagnostic file contains key context fields to enable immediate automated parsing or human auditing:

- `phase` (string): The identifier of the failing phase.
- `cycle` (integer): The cycle ID in which the failure occurred.
- `error_message` (string): The non-empty error message returned from the runner.
- `exit_code` (integer): The bridge exit code, such as `81` for `ErrArtifactTimeout` or transient errors like `80`, `85`, or `86`.
- `attempt_count` (integer): The number of attempts executed before the phase aborted.
- `timestamp` (string): The UTC timestamp when the failure occurred.

### Example Payload (`scout-failure-diag.json`)

```json
{
  "phase": "scout",
  "cycle": 173,
  "error_message": "phase scout: bridge: launch exit=81: core: bridge artifact timeout",
  "exit_code": 81,
  "attempt_count": 2,
  "timestamp": "2026-05-31T20:27:45Z"
}
```

The presence of the `failure-diag` file serves as a high-signal indicator for automated pipeline alerts. If the pipeline succeeds fully, no `failure-diag` files are created.

---

## Pause Escalation Report (`<phase>-escalation-report.json`)

When the stop-reviewer issues a `ReviewPause` verdict (e.g., an agent artifact timeout), a detailed investigation report is written to `<workspace>/<phase>-escalation-report.json`. This preserves investigation evidence (such as the recent pane tail, elapsed time, intervals, and attempt count) before the runner returns a hard timeout exit code.

### Format and Schema

The JSON object contains the following fields:

- `phase` (string): The identifier of the paused phase.
- `cycle` (integer): The cycle ID in which the pause occurred.
- `elapsed_s` (integer): The total seconds waited so far.
- `interval_s` (integer): The review interval duration.
- `attempt` (integer): The review attempt index when the pause was triggered.
- `stop_kind` (string): The classification of the stop condition (e.g., `"artifact_timeout"`).
- `action` (string): The reviewer's verdict action (e.g., `"pause"`).
- `reason` (string): The human-readable justification produced by the reviewer.
- `final_pane` (string): The last 40 lines of pane scrollback/stdout tail.

### Example Payload (`scout-escalation-report.json`)

```json
{
  "phase": "scout",
  "cycle": 189,
  "elapsed_s": 900,
  "interval_s": 300,
  "attempt": 3,
  "stop_kind": "artifact_timeout",
  "action": "pause",
  "reason": "no output during the last 300s interval — stalled; pause for investigation",
  "final_pane": "Scouting codebase...\nDeliberating on next steps...\n[idle for 300s]"
}
```

---

## Structured Self-Healing Ledger Entries

To preserve the structured audit trail of the cycle's execution, the orchestrator appends specialized entries to the cycle ledger at `.evolve/ledger.jsonl`.

### Relaunch Signal (`kind=phase_retry`)

When a transient bridge failure (exit code 80, 85, or 86) or an artifact timeout (exit code 81) triggers a self-heal retry, a `kind=phase_retry` entry is appended to the ledger. This signals that the phase encountered a recoverable issue and was relaunched.

- **ExitCode**: Records the precise bridge exit code (e.g. `80`, `81`, `85`, `86`) returned by the failing attempt.

### Recovery Signal (`kind=backfill`)

If a mandatory phase exhausts its retry budget due to `ErrArtifactTimeout` but successfully reconstructs the missing report/artifact from the terminal scrollback (via `backfill.TryExtract` with `EVOLVE_BACKFILL_ENABLED=1`), the orchestrator writes a `kind=backfill` entry to the ledger.

- **Role** (string): Set to the name of the backfilled phase (e.g. `"scout"`).
- **ExitCode** (integer): Set to `81`, the original timeout exit code that initiated the backfill.

---

## Stop-Review Ledger Trail (`kind=stop_review`)

When the tmux driver's stop-review checkpoint fires (the artifact wait interval elapsed), the reviewer adjudicates the evidence and either extends or pauses the phase. Both decisions are now appended to the ledger as `kind=stop_review` entries:

- **Role** (string): The phase agent name (e.g. `"build"`).
- **Action** (string): The reviewer's decision — `"extend"` (continue waiting) or `"pause"` (stop for investigation).
- **Message** (string): The human-readable justification the reviewer produced.

`extend` events are healthy (the reviewer judged the agent still working) and produce NO cycle-health anomaly. `pause` events are anomalous (stall detected) and surface as a `SeverityWarn` on the `self_heal_events` signal.

### Per-Phase Latency Ceiling Overrides

`checkPhaseLatency` (signal 12) reads a global ceiling `EVOLVE_PHASE_LATENCY_CEILING_S` (default 900 s, 15 min) for all phases. Individual phases can override this ceiling with a per-phase env-var:

```
EVOLVE_<UPPER_PHASE>_LATENCY_CEILING_S
```

Phase name normalization: `strings.ToUpper` + `"-"` → `"_"`.  Examples:

| Phase | Override env-var |
|-------|-----------------|
| `scout` | `EVOLVE_SCOUT_LATENCY_CEILING_S` |
| `build` | `EVOLVE_BUILD_LATENCY_CEILING_S` |
| `build-planner` | `EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S` |

When a per-phase override is absent or invalid (non-numeric, ≤ 0), the global ceiling applies.

---

## Cycle Health Self-Heal Signal (`self_heal_events`)

To track recovery events during a cycle, the integrity checker (`evolve cycle-health`) includes the `self_heal_events` signal (signal 13).

This signal automatically scans `ledger.jsonl` for any entries with `kind=phase_retry`, `kind=backfill`, or `kind=stop_review` (with `action=pause`) for the current cycle. For each event found, it generates a `SeverityWarn` anomaly containing the name of the retried/backfilled/paused phase, alerting operators that a self-heal recovery occurred during the cycle.

---

## References

- For details on how artifacts are extracted and recovered from scrollback logs, see [Artifact Backfill](artifact-backfill.md).
- To understand which pipeline aborts are caught by self-healing and transient retries, see [Self-Healing Gaps](self-healing-gaps.md).
