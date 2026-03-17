# Architecture

## Overview

Evolve Loop is a self-evolving development pipeline that orchestrates 4 specialized agents across 5 lean phases. The orchestrator (the main AI CLI session) coordinates agents via the Agent tool, managing data flow through a shared workspace, JSONL ledger, and persistent state with strategy-aware execution.

## Design Principles

### 1. Fast Iteration
Build diverse small/medium tasks each cycle. Prefer 3 small tasks over 1 large task. Each task should be completable in a single Builder pass.

### 2. Agent Isolation
Each agent has a single responsibility and owns exactly one workspace file. Agents communicate through workspace files, not direct messaging.

### 3. Hard Gates
The Auditor gates on MEDIUM+ severity findings. Both code quality issues and eval check failures block shipping. Eval tamper detection prevents bypassing gates.

### 4. Continuous Learning
Each cycle extracts instincts (episodic, semantic, procedural categories) that future cycles can read. Instincts are specific and actionable. High-confidence instincts graduate to orchestrator policy. Memory consolidation runs every 3 cycles.

### 5. Safe Autonomy
The Operator monitors for stalls, quality degradation, and repeated failures using delta metrics and MAP-Elites fitness scoring. It can HALT the loop, requiring human intervention.

### 6. No External Dependencies
All agents are self-contained. No dependency on external plugins or frameworks.

### 7. Strategy-Driven Execution
Four strategy presets (`balanced`, `innovate`, `harden`, `repair`) shape task selection, build approach, and audit strictness across all phases.

## Pipeline

```
Phase 1: DISCOVER ─── [Scout] scan + research + task selection
Phase 2: BUILD ────── [Builder] design + implement + self-test (worktree)
Phase 3: AUDIT ────── [Auditor] review + security + eval gate + tamper detection
Phase 4: SHIP ──────── orchestrator (commit + push + delta metrics)
Phase 5: LEARN ──────── orchestrator (instincts + consolidation) + [Operator] (health)
```

For multiple tasks, Phase 2-3 loop per task:
```
Scout → [Task A, Task B, Task C]
  → Builder(A) → Auditor(A) → commit
  → Builder(B) → Auditor(B) → commit
  → Builder(C) → Auditor(C) → commit
→ Ship → Learn
```

Every 5 cycles, Phase 5 includes a **meta-cycle**: split-role critique, agent effectiveness evaluation, prompt evolution, and mutation testing.

## Agents

| Agent | Purpose | Key Principle |
|-------|---------|---------------|
| Scout | Discovery + analysis + task selection | Incremental after cycle 1, difficulty graduation |
| Builder | Design + implement + self-test | Minimal change, capability gap detection |
| Auditor | Review + security + eval + tamper detection | Single-pass, MEDIUM+ blocks |
| Operator | Loop health + delta analysis + fitness scoring | Stall detection, MAP-Elites, HALT protocol |

### Dynamic Model Routing

Agents use different models based on task complexity:
- **Haiku** — Operator health checks, incremental Scout scans
- **Sonnet** — Standard Builder/Auditor work (default)
- **Opus** — Deep research, complex architectural tasks

## Shared Memory Architecture

See [memory-hierarchy.md](memory-hierarchy.md) for details.

### Layer 1: JSONL Ledger
Append-only structured log. Every agent appends one entry per invocation.

### Layer 2: Markdown Workspace
Human-readable files overwritten each cycle. Each agent reads upstream files and writes its own output.

### Layer 3: Persistent State
`state.json` persists across cycles: research cache (12hr TTL), task history, failed approaches (with structured reasoning), eval history (with delta metrics), instinct count, stagnation tracking, token budgets, plan cache, mastery level, and synthesized tools.

### Layer 4: Eval State
Eval definitions (created by Scout) and baseline results. Grep-based evals for Markdown/Shell projects.

### Layer 5: Instincts
Extracted patterns from completed cycles. YAML files with confidence scoring, categorized as episodic, semantic, or procedural. High-confidence instincts (0.9+) graduate to orchestrator policy. Memory consolidation merges related instincts every 3 cycles, archiving superseded entries.

### Layer 6: Genes
Structured fix templates (capsules) with selectors, validators, and applicability rules. Extracted from successful Builder implementations for reuse across cycles.

## Stagnation Detection

The orchestrator monitors three stagnation patterns after every Scout phase:
1. **Same-file churn** — same files appear in `failedApproaches` across 2+ consecutive cycles
2. **Same-error repeat** — same error message recurs across cycles
3. **Diminishing returns** — last 3 cycles each shipped fewer tasks than previous

When 3+ stagnation patterns are active simultaneously, the Operator triggers HALT.

## Mastery & Difficulty Graduation

The system tracks mastery level based on consecutive successes:
- **Novice** (0-2 successes) — S-complexity tasks only
- **Competent** (3-5) — S and M tasks
- **Proficient** (6+) — S, M, and L tasks

## Token Optimization

See [token-optimization.md](token-optimization.md) for details.

| Component | Tokens | Notes |
|-----------|--------|-------|
| Discovery | ~40-60K | Incremental after cycle 1 |
| Build | ~30-50K | Per task, S-complexity inline |
| Verify | ~20-30K | Single-pass audit |
| Ship+Learn | ~5K | Inline + Operator (haiku) |
| **Total/cycle** | **~100-150K** | **Budget: 200K/cycle** |

Key optimizations:
- **Fewer handoffs** — no context loss between separate architect/developer agents
- **Incremental scan** — cycle 2+ only scans what changed, not the whole codebase
- **12hr research cooldown** — web research reuses cached results
- **Single-pass audit** — one agent covers review, security, and eval
- **Inline ship** — orchestrator commits directly, no Deployer agent needed
- **Inline S-tasks** — orchestrator implements small tasks directly (inst-007 policy)
- **Plan template caching** — reuse plan structures for recurring task types (~30-50% savings)
- **Dynamic model routing** — haiku for lightweight work, opus only when needed

## Self-Improvement Infrastructure

See [self-learning.md](self-learning.md) for details.

The loop includes seven interconnected mechanisms for autonomous self-improvement:

### Process Rewards Remediation Loop
Per-cycle check in Phase 4: if any `processRewards` dimension scores below 0.7 for 2+ consecutive entries in `processRewardsHistory`, a structured remediation entry is auto-generated in `state.json.pendingImprovements`. The Scout reads these as high-priority task candidates, creating a tight feedback loop from metrics to action.

### Scout Introspection Pass
Before task selection, the Scout reviews `evalHistory` delta metrics and applies 7 heuristics to detect pipeline issues and capability gaps:
- **Performance heuristics** (5): instinct stagnation, audit retry rate, stagnation patterns, success rate, pending improvements
- **Capability gap signals** (2): overdue deferred tasks, dormant instincts uncited for 3+ cycles

Introspection-sourced tasks are labeled `source: "introspection"` or `source: "capability-gap"` and receive a priority boost in task selection.

### Process Rewards History
Rolling 3-entry window (`processRewardsHistory` in state.json) enables trend detection — distinguishing sustained degradation from one-off dips. Feeds both the remediation loop and meta-cycle reviews.

### Multi-Armed Bandit Task Selection
The Scout maintains a `taskArms` table in `state.json` with per-type reward history across five task types: `feature`, `stability`, `security`, `techdebt`, `performance`. After each shipped task, the arm for that task type is updated (pulls + 1, totalReward + 1 on success). Before finalizing the task list, the Scout applies Thompson Sampling-style weighting: arms with `avgReward >= 0.8` and `pulls >= 3` receive a +1 priority boost in selection ranking. This creates a closed feedback loop — the loop learns which task types it executes well and shifts investment toward them, without abandoning exploration of lower-pull arms.

### Semantic Task Crossover
When the Scout selects tasks, it checks the last 5 completed tasks for combinable patterns: if two completed task types haven't been combined before, a hybrid child task is synthesized (e.g., `security` + `performance` → a task that hardens a hot path). Crossover tasks are labeled `source: "crossover"` and carry both parent type signatures. This adds exploratory diversity without fully random mutations.

### Intrinsic Novelty Reward
A `fileExplorationMap` in `state.json` tracks which files each cycle has touched. When a Builder modifies a file not touched in the last 5 cycles, the task's reward is boosted by +0.1 (capped at 1.0). This nudges the loop toward unexplored areas of the codebase, preventing over-concentration on familiar files.

### Scout Decision Trace
The Scout appends a `decisionTrace` array to `scout-report.md` — one entry per candidate task listing `finalDecision` (selected/rejected/deferred) and `signals` (the reasons). This makes task selection auditable and gives the Operator a structured record for meta-cycle analysis.

## Agent Coordination Features

### Prerequisite Task Graph
Tasks can declare `dependsOn: [slug, ...]` in the scout report. The orchestrator respects this ordering: a task is not started until all its prerequisites have shipped in the current cycle. Circular dependencies are detected and rejected at Scout time.

### Builder Retrospective
After each build, the Builder appends a `## Retrospective` block to `build-report.md` with: what went well, what was harder than expected, and a suggested follow-up task slug. The Operator reads these retrospectives during the health check and forwards high-signal suggestions to `state.json.pendingImprovements`.

### Auditor Adaptive Strictness
The Auditor reads `taskArms` from state to determine audit depth. Task types with `avgReward < 0.5` receive a stricter audit pass (all MINOR findings reported, tighter threshold for MEDIUM escalation). Task types with `avgReward >= 0.9` and `pulls >= 5` receive a fast-path audit (skip redundant heuristics). This concentrates audit effort where it matters most.

### Agent Mailbox
Each agent can leave a structured message for a downstream agent by appending to `workspace/mailbox.json` (array of `{from, to, subject, body}`). The downstream agent reads mailbox entries addressed to it at the start of its phase. This replaces ad-hoc inline notes in workspace files and gives inter-agent communication a durable, inspectable channel.

### Operator Next-Cycle Brief and Session Narrative
At the end of each cycle, the Operator writes two outputs: a `next-cycle-brief` embedded in `operator-report.md` (recommended strategy, priority task types, watchlist for next cycle) and a `session-narrative` paragraph summarizing the arc of the current cycle in plain prose. When `cycle == endCycle` (last cycle of a session), the Operator also writes `workspace/session-summary.md` — a full-session retrospective covering total tasks shipped, key features added, fitness trend, and a 3-sentence synthesis.

## Context Management

At 60% context usage, the orchestrator writes a `handoff.md` file with session state, then the stop-hook resets context. The next conversation resumes from `handoff.md`, enabling indefinite runtime across context boundaries.
