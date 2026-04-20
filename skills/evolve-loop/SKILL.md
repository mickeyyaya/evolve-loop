---
name: evolve-loop
description: Use when the user invokes /evolve-loop or asks to run autonomous improvement cycles, self-evolving development, compound discovery, or multi-cycle code improvement with research, build, audit, and learning phases
argument-hint: "[cycles] [strategy] [goal]"
---

# Evolve Loop v8.10

> Self-evolving development pipeline. Orchestrates 4 agents through 6 lean phases per cycle: Discover → Build → Audit → Ship → Learn → Meta-Cycle. This skill performs destructive operations (commits, pushes, version bumps) — only invoke when the user explicitly requests it via `/evolve-loop` or asks to run improvement cycles.

**Usage:** `/evolve-loop [cycles] [strategy] [goal]`

## Contents

- [Quick Start](#quick-start)
- [What Makes Evolve-Loop Different](#what-makes-evolve-loop-different)
- [Compound Discovery Loop](#compound-discovery-loop)
- [Architecture](#architecture)
- [Orchestrator Loop](#orchestrator-loop)
- [Agents](#agents)
- [Model Routing](#model-routing)
- [Reference (read on demand)](#reference-read-on-demand)

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
| `autoresearch` | Hypothesis testing | Fixed metrics, embraces failure | Divergent, unpenalized |

**Innovate strategy note:** "Additive" means adding functionality TO existing project files, not creating standalone documentation. Research findings must be translated into code/config changes. Max 1 doc-only task per cycle.

## What Makes Evolve-Loop Different

Unlike plan-mode or one-shot implementation skills:

| Capability | Evolve-Loop | Plan Mode / One-Shot |
|-----------|-------------|---------------------|
| **Compound discovery** | Each cycle generates hypotheses, discoveries, and proposals that feed the next cycle | One-shot: plan → execute → done |
| **Proactive insights** | Builder surfaces unsolicited findings ("Things Found Beyond Your Goal") | Only does what was asked |
| **Proactive research** | Every cycle starts with evaluation-driven research: gap analysis → web queries → concept cards → strict keep/drop verdicts via Research Ledger | Research only when asked or when knowledge gap encountered |
| **Research Ledger** | Strict WORKS/DOESN'T WORK verdicts on every research-driven change. Blocks known failures, boosts validated patterns, enforces diversity | No evaluation feedback on research quality |
| **Knowledge-complete convergence** | Stops when nothing new to learn (discovery velocity = 0), not just when tasks are done | Stops when tasks are done |
| **Cross-cycle memory** | Instincts persist, bandit learns, benchmark tracks, proposals compound | Starts fresh every time |
| **Self-verification** | Independent eval scripts, integrity gates, adaptive auditor | Trusts itself |
| **Meta-learning** | Meta-cycles improve prompts, mastery graduates complexity | Single pass |
| **Failure learning** | Failed approaches DB prevents repeating mistakes | No memory of failures |

## Compound Discovery Loop

Each cycle generates not just shipped code but new knowledge. The discovery loop compounds findings across cycles — this is what plan mode cannot do.

```
Scout (Hypothesize) → Builder (Discover) → Learn (Propose) → Scout (Select from proposals)
     ↑                                                              |
     └──────────────── proposals feed back as +1 priority ──────────┘
```

| Mechanism | Agent/Phase | How It Works |
|-----------|------------|-------------|
| **Research Loop** | Phase 1 | Orient → gap analysis → web research → concept cards → strict evaluate → keep/drop |
| **Research Ledger** | Phase 6 writes, Phase 1 reads | WORKS/DOESN'T WORK verdicts; blocks known failures; boosts validated patterns; enforces diversity |
| **Concept Cards** | Phase 1 → Scout | Research-backed implementation ideas scored on feasibility/impact/novelty; +2 priority boost |
| **Hypotheses** | Scout | Speculative improvements beyond gap-filling; confidence >= 0.7 auto-promote to task candidates |
| **Discoveries** | Builder | Latent bugs, smells, opportunities found during implementation; structured with category + severity |
| **Proposals** | Learn (Phase 6) | Discoveries + hypotheses converted to next-cycle candidates in `state.json.proposals` |
| **Discovery Briefing** | Orchestrator | End-of-cycle output: shipped tasks + discoveries + proposals queued + benchmark delta |
| **Discovery Velocity** | Learn (Phase 6) | Rolling 3-cycle proposals/cycle; loop continues while velocity > 0; converges when nothing new to learn |
| **Proactive Discovery** | Learn (Phase 6) | Builder insights beyond task scope tagged `unsolicited`; surfaced as "Things Found Beyond Your Goal" |

## Architecture

```
Phase 0:   CALIBRATE ─ benchmark (once per invocation) → phase0-calibrate.md
Phase 1: RESEARCH ── proactive research loop          → online-researcher.md
Utility:   SEARCH ─── intent-aware web search engine    → smart-web-search.md
Phase 2:   DISCOVER ── [Scout] scan + task selection    → phases.md
Phase 3:   BUILD ───── [Builder] implement (worktree)   → phase3-build.md
Phase 4:   AUDIT ───── [Auditor] review + eval gate     → phases.md
Phase 5:   SHIP ────── commit + push                    → phase5-ship.md
Phase 6:   LEARN ───── instinct extraction + feedback   → phase6-learn.md
Phase 7:   META ────── self-improvement (every 5 cycles) → phase7-meta.md
```

## Orchestrator Loop

For each cycle:
1. Claim cycle number (OCC protocol)
2. **`bash scripts/phase-gate.sh <gate> $CYCLE $WORKSPACE`** — MANDATORY at every phase transition
3. Scout → Builder → Auditor → phase-gate verification → Ship → Learn
4. Inline S-tasks directly; worktree M-tasks with `isolation: "worktree"`
5. Max 3 retries per task; WARN/FAIL blocks shipping
6. **After every agent dispatch** → check for rate limit signals (see [policies.md](reference/policies.md#rate-limit-recovery-protocol)). If hit → write handoff → auto-schedule resume via `/schedule` or `/loop` → STOP
7. **Context budget gates are checkpoints, not stops.** YELLOW → lean mode, continue. RED → write handoff, continue (auto-compaction frees space). Only STOP on two consecutive RED cycle starts.
8. Output Discovery Briefing (shipped tasks, discoveries, proposals queued, benchmark, discovery velocity) → continue immediately
9. **Never stop to ask. Never skip agents. Never fabricate cycles. Complete ALL requested cycles.**

## Agents

| Role | File | Tier | Output |
|------|------|------|--------|
| Scout | `agents/evolve-scout.md` | tier-2 | `scout-report.md` |
| Builder | `agents/evolve-builder.md` | tier-2 | `build-report.md` |
| Auditor | `agents/evolve-auditor.md` | tier-2 | `audit-report.md` |

Post-cycle health (fitness, brief generation, convergence check) is handled inline by the orchestrator in Phase 6 — no Operator agent needed.

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
| [phases.md](phases.md) | Phase sequencing, context blocks, gates (orchestration spine) |
| [phase0-calibrate.md](phase0-calibrate.md) | Once per invocation (benchmark) |
| [phase1-research.md](phase1-research.md) | Every cycle (gap analysis, web research, concept cards) |
| [phase2-discover.md](phase2-discover.md) | Every cycle (Scout launch, task claiming, eval checks) |
| [phase3-build.md](phase3-build.md) | Build orchestration, Self-MoA, parallelization |
| [phase5-ship.md](phase5-ship.md) | Commit, push, process rewards |
| [phase6-learn.md](phase6-learn.md) | Instinct extraction, consolidation |
| [phase7-meta.md](phase7-meta.md) | Every 5 cycles (critique, prompt evolution) |

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
| [reference/policies.md](reference/policies.md) | Instincts, plan caching, token budgets, context management, rate limit recovery |
| [reference/safety.md](reference/safety.md) | Phase gate, anti-patterns, known incidents |
| [reference/task-selection.md](reference/task-selection.md) | Bandit mechanism, novelty, adaptive strictness |
| [agents/agent-templates.md](../../agents/agent-templates.md) | Shared agent schemas, budget awareness |
