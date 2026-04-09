# Orchestrator Policies

> Read this file when making decisions about inline execution, caching, budgets, or context management.

## Graduated Instincts

Patterns confirmed at confidence 0.9+ become mandatory behavior:

| Policy | Rule | Savings |
|--------|------|---------|
| **Inline S-tasks** (inst-007) | S-complexity, <10 lines, clear eval → implement inline, skip Builder | ~30-50K tokens |
| **Grep-based evals** (inst-004) | Markdown/shell projects → grep commands with match counts | — |
| **Meta-cycle** | Every 5 cycles → split-role critique + prompt evolution | [phase7-meta.md](../phase7-meta.md) |
| **Gene library** | Reusable fix templates in `.evolve/genes/` | [docs/genes.md](../../../docs/genes.md) |

## Plan Reuse

Builder reads prior successful build-reports from `.evolve/history/` when facing a similar task type. No separate cache needed — the history directory IS the plan archive.

## Token Budgets

| Scope | Limit | Enforcement |
|-------|-------|-------------|
| Per-task | 80K tokens | Scout breaks tasks exceeding this |
| Per-cycle | 200K tokens | Orchestrator halts if exceeded |
| Per-session | 300K tokens (30%) | Session break triggered |
| Research phase | 25K tokens | Phase 1 terminates iteration if exceeded |
| M-task + 10+ files | Likely exceeds budget | Split required |

## Context Window Strategy

Research basis: LLM performance degrades continuously as context fills. Effective context is 25-50% of theoretical max (arXiv:2410.18745). "Lost in the Middle" (Stanford 2023) shows 20%+ accuracy drop for mid-context information.

**Target: keep context usage at 20-30% of window (200-300K of 1M).**

### Context Budget Check (mandatory at cycle start)

Run `scripts/context-budget.sh` at the start of every cycle:

| Status | Exit Code | Action |
|--------|-----------|--------|
| **GREEN** (< 20%) | 0 | Continue normally |
| **YELLOW** (20-30%) | 1 | Activate lean mode. Complete current cycle, then evaluate session break |
| **RED** (> 30%) | 2 | **Session break required.** Finish current phase, write handoff, STOP |

### Session Break Protocol

When RED is signaled:
1. **Complete current phase** (never break mid-phase)
2. **Write state.json** with current version (OCC protocol)
3. **Write handoff** to `$WORKSPACE_PATH/handoff.md` AND `.evolve/workspace/handoff.md` (see template below)
4. **Output resume instructions:** `Resume: /evolve-loop <N> [strategy] [goal]`
5. **STOP** — do not start the next cycle

### Session Break Handoff Template

Required sections (all mandatory):

```markdown
# Session Break Handoff — Cycle <N>

## Resume Command
`/evolve-loop <remaining_cycles> <strategy> <goal or "autonomous">`

## Why Session Broke
- Context budget status / API rate limit / cause
- Cycles completed this session: <N>
- Phase completed before break: <phase>

## Session State
- state.json version, lastCycleNumber, strategy, goal
- mastery level + consecutive successes
- nothingToDoCount, fitnessScore, fitnessRegression

## Benchmark Status
- Overall score, weakest dimensions, high-water mark regressions

## Task Queue
- Selected (claimed, not completed), Deferred (with reasons), Recently Completed

## Failed Approaches
- Feature, approach tried, error, category, alternative

## Active Instincts (top 5 by confidence)
## Research State (cooldowns, last research topic)

## Recent Cycle Verdicts (last 3)
| Cycle | Tasks | Verdict | Notes |

## Carry Forward (context not derivable from disk)
- Orchestrator observations, strategy reasoning, pattern notes from session memory
- If rate-limit triggered: include trigger ID for cleanup
```

### What Survives vs Resets on Resume

| Survives | Source | Resets | Reason |
|----------|--------|--------|--------|
| Cycle numbers, task decisions | state.json | CYCLES_THIS_SESSION | New session = fresh context |
| Strategy, goal | handoff.md | Budget pressure / lean mode | Re-inferred by context-budget.sh |
| Remaining cycles | handoff.md only | Challenge token | Regenerated per cycle |
| Benchmark, instincts, failed approaches | state.json | Orchestrator reasoning | Partial via "Carry Forward" |
| Eval definitions | .evolve/evals/ (checksummed) | Auditor strictness | Decayed 50% on new invocation |

### Cycle-per-Session Estimates

| Mode | Tokens/Cycle | Cycles Before RED |
|------|-------------|-------------------|
| Normal (cycles 1-3) | ~62K | ~3 cycles |
| Lean (cycles 4+) | ~42K | ~4-5 cycles |
| Inline S-tasks only | ~27K | ~7-8 cycles |

**Rule of thumb: plan for 3-4 cycles per session.**

### Why Not Auto-Compact?

Claude Code's auto-compaction is lossy and unpredictable — it discards context at arbitrary points, potentially losing critical cycle state (eval checksums, challenge tokens, failed approaches). Deliberate session breaks with structured handoffs preserve 100% of decision-relevant context.

## Rate Limit Recovery Protocol

API rate limits are a hard external wall — unlike context budget (internal quality), rate limits stop execution entirely. The orchestrator MUST detect and auto-resume.

### Detection

After every agent dispatch, check for rate limit signals:

| Signal | Detection |
|--------|-----------|
| Error with "rate limit", "quota", "overloaded", "429" | Check agent return value |
| 3+ consecutive agent failures | Track `CONSECUTIVE_FAILURES` counter |
| Agent timeout without output | Possible silent throttle |

### Recovery Steps

When rate limit detected:
1. Complete current phase (never break mid-phase)
2. Write handoff using Session Break Handoff Template (cause: "API rate limit")
3. Auto-schedule resume (priority order below)
4. STOP

### Auto-Resumption Methods

| Priority | Method | When | Command |
|----------|--------|------|---------|
| 1 | `/schedule` (remote trigger) | Reset window >= 1 hour | One-time trigger at next hour mark |
| 2 | `/loop` (local retry) | Short limits, user present | `/loop 5m /evolve-loop <remaining> <strategy> <goal>` |
| 3 | Manual resume (fallback) | Scheduling unavailable | Output resume command for user |

### Rate Limit vs Context Budget

| Concern | Context Budget | Rate Limit |
|---------|---------------|------------|
| Type | Internal quality | External hard wall |
| Detection | Proactive (estimated) | Reactive (error-based) |
| Recovery | User runs `/evolve-loop` | **Auto-scheduled** via `/schedule` or `/loop` |
| Reset time | Immediate (new session) | Provider-dependent (minutes to hours) |

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
- Compound Discoveries (cross-cycle patterns, emergent themes, compounding proposals)
- Learning stats (instincts, mastery)
- Warnings and next strategy recommendation
