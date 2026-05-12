> **Layer-3 reference** — on-demand detail for the Triage agent. Read only when Triage Step 0 (inbox ingestion) needs algorithmic detail or when the Auditor needs schema verification. Routine cycles need not load this file.
>
> **TOC**: [Inbox JSON Schema](#inbox-json-schema) · [Ingestion Algorithm](#ingestion-algorithm) · [Reconcile-Compatible Schema](#reconcile-compatible-schema) · [Priority + Weight Scoring](#priority--weight-scoring) · [Error Codes](#error-codes)

# Evolve Triage Reference — Inbox Ingestion (v9.5.0+)

## Inbox JSON Schema

Files written by `scripts/utility/inject-task.sh` to `.evolve/inbox/<ts>-<rand>.json`:

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | string | yes | Unique; auto-generated as `user-<epoch>-<hex8>` if absent |
| `action` | string | yes | Non-empty task description |
| `priority` | enum | yes | `HIGH` \| `MEDIUM` \| `LOW` (stored uppercase) |
| `weight` | float\|null | no | Tie-breaker within priority class; defaults to 0.5 if null |
| `evidence_pointer` | string | no | Auto-synthesized as `inbox-injection://<injected_at>` if absent |
| `operator_note` | string | no | Free-form operator context |
| `injected_at` | ISO8601 | auto | Set by inject-task.sh |
| `injected_by` | enum | auto | `operator` \| `test` \| `automation` |

## Ingestion Algorithm

Step 0 in `evolve-triage.md` processes inbox files in this order:

1. List `.evolve/inbox/*.json` — maxdepth 1; skip `processed/` and `rejected/` subdirs.
2. Parse each file; malformed JSON → log `inbox-malformed-json` WARN in `## Inbox Errors`, move to `.evolve/inbox/rejected/cycle-<N>/`, continue.
3. Validate required fields (`id`, `action`, `priority`); missing/empty → WARN + reject.
4. Validate `priority` ∈ {HIGH, MEDIUM, LOW} and `weight` ∈ [0.0, 1.0] or null; invalid → WARN + reject.
5. Check `id` uniqueness against `state.json:carryoverTodos[]` and already-ingested items; collision → WARN + reject.
6. Transform to reconcile-compatible schema (see below); append to working set.
7. Move file to `processed/cycle-<N>/`.
8. Write ledger entry: `role=triage, action=ingest-inbox, count=<ingested>, rejected=<rejected>`.

## Reconcile-Compatible Schema

Transformation applied at Triage ingestion:

```json
{
  "id": "<from inbox>",
  "action": "<from inbox>",
  "priority": "<from inbox>",
  "weight": "<from inbox, or 0.5 if null>",
  "evidence_pointer": "<from inbox>",
  "defer_count": 0,
  "cycles_unpicked": 0,
  "first_seen_cycle": "<current cycle N>",
  "last_seen_cycle": "<current cycle N>",
  "_inbox_source": {
    "operator_note": "<from inbox>",
    "injected_at": "<from inbox>",
    "injected_by": "<from inbox>"
  }
}
```

`_inbox_source` is preserved metadata. `reconcile-carryover-todos.sh`'s `jq -c '. + {...}'` pass-through preserves it automatically (no logic change needed).

## Priority + Weight Scoring

Triage `top_n` selection honors this ordering:

1. Group by `priority`: HIGH (3) > MEDIUM (2) > LOW (1).
2. Within group, sort by `weight` descending (default 0.5 when null).
3. Within tied weight, sort by `injected_at` ascending (FIFO).
4. Apply existing top_n criteria (scope, intent-alignment, cycle-goal-blocking).

Existing priority-only todos continue to work; they're assigned weight=0.5 implicitly.

## Error Codes

| Code | Meaning |
|---|---|
| `inbox-malformed-json` | File could not be parsed as JSON |
| `inbox-missing-required` | `id`, `action`, or `priority` field absent or empty |
| `inbox-invalid-priority` | `priority` not in {HIGH, MEDIUM, LOW} |
| `inbox-invalid-weight` | `weight` present but not a float in [0.0, 1.0] |
| `inbox-id-collision` | `id` matches an existing carryoverTodo or already-ingested inbox item |

All errors result in the file being moved to `.evolve/inbox/rejected/cycle-<N>/` with a WARN line in `triage-decision.md ## Inbox Errors`.
