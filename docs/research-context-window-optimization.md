# Research: Context Window Optimization, Performance & Latency for Agentic Systems

Comprehensive literature review on optimizing LLM context window usage, improving performance and latency in multi-agent autonomous systems. Findings directly inform the evolve-loop's architecture.

---

## Executive Summary

The research identifies **10 high-impact techniques** that can reduce token usage by 40-90% and latency by 25-90% in agentic coding systems. The most impactful are:

1. **Inter-phase context isolation** (40-60% token savings) — each phase gets minimal structured handoff, not full history
2. **Prompt caching** (90% cost savings on cached tokens) — stable system context cached with TTL
3. **Active context compression** (22.7% overall, up to 57% peak) — autonomous distillation between phases
4. **Dynamic turn budgets** (24-68% cost reduction) — tight limits with on-demand extensions
5. **Model routing per phase** (52-70% cost reduction) — Haiku for simple phases, Sonnet for coding

---

## 1. Context Window Optimization for Agents

### 1.1 Neural Paging
**Source:** Chen & Liu, arXiv:2602.02228 (Feb 2026)

Treats the context window as a semantic cache with learned paging policies, analogous to OS virtual memory. A hierarchical architecture decouples symbolic reasoning from resource management.

**Results:** Reduces long-horizon reasoning complexity from O(N²) to O(N·K²) where K is the page size.

**Application:** Each phase operates with a "paged" context where only the active page is in-context. Between phases, the system pages out completed work and pages in what the next phase needs. Maintain a structured "page table" — a compact index of what information exists and where to find it.

### 1.2 InfiAgent: Infinite-Horizon Framework
**Source:** Yu, Wang, Wang, Yang, Li, arXiv:2601.03204 (Jan 2026)

Externalizes persistent state into a file-centric abstraction. At each step, the agent reconstructs context from a workspace state snapshot plus a fixed window of recent actions. Context size is completely decoupled from task duration.

**Results:** A 20B parameter model achieves competitive performance against larger proprietary systems on 80-paper literature review tasks.

**Application:** Directly applicable. Each cycle writes state to files (`cycle_state.json`), and the next cycle reconstructs context from that snapshot rather than carrying forward raw conversation history. Replace in-context cycle history with file-based state: `{phase, findings, decisions, artifacts_modified, lessons}`.

### 1.3 CoDA: Context-Decoupled Hierarchical Agent
**Source:** Liu et al., arXiv:2512.12716 (Dec 2025, WSDM '26 Oral)

Addresses "Context Explosion" by decoupling high-level planning from low-level execution. A Planner decomposes tasks while an Executor handles tool interactions, each with isolated contexts. Joint RL optimization (PECO) trains both roles.

**Results:** Maintains stable performance in long-context scenarios where all baselines suffer severe degradation.

**Application:** Each phase should have its own isolated context with only the inter-phase handoff data shared. The "discover" phase planner should not carry "build" phase tool output. This is the theoretical foundation for the evolve-loop's phase architecture.

### 1.4 Memex(RL): Indexed Experience Memory
**Source:** Wang et al., arXiv:2603.04257 (Mar 2026)

Maintains a compact working context of structured summaries and stable indices, while storing full-fidelity interactions in an external experience database. RL optimizes what to summarize and when to retrieve.

**Results:** Improves task success while using significantly smaller working context.

**Application:** Store full cycle histories externally (e.g., `.evolve/history/cycle_N/`). Keep only an indexed summary in the active context. The "learn" phase can retrieve specific past experiences on demand.

### 1.5 LSTM-MAS: Multi-Agent Long-Context System
**Source:** Jiang et al., arXiv:2601.11913 (Jan 2026)

Chains of 4 specialized agents (worker, filter, judge, manager) process text segments, emulating LSTM gates for information flow control.

**Results:** 40-121% improvement over prior best (CoA) on NarrativeQA, Qasper, HotpotQA, MuSiQue.

**Application:** Each phase could use a "filter" sub-step that strips irrelevant information before passing context to the next phase. A "judge" sub-step validates that the compressed context retains critical information.

---

## 2. Context Compression Techniques

### 2.1 Active Context Compression / Focus Agent
**Source:** Verma, arXiv:2601.07190 (Jan 2026)

Agent autonomously consolidates learnings into persistent "Knowledge" blocks while pruning raw interaction history. Inspired by slime mold exploration patterns.

**Results:** 22.7% token reduction (14.9M to 11.5M tokens) on SWE-bench, up to 57% on individual instances, maintained 60% accuracy with avg 6.0 autonomous compressions per task.

**Application:** **Highest-ROI change for the evolve-loop.** After each phase completes, trigger an autonomous compression step that distills findings into a compact knowledge block. Add a `compress_context()` step between phases.

### 2.2 Structured Distillation for Agent Memory
**Source:** Lewis, arXiv:2603.13017 (Mar 2026)

Compresses conversation exchanges into 4-field compound objects: `{exchange_core, specific_context, thematic_assignments, files_touched}`. Each exchange drops from 371 to 38 tokens.

**Results:** 11x token compression (371 → 38 tokens per exchange), retains 96% of verbatim retrieval quality (MRR 0.717 vs 0.745). Cross-layer search actually exceeds verbatim at MRR 0.759.

**Application:** Each cycle's "learn" phase output should use this structured format. When the next cycle references past work, it retrieves 38-token summaries instead of full logs. Structure: `{cycle_id, task_description, changes_made, files_touched, lessons_learned}`.

### 2.3 CtrlCoT: Chain-of-Thought Compression
**Source:** Fan et al., arXiv:2601.20467 (Jan 2026)

Dual-granularity compression combining hierarchical reasoning abstraction with logic-preserving distillation. Prunes redundant reasoning tokens while keeping numerical values and operators.

**Results:** 30.7% fewer tokens with 7.6 percentage points higher accuracy on MATH-500 vs strongest baseline.

**Application:** When using extended thinking in the "build" phase, compress the reasoning trace before storing it. Strip verbose self-reflection but keep decision points and numerical results.

### 2.4 Attn-GS: Attention-Guided Compression
**Source:** Zeng et al., arXiv:2602.07778 (Feb 2026)

Uses attention patterns to identify important signals, then compresses context while preserving those signals.

**Results:** 50x token reduction while maintaining near-full performance.

**Application:** For the "discover" phase analyzing large codebases, use attention-guided filtering to identify only the relevant code sections.

### 2.5 ShortCoder: Code-Specific Compression
**Source:** Liu et al., arXiv:2601.09703 (Jan 2026)

AST-preserving syntax simplification rules that reduce code token count while maintaining semantic equivalence.

**Results:** 18.1% token reduction in generated code, 18.1-37.8% efficiency improvement on HumanEval.

**Application:** When including code in context (the "build" phase), apply syntax simplification. Strip comments, simplify verbose patterns. Expected 18% savings on code content.

### 2.6 Cognitive Chunking / PIC
**Source:** Liu et al., arXiv:2602.13980 (Feb 2026)

Restricts memory token receptive fields to local sequential chunks via modified attention masks, inspired by human working memory.

**Results:** 29.8% improvement at 64x compression ratio, 40% training time reduction.

**Application:** Process large file contents in chunks rather than loading entire files into context.

### 2.7 LoPace: Prompt Storage Compression
**Source:** Ulla, arXiv:2602.13266 (Feb 2026)

Combines Zstandard + BPE + hybrid methods for prompt storage.

**Results:** 72.2% space savings, 4.89x mean compression ratio, 100% lossless reconstruction, 3.3-10.7 MB/s throughput.

**Application:** Compress stored prompts and agent definitions on disk. Useful for large skill/agent files.

---

## 3. Anthropic Prompt Caching (Production-Ready)

### Key Specifications
| Parameter | Value |
|-----------|-------|
| Cache hit cost | 0.1x base input price (90% savings) |
| Cache write cost | 1.25x base (5-min TTL) or 2x base (1-hour TTL) |
| Rate limits | Cache hits do NOT count against rate limits |
| Min tokens | Opus 4.6: 4096, Sonnet 4.6: 2048, Sonnet 4.5/4.1: 1024 |
| Max breakpoints | 4 per request |
| Cache prefix order | tools → system → messages |

### Implementation for Evolve-Loop
- **System prompt + tool definitions:** Stable across cycles. Cache with 1-hour TTL. At 0.1x base = 90% savings on ~4K+ tokens per API call.
- **Codebase context:** Cache project structure and key files as system context. Only invalidate on file changes.
- **Multi-turn within a cycle:** Use automatic caching so each phase reads prior phases from cache.
- **Ordering principle:** Put CLAUDE.md, tool definitions, and codebase summaries before dynamic cycle-specific content.

---

## 4. Agentic Performance & Latency

### 4.1 OPENDEV: Terminal Coding Agent Architecture
**Source:** Bui, arXiv:2603.05344 (Mar 2026)

Compound AI architecture with:
- Workload-specialized model routing
- Dual-agent separation of planning vs execution
- Lazy tool discovery (load only what current phase needs)
- Adaptive context compaction (progressively reduce older observations)
- Event-driven system reminders to combat instruction fade-out

**Application — direct analog to evolve-loop:**
- **Lazy tool discovery:** Don't load all tool definitions upfront; load only current phase tools
- **Adaptive compaction:** Recent actions get full detail; older ones get summaries
- **System reminders:** Re-inject critical instructions periodically to prevent fade-out in long sessions
- **Model routing:** Haiku for simple ops, Sonnet for coding, Opus for architecture

### 4.2 Orla: Serving Multi-Agent Systems
**Source:** Shahout et al., arXiv:2603.13605 (Mar 2026)

Serving layer with three controls: stage mapping (route to right model), workflow orchestration (scheduling + resources), memory management (KV cache across workflow boundaries).

**Application:** Map each phase to optimal model. Cache KV state between phases when possible.

### 4.3 GraphReader: 4K Beating 128K
**Source:** Li et al., arXiv:2406.14550 (Jun 2024, EMNLP 2024)

Structures long texts as graphs, agent explores coarse-to-fine with plan-reflect-revise loop using only 4K context window.

**Results:** Outperforms GPT-4-128k on inputs from 16K-256K tokens using only 4K context.

**Application:** For the "discover" phase analyzing large codebases, build a graph of the codebase structure (call graphs, dependency trees) and navigate with small context windows rather than loading entire files.

---

## 5. Multi-Agent Efficiency

### 5.1 AgentInfer: Co-Design of Inference Architecture
**Source:** Lin et al., arXiv:2512.18337 (Dec 2025)

- **AgentCollab:** Hierarchical dual-model reasoning — large model for hard reasoning, small model for routine work
- **AgentSAM:** Suffix-automaton-based speculative decoding reusing multi-session semantic memory
- **AgentCompress:** Asynchronous semantic compression while reasoning continues

**Results:** 50%+ reduction in ineffective tokens, 1.8-2.5x speedup, accuracy preserved.

**Application:** Dual-model pattern maps to scout/build/audit — Haiku for reconnaissance and routine checks, Sonnet/Opus for implementation.

### 5.2 BOAD: Bandit-Optimized Agent Discovery
**Source:** Xu et al., arXiv:2512.23631 (Dec 2025)

Formulates discovering optimal sub-agent hierarchies as a multi-armed bandit problem. Automatically discovers which specialist agents to create and how to compose them.

**Results:** 36B system ranks second on SWE-bench Live, surpassing GPT-4 and Claude.

**Application:** Instead of manually designing agents, use bandit optimization to discover optimal sub-agent configurations over cycles.

### 5.3 HiveMind: DAG-Shapley Agent Contribution Analysis
**Source:** Xia et al., arXiv:2512.06432 (Dec 2025, AAAI 2026)

Uses DAG-Shapley to quantify each agent's contribution, then auto-refines underperforming agents' prompts. Exploits DAG structure to prune non-viable coalitions.

**Results:** 80% reduction in LLM calls while preserving attribution accuracy.

**Application:** After each cycle, compute contribution of scout vs build vs audit to identify which phase wastes tokens. Auto-refine prompts for underperforming phases.

### 5.4 MasRouter: LLM Routing for Multi-Agent Systems
**Source:** Yue et al., arXiv:2502.11133 (Feb 2025)

Cascaded controller routing: determines collaboration mode, role allocation, and LLM selection per agent. Plug-and-play.

**Results:** 1.8-8.2% accuracy improvement, up to 52% cost reduction, 17-28% overhead reduction.

---

## 6. Coding Agent Optimization

### 6.1 Turn-Control Strategies
**Source:** Gao & Peng, arXiv:2510.16786 (Oct 2025)

Three strategies on SWE-bench:
1. Unrestricted baseline
2. Fixed-turn limit at 75th percentile + reminder messages
3. Dynamic-turn: on-demand extensions only when justified

**Results:**
- Fixed-turn: 24-68% cost reduction with minimal solve-rate impact
- Dynamic-turn: additional 12-24% savings beyond fixed-turn

**Application:** Implement dynamic turn budgets per phase. Scout gets tight budget (~5 turns); build gets flexible budget that extends if making progress. The 75th percentile of historical usage is the sweet spot for fixed limits.

### 6.2 SWE-Effi: Token Snowball Effect
**Source:** Fan et al., arXiv:2509.09853 (Sep 2025)

**Key findings:**
- Tokens accumulate quadratically across turns as context grows (each turn re-encodes full history)
- Agents burn massive tokens on unsolvable problems, never recognizing they're stuck
- System effectiveness depends on scaffold-model integration, not scaffold alone

**Application:** Implement early exit detection in build phase. Track per-cycle token consumption and flag snowball patterns.

### 6.3 RepoMaster: Graph-Based Repository Exploration
**Source:** Wang et al., arXiv:2505.21577 (May 2025)

Builds function-call graphs, module-dependency graphs, and hierarchical code trees to identify essential components. Progressive exploration with pruning.

**Results:** 110% boost in valid submissions, **95% token usage reduction** on GitTaskBench.

**Application:** Scout phase should build a code graph representation instead of dumping full files into context. Only pass relevant subgraphs to build phase.

### 6.4 SAGE: Self-Abstraction from Grounded Experience
**Source:** Hayashi et al., arXiv:2511.05931 (Nov 2025)

After initial execution, agent induces concise plan abstraction (key steps, dependencies, constraints) from grounded experience, then uses this as guidance for similar future tasks.

**Results:** 73.2-74% Pass@1 on SWE-bench Verified (7.2% relative improvement).

**Application:** The "learn" phase should produce plan abstractions from successful cycles and inject them into future scout/build phases. This is what the plan cache already does — validates the approach.

---

## 7. Memory Systems

### 7.1 Mem0: Production-Ready Long-Term Memory
**Source:** Chhikara et al., arXiv:2504.19413 (Apr 2025)

Dynamic memory extraction and consolidation with graph-based representations capturing relational structures.

**Results:** 26% improvement on LOCOMO benchmark, 91% reduced latency, 90%+ token cost savings vs full-context.

**Application:** Implement persistent memory layer storing cross-cycle knowledge. Each new cycle reads from memory rather than re-discovering.

### 7.2 Meta-Policy Reflexion (MPR)
**Source:** Wu et al., arXiv:2509.03990 (Sep 2025)

- **Meta-Policy Memory:** Predicate-like structured representation of reusable knowledge
- **Hard Admissibility Checks (HAC):** Domain constraints preventing invalid/unsafe actions

**Application:** "Learn" phase should produce structured predicate-form rules, not prose. Example: `IF file_type=security AND change_type=auth THEN require_audit=true`.

---

## 8. Cost Optimization

### 8.1 PORT: Training-Free Online LLM Routing
**Source:** Wu & Silwal, arXiv:2509.02718 (Sep 2025, NeurIPS 2025)

Training-free routing using approximate nearest neighbor search + one-time optimization. Achieves competitive ratio of 1 - o(1).

**Results:** 3.55x overall improvement, 1.85x cost efficiency, 4.25x throughput.

**Application:** Route each sub-task to cheapest-capable model dynamically. Open-source: github.com/fzwark/PORT.

---

## 9. Context Engineering Frameworks

### 9.1 Survey of Context Engineering
**Source:** Mei et al., arXiv:2507.13334 (Jul 2025) — 166 pages, 1411 citations

**Taxonomy:** Context Engineering = {Retrieval, Generation, Processing, Management} applied through {RAG, Memory Systems, Tool-Integrated Reasoning, Multi-Agent Systems}.

**Critical finding:** Models are proficient at understanding complex contexts but limited at generating equally sophisticated long-form outputs.

### 9.2 Structured Prompt Language
**Source:** Gong, arXiv:2602.21257 (Feb 2026)

**Results:** 65% reduction in prompt boilerplate, O(N²) to O(N²/k) attention via logical chunking.

**Application:** Structure skill files with explicit token budgets per section. Use declarative context management.

### 9.3 Context Engineering Maturity Model
**Source:** Vishnyakova, arXiv:2603.09619 (Mar 2026)

**Five quality criteria:** Relevance, Sufficiency, Isolation, Economy, Provenance.

**Application:** Each phase's context should be evaluated against these 5 criteria. "Economy" is critical — include only what the current phase needs.

---

## 10. Quantitative Summary

| Technique | Token/Cost Savings | Quality Impact | Implementation Effort |
|-----------|-------------------|----------------|----------------------|
| Prompt caching (1h TTL) | 90% on cached tokens | None (identical) | Low |
| Phase context isolation | 40-60% per cycle | Improved (less noise) | Low |
| Dynamic turn budgets | 24-68% cost reduction | Minimal impact | Low |
| Model routing per phase | 52-70% cost reduction | Minimal (Haiku handles simple) | Low-Medium |
| Lazy tool loading | 15-30% on tool tokens | None | Low |
| Structured distillation | 11x on history | 96% retrieval quality | Medium |
| Active context compression | 22.7% overall, 57% peak | Maintained | Medium |
| CoT compression | 30.7% on reasoning | +7.6pp accuracy | Medium |
| Code syntax compression | 18.1% on code tokens | Semantically equivalent | Low |
| Cross-cycle memory (Mem0) | 90%+ vs full-context | +26% task quality | Medium |
| Graph-based code nav | 95% token reduction | 110% submission boost | Medium-High |

---

## 11. Key Papers by Relevance to Evolve-Loop

### Tier 1: Must-Read (Directly Applicable)
1. **arXiv:2603.05344** — OPENDEV (closest analog to evolve-loop architecture)
2. **arXiv:2601.07190** — Focus Agent (active context compression for SWE-bench, 22.7% savings)
3. **arXiv:2601.03204** — InfiAgent (file-centric state externalization)
4. **arXiv:2512.12716** — CoDA (context-decoupled hierarchical agents, WSDM '26)
5. **arXiv:2510.16786** — Turn-Control (24-68% cost reduction with dynamic budgets)
6. **arXiv:2509.09853** — SWE-Effi (token snowball effect, early exit)
7. **arXiv:2603.13017** — Structured Distillation (11x compression, 96% quality)

### Tier 2: High Value
8. **arXiv:2507.13334** — Context Engineering survey (166-page taxonomy)
9. **arXiv:2504.19413** — Mem0 (production-ready memory, 26% quality boost)
10. **arXiv:2512.18337** — AgentInfer (dual-model, 50% token reduction)
11. **arXiv:2505.21577** — RepoMaster (95% token reduction via graph exploration)
12. **arXiv:2511.05931** — SAGE (plan abstraction, 7.2% improvement)
13. **arXiv:2601.20467** — CtrlCoT (CoT compression with accuracy gain)

### Tier 3: Worth Monitoring
14. **arXiv:2512.23631** — BOAD (bandit-optimized agent discovery)
15. **arXiv:2512.06432** — HiveMind (DAG-Shapley contribution analysis)
16. **arXiv:2602.21257** — Structured Prompt Language (65% boilerplate reduction)
17. **arXiv:2603.09619** — Context Engineering Maturity Model

---

## 12. Implementation Priority for Evolve-Loop

### Phase 1: Quick Wins (implement now)
1. **Enforce lean context after cycle 3** — already partially done; ensure strict phase isolation
2. **Dynamic turn budgets** — Scout: 5 turns max, Builder: dynamic with extensions, Auditor: 3 turns
3. **Lazy context loading** — load only current phase's tool definitions and agent prompt
4. **Structured handoff format** — `{phase, findings[], decisions[], files_modified[], instructions}`

### Phase 2: Medium-Term (next iteration)
5. **Active context compression** — add compress step between phases (Focus Agent pattern)
6. **Structured memory format** — 4-field compound objects for cycle summaries (11x compression)
7. **Model routing optimization** — Haiku for scout incremental + audit clean builds, Opus for stuck builds
8. **Graph-based code exploration** — build lightweight dependency graph for scout phase

### Phase 3: Advanced (future)
9. **DAG-Shapley contribution analysis** — identify which phases waste tokens
10. **Plan abstraction caching** — extract reusable plans from successful cycles (SAGE pattern)
11. **Suffix-automaton memory reuse** — detect and short-circuit repeated reasoning patterns
