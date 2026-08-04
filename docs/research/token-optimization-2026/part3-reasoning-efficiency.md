# Part 3 — Reasoning-Token Efficiency & Token Budgeting (2025–2026 survey)

> Research agent sweep 3 of 3, 2026-07-05, for evolve-loop token-optimization goal 805f6ced.
> Scope: cutting thinking/output tokens per invocation, mapped to phase agents (scout, triage, tdd-engineer, builder, auditor).

## Thread 1 — Concise reasoning & thinking-budget control

### 1. Chain of Draft (arXiv 2502.18600, 2025)
Prompt-only: "think step by step, ≤5 words per step" + matching few-shot. **Matches/surpasses CoT accuracy at as little as 7.6% of tokens.** Caveat: brevity degrades on small models (<3B) and zero-shot ([HoT](https://arxiv.org/pdf/2503.02003), [revisit](https://arxiv.org/abs/2506.14641)).
**Fit:** scout/triage (disposable intermediate reasoning); NOT builder on hard multi-file edits.

### 2. TALE: Token-Budget-Aware Reasoning (arXiv 2412.18547, ACL Findings 2025)
Estimate per-problem budget (zero-shot TALE-EP or finetuned TALE-PT), embed in prompt ("solve within N tokens"). **~68% token cut, <5% accuracy loss.**
**Fit:** per-phase budget line in prompts — triage sets budgets for downstream phases.

### 3. Do NOT Think That Much (arXiv 2412.21187, 2025)
First systematic overthinking study; o1-like models burn extreme multiples on trivial problems. Self-training on shortest-correct traces cuts overhead preserving accuracy (GSM8K/MATH500/GPQA/AIME).
**Fit:** difficulty-gating — trivial triage decisions never get extended-thinking; auditor on trivially-clean diffs same.

### 4. L1/LCPO: Length Controlled Policy Optimization (arXiv 2503.04697, 2025)
RL rewarding correctness + adherence to prompt-specified length; smooth cost-accuracy dial. L1-trained **1.5B matches GPT-4o at equal budgets**.
**Fit:** evidence base for explicit per-phase length dial (builder high, triage low).

### 5. SelfBudgeter (arXiv 2505.11274, 2025)
Model predicts own budget, reasons within it. **74.47% length cut on MATH at near-equal accuracy.**
**Fit:** budget pre-pass pattern — phase agent's first output line declares budget; orchestrator enforces as max_tokens (scout/triage/auditor).

### 6. DART: Difficulty-Adaptive Reasoning Truncation (arXiv 2511.01170, Nov 2025)
Adaptive truncation by estimated difficulty vs fixed cap. **KEY FAILURE MODE: uniform truncation significantly harms hard tasks** (~4.1-pt AIME drops under uniform caps).
**Fit:** WARNS against single global max_tokens — builder needs higher ceiling; caps must be difficulty-conditioned.

### 7. SyncThink: Training-Free Early Exit (arXiv 2601.03649, Jan 2026)
Decoding monitor detects when answer tokens stop attending to reasoning (saturation) → terminate thinking. **~69% token cut (2,141→656 avg), ~69% latency cut, accuracy UP (61.22→62.00%); +8.1 pts GPQA** by preventing overthinking (R1-distill).
**Fit:** needs decoding control; API approximation = interleaved "answer now if confident" checkpoints — auditor + tdd verification loops.

## Thread 2 — Effort & model-tier routing

### 8. RouteLLM (LMSYS/Berkeley, ICLR 2025)
Learned binary router (Arena preference data): easy→weak model, hard→strong. **>85% cost cut MT-Bench, 45% MMLU, 35% GSM8K retaining 95% of GPT-4 perf; only 14% of calls needed strong model** (augmented training). 2026: RouteNLP conformal cascading ([2604.23577](https://arxiv.org/pdf/2604.23577)).
**Fit:** core justification for per-phase tier pinning + learned upgrade when triage flags hard cycle.

### 9. FrugalGPT cascades (arXiv 2305.05176, TMLR; still 2025-26 reference)
Cheap model first; learned reliability scorer accepts or escalates. **Matches GPT-4 at up to 98% cost cut (50–98% range), or +4% accuracy at equal cost.**
**Fit:** auditor cascade — cheap first audit pass, escalate to Opus-class only when uncertain/flagging.

### 10. Claude Opus 4.5 `effort` parameter (Anthropic, Nov 2025 — production)
Vendor effort dial (low/med/high) governing ALL output incl. thinking + tool calls. **Medium effort matches Sonnet 4.5's best SWE-bench Verified at 76% fewer output tokens.**
**Fit:** CHEAPEST LEVER TODAY for CLI agents: per-phase effort (scout/triage=low, tdd=med, builder=high, auditor=med+escalation) — no model swaps. ([anthropic](https://www.anthropic.com/news/claude-opus-4-5))

### 11. GPT-5 reasoning-effort cost curves (arXiv 2511.18649, zero-leakage 2026 KCSAT)
Max effort: 82.6→100 (+17.4 pts) but **~4× tokens** ("drastically reduced efficiency"); medium-effort/minimal-verbosity maximized efficiency. Practitioners: 4–5× inflation for 2–5 pt gains typical.
**Fit:** default every phase to medium-or-lower; reserve max effort for retry-after-failure (builder 2nd attempts).

## Thread 3 — Output-length control

### 12. Prompt-based length control is imprecise; inverted-U (ACoT arXiv 2509.14093, 2025)
"Cut words by 90%" yields only **~42.7% actual reduction** — directionally effective, uncalibrated. Accuracy = inverted-U in chain length; failed outputs correlate with LONGEST CoTs. Difficulty-weighted length penalties shorten easy, preserve hard ([2506.10446](https://arxiv.org/html/2506.10446v1), [ALP 2505.15612](https://arxiv.org/pdf/2505.15612)).
**Fit:** brevity instructions worth ~30–50% but MUST pair with hard max_tokens contracts per phase; never rely on instruction alone. All report-writing steps.

### 13. OptimalThinkingBench (arXiv 2508.13141, 2025)
Overthinking + Underthinking joint benchmark, 33 models: thinking models **waste hundreds of tokens on simplest queries, zero accuracy gain**; NONE optimal on both sides; efficiency methods improve one side at the other's expense.
**Fit:** argues per-phase (not global) tuning — aggressive diet on triage fine; same diet on builder harmful; measure each phase against its own eval.

## Thread 4 — Scaling test-time compute DOWN

### 14. DeepConf (Meta/UCSD, 2025)
Token-level confidence kills low-confidence traces early, stops sampling when ensemble settles. **Online: up to 84.7% token cut; aggressive: 43–79% cuts at equal/better accuracy; offline: 99.9% AIME 2025 (vs 97.0% majority voting) GPT-OSS-120B.**
**Fit:** wherever pipeline samples multiple candidates (adversarial audit, eval generation): confidence-gated early stop beats fixed-N. ([emergentmind](https://www.emergentmind.com/topics/deep-think-with-confidence-deepconf))

### 15. Certaindex/Dynasor — probe-based answer stabilization (arXiv 2412.20993, 2025)
Probe in-flight reasoning ("answer right now?"); successive agreement → exit. **Up to 50% compute savings, 3.3× throughput, no accuracy drop; Dynasor-CoT 11–29% token cut at identical accuracy.**
**Fit:** ORCHESTRATOR-IMPLEMENTABLE without model access — Go loop injects mid-phase "commit or continue" checkpoints; long scout/builder phases prone to stalls.

### 16. When More Thinking Hurts (arXiv 2604.10739, Apr 2026) + Mirage of TTS (2506.04210)
Extended reasoning causes models to ABANDON previously-correct answers; marginal returns collapse; **optimal budget difficulty-dependent: easy ~2K tokens, hard ~8K**. Mirage: apparent gains partly variance artifacts.
**Fit:** per-phase thinking ceilings — triage/scout ~2K-class, builder/tdd ~8K-class, difficulty (risk-level) as multiplier.

## Thread 5 — Agent-specific: retries, reflection, self-critique

### 17. How Do AI Agents Spend Your Money? (arXiv 2604.22750, Apr 2026) ⭐
8 frontier LLMs on SWE-bench Verified agentic coding. **Agentic ~1000× chat tokens; INPUT tokens dominate cost (context re-reads, tool results), not output; same-task variance 1×–30×; accuracy peaks at INTERMEDIATE token spend (more ≠ better); self-prediction r≤0.39, systematic underestimation.**
**Fit:** MOST RELEVANT PAPER — biggest lever is context/input discipline (what scout/builder re-read per turn), not thinking diets; per-cycle caps are quality-POSITIVE (spend beyond optimum correlates with worse outcomes). All phases; builder most.

### 18. Reflection/self-critique economics (2025–2026 synthesis)
Marginal quality gain rarely justifies cost after 1–2 reflection passes; reflection HURTS when initial response already accurate (perturbs correct answers). Prospective reflection (PreFlect): 10–15% utility for 15–20% overhead. ReflexiCoder (RL-internalized): 94.51% HumanEval, ~40% token-efficiency gain. Separate-critic (different model) breaks shared blind spots at one extra call — matches adversarial-audit design. ([zylos](https://zylos.ai/research/2026-05-12-agent-self-correction-reflexion-to-prm))
**Fit:** cap reflection at ONE round per phase; auditor = the ONLY critic (cross-model, adversarial); 2nd round only on explicit failure signals (red tests, gate FAIL).

### 19. Token Economics survey (2605.09104) + Reasoning on a Budget (2507.02076)
Budget-conditioned prompting + difficulty-adaptive allocation + early exit = three composable families, EACH independently worth 30–70% savings.
**Fit:** treat per-phase quotas as allocation problem (marginal accuracy per token differs by phase), not uniform caps.

## Top 5 actionable (per-invocation)

1. **Per-phase effort/tier routing** — vendor effort params per phase (triage/scout=low, tdd/audit=medium, builder=high-on-retry-only). **76% fewer output tokens at equal SWE-bench** (Opus 4.5 medium); max effort ~4× for single-digit gains. Highest savings-to-risk; zero prompt changes.
2. **Budget-conditioned prompts (TALE/SelfBudgeter)** — triage estimates budget per task, "solve within N tokens" downstream, enforced max_tokens. **68–74% cuts <5% loss.** MUST be difficulty-conditioned (DART).
3. **Chain-of-Draft concise reasoning for report-producing phases** — ≤5-word-step drafts. **Up to 92% cut at parity**; pair with hard caps (bare instructions under-deliver ~2×).
4. **Confidence/stabilization-gated early exit** — stop when answer stabilizes. **43–84.7% cuts equal/better accuracy (DeepConf); 11–50% production (Dynasor); ~69% with accuracy UP (SyncThink)**; overthinking flips correct→wrong.
5. **One-round reflection cap + input-token discipline** — single cross-model adversarial pass on failure signals only; attack input-side waste (re-reads dominate). Reflection value collapses after 1–2 passes; caps quality-positive.

**Cross-cutting caution:** every hard-cap technique shows the same failure signature — uniform limits safe on easy, harmful on hard (~4-pt drops). Budgets must scale with task difficulty (risk-level from intent/triage): ~2K thinking class easy phases, 8K+ hard builder cycles.
