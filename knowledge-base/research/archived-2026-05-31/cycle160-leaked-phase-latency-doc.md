# Phase Latency and Error Recovery

This document details the latency tracking, structured failure diagnostics, and self-healing backfill mechanisms introduced in Evolve Loop (v14.1+).

---

## 1. Per-Phase Latency Tracking (`phase-timing.json`)

To facilitate latency diagnostics without parsing multi-megabyte stdout logs, the orchestrator outputs a structured `phase-timing.json` file to the cycle's workspace directory at the end of every cycle execution that ran at least one phase.

### Path
```
<project_root>/.evolve/runs/cycle-<N>/phase-timing.json
```

### Schema & Format
The file contains a JSON array of objects representing each phase executed during the cycle, sequentially ordered:

```json
[
  {
    "phase": "scout",
    "duration_ms": 12300,
    "verdict": "PASS",
    "cost_usd": 0.05
  },
  {
    "phase": "build",
    "duration_ms": 45200,
    "verdict": "PASS",
    "cost_usd": 0.12
  }
]
```

### Fields
- `phase`: (string) Canonical name of the phase (e.g., `scout`, `tdd`, `build`, `audit`).
- `duration_ms`: (integer) The wall-clock execution duration of the phase in milliseconds, as measured around the runner's execution loop.
- `verdict`: (string) Canonical phase outcome verdict (`PASS`, `FAIL`, `WARN`, `SKIPPED`).
- `cost_usd`: (float) The exact cost accrued by the phase execution.

---

## 2. Structured Failure Diagnostics (`<phase>-failure-diag.json`)

When a mandatory phase (such as `scout`, `tdd`, `build`, `audit`) encounters a non-recoverable error or exhausts its maximum relaunch attempts (resulting in a cycle abort), the orchestrator writes a structured diagnostic payload to the workspace before returning the error.

### Path
```
<project_root>/.evolve/runs/cycle-<N>/<phase>-failure-diag.json
```

### Schema & Format
```json
{
  "phase": "build",
  "cycle": 156,
  "error_message": "build compiler exploded",
  "exit_code": 1,
  "attempt_count": 2,
  "verdict": "FAIL",
  "diagnostics": [],
  "timestamp": "2026-05-31T17:38:20Z"
}
```

### Fields
- `phase`: (string) Canonical phase name.
- `cycle`: (integer) The active cycle number.
- `error_message`: (string) The exact error string returned by the failed execution.
- `exit_code`: (integer) The process exit status (81 for `ErrArtifactTimeout`, extracted from underlying `exec.ExitError`, parsed from process logs, or default 1).
- `attempt_count`: (integer) Total number of relaunch attempts before aborting (typically 1 or 2).
- `verdict`: (string) Fixed to `"FAIL"`.
- `diagnostics`: (array) Extracted diagnostics (if any) or empty array.
- `timestamp`: (string) UTC ISO 8601 timestamp at the time of write.

---

## 3. Relaunch Visibility (`phase_retry` Ledger Entry)

When a phase encounters a recoverable bridge artifact timeout (`ErrArtifactTimeout`), the orchestrator triggers an automatic relaunch (self-heal). To make these retries visible rather than silently absorbed, the orchestrator appends a `kind="phase_retry"` entry to the ledger before executing the retry.

### Path
```
<project_root>/.evolve/ledger.jsonl
```

### Shape
A `phase_retry` entry carries the following fields:
```json
{
  "ts": "2026-05-31T17:38:20Z",
  "cycle": 156,
  "role": "scout",
  "kind": "phase_retry",
  "exit_code": 81
}
```

---

## 4. Artifact Backfill (`EVOLVE_BACKFILL_ENABLED`)

In some cycles, a phase execution successfully completes and prints its complete markdown artifact to stdout, but the `Write` tool call to write the final file times out (throwing `ErrArtifactTimeout`). When relaunch attempts are exhausted, the orchestrator can attempt to salvage the output using the backfill mechanism.

### Configuration Gate
Gated behind the environment variable `EVOLVE_BACKFILL_ENABLED`:
- `EVOLVE_BACKFILL_ENABLED=1`: Enable recovery.
- `EVOLVE_BACKFILL_ENABLED=0` (default): Disable recovery; abort immediately.

### Extraction Heuristic
When enabled, the backfill engine scans `<workspace>/<phase>-stdout.clean.txt` for the last occurrence of the phase's known markdown header:
- `scout` -> `"# Scout Report"`
- `build` -> `"# Build Report"`
- `audit` -> `"# Audit Report"`
- `intent` -> `"# Intent"`
- `triage` -> `"# Triage"`
- `tdd` -> `"# TDD"`

If found, it extracts all content from that header to the End-of-File (trimmed). If the extracted content is at least **200 characters** in length:
1. It writes the extracted content to the phase's expected artifact path (e.g. `scout-report.md`).
2. The orchestrator logs the recovery, treats the phase as successfully completed with `VerdictWARN` (rather than aborting), and allows the cycle to continue.
