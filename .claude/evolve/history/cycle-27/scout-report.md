# Cycle 27 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 8 (phases.md, SKILL.md, memory-protocol.md, eval-runner.md, evolve-auditor.md, instinct files, state.json, project-digest.md)
- Research: performed (cooldown expired — prior queries from 2026-03-15)
- Instincts applied: 2
- **instinctsApplied:** [inst-013 (guided progressive-disclosure approach for shared values header design), inst-005 (confirmed docs-lag pattern — memory-protocol.md lags new architectural knowledge)]

## Key Findings

### Instinct Extraction Stall — CRITICAL
- `instinctsExtracted == 0` for cycles 22–26 (5 consecutive zeros, threshold is 2+)
- Most recent instinct file: `cycle-9-instincts.yaml`
- 60+ tasks shipped since cycle 9 with zero learning extraction
- `learn: 0.5` process reward sustained below 0.7 for every cycle in evalHistory
- Root cause: Phase 5 LEARN relies on passive extraction; uniform success produces no gradient signal
- Research confirms: MemRL (Jan 2026) and MemEvolve (Dec 2025) show passive extraction fails under uniform success — active forcing functions are required

### Self-Evaluation Rubric Gap — HIGH
- Auditor produces PASS/WARN/FAIL but no structured per-dimension quality scoring
- `processRewards` scores exist but rubric for deriving them is informal
- No LLM-as-a-Judge chain-of-thought justification before scoring
- Research consensus: binary scoring per dimension + CoT justification produces actionable learning signals

### Shared Values Not Formalized — MEDIUM
- Each agent independently reads SKILL.md (~3.5K tokens), phases.md (~8.7K), memory-protocol.md (~3.2K)
- No compact "team constitution" that can be injected cheaply at agent context start
- Token optimization research: KV-cache hit rate maximized when stable shared content precedes dynamic task context
- memory-protocol.md last touched cycle 8 (19 cycles ago) — novelty boost applies

## Research Findings

### AI Agent Self-Evaluation (LLM-as-a-Judge)
- Binary scoring per dimension is most reliable (binary > Likert for LLM judges)
- Split criteria: separate judges per dimension (correctness, completeness, novelty, efficiency)
- Chain-of-thought before verdict improves accuracy and debuggability
- Same-model self-evaluation has self-enhancement bias — mitigated in evolve-loop by separate agent contexts
- Sources: EvidentlyAI LLM-as-a-Judge guide, Amazon agentic evaluation lessons

### Token Optimization in Multi-Agent Systems
- Multi-agent systems use 15x more tokens than single-agent chats; 1.5x–7x duplication common
- KV-cache hit rate is primary optimization lever — static shared rules should precede dynamic context
- Coordinated forgetting (discard noise, preserve critical shared state) reduces retrieval latency
- Source: MongoDB Memory Engineering, Vellum Context Engineering

### Agent Memory Architecture
- Episodic → Semantic abstraction: key 2025 insight — trace stored as episodic first, background process abstracts to semantic (generalizable skill/rule)
- Hierarchical Procedural Memory (arxiv 2512.18950): Bayesian selection + contrastive refinement from success/failure
- The evolve-loop's passive Phase 5 extraction is the missing link between episodic cycle history and semantic instinct extraction
- Sources: arxiv 2502.06975 (Episodic Memory), arxiv 2512.18950 (Hierarchical Procedural Memory), arxiv 2502.12110 (A-MEM)

## Selected Tasks

### Task 1: add-llm-judge-eval-rubric
- **Slug:** add-llm-judge-eval-rubric
- **Type:** feature
- **Complexity:** S
- **Rationale:** The `learn: 0.5` sustained score and 5-cycle instinct stall trace to lacking structured per-dimension quality scoring in Phase 5. Adding a 4-dimension judge rubric (correctness, completeness, novelty, efficiency) with mandatory CoT justification creates the gradient signal that passive extraction lacks. Directly advances the goal (self-evaluation) and fixes the immediate root cause.
- **Acceptance Criteria:**
  - [ ] Phase 5 LEARN contains a structured rubric with at least 3 named dimensions
  - [ ] Each dimension definition includes how to score 0.0 to 1.0
  - [ ] Chain-of-thought justification is required before assigning each score
  - [ ] Low scores on any dimension trigger an instinct extraction obligation
- **Files to modify:** `skills/evolve-loop/phases.md` (S-complexity: ~20 lines added to Phase 5 LEARN)
- **Eval:** written to `evals/add-llm-judge-eval-rubric.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "LLM-as-a-Judge\|llm-judge\|self-evaluation\|judge rubric" skills/evolve-loop/phases.md` → expects exit 0 (match >= 1)
  - `grep -c "chain-of-thought\|step-by-step\|justify.*score\|justification" skills/evolve-loop/phases.md` → expects exit 0 (match >= 1)
  - `grep -c "correctness\|completeness\|novelty\|efficiency" skills/evolve-loop/phases.md` → expects exit 0 (match >= 2)

### Task 2: add-instinct-extraction-trigger
- **Slug:** add-instinct-extraction-trigger
- **Type:** feature
- **Complexity:** S
- **Rationale:** Capability gap: the instinct extraction pipeline has been silent for 5 cycles. This task adds an active forcing function to Phase 5 — when `instinctsExtracted == 0` for 2+ cycles, the orchestrator MUST run an explicit extraction prompt covering approach used, audit outcome, and future agent recommendation. This is the core mechanism from MemRL (Jan 2026) applied directly to the evolve-loop's stall.
- **Acceptance Criteria:**
  - [ ] Phase 5 LEARN contains an explicit trigger checking `instinctsExtracted == 0` for 2+ consecutive cycles
  - [ ] The trigger defines a structured extraction prompt with at least 3 named questions
  - [ ] The trigger is active even when all evals pass (not conditional on failure)
  - [ ] The extracted instincts are written to the cycle's instinct file
- **Files to modify:** `skills/evolve-loop/phases.md` (S-complexity: ~15 lines added to Phase 5 trigger conditions)
- **Eval:** written to `evals/add-instinct-extraction-trigger.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "instinctsExtracted\|instinct.*extraction.*trigger\|extraction.*stall\|force.*extraction" skills/evolve-loop/phases.md` → expects exit 0 (match >= 1)
  - `grep -c "consecutive\|2.*cycle\|back-to-back" skills/evolve-loop/phases.md` → expects exit 0 (match >= 1)

### Task 3: add-shared-values-protocol
- **Slug:** add-shared-values-protocol
- **Type:** feature
- **Complexity:** S
- **Rationale:** Formalizes shared behavioral rules as a compact Layer 0 section in memory-protocol.md. Three benefits: (1) gives parallel agents a stable shared constitution, (2) positions static rules at the top of context for KV-cache optimization, (3) adds a "learning mandate" rule that directly addresses the extraction stall. memory-protocol.md has not been touched in 19 cycles — strong novelty boost.
- **Acceptance Criteria:**
  - [ ] `memory-protocol.md` contains a "Layer 0" or "Shared Values" section before Layer 1
  - [ ] Section contains at least 3 named behavioral rules
  - [ ] At least one rule addresses learning/instinct extraction obligation
  - [ ] Section notes token efficiency rationale (static-first for KV-cache)
- **Files to modify:** `skills/evolve-loop/memory-protocol.md` (S-complexity: ~20 lines added at top)
- **Eval:** written to `evals/add-shared-values-protocol.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "Layer 0\|Shared Values\|Core Values\|Team Constitution\|shared.*rules" skills/evolve-loop/memory-protocol.md` → expects exit 0 (match >= 1)
  - `grep -c "must\|never\|always" skills/evolve-loop/memory-protocol.md` → expects exit 0 (match >= 3)

## Deferred

- **add-parallel-builder-execution**: M+ complexity, phases.md surgery >80 lines exceeds hotspot budget. Prerequisite: `add-shared-values-protocol`.
- **add-bayesian-plancache-ranking**: Requires state.json schema change + SKILL.md edits. Oversized for current cycle goal focus.
- **episodic-to-semantic-full-pipeline**: Full MemRL-style abstraction pipeline is M complexity. Partial progress via `add-instinct-extraction-trigger`.

## Decision Trace

```json
{
  "decisionTrace": [
    {
      "slug": "add-llm-judge-eval-rubric",
      "finalDecision": "selected",
      "signals": ["introspection: instinctsExtracted==0 for 5 cycles", "goal-alignment: self-evaluation", "S-complexity: phases.md hotspot kept minimal", "novelty+1: structured judge rubric not previously in phases.md"]
    },
    {
      "slug": "add-instinct-extraction-trigger",
      "finalDecision": "selected",
      "signals": ["capability-gap: instinctsExtracted==0 stall", "pendingImprovement: learn:0.5 sustained below 0.7 for 5 cycles", "S-complexity: 15 lines to phases.md trigger section"]
    },
    {
      "slug": "add-shared-values-protocol",
      "finalDecision": "selected",
      "signals": ["goal-alignment: parallel agent shared rules/values", "novelty+1: memory-protocol.md last touched cycle 8 (delta=19)", "token-optimization: KV-cache static-first pattern from research"]
    },
    {
      "slug": "add-parallel-builder-execution",
      "finalDecision": "deferred",
      "signals": ["too-large: phases.md surgery >80 lines", "prerequisite: add-shared-values-protocol", "deferralReason: M+ complexity, exceeds hotspot S-limit this cycle"]
    },
    {
      "slug": "add-bayesian-plancache-ranking",
      "finalDecision": "deferred",
      "signals": ["oversized: state.json schema + SKILL.md edits", "deferralReason: exceeds S complexity budget"]
    }
  ]
}
```
