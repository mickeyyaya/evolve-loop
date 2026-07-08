# Token Reduction in Concurrent / Multi-Agent / Fleet LLM Environments — 2025–2026 Research Sweep

> Researched 2026-07-07 for evolve-loop (2+ concurrent lanes × ~12 tmux-driven phase agents/cycle + ~12 advisor calls).
> Complements `knowledge-base/research/token-optimization-2026/` (per-agent optimization). This sweep = the **concurrency/fleet** angle: cross-agent sharing, cache-aware scheduling, hierarchical planning, fleet admission control, stage redundancy.
> Applicability notes assume evolve-loop's constraints: closed CLIs (claude-tmux/codex-tmux), subscription OAuth quotas (not raw API TPM), no control over provider serving stack.

---

## 1. Cross-agent context sharing & deduplication

### 1.1 DeLM — Decentralized Multi-Agent Systems with Shared Context
- **Source:** arXiv [2606.10662](https://arxiv.org/html/2606.10662v1), Jun 2026.
- **Mechanism:** Replaces central orchestrator with a **shared context 𝒞 + task queue 𝒯**. Parallel agents asynchronously claim subtasks, read accumulated progress, and write back *compact verified updates*. Shared context is hierarchical: raw source → reference-grounded summaries → compact gists; agents read **gists by default** and expand to detail only on demand. An **admission-time verification gate** LLM-checks every update against evidence before it enters shared state (prevents hallucination poisoning of reusable state).
- **Numbers:** SWE-bench Verified 77.4% Pass@4 at **~50% lower cost** ($0.12/task vs $0.24–0.73 for baselines); +9.3pp Avg@1 over strongest baseline; LongBench-v2 +3.6–5.7pp across 4 model families. Savings attributed to "preventing redundant exploration, sharing failures as constraints, and compressing trajectories rather than exposing full raw traces."
- **Applicability:** Direct blueprint for a per-run (and cross-lane) shared scratchpad: lanes/phases publish gist-level facts ("pkg X exports Y", "test Z flaky") to a verified board instead of each phase re-grepping the repo. The verification-gate idea maps onto evolve-loop's contract-gate — validate before a fact becomes shared state.

### 1.2 LLM-based Multi-Agent Blackboard System
- **Source:** arXiv [2510.01285](https://arxiv.org/abs/2510.01285), Sep 2025 (rev Jan 2026).
- **Mechanism:** Central agent posts *requests* to a shared blackboard; subordinate agents monitoring it **volunteer** responses based on capability. Removes the orchestrator's need to hold complete knowledge of every agent (and to re-broadcast context to each), cutting coordination token overhead in large fleets.
- **Numbers:** End-to-end success **+13–57%** vs baselines on KramaBench/DSBench/DA-Code; token savings implied via eliminated per-agent briefing/broadcast, not separately quantified.
- **Applicability:** Matches evolve-loop's `.evolve/inbox/` pattern — pull-based task claiming beats push-based briefing when lanes scale; the win is architectural (no N× re-briefing), not a tuning knob.

### 1.3 Multi-Agent Transactive Memory (MATM)
- **Source:** arXiv [2606.19911](https://arxiv.org/pdf/2606.19911), Jun 2026. Code: github.com/kimdanny/matm.
- **Mechanism:** Agents maintain a distributed **"who knows what" registry** (transactive memory, borrowed from org psychology). Before re-reading/re-exploring, an agent queries whether a teammate already holds the information; only on miss does it explore itself.
- **Numbers:** "Substantially fewer tokens" vs independent-exploration baselines on web/text environments (paper reports token-savings metrics; exact % in full text/dataset, not abstract).
- **Applicability:** The cheapest form of cross-lane dedup: an index of *what has already been read/derived this run* ("scout already mapped internal/policy") that later phases and sibling lanes consult before dispatching exploration. Fits a Go-side JSON registry keyed by file/topic.

### 1.4 SafeSieve — progressive pruning of inter-agent communication
- **Source:** arXiv [2508.11733v3](https://arxiv.org/html/2508.11733v3), rev Mar 2026.
- **Mechanism:** Dual-stage pruning of the agent communication graph: semantic-compatibility initialization of edge scores, then progressive shift to accumulated *performance feedback* (which links actually contributed to correct answers); 0-extension clustering prunes ineffective links while keeping coherent agent groups.
- **Numbers:** **12.4–27.8% token reduction** across benchmarks with accuracy UP (avg 94.01%; HumanEval 88.43→95.01%); **13.3% cost reduction** in heterogeneous multi-model deployment; robust to malicious-agent injection (−1.23% vs −4.59% for competitors).
- **Applicability:** Evidence that pruning *which artifacts flow to which phase* by measured contribution (not by "give everyone everything") saves 12–28% and can raise quality. Reinforces the artifact-dependency-graph pruning item (B2) already queued from AgentPrune (28.1–72.8% token reduction, arXiv [2410.02506](https://arxiv.org/abs/2410.02506)); SafeSieve adds the *learn-from-cycle-history* refinement.

---

## 2. Prompt/KV-cache-aware scheduling for parallel agent sessions

*(Serving-layer papers are not directly deployable against closed CLIs, but their scheduling insights translate into dispatch-order and prefix-discipline policies the orchestrator does control.)*

### 2.1 KVFlow — workflow-aware KV cache management
- **Source:** arXiv [2507.07400](https://arxiv.org/abs/2507.07400) ([OpenReview](https://openreview.net/forum?id=5Iw1nDtYmT)), Jul 2025.
- **Mechanism:** Abstracts the agent execution schedule as an **Agent Step Graph**; each agent's cached prefix gets a *steps-to-execution* value (temporal proximity to reuse) instead of LRU. Evicts caches farthest from reuse; prefetches for agents about to run.
- **Numbers:** **1.83× speedup** single workflow with large prompts; **2.19× speedup** with many concurrent workflows vs SGLang hierarchical radix cache.
- **Applicability:** The translated policy: the orchestrator KNOWS the phase DAG — dispatch phases so sessions sharing a prefix run temporally adjacent (within provider cache TTL), and don't interleave unrelated lanes' phases in a way that thrashes the provider cache between same-prefix calls.

### 2.2 Continuum / CacheTTL — TTL-aware KV retention for tool-calling agents
- **Source:** arXiv [2511.02230](https://arxiv.org/abs/2511.02230), Nov 2025 (rev May 2026).
- **Mechanism:** During tool-execution pauses (the signature of agentic workloads), retain KV cache with a **time-to-live computed from reload cost vs memory-blocking cost**; combine with program-level FCFS so multi-turn continuity is preserved rather than re-prefilling each turn.
- **Numbers:** Up to **8× improvement in average job completion time** on SWE-Bench/BFCL/OpenHands-style agents (Llama-3.1 8B/70B, Gemma-3 12B, GLM-4.5 355B).
- **Applicability:** Mirror of the provider-side 5-min prompt-cache TTL: evolve-loop phases that block >TTL on tests/builds return to a **cold cache** and pay full re-prefill. Policy: keep a phase session "warm" (or sequence long tool waits to phase boundaries), and never let an active session idle past the TTL mid-conversation.

### 2.3 Helium — "Efficient LLM Serving for Agentic Workflows: A Data Systems Perspective"
- **Source:** arXiv [2603.16104](https://arxiv.org/html/2603.16104v1), Mar 2026.
- **Mechanism:** Treats the agent workflow as a **query plan**: (a) Templated Radix Tree captures prompt-prefix hierarchy + inter-operator dependencies across the whole workflow; (b) proactive cache pre-warming of static prefixes identified at compile time; (c) **cache-aware cost-based scheduling** (assign operators to workers + order execution to maximize prefix reuse); (d) logical-plan optimization — common-subexpression elimination replaces redundant operators with cache fetches.
- **Numbers:** **1.56× over KVFlow**, 1.83× over LangGraph, 4.32× over AgentScope, **39.5× over naive vLLM** end-to-end; **+32.9% prefix-cache hit rate** vs online strategies; scheduling within **0.9% of theoretical optimum** (online heuristics >30% gap).
- **Applicability:** Strongest conceptual match to a phase pipeline: evolve-loop's recipe/prompt assembly is exactly a "compilable plan" — static prefix segments per phase are known ahead of dispatch, so cache-hit-maximizing dispatch order is computable in Go before any LLM call.

### 2.4 TokenDance — collective KV cache sharing across agents
- **Source:** arXiv [2604.03143](https://arxiv.org/abs/2604.03143), Apr 2026.
- **Mechanism:** In All-Gather multi-agent patterns (scheduler collects all agents' outputs, redistributes combined context), every agent stores near-identical KV. A **KV Collector** does the reuse for the whole round in one collective step; **diff-aware storage** encodes sibling caches as block-sparse diffs against a master copy.
- **Numbers:** **2.7× more concurrent agents** than vLLM prefix caching under SLO; **up to 17.5× per-agent KV storage reduction** (11–17× compression); 1.9× prefill speedup.
- **Applicability:** Quantifies how much of concurrent-agent context is literally duplicated (11–17×!). For evolve-loop: the same repo brief/CLAUDE.md/policy text flowing into 12 phase sessions × 2 lanes is the dominant duplicated payload — factor it into one shared, byte-stable prefix rather than N bespoke assemblies.

### 2.5 "Don't Break the Cache" — prompt caching under long-horizon agentic tasks
- **Source:** arXiv [2601.06007v2](https://arxiv.org/html/2601.06007v2), Jan 31 2026.
- **Mechanism:** Systematic evaluation of provider prompt caching (OpenAI/Anthropic/Google) on agentic workloads. Cache-breakers: timestamps/session IDs in system prompts, **dynamic tool definitions (esp. MCP)**, variable tool results, unpredictable history growth. Key trap: "naively enabling full-context caching can paradoxically increase latency" — cache writes for content never reused.
- **Numbers:** **41–80% cost reduction** (GPT-5.2 / Claude Sonnet 4.5 / Gemini 2.5 Pro); 13–31% TTFT reduction; savings scale ~linearly with prompt size (10–89% from 500→50K tokens); stable across 3–50 tool calls.
- **Recommendations:** cache only stable content; put cache-breaking IDs *after* the static prompt; prefer code-generation over per-request dynamic function defs; bigger static prefixes → bigger wins.
- **Applicability:** Directly actionable audit list for evolve-loop's prompt assembly (extends queued A2 "cache-stable prefixes" with the *dynamic-tool-definition* and *deliberate cache-breakpoint placement* findings). MCP tool lists that vary per session are a named cache killer.

### 2.6 Provider cache economics + cache-aware rate limits (the quota multiplier)
- **Sources:** Anthropic [token-saving updates](https://claude.com/blog/token-saving-updates) (Mar 2025), [Claude rate-limits docs](https://platform.claude.com/docs/en/api/rate-limits), practitioner guides ([TrueFoundry](https://www.truefoundry.com/blog/provider-agnostic-prompt-caching-llm-gateway), [2026 engineering guides](https://jobsbyculture.com/blog/prompt-caching-engineers-guide-2026)).
- **Mechanism/economics:** Anthropic cache reads = **0.1× input price** (5-min TTL write +25%, 1-h TTL write +100%; 1-h TTL breaks even after 2 hits); OpenAI automatic prefix caching ≈ **50% discount**; Gemini 2.5 ≈ 90%. **Cache-aware rate limits:** on the Claude API, cache-read tokens **do not count toward ITPM** (only uncached input + cache-creation do; Haiku 3.5 excepted) — e.g. a 2M ITPM limit with an 80% hit rate processes an effective **10M tokens/min**.
- **Numbers:** up to 90% cost / 85% latency reduction on long prompts; token-efficient tool use −14% output avg (up to −70%).
- **Applicability:** For the operator's *rate-benched provider* scenario this is the single biggest lever on API-key traffic: cache hits don't just cut cost, they **multiply effective quota ~5×** at 80% hit rate. (Caveat: subscription/OAuth seats meter differently — see 2.7.)

### 2.7 Claude Code cache-TTL churn & subscription quota drain (practitioner reality)
- **Source:** [devclass, 2026-04-14](https://www.devclass.com/ai-ml/2026/04/14/claude-code-cache-confusion-as-anthropic-tweaks-defaults-but-quotas-still-drain/5216975).
- **Mechanism:** Anthropic flipped Claude Code's cache TTL 5min→1h (Feb 2026) → back to 5min (Mar 2026). With 1M-token context windows, a **stale session resumed after >TTL = full cache miss** = massive quota hit. Users report $200/mo seats hitting limits for the first time; Pro seats "as few as two prompts in five hours"; an enterprise seat maxing session usage in <2h with "overthinking loops".
- **Applicability:** Evolve-loop's tmux sessions on OAuth seats live exactly here. Fleet policies: (a) don't resume stale phase sessions after long gaps — restart with a compact brief instead of re-prefilling a giant history; (b) time-pack each phase's LLM turns densely inside the 5-min TTL window; (c) treat long-idle-then-continue as the most expensive pattern in the system.

---

## 3. Hierarchical planning to cut redundant exploration

### 3.1 Anthropic — "How we built our multi-agent research system"
- **Source:** [Anthropic engineering blog](https://www.anthropic.com/engineering/multi-agent-research-system), Jun 2025.
- **Mechanism:** Orchestrator-worker: Opus lead plans and **compresses the question into distilled briefs**; 3–5 Sonnet subagents explore in parallel in isolated context windows; each condenses to "the most important tokens" for the lead; separate citation pass.
- **Numbers:** **Token usage alone explains 80% of performance variance**; multi-agent ≈ **15× the tokens** of chat; Opus-lead + Sonnet-workers beat single Opus by **90.2%** on research evals. Explicit caution: domains needing shared context with many inter-dependencies — "most coding tasks" — are a poor fit for wide parallelism.
- **Applicability:** Canonical planner-reads-broad / workers-get-briefs pattern. For evolve-loop: scout (broad) → triage (distill) → builder (narrow brief) already matches; the discipline is making sure builder/audit get *briefs*, never scout's raw exploration. Also a warning against over-parallelizing tightly-coupled build work across lanes.

### 3.2 Lemon Agent Technical Report
- **Source:** arXiv [2602.07092](https://arxiv.org/html/2602.07092v1), Feb 2026.
- **Mechanism:** AgentCortex Planner-Executor-Memory; orchestrator activates multi-worker mode **only when the task has multiple independent sub-goals** (adaptive width — echoes evolve-loop's lane-width question); three-tier progressive context compression (intra-tool truncation → intra-round summarization → cross-round retroactive compression with in-place replacement).
- **Numbers:** GAIA 91.36%, xbench-DeepSearch 77+; token-efficiency %s not published.
- **Applicability:** "Width only when sub-goals are independent" is the planning-side analog of the fleet's real-files disjointness work (L2) — parallelism is *earned by decomposition*, not defaulted.

### 3.3 Scoped sub-goal contexts (Task-Decomposed Planning)
- **Source:** reported in "From Agent Loops to Structured Graphs: A Scheduler-Theoretic Framework for LLM Agent Execution", arXiv [2604.11378](https://arxiv.org/html/2604.11378v1), Apr 2026. *(Claim from search abstract — verify against full text before citing in an ADR.)*
- **Mechanism:** Decompose task into a DAG of sub-goals, each executing with a **scoped context** that isolates its history from the global context.
- **Numbers:** up to **82% token reduction** attributed to context isolation.
- **Applicability:** Strongest single number for "phase gets only its scope, not the run's whole history" — supports hard-scoping each phase agent's context to its contract inputs.

### 3.4 Related: hierarchical/difficulty-aware orchestration
- [Difficulty-Aware Agent Orchestration](https://arxiv.org/html/2509.11079v1) (Sep 2025) — route easy tasks to shallow/cheap paths, deep multi-agent only when needed; [Benchmarking Multi-Agent Architectures for Financial Document Processing](https://arxiv.org/html/2603.22651v1) (Mar 2026) — "token efficiency is high when each agent receives only the relevant document sections"; [AgentArk](https://arxiv.org/html/2602.03955v1) (Feb 2026) — distill a multi-agent system's behavior into ONE agent, eliminating inter-agent token traffic entirely for matured workflows.
- **Applicability:** AgentArk is a provocative endgame for evolve-loop's stable phases: once a phase pair's interaction is predictable, collapse it into a single cheaper call.

---

## 4. Token budgeting / admission control across a fleet

### 4.1 HiveMind — OS-inspired scheduling for concurrent LLM agent workloads ⭐
- **Source:** arXiv [2604.17111](https://arxiv.org/html/2604.17111v1), Apr 2026 (Agyemang, Kponyo, Somuah et al.).
- **Mechanism:** Transparent HTTP proxy with 5 primitives: (1) **admission control** — condition-variable gate on in-flight requests; (2) **dual rate-limit tracking** — reactive header parsing + proactive sliding-window RPM/TPM counters; (3) **AIMD backpressure + circuit breaker** (TCP-style congestion control on latency, breaker on sustained errors); (4) **per-agent token budgets** — warn at 85%, checkpoint at 100%; (5) priority queue over a dependency DAG ordered by priority/estimated-token-cost/age.
- **Numbers:** uncoordinated concurrency at 10+ agents = **72–100% failure**; HiveMind = **0–18%**. **Wasted tokens by dead agents −48–100%**; eval-workload daily cost **$6.55 → $0.24 (−96%)** at Opus pricing; <3ms proxy overhead; direct-mode throughput collapses to zero beyond 5 agents while HiveMind scales linearly. Ablation: **transparent centralized retry is the single most impactful primitive** (removing it → 63.6% failure); admission control alone is insufficient (81.8% failure).
- **Applicability:** The closest published system to the operator's exact pain (concurrent lanes + rate-benched provider). Two transplantable lessons: (a) the biggest waste is **tokens burned by agents that eventually die** — a lane that fails at phase 9 wastes its whole cycle; centralized retry + checkpointing at budget exhaustion is what recovers that; (b) fleet admission must be **provider-quota-aware at SELECTION time**, not just dispatch (rhymes with the fleet priority-inversion fix).

### 4.2 Agent Contracts — formal resource-bounded autonomy
- **Source:** arXiv [2601.08815](https://arxiv.org/pdf/2601.08815), Jan 2026.
- **Mechanism:** Formal contracts binding an agent to explicit resource budgets (tokens/calls/time) with defined degraded behaviors on exhaustion — makes "what happens when a lane runs out" a typed contract rather than an emergent failure.
- **Applicability:** Formal backing for `fleet.budget` (Q1–Q4 already shipped): per-lane budgets should carry a *contracted degradation* (checkpoint + hand back partial artifacts), not a mid-phase death.

### 4.3 Gateway practice — per-tenant budgets, fail-fast, adaptive limits
- **Sources:** [agentgateway](https://agentgateway.dev/docs/kubernetes/2.2.x/llm/rate-limit/), [Portkey](https://portkey.ai/blog/rate-limiting-for-llm-applications/), [TrueFoundry](https://www.truefoundry.com/blog/rate-limiting-in-llm-gateway), [Zuplo 2026](https://zuplo.com/learning-center/token-based-rate-limiting-ai-agents), 2025–2026.
- **Mechanism:** Production consensus: single choke-point owns the provider key; per-tenant (per-lane) token budgets enforced *pre-flight* — over-budget requests **fail fast with 429 and never reach the provider**; 429/quota-exhaustion/outage degrade via **routing + model downgrade, not 500s**; limits adapt to real-time provider headroom (looser off-peak, tighter under contention).
- **Applicability:** Corroborates evolve-loop's usageprobe + signalcenter direction; the missing piece in most setups (and partially in evolve-loop) is *pre-flight estimated-cost admission* per phase dispatch, not post-hoc accounting.

---

## 5. Redundancy elimination between pipeline stages

### 5.1 "Redundant or Necessary?" — benchmark of redundant agent steps
- **Source:** arXiv [2605.29893](https://arxiv.org/html/2605.29893), May 2026.
- **Mechanism:** Taxonomy of trajectory redundancy: **abnormal steps** (tool failures), **duplicated steps** (identical call, unchanged result), **incorrect steps**, **exploratory steps** (investigation that never feeds the solution). Evaluates LLM detectors (one-to-one / window-to-one / all-to-all).
- **Numbers:** Detection is HARD — best step-level score only **24.88%**; best trajectory-level (GPT-5.4, ±3-step window) 70.88%.
- **Applicability:** Two readings: (a) duplicated tool calls with unchanged results are a mechanically detectable class — a Go-side **tool-call result memo (hash → cached result)** inside a phase session catches them deterministically, no LLM detector needed; (b) don't build an LLM-based "redundancy judge" — the SOTA is too weak.

### 5.2 Agent-Omit — adaptive omission of thoughts/observations
- **Source:** arXiv [2602.04284](https://arxiv.org/abs/2602.04284), Feb 2026 (rev May 2026).
- **Mechanism:** Trains agents (cold-start synthetic data + RL with omission reward) to adaptively drop redundant *thoughts* and *observations* from their own interaction history.
- **Numbers:** 8B model matches 7 frontier agents' performance with a better efficiency-effectiveness frontier (abstract gives no single %).
- **Applicability:** Learned counterpart of the already-queued deterministic observation-masking (A1). Confirms the target class; evolve-loop should keep the deterministic version (closed CLIs can't be fine-tuned).

### 5.3 Spec Kit Agents — phase-scoped evidence collection
- **Source:** arXiv [2604.05278](https://arxiv.org/html/2604.05278v1), Apr 2026.
- **Mechanism:** Orchestrated spec-driven pipeline where **read-only discovery hooks probe repo state before each phase** and pin phase-scoped evidence, instead of each phase's LLM re-doing best-effort retrieval. Also: early subagents lacking stop conditions **re-read the same files unboundedly**; explicit stop conditions + anti-loop instructions fixed it.
- **Applicability:** "Grounding as a deterministic pre-phase operation" = evolve-loop could compute each phase's evidence pack in Go (diff, exports, failing tests) and hand it over, replacing N× LLM-driven repo re-exploration. The stop-condition lesson applies verbatim to phase prompts.

### 5.4 Evaluating AGENTS.md — context files can INCREASE cost
- **Source:** arXiv [2602.11988](https://arxiv.org/html/2602.11988v1), Feb 2026.
- **Mechanism/Numbers:** Repository context files (AGENTS.md-style) induce more exploration/testing/reasoning by coding agents and **increase costs by >20%** without reliable quality gains.
- **Applicability:** Counterintuitive and directly relevant: evolve-loop injects CLAUDE.md/AGENTS.md into ~24 sessions/cycle. Audit which phases actually need the full instruction stack vs a role-scoped digest — the instruction payload itself is a per-agent tax that can *provoke* extra spend.

### 5.5 ProcMEM & artifact reuse across runs
- **Source:** via [VoltAgent awesome-ai-agent-papers](https://github.com/VoltAgent/awesome-ai-agent-papers) (2026 collection); ProcMEM = procedural-skill memory reuse across runs.
- **Mechanism:** Save step-level procedural artifacts (plans, workflows, reusable code, traces) from past runs; reuse instead of re-deriving.
- **Applicability:** Evolve-loop's knowledge-base already stores lessons; the gap is *machine-consumed* procedural artifacts (e.g., "how to run this repo's tests" derived once, injected forever) so no phase ever re-derives environment facts.

---

## 6. Cross-cutting measurement table

| Technique | Source | Savings / effect |
|---|---|---|
| Shared verified context + gist hierarchy (DeLM) | 2606.10662 | **−50% cost/task** on SWE-bench Verified, accuracy UP |
| Fleet admission + budgets + retry (HiveMind) | 2604.17111 | dead-agent waste **−48–100%**; eval cost **−96%**; failures 72–100%→0–18% |
| Communication-graph pruning (SafeSieve / AgentPrune) | 2508.11733 / 2410.02506 | **−12.4–27.8%** / **−28.1–72.8%** tokens, accuracy flat/up |
| Strategic prompt caching in agentic loops | 2601.06007 | **−41–80% cost**, −13–31% TTFT |
| Cache-aware rate limits (Anthropic API) | claude.com/blog | cache reads exempt from ITPM → **~5× effective quota** @80% hit |
| Scoped sub-goal contexts | 2604.11378 (verify) | up to **−82% tokens** |
| Workflow-aware cache scheduling (KVFlow/Helium) | 2507.07400 / 2603.16104 | 1.83–2.19× / up to 1.56× over KVFlow, **+32.9% cache-hit rate** |
| TTL-aware retention across tool pauses (Continuum) | 2511.02230 | up to **8× job-completion time** |
| Collective KV dedup across agents (TokenDance) | 2604.03143 | 2.7× concurrency, **11–17× duplicated-context compression** |
| Planner-worker distillation (Anthropic) | eng blog | token spend explains **80% of variance**; MAS = 15× chat tokens |
| Context files tax | 2602.11988 | AGENTS.md-style files **+>20% cost** |
| Token-efficient tool use (Anthropic beta) | claude.com/blog | **−14% avg / −70% max** output tokens |

## 7. Priority translation for evolve-loop (fleet-specific, beyond the existing token-optimization-2026 top-10)

1. **Quota-multiplying cache discipline under concurrency** (2.5, 2.6, 2.7): byte-identical static prefix across all lanes/phases; cache-breakers (run-id, cycle number, timestamps, per-session MCP tool lists) pushed to the tail; dispatch same-prefix sessions temporally adjacent within the 5-min TTL; never resume stale >TTL sessions — restart with compact brief. On API-key traffic this multiplies effective ITPM ~5×.
2. **Fleet admission control at selection time + centralized retry + budget-checkpointing** (4.1, 4.3): pre-flight estimated-cost gate per phase dispatch; per-lane budget with contracted degradation (checkpoint, hand back artifacts) — the published −96% cost / −48–100% dead-agent-waste numbers come from exactly the operator's failure mode (uncoordinated 10+ agents on rate-limited providers).
3. **Shared verified run-context (gist board) + who-knows-what registry** (1.1, 1.3): one Go-maintained board per run: verified facts at gist granularity, expandable on demand; lanes consult before exploring. DeLM's −50% cost with *better* accuracy is the strongest cross-agent number found.
4. **Phase-scoped evidence packs computed in Go** (5.3, 3.3): deterministic pre-phase grounding (diff, exports, failing tests, contract inputs only) instead of LLM re-exploration; scoped contexts show up to −82%.
5. **Audit the instruction payload itself** (5.4): >20% cost *increase* from repo context files means the ~24×/cycle CLAUDE.md injection deserves a role-scoped digest experiment.
