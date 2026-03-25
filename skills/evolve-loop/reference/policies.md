# Orchestrator Policies

> Read this file when making decisions about inline execution, caching, budgets, or context management.

## Graduated Instincts

Patterns confirmed at confidence 0.9+ become mandatory behavior:

| Policy | Rule | Savings |
|--------|------|---------|
| **Inline S-tasks** (inst-007) | S-complexity, <10 lines, clear eval → implement inline, skip Builder | ~30-50K tokens |
| **Grep-based evals** (inst-004) | Markdown/shell projects → grep commands with match counts | — |
| **Meta-cycle** | Every 5 cycles → split-role critique + prompt evolution | [phase6-metacycle.md](../phase6-metacycle.md) |
| **Gene library** | Reusable fix templates in `.evolve/genes/` | [docs/genes.md](../../../docs/genes.md) |

## Plan Reuse

Builder reads prior successful build-reports from `.evolve/history/` when facing a similar task type. No separate cache needed — the history directory IS the plan archive.

## Token Budgets

| Scope | Limit | Enforcement |
|-------|-------|-------------|
| Per-task | 80K tokens | Scout breaks tasks exceeding this |
| Per-cycle | 200K tokens | Orchestrator halts if exceeded |
| M-task + 10+ files | Likely exceeds budget | Split required |

## Context Management

- After each cycle → write `handoff.md` checkpoint + 5-line summary
- **Continue immediately** — never stop, never ask, never fabricate
- **Lean mode** (cycle 4+ OR budget pressure high):
  - Read state.json once at cycle start
  - Use agent return summaries, not full workspace files
  - Skip redundant file re-reads
- **AgentDiet compression** between phases: prune expired context at each boundary

## Final Session Report

After all cycles → generate `final-report.md`:
- Summary narrative (3-4 sentences)
- Task table (cycle, slug, type, verdict, attempts)
- Benchmark trajectory (per-dimension start/end/delta)
- Compound Discoveries (cross-cycle patterns: discoveries that built on each other, emergent themes, proposals that compound across cycles)
- Learning stats (instincts, mastery)
- Warnings and next strategy recommendation

## Session Break Handoff Template

When a session ends mid-run, write handoff with these sections in order:
1. **Session State** — current cycle, remaining cycles, active strategy
2. **Recent Cycle Verdicts** — last 3 cycles with task slugs and verdicts
3. **Unsolicited Insights** — aggregate all proposals tagged `"unsolicited": true` across cycles; present as "Things Found Beyond Your Goal" for user review
4. **Carry Forward** — pending tasks, deferred items, unresolved blockers
