# Architecture

## Overview

Evolve Loop is a self-evolving development pipeline that orchestrates 4 specialized agents across 5 lean phases. The orchestrator (the main Claude Code session) coordinates agents via the Agent tool, managing data flow through a shared workspace, JSONL ledger, and persistent state with strategy-aware execution.

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
The Operator monitors for stalls, quality degradation, and repeated failures using delta metrics and MAP-Elites fitness scoring. It can HALT the loop, requiring human intervention. Denial-of-wallet guardrails cap cycles per session.

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

## Context Management

At 60% context usage, the orchestrator writes a `handoff.md` file with session state, then the stop-hook resets context. The next conversation resumes from `handoff.md`, enabling indefinite runtime across context boundaries.
