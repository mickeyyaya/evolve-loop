# Architecture

## Overview

Evolve Loop is a self-evolving development pipeline that orchestrates 4 specialized agents across 5 lean phases. The orchestrator (the main Claude Code session) coordinates agents via the Agent tool, managing data flow through a shared workspace and JSONL ledger.

## Design Principles

### 1. Fast Iteration
Build diverse small/medium tasks each cycle. Prefer 3 small tasks over 1 large task. Each task should be completable in a single Builder pass.

### 2. Agent Isolation
Each agent has a single responsibility and owns exactly one workspace file. Agents communicate through workspace files, not direct messaging.

### 3. Hard Gates
The Auditor gates on MEDIUM+ severity findings. Both code quality issues and eval check failures block shipping.

### 4. Continuous Learning
Each cycle extracts instincts (patterns) that future cycles can read. Instincts are specific and actionable, not generic advice. The system gets smarter over time.

### 5. Safe Autonomy
The Operator monitors for stalls, quality degradation, and repeated failures. It can HALT the loop, requiring human intervention.

### 6. No External Dependencies
All agents are self-contained. No dependency on external plugins or frameworks.

## Pipeline

```
Phase 1: DISCOVER ─── [Scout] scan + research + task selection
Phase 2: BUILD ────── [Builder] design + implement + self-test (worktree)
Phase 3: AUDIT ────── [Auditor] review + security + eval gate
Phase 4: SHIP ──────── orchestrator (commit + push)
Phase 5: LEARN ──────── orchestrator (instincts) + [Operator] (health check)
```

For multiple tasks, Phase 2-3 loop per task:
```
Scout → [Task A, Task B, Task C]
  → Builder(A) → Auditor(A) → commit
  → Builder(B) → Auditor(B) → commit
  → Builder(C) → Auditor(C) → commit
→ Ship → Learn
```

## Agents

| Agent | Purpose | Key Principle |
|-------|---------|---------------|
| Scout | Discovery + analysis + task selection | Incremental after cycle 1 |
| Builder | Design + implement + self-test | Minimal change, reversibility |
| Auditor | Review + security + eval + pipeline integrity | Single-pass, MEDIUM+ blocks |
| Operator | Loop health monitoring | Stall detection, HALT protocol |

## Shared Memory Architecture

### Layer 1: JSONL Ledger
Append-only structured log. Every agent appends one entry per invocation.

### Layer 2: Markdown Workspace
Human-readable files overwritten each cycle. Each agent reads upstream files and writes its own output.

### Layer 3: Persistent State
`state.json` persists across cycles: research cache (12hr TTL), task history, failed approaches, eval history, instinct count.

### Layer 4: Eval State
Eval definitions (created by Scout) and baseline results.

### Layer 5: Instincts
Extracted patterns from completed cycles. YAML files with confidence scoring that evolve over time.

## Token Optimization

The v4 architecture reduces token usage ~60-70% compared to v3:

| Component | v3 (11 agents) | v4 (4 agents) | Savings |
|-----------|----------------|---------------|---------|
| Discovery | ~170K (3 parallel) | ~40-60K (1 Scout) | ~65% |
| Build | ~70K (Architect+Developer) | ~30-50K (1 Builder) | ~45% |
| Verify | ~90K (3 parallel) | ~20-30K (1 Auditor) | ~70% |
| Ship+Learn | ~15K (Deployer+Operator) | ~5K (inline+Operator) | ~65% |
| **Total/cycle** | **~350K** | **~100-150K** | **~60%** |

Key optimizations:
- **Fewer handoffs** — no context loss between Architect→Developer or PM→Planner
- **Incremental scan** — cycle 2+ only scans what changed, not the whole codebase
- **12hr research cooldown** — web research reuses cached results
- **Single-pass audit** — one agent covers review, security, and eval
- **Inline ship** — orchestrator commits directly, no Deployer agent needed
