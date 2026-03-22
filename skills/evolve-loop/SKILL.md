---
name: evolve-loop
description: "Platform-agnostic self-evolving development pipeline — 4 agents across 5 phases. Works with Claude Code, Gemini CLI, or any LLM with file I/O and shell access."
argument-hint: "[cycles] [strategy] [goal]"
disable-model-invocation: true
---

# Evolve Loop v7.9

4 agents, 5 phases, fast iteration. Discover → Build → Audit → Ship → Learn.

**Usage:** `/evolve-loop [cycles] [strategy] [goal]`

## Argument Parsing

- First token is a number → `cycles` (default: 2)
- Token matches `innovate|harden|repair|ultrathink` → `strategy` (default: `balanced`)
- Remaining tokens → `goal` (default: null = autonomous discovery)

| Strategy | Focus | Approach | Strictness |
|----------|-------|----------|------------|
| `balanced` | Broad discovery | Standard | MEDIUM+ blocks |
| `innovate` | New features, gaps | Additive, new files | Relaxed style, strict correctness |
| `harden` | Stability, tests | Defensive coding | Strict all |
| `repair` | Bugs, broken tests | Fix-only, smallest diff | Strict regressions |
| `ultrathink` | Complex refactors | tier-1 forced, stepwise | Strict all + confidence checks |

## Architecture

```
Phase 0: CALIBRATE ── benchmark scoring (once per invocation)        → phase0-calibrate.md
Phase 1: DISCOVER ─── [Scout] scan + research + task selection       → phases.md
Phase 2: BUILD ────── [Builder] implement + self-test (worktree)     → phase2-build.md
Phase 3: AUDIT ────── [Auditor] review + eval gate                   → phases.md
Phase 4: SHIP ──────── commit + push                                 → phase4-ship.md
Phase 5: LEARN ──────── instinct extraction + operator               → phase5-learn.md
Phase 6: META-CYCLE ── self-improvement (every 5 cycles)             → phase6-metacycle.md
```

Multi-task: partition by file overlap → independent tasks build in parallel worktrees.

## Orchestrator Loop

For each cycle:
1. Claim cycle number (OCC protocol)
2. **`bash scripts/phase-gate.sh discover-to-build $CYCLE $WORKSPACE`** — MANDATORY at every gate
3. Scout → Builder → Auditor → phase-gate verification → Ship → Learn
4. Inline S-tasks directly (per inst-007); worktree M-tasks with `isolation: "worktree"`
5. Max 3 retry attempts per task; Auditor WARN/FAIL blocks shipping
6. Output cycle summary (5-8 lines) → continue immediately to next cycle
7. **Never stop to ask.** Never skip agents. Never fabricate cycles. Maximum velocity, zero shortcuts.

## Agents

| Role | File | Tier | Output |
|------|------|------|--------|
| Scout | `agents/evolve-scout.md` | tier-2 | `scout-report.md` |
| Builder | `agents/evolve-builder.md` | tier-2 | `build-report.md` |
| Auditor | `agents/evolve-auditor.md` | tier-2 | `audit-report.md` |
| Operator | `agents/evolve-operator.md` | tier-3 | `operator-log.md` |

## Model Routing (quick reference)

| Phase | Default | Upgrade → | Downgrade → |
|-------|---------|-----------|-------------|
| Scout | tier-2 | Cycle 1 / goal-directed → tier-1 | Cycle 4+ → tier-3 |
| Builder | tier-2 | M+5 files / retry ≥ 2 → tier-1 | S + cache → tier-3 |
| Auditor | tier-2 | Security-sensitive → tier-1 | Clean build → tier-3 |
| Operator | tier-3 | Last cycle / regression → tier-2 | — |

Full routing rules: [docs/model-routing.md](../../docs/model-routing.md)

## Reference Files (read on demand, not loaded at start)

| File | When to read |
|------|-------------|
| [phases.md](phases.md) | Phase sequencing, context blocks, phase gates |
| [phase0-calibrate.md](phase0-calibrate.md) | Once per invocation (benchmark scoring) |
| [phase2-build.md](phase2-build.md) | Build orchestration, parallelization, Self-MoA |
| [phase4-ship.md](phase4-ship.md) | Commit, push, process rewards |
| [phase5-learn.md](phase5-learn.md) | Instinct extraction, consolidation |
| [phase6-metacycle.md](phase6-metacycle.md) | Every 5 cycles (split-role critique, prompt evolution) |
| [memory-protocol.md](memory-protocol.md) | State.json schema, OCC protocol, ledger format |
| [eval-runner.md](eval-runner.md) | Eval gate, grader types, retry protocol |
| [benchmark-eval.md](benchmark-eval.md) | 8-dimension scoring rubric |
| [resources/initialization.md](resources/initialization.md) | Session init, directories, domain detection |
| [resources/policies.md](resources/policies.md) | Graduated instincts, plan caching, token budgets |
| [resources/safety.md](resources/safety.md) | Phase gate, tamper detection, anti-patterns, known incidents |
| [resources/task-selection.md](resources/task-selection.md) | Bandit mechanism, novelty bonus, adaptive strictness |
| [agents/agent-templates.md](../../agents/agent-templates.md) | Shared input/output schemas, budget awareness |
