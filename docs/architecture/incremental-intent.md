---
name: incremental-intent
description: Delta-mode intent resolution — skip full intent rewrite when goal is unchanged across cycles in a batch
metadata:
  type: architecture
---

> **Incremental intent (v0.1)** — opt-in delta mode (`EVOLVE_INTENT_DELTA=1`) that compares a normalized goal hash against the current batch to determine whether the intent persona must run in full or can emit a lightweight patch. Builds on [[intent-phase.md]] without duplicating its core design.

## TLDR

**Synopsis:** When `EVOLVE_INTENT_DELTA=1`, the pipeline computes a `GOAL_HASH` before the intent phase; if the hash matches the current batch's stored hash, the intent persona emits `intent-delta.md` (a patch) or `[intent-unchanged]` instead of a full `intent.md`, saving tokens and turn budget.

**Key points:**
- `EVOLVE_INTENT_DELTA=0` (default): existing full-intent flow unchanged
- `EVOLVE_INTENT_DELTA=1`: opt-in delta mode — `intent-batch-resolve.sh` computes `INTENT_MODE=full|delta` before intent phase
- `intent-merge-patches.sh` materializes the final `intent.md` from the patch after the intent phase
- `[intent-unchanged]` marker is a valid delta-mode output — treated as no-op by merge script and accepted by `gate_intent_to_research`
- Karpathy Rule constraint: delta mode must still trigger full re-examination on FAIL audits or new inbox/carryover events

**Non-goals:** Cycle 2–4 enhancements (hybrid decomp, builder/auditor read slice, AB measurement); changes to full-intent mode behavior; retroactive migration of existing cycles.

## Table of Contents

1. [Why Incremental Intent](#why-incremental-intent)
2. [Batch and GoalHash Concepts](#batch-and-goalhash-concepts)
3. [Delta vs Full Trigger Logic](#delta-vs-full-trigger-logic)
4. [Intent-delta.md Format](#intent-deltamed-format)
5. [Batch Directory Layout](#batch-directory-layout)
6. [State.json Schema Addition](#statejson-schema-addition)
7. [Phase Pipeline Integration](#phase-pipeline-integration)
8. [Phase-gate Contract for Delta Mode](#phase-gate-contract-for-delta-mode)
9. [Fallback Semantics](#fallback-semantics)
10. [Karpathy Rule Constraint](#karpathy-rule-constraint)
11. [References](#references)

---

## Why Incremental Intent

The full intent phase costs ~1–2 turns and $0.10–0.30 per cycle (Opus pricing). For multi-cycle batches pursuing the same goal — e.g., a 5-cycle queue-drain of carryover todos — the intent is structurally identical across cycles. Re-running a full intent persona each time is wasteful.

At the same time, the intent phase exists to surface wrong assumptions (Karpathy's #1 failure mode). Skipping it entirely risks drift. Delta mode threads the needle: run full intent on first cycle and on trigger conditions; emit a lightweight patch or unchanged marker otherwise.

---

## Batch and GoalHash Concepts

A **batch** is a sequence of cycles sharing the same normalized goal. It has:
- `batchId` — a deterministic prefix + timestamp string (e.g., `batch-20260519T142300Z`)
- `goalHash` — SHA256 of the normalized goal text (whitespace-collapsed, lowercased)

A **GoalHash match** means the current cycle's goal is materially the same as the batch's stored goal. A mismatch forces `INTENT_MODE=full` regardless of `EVOLVE_INTENT_DELTA`.

`state.json:currentBatch` stores the active batch context:

```json
{
  "currentBatch": {
    "batchId": "batch-20260519T142300Z",
    "goalHash": "abc123...",
    "startCycle": 84,
    "intentFile": ".evolve/batch/batch-20260519T142300Z/intent.md"
  }
}
```

---

## Delta vs Full Trigger Logic

`intent-batch-resolve.sh` computes `INTENT_MODE` using this decision tree:

```
EVOLVE_INTENT_DELTA == 0  →  INTENT_MODE=full  (default, env-off)
goalHash != currentBatch.goalHash  →  INTENT_MODE=full  (goal changed)
currentBatch.goalHash not set  →  INTENT_MODE=full  (first cycle of batch)
lastAuditVerdict == FAIL  →  INTENT_MODE=full  (re-examine premises)
newInboxOrCarryover == true  →  INTENT_MODE=full  (scope changed)
else  →  INTENT_MODE=delta
```

The first three conditions ensure correctness. The last two implement the Karpathy Rule: even when the goal hash matches, a FAIL audit or new scope items warrant fresh premise-challenging.

---

## Intent-delta.md Format

When `INTENT_MODE=delta`, the intent persona emits one of two outputs:

**Option A — Patch file (`intent-delta.md`):**

```markdown
---
intent_delta: true
cycle: <N>
base_intent: <batchId>/intent.md
---

## Changed fields

### constraints
- ADDED: "<new constraint>"

### acceptance_checks
- MODIFIED check: "<old>" → "<new>"

## Unchanged

All other fields from base intent carry forward unchanged.
```

**Option B — Unchanged marker:**

The file `intent-delta.md` contains only the literal text `[intent-unchanged]`. This signals that no material changes to intent are needed this cycle. `intent-merge-patches.sh` treats this as a no-op.

---

## Batch Directory Layout

```
.evolve/batch/<batchId>/
    intent.md          ← canonical merged intent for this batch
    intent-delta-N.md  ← per-cycle delta archives (optional)
```

After `intent-merge-patches.sh` runs, the workspace `intent.md` is a symlink to `.evolve/batch/<batchId>/intent.md`. This ensures the intent persona reads the merged canonical file on subsequent delta cycles.

---

## State.json Schema Addition

`state.json:currentBatch` is a new optional top-level key:

| Field | Type | Description |
|-------|------|-------------|
| `batchId` | string | Batch identifier |
| `goalHash` | string | SHA256 of normalized goal text |
| `startCycle` | number | First cycle of this batch |
| `intentFile` | string | Path to canonical intent.md for this batch |

`intent-batch-resolve.sh` writes this key when starting a new batch (full mode) or updates `goalHash` when the goal changes. Reading is idempotent; missing key → `INTENT_MODE=full`.

---

## Phase Pipeline Integration

`run-cycle.sh` additions when `EVOLVE_INTENT_DELTA=1`:

**Before intent phase:**
```bash
eval "$(bash legacy/scripts/lifecycle/intent-batch-resolve.sh "$WORKSPACE/intent.md")"
export INTENT_MODE BATCH_ID GOAL_HASH
```

**After intent phase:**
```bash
bash legacy/scripts/lifecycle/intent-merge-patches.sh "$WORKSPACE/intent.md" "$WORKSPACE/intent-delta.md"
# Set up symlink: workspace/intent.md → .evolve/batch/<batchId>/intent.md
```

When `EVOLVE_INTENT_DELTA=0` (default): both script invocations are skipped entirely. Existing flow is unchanged.

---

## Phase-gate Contract for Delta Mode

`gate_intent_to_research` additions when `EVOLVE_INTENT_DELTA=1`:

- Accept `intent-delta.md` containing `[intent-unchanged]` → pass (no structural checks required)
- Accept `intent-delta.md` with patch format → pass (structural checks apply to the merged `intent.md`, not the delta file)
- When `EVOLVE_INTENT_DELTA=0`: existing validation logic unchanged (requires `intent.md` with full structure)

The YAML frontmatter + `awn_class` + `challenged_premises` checks always apply to the resolved (merged) `intent.md`, never to `intent-delta.md` directly.

---

## Fallback Semantics

| Condition | Behavior |
|-----------|----------|
| `EVOLVE_INTENT_DELTA=0` (default) | Full intent every cycle; delta scripts not invoked |
| `EVOLVE_INTENT_DELTA=1` + no batch | `INTENT_MODE=full`; new batch created |
| `EVOLVE_INTENT_DELTA=1` + hash match | `INTENT_MODE=delta` (unless Karpathy Rule triggers full) |
| `EVOLVE_INTENT_DELTA=1` + hash mismatch | `INTENT_MODE=full`; batch updated |
| `intent-batch-resolve.sh` fails | Fall through to `INTENT_MODE=full` (safe default) |
| `intent-merge-patches.sh` fails | Log WARN; existing `intent.md` unchanged |

---

## Karpathy Rule Constraint

Delta mode must not suppress premise-challenging when it matters most. Two conditions force `INTENT_MODE=full` even when the goal hash matches:

1. **Prior FAIL audit** — the last cycle in this batch produced a FAIL verdict. Premises that were accepted may have contributed to the failure. Re-examine from scratch.

2. **New inbox/carryover events** — new tasks arrived since the last full intent cycle. The scope has changed; the intent persona must update `acceptance_checks` and `non_goals`.

This constraint is enforced in `intent-batch-resolve.sh` by reading `state.json:lastAuditVerdict` and checking for new carryover items added since `currentBatch.startCycle`.

---

## References

- `docs/architecture/intent-phase.md` — base intent phase design (read first to avoid duplication)
- `legacy/scripts/lifecycle/intent-batch-resolve.sh` — computes `INTENT_MODE`, `BATCH_ID`, `GOAL_HASH`
- `legacy/scripts/lifecycle/intent-merge-patches.sh` — applies delta patches or handles `[intent-unchanged]`
- `agents/evolve-intent.md` — intent persona with delta-mode output contract
- `archive/legacy/scripts/dispatch/run-cycle.sh` — `EVOLVE_INTENT_DELTA` integration point
- `legacy/scripts/lifecycle/phase-gate.sh` — `gate_intent_to_research` with delta acceptance
- CLAUDE.md env-var table — `EVOLVE_INTENT_DELTA` entry
