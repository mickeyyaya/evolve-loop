# Part 1 — Prompt/Context Compression & Context Engineering for LLM Agents (2025–2026 survey)

> Research agent sweep 1 of 3, 2026-07-05, for evolve-loop token-optimization goal 805f6ced.
> Scope: multi-phase pipeline (scout→triage→tdd→build→audit→ship), each phase a separate LLM invocation with injected markdown artifacts.

## Thread 1 — Prompt Compression

### 1. LLMLingua family — LLMLingua-2 remains the lossy-compression state of practice
**Microsoft (Jiang et al. / Pan et al.), EMNLP 2023 + ACL Findings 2024; still the 2026 production baseline.**
Token-classification-based, task-agnostic extractive compression: a small distilled model (trained on GPT-4-generated compression data) scores each token keep/drop. Original LLMLingua: up to **20× compression with ~1.5% performance loss** on reasoning tasks; LLMLingua-2: **2×–5× compression with up to 2.9× end-to-end latency reduction**; production workloads report **4–10× as the practical quality ceiling** ([arXiv 2310.05736](https://arxiv.org/html/2310.05736v2), [LongLLMLingua 2310.06839](https://arxiv.org/pdf/2310.06839), [TokenMix 2026 field report](https://tokenmix.ai/blog/llmlingua-prompt-compression-2026)). 2025–26 successors: **SCOPE** ([arXiv 2508.15813](https://arxiv.org/html/2508.15813v1)) training-free generative chunk-and-summarize; **MOOSComp** ([arXiv 2504.16786](https://arxiv.org/pdf/2504.16786)) fixes over-smoothing; **CompactPrompt** ([arXiv 2510.18043](https://arxiv.org/html/2510.18043v1)) unified prompt+data compression.
**Applicability:** deterministic pre-injection filter for bulky low-density artifacts (test logs, diffs in build-report.md) at conservative 2–4×; risky for dense hand-schema'd markdown.

### 2. 500xCompressor — soft-prompt/KV compression at extreme ratios
**Li & Collier (Cambridge), ACL 2025 Main, [arXiv 2408.03094](https://arxiv.org/html/2408.03094v1), [ACL](https://aclanthology.org/2025.acl-long.1219/), [GitHub](https://github.com/ZongqianLi/500xCompressor).**
Compresses up to 500 NL tokens into 1 special token by storing **KV values (not embeddings)**, ~0.25–0.3% added params; frozen target LLM consumes compressed tokens without finetuning. **6×–480× compression**, retaining **62–73% of QA capability at peak**; beats ICAE by +2.06–9.23% F1 on ArxivQA. Apex of the gist-token lineage (Mu et al., NeurIPS 2023, ~26×).
**Applicability:** **NOT deployable** for closed CLIs (claude/codex/gemini) — needs model weight/KV access. Watch item only (relevant if self-hosted model joins fleet, e.g. ollama).

### 3. Lossless meta-token compression
**[arXiv 2506.00307](https://arxiv.org/pdf/2506.00307), 2025.** Lossless token-sequence compression via meta-tokens — the lossless alternative where extractive dropping is unacceptable. Lower ratios, zero semantic loss.
**Applicability:** validates two-tier artifact policy: lossless (structural dedup, boilerplate strip) for contract-bearing sections; lossy only for narrative/log sections.

## Thread 2 — Context Engineering for Agents

### 4. Anthropic — "Effective Context Engineering for AI Agents"
**Anthropic engineering blog, Sept 2025, [URL](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents).**
Canonical guidance. Names **context rot** (n² attention stretch → accuracy degrades with token count). Three long-horizon techniques: **compaction** (summarize + reinit); **structured note-taking** (persistent memory OUTSIDE the window — files); **sub-agent architectures** with isolated windows returning condensed **~1,000–2,000 token** summaries to the orchestrator. Advocates **just-in-time retrieval** (lightweight identifiers, load at runtime) over pre-loaded context.
**Applicability:** direct blueprint validation of evolve-loop design (phase agents ≈ sub-agents; artifacts ≈ structured notes); actionable delta = enforce 1–2k-token summary budget on inter-phase artifacts + inject POINTERS not full bodies.

### 5. Anthropic — Context Editing + Memory Tool (productized compaction)
**Launched with Sonnet 4.5 (2025); server-side compaction productized on Opus 4.6 (2026). [Announcement](https://www.anthropic.com/news/context-management), [context-editing docs](https://platform.claude.com/docs/en/build-with-claude/context-editing), [memory-tool docs](https://platform.claude.com/docs/en/agents-and-tools/tool-use/memory-tool), [token-saving updates](https://www.anthropic.com/news/token-saving-updates).**
**Context editing** auto-clears STALE TOOL CALLS/RESULTS when nearing limits; memory tool = client-side file store. 100-turn web-search eval: completes workflows that otherwise exhaust context **while reducing token consumption by 84%**.
**Applicability:** single most transferable datapoint — stale tool-result eviction is highest-yield lowest-risk; phase drivers could clear tool outputs older than N turns.

### 6. Context-Folding + FoldGRPO
**[arXiv 2510.11967](https://arxiv.org/pdf/2510.11967), 2025.**
`branch(description, prompt)` / `return(message)` actions → two-level plan-execute hierarchy: token-heavy subtasks run in clean sub-context, then **fold** — only outcome summary persists. FoldGRPO trains end-to-end. On Deep Research + SWE benchmarks: **matches/exceeds ReAct with ~10× smaller active context**; substantially beats summarization-based management.
**Applicability:** phase boundaries are hand-built folds; argues for INTRA-phase folding — e.g. build spawns a bridge sub-agent per failing test, folds each to one-paragraph outcome.

### 7. Self-Compacting Language Model Agents
**[arXiv 2606.23525](https://arxiv.org/pdf/2606.23525), June 2026.**
Model itself decides when/how to compact (vs fixed 70–90% threshold). MathArena/BrowseComp/SWE-Bench: learned self-compaction **matches or beats threshold compaction at a fraction of token cost** (picks semantically cheap compression points).
**Applicability:** phase prompts should instruct self-compaction at natural task boundaries ("after tests pass, summarize and drop exploration") instead of harness threshold.

### 8. Structured Context Eviction
**[arXiv 2606.11213](https://arxiv.org/pdf/2606.11213), June 2026.**
Selectively **evict low-priority items** (completed subtasks, obsolete observations) preserving dependency relationships + key facts, instead of lossy whole-context summary. TerminalBench/SWE-bench: better context utilization; eviction preserves ACTIONABLE detail that summaries destroy.
**Applicability:** artifact schema design — scout-report.md gets evictable sections (exploration narrative) vs never-evict sections (decisions, acceptance criteria, open questions); downstream phases get post-eviction artifact.

### 9. Manus — production context-engineering lessons
**Manus blog, July 2025, [URL](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus); [Lance Martin analysis](https://rlancemartin.github.io/2025/10/15/manus/).**
(a) **KV-cache hit rate = #1 production metric** — cached input ~10× cheaper ($0.30 vs $3.00/MTok Sonnet); byte-stable prompt prefixes (no timestamps), append-only context, deterministic serialization. (b) **File system as unlimited external context** with RESTORABLE compression (keep URL, drop page content). (c) **Recitation** — agent rewrites todo.md into recent context to hold goal attention. (d) **Mask tools, don't remove** (removal invalidates cache). (e) Keep errors in context (model learns from them).
**Applicability:** cheapest wins — audit per-phase prompt assembly for cache-busting dynamic tokens; inject artifacts by path (restorable) not fully inlined.

## Thread 3 — Agent Memory Systems

### 10. Mem0 — extraction + consolidation memory
**Chhikara et al., [arXiv 2504.19413](https://arxiv.org/abs/2504.19413), 2025; [2026 benchmark update](https://mem0.ai/blog/state-of-ai-agent-memory-2026), [research](https://mem0.ai/research).**
Extract salient facts → consolidate (add/update/delete/no-op). LOCOMO: **+26% over OpenAI memory, 91% lower p95 latency, >90% token cost reduction vs full-context** (26k tokens history → ~1.8k facts). 2026: 92.5 LoCoMo / 94.4 LongMemEval at **<7k tokens per retrieval**. CAVEAT ([controlled coding-agent benchmark](https://medium.com/@mrsandelin/the-first-controlled-benchmark-of-ai-memory-in-coding-agents-8e0bb776d39e)): vendor "90% savings" = memory-footprint compression, NOT end-task cost for coding agents.
**Applicability:** extract-then-consolidate fits knowledge-base/ — consolidation pass dedup+prune cycle learnings shrinks what scout injects per cycle.

### 11. A-MEM — Zettelkasten-style agentic memory
**Xu et al., [arXiv 2502.12110](https://arxiv.org/abs/2502.12110), 2025 ([GitHub](https://github.com/agiresearch/A-mem)).**
Atomic notes with contextual descriptions, dynamically indexed and LINKED; retrieval = selective top-k over note graph. Beats MemGPT/MemoryBank/ReadAgent on LoCoMo F1/BLEU-1 (strongest multi-hop) using **1,000–2,500 context tokens vs 16,000–17,000** baseline (~85–93% savings/op).
**Applicability:** knowledge-base cycle files already atomic notes; missing piece = linking + selective top-k injection — inject 3 most-relevant prior-cycle notes into scout, not the whole index.

### 12. Sleep-time Compute (Letta/UC Berkeley)
**Lin, Snell, Packer, Wooders, Stoica, Gonzalez et al., [arXiv 2504.13171](https://arxiv.org/abs/2504.13171), 2025 ([Letta blog](https://www.letta.com/blog/sleep-time-compute/)).**
Models "think" offline about persistent context BEFORE queries arrive, pre-computing inferences so test-time calls start warm. **~5× reduction in test-time compute at equal accuracy** (Stateful GSM-Symbolic/AIME); scaling sleep-time lifts accuracy **+13–18%**; cost amortizes across related queries. Follow-up: ["Language Models Need Sleep"](https://arxiv.org/html/2605.26099v1) (2026).
**Applicability:** retro/memo phases are natural sleep-time slots — pre-digest cycle history + repo state into a compact "warm-start brief" between cycles.

### 13. Memory consolidation gating — SAGE
**[arXiv 2605.30711](https://arxiv.org/pdf/2605.30711), 2026.**
**Novelty gate** for memory evolution: only sufficiently novel experiences trigger writes/updates — prevents unbounded memory growth + redundant-write token spend.
**Applicability:** novelty-gate knowledge-base writes — memo phase skips observations duplicating existing lessons (cheap similarity check in Go), keeping injectable corpus small.

## Thread 4 — Long-Horizon Coding-Agent Context Management

### 14. The Complexity Trap — observation masking beats LLM summarization ⭐
**JetBrains Research, [arXiv 2508.21433](https://arxiv.org/html/2508.21433), 2025; [blog](https://blog.jetbrains.com/research/2025/12/efficient-context-management/).**
KEY EMPIRICAL RESULT OF 2025 for coding agents. Observation tokens = **~84% of an average SWE-agent turn**; replacing observations older than a rolling window with placeholders (keeping full reasoning/action chain) matches or beats LLM summarization. SWE-bench Verified w/ Qwen3-Coder 480B: masking **52.7% cost reduction, 54.8% solve rate** vs summarization 50.4% / 53.8%; optimal window **M=10 turns**. Summarization ELONGATES trajectories 13–15%. Hybrid (mask + occasional summarize) saves further 7–11%.
**Applicability:** TOP deterministic win — observation masking in tmux/headless drivers (pure Go, no LLM calls), masking tool outputs older than ~10 turns within each phase session.

### 15. SWE-Pruner — adaptive context pruning middleware
**[arXiv 2601.16746](https://api.emergentmind.com/papers/2601.16746), Jan 2026.**
**0.6B neural skimmer** middleware between agent and file-reading tools; goal-hint-conditioned line-level pruning via CRF head + reranker, preserving code structure (**>87% AST correctness** vs <15% token-level). **23–38% token reduction on SWE-bench Verified** at comparable success, up to **26% fewer interaction rounds**, 29–54% on SWE-QA, **14.84× on LongCodeQA**, <100ms latency.
**Applicability:** tool-output interception (not the prompt) is the right layer — bridge could truncate/filter file-read results against the phase's goal, even heuristic (no trained skimmer).

### 16. Context as a Tool — agent-controlled context management
**[arXiv 2512.22087](https://arxiv.org/html/2512.22087v1), Dec 2025.**
Context ops (prune, fold, summarize) exposed as TOOLS the SWE-agent calls itself. Agent-managed context: **average token count stabilizes below 32k after ~100 rounds, no continuous growth** — vs unmanaged rolling contexts spanning millions/session ("accumulated context is not merely expensive but actively harmful"). Related: [SWE-EVO](https://arxiv.org/pdf/2512.18470).
**Applicability:** converges with #7 — give phase agents explicit "compact now"/"drop section" tools; bounded steady-state ~32k per phase is an achievable target metric.

## Top 5 Actionable Techniques (ranked for evolve-loop)

1. **Observation masking / stale tool-result eviction (rolling window ≈10 turns).** ≥50% cost reduction at equal-or-better solve rates, purely deterministic, implementable in Go drivers ([Complexity Trap](https://arxiv.org/html/2508.21433); [Anthropic context editing 84%](https://www.anthropic.com/news/context-management)). Do FIRST, before any compression scheme.
2. **Just-in-time artifact retrieval with per-phase section contracts.** Stop injecting full scout-report.md/build-report.md; inject ≤1–2k-token summary + file paths; each phase reads only sections its contract needs ([Anthropic JIT](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents); [Manus](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus); [A-MEM 1–2.5k vs 16–17k](https://arxiv.org/abs/2502.12110)).
3. **Structured eviction over blind summarization in artifact schemas.** Never-evict sections (decisions, acceptance criteria, open questions, dependency facts) vs evictable (exploration narrative, resolved-subtask detail); summaries destroy actionable detail + elongate trajectories 13–15% ([2606.11213](https://arxiv.org/pdf/2606.11213); [JetBrains](https://blog.jetbrains.com/research/2025/12/efficient-context-management/)).
4. **KV-cache-stable prompt prefixes + append-only assembly.** Byte-stable system/skill text, no timestamps in prefixes, deterministic serialization, mask-don't-remove tools: ~10× price differential on cached input ([Manus](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus)).
5. **Sleep-time consolidation between cycles.** Retro/memo as offline compute: consolidate knowledge-base (Mem0-style add/update/delete + SAGE novelty gate) into compact warm-start brief for next scout — 5×-compute / +13–18%-accuracy pattern amortizes across repeating cycles ([2504.13171](https://arxiv.org/abs/2504.13171); [2504.19413](https://arxiv.org/abs/2504.19413); [2605.30711](https://arxiv.org/pdf/2605.30711)).

**Honorable mention:** LLMLingua-2 at conservative 2–4× for bulky log/diff sections only. Soft-prompt methods (500xCompressor, gist) inapplicable to closed-CLI fleets — track, don't build.

**Cross-cutting caution:** vendor memory-savings numbers (85–93%) measure per-memory-op footprint, NOT end-task cost for coding agents — validate against the pipeline's own per-cycle token telemetry.
