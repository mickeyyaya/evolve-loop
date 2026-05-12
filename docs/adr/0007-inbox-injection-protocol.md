# ADR 0007 — Inbox-Injection Protocol

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-13 |
| Cycle | 27 |
| Affects | Operator API, Triage phase, carryoverTodos backlog |

## Context

Operators needed a channel to inject tasks into a running `/evolve-loop` dispatcher. Direct `state.json` writes failed silently: the reconcile pass drops entries that don't match the canonical 8-field schema, and direct writes race the orchestrator's read-modify-write pass.

Three constraints shaped the design:

1. `feedback_defer_to_cycle_completion.md` — direct in-cycle state.json edits may be silently dropped.
2. `feedback_multi_project_isolation.md` — any new state must be per-project (namespaced by `$EVOLVE_PROJECT_ROOT`).
3. `feedback_doc_stewardship_policy.md` — feature lands with ADR + docs/ entry per policy.

## Decision

Use a file-based inbox directory (`.evolve/inbox/`) with Triage ingestion at phase start.

Operators write task files via `scripts/utility/inject-task.sh`. Triage validates, transforms to reconcile-compatible schema, and ingests at the start of its phase. Processed files are archived to `processed/cycle-N/`; rejected files to `rejected/cycle-N/`.

## Consequences

**Positive:**
- No race condition — Triage reads inbox exactly once per cycle, at phase start.
- Schema enforced at ingestion — bad entries are rejected with WARN, not silently dropped.
- Per-project by construction — `.evolve/inbox/` lives under `$EVOLVE_PROJECT_ROOT`.
- Cancellable — operators delete the file before the next Triage phase.
- Audit trail — every ingestion writes a ledger entry.

**Negative:**
- Injections during an active cycle are deferred to the next cycle (by design).
- No batch multi-priority injection in v1 (one task per CLI invocation).

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| HTTP API endpoint | Overkill for CLI tool; introduces security surface |
| Direct state.json write | Race window + schema burden on operator; reconcile drops mismatched entries |
| Named pipe / FIFO | Complex lifecycle; process-coupling |
| Daemon process | Heavyweight; incompatible with CI/CD workflows |

## Implementation

- `scripts/utility/inject-task.sh` — CLI: validate schema, atomic write to inbox
- `agents/evolve-triage.md` — Step 0: inbox ingestion before top_n logic
- `agents/evolve-triage-reference.md` — algorithm + schema reference (Layer-3)
- `docs/architecture/inbox-injection-protocol.md` — operator-facing protocol doc
- `scripts/tests/inject-task-test.sh` — CLI validation test coverage
- `scripts/tests/triage-inbox-ingestion-test.sh` — schema + ingestion structure tests
