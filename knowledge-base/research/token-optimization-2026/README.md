# Token Optimization for the Evolve-Loop Pipeline — 2025/2026 Research Survey & Applicability Map

> **Status:** research complete 2026-07-05 (3 parallel web sweeps, ~50 papers/sources, all 2025–2026 unless seminal).
> **Goal served:** `805f6ced` — "Optimize per-agent token usage across all phase agents … and the system; preserve every phase-integrity guarantee."
> **Companion files:** [part1-context-compression.md](part1-context-compression.md) · [part2-multiagent-economics.md](part2-multiagent-economics.md) · [part3-reasoning-efficiency.md](part3-reasoning-efficiency.md) — full per-paper detail + citations.

## Executive summary

Three independent literature sweeps converge on one hierarchy of leverage for a multi-phase agent pipeline:

1. **Input tokens dominate cost, not output.** Agentic workloads run 2:1–100:1 input:output; context re-reads and tool results are the spend (arXiv 2604.22750, AgentTaxo ICML 2025, Manus). The biggest levers are therefore *context discipline* and *cache economics*, not "think less."
2. **Deterministic beats learned.** The best 2025 empirical result for coding agents: simple observation masking (evict tool outputs older than ~10 turns) beats LLM summarization on BOTH cost (−52.7%) and solve rate (JetBrains "Complexity Trap", arXiv 2508.21433). Anthropic's productized stale-tool-result eviction measured **−84% tokens** on 100-turn workflows.
3. **Caps are quality-POSITIVE.** Accuracy peaks at intermediate token spend; overthinking flips correct answers to wrong (arXiv 2604.10739, 2604.22750). Token budgets are not a cost/quality tradeoff — past the optimum they improve both.
4. **The one hard failure mode: uniform caps.** Fixed truncation harms hard tasks (~4-pt AIME drops). Every budget must be difficulty-conditioned (DART 2511.01170): ~2K-token thinking class for easy phases, 8K+ for hard builder cycles.
5. **Report size is the whole game for phase handoffs.** A subagent doing 10K tokens of work returning 500 saves 95%; returning an 8K "detailed report" saves nothing (Anthropic sub-agent guidance: 1–2K-token return budgets).

## Unified top-10 (ranked by evidence × implementability in evolve-loop)

| # | Technique | Evidence | Where in evolve-loop |
|---|---|---|---|
| 1 | **Observation masking / stale tool-result eviction** (rolling ~10-turn window) | −52.7% cost at BETTER solve rate (2508.21433); −84% (Anthropic) | bridge tmux/headless drivers (pure Go, deterministic) |
| 2 | **Cache-aligned stable prompt prefixes** ({byte-stable static}+{dynamic tail}, no timestamps/cycle-numbers early) | 0.1× cached input (Anthropic 1h-TTL), 50% (OpenAI); input=54–75% of spend; Manus: #1 production metric | recipe/prompt assembly in bridge; both fleet lanes share prefix → lane-2 prefill at 0.1× |
| 3 | **Hard report-size contracts** (≤1–2K tokens, bounded lists, schema-bound handoff) | 14× measured subagent compression; returns >8K save nothing | contract-gate on scout-report/build-report/audit-report; Go-validated before next phase launches |
| 4 | **Per-phase effort/tier routing** (low for scout/triage, high only on builder retry) | Opus 4.5 medium = Sonnet-best SWE-bench at −76% output tokens; max effort ~4× for single-digit gains | advisor overlay + profiles (tier system exists; add effort dial) |
| 5 | **Artifact-dependency-graph pruning** (each phase gets only its true edges) | AgentPrune −28–73% tokens at equal accuracy; audit=59.4% of agentic-SWE tokens (Tokenomics 2601.14470) | phase contracts: audit gets diff+build-report NOT scout backlog; ship gets verdicts only |
| 6 | **Budget-conditioned prompts, difficulty-scaled** (TALE/SelfBudgeter; triage estimates downstream budgets) | −68–74% tokens <5% loss; MUST be difficulty-conditioned (DART) | triage emits per-task token-budget hint; orchestrator enforces per-phase max |
| 7 | **Per-phase token stop-losses + attribution** | 30× same-task variance; self-prediction r≤0.39; accuracy peaks at intermediate spend | orchestrator kill/retry on runaway phase; per-phase/lane usage attribution (plumbing exists) |
| 8 | **Structured eviction over blind summarization in artifact schemas** (never-evict: decisions/acceptance/open-questions; evictable: narrative) | summaries destroy actionable detail + elongate trajectories 13–15% | artifact schema redesign (scout-report sections) |
| 9 | **One-round reflection cap; auditor is the only critic** | reflection value collapses after 1–2 passes; hurts already-correct outputs | adversarial-audit already cross-model; cap builder self-critique + audit iterations |
| 10 | **Sleep-time consolidation between cycles** (memo/retro pre-digest → warm-start brief; novelty-gate KB writes) | 5× test-time compute cut, +13–18% accuracy (Letta); Mem0/A-MEM 85–93% per-op savings; SAGE novelty gate | memo/retro phases; knowledge-base consolidation pass; scout injects top-k linked notes not whole index |

**Track, don't build:** soft-prompt/KV compression (500xCompressor, gist tokens) — needs model-weight access, inapplicable to closed CLIs; latent inter-agent channels; LLMLingua-2 only for bulky log/diff sections at conservative 2–4×.

## Applicability map — evolve-loop specifics

### A. Bridge/drivers (`go/internal/bridge/`) — deterministic, highest yield
- **A1. Observation masking in tmux/headless sessions:** mask tool outputs older than ~10 turns within a phase session (placeholder text, keep reasoning/action chain). Pure Go. Expected: ~50% phase-session cost cut (finding #1).
- **A2. Cache-stable prompt prefix audit:** find every cycle-varying string (cycle number, timestamps, run-ids, randomized ordering) injected EARLY in phase prompts; move to prompt tail. Deterministic serialization everywhere. Expected: cached-input pricing on the stable preamble across 6+ phases × 2 lanes (finding #2).
- **A3. Tool-result clearing post-verdict:** after a verdict/summary line is extracted from test/build output, drop the raw output from context (keep pass/fail + counts) (TokenPilot, Anthropic context editing).

### B. Artifact contracts (`contract gate`, phase report schemas) — the handoff layer
- **B1. Report-size contracts:** hard caps (~1–2K tokens) on scout-report/build-report/audit-report handoff SUMMARIES; full detail stays on disk, downstream phases read sections just-in-time by path (finding #3 + JIT retrieval).
- **B2. Dependency-graph pruning:** encode per-phase artifact needs (audit: diff+build-report; ship: verdicts; tdd: acceptance criteria not exploration narrative) — stop accumulating all prior artifacts into later phases (finding #5).
- **B3. Structured-eviction schema:** split artifacts into never-evict (decisions, acceptance criteria, open questions, dependency facts) and evictable (exploration narrative) sections; evict rather than summarize (finding #8).

### C. Advisor/routing (`internal/advisor`, profiles, model catalog) — effort economics
- **C1. Per-phase effort dial:** extend tier routing with vendor effort params (claude `effort`, GPT-5 `reasoning_effort`): scout/triage=low, tdd/audit=medium, builder=medium→high-on-retry. Expected: up to 76% output-token cut on over-provisioned phases (finding #4).
- **C2. Difficulty-conditioned budgets:** triage's risk/complexity classification scales downstream thinking budgets (~2K easy / 8K+ hard) — NEVER uniform caps (finding #6 + DART caution).
- **C3. Auditor cascade:** cheap-tier first audit pass; escalate to deep-tier only on uncertainty/flags (FrugalGPT: up to 98% cost cut pattern; audit is 59% of pipeline tokens).

### D. Orchestrator (`internal/core`) — enforcement
- **D1. Per-phase token stop-losses:** deterministic kill/retry when a phase exceeds its difficulty-conditioned budget (30× variance finding); per-phase/lane attribution logging (usage plumbing exists — surface it).
- **D2. Reflection caps:** bound audit-loop iterations (adversarial-review→audit re-entry) to 1 unless explicit failure signals.

### E. Memory/knowledge-base (memo, retro, scout injection)
- **E1. Sleep-time warm-start brief:** memo/retro consolidate cycle history into a compact brief; next scout starts warm instead of re-deriving (5× pattern).
- **E2. Novelty-gated KB writes + top-k injection:** memo skips near-duplicate observations; scout injects the 3 most-relevant prior-cycle notes, not the whole index (A-MEM: 1–2.5K vs 16–17K tokens).

### F. Fleet (2-lane) economics
- **F1. Shared static prefixes across lanes** → lane-2 prefill at cached price (finding #2).
- **F2. Disjointness is the economic gate:** lanes pay only when work is truly disjoint (equal-budget study 2604.02460 + Cognition) — reinforces the triage-disjointness work (0.94/0.92 inbox items) as cost-critical, not just correctness.

## Measurement first (baseline before optimizing)
Per AgentTaxo/Tokenomics: compute per-phase **input:output ratios** and per-phase share of cycle tokens from existing `phase-timing.json`/usage artifacts. Hypotheses to verify on evolve-loop's own telemetry: (a) audit/adversarial phases ≈ half of cycle tokens; (b) input dominates ≥2:1 per phase; (c) any phase >3:1 is re-ingesting artifacts it doesn't need. The winner techniques (A1/A2/B1) should each show up as measurable deltas on this baseline within a few cycles of landing.

## Sources
~50 primary sources across the three companion files; headline citations: arXiv 2508.21433 (Complexity Trap), 2604.22750 (Agent spend), 2605.09104 (Token Economics survey), 2412.18547 (TALE), 2502.18600 (CoD), 2601.06007 (Don't Break the Cache), 2410.02506 (AgentPrune), 2601.14470 (Tokenomics), 2510.11967 (Context-Folding), 2606.11213 (Structured Eviction), 2504.13171 (Sleep-time), Anthropic effective-context-engineering + context-management + Opus 4.5 effort, Manus context-engineering lessons, JetBrains efficient-context-management.
