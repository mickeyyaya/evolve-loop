# Self-Learning Architecture

The evolve-loop improves autonomously across cycles. Each phase produces signal — build outcomes, audit verdicts, eval scores — that feeds back into future cycles as instincts, bandit arm updates, and plan templates. The loop does not just execute tasks; it learns from them.

## Overview

Self-learning in the evolve-loop is not a single mechanism but a layered system of feedback loops. Seven interconnected mechanisms collect signal, extract patterns, bias selection, and refine execution — all without human intervention. The architecture is designed so that every failure gradient becomes a policy update by the next cycle.

---

## Self-Improvement Mechanisms

### a. Instinct Extraction (Phase 5 LEARN)

After each cycle, the orchestrator analyzes build reports, audit verdicts, and eval outcomes to extract **instincts** — specific, actionable patterns. Each instinct captures a single observation with a confidence score and a memory category (`episodic`, `semantic`, `procedural`).

Extraction is mandatory when any LLM-as-a-Judge dimension scores below 0.7. This forcing function prevents stalls during uniform-success periods when the gradient is weak.

Instincts are stored as YAML files under `.evolve/instincts/personal/` and injected into the Scout and Builder context as a compact `instinctSummary` array — avoiding full-file reads on each cycle. Each instinct is tagged with a functional memory category (strategic, episodic, semantic, procedural, tool-use, metacognitive) — see [Memory Hierarchy: Functional Memory Categories](memory-hierarchy.md#functional-memory-categories).

### b. LLM-as-a-Judge Self-Evaluation (Phase 5 LEARN)

After instinct extraction, the orchestrator scores the cycle on four dimensions using a structured rubric. Each score requires a chain-of-thought justification before a numeric value is assigned.

| Dimension | Guiding Question | Pass Threshold |
|-----------|-----------------|----------------|
| Correctness | Did the build produce the intended behavior? Did evals pass? | ≥0.7 |
| Completeness | Were all acceptance criteria addressed? No missing edge cases? | ≥0.7 |
| Novelty | Did the cycle surface new patterns, techniques, or knowledge? | ≥0.7 |
| Efficiency | Were tokens, attempts, and file changes minimized? Was scope right-sized? | ≥0.7 |

Any dimension scoring below 0.7 triggers mandatory instinct extraction for that failure before the cycle closes. Scores are recorded in `workspace/build-report.md` under `## Self-Evaluation`.

### Stepwise Confidence Scoring

Before assigning a holistic score per dimension, decompose the evaluation into 2-3 discrete evidence items and score each independently. The final dimension score is derived from the per-step scores rather than a single intuitive judgment.

**Protocol:**

1. For each dimension, enumerate 2-3 concrete evidence items (e.g., "eval graders all passed", "one acceptance criterion was partially met", "no new instincts surfaced").
2. Assign a mini-score (0.0-1.0) to each evidence item based on observable outcomes.
3. Derive the final dimension score as the mean of the mini-scores, rounded to one decimal.

**Why stepwise scoring matters:**

Per-step evidence decomposition improves calibration by reducing anchoring bias — the tendency to assign a score based on overall impression rather than specific evidence. Research (arxiv 2511.07364, 2025) demonstrates +15% AUC-ROC improvement in multi-step failure detection when evaluators score individual evidence items before deriving an aggregate score. This is especially effective for the Completeness and Correctness dimensions, where partial success can mask specific gaps.

The stepwise approach is referenced in `skills/evolve-loop/phase5-learn.md` (Self-Evaluation section) and complements the chain-of-thought justification already required per dimension.

### c. Multi-Armed Bandit Task Selection

The Scout maintains a `taskArms` table in `state.json` with per-type reward history across five task types: `feature`, `stability`, `security`, `techdebt`, `performance`. Each arm tracks pull count and cumulative reward.

After a task ships successfully, its arm is updated: `pulls + 1`, `totalReward + 1`. Before finalizing the task list each cycle, the Scout applies **Thompson Sampling**-style weighting: arms with `avgReward >= 0.8` and `pulls >= 3` receive a +1 priority boost. Arms with fewer pulls remain eligible for exploration, preventing over-exploitation.

The bandit reward signal flows back from Phase 5 into Phase 1 of the next cycle — the loop learns which task types it executes well and shifts investment toward them.

### d. Plan Template Caching

When a task is structurally similar to one solved in a previous cycle, the Builder reuses the cached plan template rather than re-planning from scratch. Templates are matched by task type and structural similarity to prior tasks.

Reusing a plan template typically saves 30–50% of build tokens for that task. The cache is populated from plans that shipped successfully and passed audit without retries. Plan templates compose well with instincts: a cached plan combined with relevant instincts enables near-zero ramp-up time for familiar task shapes.

### e. Memory Consolidation (every 3 cycles)

When `instinctCount > 20` or every 3 cycles, the orchestrator consolidates the instinct set:

1. **Cluster** — Instincts with >85% semantic similarity are merged into higher-level abstractions. Example: two camelCase instincts merge into one covering all JSON keys.
2. **Archive originals** — Superseded instincts move to `.evolve/instincts/archived/` with a `supersededBy` field. Nothing is deleted.
3. **Temporal decay** — Instincts unreferenced in the last 5 cycles lose 0.1 confidence per consolidation pass. Below 0.3 they are archived as stale.
4. **Entropy gating** — A new instinct that is >90% similar to an existing one updates the existing one's confidence instead of creating a duplicate.

Consolidation keeps the active instinct set compact, relevant, and non-redundant. The episodic→semantic abstraction pathway operates here: repeated episodic observations (what happened in cycle N) consolidate into semantic knowledge (domain conventions, architecture facts) that applies across all future cycles.

### f. Instinct Promotion (project → global)

High-confidence instincts that are not project-specific can be promoted to global scope at `~/.evolve/instincts/personal/`. Promotion criteria:

- Confidence >= 0.8
- Confirmed across 2+ cycles
- Generalizable beyond this codebase

The promoted copy gets a `promotedFrom` field recording the project and cycle of origin. The original remains in the project instincts directory as the source of truth. Promoted instincts are available to all evolve-loop instances running on any project.

### g. Meta-Cycle Review (every 5 cycles)

When `cycle % 5 === 0`, the orchestrator runs a split-role critique during Phase 5, after instinct extraction:

| Critic | Focus |
|--------|-------|
| Efficiency Critic | Token usage, task sizing, model routing |
| Correctness Critic | Eval pass rates, audit verdicts, regression trends |
| Novelty Critic | Instinct diversity, task variety, stagnation |

The synthesis prioritizes correctness > efficiency > novelty. Output includes agent effectiveness scores, mutation test results (eval kill rate target: >80%), and up to 2 automated prompt edits via a TextGrad-style critique-synthesize loop. Prompt edits auto-revert if the next meta-cycle shows degradation.

### h. Coefficient of Self-Improvement (CSI)

The Coefficient of Self-Improvement quantifies whether the loop is getting better, plateauing, or regressing over recent cycles. It is computed during Phase 5 LEARN by the Operator after self-evaluation scores are recorded.

**Formula:**

```
CSI = (fitnessScore[N] - fitnessScore[N-k]) / k
```

Where `fitnessScore` is the mean of the four LLM-as-a-Judge dimension scores for a cycle, `N` is the current cycle, and `k = 3` (rolling window of three cycles). CSI is undefined until cycle 4 (the first window with enough history).

**Interpretation:**

| CSI Value | Meaning | Action |
|-----------|---------|--------|
| CSI > 0 | Loop is improving | Continue current strategy |
| CSI ≈ 0 | Plateau reached | Consider increasing task complexity or novelty |
| CSI < 0 | Regression detected | Investigate recent instincts and prompt edits |

**Regression safeguard:** When CSI < 0 for 2 or more consecutive rolling windows, the Operator triggers a strategy change (e.g., switching from `aggressive` to `balanced`) or initiates a HALT for human review. This prevents the loop from compounding regressions across multiple cycles.

**Design rationale:** CSI maps to the Karpathy autoresearch / GVU pattern — a tight edit → run → measure cycle where each iteration's delta is tracked quantitatively. By reducing multi-dimensional eval scores to a single directional derivative, CSI gives the Operator a clear signal for meta-level decisions without requiring full score decomposition each cycle.

CSI values are recorded in `workspace/build-report.md` alongside the self-evaluation scores and are available to the Scout for task selection in subsequent cycles.

### i. Confidence-Correctness Alignment (Phase 5 LEARN)

The LLM-as-a-Judge self-evaluation assigns confidence via stepwise scoring, but stated confidence can drift from actual correctness over time. Confidence-correctness alignment detects and corrects this miscalibration.

**Concept:** Track the relationship between the Judge's stated confidence (dimension scores) and the actual correctness rate (determined by downstream eval pass/fail outcomes). When these diverge, the Judge is either overconfident or underconfident — both degrade learning signal quality. Reference: "Know When You're Wrong" (arxiv 2603.06604).

**Calibration error formula:**

```
calibration_error = |mean_confidence - actual_accuracy|   (rolling window, k=5 cycles)
```

Where `mean_confidence` is the average self-evaluation score across dimensions and `actual_accuracy` is the fraction of eval graders that passed in the same window.

**Recalibration trigger:** When `calibration_error > 0.15`, the Operator forces recalibration for the next cycle:

1. **Force stepwise scoring** — Require per-evidence mini-scores (see § Stepwise Confidence Scoring) even if the dimension would otherwise score above 0.7.
2. **Increase evidence requirements** — Raise the minimum evidence items per dimension from 2 to 3.
3. **Log the miscalibration** — Record `calibrationError` and `recalibrationTriggered: true` in `workspace/build-report.md` so the Scout and future cycles can observe the correction.

Recalibration auto-disables when `calibration_error` drops below 0.10 for two consecutive cycles. This mechanism maps directly to the self-evaluation protocol in Phase 5 LEARN and complements CSI by addressing score quality rather than score trajectory.

### j. Self-Evolving Agent Taxonomy

Reference: "Survey of Self-Evolving Agents" (arxiv 2507.21046). This survey formalizes a four-stage evolution lifecycle that applies to any agent system capable of autonomous improvement.

**Four-stage evolution lifecycle:**

| Stage | Description | Evolve-Loop Phase |
|-------|-------------|-------------------|
| Perceive | Observe environment, collect feedback signals | Scout (scans codebase, reads instincts, gathers evals) |
| Learn | Extract patterns and update internal knowledge | Builder (extracts plan from Scout brief, applies instincts) |
| Self-Modify | Apply learned patterns to change own behavior | Builder (implements changes, mutates prompts via meta-cycle) |
| Verify | Validate modifications against quality criteria | Auditor (eval graders, LLM-as-a-Judge, audit verdicts) |

**Taxonomy dimensions — what evolves and how:**

- **What evolves:** Parameters (model weights), prompts (system instructions, templates), architecture (tool selection, agent topology). The evolve-loop operates at the *prompt level* — instincts, plan templates, and meta-cycle prompt edits are the primary mutation targets.
- **How it evolves:** Self-play (agent critiques own output), environment feedback (eval graders, build pass/fail), reflection (Phase 5 self-evaluation, CSI tracking). The evolve-loop uses *reflection-based feedback* as its primary learning signal, augmented by environment feedback from deterministic eval graders.

**Position in the taxonomy:** The evolve-loop is a prompt-level self-evolving system with reflection-based feedback. It does not modify model weights or agent architecture at runtime. Evolution is bounded by the meta-cycle review (every 5 cycles) which applies up to 2 prompt edits per pass, with automatic rollback on regression — placing it in the "constrained self-modification" category of the taxonomy.

---

## Instinct Lifecycle

```
Extract → Score → Cite → Consolidate → Promote → Archive
```

1. **Extraction** — Phase 5 extracts instincts from cycle artifacts. Forced if any self-evaluation dimension scores <0.7.
2. **Confidence scoring** — New instincts start at 0.5–0.6. Confirmed in a later cycle: +0.1. Contradicted: -0.1.
3. **Citation tracking** — Agents log which instinct IDs influenced their decisions (`instinctsApplied`). Uncited instincts for 3+ cycles are flagged as dormant by Scout introspection.
4. **Consolidation** — Every 3 cycles: cluster similar instincts, archive superseded ones, apply temporal decay, enforce entropy gating.
5. **Promotion** — Instincts reaching confidence >= 0.8 and confirmed across 2+ cycles are promoted to global scope.
6. **Archival** — Stale instincts (confidence < 0.3) or superseded instincts move to `archived/`. Provenance is always preserved.

---

## Feedback Loop Architecture

Each mechanism feeds the next:

```
Cycle outcome
  → LLM-as-a-Judge scores (self-evaluation)
      → Instinct extraction (forced on <0.7 dimensions)
          → Confidence updates (citation tracking)
              → Memory consolidation (cluster/decay/gate)
                  → Instinct promotion (global scope)
                      → Scout instinct reading (next cycle)

Cycle outcome
  → Bandit arm update (reward + 1 on success)
      → Thompson Sampling boost (next cycle Scout)
          → Task selection bias (high-reward types prioritized)

Plan succeeded + no audit retries
  → Plan template cached
      → Builder reuses template (next similar task)

Cycle N % 5 == 0
  → Meta-cycle review
      → Prompt evolution (up to 2 edits)
          → Agent behavior refined (subsequent cycles)

fitnessScore rolling window (k=3)
  → CSI computed (Phase 5 LEARN)
      → CSI > 0: continue | CSI ≈ 0: nudge | CSI < 0: strategy change or HALT
```

Every feedback path closes within 1–5 cycles. No signal is discarded — failed attempts, dormant instincts, and weak evals each have a downstream effect on the pipeline.

---

## Self-Learning Anti-Patterns

### Extraction Stall

When the ship rate is uniformly high (100% success), the failure gradient is flat and instinct extraction produces no output. The loop stops learning from its own success.

**Mitigation:** LLM-as-a-Judge forcing function. If any dimension scores <0.7, extraction is mandatory even during success streaks. Scout introspection detects `instinctsExtracted == 0` for consecutive cycles and raises it as a capability gap signal.

### Instinct Drift

Instincts extracted from an earlier codebase state may become incorrect as the codebase evolves. Applying a stale procedural instinct to a changed module produces incorrect implementations.

**Mitigation:** Temporal decay (confidence -0.1 per consolidation pass for unreferenced instincts), citation tracking (dormant instincts flagged by Scout), and archival below confidence 0.3. Instincts are not permanent facts — they decay without reinforcement.

### Over-Consolidation

Aggressive clustering can merge genuinely distinct patterns into a single abstraction that is too coarse to be actionable. A merged instinct that conflates two different conventions provides weaker guidance than either original.

**Mitigation:** Consolidation requires >85% semantic similarity (not just surface similarity). Archived originals are never deleted — they can be recovered if the merged form proves too broad. Entropy gating applies to new instincts, not to consolidation merges.

For techniques that complement self-learning by improving the accuracy of outputs being evaluated (chain-of-thought prompting, multi-stage verification, context alignment scoring, uncertainty acknowledgment), see `docs/accuracy-self-correction.md`.
