---
name: evolve-memo-reference
description: Layer-3 on-demand reference for the Layer-P memo phase. Defines the section template and downstream-consumer contract for the memo subagent output.
---

# Evolve Memo Reference

> Layer-3 on-demand reference for the Layer-P memo phase. Read the Layer-P Memo Phase Contract section in `agents/evolve-orchestrator.md` first — come here only when you need the full `memo.md` section template or rationale. See `CONTEXT.md` for canonical definitions of "memo", "cycle memo", and "Layer-P".

## memo-template

`memo.md` is a cycle memo, not a report. Its audience is the next cycle's orchestrator and Scout, not an auditor reviewing the current cycle. Every section listed below is required; the total document MUST be ≤100 lines.

### Required sections (in order)

| Section heading | Content rule |
|---|---|
| `## Cycle N — Memo` | State the cycle number and one sentence summarizing what shipped |
| `## Artifact Index` | List each cycle report by relative path and SHA8; do NOT paraphrase or excerpt content |
| `## Skill Suggestions` | List 2–4 persona-action suggestions for the next cycle, each naming a specific carryover ID and the recommended evolve-loop flag or persona mode |
| `## carryoverTodo Guidance` | Name which carryover IDs to prioritize next cycle, state the sequencing rationale, and note any preconditions satisfied this cycle |

### Artifact Index format

Each entry in `## Artifact Index` is a single prose line stating the relative path (from `$PROJECT_ROOT`) and the SHA8 of the file at the time of ship. The orchestrator MUST NOT quote, excerpt, or paraphrase content from these files — the index is a pointer, not a summary. Path and SHA8 are sufficient for the next cycle's agents to locate and read the artifact independently.

### Skill Suggestions format

Each suggestion is one imperative sentence naming the carryover ID it targets, the recommended persona or flag, and optionally why. Two to four entries. Suggestions map directly to evolve-loop persona actions: a specific `EVOLVE_TASK_MODE`, a `subagent-run.sh` invocation variant, or an env flag to enable. Keep each suggestion to one sentence.

### carryoverTodo Guidance format

State the top-pick ID and its sequencing rationale in 2–4 sentences. If a precondition was satisfied this cycle (for example, c27 shipped and unblocks c28 and c29), say so explicitly so the next orchestrator does not need to re-derive it from the full carryover list.

### What memo.md MUST NOT contain

| Prohibited content | Correct location |
|---|---|
| Findings or defect analysis | audit-report.md |
| Re-statement of scout-report content | scout-report.md |
| New carryover entries not in carryover-todos.json | carryover-todos.json (via reconcile-carryover-todos.sh) |
| Any section that causes total line count to exceed 100 | Split into next cycle's memo if needed |

### Downstream consumers

`merge-lesson-into-state.sh` reads `handoff-retrospective.json` to extract lesson context before writing instincts to `state.json`.
`memo.md` is not a dependency of the merge script — the merge script exits 0 on absent `handoff-retrospective.json` (PASS cycles have no lesson to merge) and exits 2 only when a lesson YAML referenced in `handoff-retrospective.json` is missing from `$LESSONS_DIR`.
The next cycle's orchestrator reads `memo.md` during calibrate to orient itself without re-reading all prior reports.
