# Cycle 30 Scout Report

## Discovery Summary
- Scan mode: incremental (meta-cycle)
- Files analyzed: 14 (state.json, phases.md, memory-protocol.md, architecture.md, meta-cycle.md, self-learning.md, docs/*.md, instinct YAMLs cycles 1-21, builder-notes.md, agent-mailbox.md)
- Research: skipped (meta-cycle rules prohibit web research this cycle)
- Instincts applied: 3
- **instinctsApplied:** [inst-014 (activate-dormant-infrastructure — instinctSummary was designed but never populated, applying to task 1), inst-015 (signal-to-action-wiring — extraction trigger exists but stalled, applying to task 2), inst-016 (gap-detection-over-degradation-only — meta-cycle surface a capability gap not a degradation)]

## Meta-Cycle Review: Cycles 25-29

### Performance Metrics (cycles 22-29 from evalHistory)
| Metric | Value | Status |
|--------|-------|--------|
| tasksShipped | 3.0 avg (cycles 22-29) | GREEN |
| successRate | 1.0 (8 consecutive) | GREEN |
| auditIterations | 1.0 avg | GREEN |
| instinctsExtracted | 0 (all 7 cycles) | CRITICAL RED |
| stagnationPatterns | 0 | GREEN |

### Split-Role Critique

**Efficiency Critic:**
Token usage is well-optimized. All recent tasks are S-complexity targeting new files. Plan cache has 4 high-success entries. No wasted retries. Score: GREEN.

**Correctness Critic:**
All 78 tasks shipped with 100% success rate across 30 cycles. Audit verdicts clean. However: `instinctSummary` field is absent from state.json despite being referenced throughout phases.md and agent definitions. This means agents have been running without instinct guidance for cycles 22-29 — a silent failure mode. Score: AMBER (silent gap, not a build failure).

**Novelty Critic:**
CRITICAL: `instinctsExtracted == 0` for 7+ consecutive cycles. The instinct extraction trigger (added cycle 27, step 3b in phases.md) was never validated to work — it depends on checking `evalHistory.instinctsExtracted` values, but because the field has always been 0 since the check was designed, the trigger should have fired every cycle since cycle 27. It did not fire. Root cause: the orchestrator implementing the LEARN step is not consistently running the forced extraction block. Additionally, `instinctSummary` does not exist in state.json, meaning agents have been reading `null` for this field. Score: RED.

---

## Key Findings

### Instinct Infrastructure — CRITICAL
The `instinctSummary` compact array field is documented in memory-protocol.md and phases.md as the primary mechanism for agents to consume instincts without reading YAML files. It does NOT exist in state.json. This means:
- The Scout context block passes `instinctSummary: null` to agents
- Agents cannot benefit from the 18 instincts that exist in personal YAML files
- Citation tracking cannot work (no IDs to cite)
- The `learn: 0.5` processReward score is stale — per the current rubric (added cycle 21), "no instincts extracted" = 0.0, not 0.5

### Instinct Extraction Stall — CRITICAL (capability-gap signal)
Zero instincts have been extracted since cycle 21. The extraction trigger (step 3b in phases.md) requires `recentZero = evalHistory.slice(-2).every(c => c.instinctsExtracted === 0)`. This condition has been true since cycle 22 but forced extraction never ran. The cycle-22 through cycle-29 work produced 24 tasks across rich domains (bandit selection, counterfactual logging, crossover, novelty reward, decision trace, prerequisite graph, LLM-as-a-Judge, extraction trigger, shared values, token optimization, memory hierarchy, architecture cross-refs). These cycles contain at least 5-8 extractable instincts that are currently unrecorded.

### Meta-Cycle Doc Gap — MEDIUM
`docs/meta-cycle.md` (76 lines) was written in cycle 10 and last updated in cycle 10. It does not reference:
- LLM-as-a-Judge self-evaluation (added cycle 27)
- The link to `docs/self-learning.md` (added cycle 28)
- The correct output format (phases.md Phase 5 has `## Self-Evaluation` heading requirement not reflected in meta-cycle.md)

### Learn Score Stale — LOW
`state.json.processRewards.learn` is `0.5`. The rubric in phases.md defines `learn: 0.5` as "instincts extracted but none cited". With `instinctsExtracted == 0` across all recent cycles, the correct score under the current rubric is `0.0`. The value should be corrected as part of the meta-cycle doc task.

---

## Selected Tasks

### Task 1: Populate instinctSummary in state.json
- **Slug:** populate-instinct-summary
- **Type:** stability
- **Complexity:** S
- **Source:** capability-gap (inst-014 applied — activate-dormant-infrastructure)
- **Rationale:** instinctSummary is referenced throughout all agent definitions and phases.md as the primary compact instinct feed. The field does not exist in state.json. Every cycle since the mechanism was designed (cycle 8) has been running without instinct injection. This is a silent high-impact gap — populating it gives all 18 instincts to every future agent at near-zero token cost. 1 file modified (state.json). Priority boost: +1 (capability-gap).
- **Acceptance Criteria:**
  - [ ] `state.json` contains an `instinctSummary` array with >= 10 entries
  - [ ] Each entry has `id`, `pattern`, `confidence` fields (matching the schema in phases.md)
  - [ ] `inst-007` is present and marked `graduated: true`
  - [ ] `inst-013` is present
  - [ ] `instinctCount` remains at 18 (no regression)
- **Files to modify:** `.claude/evolve/state.json`
- **Eval:** written to `evals/populate-instinct-summary.md`
- **Eval Graders** (inline):
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert 'instinctSummary' in s and len(s['instinctSummary']) >= 10"` → expects exit 0
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); ids=[e['id'] for e in s['instinctSummary']]; assert 'inst-007' in ids and 'inst-013' in ids"` → expects exit 0

### Task 2: Extract instincts from cycles 22-29
- **Slug:** extract-instincts-cycles-22-29
- **Type:** stability
- **Complexity:** S
- **Source:** capability-gap (inst-015 applied — signal-to-action-wiring; the extraction trigger exists but produced no output)
- **Rationale:** Eight cycles of work (bandit task selection, counterfactual annotations, semantic crossover, novelty reward, decision trace, prerequisite graph, LLM-as-a-Judge, instinct extraction trigger, shared values, token optimization, memory hierarchy, architecture cross-refs, session summary card, operator next-cycle brief) contain at least 5-8 extractable procedural and architectural patterns. No instinct file exists for cycles 22-29. This is the exact scenario the extraction trigger was designed to prevent. Writing `cycle-22-29-instincts.yaml` validates the trigger mechanics, restores the learn reward, and gives future cycles actionable patterns from this goal's work.
- **Acceptance Criteria:**
  - [ ] File `.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml` created with >= 5 instincts
  - [ ] Each instinct has `id`, `pattern`, `description`, `confidence`, `source`, `type`, `category` fields
  - [ ] Sources reference cycles 22-29 (e.g., `source: "cycle-27/add-llm-judge-eval-rubric"`)
  - [ ] `state.json.instinctCount` updated to reflect new total (>= 23)
- **Files to modify:** `.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml` (new), `.claude/evolve/state.json`
- **Eval:** written to `evals/extract-instincts-cycles-22-29.md`
- **Eval Graders** (inline):
  - `test -f .claude/evolve/instincts/personal/cycle-22-29-instincts.yaml` → expects exit 0
  - `grep -c "^- id:" .claude/evolve/instincts/personal/cycle-22-29-instincts.yaml | xargs -I{} test {} -ge 5` → expects exit 0
  - `grep -q "source:.*cycle-2[2-9]" .claude/evolve/instincts/personal/cycle-22-29-instincts.yaml` → expects exit 0

### Task 3: Update meta-cycle doc and fix stale learn score
- **Slug:** update-meta-cycle-doc-and-learn-score
- **Type:** techdebt
- **Complexity:** S
- **Source:** meta-cycle (meta-cycle-30 finding — doc stale since cycle 10, missing LLM-as-a-Judge integration)
- **Rationale:** `docs/meta-cycle.md` was last substantively updated in cycle 10. It predates LLM-as-a-Judge self-evaluation (cycle 27), the link to `docs/self-learning.md` (cycle 28), and the current `## Self-Evaluation` output format in phases.md. The doc now misrepresents the meta-cycle process for anyone reading it. Additionally, `state.json.processRewards.learn = 0.5` is stale — correct value per current rubric is `0.0` (no instincts extracted, none cited). Fixing both in one task keeps the doc consistent with the implementation. ~15-20 lines added to meta-cycle.md.
- **Acceptance Criteria:**
  - [ ] `docs/meta-cycle.md` mentions "LLM-as-a-Judge" or "self-evaluation"
  - [ ] `docs/meta-cycle.md` links to `docs/self-learning.md`
  - [ ] `docs/meta-cycle.md` is >= 80 lines (was 76, needs new section)
  - [ ] `state.json.processRewards.learn` corrected from `0.5` to `0.0`
- **Files to modify:** `docs/meta-cycle.md`, `.claude/evolve/state.json`
- **Eval:** written to `evals/update-meta-cycle-doc-and-learn-score.md`
- **Eval Graders** (inline):
  - `grep -q "LLM-as-a-Judge\|self-evaluation" docs/meta-cycle.md` → expects exit 0
  - `grep -q "self-learning.md" docs/meta-cycle.md` → expects exit 0
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert s['processRewards']['learn'] == 0.0, 'learn score not corrected'"` → expects exit 0

---

## Deferred
- Full phases.md rewrite to enforce extraction trigger more explicitly: deferred — phases.md is a hotspot (blast radius), and the extraction trigger text is already correct; the issue is orchestrator compliance, not the spec
- Instinct global promotion sweep (inst-004 at 0.95, inst-007 at 0.9, inst-011 at 0.9): deferred — already in scope for memory consolidation in LEARN phase; adding as a separate task would duplicate work

---

## Decision Trace

```json
{
  "decisionTrace": [
    {
      "slug": "populate-instinct-summary",
      "finalDecision": "selected",
      "signals": ["capability-gap", "dormant-infrastructure", "inst-014-applied", "unblocks-agent-instinct-reading", "novelty+1-state-json-not-touched-since-cycle-27"]
    },
    {
      "slug": "extract-instincts-cycles-22-29",
      "finalDecision": "selected",
      "signals": ["capability-gap", "instinctsExtracted==0-for-7-cycles", "inst-015-applied", "new-file-zero-blast-radius", "validates-extraction-trigger", "novelty+1-new-file"]
    },
    {
      "slug": "update-meta-cycle-doc-and-learn-score",
      "finalDecision": "selected",
      "signals": ["meta-cycle-30-finding", "stale-doc-since-cycle-10", "stale-learn-score", "techdebt", "S-complexity-safe"]
    },
    {
      "slug": "phases-md-extraction-trigger-rewrite",
      "finalDecision": "rejected",
      "signals": ["hotspot-file", "blast-radius-risk", "text-already-correct", "compliance-issue-not-spec-issue"]
    },
    {
      "slug": "instinct-global-promotion-sweep",
      "finalDecision": "deferred",
      "signals": ["duplicates-learn-phase-work", "covered-by-memory-consolidation-cycle30%3"]
    }
  ]
}
```

---

## Mailbox Posts

| from | to | type | cycle | persistent | message |
|------|----|------|-------|------------|---------|
| scout | builder | hint | 30 | false | Task 1 (populate-instinct-summary): modify state.json by adding instinctSummary array. Read all YAML files under .claude/evolve/instincts/personal/ to build the compact entries. Use the schema from phases.md step 4 instinctSummary block. |
| scout | builder | hint | 30 | false | Task 2 (extract-instincts-cycles-22-29): review notes.md cycles 22-29 and the evals/ files for those cycles to find extractable patterns. Focus on: worktree isolation, bandit selection, crossover, novelty reward, prerequisite graphs, LLM-as-a-Judge rubric, shared values KV-cache pattern. |
| scout | builder | hint | 30 | false | Task 3 (update-meta-cycle-doc-and-learn-score): docs/meta-cycle.md is 76 lines. Add a new "## LLM-as-a-Judge Integration" section (~8 lines) and a link to self-learning.md under the Output section. For learn score: set state.json processRewards.learn from 0.5 to 0.0. |
| scout | auditor | hint | 30 | false | Eval graders for Task 1 use python3 with .claude/evolve/state.json path. Verify python3 is available. Use absolute paths if running from worktree. |

