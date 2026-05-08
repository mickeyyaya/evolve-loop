---
name: reference
description: Reference doc.
---

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
| Per-cycle | 35K tokens (normal) / 20K (lean) | Per-cycle budget gate |
| Research phase | 25K tokens | Phase 1 terminates iteration if exceeded |
| M-task + 10+ files | Likely exceeds budget | Split required |

**Note:** There is no per-session cumulative budget. Each cycle is an independent plan-mode unit. Auto-compaction reclaims context from older cycles. The budget gate checks "does ONE more cycle fit?" not "how much total have we used?"

## Context Window Strategy

Each cycle is an **independent unit** — it has its own goal, runs its own agents (in isolated subagent context), and persists all results to files. Between cycles, Claude Code auto-compresses older conversation turns. The only accumulated context is:
- Static overhead (system prompt, rules): ~25K — always present
- Recent orchestrator conversation (~1-2 cycles): ~50K — not yet compacted
- Residual growth (~3K/cycle): compressed summaries, handoff fragments

This means effective context usage is **roughly constant** regardless of cycle count, with only slow residual growth. A 10-cycle and a 3-cycle session use similar amounts of live context.

### Context Budget Check (mandatory at cycle start)

Run `scripts/verification/context-budget.sh` at the start of every cycle. It answers: "Is there room for one more cycle?"

| Status | Exit Code | Trigger | Action |
|--------|-----------|---------|--------|
| **GREEN** | 0 | Cycles 1-9 (default) | Continue normally — full per-cycle budget available |
| **YELLOW** | 1 | Cycle 10+ OR headroom tight | Lean mode for this cycle. **Continue — YELLOW is NOT a stop signal.** |
| **RED** | 2 | Cycle 30+ OR headroom < one lean cycle | Write handoff checkpoint. Only STOP on two consecutive RED cycle starts. |

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

### Cycles per Session

With the per-cycle budget model, cycle count is limited by residual growth (3K/cycle), not cumulative cost:

| Mode | Per-Cycle Cost | GREEN Until | YELLOW Until | RED At |
|------|---------------|-------------|-------------|--------|
| Normal | ~35K | Cycle 10 | — | — |
| Lean (auto from cycle 10) | ~20K | — | Cycle 30 | Cycle 30+ |

**No practical limit for 10-cycle requests.** Even 20+ cycles complete in YELLOW (lean mode, no stop). RED is a safety valve at cycle 30+ for edge cases.

### Auto-Compaction + Handoff Hybrid

Claude Code automatically compresses older conversation turns between cycles. The evolve-loop leverages this by persisting ALL critical state in files (state.json, reports, evals, ledger) — conversation history is supplementary orchestration context, not the source of truth.

Each cycle is independent: plan → implement → audit → ship → learn → done. The next cycle starts with a nearly fresh context window (static overhead + recent turns + residual growth).

Handoff files (`handoff.md`) are written at every cycle checkpoint as insurance. But compaction is the primary mechanism for long sessions — session breaks are a **LAST RESORT** triggered only at RED (cycle 30+ or headroom exhausted) after two consecutive RED confirmations.

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
