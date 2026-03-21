# Research Applied: Context Window Optimization for Autonomous Agent Loops

A comprehensive report documenting what we learned from published research, how each finding was applied to the evolve-loop, and what the results mean. Written for humans who want to understand the tradeoffs between token efficiency and output quality in multi-agent systems.

**Date:** March 21, 2026
**Cycles:** 112-114
**Research corpus:** 30+ papers from arXiv (2024-2026), Anthropic safety research, NeurIPS/WSDM/EMNLP proceedings

---

## Table of Contents

1. [The Core Problem](#1-the-core-problem)
2. [What the Research Says](#2-what-the-research-says)
3. [What We Applied and Why](#3-what-we-applied-and-why)
4. [The Quality-Cost Tradeoff](#4-the-quality-cost-tradeoff)
5. [Technique-by-Technique Deep Dive](#5-technique-by-technique-deep-dive)
6. [What We Chose NOT to Implement](#6-what-we-chose-not-to-implement)
7. [Quantitative Impact Summary](#7-quantitative-impact-summary)
8. [Lessons Learned](#8-lessons-learned)
9. [References](#9-references)

---

## 1. The Core Problem

An autonomous coding loop like evolve-loop runs multiple agents (Scout, Builder, Auditor, Operator) across multiple phases per cycle, and multiple cycles per session. Each agent call sends a prompt to an LLM and receives a response. The cost and latency scale with the number of tokens in that prompt.

**The token snowball effect** (Fan et al., arXiv:2509.09853): In multi-turn agent interactions, tokens accumulate quadratically. Each turn adds new content to the conversation context, and every subsequent turn must re-process all previous content. A loop that runs 2x too many turns costs roughly 4x as many tokens.

In practice, the evolve-loop's orchestrator context grows ~40-60K tokens per cycle. By cycle 6, this hits ~300K+ tokens, causing:
- Progressive slowdown (each cycle takes longer than the last)
- Increased cost per cycle even for similar-complexity tasks
- Quality degradation as relevant information gets diluted in noise

**The research question:** How do we reduce token usage without losing the quality of the work the agents produce?

---

## 2. What the Research Says

We surveyed 30+ papers published between 2024-2026. The key findings cluster into five themes:

### Theme 1: Phase Isolation Beats Monolithic Context
**Key paper:** CoDA (Liu et al., arXiv:2512.12716, WSDM '26 Oral)

When a planner and an executor share the same context window, the executor's tool-call outputs pollute the planner's reasoning space. CoDA showed that **decoupling planning from execution** — giving each its own isolated context with only structured handoffs between them — maintains stable performance even as task length grows, while monolithic approaches degrade severely.

**What this means:** Each phase of the evolve-loop should get a fresh, minimal context. The Scout's raw file-scanning output should never appear in the Builder's context. Instead, a compact handoff summarizes what the Scout found.

### Theme 2: Compression Works If You Keep the Right Things
**Key paper:** Focus Agent (Verma, arXiv:2601.07190)

The Focus Agent achieved **22.7% token reduction** on SWE-bench while maintaining solve rate within 1.2% of the uncompressed baseline. The technique: autonomously consolidate learnings into persistent "Knowledge" blocks, pruning raw interaction history.

A complementary paper — Structured Distillation (Lewis, arXiv:2603.13017) — showed **11x compression** (371 → 38 tokens per exchange) with **96% retrieval quality** by distilling conversations into 4-field compound objects.

**The critical insight:** Not all tokens are equal. A 500-token tool output might contain only 38 tokens of actually useful information. The question isn't "can we compress?" but "what do we compress away?"

### Theme 3: Turn Budgets Are More Effective Than Token Limits
**Key paper:** Turn-Control (Gao & Peng, arXiv:2510.16786)

Tested three strategies on SWE-bench:
1. Unrestricted baseline
2. Fixed-turn limit at 75th percentile + reminder messages
3. Dynamic-turn: on-demand extensions only when justified

Results:
- Fixed limits: **24-68% cost reduction** with minimal solve-rate impact
- Dynamic extensions: **additional 12-24% savings** beyond fixed limits

**Why turns matter more than tokens:** A token budget is a lagging indicator — by the time you hit it, you've already wasted the tokens. A turn budget is a leading indicator — it forces the agent to plan before acting.

### Theme 4: Model Routing Saves 50-70% Without Quality Loss
**Key paper:** MasRouter (Yue et al., arXiv:2502.11133)

Intelligent routing — sending each sub-task to the cheapest model that can handle it — achieved **52-70% cost reduction** with accuracy improvements of 1.8-8.2%.

**The non-obvious finding:** Routing actually *improved* quality in some cases, because simpler models are less likely to overthink straightforward tasks.

### Theme 5: Graph Navigation Beats Brute-Force Context
**Key papers:** RepoMaster (Wang et al., arXiv:2505.21577), GraphReader (Li et al., arXiv:2406.14550)

RepoMaster: **95% token reduction** on repository exploration by building dependency graphs. GraphReader: a **4K context window with graph navigation outperforms a 128K context** reading raw text.

**Why this matters:** Most tasks in a codebase are local — they touch 3-7 files even in a 500-file repo. Loading all 500 files to find the right 5 is a 100x waste.

---

## 3. What We Applied and Why

We implemented changes across cycles 112-114, organized by priority. Each change was selected based on three criteria:
1. **Impact** — expected token/cost savings from the research
2. **Quality preservation** — must maintain or improve output quality
3. **Implementation feasibility** — must work as prompt instructions + bash scripts (no custom training)

### Cycle 112: Quick Wins (Phase 1)

| Change | File Modified | Research Basis | Expected Savings |
|--------|--------------|----------------|-----------------|
| Structured inter-phase handoff format | `phases.md` | CoDA, InfiAgent | 40-60% per cycle |
| Dynamic turn budgets per phase | `token-optimization.md` | Turn-Control | 24-68% cost |
| Active context compression pattern | `token-optimization.md` | Focus Agent | 22.7% overall |

### Cycle 113: Medium-Term (Phase 2)

| Change | File Modified | Research Basis | Expected Savings |
|--------|--------------|----------------|-----------------|
| Structured memory distillation format | `phase5-learn.md`, `token-optimization.md` | Structured Distillation | 11x on history |
| Concrete model routing with quality guardrails | `token-optimization.md` | MasRouter | 52-70% cost |
| Graph-based code exploration for Scout | `graph-exploration.md` (new) | RepoMaster, GraphReader | 95% on exploration |

---

## 4. The Quality-Cost Tradeoff

**This is the most important section.** Every optimization technique creates a tension between saving tokens and preserving quality. Here's how each technique handles this tension:

### Techniques That Are Quality-Neutral (No Tradeoff)

| Technique | Why No Quality Loss |
|-----------|-------------------|
| Prompt caching | Identical tokens, just served from cache |
| Lazy tool loading | Removes irrelevant tool descriptions, not relevant ones |
| KV-cache prefix ordering | Same content, different ordering for cache efficiency |

### Techniques That Improve Quality *AND* Save Tokens

| Technique | Why Quality Improves | Evidence |
|-----------|---------------------|----------|
| Phase isolation | Removes noise that dilutes relevant context | CoDA: maintained performance where baselines degraded |
| Turn budgets | Forces planning over trial-and-error | Turn-Control: solve rate maintained or improved |
| Graph navigation | Focuses on relevant files instead of random scanning | RepoMaster: 110% improvement in submissions |

### Techniques That Trade Quality for Savings (Must Be Managed)

| Technique | Quality Risk | Guardrail |
|-----------|-------------|-----------|
| Model downgrading (tier-2 → tier-3) | Cheaper model may miss subtle issues | `consecutiveClean >= 3` gate; automatic revert on any WARN/FAIL |
| Context compression | Compressed summary may omit critical detail | 96% retrieval quality benchmark; never compress load-bearing files |
| Memory distillation | 4-field summary loses nuance | Full cycle logs preserved in `.evolve/history/` for retrieval |
| Adaptive audit strictness | Reduced checklist may miss edge cases | Sections D/E (eval integrity) never skipped; random 20% full audit |

### The Golden Rule

> **Optimize the container, not the content.** Remove redundant *structure* (duplicate tool descriptions, stale history, re-read files), but never remove *information* that the current phase needs to make correct decisions.

---

## 5. Technique-by-Technique Deep Dive

### 5.1 Structured Inter-Phase Handoff Format

**Problem:** When Builder starts, it inherits the orchestrator's full conversation history from the Scout phase — including all file reads, search results, and analysis that the Builder doesn't need.

**Solution:** Define a formal `phaseHandoff` JSON schema that each phase writes as its final output. The next phase reads only this compact handoff, not the raw conversation.

```json
{
  "phase": "scout",
  "cycle": 112,
  "findings": ["phases.md lacks handoff schema", "token-optimization.md has stub"],
  "decisions": ["selected 3 tasks", "skipped research (internal goal)"],
  "files_modified": [],
  "next_phase_context": "Build these 3 tasks: add-structured-phase-handoff-format, ..."
}
```

**Research basis:**
- **CoDA** (arXiv:2512.12716): Context-decoupled hierarchical agents. Planner and Executor have isolated contexts. Result: stable performance in long-context scenarios.
- **InfiAgent** (arXiv:2601.03204): File-centric state externalization. Context size decoupled from task duration. A 20B model matches proprietary systems.
- **OPENDEV** (arXiv:2603.05344): Adaptive context compaction. Recent actions get full detail; older ones get summaries.

**What we implemented:** Added `### Inter-Phase Handoff Format` subsection to `phases.md` defining the schema and ownership table (which agent writes, which reads).

**Expected impact:** 40-60% token reduction per cycle by eliminating redundant context forwarding between phases.

**Quality impact:** Positive — less noise means the Builder focuses on its actual task rather than parsing irrelevant Scout analysis.

### 5.2 Dynamic Turn Budgets

**Problem:** Without turn limits, agents wander. The SWE-Effi study (arXiv:2509.09853) showed agents burn massive tokens on unsolvable problems — they never recognize they're stuck.

**Solution:** Per-phase turn budgets with dynamic extensions for Builder:
- Scout: 5 turns (tight — produce a task list, don't overthink)
- Builder: 10 turns + up to 5 dynamic extensions (flexible — code is unpredictable)
- Auditor: 3 turns (focused — render a verdict)
- Operator: 2 turns (brief — write state and move on)

**The dynamic extension mechanism:** At turn 10, the orchestrator checks two things: (1) Has at least one file changed in the last 3 turns? (2) Have eval grader results improved? If both pass, 5 more turns are granted. If neither, the build is classified as "stuck" and terminated early.

**Research basis:**
- **Turn-Control** (arXiv:2510.16786): 24-68% cost reduction with fixed limits; additional 12-24% with dynamic extensions.
- **SWE-Effi** (arXiv:2509.09853): Documented the "token snowball effect" — quadratic token growth per turn.

**What we implemented:** Replaced the placeholder stub in `token-optimization.md` with concrete per-phase budgets, the dynamic extension protocol, and early-exit detection.

**Expected impact:** 24-68% cost reduction.

**Quality impact:** Neutral to positive — forced planning improves code quality. Early-exit prevents wasted effort on stuck builds.

### 5.3 Active Context Compression

**Problem:** Within a single phase, tool outputs accumulate. A Builder that reads 10 files has ~30K tokens of file content in context, much of which is irrelevant to the current implementation step.

**Solution:** The Focus Agent pattern — after 5+ tool calls, autonomously consolidate accumulated context into a structured summary before the next reasoning step.

The summary uses a 4-field compound format:
- `exchange_core`: Key decisions and rationale (stripped of intermediate reasoning)
- `specific_context`: Concrete facts (file names, error messages, API shapes)
- `thematic_assignments`: High-level task ownership
- `files_touched`: List of modified files

**Research basis:**
- **Focus Agent** (arXiv:2601.07190): 22.7% overall token reduction, 57% peak on tool-heavy phases, solve rate maintained within 1.2%.
- **Structured Distillation** (arXiv:2603.13017): 11x compression (371 → 38 tokens per exchange), 96% retrieval quality retained.

**What we implemented:** New "Active Context Compression" section in `token-optimization.md` documenting the pattern, trigger points, distillation format, and integration with evolve-loop phases.

**Expected impact:** 22.7% overall, up to 57% on tool-heavy phases.

**Quality impact:** Minimal loss — 96% retrieval quality is the benchmark. The 4% loss is on peripheral details, not core decisions.

### 5.4 Structured Memory Distillation

**Problem:** The "learn" phase stores cycle summaries that grow over time. Raw cycle logs are verbose — a cycle that read 15 files and ran 20 commands produces thousands of tokens of history.

**Solution:** Compress each cycle's memory into the 4-field compound format before storage. Each exchange drops from ~371 tokens to ~38 tokens (11x compression).

**Research basis:** Structured Distillation (arXiv:2603.13017). Cross-layer search on distilled memories actually *exceeds* verbatim retrieval quality (MRR 0.759 vs 0.745).

**What we implemented:** Added structured distillation format to `phase5-learn.md` Memory Consolidation section.

**Quality impact:** Positive — structured memories are *easier* to search than raw logs.

### 5.5 Concrete Model Routing with Quality Guardrails

**Problem:** Abstract tier labels (tier-1, tier-2, tier-3) don't tell the orchestrator which specific model to use. And naive cost-minimization (always use the cheapest model) degrades quality.

**Solution:** Map tiers to concrete models (Opus 4.6 = tier-1, Sonnet 4.6 = tier-2, Haiku 4.5 = tier-3) with explicit quality guardrails:
- Downgrade tier-2 → tier-3 only when `consecutiveClean >= 3` for the task type
- Revert immediately on any WARN/FAIL
- Block tier-3 Builder routing if eval first-attempt failure rate > 33%
- Suspend all tier-3 routing if benchmark drops > 3 points

**Research basis:** MasRouter (arXiv:2502.11133): 52-70% cost reduction with accuracy maintained or improved.

**What we implemented:** New "Concrete Anthropic Model Mappings" and "Quality Guardrails for Tier Downgrading" subsections in `token-optimization.md`.

**Quality impact:** Quality-guarded — cheaper routing is earned through demonstrated reliability and lost immediately on any quality signal.

### 5.6 Graph-Based Code Exploration

**Problem:** Scout phase reads many files to find the few that matter. In a 500-file repo, this is wasteful.

**Solution:** Build a lightweight graph (nodes = files, edges = imports/calls) and traverse it selectively. The Scout reads 3-7 relevant files instead of scanning 50+.

**IMPORTANT:** Graph exploration complements, not replaces, full file reading. Agent definitions, skill files, and config files are always read in full. Graph exploration answers "which files?" — full reading answers "what does this file say?"

**Research basis:**
- **RepoMaster** (arXiv:2505.21577): 95% token reduction on repository exploration, 110% improvement in SWE-bench submissions.
- **GraphReader** (arXiv:2406.14550): 4K context with graph navigation outperforms 128K raw context.

**What we implemented:** New `docs/graph-exploration.md` documenting the pattern, traversal algorithm, and when-to-use decision table.

**Quality impact:** Positive — focused file reading improves task relevance. The decision table ensures critical files are never skipped.

---

## 6. What We Chose NOT to Implement

Some research techniques were not applied, and it's important to document why:

| Technique | Paper | Why Not Applied |
|-----------|-------|-----------------|
| MDP-based prompt compression | DCP-Agent (arXiv:2504.11004) | Requires training a compression model — not feasible as prompt instructions |
| Suffix-automaton memory reuse | AgentSAM (arXiv:2512.18337) | Requires custom inference infrastructure — API users can't implement |
| Bandit-optimized agent discovery | BOAD (arXiv:2512.23631) | Requires many exploration cycles to discover optimal agents — too expensive for practical use |
| Noise injection for sandbagging | Tice et al. (arXiv:2412.01784) | Requires model weight access — API-only constraint |
| Evolutionary plan search | LoongFlow (arXiv:2512.24077) | High implementation complexity for marginal gains over plan caching |
| Full prompt caching with 1h TTL | Anthropic docs | Requires API-level integration that the skill can't control (it's the harness's job) |
| KV cache TTL management | Continuum (arXiv:2511.02230) | Relevant for self-hosted models, not API usage |

**Common pattern:** We excluded techniques that require custom model training, model weight access, or infrastructure control. The evolve-loop operates within the constraints of standard LLM API calls with prompt engineering.

---

## 7. Quantitative Impact Summary

### Expected Savings (Combined)

| Technique | Token Savings | Cost Savings | Latency Impact |
|-----------|-------------|-------------|----------------|
| Phase handoff isolation | 40-60% per cycle | 40-60% | Neutral |
| Dynamic turn budgets | 24-68% | 24-68% | Reduced (fewer turns) |
| Active context compression | 22.7% overall | 22.7% | Slight overhead from compression step |
| Model routing (with guardrails) | — | 52-70% | Neutral |
| Structured memory distillation | 11x on history | Moderate | Neutral |
| Graph exploration for Scout | 95% on exploration | Significant | Reduced (fewer reads) |

### Net Expected Impact

The techniques are not all additive (some overlap), but conservative estimates suggest:
- **Per-cycle token usage:** ~40-50% reduction (from ~50K to ~25-30K)
- **Per-cycle cost:** ~50-60% reduction (model routing compounds with token savings)
- **Per-cycle latency:** ~20-30% reduction (fewer turns, fewer file reads)
- **Quality:** Maintained or improved (every technique has quality guardrails)

### What We Can Measure

The evolve-loop tracks these metrics in `state.json`:
- `processRewards.skillEfficiency`: Token usage trend
- `fitnessScore`: Composite quality metric
- `auditorProfile.consecutiveClean`: Quality signal for model routing decisions
- `projectBenchmark.overall`: Project-level quality score

If token savings come with quality degradation, these metrics will catch it. The Operator's fitness regression detection triggers a HALT if quality drops for 2 consecutive cycles.

---

## 8. Lessons Learned

### Lesson 1: The Best Optimization Is Not Sending Tokens at All

The highest-impact techniques aren't about compressing what you send — they're about not sending it in the first place. Phase isolation (don't forward Scout history to Builder) and graph exploration (don't read irrelevant files) are more effective than any compression algorithm because they eliminate tokens at the source.

### Lesson 2: Turn Budgets Beat Token Budgets

A flat 80K token limit is a lagging indicator. By the time you hit it, you've already wasted tokens on unproductive turns. A 10-turn budget with dynamic extensions forces the agent to plan efficiently and exit early when stuck. This is counterintuitive — constraints improve performance.

### Lesson 3: Quality Guardrails Must Be Automatic, Not Advisory

"Be careful when downgrading models" is an advisory that gets ignored under pressure. `consecutiveClean >= 3` with automatic revert on WARN is a rule that enforces itself. Every optimization we applied has a machine-checkable quality gate, not just a human-readable warning.

### Lesson 4: Compression Quality Depends on What You Keep, Not How Much You Remove

11x compression sounds aggressive, but Structured Distillation showed that keeping 4 specific fields (`exchange_core`, `specific_context`, `thematic_assignments`, `files_touched`) retains 96% of retrieval quality. The other 90% of tokens were reasoning traces, formatting, and repetition that downstream agents never use.

### Lesson 5: Graph Navigation Is Transformative for Discovery

The jump from 128K raw context to 4K with graph navigation (GraphReader) is not incremental — it's a paradigm shift. The evolve-loop's Scout phase was scanning files linearly. Graph traversal changes the question from "read everything and filter" to "find what's connected and read only that."

### Lesson 6: Research Claims Require Context

Papers report results on specific benchmarks. "95% token reduction" on RepoMaster was measured on GitTaskBench, a benchmark of large repositories with well-defined tasks. Our evolve-loop operates on smaller repos where the gains are proportionally smaller. Similarly, "52-70% cost reduction" from MasRouter assumes a diverse workload — a workload that's 100% complex coding tasks won't see 70% savings from model routing.

**Always read the benchmark conditions, not just the headline numbers.**

---

## 9. References

### Tier 1: Directly Applied
1. Liu et al. **"CoDA: Context-Decoupled Hierarchical Agent"** — arXiv:2512.12716, WSDM '26 Oral. Context isolation for planning vs execution.
2. Verma. **"Active Context Compression for Long-Term Agentic Memory"** — arXiv:2601.07190. Focus Agent pattern, 22.7% token reduction.
3. Lewis. **"Structured Distillation for Agent Memory"** — arXiv:2603.13017. 11x compression, 96% retrieval quality.
4. Gao & Peng. **"Turn-Control Strategies for Coding Agents"** — arXiv:2510.16786. 24-68% cost reduction with dynamic budgets.
5. Fan et al. **"SWE-Effi: Efficiency Under Resource Constraints"** — arXiv:2509.09853. Token snowball effect, stuck build detection.
6. Yue et al. **"MasRouter: Learning to Route LLMs for Multi-Agent Systems"** — arXiv:2502.11133. 52-70% cost reduction via routing.
7. Wang et al. **"RepoMaster: Graph-Based Repository Exploration"** — arXiv:2505.21577. 95% token reduction in code navigation.
8. Li et al. **"GraphReader"** — arXiv:2406.14550, EMNLP 2024. 4K context beats 128K with graph navigation.
9. Yu et al. **"InfiAgent: Infinite-Horizon Framework"** — arXiv:2601.03204. File-centric state externalization.
10. Bui. **"OPENDEV: Terminal Coding Agent Architecture"** — arXiv:2603.05344. Closest analog to evolve-loop architecture.

### Tier 2: Informed Design Decisions
11. Chen & Liu. **"Neural Paging"** — arXiv:2602.02228. Semantic caching with paging policies.
12. Wang et al. **"Memex(RL): Indexed Experience Memory"** — arXiv:2603.04257. Compact working context with external retrieval.
13. Jiang et al. **"LSTM-MAS: Multi-Agent Long-Context System"** — arXiv:2601.11913. 40-121% improvement via LSTM-gate agents.
14. Fan et al. **"CtrlCoT: Chain-of-Thought Compression"** — arXiv:2601.20467. 30.7% fewer tokens, +7.6pp accuracy.
15. Zeng et al. **"Attn-GS: Attention-Guided Compression"** — arXiv:2602.07778. 50x token reduction.
16. Liu et al. **"ShortCoder: Code-Specific Compression"** — arXiv:2601.09703. 18.1% token reduction in code.
17. Mei et al. **"Survey of Context Engineering"** — arXiv:2507.13334. 166-page taxonomy of context engineering.
18. Chhikara et al. **"Mem0: Production-Ready Long-Term Memory"** — arXiv:2504.19413. 26% quality improvement, 90% token savings.
19. Lin et al. **"AgentInfer: Co-Design of Inference Architecture"** — arXiv:2512.18337. 50% ineffective token reduction.
20. Wu & Silwal. **"PORT: Training-Free Online LLM Routing"** — arXiv:2509.02718, NeurIPS 2025. 3.55x routing improvement.

### Tier 3: Background Research
21. Hayashi et al. **"SAGE: Self-Abstraction from Grounded Experience"** — arXiv:2511.05931. Plan abstraction, 7.2% improvement.
22. Wu et al. **"Meta-Policy Reflexion"** — arXiv:2509.03990. Structured predicate memory.
23. Xu et al. **"BOAD: Bandit-Optimized Agent Discovery"** — arXiv:2512.23631. MAB for agent hierarchy.
24. Xia et al. **"HiveMind: DAG-Shapley for Agent Contribution"** — arXiv:2512.06432, AAAI 2026. 80% fewer LLM calls.
25. Shahout et al. **"Orla: Serving Multi-Agent Systems"** — arXiv:2603.13605. Workflow-level KV cache management.
26. Gong. **"Structured Prompt Language"** — arXiv:2602.21257. 65% prompt boilerplate reduction.
27. Vishnyakova. **"Context Engineering Maturity Model"** — arXiv:2603.09619. Five quality criteria framework.
28. NeurIPS 2025. **"Agentic Plan Caching"** — 50.31% cost reduction, 96.61% performance retention.
29. Liu et al. **"Cognitive Chunking / PIC"** — arXiv:2602.13980. 29.8% improvement at 64x compression.
30. Ulla. **"LoPace: Prompt Storage Compression"** — arXiv:2602.13266. 72.2% storage savings.
