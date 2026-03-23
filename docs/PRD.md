# Evolve Loop — Product Requirements Document (PRD)

**Version:** 8.0.0 | **Last Updated:** 2026-03-23 | **Status:** Production (v8.0.0, 151 cycles completed)

---

## 1. Product Overview

Evolve Loop is a **self-evolving development pipeline** that runs as a plugin for AI CLIs (Claude Code, Gemini CLI). It orchestrates 4 specialized AI agents across 5 lean phases to autonomously discover, build, audit, and ship improvements to any codebase — then learns from each cycle to improve itself.

### Problem Statement

AI coding assistants execute tasks well but lack a structured loop for continuous, autonomous improvement. Developers must manually identify work, guide implementation, review output, and track learning. This creates a bottleneck: the human is the loop.

### Solution

A self-contained pipeline that:
1. **Discovers** what to build next (Scout agent)
2. **Builds** it in isolation (Builder agent, git worktree)
3. **Audits** it with hard quality gates (Auditor agent)
4. **Ships** it automatically (orchestrator, git push)
5. **Learns** from the outcome for next time (orchestrator + Operator agent)

### Target Users

- Developers who want autonomous code improvement on their repos
- Teams running overnight quality improvement sessions
- Projects with well-defined eval criteria that can be automated

---

## 2. Current Project Status

### Key Metrics (as of Cycle 151)

| Metric | Value |
|--------|-------|
| Total cycles completed | 151 |
| Total tasks shipped | 67+ |
| Total tasks failed | 0 |
| Success rate | 100% |
| Avg tasks per cycle | 2.5 |
| Mastery level | Proficient (32 consecutive successes) |
| Benchmark score | 94.4/100 |
| Commits | 285 |
| Plugin version | v8.0.0 |

### Benchmark Trajectory

```
Score
100 ┤
 95 ┤                                                    ●━━━━━━━━━━━━━━━━━●
 94 ┤                                               ┌────┘
 91 ┤                                     ┌──────────┘
 90 ┤                               ●─────┘
 88 ┤                         ●─────┘
 86 ┤                   ●─────┘
 84 ┤             ●─────┘
 83 ┤       ●─────┘
 80 ┤───────┘
    └──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──
       1  3  5  7  9  11 13 16 18 20 24    139  151  Cycle
```

| Calibration Point | Cycle | Overall Score |
|-------------------|-------|---------------|
| Initial baseline | 1 | 83.8 |
| Domain generalization start | 3 | 86.0 |
| Research integration | 8 | 88.1 |
| Post-research recalibration | 16 | 87.4* |
| Estimated mid-run | 24 | 91.0 |
| Latest calibration | 139 | 94.4 |

*\*Cycle 16 recalibration applied stricter LLM rubrics, explaining the apparent dip. Automated scores continued rising.*

### Benchmark Dimensions (Last Calibration: Cycle 139)

| Dimension | Automated | LLM | Composite |
|-----------|-----------|-----|-----------|
| Defensive Design | 100 | 100 | **100** |
| Convention Adherence | 100 | 100 | **100** |
| Eval Infrastructure | 100 | 100 | **100** |
| Feature Coverage | 100 | 100 | **100** |
| Specification Consistency | 93 | 100 | **95** |
| Modularity | 100 | 75 | **93** |
| Schema Hygiene | 100 | 75 | **93** |
| Documentation Completeness | 82 | 75 | **80** |

---

## 3. Architecture & Flow

### High-Level Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                        ORCHESTRATOR                             │
│   (Main AI CLI session — coordinates all phases)                │
│                                                                 │
│   ┌──────────┐   ┌──────────┐   ┌──────────┐                  │
│   │  Phase 0  │   │  Phase 4  │   │  Phase 5  │                │
│   │ CALIBRATE │   │   SHIP    │   │   LEARN   │                │
│   │(benchmark)│   │(git push) │   │(instincts)│                │
│   └──────────┘   └──────────┘   └──────────┘                  │
│                                                                 │
│   ┌──────────────────────────────────────────────────────────┐ │
│   │                    PER-CYCLE LOOP                         │ │
│   │                                                          │ │
│   │  Phase 1         Phase 2         Phase 3                 │ │
│   │  DISCOVER        BUILD           AUDIT                   │ │
│   │  ┌────────┐      ┌────────┐      ┌────────┐            │ │
│   │  │ Scout  │─────▶│Builder │─────▶│Auditor │            │ │
│   │  │ Agent  │      │ Agent  │      │ Agent  │            │ │
│   │  └────────┘      └────────┘      └────────┘            │ │
│   │       │               │               │                 │ │
│   │       ▼               ▼               ▼                 │ │
│   │  scout-report    build-report    audit-report           │ │
│   │  + eval defs     (worktree)      PASS/WARN/FAIL        │ │
│   │                                      │                  │ │
│   │                              FAIL ◄──┤──► PASS          │ │
│   │                              retry   │   commit         │ │
│   │                              (max 3) │                  │ │
│   └──────────────────────────────────────────────────────────┘ │
│                                                                 │
│   Phase 5: Operator Agent ──▶ operator-log + next-cycle-brief  │
└─────────────────────────────────────────────────────────────────┘
```

### Per-Cycle Data Flow

```
                    state.json (read)
                         │
                         ▼
Phase 1: Scout ─── scout-report.md ──── .evolve/evals/*.md
                         │
                    (per task)
                         ▼
Phase 2: Builder ── build-report.md ── experiments.jsonl (append)
         (worktree)      │
                         ▼
Phase 3: Auditor ── audit-report.md
                         │
                    PASS? ─── YES ──▶ git commit
                         │
                    NO ──▶ retry (max 3)
                         │
Phase 4: Ship ──── git push ──── version bump ──── publish.sh
                         │
Phase 5: Learn ─── instinct extraction ──── state.json (write)
         │
         └── Operator ── operator-log.md ── next-cycle-brief.json
```

### Memory Architecture (6 Layers)

```
Layer 0: Shared Values ────── Static rules, prefix-cached, read-only
Layer 1: JSONL Ledger ─────── Append-only structured log (ledger.jsonl)
Layer 2: Markdown Workspace ── Per-cycle agent files (scout/build/audit/operator reports)
Layer 3: Persistent State ──── state.json (OCC-protected, cross-cycle)
Layer 4: Instincts ────────── YAML patterns with confidence scoring
Layer 5: History Archives ──── Immutable cycle snapshots (.evolve/history/)
Layer 6: Genes ────────────── Structured fix templates with selectors
```

---

## 4. Phase-by-Phase Detail

### Phase 0: CALIBRATE (once per invocation)

**Purpose:** Establish a quantitative project quality baseline.

**How it works:**
1. Run automated bash checks for 8 quality dimensions (e.g., grep for broken links, check frontmatter presence, validate schema consistency)
2. Run LLM judgment pass — tier-3 model (tier-2 for first calibration) scores each dimension against an anchored rubric (0/25/50/75/100)
3. Compute composite: `dimension = 0.7 × automated + 0.3 × LLM`
4. Compute overall: `mean(all 8 composites)`
5. Store in `state.json.projectBenchmark`
6. Identify weakest 2-3 dimensions → pass to Scout as `benchmarkWeaknesses`

**Model routing:** First calibration uses tier-2 for accurate baseline; subsequent calibrations use tier-3.

**Deduplication:** Skips if last calibration was <1 hour ago (parallel-run safe).

**Anti-gaming:** `benchmark-eval.md` is checksummed; Builder cannot modify it.

### Phase 1: DISCOVER (Scout Agent)

**Purpose:** Scan the project, identify improvement opportunities, select 2-4 tasks, write eval definitions.

**How it measures:**
- Reads `benchmarkWeaknesses` to prioritize dimensions needing improvement
- Reads `taskArms` (bandit) to bias toward high-reward task types
- Reads `fileExplorationMap` to boost novelty (files untouched for 3+ cycles)
- Reads `operatorBrief` for strategy recommendations
- Reads `experiments.jsonl` to avoid re-proposing failed approaches
- Reads `instinctSummary` to apply learned patterns

**Task selection algorithm:**
```
For each candidate task:
  priority = base_priority
  + benchmarkBoost (weakest dimension → +2)
  + banditBoost (avgReward >= 0.8 && pulls >= 3 → +1)
  + noveltyBoost (target files not in explorationMap for 3+ cycles → +1)
  + operatorBoost (task type in brief.taskTypeBoosts → +1)
  - avoidPenalty (task targets files in brief.avoidAreas → skip)

Select top 2-4 tasks within token budget (200K/cycle)
```

**Outputs:**
- `scout-report.md` with task list, rationale, decision trace
- `.evolve/evals/<task-slug>.md` with acceptance criteria and inline eval graders

### Phase 2: BUILD (Builder Agent, per task)

**Purpose:** Design and implement each task in an isolated git worktree.

**Isolation model:**
- Coding projects: `git worktree` (default)
- Writing/research: file-copy isolation
- Each task gets a fresh worktree; changes stay isolated until audit passes

**How it works:**
1. Read task from scout-report (includes inline eval graders)
2. Read relevant instincts from `instinctSummary`
3. Design approach using chain-of-thought reasoning
4. Implement with minimal changes
5. Self-verify: run eval graders, check acceptance criteria
6. Append result to `experiments.jsonl`

**Retry logic:** If audit fails, Builder gets issues and retries in a fresh worktree (max 3 attempts).

### Phase 3: AUDIT (Auditor Agent, per task)

**Purpose:** Independent quality gate before shipping.

**Checklist dimensions:**
1. **Code Quality** — naming, complexity, duplication
2. **Security** — injection, secrets, access control
3. **Pipeline Integrity** — eval tamper detection, protected file checks
4. **Eval Gate** — run all eval graders, verify acceptance criteria

**Verdict system:**

| Verdict | Meaning | Action |
|---------|---------|--------|
| PASS | No MEDIUM+ issues, all evals pass | Merge to main |
| WARN | MEDIUM issues found | Retry Builder with issues |
| FAIL | CRITICAL/HIGH issues or eval failures | Retry Builder with issues |

**Adaptive strictness:** Task types with `consecutiveClean >= 5` get reduced checklist (Security + Eval Gate only). Types touching agent/skill files always get full checklist.

### Benchmark Delta Check (between AUDIT and SHIP)

**Purpose:** Verify the cycle improved (or didn't regress) project quality.

**How it calculates:**
1. Re-run automated checks for relevant dimensions only
2. Compare to Phase 0 baseline:
   - Any dimension improved (+2 or more) → Ship
   - All stable (±1) → Ship with warning
   - Any dimension regressed (-3 or more) → Block, return to Builder
3. Blocked tasks get 1 retry; second block drops the task entirely

### Phase 4: SHIP (Orchestrator inline)

**Purpose:** Persist and distribute completed work.

**Steps:**
1. Acquire `.evolve/.ship-lock` (serial, one run at a time)
2. `git pull --rebase origin main`
3. `git push origin main`
4. Auto-bump patch version in `plugin.json` / `marketplace.json`
5. Run `./publish.sh` (syncs cache for Claude Code + Gemini CLI)
6. Update `state.json`:
   - Mark tasks completed in `evaluatedTasks`
   - Update bandit arms (`taskArms`)
   - Compute `fitnessScore = 0.25×discover + 0.30×build + 0.20×audit + 0.15×ship + 0.10×learn`
   - Record `processRewards` per phase
   - Update `evalHistory` with delta metrics
   - Update `mastery.consecutiveSuccesses`
   - Increment OCC `version`
7. Release ship lock

### Phase 5: LEARN (Orchestrator + Operator Agent)

**Purpose:** Extract knowledge, consolidate memory, check pipeline health.

**Sub-steps:**

| Step | What | How |
|------|------|-----|
| 1. Archive | Copy workspace to `.evolve/history/cycle-N/` | `cp` |
| 2. Consolidation check | If cycle % 3 == 0 or instinctCount > 20 | Cluster, decay, archive |
| 3. Instinct citation | Scan scout/build reports for `instinctsApplied` | Boost cited instinct confidence by +0.05 |
| 4. Instinct extraction | Deep reasoning about what worked/failed | Write YAML to `.evolve/instincts/personal/` |
| 5. Self-evaluation | LLM-as-a-Judge on 4 dimensions | Correctness, Completeness, Novelty, Efficiency (≥0.7 pass); tier-1 for problem cycles (audit retries/eval failures/miscalibration) |
| 6. Operator check | Launch Operator agent | Stall detection, recommendations, next-cycle brief; tier-2 for last cycle/fitness regression/meta-cycle |
| 7. Notes update | Append cycle entry to `notes.md` | Rolling window, compressed every 5 cycles |
| 8. Cycle summary | Output enhanced box-format summary | Benchmark delta, audit iterations, warnings |
| 9. Handoff | Write checkpoint to `handoff.md` | Compaction anchor for context survival |

**Instinct graduation:** Instincts with confidence ≥ 0.75, confirmed in 3+ cycles, and not contradicted → `graduated: true` → Builder applies them as mandatory guidance.

**Meta-cycle (every 5 cycles):**
- Split-role critique (Efficiency, Correctness, Novelty critics)
- Agent effectiveness evaluation
- Up to 2 automated prompt edits (TextGrad-style)
- Mutation testing (target: >80% kill rate)
- Topology review

---

## 5. Evaluation & Measurement System

### Scoring Formulas

| Metric | Formula | Purpose |
|--------|---------|---------|
| Benchmark composite | `0.7 × automated + 0.3 × LLM` | Per-dimension quality score |
| Overall benchmark | `mean(8 dimension composites)` | Single project quality number |
| Fitness score | `0.25×discover + 0.30×build + 0.20×audit + 0.15×ship + 0.10×learn` | Weighted process quality |
| CSI (Coefficient of Self-Improvement) | `(fitness[N] - fitness[N-k]) / k` (k=3) | Is the loop getting better? |
| Calibration error | `|mean_confidence - actual_accuracy|` (k=5) | Are self-evaluations honest? |
| Bandit avg reward | `totalReward / pulls` | Task type success rate |

### Process Rewards Rubric

| Phase | 1.0 | 0.5 | 0.0 |
|-------|-----|-----|-----|
| Discover | All tasks shipped | 50%+ shipped | <50% shipped |
| Build | All pass first attempt | Some retries needed | 3+ audit failures |
| Audit | No false positives | 1 false positive | Multiple false positives |
| Ship | Clean commit | Minor fixup | Failed to push |
| Learn | Instincts extracted AND cited | Extracted but not cited | None extracted |
| Skill Efficiency | Tokens decreased | Stable (±5%) | Tokens increased |

### Convergence Detection

The loop stops when:
1. Cycle limit reached (user-specified)
2. `stagnation.nothingToDoCount >= 3` (Scout found nothing 3 times)
3. Operator HALT (quality regression, 3+ active stagnation patterns)

---

## 6. Roadmap & Remaining Work

### Completed Milestones

| Phase | Cycles | Key Deliverables |
|-------|--------|-----------------|
| Core pipeline | 1-7 | 5 phases, 4 agents, eval gating, domain generalization |
| Research integration | 8-13 | CoT, MSV, mutation testing, bandit selection, plan caching |
| Self-improvement | 16-21 | Stepwise scoring, CSI, confidence alignment, security self-check |
| Documentation | 22-24 | Operator brief, run isolation, experiment journal, parallel safety |
| Platform compatibility | 25-131 | Multi-platform support, phase decomposition, agent templates |
| Pipeline optimization | 132-141 | Self-MoA builds, budget-aware agents, speculative auditor, phase gate |
| Research expansion | 142-151 | Enterprise eval, agent personalization, adversarial co-evolution, runtime guardrails |

### Open Items

- [ ] Design domain detection signals and export pipeline
- [ ] End-to-end validation on a real writing or research project
- [ ] Domain-specific instinct templates

---

## 7. Reference Documents

| Document | Purpose |
|----------|---------|
| [architecture.md](architecture.md) | System design, agents, memory layers |
| [self-learning.md](self-learning.md) | 7 self-improvement mechanisms |
| [memory-hierarchy.md](memory-hierarchy.md) | 6-layer memory architecture |
| [instincts.md](instincts.md) | Instinct system, graduation, consolidation |
| [operator-brief.md](operator-brief.md) | Cross-cycle communication protocol |
| [run-isolation.md](run-isolation.md) | Parallel invocation safety |
| [parallel-safety.md](parallel-safety.md) | OCC, ship-lock, conflict resolution |
| [experiment-journal.md](experiment-journal.md) | Anti-repeat memory |
| [genes.md](genes.md) | Structured fix templates |
| [meta-cycle.md](meta-cycle.md) | Every-5-cycles self-evaluation |
| [island-model.md](island-model.md) | Advanced parallel evolution |
| [token-optimization.md](token-optimization.md) | Cost reduction strategies |
| [configuration.md](configuration.md) | Setup and domain detection |
| [showcase.md](showcase.md) | Annotated cycle walkthrough |
