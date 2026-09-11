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
- `context_fill_ratio` (number, optional): How full the model's context window got on the terminal attempt — `contextfill.FillRatio` over `contextfill.WindowSizeForTier(resolved_model)`. **Omitted** when the tier was not resolvable, so an absent key means "unknown", never a genuine `0.0`. Not clamped at `1.0`: a phase that overran its window stays distinguishable from one that just fit.
- `context_window_hot` (boolean, optional): `true` when `context_fill_ratio` reached the inclusive `contextfill.HotThreshold` (0.85). Omitted when false or unknown.

Legacy logs written before a field existed parse to its zero value rather than erroring — every addition to this schema is additive and degrades to "absent". `Rollup()` summarises the hot phases at cycle level as `hot_phase_count` / `hot_phases`. Design detail: `docs/architecture/context-window-control.md` § Telemetry wiring.

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
    "attempt_count": 2,
    "resolved_model": "deep",
    "context_fill_ratio": 0.95,
    "context_window_hot": true
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
- `delivery_failure` (string): The classified submission failure (for example, `prompt submit_wedged (resends=3)`); empty for generic artifact timeouts and non-timeout failures.
- `exit_code` (integer): The bridge exit code, such as `81` for `ErrArtifactTimeout` or transient errors like `80`, `85`, or `86`.
- `attempt_count` (integer): The number of attempts executed before the phase aborted.
- `timestamp` (string): The UTC timestamp when the failure occurred.

### Example Payload (`scout-failure-diag.json`)

```json
{
  "phase": "scout",
  "cycle": 173,
  "error_message": "phase scout: bridge: launch exit=81: core: bridge artifact timeout",
  "delivery_failure": "",
  "exit_code": 81,
  "attempt_count": 2,
  "timestamp": "2026-05-31T20:27:45Z"
}
```

The presence of the `failure-diag` file serves as a high-signal indicator for automated pipeline alerts. If the pipeline succeeds fully, no `failure-diag` files are created.

### Tmux artifact-timeout context

An interactive model wait that exits with code 81 emits one bounded marker as
its final timeout diagnostic. `Engine.Launch` copies that marker into
`error_message`, so operators and failure-learning code see the same evidence:

```text
artifact-timeout: cause=completion_detector_error reason="reviewer paused" \
  phase=build cycle=73 driver=claude-tmux artifact="build-report.md" \
  waited=1200s interval=1200s extends_used=0 max_extends=6 \
  last_review=pause liveness=idle progressed=false busy=false transient=false \
  detector_error="relocate ...: not a directory"
```

`cause` is a closed, host-authored classification. Read it before interpreting
the free-form evidence:

| Cause | Meaning | First action |
|---|---|---|
| `context_cancelled` | The wait coordinator observed cancellation and its detached final completion check did not confirm a deliverable. | Find the orchestrator timeout or shutdown source. |
| `completion_detector_error` | The terminal completion check failed locally. | Read `detector_error`; repair the path, tmux, or git evidence operation it names. |
| `submit_wedged` | Bounded submission verification proved the prompt or nudge stayed in the input line. | Recreate or relaunch the REPL session. |
| `transient_upstream` | The launched family's manifest recognized a temporary provider failure in the agent-stripped pane. | Retry after provider recovery; do not raise the artifact budget first. |
| `review_stop` | The reviewer or fatal-pane gate selected a terminal stop. | Read `reason` and the escalation report. |
| `review_pause` | The reviewer selected an investigation pause. | Compare liveness and extension counters, then read the escalation report. |
| `incomplete` | No known terminal signal explained the missing completion. | Treat it as an unclassified bridge defect and preserve the logs. |

`reason` and `detector_error` are independently bounded and escaped. Long
evidence preserves both its operation prefix and leaf error suffix. Phase,
driver, cycle, and artifact identify the failed attempt without exposing full
workspace paths. A detector error from an earlier poll remains as secondary
evidence, while `cause=completion_detector_error` is used only when the final
detector observation itself failed. A completion confirmed after an earlier
fault emits no timeout marker.

The authoritative line begins with the exact `[bridge] artifact-timeout:`
prefix and is the final matching line in stderr. Reviewer reasons are bounded,
quoted, and stripped of terminal and Unicode formatting controls before console
logging. Consequently, inline text or a newline-prefixed fake marker in
free-form evidence cannot displace the host-written terminal marker.

The complete exit-81 summary is capped at 1,024 Unicode code points. Other
bridge errors retain their existing shorter bound. Exit code 81 and
`ErrArtifactTimeout` semantics are unchanged.

### Verified Submission Delivery Failures — Issue / Gap / Solution

- **Issue:** A tmux prompt or nudge could remain parked after all three bounded Enter re-sends. Submit verification classified the pane as `submit_wedged`, but the driver still consumed the normal artifact-wait budget before returning exit 81.
- **Gap:** Both tmux consumer sites recorded the classification without acting on it, and the terminal failure diagnostic exposed only a flat `error_message`. Generic silence and a verified delivery failure therefore looked identical to automation.
- **Solution:** The prompt site now short-circuits through the existing `ExitArtifactTimeout` marker, while the nudge site carries its classified reason into that same marker. A typed, host-owned `cause=submit_wedged` field authorizes extraction of the escaped reason into `delivery_failure`; reviewer prose cannot forge that classification. Historical markers without `cause` retain the legacy reason fallback. Generic silence and unrelated failures leave the field empty. The existing resend cap and one-relaunch dispatcher contract are unchanged.

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

If a mandatory phase exhausts its retry budget due to `ErrArtifactTimeout` but successfully reconstructs the missing report/artifact from the terminal scrollback (via `backfill.TryExtract` with `workflow.backfill_enabled=true`), the orchestrator writes a `kind=backfill` entry to the ledger.

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

When a per-phase override is absent or invalid (non-numeric, ≤ 0), the global ceiling applies.

---

## Cycle Health Self-Heal Signal (`self_heal_events`)

To track recovery events during a cycle, the integrity checker (`evolve cycle-health`) includes the `self_heal_events` signal (signal 13).

This signal automatically scans `ledger.jsonl` for any entries with `kind=phase_retry`, `kind=backfill`, or `kind=stop_review` (with `action=pause`) for the current cycle. For each event found, it generates a `SeverityWarn` anomaly containing the name of the retried/backfilled/paused phase, alerting operators that a self-heal recovery occurred during the cycle.

---

## References

- For details on how artifacts are extracted and recovered from scrollback logs, see [Artifact Backfill](artifact-backfill.md).
- To understand which pipeline aborts are caught by self-healing and transient retries, see [Self-Healing Gaps](self-healing-gaps.md).
