---
name: evolve-loop
description: "Self-evolving development pipeline — 4 specialized agents (Scout, Builder, Auditor, Operator) across 5 phases. Build diverse small/medium tasks each cycle, iterate fast with quality gates. No external dependencies."
argument-hint: "[cycles] [strategy] [goal]"
disable-model-invocation: true
---

# Evolve Loop v6

Orchestrates 4 specialized agents through 5 lean phases per cycle. Optimized for fast iteration: discover → build → audit → ship → learn. Each cycle targets 2-4 small/medium tasks, builds them in isolated worktrees, and gates on MEDIUM+ audit findings.

**Usage:** `/evolve-loop [cycles] [strategy] [goal]`

## Argument Parsing

Parse `$ARGUMENTS` as follows:
- If the first token is a number → use it as `cycles` (number of NEW cycles to run)
- If a token matches a strategy name (`innovate`, `harden`, `repair`) → use it as `strategy`
- Remaining tokens → `goal`
- Defaults: `cycles` = 2, `strategy` = `balanced`, `goal` = null (autonomous discovery mode)

### Strategy Presets

Strategies steer cycle intent without requiring a full goal string:

| Strategy | Scout Focus | Builder Approach | Auditor Strictness |
|----------|-------------|------------------|-------------------|
| `balanced` | Broad discovery, mixed task types | Standard minimal-change | Normal (MEDIUM+ blocks) |
| `innovate` | New features, missing functionality, gaps | Prefer additive changes, new files | Relaxed on style, strict on correctness |
| `harden` | Stability, test coverage, error handling, edge cases | Defensive coding, add tests/validation | Strict on all dimensions |
| `repair` | Bugs, broken functionality, failing tests | Fix-only, smallest possible diff | Strict on regressions, relaxed on new code |

The strategy is passed to all agents via the context block as `"strategy": "<name>"`. Each agent adapts its behavior accordingly (see agent definitions for details).

**Strategy + Goal interaction:** When both are provided, the strategy sets the *approach style* while the goal sets the *direction*. Example: `/evolve-loop 3 harden add payment processing` → build payment features with defensive coding and strict auditing.

The `cycles` argument is **additive** — it specifies how many cycles to run from the current position, not a total. The orchestrator reads `lastCycleNumber` from state.json and computes:
- `startCycle = lastCycleNumber + 1`
- `endCycle = lastCycleNumber + cycles`

Examples:
- `/evolve-loop` → run 2 more cycles, balanced strategy
- `/evolve-loop 3` → run 3 more cycles, balanced strategy
- `/evolve-loop innovate` → run 2 more cycles, innovate strategy
- `/evolve-loop 3 harden` → run 3 more cycles, harden strategy
- `/evolve-loop 1 add dark mode support` → run 1 more cycle, balanced, goal="add dark mode support"
- `/evolve-loop 3 repair fix auth flow` → run 3 more cycles, repair strategy, goal="fix auth flow"

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
   mkdir -p .claude/evolve/workspace .claude/evolve/history .claude/evolve/evals .claude/evolve/instincts/personal .claude/evolve/instincts/archived .claude/evolve/genes .claude/evolve/tools
   ```

2. Read `.claude/evolve/state.json` if it exists. If not, initialize:
   ```json
   {"lastUpdated":"<now>","lastCycleNumber":0,"strategy":"balanced","research":{"queries":[]},"evaluatedTasks":[],"failedApproaches":[],"evalHistory":[],"instinctCount":0,"operatorWarnings":[],"nothingToDoCount":0,"maxCyclesPerSession":10,"warnAfterCycles":5,"tokenBudget":{"perTask":80000,"perCycle":200000},"stagnation":{"nothingToDoCount":0,"recentPatterns":[]},"planCache":[],"mastery":{"level":"novice","consecutiveSuccesses":0}}
   ```

   **Compute cycle range** (after reading state.json):
   - Read `lastCycleNumber` (default 0) from state.json
   - `startCycle = lastCycleNumber + 1`
   - `endCycle = lastCycleNumber + cycles`

   **Denial-of-wallet guardrails** (enforce after computing cycle range):
   - Read `maxCyclesPerSession` (default 10) and `warnAfterCycles` (default 5) from state.json
   - If `endCycle` > `maxCyclesPerSession`: HALT — "Requested cycles would reach cycle {endCycle}, exceeding maxCyclesPerSession ({maxCyclesPerSession}). Reduce the cycle count or update state.json to raise the cap."
   - If `cycles` >= `warnAfterCycles`: WARN — "Running {cycles} cycles (cycle {startCycle}→{endCycle}). Cost may be significant. Continue? (warnAfterCycles={warnAfterCycles})"

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
4. **Ship: commit and push** — every cycle MUST end with committed and pushed code
5. Learn: archive, extract instincts, operator check

For detailed phase-by-phase instructions, see [phases.md](phases.md).
For the shared memory protocol, see [memory-protocol.md](memory-protocol.md).
For the eval hard gate instructions, see [eval-runner.md](eval-runner.md).

## Agent Definitions

All agents are custom, self-contained. No external dependencies.

| Role | Agent File | Default Model | Workspace File |
|------|-----------|---------------|----------------|
| Scout | `evolve-scout.md` | sonnet | `scout-report.md` |
| Builder | `evolve-builder.md` | sonnet | `build-report.md` |
| Auditor | `evolve-auditor.md` | sonnet | `audit-report.md` |
| Operator | `evolve-operator.md` | haiku | `operator-log.md` |

## Dynamic Model Routing

The orchestrator selects the model for each agent invocation based on phase complexity, optimizing cost without sacrificing quality:

| Phase | Default Model | Upgrade Condition | Downgrade Condition |
|-------|--------------|-------------------|---------------------|
| Scout (DISCOVER) | sonnet | Goal requires deep research → opus | Cycle 2+ incremental scan → haiku |
| Builder (BUILD) | sonnet | Task complexity M + 5+ files → opus | S-complexity inline tasks → haiku |
| Auditor (AUDIT) | sonnet | Security-sensitive changes → opus | Clean build report, no risks flagged → haiku |
| Operator (LEARN) | haiku | HALT conditions detected → sonnet | Standard post-cycle → haiku |
| Meta-cycle review | opus | Always uses deep reasoning | — |

**Routing rules:**
- The orchestrator decides the model at launch time based on context (task complexity, strategy, cycle number)
- Override with `model` parameter in agent context if needed
- Track model usage in ledger entries for cost analysis
- The `repair` strategy always uses sonnet+ for Builder (accuracy matters more than cost)
- The `innovate` strategy can use haiku for Auditor on style checks (relaxed strictness)

**Eval Runner** — orchestrator-executed (not an agent), instructions in [eval-runner.md](eval-runner.md).

## Orchestrator Policies

These are graduated instincts — patterns confirmed across multiple cycles with high confidence (0.9+).

1. **Inline S-complexity tasks** (from inst-007, confidence 0.9): For small, well-defined tasks (S complexity, <10 lines changed, fully specified with eval definitions), the orchestrator may implement inline instead of spawning a builder agent. This saves ~30-50K tokens per task. Only applies when acceptance criteria and eval graders are unambiguous.

2. **Grep-based evals** (from inst-004, confidence 0.9): For Markdown/Shell projects without test infrastructure, grep-based eval checks are effective acceptance gates. Define specific grep commands with expected match counts.

3. **Meta-cycle self-improvement** (every 5 cycles): The orchestrator runs a meta-evaluation of its own pipeline effectiveness, analyzing success rates, agent efficiency, and stagnation patterns. May propose changes to agent prompts, strategies, or budgets. See Phase 5 step 6 in [phases.md](phases.md).

4. **Automated prompt evolution** (during meta-cycles): Uses a critique-synthesize loop to refine agent prompts based on cycle outcomes. Maximum 2 edits per meta-cycle, auto-reverts if performance degrades. See Phase 5 step 6d in [phases.md](phases.md).

5. **Gene/Capsule library**: Structured, reusable fix templates with pattern-matching selectors and pre/post validation. More actionable than instincts — genes describe *how to fix* with executable steps. See [docs/genes.md](docs/genes.md).

6. **Island model evolution** (advanced): Maintain 3-5 independent configurations evolving in parallel, with periodic migration of best-performing traits. See [docs/island-model.md](docs/island-model.md).

## Plan Template Caching

When a task is structurally similar to one solved in a previous cycle, reuse the plan template instead of full re-planning:

1. **Match:** After Scout selects tasks, check `state.json planCache` for templates matching the task type + affected file patterns
2. **Adapt:** If a match is found (similarity > 0.7), pass the cached template to the Builder as `priorPlan` in context. The Builder adapts it rather than designing from scratch.
3. **Store:** After a successful build (PASS audit), extract the plan as a reusable template:
   ```json
   {
     "slug": "<task-slug>",
     "taskType": "feature|stability|security|techdebt|performance",
     "filePatterns": ["src/**/*.ts", "tests/**/*.test.ts"],
     "approach": "<1-2 sentence approach summary>",
     "steps": ["<step 1>", "<step 2>"],
     "cycle": <N>,
     "successCount": 1
   }
   ```
4. **Evict:** Templates with 0 reuses after 10 cycles are pruned. Templates whose reuse leads to failure get demoted.

Plan caching achieves ~30-50% cost reduction on repeated task patterns by avoiding redundant analysis.

## Token Budgets

Each task and cycle has a token budget to prevent runaway costs:

- **Per-task budget** (`tokenBudget.perTask`, default 80,000): Maximum tokens a single Builder invocation should consume. If a task appears likely to exceed this (based on complexity/file count), the Scout should break it into smaller tasks.
- **Per-cycle budget** (`tokenBudget.perCycle`, default 200,000): Maximum tokens across all agents in a single cycle. The orchestrator tracks cumulative token usage and halts the cycle if exceeded.

**Budget enforcement:** These are soft limits — the orchestrator monitors usage and warns. If a Builder consistently exceeds the per-task budget, the Operator should recommend smaller task sizing in the next cycle.

The Scout MUST consider token budgets when sizing tasks. A task with complexity M that touches 10+ files is likely to exceed 80K tokens and should be split.

## Context Management

The evolve-loop uses a **stop-hook pattern** for context management (inspired by Ralph Loop). Instead of running until context exhaustion:

1. After each cycle, assess context window usage
2. If above 60% capacity → write a `handoff.md` file with session state and resume command
3. STOP gracefully, allowing the user to resume in a fresh session

This enables **indefinite runtime** across sessions. The handoff file carries forward all context needed to continue: strategy, goal, remaining cycles, stagnation patterns, and operator warnings.

The orchestrator reads `handoff.md` during initialization if it exists, applying the carried-forward context.

## Safety & Integrity

Self-modifying systems require explicit safety mechanisms to prevent misevolution:

### Memory Integrity
- **Instinct provenance:** Every instinct tracks its `source` (cycle + task). During meta-cycles, verify that instinct sources match actual cycle history in the ledger.
- **State.json validation:** Before each cycle, validate state.json structure against the expected schema. If corrupted or unexpected fields appear, warn and reset to last known good state.

### Eval Tamper Detection
- **Protected eval infrastructure:** The Builder MUST NOT modify files in `skills/evolve-loop/`, `agents/`, or `.claude-plugin/` unless the task explicitly targets the evolve-loop itself.
- **Eval checksum tracking:** After Scout creates eval definitions, the orchestrator records a checksum of each eval file. Before Auditor runs evals, verify checksums haven't changed. If they have and it wasn't a legitimate Scout update → HALT.
- **Objective hacking detection:** If a Builder removes or weakens eval criteria, assertion counts, or test commands, the Auditor flags this as CRITICAL severity regardless of other results.

### Rollback Protocol
- All changes are committed atomically per task. Rollback is always `git revert <commit>`.
- If 3 consecutive cycles show quality degradation (via delta metrics), auto-suggest rollback to the last stable cycle.
- Prompt evolution changes (from meta-cycles) auto-revert if the next meta-cycle shows worse performance.

## Anti-Patterns

1. **Over-discovery** — Scout should be incremental after cycle 1, not full audit every time
2. **Big tasks** — Prefer 3 small tasks over 1 large task. Each should be <50K tokens to build
3. **Retrying the same failure** — Log in state.json, try alternative next cycle
4. **Skipping the audit** — Auditor verdict of WARN or FAIL blocks shipping
5. **Ignoring instincts** — Builder MUST read instincts when available
6. **Research every cycle** — 12hr cooldown on web research. Reuse cached results
7. **Ceremony over substance** — Workspace files should be concise, not exhaustive
8. **Ignoring HALT** — When Operator returns HALT, pause and present to user
