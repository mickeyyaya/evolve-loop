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
| Per-session | 300K tokens (30%) | Session break triggered |
| M-task + 10+ files | Likely exceeds budget | Split required |

## Context Window Strategy

Research basis: LLM performance degrades continuously as context fills. Effective context is 25-50% of theoretical max (arXiv:2410.18745). Degradation visible at 25% capacity (Chroma "Context Rot", 2025). "Lost in the Middle" (Stanford 2023) shows 20%+ accuracy drop for mid-context information.

**Target: keep context usage at 20-30% of window (200-300K of 1M).**

### Context Budget Check (mandatory at cycle start)

Run `scripts/context-budget.sh` at the start of every cycle:

```bash
BUDGET_JSON=$(bash scripts/context-budget.sh "$CYCLE_NUMBER" "$CYCLES_THIS_SESSION" "$WORKSPACE_PATH")
BUDGET_STATUS=$(echo "$BUDGET_JSON" | grep -o '"status": *"[^"]*"' | cut -d'"' -f4)
```

| Status | Exit Code | Action |
|--------|-----------|--------|
| **GREEN** (< 20%) | 0 | Continue normally |
| **YELLOW** (20-30%) | 1 | Activate lean mode immediately. Complete current cycle, then evaluate session break |
| **RED** (> 30%) | 2 | **Session break required.** Finish current phase, write handoff, start new session |

### Session Break Protocol

When RED is signaled, the orchestrator MUST:

1. **Complete the current phase** (never break mid-phase)
2. **Write state.json** with current version (OCC protocol)
3. **Write session-break handoff** to `$WORKSPACE_PATH/handoff.md` AND `.evolve/workspace/handoff.md` using the template below
4. **Output resume instructions** to the user:
   ```
   Context budget reached 30%. Session break for optimal performance.
   Resume: /evolve-loop <N> [strategy] [goal]
   ```
5. **STOP** — do not start the next cycle in this session

### Session Break Handoff Template (MANDATORY format)

This template ensures zero information loss across session boundaries. Every field is required.

```markdown
# Session Break Handoff — Cycle <N>

## Resume Command
`/evolve-loop <remaining_cycles> <strategy> <goal or "autonomous">`

## Why Session Broke
- Context budget status: RED (<estimated_tokens> tokens, <percent>% of 1M)
- Cycles completed this session: <N>
- Phase completed before break: <DISCOVER|BUILD|AUDIT|SHIP|LEARN>

## Session State (snapshot of state.json at break time)
- state.json version: <V>
- lastCycleNumber: <N>
- strategy: <balanced|innovate|harden|repair|ultrathink>
- goal: <goal string or "null (autonomous discovery mode)">
- mastery: <level> (<N> consecutive successes)
- nothingToDoCount: <0|1|2>
- fitnessScore: <score>
- fitnessRegression: <true|false>

## Benchmark Status
- Overall: <score>/100 (calibrated at cycle <N>, <timestamp>)
- Weakest dimensions: <dim1> (<score>), <dim2> (<score>)
- High-water mark regressions: <list or "none">

## Task Queue
### Selected (claimed but not yet completed this session)
- <task-slug>: <brief description, current phase status>

### Deferred (with reasons and prerequisites)
- <task-slug>: <reason>, revisit after <date or condition>

### Recently Completed (this session)
- <task-slug> (cycle <N>): <verdict>, <1-line summary>

## Failed Approaches (CRITICAL — avoid repeating)
- <feature>: tried <approach>, failed because <error> (category: <planning|tool-use|reasoning|context|integration>). Alternative: <suggestion>

## Active Instincts (top 5 by confidence)
- inst-<NNN> (<confidence>): <pattern summary>

## Research State
- Active cooldowns: <N> queries, expires at <ISO-8601>
- Last research: <topic> at <timestamp>

## Recent Cycle Verdicts (last 3)
| Cycle | Tasks | Verdict | Instincts | Notes |
|-------|-------|---------|-----------|-------|
| <N>   | <count> | <PASS/WARN/FAIL> | <count> | <1-line> |

## Carry Forward (context not derivable from disk)
- <any orchestrator observations, strategy reasoning, or pattern notes that exist only in session memory>
```

### What Survives vs What Resets on Resume

| Category | Survives? | Source | Notes |
|----------|-----------|--------|-------|
| Cycle numbers, task decisions | YES | state.json | Direct read |
| Strategy, goal | YES | handoff.md + state.json | Goal only in handoff if user-provided |
| Remaining cycles | YES | handoff.md resume command | Not in state.json — **handoff is the only source** |
| Benchmark scores | YES | state.json | Skip Phase 0 if < 24hr old |
| Instincts + confidence | YES | state.json + .evolve/instincts/ | Direct read |
| Failed approaches | YES | state.json | Scout reads to avoid repeating |
| Eval definitions | YES | .evolve/evals/ | Checksummed |
| Research cooldowns | YES | state.json.research | TTL-based expiry |
| Auditor strictness | YES | state.json | **Decayed 50% on new invocation** (by design) |
| CYCLES_THIS_SESSION | RESETS to 0 | Fresh session | Correct — new session = fresh context window |
| Budget pressure / lean mode | RESETS | Re-inferred | context-budget.sh re-evaluates from 0 |
| Challenge token | RESETS | Regenerated | New token per cycle (by design) |
| Orchestrator reasoning | PARTIAL | handoff.md "Carry Forward" | **Write anything non-obvious here** |

### Cycle-per-Session Estimates

| Mode | Tokens/Cycle | Cycles Before RED |
|------|-------------|-------------------|
| Normal (cycles 1-3) | ~62K | ~3 cycles |
| Lean (cycles 4+) | ~42K | ~4-5 cycles |
| Inline S-tasks only | ~27K | ~7-8 cycles |

**Rule of thumb: plan for 3-4 cycles per session.** Request `/evolve-loop 3` for optimal context utilization.

### Why Not Auto-Compact?

Claude Code's auto-compaction is lossy and unpredictable — it discards context at arbitrary points, potentially losing critical cycle state (eval checksums, challenge tokens, failed approaches). Deliberate session breaks with structured handoffs preserve 100% of decision-relevant context while resetting the noise floor.

## Context Management

- After each cycle → write `handoff.md` checkpoint + 5-line summary
- **Continue immediately** — never stop, never ask, never fabricate (unless RED)
- **Lean mode** (cycle 4+ OR budget pressure high OR YELLOW):
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
