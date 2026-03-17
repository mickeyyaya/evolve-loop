# Cycle 28 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 8 (phases.md, memory-protocol.md, SKILL.md, architecture.md, docs/*.md, state.json)
- Research: skipped (no-research directive in context; cached research from cycle 27 sufficient)
- Instincts applied: 2
- **instinctsApplied:** [inst applied via builderNotes: phases.md hotspot guidance; inst applied via evalHistory: instinctsExtracted==0 for 6 consecutive cycles — active capability gap signal]

## Key Findings

### Goal Progress — HIGH
- Self-evaluation: DONE (LLM-as-a-Judge rubric, cycle 27)
- Instinct extraction stall fix: DONE (forcing function, cycle 27)
- Shared values / parallel coordination: DONE (Layer 0 protocol, cycle 27)
- Token optimization: PARTIAL — architecture.md has a 20-line section but no standalone reference doc with full pattern coverage
- Agent memory architecture: PARTIAL — memory-protocol.md documents the schema but lacks a reader-friendly guide showing how all 6 layers interrelate and how episodic→semantic abstraction works end-to-end
- Self-learning skill: NOT STARTED — the biggest remaining deliverable; all mechanisms exist but are scattered across phases.md, architecture.md, SKILL.md with no unified reference

### Code Quality — MEDIUM
- phases.md at 766 lines (Builder-flagged threshold: 700). Builder notes recommend extracting standalone sections to linked docs rather than adding more content inline. This is consistent with selecting new docs/ files as targets for this cycle rather than extending phases.md.

### Capability Gap — MEDIUM
- instinctsExtracted == 0 for cycles 22-27 (6 consecutive cycles). The extraction trigger added in cycle 27 is a forcing function at Phase 5 runtime — it doesn't retroactively fix the gap. The gap signals that uniform success (100% ship rate) reduces the informational signal available for extraction. No new task is proposed for this — the Phase 5 forcing function should handle it going forward; the signal is noted for context.

## Research
- Skipped. Goal is documentation of existing mechanisms. No external knowledge needed.

## Selected Tasks

### Task 1: Add Token Optimization Doc
- **Slug:** add-token-optimization-doc
- **Type:** feature
- **Complexity:** S
- **Rationale:** Goal gap. No standalone token optimization reference exists. architecture.md has a 20-line stub; users and agents need a comprehensive reference covering all patterns: model routing, KV-cache prefix sharing, context compression, instinct summaries, plan caching, incremental scan, research cooldown, worktree isolation, and the token budget system. S complexity: new file, no existing file modified.
- **Acceptance Criteria:**
  - [ ] `docs/token-optimization.md` created with 5+ sections
  - [ ] Covers model routing (haiku/sonnet/opus per task type)
  - [ ] Covers KV-cache with Layer 0 shared values as prefix optimization
  - [ ] Covers instinct summary, plan cache, incremental scan, research cooldown patterns
  - [ ] Covers token budget schema (perTask/perCycle) with concrete numbers
- **Files to modify:** `docs/token-optimization.md` (new)
- **Eval:** written to `evals/add-token-optimization-doc.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `test -f /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md` → expects exit 0
  - `grep -c "model.routing\|model routing\|haiku\|sonnet\|opus" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md` → expects ≥1
  - `grep -c "KV.cache\|kv.cache\|prompt.cache\|cache.hit" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md` → expects ≥1
  - `grep -c "instinct.summar\|plan.cache\|incremental.scan\|research.cooldown" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md` → expects ≥1

### Task 2: Add Self-Learning Skill Doc
- **Slug:** add-self-learning-skill-doc
- **Type:** feature
- **Complexity:** M
- **Rationale:** The primary deliverable of the goal. All seven self-improvement mechanisms exist (process rewards, Scout introspection, process rewards history, bandit task selector, semantic crossover, intrinsic novelty reward, Scout decision trace) plus the LLM-as-a-Judge and instinct extraction trigger added in cycle 27. None of these have a single unified document. This doc becomes the "how does this system learn?" reference — describing the full self-learning loop from observation to instinct to policy to promotion, with all seven mechanisms and how they interconnect. M complexity: new file, potentially references 3-4 existing files to ensure accuracy.
- **Acceptance Criteria:**
  - [ ] `docs/self-learning.md` created with comprehensive coverage
  - [ ] Documents all 7 self-improvement mechanisms from architecture.md
  - [ ] Documents instinct lifecycle: extraction → confidence scoring → promotion → consolidation
  - [ ] Documents LLM-as-a-Judge feedback loop
  - [ ] Documents bandit arm learning and how it feeds task selection
  - [ ] Includes a "how it all connects" diagram or flow section
- **Files to modify:** `docs/self-learning.md` (new)
- **Eval:** written to `evals/add-self-learning-skill-doc.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `test -f /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md` → expects exit 0
  - `grep -c "instinct\|Instinct" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md` → expects ≥5
  - `grep -c "bandit\|Bandit\|reward\|Reward" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md` → expects ≥3
  - `grep -c "LLM-as-a-Judge\|llm.judge\|self.eval" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md` → expects ≥1
  - `grep -c "consolidat\|episodic\|semantic\|procedural" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md` → expects ≥3

### Task 3: Add Memory Hierarchy Doc
- **Slug:** add-memory-hierarchy-doc
- **Type:** feature
- **Complexity:** S
- **Rationale:** Goal gap — "agent memory best practices" is PARTIAL. memory-protocol.md is a technical schema reference; what's missing is a reader-friendly architecture guide showing how all 6 layers interrelate, how memory traversal works (which agents read which layers, when), and the episodic→semantic abstraction cycle. This doc would also serve as a reusable reference for anyone building agent memory systems on top of this codebase. S complexity: new file, reads memory-protocol.md for accuracy.
- **Acceptance Criteria:**
  - [ ] `docs/memory-hierarchy.md` created with layer-by-layer architecture
  - [ ] Covers all 6 layers (0: values, 1: ledger, 2: workspace, 3: state, 4: evals, 5: instincts) plus Layer 6 (experiment journal)
  - [ ] Describes episodic→semantic abstraction pathway
  - [ ] Describes memory consolidation cycle (every 3 cycles)
  - [ ] Includes a "which agent reads which layer" access matrix
- **Files to modify:** `docs/memory-hierarchy.md` (new)
- **Eval:** written to `evals/add-memory-hierarchy-doc.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `test -f /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md` → expects exit 0
  - `grep -c "Layer [0-9]\|Layer [0-9]:" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md` → expects ≥4
  - `grep -c "episodic\|semantic\|procedural" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md` → expects ≥3
  - `grep -c "consolidat\|abstraction\|promotion" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md` → expects ≥2

## Deferred
- Parallel agent execution mechanism: deferred — existing shared values protocol covers coordination rules; a runtime parallel execution feature would require changes to the orchestrator invocation model (phases.md L-complexity territory). Revisit after phases.md is refactored.
- phases.md extraction/refactor: deferred — Builder recommended it but phases.md is stable and well-anchored. The current cycle targets new docs/ files which avoids touching the hotspot entirely, consistent with blast-radius-awareness instinct.

## Decision Trace

```json
{
  "decisionTrace": [
    {
      "slug": "add-token-optimization-doc",
      "finalDecision": "selected",
      "signals": ["goal-gap", "novelty+1 (new file, no lastTouchedCycle)", "S-complexity fits within budget"]
    },
    {
      "slug": "add-self-learning-skill-doc",
      "finalDecision": "selected",
      "signals": ["goal-primary-deliverable", "novelty+1 (new file)", "M-complexity fits within budget"]
    },
    {
      "slug": "add-memory-hierarchy-doc",
      "finalDecision": "selected",
      "signals": ["goal-gap", "novelty+1 (new file)", "S-complexity fits within budget", "capability-gap (memory architecture partial)"]
    },
    {
      "slug": "add-parallel-execution-mechanism",
      "finalDecision": "deferred",
      "signals": ["phases.md hotspot risk", "L-complexity runtime change", "deferralReason: requires orchestrator model changes beyond current cycle scope"]
    },
    {
      "slug": "phases-md-extraction-refactor",
      "finalDecision": "deferred",
      "signals": ["blast-radius high", "stable file", "deferralReason: no new approach; current cycle avoids the hotspot entirely"]
    }
  ]
}
```

---

## Mailbox Post (for Builder)

```markdown
| from  | to      | type | cycle | persistent | message                                                                                      |
|-------|---------|------|-------|------------|----------------------------------------------------------------------------------------------|
| scout | builder | hint | 28    | false      | All 3 tasks target new docs/ files — no existing files modified. Zero blast radius.         |
| scout | builder | hint | 28    | false      | Read architecture.md Self-Improvement section (lines 123-168) before writing self-learning.md — all 7 mechanisms are documented there. |
| scout | builder | hint | 28    | false      | Read memory-protocol.md Layers 0-6 before writing memory-hierarchy.md — use it as ground truth. |
| scout | auditor | hint | 28    | false      | Evals use test -f and grep -c; all target absolute paths. Verify file exists before grep.   |
```

---

## Ledger Entry
```json
{"ts":"2026-03-17T10:30:00Z","cycle":28,"role":"scout","type":"discovery","data":{"scanMode":"incremental","filesAnalyzed":8,"researchPerformed":false,"tasksSelected":3,"instinctsApplied":2}}
```
