---
name: evolve-loop
description: "Self-evolving development pipeline — 4 specialized agents (Scout, Builder, Auditor, Operator) across 5 phases. Build diverse small/medium tasks each cycle, iterate fast with quality gates. No external dependencies."
argument-hint: "[cycles] [strategy] [goal]"
disable-model-invocation: true
---

# Evolve Loop v6.7

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

The `cycles` argument is **additive** — it specifies how many new cycles to run. Each cycle atomically claims its cycle number at runtime (see Atomic Cycle Number Allocation below), so multiple parallel invocations get non-colliding cycle numbers.

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
Phase 0:   CALIBRATE ── orchestrator ── project benchmark scoring (once per invocation)
Phase 1:   DISCOVER ─── sequential ─── [Scout] scan + research + task selection
Phase 2:   BUILD ────── sequential ─── [Builder] design + implement + self-test (worktree)
Phase 3:   AUDIT ────── sequential ─── [Auditor] review + security + eval gate
           Δ CHECK ──── orchestrator ── benchmark delta gate (between AUDIT and SHIP)
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

1. Ensure directories exist and generate run ID:
   ```bash
   mkdir -p .evolve/workspace .evolve/history .evolve/evals .evolve/instincts/personal .evolve/instincts/archived .evolve/genes .evolve/tools .evolve/runs

   # Generate unique run ID for this invocation
   RUN_ID="run-$(date +%s%3N)-$(openssl rand -hex 2)"
   mkdir -p ".evolve/runs/$RUN_ID/workspace"
   WORKSPACE_PATH=".evolve/runs/$RUN_ID/workspace"

   # Prune run directories older than 48 hours
   find .evolve/runs/ -maxdepth 1 -type d -name 'run-*' -mtime +2 -exec rm -rf {} \; 2>/dev/null
   ```

   All workspace files for this invocation are scoped to `$WORKSPACE_PATH` instead of `.evolve/workspace/`. Shared directories (`.evolve/evals/`, `.evolve/instincts/`, `.evolve/history/`, `.evolve/genes/`, `.evolve/tools/`) remain unchanged.

   **Migration from pre-parallel layout** (one-time, idempotent):
   ```bash
   # Migrate project-digest.md to shared location
   if [ -f .evolve/workspace/project-digest.md ] && [ ! -f .evolve/project-digest.md ]; then
     cp .evolve/workspace/project-digest.md .evolve/project-digest.md
   fi

   # Migrate latest-brief.json to shared location
   if [ -f .evolve/workspace/next-cycle-brief.json ] && [ ! -f .evolve/latest-brief.json ]; then
     cp .evolve/workspace/next-cycle-brief.json .evolve/latest-brief.json
   fi

   # Seed run workspace from existing shared workspace (so first run has builder-notes, handoff, etc.)
   if [ -d .evolve/workspace ] && [ "$(ls -A .evolve/workspace/ 2>/dev/null)" ]; then
     cp -rn .evolve/workspace/* "$WORKSPACE_PATH/" 2>/dev/null || true
   fi
   ```

2. Read `.evolve/state.json` if it exists. If not, initialize:
   ```json
   {"lastUpdated":"<now>","lastCycleNumber":0,"version":0,"strategy":"balanced","research":{"queries":[]},"evaluatedTasks":[],"failedApproaches":[],"evalHistory":[],"instinctCount":0,"operatorWarnings":[],"nothingToDoCount":0,"warnAfterCycles":5,"tokenBudget":{"perTask":80000,"perCycle":200000},"stagnation":{"nothingToDoCount":0,"recentPatterns":[]},"planCache":[],"mastery":{"level":"novice","consecutiveSuccesses":0},"synthesizedTools":[],"ledgerSummary":{"totalEntries":0,"cycleRange":[0,0],"scoutRuns":0,"builderRuns":0,"totalTasksShipped":0,"totalTasksFailed":0,"avgTasksPerCycle":0},"instinctSummary":[],"projectBenchmark":{"lastCalibrated":null,"calibrationCycle":0,"overall":0,"dimensions":{},"history":[],"highWaterMarks":{}},"auditorProfile":{"feature":{"passFirstAttempt":0,"consecutiveClean":0},"stability":{"passFirstAttempt":0,"consecutiveClean":0},"security":{"passFirstAttempt":0,"consecutiveClean":0},"techdebt":{"passFirstAttempt":0,"consecutiveClean":0},"performance":{"passFirstAttempt":0,"consecutiveClean":0}}}
   ```

   **State migration** (after reading existing state.json):
   - If `version` field is missing → add `"version": 0` (pre-parallel state.json)
   - Write the migrated state back immediately

   **Track remaining cycles:**
   - Set `remainingCycles = cycles` (from argument parsing)
   - Cycle numbers are claimed atomically at the start of each cycle (see phases.md), not computed upfront
   - This allows parallel invocations to get non-colliding cycle numbers

   **Cost awareness:**
   - Read `warnAfterCycles` (default 5) from state.json
   - If `cycles` >= `warnAfterCycles`: WARN — "Running {cycles} cycles. Cost may be significant. Continue? (warnAfterCycles={warnAfterCycles})"

3. **Detect project context and domain:**

   **Step 3a — Check for domain override:**
   ```bash
   cat .evolve/domain.json 2>/dev/null
   ```
   If `.evolve/domain.json` exists, read its fields and merge into `projectContext`:
   ```json
   {
     "domain": "coding|writing|research|design|mixed",
     "evalMode": "bash|rubric|hybrid",
     "shipMechanism": "git|file-save|export|custom",
     "buildIsolation": "worktree|file-copy|branch|none"
   }
   ```

   **Step 3b — Auto-detect (when domain.json absent or incomplete):**
   Scan the project root for domain signals:
   - `package.json`, `go.mod`, `Cargo.toml`, `*.py`, test commands → `domain: "coding"`
   - `*.md`/`*.docx`/`*.txt` majority (>60%), no build commands → `domain: "writing"`
   - `*.md` with citation patterns, `references/` dir → `domain: "research"`
   - `*.figma`/`*.sketch`/`*.svg` majority → `domain: "design"`
   - Default: `"coding"` (backward-compatible)

   **Step 3c — Build projectContext:**
   ```json
   {
     "projectContext": {
       "domain": "<detected or overridden>",
       "language": "<detected>",
       "framework": "<detected>",
       "testCommand": "<detected>",
       "evalMode": "<from domain.json or default based on domain>",
       "shipMechanism": "<from domain.json or default based on domain>",
       "buildIsolation": "<from domain.json or default based on domain>"
     }
   }
   ```

   **Domain defaults** (when domain.json doesn't specify):
   | Domain | evalMode | shipMechanism | buildIsolation |
   |--------|----------|---------------|----------------|
   | coding | bash | git | worktree |
   | writing | rubric | file-save | file-copy |
   | research | hybrid | file-save | file-copy |
   | design | rubric | export | file-copy |
   | mixed | hybrid | git | worktree |

   `projectContext.domain` is passed to all agents via context blocks. Agents use it to select appropriate eval graders (see [eval-runner.md](eval-runner.md) Non-Code Eval Graders) and ship mechanisms (see [docs/domain-adapters.md](../../docs/domain-adapters.md)).

   See [docs/configuration.md](../../docs/configuration.md) Domain Detection for full signal tables and override documentation.

4. **Pre-flight check** (inline, no agent):
   ```bash
   git status --porcelain   # must be clean
   git worktree list        # worktree support available
   ls .evolve/evals/ 2>/dev/null  # evals exist (skip check on cycle 1)
   ```
   If git is dirty, warn user before proceeding.

## Orchestrator Loop

You are the orchestrator. For each cycle:
1. **Claim cycle number** atomically via OCC protocol (see phases.md)
2. Launch Scout → collect task list → **claim tasks** via OCC to prevent duplicates across parallel runs
3. For each task: Launch Builder (**MUST use `isolation: "worktree"`**) → Launch Auditor
4. If Auditor PASS → commit. If WARN/FAIL → re-run Builder with issues (max 3 attempts, each in a fresh worktree)
5. **Ship: commit and push** — acquire `.evolve/.ship-lock` before pushing (serial SHIP phase)
6. Learn: archive, extract instincts, operator check
7. Decrement `remainingCycles`; if 0 → run cleanup and exit

**Run Cleanup** (after all cycles complete):
```bash
# Copy final workspace to shared location for backward compatibility
cp -rp "$WORKSPACE_PATH"/* .evolve/workspace/ 2>/dev/null
# Keep run directory for 48 hours (pruned on next invocation init)
```

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

5. **Gene/Capsule library**: Structured, reusable fix templates with pattern-matching selectors and pre/post validation. More actionable than instincts — genes describe *how to fix* with executable steps. See [docs/genes.md](../../docs/genes.md).

6. **Island model evolution** (advanced): Maintain 3-5 independent configurations evolving in parallel, with periodic migration of best-performing traits. See [docs/island-model.md](../../docs/island-model.md).

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

The evolve-loop runs **continuously through all requested cycles without stopping**. It never pauses to ask the user to resume.

1. After each cycle, write a `handoff.md` file with session state as a safety checkpoint (in case the session is interrupted externally)
2. **Continue immediately to the next cycle** — do NOT stop, do NOT output a resume command, do NOT wait for user input
3. If context window pressure is high, minimize workspace file sizes and rely on state.json summaries rather than re-reading full files

The handoff file is a **checkpoint only** — it exists so that if a session is externally interrupted, a new session can pick up where it left off. It is NOT a signal to stop.

The orchestrator reads `handoff.md` during initialization if it exists, applying the carried-forward context.

## Safety & Integrity

Self-modifying systems require explicit safety mechanisms to prevent misevolution:

### Memory Integrity
- **Instinct provenance:** Every instinct tracks its `source` (cycle + task). During meta-cycles, verify that instinct sources match actual cycle history in the ledger.
- **State.json validation:** Before each cycle, validate state.json structure against the expected schema. If corrupted or unexpected fields appear, warn and reset to last known good state.

### Eval Tamper Detection
- **Protected eval infrastructure:** The Builder MUST NOT modify files in `skills/evolve-loop/`, `agents/`, or CLI plugin metadata folders (like `.claude-plugin/`) unless the task explicitly targets the evolve-loop itself.
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
6. **Research every cycle** — 12hr cooldown on web research. Reuse cached results. For cross-run deduplication in parallel executions, see [Cross-Run Research Deduplication](../../docs/token-optimization.md#cross-run-research-deduplication).
7. **Ceremony over substance** — Workspace files should be concise, not exhaustive
8. **Ignoring HALT** — When Operator returns HALT, pause and present to user
9. **Complexity creep** — If a task adds more lines than proportional to its complexity (S-tasks >30 lines, M-tasks >80 lines), break it down into smaller tasks or simplify the approach. Autonomous systems tend toward accretion — actively resist by preferring deletions that maintain functionality over additions

## Auditor Adaptive Strictness

The Auditor uses `auditorProfile` (stored in state.json) to skip redundant checklist sections for task types that have built up a reliability record. This reduces token usage on routine audits without sacrificing safety on novel or high-risk changes.

- `auditorProfile.<type>.consecutiveClean` — number of consecutive audits with no MEDIUM+ issues for that task type
- `auditorProfile.<type>.passFirstAttempt` — cumulative count of audits that passed on the first attempt
- When `consecutiveClean >= 5`: Auditor runs reduced checklist (Security + Eval Gate only). Code Quality and Pipeline Integrity sections are skipped.
- The orchestrator resets `consecutiveClean` to 0 after any audit that produces a WARN, FAIL, or MEDIUM+ issue.
- `harden`, `repair` strategies and tasks touching agent/skill files always receive the full checklist regardless of profile.

The orchestrator passes `auditorProfile` as part of the Auditor context block (see phases.md Phase 3).

## Bandit Task Selection

The Scout uses a multi-armed bandit mechanism to bias task selection toward historically high-reward task types. This makes the loop adaptive: task types that consistently ship successfully receive a selection boost, while types that stall or fail are deprioritized.

### Mechanism

Each task type (`feature`, `stability`, `security`, `techdebt`, `performance`) has an arm tracked in `state.json` under `taskArms`. The Scout reads `taskArms` before finalizing the task list and applies Thompson Sampling-style weighting:

- **Exploration vs. exploitation**: Each arm accumulates `pulls` (times selected) and `totalReward` (sum of outcomes, 0 or 1 per shipped task). Arms with higher `avgReward` receive a priority boost of up to +1 complexity level (e.g., a normally M task from a high-reward type may be treated as S-priority).
- **Boost rule**: If `avgReward >= 0.8` and `pulls >= 3`, the task type earns a +1 priority boost in selection ranking. The boost is capped at one level to avoid over-exploitation.
- **Exploration floor**: Arms with fewer than 3 pulls are always eligible regardless of reward history, ensuring all task types remain in rotation.

### Update Rule

After Phase 4 (SHIP), the orchestrator updates `taskArms` for each shipped task type:
- Successful ship: `totalReward += 1`, `pulls += 1`
- Failed/aborted: `pulls += 1` (reward unchanged)
- Recompute `avgReward = totalReward / pulls`

### Novelty / Curiosity Bonus

Tasks that touch files not recently modified receive a priority boost, encouraging exploration of under-visited areas. Scout computes novelty using `state.json fileExplorationMap`:

- If a task's target files have `lastTouchedCycle <= currentCycle - 3` (or are absent from the map), the task receives **+1 priority boost** in selection ranking.
- This exploration reward is subordinate to bandit boosts — novelty boost applies first, then bandit boost may stack.
- Files touched this cycle are updated in `fileExplorationMap` during Phase 4 (SHIP).

### Interaction with Strategy

The bandit boost is subordinate to the active strategy:
- `innovate` — forces `feature` arm into consideration regardless of reward history
- `repair` — forces `stability` arm; suppresses `feature` boost
- `harden` — `stability` and `security` arms always eligible at full priority
- `balanced` — bandit weighting applies without override
