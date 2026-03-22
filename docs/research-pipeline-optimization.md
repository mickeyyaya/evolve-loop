# Research: Pipeline Optimization — Parallelization, Trimming, and Multi-Model Strategies

**Cycle:** 132 | **Date:** 2026-03-22 | **Strategy:** innovate
**Goal:** Investigate whether to trim the pipeline, add multi-model ensembles, or increase parallelization — and what the latest research recommends.

---

## Executive Summary

Three parallel research agents surveyed 25+ papers and production systems from 2025-2026. The consensus is clear: **hybrid approach, leaning toward trim** — fewer sequential phases, same model family, selective parallelism only for hard tasks, and aggressive context compression between steps.

**Key finding:** The evolve-loop's 4-agent architecture is at the empirically-validated sweet spot (Google/MIT's saturation threshold). Adding more agents would **increase** error amplification. The highest-ROI improvements come from **how agents communicate** (context engineering) and **when to parallelize** (adaptive compute allocation), not from adding more agents or mixing providers.

---

## Part 1: Should We Trim the Pipeline?

### Evidence For Trimming

**1. Google/MIT — "Towards a Science of Scaling Agent Systems" (Dec 2025)**
- Evaluated 180 agent configurations across 5 architectures and 3 LLM families
- **Saturation at ~4 agents** — below 4, structured agents help; above 4, coordination overhead consumes benefits
- Unstructured multi-agent networks **amplify errors up to 17.2x** vs single-agent baselines
- **Centralized/hybrid coordination** (which the evolve-loop uses) yields superior scaling efficiency
- Source: [arXiv:2512.08296](https://arxiv.org/abs/2512.08296)

**2. Cognition AI (Devin) — "Don't Build Multi-Agents" (June 2025)**
- Multi-agent is inherently fragile due to context isolation between agents
- Advocates **single-threaded linear agents** for coding, where memory consistency and logical coherence are paramount
- Key principle: **Context Engineering** — agents must share full traces, not isolated messages
- Source: [cognition.ai/blog/dont-build-multi-agents](https://cognition.ai/blog/dont-build-multi-agents)

**3. AgentDiet — Trajectory Reduction (Sep 2025)**
- Removes useless, redundant, and expired information from agent trajectories
- Results: **40-60% input token savings, 21-36% cost reduction** with negligible performance impact
- Categories of waste: irrelevant cache files, repeated file content from edit tools, expired context from past steps
- Source: [arXiv:2509.23586](https://arxiv.org/abs/2509.23586)

### Analysis: What This Means for Evolve-Loop

The evolve-loop has exactly 4 agents (Scout, Builder, Auditor, Operator) — right at the Google/MIT optimal threshold. **We should NOT add agents.** However, we can trim *phases*:

- Phase 0 (CALIBRATE) already runs once per invocation — ✅ already optimized
- Phase 1 (DISCOVER) + Phase 2 (BUILD) are sequential — could the Scout's task selection feed directly into the Builder without a separate phase boundary?
- Phase 4 (SHIP) is inline — ✅ already lean
- Phase 5 (LEARN) + Phase 6 (META-CYCLE) already separated — ✅ already optimized

**Verdict:** The pipeline structure is sound. The main trimming opportunity is **within-phase context compression** (AgentDiet pattern), not phase elimination.

---

## Part 2: Should We Add Multi-Model Ensembles?

### Evidence Against Naive Ensembles

**1. Self-MoA — "Rethinking Mixture-of-Agents" (Princeton, Feb 2025)**
- Using a **single top-performing model** as both proposer and aggregator **outperforms mixed-model MoA by 6.6%**
- Why: mixing different LLMs lowers average quality. Diversity from temperature sampling within one strong model is more effective
- **Self-MoA achieved #1 on AlpacaEval 2.0 leaderboard**
- Source: [arXiv:2502.00674](https://arxiv.org/abs/2502.00674)

**2. Best-of-N Reward Hacking Risk**
- Over-optimizing on proxy rewards degrades true quality
- Mitigations: SRBoN (stochastic regularization, [arXiv:2502.12668](https://arxiv.org/abs/2502.12668)), Self-Certainty scoring ([arXiv:2502.18581](https://arxiv.org/html/2502.18581v1))

### Evidence For Selective Parallel Builds

**1. M1-Parallel (July 2025)**
- Runs multiple multi-agent teams in parallel on the same task
- With early termination: **2.2x speedup with no accuracy loss**
- With aggregation: higher accuracy at cost of latency
- Source: [arXiv:2507.08944](https://arxiv.org/abs/2507.08944)

**2. Test-Time Compute Scaling (ICLR 2025)**
- Scaling inference compute can be **more effective than scaling model parameters**
- But: optimal allocation is **prompt-difficulty-dependent**
- Easy problems: single pass is sufficient
- Hard problems: verification/refinement or parallel sampling helps
- Source: [arXiv:2408.03314](https://arxiv.org/abs/2408.03314)

### Analysis: What This Means for Evolve-Loop

**Do NOT mix LLM providers** (Self-MoA finding). Stick with one model family.

**Do add Self-MoA parallel builds for M-complexity tasks:**
- Spawn 2-3 Builder agents with temperature variation on the same task
- First one that passes eval graders wins (early termination = 2.2x speedup)
- Skip for S-complexity tasks (single pass sufficient)
- This is the highest-ROI ensemble technique: same model, different samples, real verification (eval graders, not proxy rewards)

**Cost:** ~2-3x token usage for M tasks. **Benefit:** Higher first-attempt pass rate + latency reduction.

---

## Part 3: Can More Tasks Be Parallelized Without Sacrificing Quality?

### Evidence: Parallel Execution Improves Quality

**1. Sherlock — Speculative Execution for Agents (Microsoft, Nov 2025)**
- Starts downstream tasks speculatively while verification runs in background
- If verification fails, execution rolls back
- Results: **18.3% accuracy gain** with **48.7% latency reduction**
- Source: [arXiv:2511.00330](https://arxiv.org/abs/2511.00330)

**2. AgentAuditor — Divergence-Point Auditing (Feb 2026)**
- When running multiple agents, builds a Reasoning Tree and audits only at divergence points
- Uses only **973 tokens per sample** (44.8% reduction vs LLM-as-Judge)
- Recovers **65-82% of minority-correct cases** where majority vote fails
- Source: [arXiv:2602.09341](https://arxiv.org/abs/2602.09341)

**3. Anthropic's Parallel Auditing (2025)**
- Runs many auditing agents in parallel in an outer loop
- Achieved **42% solve rate** on alignment auditing tasks
- Source: [alignment.anthropic.com/2025/automated-auditing/](https://alignment.anthropic.com/2025/automated-auditing/)

### Parallelization Opportunities for Evolve-Loop

| Current (Sequential) | Proposed (Parallel) | Expected Impact |
|----------------------|--------------------|-----------------|
| Builder → then Auditor | Builder + Auditor speculative concurrent | 48% latency reduction (Sherlock) |
| One Builder per task | 2-3 Builders with early termination (M tasks only) | 2.2x speedup, higher pass rate (M1-Parallel) |
| Full audit every task | Divergence-point audit when parallel Builders disagree | 45% audit token reduction (AgentAuditor) |
| Independent tasks build sequentially | Already parallel (v7.4.0) | ✅ Already implemented |

**Quality safeguards:**
- Rollback on verification failure (Sherlock pattern)
- Self-certainty scoring as cheap pre-filter before expensive Auditor
- Anti-consensus optimization to recover minority-correct answers (AgentAuditor)

---

## Part 4: Context Engineering Improvements

### Evidence: Biggest ROI Is in Context Management

**1. BATS — Budget-Aware Tool-Use Scaling (Google, Nov 2025)**
- Provides explicit budget signals (remaining tokens, remaining tool calls) to agents
- Agents self-regulate without additional training
- Source: [arXiv:2511.17006](https://arxiv.org/abs/2511.17006)

**2. Anthropic's Four-Strategy Framework (Sep 2025)**
- **Write** (scratchpads), **Select** (relevant memories), **Compress** (pruning), **Isolate** (split contexts)
- The evolve-loop uses Isolate well but has gaps in Select and Compress
- Source: [anthropic.com/engineering/effective-context-engineering-for-ai-agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)

**3. ACE — Agentic Context Engineering (Oct 2025)**
- Treats contexts as evolving playbooks with generation/reflection/curation
- +10.6% on agent tasks vs baselines
- Addresses brevity bias and context collapse
- Source: [arXiv:2510.04618](https://arxiv.org/abs/2510.04618)

**4. OPENDEV — Terminal Agent Architecture (Mar 2026)**
- Eager context building (pre-compute full context at cycle start) beats lazy accumulation
- Enables proactive budget management: measure context size upfront, decide lean mode immediately
- Source: [arXiv:2603.05344](https://arxiv.org/abs/2603.05344)

**5. Token Consumption Prediction (2025)**
- Input tokens dominate cost; once a large file is opened, cost snowballs in every subsequent step
- Token consumption can be predicted before execution, enabling budget-aware routing
- Source: [OpenReview:1bUeVB3fov](https://openreview.net/forum?id=1bUeVB3fov)

### Context Engineering Opportunities for Evolve-Loop

| Gap | Current State | Proposed Improvement | Expected Impact |
|-----|--------------|---------------------|-----------------|
| Budget awareness | Orchestrator tracks cycles, not tokens | Add `budgetRemaining` to agent context | Agents self-regulate effort |
| Context selection | Agents get full state.json | Per-phase context selector (agents get only what they need) | 15-25% token savings |
| Trajectory compression | Lean mode at cycle 4+ | AgentDiet-style compaction between every phase | 30-50% token savings |
| Eager budget estimation | Lean mode triggered reactively | Pre-compute cost estimate at cycle start | Proactive optimization |
| Eval-guided task selection | benchmarkWeaknesses passed to Scout | Add eval-delta prediction to task proposals | Better task ROI |

---

## Part 5: Implementation Plan — Prioritized Tasks

Based on the research, here are the concrete improvements ranked by impact-to-effort ratio:

### Priority 1: Self-MoA Parallel Builds for M-Complexity Tasks
**What:** When a task is M-complexity, spawn 2-3 Builder agents with temperature variation. First to pass eval graders wins (early termination). Skip for S tasks.
**Why:** M1-Parallel shows 2.2x speedup with no accuracy loss. Self-MoA shows single-model diversity outperforms multi-model mixing by 6.6%.
**Where:** `skills/evolve-loop/phase2-build.md` — add Self-MoA dispatch logic
**Expected impact:** 2x latency reduction on M tasks, higher first-attempt pass rate
**Research basis:** M1-Parallel (arXiv:2507.08944), Self-MoA (arXiv:2502.00674)

### Priority 2: Budget-Aware Agent Context
**What:** Add `budgetRemaining` (tokens and cycles) to every agent context block. Agents adapt behavior based on remaining resources.
**Why:** BATS framework shows agents self-regulate without training. Prevents budget overruns that waste cycles.
**Where:** `agents/agent-templates.md` + `skills/evolve-loop/phases.md` context blocks
**Expected impact:** Prevents wasted cycles, enables 20%+ longer sessions within same budget
**Research basis:** BATS (arXiv:2511.17006)

### Priority 3: Per-Phase Context Selection
**What:** Build context selectors that provide each agent only the information it needs. Builder doesn't need benchmark details. Auditor doesn't need research history.
**Why:** Anthropic's Select strategy. Currently agents get ~full state.json.
**Where:** `skills/evolve-loop/phases.md` context block construction
**Expected impact:** 15-25% token savings per agent invocation
**Research basis:** Anthropic Context Engineering (2025)

### Priority 4: Speculative Auditor Execution
**What:** Start Auditor as soon as Builder produces first draft / partial output. Run concurrently.
**Why:** Sherlock shows 48.7% latency reduction + 18.3% accuracy gain from speculative execution.
**Where:** `skills/evolve-loop/phase2-build.md` — add speculative dispatch
**Expected impact:** Cycle latency halved for build+audit sequence
**Research basis:** Sherlock (arXiv:2511.00330)

### Priority 5: Eval-Delta Prediction in Scout
**What:** Each task proposed by Scout includes an expected eval improvement prediction. Phase 5 tracks prediction accuracy.
**Why:** Eval-driven development shows 5x faster shipping. Prediction tracking improves task selection over time.
**Where:** `agents/evolve-scout.md` task output format + `skills/evolve-loop/phase5-learn.md`
**Expected impact:** Better task ROI, faster benchmark improvement
**Research basis:** Anthropic Demystifying Evals, arXiv:2411.13768

### Priority 6: AgentDiet Trajectory Compression
**What:** Between every phase transition, compress the orchestrator's accumulated context using structured distillation (already partially implemented via handoff files).
**Why:** AgentDiet shows 40-60% token savings with negligible quality loss.
**Where:** `skills/evolve-loop/phases.md` phase boundary sections
**Expected impact:** 30-50% token savings per cycle
**Research basis:** AgentDiet (arXiv:2509.23586)

---

## Key Papers Reference Table

| Paper | Year | Key Finding | Relevance to Evolve-Loop |
|-------|------|-------------|-------------------------|
| Google/MIT Scaling Agent Systems | Dec 2025 | 4-agent saturation, 17.2x error amplification risk | Validates our 4-agent architecture |
| Cognition "Don't Build Multi-Agents" | Jun 2025 | Context isolation fragility | Supports lean pipeline approach |
| Self-MoA (Princeton) | Feb 2025 | Single model + temperature > multi-model mixing | Use same model for parallel builds |
| M1-Parallel | Jul 2025 | 2.2x speedup with early termination | Self-MoA parallel builds |
| Sherlock (Microsoft) | Nov 2025 | 48.7% latency reduction via speculative execution | Speculative auditor |
| AgentAuditor | Feb 2026 | 44.8% audit token reduction via divergence-point auditing | Efficient parallel audit |
| AgentDiet | Sep 2025 | 40-60% token savings via trajectory reduction | Context compression |
| BATS (Google) | Nov 2025 | Budget-aware self-regulation | Add budgetRemaining to context |
| Anthropic Context Engineering | Sep 2025 | Write/Select/Compress/Isolate framework | Per-phase context selection |
| ACE | Oct 2025 | +10.6% via evolving context playbooks | Strategy playbook refinement |
| OPENDEV | Mar 2026 | Eager context building beats lazy | Pre-compute context at cycle start |
| Test-Time Compute Scaling | 2025 | Difficulty-adaptive compute allocation | Self-MoA only for hard tasks |
| Eval-Driven Development | 2025 | 5x faster shipping with eval guidance | Eval-delta prediction |

---

## Verdict

**Don't trim agents** — 4 is the sweet spot per Google/MIT research.
**Don't mix providers** — Self-MoA with one strong model beats multi-model ensembles.
**Do add Self-MoA parallel builds** — for M-complexity tasks only (adaptive compute allocation).
**Do compress context aggressively** — AgentDiet-style pruning between phases is the highest-ROI change.
**Do add budget awareness** — BATS-style signals let agents self-regulate.
**Do start Auditor speculatively** — Sherlock pattern halves build+audit latency.
