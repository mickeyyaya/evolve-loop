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

### Example Payload

```json
[
  {
    "phase": "scout",
    "duration_ms": 12450,
    "verdict": "PASS",
    "cost_usd": 0.0452
  },
  {
    "phase": "build",
    "duration_ms": 48720,
    "verdict": "PASS",
    "cost_usd": 0.3812
  }
]
```

By collecting the `duration_ms` field for every phase, operator scripts or dashboard subcommands can aggregate telemetry to spot compile bottle-necks or identify high-latency cycles.

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

## References

- For details on how artifacts are extracted and recovered from scrollback logs, see [Artifact Backfill](artifact-backfill.md).
- To understand which pipeline aborts are caught by self-healing and transient retries, see [Self-Healing Gaps](self-healing-gaps.md).
