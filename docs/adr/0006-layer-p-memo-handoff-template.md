# ADR 0006 — Layer-P Memo Phase Contract

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-12 |
| Cycle | 24–25 |
| Affects | evolve-orchestrator.md, evolve-memo-reference.md, memo phase, carryover todos |

## Context

The memo phase (Layer-P, v8.57.0+) was added to emit carryover todos after PASS cycles. Initial implementation in commit `4057ae5` (cycle 24) codified the contract in `evolve-orchestrator.md` but contained a false causal claim: the assertion that `merge-lesson-into-state.sh` required `memo.md` to be present before it could run.

This false claim was structurally problematic because:
1. It implied a dependency that did not exist in the code (`merge-lesson-into-state.sh` reads `handoff-retrospective.json`, not `memo.md`).
2. Orchestrators that believed the claim serialized incorrectly: waiting for `memo.md` before running the merge script when the two are independent.
3. Terminology was inconsistent: early versions of the contract called `memo.md` a "handoff document" rather than a "cycle memo" (the canonical term per `CONTEXT.md`, ADR 0004).

Commit `6384d66` (cycle 25, c27b) corrected the false claim and updated terminology.

## Decision

The Layer-P memo phase contract (as corrected by `6384d66`) specifies:

**Two artifacts emitted by the memo agent:**
- `carryover-todos.json` — machine-readable; consumed by `reconcile-carryover-todos.sh`.
- `memo.md` — human-readable cycle memo at path `.evolve/runs/cycle-N/memo.md`.

**Six requirements for `memo.md` (quality gate enforced by orchestrator after `subagent-run.sh memo` returns):**

| Requirement | Rule |
|---|---|
| Output path | `$WORKSPACE/memo.md` |
| Artifact references | MUST cite scout-report, build-report, and audit-report by path and SHA; MUST NOT re-summarize their content |
| Skill suggestions | MUST list 2–4 persona-action suggestions for the next cycle |
| carryoverTodo guidance | MUST name which carryover IDs to prioritize next cycle and explain why |
| Line cap | MUST be ≤100 lines |
| Anti-goal | MUST NOT replace or paraphrase audit-report — memo is a cycle memo, not a re-audit |

**Corrected dependency model (from `6384d66`):**
- `merge-lesson-into-state.sh` reads `handoff-retrospective.json` — independent of `memo.md`.
- The quality gate for `memo.md` (existence + ≤100 lines) is a separate orchestrator concern.
- The orchestrator reads `memo.md` during the next cycle's calibrate phase to orient itself; this is the only runtime consumer.

If `memo.md` is absent after `subagent-run.sh memo` returns exit 0, the orchestrator records `code-audit-warn` via `record-failure-to-state.sh` before continuing — it does not block the merge script.

## Consequences

**Positive:**
- Clear quality gate for the human-readable cycle memo; absent or oversized memos surface as WARN rather than silent failure.
- Dependency model corrected: `merge-lesson-into-state.sh` and memo phase are independent and can be reasoned about separately.
- Canonical terminology ("cycle memo") aligned with `CONTEXT.md` (ADR 0004); "handoff document" retired.

**Negative:**
- The false claim in `4057ae5` was live for one cycle (cycle 24) before correction; any orchestrator that ran during cycle 24 may have serialized incorrectly. Ledger audit shows no shipped cycles with the wrong serialization order.
- The ≤100-line cap on `memo.md` is a blunt constraint; complex cycles may produce memos that require editing to fit within the cap.

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Allow arbitrary `memo.md` length | Memos grow unboundedly; long memos defeat the purpose of a concise cycle summary |
| Combine `carryover-todos.json` and `memo.md` into one file | Machine-parseable JSON and human-readable markdown serve different consumers; separation of concerns |
| Remove the memo quality gate | Silent failures; the 13-WARN streak (ADR 0004) shows that absent validation leads to drift |
| Have `merge-lesson-into-state.sh` depend on `memo.md` | Not how the code works; would create a false coupling that complicates failure recovery |

## Implementation

- `agents/evolve-orchestrator.md` — six-requirement table + quality gate logic (initial: `4057ae5`; corrected: `6384d66`)
- `agents/evolve-memo-reference.md` — `memo-template` section with the canonical section template (Layer-3 on-demand)

## Cross-reference

- `agents/evolve-memo-reference.md` — section `memo-template` for the full `memo.md` section template.
- `CONTEXT.md` — canonical definition of "memo" (cycle memo, not handoff document).
- ADR 0004 — CONTEXT.md adoption that established the canonical terminology corrected in `6384d66`.
