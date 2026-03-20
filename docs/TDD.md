# Evolve Loop — Technical Design Document (TDD)

**Version:** 7.3.0 | **Last Updated:** 2026-03-20 | **Cycles Completed:** 24

---

## 1. System Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         AI CLI Host (Claude Code / Gemini CLI)      │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                       ORCHESTRATOR                            │  │
│  │  (SKILL.md entry point — runs in main CLI session context)    │  │
│  │                                                               │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐ │  │
│  │  │Phase 0  │  │Phase 4  │  │Phase 5  │  │ Context Manager │ │  │
│  │  │CALIBRATE│  │  SHIP   │  │  LEARN  │  │ (handoff.md,    │ │  │
│  │  │(bench-  │  │(git,    │  │(instinct│  │  state.json,    │ │  │
│  │  │ mark)   │  │ publish)│  │ extract)│  │  lean mode)     │ │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘ │  │
│  │                                                               │  │
│  │  ┌─────────────────── Agent Pool ──────────────────────────┐ │  │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐ │ │  │
│  │  │  │  Scout   │  │ Builder  │  │ Auditor  │  │Operator│ │ │  │
│  │  │  │ (sonnet) │  │ (sonnet) │  │ (sonnet) │  │(haiku) │ │ │  │
│  │  │  │          │  │ worktree │  │ eval gate│  │ health │ │ │  │
│  │  │  └──────────┘  └──────────┘  └──────────┘  └────────┘ │ │  │
│  │  └─────────────────────────────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────── Persistent Storage ──────────────────────┐  │
│  │  .evolve/                                                     │  │
│  │  ├── state.json          (OCC-protected, version field)       │  │
│  │  ├── ledger.jsonl        (append-only, structured)            │  │
│  │  ├── notes.md            (rolling window, compressed/5 cyc)   │  │
│  │  ├── project-digest.md   (shared, regenerated every 10 cyc)   │  │
│  │  ├── latest-brief.json   (shared, last-writer-wins)           │  │
│  │  ├── runs/{RUN_ID}/workspace/  (per-run isolation)            │  │
│  │  ├── evals/              (shared, Scout-written)              │  │
│  │  ├── instincts/personal/ (YAML, confidence-scored)            │  │
│  │  ├── instincts/archived/ (superseded, never deleted)          │  │
│  │  ├── genes/              (fix templates)                      │  │
│  │  └── history/cycle-N/    (immutable archives)                 │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### Concurrency Model

Multiple `/evolve-loop` invocations can run in parallel against the same repo:

```
Run A (cycles 22, 23)          Run B (cycles 24, 25)
  │                              │
  ├─ OCC claim cycle 22         ├─ OCC claim cycle 24
  ├─ Scout (own workspace)      ├─ Scout (own workspace)
  ├─ Builder (worktree A)       ├─ Builder (worktree B)
  ├─ Auditor                    ├─ Auditor
  ├─ SHIP (acquire lock) ◄──── waits for lock ────┤
  │  git push                    │
  │  release lock ──────────────► SHIP (acquire lock)
  ├─ LEARN                      │  git pull --rebase
  └─ cycle 23...                │  git push
                                │  release lock
                                ├─ LEARN
                                └─ cycle 25...
```

**Coordination primitives:**

| Primitive | Mechanism | Protects |
|-----------|-----------|----------|
| OCC (Optimistic Concurrency Control) | `version` field in state.json | Cycle allocation, task claiming, state updates |
| Ship Lock | `mkdir .evolve/.ship-lock` (atomic on POSIX) | Git push serialization |
| Run Isolation | Per-run `$WORKSPACE_PATH` | Workspace file conflicts |
| Append-only logs | File append semantics | `ledger.jsonl`, `experiments.jsonl` |

---

## 2. State Machine

### Cycle Lifecycle

```
                    ┌──────────────────────┐
                    │   CLAIM CYCLE (OCC)   │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   PHASE 1: DISCOVER   │
                    │   (Scout agent)       │
                    └──────────┬───────────┘
                               │
                    no tasks?──┤──► STAGNATION CHECK
                               │         │
                               │    nothingToDoCount >= 3?
                               │         │
                               │    YES → CONVERGED (stop)
                               │    NO  → Phase 5 (operator only)
                               │
                    ┌──────────▼───────────┐
               ┌───▶│   PHASE 2: BUILD     │◄──────────────┐
               │    │   (Builder agent,     │               │
               │    │    worktree)          │               │
               │    └──────────┬───────────┘               │
               │               │                           │
               │    ┌──────────▼───────────┐               │
               │    │   PHASE 3: AUDIT     │               │
               │    │   (Auditor agent)     │               │
               │    └──────────┬───────────┘               │
               │               │                           │
               │         ┌─────┴─────┐                     │
               │         │           │                     │
               │       PASS        WARN/FAIL               │
               │         │           │                     │
               │         │      attempts < 3? ──YES──────┘
               │         │           │
               │         │        NO → log failure, skip task
               │         │
               │    ┌────▼─────────────────┐
               │    │  BENCHMARK DELTA CHECK│
               │    └────┬─────────────────┘
               │         │
               │    regression? ──YES──► retry (1 more attempt)
               │         │
               │         NO
               │         │
               │    more tasks? ──YES───┘
               │         │
               │         NO
               │         │
               │    ┌────▼─────────────────┐
               │    │   PHASE 4: SHIP       │
               │    │   (acquire lock,      │
               │    │    push, bump, publish)│
               │    └────┬─────────────────┘
               │         │
               │    ┌────▼─────────────────┐
               │    │   PHASE 5: LEARN      │
               │    │   (instincts, eval,   │
               │    │    operator, handoff)  │
               │    └────┬─────────────────┘
               │         │
               │    remainingCycles > 0?
               │         │
               │    YES ─┘  (next cycle)
               │
               │    NO → RUN CLEANUP
               │         │
               │    ┌────▼─────────────────┐
               │    │  FINAL SESSION REPORT │
               │    │  (output to user)     │
               │    └──────────────────────┘
```

### Mastery State Machine

```
┌────────┐  3 consecutive  ┌───────────┐  3 more      ┌────────────┐
│ NOVICE │ ──successes────▶│ COMPETENT │ ─successes──▶│ PROFICIENT │
│ S only │                 │ S + M     │              │ S + M + L  │
└────────┘                 └───────────┘              └────────────┘
     ▲                          ▲                          │
     └──── 2 cycles < 50% ─────┴──── 2 cycles < 50% ─────┘
```

Current: **Proficient** (22 consecutive successes)

---

## 3. Data Schemas

### state.json (Core Fields)

```json
{
  "lastCycleNumber": 24,
  "version": 29,                              // OCC version counter
  "strategy": "balanced",                     // Current strategy preset
  "mastery": {"level": "proficient", "consecutiveSuccesses": 22},

  "projectBenchmark": {
    "overall": 91.0,                          // Mean of 8 dimension composites
    "dimensions": {                           // Per-dimension: automated, llm, composite
      "documentationCompleteness": {"automated": 95, "llm": 75, "composite": 89},
      "defensiveDesign": {"automated": 100, "llm": 100, "composite": 100}
      // ... 6 more dimensions
    },
    "history": [                              // Benchmark trajectory (last 5 calibrations)
      {"cycle": 1, "overall": 83.8},
      {"cycle": 3, "overall": 86.0},
      {"cycle": 8, "overall": 88.1},
      {"cycle": 16, "overall": 87.4}
    ],
    "highWaterMarks": {"defensiveDesign": 100, "conventionAdherence": 100}
  },

  "taskArms": {                               // Multi-armed bandit
    "feature": {"pulls": 18, "totalReward": 18, "avgReward": 1.0},
    "techdebt": {"pulls": 18, "totalReward": 18, "avgReward": 1.0},
    "stability": {"pulls": 7, "totalReward": 7, "avgReward": 1.0}
    // ... security (2 pulls), performance (1 pull)
  },

  "evalHistory": [                            // Last 5 cycle results
    {"cycle": 24, "verdict": "PASS", "checks": 7, "passed": 7, "failed": 0,
     "delta": {"tasksShipped": 3, "tasksAttempted": 3, "auditIterations": 1.0,
               "successRate": 1.0, "instinctsExtracted": 0}}
  ],

  "processRewards": {                         // Current cycle scores (0.0-1.0)
    "discover": 1.0, "build": 1.0, "audit": 1.0,
    "ship": 1.0, "learn": 0.5, "skillEfficiency": 1.0
  },

  "fitnessScore": 0.95,                      // Weighted process quality
  "fitnessHistory": [{"cycle": 22, "score": 0.95}, {"cycle": 23, "score": 0.95}],

  "instinctSummary": [                        // Compact instinct index
    {"id": "inst-005", "pattern": "cross-reference-new-docs", "confidence": 0.7}
  ],

  "ledgerSummary": {                          // Aggregated stats (agents read this, not full ledger)
    "totalTasksShipped": 59, "totalTasksFailed": 0, "avgTasksPerCycle": 2.5
  }
}
```

### Eval Definition Format

```markdown
# Eval: <task-slug>

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Eval Graders (bash, exit 0 = pass)
- `test -f docs/new-file.md` → expects exit 0
- `grep -q "required-content" docs/new-file.md` → expects exit 0
- `wc -l < docs/new-file.md | awk '{exit ($1 < 30 || $1 > 120)}'` → expects exit 0
```

### Instinct YAML Format

```yaml
- id: inst-005
  pattern: "cross-reference-new-docs"
  description: "When adding a new doc, always add a cross-reference link from architecture.md or the parent doc. Orphaned docs reduce modularity scores."
  confidence: 0.70
  source: "cycle-2/fix-architecture-mailbox-reference"
  type: "convention"
  category: "semantic"
  graduated: false
```

### Experiment Journal (experiments.jsonl)

```jsonl
{"cycle":22,"task":"add-operator-brief-spec-doc","attempt":1,"verdict":"PASS","approach":"create standalone doc","metric":"7/7 checks passed"}
{"cycle":22,"task":"add-run-isolation-doc","attempt":1,"verdict":"PASS","approach":"document RUN_ID/WORKSPACE_PATH model","metric":"5/5 checks passed"}
```

---

## 4. Algorithms & Calculations

### 4.1 Benchmark Scoring

```
Per dimension:
  composite = round(0.7 × automated_score + 0.3 × llm_score)

Overall:
  overall = round(mean(all 8 composites), 1)

Regression detection:
  if any dimension.current < dimension.baseline - 3 → BLOCK shipping
  if any dimension.current > dimension.highWaterMark → update HWM
  if any dimension.current < dimension.HWM - 10 → mandatory remediation task
```

### 4.2 Fitness Score

Weighted average of process reward dimensions:

```
fitnessScore = round(
  0.25 × discover +
  0.30 × build +
  0.20 × audit +
  0.15 × ship +
  0.10 × learn
, 2)
```

Build gets the highest weight (0.30) because it's the core value-producing phase.

**Regression detection:**
- If fitnessScore decreased for 2 consecutive cycles → `fitnessRegression: true`
- Operator reads this as a HALT-worthy signal

### 4.3 Coefficient of Self-Improvement (CSI)

```
CSI = (fitnessScore[N] - fitnessScore[N-k]) / k    where k = 3

CSI > 0  → improving (continue)
CSI ≈ 0  → plateau (increase complexity or novelty)
CSI < 0  → regression (investigate, possibly HALT)
```

### 4.4 Multi-Armed Bandit (Thompson Sampling)

```
After each shipped task:
  arm.pulls += 1
  if success: arm.totalReward += 1
  arm.avgReward = arm.totalReward / arm.pulls

During task selection:
  if arm.avgReward >= 0.8 AND arm.pulls >= 3:
    task receives +1 priority boost
  if arm.pulls < 3:
    task is always eligible (exploration floor)
```

### 4.5 Instinct Confidence Evolution

```
New instinct:        confidence = 0.5
Cited by agent:      confidence += 0.05 (max 1.0)
Graduated citation:  confidence += 0.10 (max 1.0)
Consolidation decay: confidence -= 0.10 (if unreferenced for 5 cycles)
Graduation reversal: confidence -= 0.20 (on 2+ consecutive failures)
Archive threshold:   confidence < 0.30 → archived as stale
```

### 4.6 Confidence-Correctness Alignment

```
calibration_error = |mean_confidence - actual_accuracy|
  (rolling window k = 5 cycles)

mean_confidence = avg(self-evaluation scores across dimensions)
actual_accuracy = fraction(eval graders passed)

if calibration_error > 0.15 → force recalibration:
  - Require 3 evidence items per dimension (up from 2)
  - Force stepwise scoring even above 0.7

Auto-disable when calibration_error < 0.10 for 2 consecutive cycles
```

---

## 5. Token Economics

### Per-Cycle Budget

| Component | Typical | Budget | Optimization |
|-----------|---------|--------|-------------|
| Scout (Phase 1) | 40-60K | — | Incremental after cycle 1; haiku for simple scans |
| Builder (Phase 2) | 30-50K per task | 80K/task | Inline S-tasks save ~30-50K; plan cache saves ~30-50% |
| Auditor (Phase 3) | 20-30K | — | Adaptive strictness; haiku for clean builds |
| Ship + Learn (Phase 4-5) | ~5K | — | Orchestrator inline + haiku Operator |
| **Total/cycle** | **~100-150K** | **200K** | Lean mode after cycle 4 saves ~15-20K |

### Model Routing

| Phase | Default | Upgrade → | Downgrade → |
|-------|---------|-----------|-------------|
| Scout | sonnet | opus (deep research) | haiku (incremental) |
| Builder | sonnet | opus (M + 5+ files) | haiku (S inline) |
| Auditor | sonnet | opus (security) | haiku (clean build) |
| Operator | haiku | sonnet (HALT suspected) | — |
| Meta-cycle | opus | — | — |

### Context Management

- **Lean mode** (cycles 4+): Read state.json once per cycle, extract agent results from return value, skip redundant file reads
- **Handoff checkpoint**: Written after each cycle as compaction anchor
- **Notes compression**: Every 5 cycles, compress old entries to ~500-byte summary
- **instinctSummary/ledgerSummary**: Agents read compact summaries instead of full files

---

## 6. Safety & Integrity

### Eval Tamper Detection

```
Phase 1: Scout writes evals → orchestrator computes sha256sum
Phase 3: Before Auditor runs → orchestrator verifies sha256sum
         If mismatch → HALT: "Eval tamper detected"
```

**Protected paths:** Builder MUST NOT modify `skills/evolve-loop/`, `agents/`, `.claude-plugin/` unless the task explicitly targets evolve-loop itself.

### Rollback Protocol

- All changes committed atomically per task → `git revert <SHA>` always works
- 3 consecutive quality degradation cycles → auto-suggest rollback
- Meta-cycle prompt edits auto-revert if next meta-cycle shows worse performance

### Convergence Safety

| Signal | Threshold | Action |
|--------|-----------|--------|
| Scout finds nothing | nothingToDoCount >= 3 | STOP: "Project converged" |
| Quality regression | fitnessScore down 2 consecutive | Operator HALT |
| Stagnation patterns | 3+ active simultaneously | Operator HALT |
| Cost warning | cycles >= warnAfterCycles (default 5) | Warn user |

---

## 7. Complete Task History

### Tasks by Cycle

| Cycle | Tasks Shipped | Theme |
|-------|---------------|-------|
| 1 | 3 | Phase splitting, token budgets, policy design |
| 2 | 3 | Skill building, context handoff, architecture fix |
| 3 | 3 | Domain adapters, detection guide, non-code evals |
| 4 | 2 | Domain-aware init, writing showcase |
| 5 | 2 | Domain-aware SHIP, generalization status |
| 6 | 2 | BUILD isolation adapter, domain benchmark |
| 7 | 2 | Generalization status, research walkthrough |
| 8 | 3 | Broken links, token optimization, accuracy doc |
| 9 | 3 | Performance profiling, security doc, CoT graders |
| 10 | 2 | README update, project digest |
| 11 | 2 | Plan cache spec, instinct graduation spec |
| 12 | 2 | Changelog, parallel research dedup |
| 13 | 3 | CoT Builder, MSV Auditor, mutation testing |
| 16 | 3 | Link fixes, stepwise eval, experience scoring |
| 17 | 3 | MUSE categories, CSI metric, Phase 4 extraction |
| 18 | 3 | Link-checker fix, confidence alignment, schema fix |
| 19 | 3 | Changelog, project digest, self-evolving taxonomy |
| 20 | 3 | Instinct extraction, validation protocol, scout guide |
| 21 | 3 | Stepwise enforcement, security self-check, schema fix |
| 22 | 3 | Operator brief doc, run isolation doc, experiment journal doc |
| 23 | 3 | Changelog v7.3.0, instinct graduation section, parallel safety doc |
| 24 | 3 | Architecture reference links, island-model integration, genes integration |

**Totals:** 59 tasks shipped, 0 failed, 24 cycles completed.

*Note: Cycles 14-15 were skipped (session gap between deployments).*

### Cumulative Growth

```
Tasks
 60 ┤                                                          ●
 55 ┤                                                    ●─────┘
 50 ┤                                              ●─────┘
 45 ┤                                        ●─────┘
 40 ┤                                  ●─────┘
 35 ┤                            ●─────┘
 30 ┤                      ●─────┘
 25 ┤                ●─────┘
 20 ┤          ●─────┘
 15 ┤    ●─────┘
 10 ┤────┘
  5 ┤──┘
  0 ┤
    └──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──
       1  3  5  7  9  11 13 16 18 20 22 24   Cycle
```

---

## 8. File Map

### Skill Files (orchestrator reads these)

| File | Lines | Purpose |
|------|-------|---------|
| `skills/evolve-loop/SKILL.md` | ~444 | Entry point, init, orchestrator loop, run cleanup, final report |
| `skills/evolve-loop/phases.md` | ~483 | Phase 0-3 detailed instructions |
| `skills/evolve-loop/phase4-ship.md` | ~216 | Phase 4 SHIP, process rewards, version bump |
| `skills/evolve-loop/phase5-learn.md` | ~430 | Phase 5 LEARN, instincts, operator, meta-cycle |
| `skills/evolve-loop/memory-protocol.md` | ~422 | State schema, ledger format, OCC protocol |
| `skills/evolve-loop/eval-runner.md` | ~261 | Eval execution, grader types, non-code graders |
| `skills/evolve-loop/benchmark-eval.md` | ~479 | 8 benchmark dimensions, automated checks, LLM rubrics |

### Agent Definitions

| File | Lines | Model | Workspace Output |
|------|-------|-------|-----------------|
| `agents/evolve-scout.md` | ~333 | sonnet | `scout-report.md` |
| `agents/evolve-builder.md` | ~222 | sonnet | `build-report.md` |
| `agents/evolve-auditor.md` | ~193 | sonnet | `audit-report.md` |
| `agents/evolve-operator.md` | ~237 | haiku | `operator-log.md` |

### Documentation

| File | Purpose |
|------|---------|
| `docs/PRD.md` | Product requirements, status, architecture overview |
| `docs/TDD.md` | Technical design, schemas, algorithms, state machine |
| `docs/architecture.md` | System design, agents, memory, coordination |
| `docs/self-learning.md` | 7 self-improvement mechanisms |
| `docs/memory-hierarchy.md` | 6-layer memory, access matrix |
| `docs/instincts.md` | Instinct system, graduation, consolidation |
| `docs/operator-brief.md` | Cross-cycle communication protocol |
| `docs/run-isolation.md` | Parallel invocation safety |
| `docs/parallel-safety.md` | OCC, ship-lock, conflict resolution |
| `docs/experiment-journal.md` | Anti-repeat memory |
| `docs/genes.md` | Fix templates |
| `docs/meta-cycle.md` | Every-5-cycles self-evaluation |
| `docs/island-model.md` | Parallel configuration evolution |
| `docs/token-optimization.md` | Cost reduction strategies |
| `docs/configuration.md` | Setup and domain detection |
| `docs/showcase.md` | Annotated cycle walkthrough |
| `docs/accuracy-self-correction.md` | CoT + multi-stage verification |
| `docs/performance-profiling.md` | Cost-bottleneck analysis |
| `docs/security-considerations.md` | Pipeline integrity |
| `docs/domain-adapters.md` | Multi-domain support |
| `docs/policy-design.md` | Agent policy patterns |
| `docs/eval-grader-best-practices.md` | Eval authoring guide |
