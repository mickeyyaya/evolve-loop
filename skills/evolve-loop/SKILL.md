---
name: evolve-loop
description: "Self-evolving development pipeline — 4 specialized agents (Scout, Builder, Auditor, Operator) across 5 phases. Build diverse small/medium tasks each cycle, iterate fast with quality gates. No external dependencies."
argument-hint: "[cycles] [goal]"
disable-model-invocation: true
---

# Evolve Loop v4

Orchestrates 4 specialized agents through 5 lean phases per cycle. Optimized for fast iteration: discover → build → audit → ship → learn. Each cycle targets 2-4 small/medium tasks, builds them in isolated worktrees, and gates on MEDIUM+ audit findings.

**Usage:** `/evolve-loop [cycles] [goal]`

## Argument Parsing

Parse `$ARGUMENTS` as follows:
- If the first token is a number → use it as `cycles`, remainder is `goal`
- If the first token is NOT a number → `cycles` defaults to 2, entire input is `goal`
- If empty → `cycles` = 2, `goal` = null (autonomous discovery mode)

Examples:
- `/evolve-loop` → cycles=2, goal=null
- `/evolve-loop 3` → cycles=3, goal=null
- `/evolve-loop 1 add dark mode support` → cycles=1, goal="add dark mode support"
- `/evolve-loop add user authentication` → cycles=2, goal="add user authentication"

## Goal Modes

**With goal (directed mode):** Scout focuses discovery and task selection on advancing the goal. Builder implements goal-relevant tasks. Auditor checks goal alignment.

**Without goal (autonomous mode):** Scout performs broad discovery, picks highest-impact work across all dimensions.

## Architecture

```
Phase 1:   DISCOVER ─── sequential ─── [Scout] scan + research + task selection
Phase 2:   BUILD ────── sequential ─── [Builder] design + implement + self-test (worktree)
Phase 3:   AUDIT ────── sequential ─── [Auditor] review + security + eval gate
Phase 4:   SHIP ──────── orchestrator ── commit + push (inline, no agent)
Phase 5:   LEARN ──────── orchestrator ── archive + instinct extraction + operator check
```

For multiple tasks per cycle, Phase 2-3 loop:
```
Scout → [Task A, Task B, Task C]
  → Builder(A) → Auditor(A) → commit
  → Builder(B) → Auditor(B) → commit
  → Builder(C) → Auditor(C) → commit
→ Ship → Learn
```

## Initialization (once per session)

1. Ensure directories exist:
   ```bash
   mkdir -p .claude/evolve/workspace .claude/evolve/history .claude/evolve/evals .claude/evolve/instincts/personal
   ```

2. Read `.claude/evolve/state.json` if it exists. If not, initialize:
   ```json
   {"lastUpdated":"<now>","research":{"queries":[]},"evaluatedTasks":[],"failedApproaches":[],"evalHistory":[],"instinctCount":0,"operatorWarnings":[],"nothingToDoCount":0}
   ```

3. Auto-detect project context (language, framework, test commands, domain). Store as `projectContext`.

4. **Pre-flight check** (inline, no agent):
   ```bash
   git status --porcelain   # must be clean
   git worktree list        # worktree support available
   ls .claude/evolve/evals/ 2>/dev/null  # evals exist (skip check on cycle 1)
   ```
   If git is dirty, warn user before proceeding.

## Orchestrator Loop

You are the orchestrator. For each cycle:
1. Launch Scout → collect task list
2. For each task: Launch Builder (worktree) → Launch Auditor
3. If Auditor PASS → commit. If WARN/FAIL → re-run Builder with issues (max 3 attempts)
4. Ship all passing commits
5. Learn: archive, extract instincts, operator check

For detailed phase-by-phase instructions, see [phases.md](phases.md).
For the shared memory protocol, see [memory-protocol.md](memory-protocol.md).
For the eval hard gate instructions, see [eval-runner.md](eval-runner.md).

## Agent Definitions

All agents are custom, self-contained. No external dependencies.

| Role | Agent File | Model | Workspace File |
|------|-----------|-------|----------------|
| Scout | `evolve-scout.md` | sonnet | `scout-report.md` |
| Builder | `evolve-builder.md` | sonnet | `build-report.md` |
| Auditor | `evolve-auditor.md` | sonnet | `audit-report.md` |
| Operator | `evolve-operator.md` | sonnet | `operator-log.md` |

**Eval Runner** — orchestrator-executed (not an agent), instructions in [eval-runner.md](eval-runner.md).

## Anti-Patterns

1. **Over-discovery** — Scout should be incremental after cycle 1, not full audit every time
2. **Big tasks** — Prefer 3 small tasks over 1 large task. Each should be <50K tokens to build
3. **Retrying the same failure** — Log in state.json, try alternative next cycle
4. **Skipping the audit** — Auditor verdict of WARN or FAIL blocks shipping
5. **Ignoring instincts** — Builder MUST read instincts when available
6. **Research every cycle** — 12hr cooldown on web research. Reuse cached results
7. **Ceremony over substance** — Workspace files should be concise, not exhaustive
8. **Ignoring HALT** — When Operator returns HALT, pause and present to user
