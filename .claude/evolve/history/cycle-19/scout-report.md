# Cycle 19 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 8 (changedFiles from cycle 18 + instinct files)
- Research: performed (cooldown expired — last queries at 2026-03-14T00:00:00Z, now 2026-03-14)
- Instincts applied: 2 (inst-013 progressive-disclosure, inst-014 activate-dormant-infrastructure)

---

## Key Findings

### Features — HIGH
- **Instinct-to-behavior gap:** 14 instincts are extracted and stored as YAML, but no mechanism converts high-confidence instincts into task proposals. inst-013 (progressive-disclosure, 0.6) and inst-014 (activate-dormant-infrastructure, 0.6) have been sitting dormant since cycles 17-18 with no acted-upon cycle. The instinct system learns passively but cannot yet trigger active change.
- **processRewards is signal-without-action:** The `skillEfficiency` reward dimension was added in cycle 18, and the full scoring rubric exists in phases.md. But rewards are only reviewed during meta-cycles (every 5 cycles). A `learn: 0.8` score right now generates no immediate remediation. There is no per-cycle feedback path from reward scores to task proposals.
- **Scout discovery is purely reactive:** Codebase analysis + web search drives task selection, but the loop has no introspective pass over its own `evalHistory` to propose improvements to itself. If `instinctsExtracted` has been 0 for 3+ cycles, or `auditIterations > 1.0` consistently, there is no automated path to detect and fix this.

### Architecture — MEDIUM
- The `processRewards` field in state.json stores only the latest cycle's scores (single object, not a history). This makes it impossible to detect sustained degradation vs a one-off dip. A rolling window of the last 3 cycles' rewards would support trend detection.

---

## Research

- **"self-improving AI agent systems autonomous learning 2025 2026"**: Self-evolving agents bridge static foundation models with lifelong adaptability. The key mechanism is a feedback loop where agents capture failures, evaluate outputs against criteria, and use that signal to update behavior. The Self-Challenging Agent (SCA) pattern alternates challenger/executor roles with self-fine-tuning. (source: https://arxiv.org/abs/2508.07407)

- **"meta-learning agent prompt evolution self-modification LLM"**: OpenAI cookbook shows a meta-prompting loop: original prompt + failed output + grader feedback → optimized prompt version. Triggers on grader scores below 0.85 threshold. Key insight: **grader scores function as decision signals**, not just metrics. Versioned prompt history ensures auditability. GEPA (reflective prompt evolution) can outperform reinforcement learning for agent improvement. (source: https://developers.openai.com/cookbook/examples/partners/self_evolving_agents/autonomous_agent_retraining)

**Key synthesis:** The evolve-loop already has all the infrastructure: `processRewards` (grader scores), `instinctSummary` (learned patterns), `evalHistory` with delta metrics (execution trace). What's missing is the **signal-to-action wiring** — automated paths from these signals to task proposals and behavior modification.

---

## Selected Tasks

### Task 1: processRewards Per-Cycle Remediation Loop
- **Slug:** `add-process-rewards-remediation-loop`
- **Type:** feature
- **Complexity:** S
- **Rationale:** `processRewards` scores are computed every cycle but reviewed only at meta-cycles (every 5). This 5-cycle lag means a sustained `learn: 0.8` or `build: 0.5` goes unaddressed for multiple cycles. Adding a per-cycle check — "if any reward < 0.7, log a remediation action to `state.json.pendingImprovements`" — creates a tight feedback loop. The Scout's next cycle context includes `pendingImprovements` as priority task candidates. This is the minimal wiring needed to turn process reward signals into automatic self-improvement actions. Directly advances the goal: self-improving efficiency through each cycle.
- **Acceptance Criteria:**
  - [ ] phases.md Phase 4 contains a `pendingImprovements` update step: if any processReward dimension < 0.7, append a structured remediation entry to state.json
  - [ ] state.json schema (memory-protocol.md) documents the `pendingImprovements` field with its schema
  - [ ] Scout agent (evolve-scout.md) reads `pendingImprovements` from context and treats non-empty entries as high-priority task candidates
  - [ ] The remediation entry schema includes: `dimension`, `score`, `suggestedTask`, `cycle`, `priority`
- **Files to modify:**
  - `/Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` (Phase 4 update step)
  - `/Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` (schema docs)
  - `/Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` (read pendingImprovements from context)
- **Eval:** written to `evals/add-process-rewards-remediation-loop.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects at least 1 match
  - `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects at least 1 match
  - `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 1 match
  - `grep -n "dimension.*score.*suggestedTask\|suggestedTask.*dimension\|score.*dimension" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects at least 1 match OR verify schema fields are documented inline

---

### Task 2: Scout Introspective Self-Proposal Step
- **Slug:** `add-scout-introspective-self-proposal`
- **Type:** feature
- **Complexity:** M
- **Rationale:** Currently Scout discovers tasks by scanning the codebase and searching the web. It has no step where it looks inward at the loop's own execution history (`evalHistory` delta metrics) and proposes improvements to the pipeline itself. Research shows that self-evolving systems require closed-loop feedback from their own execution traces. This task adds an "Introspection Pass" to Scout's responsibilities: review the last 3 cycles' delta metrics from `evalHistory` and `pendingImprovements`, then generate pipeline self-improvement task candidates. Specific rules: if `instinctsExtracted == 0` for 2+ consecutive cycles → propose instinct-enrichment task; if avg `auditIterations > 1.2` → propose Builder guidance task; if `stagnationPatterns > 0` → propose task diversity task. This closes the self-building capability gap.
- **Acceptance Criteria:**
  - [ ] evolve-scout.md has an "Introspection Pass" step in Responsibilities section (after Codebase Analysis, before Task Selection)
  - [ ] The introspection pass defines at least 3 concrete self-improvement heuristics with specific thresholds (instinctsExtracted, auditIterations, stagnationPatterns)
  - [ ] Introspection-proposed tasks are labeled with `source: "introspection"` in the scout report
  - [ ] The step reads `stateJson.evalHistory` delta metrics (already passed in context — no additional file reads needed)
  - [ ] The step also reads `stateJson.pendingImprovements` if present
- **Files to modify:**
  - `/Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` (new Introspection Pass section)
- **Eval:** written to `evals/add-scout-introspective-self-proposal.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -n "Introspection Pass\|introspection pass" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 1 match
  - `grep -n "instinctsExtracted\|auditIterations\|stagnationPatterns" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 3 matches
  - `grep -n "source.*introspection\|introspection.*source" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 1 match
  - `grep -c "heuristic\|threshold\|>.*cycle\|consecutive" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects count >= 2

---

### Task 3: processRewards History Window in State Schema
- **Slug:** `add-process-rewards-history`
- **Type:** feature
- **Complexity:** S
- **Rationale:** The current `processRewards` field stores only the latest cycle's scores (flat object). This makes trend detection impossible — there's no way to distinguish a sustained `learn: 0.7` (chronic problem) from a one-off dip. The meta-cycle review and remediation loop both need to detect *sustained* degradation, not just single-cycle scores. This task changes the schema to store a rolling window of 3 cycles' process rewards. This is minimal (schema change + write logic update), and is prerequisite for the remediation loop in Task 1 to have trend data. Directly enables self-improving: the loop can now see its own efficiency trend.
- **Acceptance Criteria:**
  - [ ] memory-protocol.md `processRewards` schema updated to show rolling array format with `cycle` and dimension scores per entry
  - [ ] phases.md Phase 4 update step writes to `processRewardsHistory` array (not flat object), keeping last 3 entries
  - [ ] The remediation threshold logic from Task 1 references `processRewardsHistory` for trend detection (sustained low = 2+ consecutive cycles below threshold)
- **Files to modify:**
  - `/Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` (schema update)
  - `/Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` (write logic update)
- **Eval:** written to `evals/add-process-rewards-history.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -n "processRewardsHistory\|rewardsHistory" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects at least 1 match
  - `grep -n "processRewardsHistory\|rewardsHistory" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects at least 1 match
  - `grep -n "rolling\|last 3\|window\|trim.*3\|3 entries" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects at least 1 match

---

## Deferred
- **Full instinct-to-task-proposal system** (L complexity): Automatically converting instincts with confidence >= 0.7 that haven't been acted on into task candidates requires instinct lifecycle tracking (an "acted on" flag or cycle counter per instinct). Deferred — this is a follow-on to the introspection pass in Task 2, which covers the most common case via delta metrics.
- **EvoPrompt / Promptbreeder-style automated prompt mutation** (L complexity): Evolving agent prompts using evolutionary algorithms (crossover, mutation, selection) across cycles requires significant new infrastructure (prompt version store, tournament selection logic). Deferred to a dedicated goal cycle.
- **Island model activation** (L complexity): docs/island-model.md is referenced in SKILL.md but the mechanism is dormant. Activating parallel configurations requires worktree branching and comparison infrastructure. Per inst-014, this is a candidate for activation, but complexity exceeds this cycle's budget.
