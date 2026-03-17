# Cycle 31 Scout Report

## Discovery Summary
- Scan mode: incremental (final cycle)
- Files analyzed: 8 (state.json, agent-mailbox.md, builder-notes.md, cycle-22-29-instincts.yaml, docs/meta-cycle.md, docs/architecture.md, CHANGELOG.md, plugin.json)
- Research: skipped (final cycle — no web research permitted)
- Instincts applied: 2
- **instinctsApplied:** [inst-014 (activate-dormant-infrastructure — instinctSummary sync gap is exactly the pattern this instinct describes: infrastructure was built but not fully wired), inst-022 (meta-cycle-extraction-stall — the extraction schedule gap is the upstream cause of the 8-cycle stall; inst-022 itself now guides prevention)]

## Key Findings

### instinctSummary Desync — HIGH
Cycle 30 extracted 6 new instincts (inst-018 through inst-023) and wrote them to `cycle-22-29-instincts.yaml`. The `instinctCount` in state.json was correctly updated to 24. However, the `instinctSummary` compact array still contains only 17 entries (inst-001 through inst-017). The 6 new instincts are absent from the summary.

Impact: every future cycle reads `instinctSummary` as the authoritative instinct feed. Agents will not see inst-018 through inst-023. This defeats the purpose of extracting them. The gap is a direct data inconsistency (count says 24, summary has 17).

Note on instinctCount: cross-referencing the actual YAML files:
- `grep -c "^- id:"` across all personal YAML files yields 6 (cycle-22-29) + the prior files. The builder-notes say "inst-001 through inst-023 = 23 active instincts" (inst-024 is not in the YAML). instinctCount should be corrected to 23 alongside the summary sync.

### No Extraction Schedule — MEDIUM
Builder-notes from cycle 30 explicitly flagged: "Schedule next extraction task around cycle 33-34 to avoid another 8-cycle stall." No extraction schedule exists in state.json. Without a proactive reminder, the next session will follow the same pattern. Adding a `nextExtractionCycle` field (target: 34) or a `pendingImprovements` entry is a 1-line change with session-spanning benefit.

### Everything Else — GREEN
- meta-cycle.md: updated in cycle 30, includes LLM-as-a-Judge section and self-learning.md link (90 lines)
- instincts YAML coverage: cycles 1-9, 17-21, 22-29 all have files; no gaps
- processRewards.learn: corrected to 0.0 in cycle 30
- Plugin version: 6.9.0 in both plugin.json and marketplace.json, consistent
- Docs cross-references: architecture.md, self-learning.md, memory-hierarchy.md all cross-linked

---

## Selected Tasks

### Task 1: Sync instinctSummary with inst-018 through inst-023
- **Slug:** sync-instinct-summary-cycle31
- **Type:** stability
- **Complexity:** S
- **Rationale:** instinctSummary has 17 entries; instinctCount says 24; YAML has 23 active instincts. The 6 new instincts from cycle 30 were written to disk but never appended to the compact summary array. Every future cycle runs without inst-018 through inst-023. Fix is: read cycle-22-29-instincts.yaml, append 6 compact entries to instinctSummary, correct instinctCount to 23. 1 file, ~20 lines of JSON addition.
- **Acceptance Criteria:**
  - [ ] `state.json.instinctSummary` has 23 entries (inst-001 through inst-023)
  - [ ] inst-018 through inst-023 all present with correct `pattern`, `confidence`, `type` fields
  - [ ] `state.json.instinctCount` corrected to 23
  - [ ] state.json remains valid JSON
- **Files to modify:** `.claude/evolve/state.json`
- **Eval:** written to `evals/sync-instinct-summary-cycle31.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert len(s['instinctSummary']) == 23"` → expects exit 0
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); ids=[e['id'] for e in s['instinctSummary']]; missing=[f'inst-0{i:02d}' for i in range(18,24) if f'inst-0{i:02d}' not in ids]; assert not missing, f'missing: {missing}'"` → expects exit 0
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert s['instinctCount'] == 23"` → expects exit 0

### Task 2: Add next instinct extraction schedule hint to state.json
- **Slug:** add-extraction-schedule-hint
- **Type:** stability
- **Complexity:** S
- **Rationale:** The 8-cycle extraction stall (cycles 22-29) was caused by having no scheduled reminder for extraction. Builder-notes from cycle 30 explicitly recommended scheduling the next extraction at cycle 33-34. Adding `"nextExtractionCycle": 34` to state.json (and a matching `pendingImprovements` entry so the Scout sees it as a task candidate) closes this prevention gap. This is the minimum intervention to prevent the stall from recurring in the next session. 1 file, 5-8 lines changed.
- **Acceptance Criteria:**
  - [ ] `state.json.nextExtractionCycle` field set to 34
  - [ ] `state.json.pendingImprovements` contains an entry for instinct extraction with `triggerAtCycle: 34`
  - [ ] state.json remains valid JSON
- **Files to modify:** `.claude/evolve/state.json`
- **Eval:** written to `evals/add-extraction-schedule-hint.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert s.get('nextExtractionCycle') == 34"` → expects exit 0
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert any('extraction' in str(p).lower() for p in s.get('pendingImprovements', []))"` → expects exit 0

---

## Deferred
- Full instinct graduation sweep (inst-004, inst-007, inst-011 all at >= 0.9 confidence): deferred — this is covered by the LEARN phase memory consolidation; adding a separate task would duplicate that work
- Docs version bump to v6.9.1: deferred — no code changes this cycle; the CHANGELOG is current at v6.9.0; bumping for documentation-only changes adds noise
- Agent mailbox cleanup (removing stale cycle-28/29/30 hints): deferred — mailbox messages are ephemeral; the file is not shown to users and cleanup adds minimal value on a final cycle

---

## Decision Trace

```json
{
  "decisionTrace": [
    {
      "slug": "sync-instinct-summary-cycle31",
      "finalDecision": "selected",
      "signals": ["data-inconsistency", "instinctSummary-has-17-instinctCount-says-24", "inst-014-applied", "zero-blast-radius-state-json-only", "final-cycle-S-complexity"]
    },
    {
      "slug": "add-extraction-schedule-hint",
      "finalDecision": "selected",
      "signals": ["builder-notes-explicit-recommendation", "prevents-8-cycle-stall-recurrence", "inst-022-applied", "1-file-5-lines", "final-cycle-S-complexity"]
    },
    {
      "slug": "instinct-graduation-sweep",
      "finalDecision": "deferred",
      "signals": ["duplicates-learn-phase-work", "covered-by-memory-consolidation", "not-needed-this-cycle"]
    },
    {
      "slug": "version-bump-6-9-1",
      "finalDecision": "rejected",
      "signals": ["no-code-changes-this-cycle", "CHANGELOG-current-at-6-9-0", "noise-only"]
    },
    {
      "slug": "mailbox-cleanup",
      "finalDecision": "rejected",
      "signals": ["ephemeral-file", "minimal-value-final-cycle"]
    }
  ]
}
```

---

## Mailbox Posts

| from | to | type | cycle | persistent | message |
|------|----|------|-------|------------|---------|
| scout | builder | hint | 31 | false | Task 1 (sync-instinct-summary-cycle31): read .claude/evolve/instincts/personal/cycle-22-29-instincts.yaml to get inst-018 through inst-023 fields. Append 6 compact entries to state.json instinctSummary array. Each entry needs id, pattern, confidence, type. Also correct instinctCount from 24 to 23 (actual count in YAML files is 23). |
| scout | builder | hint | 31 | false | Task 2 (add-extraction-schedule-hint): add "nextExtractionCycle": 34 at top-level of state.json. Also append to pendingImprovements array: {"type": "instinct-extraction", "description": "Extract instincts from cycles 31-33. Schedule triggered by cycle-30 builder-notes recommendation.", "triggerAtCycle": 34, "priority": "medium"}. |
| scout | auditor | hint | 31 | false | Both tasks modify only state.json. Eval graders use python3 with relative path .claude/evolve/state.json — use absolute path in worktree context. Both evals are pure JSON assertions, no bash commands needed beyond python3. |

---

## Ledger Entry
```json
{"ts":"2026-03-17T14:00:00Z","cycle":31,"role":"scout","type":"discovery","data":{"scanMode":"incremental","filesAnalyzed":8,"researchPerformed":false,"tasksSelected":2,"instinctsApplied":2}}
```
