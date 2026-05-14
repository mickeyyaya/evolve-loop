# Abnormal-Event Capture Pipeline

> **Status:** Phase B complete (cycle 46). Detectors wired in `subagent-run.sh`, gate logic in `phase-gate.sh`, `reconcile-carryover-todos.sh` promotion wired.

## Table of Contents

1. [Overview](#overview)
2. [Event Schema](#event-schema)
3. [Detector Table](#detector-table)
4. [Pipeline Flow](#pipeline-flow)
5. [Reconcile-Carryover Promotion Contract](#reconcile-carryover-promotion-contract)
6. [Gate Behavior](#gate-behavior)
7. [Troubleshooting](#troubleshooting)

---

## Overview

The abnormal-event capture pipeline provides structured observability for anomalies detected during phase dispatch. When a phase exits abnormally (quota exhaustion, integrity failure, dispatch error, etc.), a structured JSONL record is appended to `$WORKSPACE/abnormal-events.jsonl`. Downstream pipeline stages consume this file to:

- Trigger the retrospective phase even on PASS verdicts (`gate_audit_to_retrospective`)
- Promote unresolved anomaly types to `carryoverTodos[]` as HIGH-priority items (`reconcile-carryover-todos.sh`)

---

## Event Schema

Each line in `abnormal-events.jsonl` is a JSON object:

| Field | Type | Description |
|-------|------|-------------|
| `event_type` | string | Detector slug (see Detector Table) |
| `timestamp` | ISO-8601 | UTC timestamp when event was appended |
| `source_phase` | string | Always `"subagent-run"` (the writer) |
| `severity` | `HIGH` \| `LOW` | Impact level |
| `details` | string | Human-readable description |
| `remediation_hint` | string | Suggested operator action |

**Example:**
```json
{"event_type":"dispatch-error","timestamp":"2026-05-14T08:00:00Z","source_phase":"subagent-run","severity":"HIGH","details":"builder exited rc=1 stderr empty","remediation_hint":"check quota; retry with EVOLVE_CHECKPOINT_REQUEST=1"}
```

---

## Detector Table

Detectors are implemented in `scripts/dispatch/subagent-run.sh` via the `_append_abnormal_event()` function.

| event_type | Severity | Trigger Condition | Remediation Hint |
|------------|----------|-------------------|------------------|
| `dispatch-error` | HIGH | Phase exited non-zero rc with non-empty stderr | Check logs; re-run cycle |
| `quota-likely` | HIGH | rc=1 + empty stderr + cost ≥ `EVOLVE_QUOTA_DANGER_PCT`% of cap | Check subscription quota; use `--resume` |
| `integrity-fail` | HIGH | Artifact missing or SHA mismatch after phase | Investigate worktree corruption; re-run |
| `context-overflow` | HIGH | Prompt exceeds `EVOLVE_PROMPT_MAX_TOKENS` | Enable `EVOLVE_CONTEXT_AUTOTRIM=1` |
| `watchdog-kill` | HIGH | Phase watchdog sent SIGTERM | Increase `EVOLVE_WATCHDOG_TIMEOUT`; scope task smaller |

---

## Pipeline Flow

```
subagent-run.sh
  └─ _append_abnormal_event() → $WORKSPACE/abnormal-events.jsonl
       │
       ▼
phase-gate.sh gate_audit_to_retrospective
  └─ if abnormal-events.jsonl non-empty AND verdict=PASS:
       write .cycle-verdict=PASS-WITH-ABNORMAL
       allow retrospective phase
       │
       ▼
retrospective subagent
  └─ captures lessons from abnormal events
       │
       ▼
reconcile-carryover-todos.sh
  └─ for each unique event_type in abnormal-events.jsonl:
       promote to carryoverTodos[] priority=HIGH
       _inbox_source='abnormal-event:<type>'
```

---

## Reconcile-Carryover Promotion Contract

`scripts/lifecycle/reconcile-carryover-todos.sh` reads `$WORKSPACE/abnormal-events.jsonl` after normal carryover processing.

**Promotion rules:**
1. Parse all unique `event_type` values from the JSONL file.
2. For each unique type, check if a todo with `_inbox_source='abnormal-event:<type>'` already exists in `state.json:carryoverTodos[]`.
3. If not present, append a new entry:

```json
{
  "id": "abnormal-<event_type>-c<cycle>",
  "action": "Investigate and resolve abnormal event: <event_type>",
  "priority": "HIGH",
  "evidence_pointer": "abnormal-events.jsonl",
  "_inbox_source": "abnormal-event:<event_type>",
  "defer_count": 0,
  "first_seen_cycle": <cycle>,
  "last_seen_cycle": <cycle>,
  "cycles_unpicked": 0,
  "created_at": "<ISO-8601>"
}
```

4. Promotion is best-effort; failures emit a WARN log but do not fail the reconcile step.

**Deduplication:** The `_inbox_source` field is the deduplication key. Multiple occurrences of the same `event_type` in a single cycle produce exactly one carryoverTodo.

---

## Gate Behavior

### `gate_audit_to_retrospective` (phase-gate.sh)

| Verdict in audit-report.md | abnormal-events.jsonl | Gate decision |
|----------------------------|-----------------------|---------------|
| FAIL or WARN | any | PASS → advance to retrospective |
| PASS | non-empty | PASS → advance to retrospective; `.cycle-verdict=PASS-WITH-ABNORMAL` |
| PASS | empty or absent | FAIL → use `gate_audit_to_ship` instead |

### `.cycle-verdict` file

Written by `gate_audit_to_retrospective` for downstream consumers:

| Value | Meaning |
|-------|---------|
| `PASS-WITH-ABNORMAL` | Cycle passed audit but has abnormal events requiring retrospective |
| `FAIL` | Audit verdict was FAIL |
| `WARN` | Audit verdict was WARN |

---

## Troubleshooting

**Problem:** `abnormal-events.jsonl` not created despite a phase failure.

Check: Was the workspace directory present when `_append_abnormal_event` was called? The function is a no-op when `$WORKSPACE` does not exist. Verify `ls $WORKSPACE` from the run logs.

**Problem:** Gate blocks retrospective on PASS cycle with abnormal events.

Check: Is `abnormal-events.jsonl` non-empty? (`wc -l $WORKSPACE/abnormal-events.jsonl`). The gate uses `-s` (non-zero size), so an empty file is treated as absent.

**Problem:** Abnormal events not promoted to carryoverTodos after cycle.

Check: Did `reconcile-carryover-todos.sh` run? It only fires post-ship/post-retrospective via the orchestrator. Also verify the workspace path passed via `--workspace` matches the workspace that contains `abnormal-events.jsonl`.

**Problem:** Duplicate carryoverTodos for the same event_type.

The deduplication key is `_inbox_source='abnormal-event:<type>'`. If duplicates appear, check whether the field was stripped by an earlier version of `merge-lesson-into-state.sh` that did not preserve unknown fields.
