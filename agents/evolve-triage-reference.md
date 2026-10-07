> **Layer-3 reference** — on-demand detail for the Triage agent. Read only when Triage Step 0 (inbox ingestion) needs algorithmic detail or when the Auditor needs schema verification. Routine cycles need not load this file.
>
> **TOC**: [Inbox JSON Schema](#inbox-json-schema) · [Ingestion Algorithm](#ingestion-algorithm) · [Reconcile-Compatible Schema](#reconcile-compatible-schema) · [Priority: the Inbox Rank](#priority-the-inbox-rank) · [Error Codes](#error-codes)

# Evolve Triage Reference — Inbox Ingestion (v9.5.0+)

## Inbox JSON Schema

Files written by `legacy/scripts/utility/inject-task.sh` to `.evolve/inbox/<ts>-<rand>.json`:

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | string | yes | Unique; auto-generated as `user-<epoch>-<hex8>` if absent |
| `action` | string | yes | Non-empty task description |
| `priority` | enum | yes | `HIGH` \| `MEDIUM` \| `LOW` (stored uppercase) |
| `weight` | float\|null | no | The filer's judgment of value: the `base` input of the inbox rank; defaults to 0.5 if null |
| `priority_class` | enum | yes for `evolve inbox add` | One of the policy's `inbox_priority.class_order`; the `class` input of the rank (ADR-0121) |
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

## Priority: the Inbox Rank

Since 2026-10-06 (ADR-0121, plan P2) every inbox consumer reads one order, `inboxrank.Order`:

1. **The score.** `score = Σ factor × feature` over `base` (the `weight`), `class` (the `priority_class` position in `inbox_priority.class_order`), `unblocks` (queued items whose `deps` reach this one), `recurrence` (the recurrence ledger's count), `age` and `goal` (an active campaign). The factor weights live in `.evolve/policy.json` `inbox_priority`.
2. **Ties** break on a declared fix surface first, then the older filing date, then the id.
3. **The menu.** The prompt's `inbox_batches` lists batches in that order (a batch ranks by its best-ranked member, and members keep the rank order); each item's sub-line shows its score and top factor.
4. Apply the existing top_n criteria (scope, intent-alignment, cycle-goal-blocking) over that order.

`evolve inbox rank [--explain <id>]` prints the same order with every factor.

> **Superseded (2026-10-06).** The ordering this section described before P2 was: group by `priority` (HIGH 3 > MEDIUM 2 > LOW 1), then `weight` descending (0.5 when null), then `injected_at` ascending (FIFO). The wired code had already sorted by `weight` alone; the rank replaced both.

## Error Codes

| Code | Meaning |
|---|---|
| `inbox-malformed-json` | File could not be parsed as JSON |
| `inbox-missing-required` | `id`, `action`, or `priority` field absent or empty |
| `inbox-invalid-priority` | `priority` not in {HIGH, MEDIUM, LOW} |
| `inbox-invalid-weight` | `weight` present but not a float in [0.0, 1.0] |
| `inbox-id-collision` | `id` matches an existing carryoverTodo or already-ingested inbox item |

All errors result in the file being moved to `.evolve/inbox/rejected/cycle-<N>/` with a WARN line in `triage-report.md ## Inbox Errors`.
