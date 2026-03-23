> Read this file when running Phase 4 (SHIP) or Phase 5 (LEARN). Contains research-backed techniques for memory management, instinct lifecycle, coordination, self-evaluation, and meta-learning.

## Contents
- [Strategy Playbook Protocol](#strategy-playbook-protocol) — incremental knowledge accumulation (ACE)
- [Instinct Forgetting Protocol](#instinct-forgetting-protocol) — strategic discard of zero-use instincts
- [Instinct Trust Governance](#instinct-trust-governance) — 3-tier trust for external instincts
- [Gene Self-Play Evolution](#gene-self-play-evolution) — adversarial gene refinement (Tool-R0)
- [Confidence-Correctness Alignment](#confidence-correctness-alignment) — detect miscalibration
- [Coefficient of Self-Improvement](#coefficient-of-self-improvement) — rolling improvement metric
- [Multi-Agent Coordination](#multi-agent-coordination) — DAG topology routing (AdaptOrch)
- [Adversarial Eval Co-Evolution](#adversarial-eval-co-evolution) — Mistake Book pattern (Code-A1)
- [EvoScore Instinct Decay](#evoscore-instinct-decay) — time-weighted confidence (SWE-CI)
- [Cost-Performance Feedback](#cost-performance-feedback) — token budget calibration (DAAO)
- [Personalization via Instincts](#personalization-via-instincts) — preference learning (PPP)

---

## Strategy Playbook Protocol

**Source:** ACE (arXiv:2510.04618)

**When:** Phase 5, before instinct extraction (step 4).

**Protocol (Generation-Reflection-Curation):**
1. **Reflect:** "What happened? What worked? What surprised me?" (2-3 sentences)
2. **Curate:** Decide what goes to playbook (2+ cycle patterns) vs instinct (single observation)
3. **Update playbook:** Append to relevant section, never rewrite wholesale

**Anti-collapse rules:**
- Never merge entries from different sections
- Cap consolidation ratio at 3:1
- Preserve specific file paths and error messages

---

## Instinct Forgetting Protocol

**Source:** arXiv:2505.00675, arXiv:2603.07670

**When:** Every 10 cycles during consolidation.

| Step | Action | Threshold |
|------|--------|-----------|
| Usage scan | Count citations in ledger (last 10 cycles) | — |
| Candidate selection | Instincts with 0 citations | `usageFrequency == 0` |
| Staleness check | Confidence < 0.4? | Auto-discard |
| Causal review | Learned from a still-relevant failure? | Retain if causal link active |
| Discard | Move to `archived/` | `archivedReason: "zero-use-discard"` |
| Merge | >85% similarity, different confidence | Keep higher confidence |

**Exempt:** Graduated instincts (confidence >= 0.75, cited 3+ cycles).

---

## Instinct Trust Governance

**Source:** Agent Skills (arXiv:2602.12430) — 26.1% community skills have vulnerabilities.

**When:** Before promoting instinct to global scope (`~/.evolve/instincts/personal/`).

| Tier | Origin | Restrictions |
|------|--------|-------------|
| Tier-1 | This project's cycles | None — first-party |
| Tier-2 | Verified project | Cannot modify eval files or agent prompts |
| Tier-3 | Unverified external | Read-only, no promotion, sandboxed |

**Gate:** Provenance check + no eval/prompt references + confidence >= 0.8 + 3 confirmations.

---

## Gene Self-Play Evolution

**Source:** Tool-R0 (arXiv:2602.21320)

**When:** A gene fires but validation fails.

1. Gene enters "adversarial refinement" — LEARN proposes targeted mutation
2. Mutated gene tested against same error pattern next cycle
3. 3+ successful mutations → promote to high-confidence capsule
4. 3 consecutive failures → archive with `archivedReason: "self-play-failure"`

---

## Confidence-Correctness Alignment

**Source:** arXiv:2603.06604

**When:** Phase 5, after self-evaluation scores recorded.

```
calibration_error = |mean_confidence - actual_accuracy| (k=5 cycle window)
```

| calibration_error | Action |
|-------------------|--------|
| > 0.15 | Force stepwise scoring, increase evidence items to 3, log miscalibration |
| < 0.10 for 2 cycles | Auto-disable recalibration |

---

## Coefficient of Self-Improvement

**When:** Phase 5, after fitness scores recorded.

```
CSI = (fitnessScore[N] - fitnessScore[N-k]) / k    (k=3)
```

| CSI | Action |
|-----|--------|
| > 0 | Continue current strategy |
| ≈ 0 | Increase task complexity or novelty |
| < 0 for 2+ windows | Strategy change or HALT |

---

## Multi-Agent Coordination

**Source:** AdaptOrch (arXiv:2602.16873)

**When:** Planning task execution order within a cycle.

Model tasks as DAG before execution:
- Nodes with in-degree 0 → parallel execution
- Linear chains → sequential
- Fan-out → hierarchical coordination

Parallel eval graders: run independently, reduce wall-clock time.

---

## Adversarial Eval Co-Evolution

**Source:** Code-A1 (arXiv:2603.15611)

**When:** Phase 5, reviewing eval grader effectiveness.

**Mistake Book pattern:** Retain eval graders that caught real bugs across cycles. Archive graders that only caught trivial issues. Eval graders should become harder as mastery level increases.

**Composite cycle reward:**
```
cycle_reward = ship_rate * 0.5 + eval_rigor * 0.3 + novelty * 0.2
```

---

## EvoScore Instinct Decay

**Source:** SWE-CI (March 2026)

**When:** Phase 5, updating instinct confidence.

Recent failures weight more than old successes:
```
adjusted_confidence = base_confidence * (1 - recent_failure_rate)
```

Track repeated Auditor WARNs on same files across 3+ cycles → technical debt signal.

---

## Cost-Performance Feedback

**Source:** DAAO (arXiv:2509.11079)

**When:** Phase 4, after task ships.

Record actual vs estimated token cost. Update `taskTypeDifficulty`:
- Delta > +20% consistently → increase avgDifficulty +0.5
- Delta < -20% consistently → decrease avgDifficulty -0.3

---

## Personalization via Instincts

**Source:** PPP (arXiv:2511.02208), arXiv:2602.22680

**When:** Phase 5, during instinct extraction.

Add preference dimension: instincts can capture user/project preferences (not just technical patterns).

**Rule:** Require 2+ cycle confirmations before promoting a preference instinct. Low-confidence preferences do not override high-confidence conventions.
