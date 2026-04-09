> Read this file when running Phase 2 (DISCOVER). Contains research-backed techniques for task selection, research quality, goal tracking, and difficulty estimation.

## Contents
- [Failure Pattern Reading](#failure-pattern-reading) — avoid repeating failures (DGM)
- [Difficulty-Aware Task Scoring](#difficulty-aware-task-scoring) — continuous 1-10 scoring (DAAO)
- [Goal-Continuity Milestones](#goal-continuity-milestones) — multi-cycle goal tracking (GoalAct)
- [Research Quality Scoring](#research-quality-scoring) — per-query novelty/relevance/yield (HiPRAG)
- [Pre-Execution Simulation](#pre-execution-simulation) — simulate before committing (WebDreamer)
- [Bandit + Novelty Boost](#bandit--novelty-boost) — task type selection (DGM novelty)
- [Proactive Task Deferral](#proactive-task-deferral) — strategic clarification (PPP)

---

## Failure Pattern Reading

**Source:** DGM (arXiv:2505.22954)

**When:** Before finalizing task list, read `state.json.failedApproaches`.

| Root Cause | Scout Action |
|-----------|-------------|
| `implementation-error` | Retry same approach with explicit constraints |
| `scope-mismatch` | Re-scope at correct complexity |
| `eval-gap` | Fix eval definition, re-run same approach |
| `approach-flaw` | Use `alternativeApproach` field, try different strategy |

**Rule:** If last 2 failures on same file share a root cause → avoid that file unless `noveltyScore > 0.7`.

---

## Difficulty-Aware Task Scoring

**Source:** DAAO (arXiv:2509.11079)

**When:** Assign `difficultyScore` to each proposed task.

| Score | Difficulty | Model Tier | Token Budget |
|-------|-----------|------------|-------------|
| 1-3 | Simple | tier-3 | 20-30K |
| 4-6 | Moderate | tier-2 | 30-60K |
| 7-9 | Complex | tier-1 | 60-100K |
| 10 | Extreme | tier-1 + thinking | 100K+ |

**Rule:** Check `state.json.taskTypeDifficulty` for per-type success rates. If `successRateByBand["7-9"] < 0.5` → split task or upgrade model tier.

---

## Goal-Continuity Milestones

**Source:** GoalAct (arXiv:2504.16563)

**When:** A multi-cycle goal is active (passed via `/evolve-loop N goal`).

**Protocol:**
1. Cycle 1: Decompose goal into 3-5 milestones in `state.json.goalMilestones`
2. Each cycle: Proposed tasks must advance a pending milestone
3. If no task advances a milestone → replan (branch trap detection)
4. Mark milestones `completed` with cycle number

---

## Research Quality Scoring

**Source:** HiPRAG (arXiv:2510.07794)

**When:** After each web search query.

| Dimension | Score 0.0-1.0 | Criteria |
|-----------|--------------|---------|
| Novelty | Not in existing `.evolve/research/` capsules | High if new domain |
| Relevance | Directly applicable to current goal | High if maps to evolve-loop |
| Yield | Contains actionable task-translatable findings | High if concrete technique |

**Rules:**
- Composite < 0.3 → skip knowledge capsule
- Composite > 0.7 → +1 priority boost to derived tasks
- Record in `stateJson.research.queries` for Operator tracking

---

## Pre-Execution Simulation

**Source:** WebDreamer (arXiv:2411.06559)

**When:** Before finalizing task selection for M+ complexity tasks.

**Simulate:** "If I assign this task..."
1. Which files will the Builder need to read? (scope prediction)
2. How many lines will change? (token budget estimation)
3. What eval graders will catch real issues? (eval quality prediction)

**Cost-benefit:** ~2-5K tokens per simulation. Break-even: 1 prevented failure per 5 simulations.

---

## Bandit + Novelty Boost

**Source:** DGM (arXiv:2505.22954), Thompson Sampling

**When:** Final task ranking before selection.

| Condition | Boost |
|-----------|-------|
| Task type `avgReward >= 0.8` and `pulls >= 3` | +1 priority |
| Target files `lastTouchedCycle <= currentCycle - 3` | +1 novelty |
| Benchmark weakness dimension match | +2 priority |
| Approach differs from last 3 cycles | +1 diversity (DGM) |

---

## Proactive Task Deferral

**Source:** PPP (arXiv:2511.02208)

**When:** Goal is vague or task scope is uncertain.

**Rule:** Propose a narrower interpretation rather than attempting broad coverage. Strategic deferral with rationale is better than shipping a misaligned task. Record `deferralReason` for future cycle retrieval.
