# Retrospective Pipeline — Lesson Persistence Contract

## Overview

The retrospective pipeline converts audit failures into durable lessons that
future cycles can learn from. It has two layers (Argyris & Schon 1978):

- **Single-loop:** `record-failure-to-state.sh` captures raw `failedApproaches[]` entries.
- **Double-loop:** `merge-lesson-into-state.sh` merges structured YAML lessons into
  `instinctSummary[]`, enabling next-cycle Scout/Builder/Auditor to see them.

## Lifecycle

```
Audit FAIL/WARN
  │
  ├── record-failure-to-state.sh $WORKSPACE $VERDICT
  │      Writes to state.json:failedApproaches[]
  │
  ├── subagent-run.sh retrospective $CYCLE $WORKSPACE
  │      Retrospective agent reads audit artifacts, produces:
  │      - retrospective-report.md (prose + ## Lessons YAML block)
  │      - handoff-retrospective.json (lessonIds[], lessonFiles[])
  │      - .evolve/instincts/lessons/<id>.yaml (one file per lesson)
  │
  └── merge-lesson-into-state.sh $WORKSPACE
         Reads handoff-retrospective.json
         Verifies each lessonId has a .yaml file on disk (integrity check)
         Appends to state.json:instinctSummary[] (pattern, confidence, type)
         Updates state.json:instinctCount
```

## Lesson YAML Format

Each lesson file at `.evolve/instincts/lessons/<id>.yaml` uses this format:

```yaml
id: cycle-N-slug
cycle: N
timestamp: "YYYY-MM-DD"
classification: code-audit-fail | code-audit-warn | unknown-classification
pattern: "one-line root-cause pattern for future matching"
lesson: "what should be done differently"
prevention: "concrete step to prevent recurrence"
instinct:
  - "one instinct per bullet"
priority: HIGH | MEDIUM | LOW
```

The `instinctSummary[]` entry in `state.json` records only the machine-actionable fields:

```json
{
  "id": "cycle-N-slug",
  "pattern": "...",
  "confidence": 0.90,
  "type": "failure-lesson",
  "errorCategory": "..."
}
```

## Gate: `gate_retrospective_to_complete`

`phase-gate.sh gate_retrospective_to_complete` verifies that:
1. `handoff-retrospective.json` exists with a `lessonIds[]` array
2. For each lessonId, a corresponding `.yaml` file exists on disk
3. Each lessonId appears in `state.json:instinctSummary[]`

This gate prevents PASS cycles from silently omitting the double-loop step.

## Backfill

When lesson YAML blocks exist only in prose within `retrospective-report.md`
(pre-v8.45 retrospectives, or gate-bypass incidents), run:

```bash
bash scripts/utility/backfill-lessons.sh [--dry-run] [--cycle N]
```

This script:
1. **Extracts** `- id:` items from ` ```yaml ``` ` blocks in `retrospective-report.md`
2. **Writes** each as `instincts/lessons/<id>.yaml` (skips existing files)
3. **Syncs** all on-disk `.yaml` files missing from `state.json:instinctSummary[]`

The backfill is idempotent — running it multiple times produces the same result.
Verify coverage with:

```bash
bash scripts/tests/lesson-persistence-test.sh
```

## Known Issues

- **Cycle-38 to cycle-39 gap:** `instinctSummary` froze after cycle-38 because
  the retrospective wrote YAML blocks in prose but the gate didn't enforce file-
  creation. Fixed by `backfill-lessons.sh` (cycle-44) and
  `gate_retrospective_to_complete` (cycle-44).
- **Cycle-40:** Three lesson YAMLs existed in `retrospective-report.md` only;
  `backfill-lessons.sh --cycle 40` writes them to disk and syncs `state.json`.
