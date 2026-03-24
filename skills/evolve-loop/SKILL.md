---
name: evolve-loop
description: "Self-evolving development pipeline with 4 agents across 6 phases. Discovers, builds, audits, and ships improvements autonomously. Works with Claude Code, Gemini CLI, or any LLM with file I/O and shell access. Use when the user invokes /evolve-loop or asks to run autonomous improvement cycles."
argument-hint: "[cycles] [strategy] [goal]"
disable-model-invocation: true
---

# Evolve Loop v8.0

Orchestrates 4 agents through 6 lean phases per cycle: Discover → Build → Audit → Ship → Learn → Meta-Cycle.

**Usage:** `/evolve-loop [cycles] [strategy] [goal]`

## Quick Start

Parse `$ARGUMENTS`:
- First number → `cycles` (default: 2)
- `innovate|harden|repair|ultrathink` → `strategy` (default: `balanced`)
- Remaining → `goal` (default: null = autonomous)

| Strategy | Focus | Approach | Strictness |
|----------|-------|----------|------------|
| `balanced` | Broad discovery | Standard | MEDIUM+ blocks |
| `innovate` | New features, gaps | Additive — implement into existing files | Relaxed style |
| `harden` | Stability, tests | Defensive | Strict all |
| `repair` | Bugs, broken tests | Fix-only, smallest diff | Strict regressions |
| `ultrathink` | Complex refactors | tier-1 forced | Strict + confidence |

**Innovate strategy note:** "Additive" means adding functionality TO existing project files, not creating standalone documentation. Research findings must be translated into code/config changes. Max 1 doc-only task per cycle.

## What Makes Evolve-Loop Different

Unlike plan-mode or one-shot implementation skills:

| Capability | Evolve-Loop | Plan Mode / One-Shot |
|-----------|-------------|---------------------|
| Cross-cycle memory | Instincts persist, bandit learns, benchmark tracks | Starts fresh every time |
| Self-verification | Independent eval scripts, integrity gates, adaptive auditor | Trusts itself |
| Continuous depth | Meta-cycles improve prompts, mastery graduates complexity | Single pass |
| Research→Implementation | Findings become code changes, not just docs | Research is separate |
| Failure learning | Failed approaches DB prevents repeating mistakes | No memory of failures |
| Quality regression prevention | 8D benchmark blocks regressions before shipping | No tracking |

## Architecture

```
Phase 0: CALIBRATE ─ benchmark (once per invocation)  → phase0-calibrate.md
Phase 1: DISCOVER ── [Scout] scan + task selection     → phases.md
Phase 2: BUILD ───── [Builder] implement (worktree)    → phase2-build.md
Phase 3: AUDIT ───── [Auditor] review + eval gate      → phases.md
Phase 4: SHIP ────── commit + push                     → phase4-ship.md
Phase 5: LEARN ───── instinct extraction + operator    → phase5-learn.md
Phase 6: META ────── self-improvement (every 5 cycles) → phase6-metacycle.md
```

## Orchestrator Loop

For each cycle:
1. Claim cycle number (OCC protocol)
2. **`bash scripts/phase-gate.sh <gate> $CYCLE $WORKSPACE`** — MANDATORY at every phase transition
3. Scout → Builder → Auditor → phase-gate verification → Ship → Learn
4. Inline S-tasks directly; worktree M-tasks with `isolation: "worktree"`
5. Max 3 retries per task; WARN/FAIL blocks shipping
6. Output 5-line cycle summary → continue immediately
7. **Never stop to ask. Never skip agents. Never fabricate cycles.**

## Agents

| Role | File | Tier | Output |
|------|------|------|--------|
| Scout | `agents/evolve-scout.md` | tier-2 | `scout-report.md` |
| Builder | `agents/evolve-builder.md` | tier-2 | `build-report.md` |
| Auditor | `agents/evolve-auditor.md` | tier-2 | `audit-report.md` |

Post-cycle health (fitness, brief generation, convergence check) is handled inline by the orchestrator in Phase 5 — no Operator agent needed.

## Model Routing

| Phase | Default | Upgrade → | Downgrade → |
|-------|---------|-----------|-------------|
| Scout | tier-2 | Cycle 1 / goal → tier-1 | Cycle 4+ → tier-3 |
| Builder | tier-2 | M+5 files / retry ≥ 2 → tier-1 | S + cache → tier-3 |
| Auditor | tier-2 | Security → tier-1 | Clean → tier-3 |

Full rules: [reference/model-routing.md](../../docs/model-routing.md)

## Reference (read on demand)

### Phase Instructions
| File | When to read |
|------|-------------|
| [phases.md](phases.md) | Phase sequencing, context blocks, gates |
| [phase0-calibrate.md](phase0-calibrate.md) | Once per invocation (benchmark) |
| [phase2-build.md](phase2-build.md) | Build orchestration, Self-MoA, parallelization |
| [phase4-ship.md](phase4-ship.md) | Commit, push, process rewards |
| [phase5-learn.md](phase5-learn.md) | Instinct extraction, consolidation |
| [phase6-metacycle.md](phase6-metacycle.md) | Every 5 cycles (critique, prompt evolution) |

### Configuration & Protocols
| File | When to read |
|------|-------------|
| [memory-protocol.md](memory-protocol.md) | State.json schema, OCC, ledger format |
| [eval-runner.md](eval-runner.md) | Eval gate, grader types, retry protocol |
| [benchmark-eval.md](benchmark-eval.md) | 8-dimension scoring rubric |
| [reference/initialization.md](reference/initialization.md) | Session setup, directories, domain detection |

### Policies & Safety
| File | When to read |
|------|-------------|
| [reference/policies.md](reference/policies.md) | Instincts, plan caching, token budgets, context management |
| [reference/safety.md](reference/safety.md) | Phase gate, anti-patterns, known incidents |
| [reference/task-selection.md](reference/task-selection.md) | Bandit mechanism, novelty, adaptive strictness |
| [agents/agent-templates.md](../../agents/agent-templates.md) | Shared agent schemas, budget awareness |
