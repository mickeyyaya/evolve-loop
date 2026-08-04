# Part 2 — Multi-Agent LLM Token Economics (2025–2026 survey)

> Research agent sweep 2 of 3, 2026-07-05, for evolve-loop token-optimization goal 805f6ced.
> Target: CI pipeline (scout→triage→tdd→build→audit→ship), separate CLI invocations passing markdown artifacts + 2-lane fleet.

## Thread 1 — Inter-agent communication efficiency

### 1. Anthropic multi-agent research system — the 15× token multiplier (2025)
Orchestrator-worker: lead agent spawns 3–5 parallel subagents; each gets "objective, output format, tool/source guidance, task boundaries" and returns structured findings, not transcripts.
**Quant:** agents ~4× chat tokens; multi-agent ~15×; token usage explains ~80% of performance variance on browsing; parallel subagent+tool calls cut research time up to 90%. Rule: multi-agent only pays when task value exceeds token premium. ([anthropic.com](https://www.anthropic.com/engineering/multi-agent-research-system))
**Applicability:** effort-scaling rule — give scout/triage explicit tool-call + output budgets per task complexity.

### 2. Cognition "Don't Build Multi-Agents" — context-sharing argument (2025)
(1) Share full agent traces, not just messages; (2) actions carry implicit decisions → parallel siblings without shared context diverge. Recommends single-threaded linear agents + dedicated context-compression model for long traces. No benchmarks (architectural). ([cognition.com](https://cognition.com/blog/dont-build-multi-agents), [follow-up](https://cognition.com/blog/multi-agents-working))
**Applicability:** the sequential phase chain is exactly the recommended shape; risk point = 2-lane fleet — lanes must be truly disjoint (triage-disjointness gate is the ECONOMIC gate, not just correctness).

### 3. DroidSpeak — KV-cache sharing cross-LLM (arXiv 2411.02820, MSR/UChicago)
Agent B reuses most of agent A's KV-cache layers, recomputes few. **Quant:** up to 4× throughput, ~3.1× faster prefill/TTFT, negligible loss. ([arxiv](https://arxiv.org/abs/2411.02820))
**Applicability:** serving-layer only — argument for keeping phases on same provider/model family per cycle so provider caches do the equivalent.

### 4. KVComm — cross-context KV reuse (arXiv 2510.12872, 2025)
Solves "same content, different prefix offset": anchor pool of cache deviations lets agents reuse KV segments at different positions. **Quant:** >70% reuse; up to 7.8× prefill speedup (5-agent); TTFT 430ms→55ms; zero quality loss. ([arxiv](https://arxiv.org/html/2510.12872))
**Applicability:** design rule — put the shared artifact (scout-report.md) at an IDENTICAL PREFIX POSITION in every downstream phase prompt.

### 5. AgentPrune "Cut the Crap" — communication-graph pruning (ICLR 2025, arXiv 2410.02506)
Formalizes communication redundancy; prunes low-value edges of the message-passing graph one-shot. **Quant:** 28.1–72.8% token reduction at comparable accuracy; SOTA topologies matched at $5.6 vs $43.7; GSM8K+GPTSwarm −60.6% prompt tokens (+0.84% accuracy). ([arxiv](https://arxiv.org/abs/2410.02506))
**Applicability:** prune the artifact-flow graph — audit doesn't need scout's full backlog; ship doesn't need tdd's red-phase log; each phase receives only its true dependency edges.

### 6. Sparse/dynamic communication topologies (2025–2026 cluster)
Guided Topology Diffusion ([2510.07799](https://arxiv.org/abs/2510.07799)); GoAgent **17% token reduction** at maintained accuracy ([2603.19677](https://arxiv.org/pdf/2603.19677)); small-world connectivity near-FC accuracy ([2512.18094](https://arxiv.org/html/2512.18094)); sparse debate ([2406.11776](https://arxiv.org/html/2406.11776v1)).
**Applicability:** linear pipeline is already maximally sparse — WARNING against adding edges (no auditor↔builder round-trips; verdict-file handoff is token-optimal).

### 7. Single-agent matches multi-agent at equal token budgets (arXiv 2604.02460, 2026)
Controlled (Qwen3, R1-Distill, Gemini 2.5): tokens held constant → single-agent matches/beats MAS on multi-hop reasoning; published MAS advantages often "unaccounted computation and context effects." ([arxiv](https://arxiv.org/pdf/2604.02460))
**Applicability:** justify each phase-split by ROLE-ISOLATION value (anti-cooperative-bias auditor, clean-context TDD), not assumed quality; consider merging low-value adjacent phases (scout+triage on trivial cycles).

## Thread 2 — Orchestrator context hygiene

### 8. Anthropic "Effective Context Engineering" (2025)
Orchestrator holds plans/synthesis only; subagents return distilled summaries; raw tool results cleared once consumed ("safest, lightest-touch compaction"); durable state → external files. **Quant:** subagents use tens of thousands of tokens but return **1,000–2,000-token summaries**. ([anthropic](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents))
**Applicability:** "conclusions-not-transcripts" contract at every phase boundary; 1–2K-token return budget = concrete number to enforce on reports.

### 9. Subagent isolation measured compression (Claude Code ecosystem, 2025–2026)
Documented trace: subagent reads 6,100 tokens, returns **420-token summary (~14×)**; rule: 10K work returning 500 saves 9.5K; returning 8K "detailed report" saves almost nothing. ([richsnapp.com](https://www.richsnapp.com/article/2025/10-05-context-management-with-subagents-in-claude-code), [code.claude.com](https://code.claude.com/docs/en/sub-agents))
**Applicability:** REPORT SIZE IS THE WHOLE GAME — bloated build-report.md destroys isolation benefit for audit; cap and lint artifact sizes.

## Thread 3 — Prompt/KV caching economics

### 10. Anthropic prompt caching (docs, 2025–2026)
`cache_control` breakpoints; reads **0.1× base input**; writes 1.25× (5-min TTL) / 2× (1-h TTL); break-even 1 read (5-min) / 2 reads (1-h); up to 90% cost + 85% latency cuts. E.g. 15K-token preamble $0.045→$0.0045/request. ([docs](https://platform.claude.com/docs/en/build-with-claude/prompt-caching))
**Applicability:** phases re-send large stable preambles every invocation — 1-h-TTL cache amortizes across all 6 phases AND both fleet lanes if prefix is byte-identical.

### 11. OpenAI automatic prefix caching (2024→2026)
Automatic 50% discount on longest previously-seen prefix (≥1,024 tokens, 128-token increments); covers messages/tool defs/schemas; evicted 5–10 min idle, ≤1 h. Exact-prefix match — early-token change invalidates everything after. ([openai](https://openai.com/index/api-prompt-caching/))
**Applicability:** codex phases — volatile content (cycle number, task list, timestamps) at prompt END; back-to-back phase scheduling increases hits.

### 12. "Don't Break the Cache" (arXiv 2601.06007, 2026)
First systematic eval of provider caching in real agent loops. **Quant:** 20–40% overall cost savings (41–80% well-tuned); TTFT 13–31%; hit rates 60–85% stable loops, collapsing on context mutation. **Strategic boundary control (cache system prompt + tool defs, exclude dynamic tool results) beats naive full-context caching.** ([arxiv](https://arxiv.org/pdf/2601.06007))
**Applicability:** breakpoint between {static: rules+skill+tool defs} and {dynamic: cycle artifacts}; never inject per-cycle values into the static block.

### 13. Manus — "KV-cache hit rate is the #1 metric" (2025)
Append-only context; no timestamps in system prompts; deterministic JSON key order; never add/remove tools mid-run (mask logits); large content → files with references. **Quant:** 10× price difference cached vs not; agentic input:output ~**100:1** → input caching dominates cost. ([manus](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus))
**Applicability:** per-phase tmux sessions: stable prefix, artifacts-by-file-reference, audit prompt templating for cycle-varying strings early in prompt.

### 14. TokenPilot — cache-efficient context management (arXiv 2606.17016, 2026)
Preserves high-value segments, evicts low-confidence intermediate state, pre-caches recurring tool patterns. **Quant:** 30–50% API cost cut, 40–60% fewer tokens, 25–35% latency cut; hit rates 60–80% vs 10–20% baseline. ([arxiv](https://arxiv.org/pdf/2606.17016))
**Applicability:** clear raw test/build tool output after verdict extraction — keep only pass/fail summary.

## Thread 4 — Structured output as token control

### 15. Structured outputs: schema as SIZE CONTRACT, not raw saver (2024–2026)
Schema-forced output doesn't inherently shrink — tiny payloads get ~40% MORE tokens from scaffolding; full structured responses can be 2–3× a free-text summary. BUT (a) constrained decoding can skip deterministic boilerplate; (b) bounded fields (enums, maxItems, fixed sections) = enforceable report-size contract. Caveats: +10–30% latency; format restrictions can degrade reasoning ("Let Me Speak Freely?"; GPT-5 extraction pass rates 86.9%→70.0% under structured mode). ([aidancooper](https://www.aidancooper.co.uk/constrained-decoding/), [2408.02442](https://arxiv.org/pdf/2408.02442), [2602.12247](https://arxiv.org/pdf/2602.12247))
**Applicability:** free-form reasoning INSIDE phases; schema-bound HANDOFF with hard caps (top_n≤N, findings≤K, ≤2K tokens) — the win is bounding downstream INPUT.

## Thread 5 — Benchmarks quantifying pipeline waste

### 16. AgentTaxo — the "communication tax" (ICML 2025)
Token distribution across linear (MetaGPT), flat (CAMEL), hierarchical (AgentVerse); DUPLICATED tokens = dominant inefficiency. **Quant:** input:output **2:1–3:1** — context (re-)loading dominates. ([icml](https://icml.cc/virtual/2025/49320), [openreview](https://openreview.net/forum?id=0iLbiYYIpC))
**Applicability:** measure per-phase input:output ratio (ledger/usage plumbing exists) — any phase >~3:1 is re-ingesting artifacts it doesn't need.

### 17. Tokenomics — where tokens go in agentic SWE (arXiv 2601.14470, 2026)
ChatDev+GPT-5, 30 SWE tasks. **Quant:** **code review = 59.4% of all tokens** (iterative loops); coding 8.6%; design 2.4%; input ~54% of consumption at ~2:1; "naive full-context passing" during verification = named culprit. ([arxiv](https://arxiv.org/html/2601.14470v1))
**Applicability:** audit/adversarial phases are statistically THE token hog — bound audit iterations, pass diffs+reports never full-tree.

### 18. Token Economics for LLM Agents — survey (arXiv 2605.09104, 2026)
Tokens as economic primitive; "Coasian boundary" N* for optimal agent-team size. **Quant:** OpenRouter weekly tokens 68× growth (0.4T→27T Dec'24→Mar'26); agentic codegen >1000× single-turn tokens; MAS transaction costs ~O(|V|²); **token usage varies up to 30× across identical runs**; models can't predict own consumption (r≤0.39). Recommends hard loop budgets/stop-losses + per-role attribution. ([arxiv](https://arxiv.org/html/2605.09104))
**Applicability:** per-phase token stop-losses in orchestrator (kill/retry runaway phase); O(|V|²) caps fleet-lane growth economics.

### 19. Latent/compressed inter-agent channels — frontier (2025–2026)
Hidden-state exchange ([2511.09149](https://arxiv.org/html/2511.09149v1)); HyLaT hybrid latent-text ([2605.25421](https://arxiv.org/pdf/2605.25421)); EcoLANG induced compressed languages ([2505.06904](https://arxiv.org/pdf/2505.06904)); action-state communication ([2606.05304](https://arxiv.org/pdf/2606.05304)). Latent messages ~8 decoding steps preserving success.
**Applicability:** not implementable across vendor CLIs; transferable idea = ACTION-STATE HANDOFFS — pass "what was decided/changed" (state deltas) not transcripts.

## Top 5 actionable (system-level)

1. **Cache-aligned stable prompt prefixes per phase** — {byte-identical static: rules/skill/tool defs} + {dynamic tail: artifacts}; no timestamps/cycle numbers in static block; 1-h TTL on Anthropic; ≥1,024-token stable prefixes on OpenAI. Input = 54–75% of spend at 2:1–100:1 ratios; 50–90% unit-price cut; amplified by fleet if lanes share prefix.
2. **Hard report-size contracts at every phase boundary** — schema-bound artifacts, ~1–2K tokens, bounded lists, validated by Go orchestrator before next phase launches.
3. **Prune the artifact-dependency graph** — each phase gets only true dependency edges (audit: diff+build-report; ship: verdicts). Measured 28–73% reduction.
4. **Budget the verification loop specifically** — audit ≈59% of agentic-SWE tokens; cap iterations, scoped diffs, clear raw tool output post-verdict.
5. **Per-phase token stop-losses + attribution** — 30× run variance, self-prediction r≤0.39 → deterministic per-phase budgets with kill/retry + per-phase/lane attribution logging.

**Fleet note:** equal-budget study + Cognition argument ⇒ lanes only pay when work is truly disjoint — triage-disjointness is the ECONOMIC gate; identical static prefixes across lanes turn lane-2 prefill into 0.1× cache reads.
