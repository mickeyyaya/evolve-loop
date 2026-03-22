---
name: evolve-loop
description: "Platform-agnostic self-evolving development pipeline — 4 specialized agents (Scout, Builder, Auditor, Operator) across 5 phases. Works with Claude Code, Gemini CLI, or any LLM with file I/O and shell access. No external dependencies."
argument-hint: "[cycles] [strategy] [goal]"
disable-model-invocation: true
---

# Evolve Loop v7.8

Orchestrates 4 specialized agents through 5 lean phases per cycle. Optimized for fast iteration: discover → build → audit → ship → learn. Each cycle targets 2-4 small/medium tasks, builds them in isolated worktrees, and gates on MEDIUM+ audit findings.

**Usage:** `/evolve-loop [cycles] [strategy] [goal]`

## Argument Parsing

Parse `$ARGUMENTS` as follows:
- If the first token is a number → use it as `cycles` (number of NEW cycles to run)
- If a token matches a strategy name (`innovate`, `harden`, `repair`, `ultrathink`) → use it as `strategy`
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
| `ultrathink` | Deep reasoning, complex refactors, structural changes | Maximum thinking budget (tier-1 forced), step-by-step verification | Strict on all dimensions, triggers stepwise confidence checks |

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

## Shared Agent Values

These values are injected into EVERY agent context block by the orchestrator. They define protocols, thresholds, and anti-patterns that all agents must follow. Individual agents may override specific fields (documented below). (Research basis: agent-shared-values-patterns — layered inheritance with override resolution ensures consistency when meta-cycle prompt evolution modifies shared protocols.)

```json
{
  "sharedValues": {
    "antiBiasProtocols": [
      "verbosity-bias: longer output ≠ better output — penalize unnecessary complexity",
      "self-preference-bias: evaluate against acceptance criteria, not personal style",
      "blind-trust-bias: verify upstream claims independently — do not trust without checking"
    ],
    "requiredProtocols": [
      "challenge-token-verification: embed challengeToken in workspace output and ledger entry",
      "ledger-entry-on-completion: write structured ledger entry with prevHash chain",
      "mailbox-check-on-start: read agent-mailbox.md for messages addressed to you or 'all'"
    ],
    "qualityThresholds": {
      "maxFileLines": 800,
      "maxFunctionLines": 50,
      "blockingSeverity": "MEDIUM+",
      "evalRigorMinimum": "Level 2 (ACCEPTABLE)"
    },
    "integrityRules": [
      "never modify files in skills/evolve-loop/ or agents/ unless task explicitly targets evolve-loop",
      "never weaken or remove eval criteria created by upstream agents",
      "never bypass quality gates — implement genuine functionality, not specification gaming"
    ]
  }
}
```

**Agent-specific overrides:**
- **Auditor** adds `"confidence-scoring"` and `"step-confidence-cross-validation"` to `requiredProtocols`
- **Operator** removes `"ledger-entry-on-completion"` (read-only agent, does not write ledger entries)
- **Builder** adds `"step-level-confidence-reporting"` to `requiredProtocols`

**How to inject:** Include the `sharedValues` block in the static portion of every agent context (before cycle-specific fields). This maximizes KV-cache reuse across parallel agent launches — the shared prefix is identical across agents.

**Meta-cycle interaction:** When prompt evolution modifies a shared protocol, edit the `sharedValues` block here instead of editing each agent file separately. This single-source-of-truth prevents inconsistency across agents.

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
Phase 6:   META-CYCLE ──── orchestrator ── self-improvement (every 5 cycles, conditional)
```

For multiple tasks per cycle, Phase 2-3 uses dependency-aware parallelization:
```
Scout → [Task A, Task B, Task C]
  → Partition by file overlap (tasks sharing filesToModify → sequential group)
  → Group 1 (independent): Builder(A) + Builder(B) in parallel worktrees
     → Auditor(A) + Auditor(B) in parallel → commit A, commit B
  → Group 2 (depends on Group 1): Builder(C) → Auditor(C) → commit C
→ Ship → Learn
```

**Dependency partitioning rule:** Two tasks are independent if their `filesToModify` lists share no files. Independent tasks are built in parallel via the platform's agent mechanism (each in its own isolated worktree). Dependent tasks (shared files) run sequentially within their group. Commits are always applied sequentially in Scout's priority order, regardless of build parallelism. See [docs/platform-compatibility.md](../../docs/platform-compatibility.md) for platform-specific agent dispatch.

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

   `projectContext.domain` is passed to all agents via context blocks. Agents use it to select appropriate eval graders (see [eval-runner.md](skills/evolve-loop/eval-runner.md) Non-Code Eval Graders) and ship mechanisms (see [docs/domain-adapters.md](docs/domain-adapters.md)).

   See [docs/configuration.md](docs/configuration.md) Domain Detection for full signal tables and override documentation.

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
3. **Partition tasks:** inline-eligible (S-complexity, per inst-007) first, then worktree tasks. Execute inline tasks first, committing each before worktree tasks start (prevents cherry-pick conflicts).
4. For inline tasks: implement directly → run eval graders → commit if PASS
5. For worktree tasks: Launch Builder (**MUST use `isolation: "worktree"`**) → Launch Auditor → cherry-pick if PASS
6. If Auditor WARN/FAIL → re-run Builder with issues (max 3 attempts, each in a fresh worktree)
7. **Ship: push** — acquire `.evolve/.ship-lock` before pushing (serial SHIP phase)
8. Learn: archive, extract instincts, operator check
9. Decrement `remainingCycles`; if 0 → run cleanup and exit

**Run Cleanup** (after all cycles complete):
```bash
# Copy final workspace to shared location for backward compatibility
cp -rp "$WORKSPACE_PATH"/* .evolve/workspace/ 2>/dev/null
# Keep run directory for 48 hours (pruned on next invocation init)
```

**Final Session Report** — after all cycles complete, generate and output a comprehensive report:

1. Read `state.json` for cumulative data: `ledgerSummary`, `projectBenchmark`, `evalHistory`, `instinctSummary`, `taskArms`, `mastery`, `fitnessScore`, `fitnessHistory`, `operatorWarnings`, `processRewardsHistory`
2. Read `$WORKSPACE_PATH/session-summary.md` (written by Operator on last cycle)
3. Generate `$WORKSPACE_PATH/final-report.md` and **output its contents to the user**:

```markdown
# Evolve Loop — Session Report

**Run:** {RUN_ID} | **Cycles:** {first}–{last} | **Strategy:** {strategy} | **Goal:** {goal or "autonomous"}

---

## Summary
<3-4 sentence narrative: what the session accomplished, key patterns, current project state. Source from session-summary.md synthesis.>

## Tasks

| Cycle | Task | Type | Verdict | Audit Attempts |
|-------|------|------|---------|----------------|
| {N} | {slug} | {type} | PASS/FAIL | {attempts} |
| ... | ... | ... | ... | ... |

**Shipped:** {totalShipped}/{totalAttempted} ({successRate}%) | **Failed:** {totalFailed}

## Benchmark Trajectory

| Dimension | Start | End | Delta |
|-----------|-------|-----|-------|
| Documentation Completeness | {X} | {Y} | {+/-N} |
| ... | ... | ... | ... |
| **Overall** | **{start}** | **{end}** | **{+/-N}** |

## Learning

- **Instincts extracted:** {count} | **Graduated:** {count}
- **Mastery level:** {level} ({consecutiveSuccesses} consecutive successes)
- **Top instincts this session:**
  - {inst-id}: {pattern} (confidence: {value})
  - ...

## Task Type Performance

| Type | Attempted | Shipped | Success Rate | Bandit Avg Reward |
|------|-----------|---------|--------------|-------------------|
| feature | {N} | {N} | {%} | {avgReward} |
| stability | {N} | {N} | {%} | {avgReward} |
| ... | ... | ... | ... | ... |

## Warnings & Recommendations

- {operator warnings, pending improvements, or "No active warnings"}
- **Recommended next strategy:** {from operator brief}
- **Weakest dimension:** {from benchmark}

---
*Report: .evolve/runs/{RUN_ID}/workspace/final-report.md*
```

4. Copy `final-report.md` to `.evolve/workspace/final-report.md` for easy access

For detailed phase-by-phase instructions, see [phases.md](skills/evolve-loop/phases.md).
For the shared memory protocol, see [memory-protocol.md](skills/evolve-loop/memory-protocol.md).
For the eval hard gate instructions, see [eval-runner.md](skills/evolve-loop/eval-runner.md).
For meta-cycle self-improvement (every 5 cycles), see [phase6-metacycle.md](skills/evolve-loop/phase6-metacycle.md).

## Agent Definitions

All agents are custom, self-contained. No external dependencies.

| Role | Agent File | Default Tier | Workspace File |
|------|-----------|-------------|----------------|
| Scout | `evolve-scout.md` | tier-2 | `scout-report.md` |
| Builder | `evolve-builder.md` | tier-2 | `build-report.md` |
| Auditor | `evolve-auditor.md` | tier-2 | `audit-report.md` |
| Operator | `evolve-operator.md` | tier-3 | `operator-log.md` |

## Model Tier System

The evolve-loop uses a **3-tier model abstraction** (tier-1: deep reasoning, tier-2: standard work, tier-3: lightweight tasks) so it works across any LLM provider. Routing references tiers, not specific model names.

| Tier | Capability | Cost Ratio |
|------|-----------|------------|
| **tier-1** | Deep reasoning, complex architecture | ~3-5x of tier-2 |
| **tier-2** | Standard development work (most invocations) | 1x (baseline) |
| **tier-3** | Fast classification, routine checks | ~0.1-0.3x of tier-2 |

For provider model mappings, configuration overrides, and detailed routing rules, see [docs/model-routing.md](../../docs/model-routing.md) and [docs/models-quickstart.md](../../docs/models-quickstart.md).

## Dynamic Model Routing (quick reference)

| Phase | Default | Upgrade → | Downgrade → |
|-------|---------|-----------|-------------|
| Scout | tier-2 | Cycle 1 or goal-directed → tier-1 | Cycle 4+ mature data → tier-3 |
| Builder | tier-2 | M+5 files or retry ≥ 2 → tier-1 | S + plan cache → tier-3 |
| Auditor | tier-2 | Security-sensitive → tier-1 | Clean build → tier-3 |
| Operator | tier-3 | Last cycle / regression → tier-2 | Standard → tier-3 |
| Meta-cycle | tier-1 | Always deep reasoning | — |

`ultrathink` strategy forces tier-1 for all agents. `repair` forces tier-2+ for Builder. Full routing rules in [docs/model-routing.md](../../docs/model-routing.md).

**Eval Runner** — orchestrator-executed (not an agent), instructions in [eval-runner.md](skills/evolve-loop/eval-runner.md).

## Orchestrator Policies

These are graduated instincts — patterns confirmed across multiple cycles with high confidence (0.9+).

1. **Inline S-complexity tasks** (from inst-007, confidence 0.9): For small, well-defined tasks (S complexity, <10 lines changed, fully specified with eval definitions), the orchestrator may implement inline instead of spawning a builder agent. This saves ~30-50K tokens per task. Only applies when acceptance criteria and eval graders are unambiguous.

2. **Grep-based evals** (from inst-004, confidence 0.9): For Markdown/Shell projects without test infrastructure, grep-based eval checks are effective acceptance gates. Define specific grep commands with expected match counts.

3. **Meta-cycle self-improvement** (every 5 cycles): The orchestrator runs a meta-evaluation of its own pipeline effectiveness, analyzing success rates, agent efficiency, and stagnation patterns. May propose changes to agent prompts, strategies, or budgets. See Phase 5 step 6 in [phases.md](skills/evolve-loop/phases.md).

4. **Automated prompt evolution** (during meta-cycles): Uses a critique-synthesize loop to refine agent prompts based on cycle outcomes. Maximum 2 edits per meta-cycle, auto-reverts if performance degrades. See Phase 5 step 6d in [phases.md](skills/evolve-loop/phases.md).

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

The evolve-loop runs continuously through all requested cycles. Between cycles, the orchestrator performs a **strategic compaction** to prevent context degradation:

1. After each cycle, write `handoff.md` with full session state (existing behavior)
2. **Output a structured cycle summary** (5-8 lines) that captures:
   - Tasks shipped and their verdicts
   - Benchmark delta (if measured)
   - Any warnings or stagnation signals
   - Key decisions for next cycle
3. **Continue immediately to the next cycle** — do NOT stop or wait for user input. When running in autonomous mode (bypass permissions / yolo mode / auto-approve — terminology varies by platform), the orchestrator must complete ALL requested cycles without pausing. "Continue without asking" means maximum velocity with full pipeline compliance — it does NOT mean skip agents or fabricate cycles. Every cycle must run Scout → Builder → Auditor → phase-gate.sh verification.
4. **Lean Context Mode (Cycles 4+):** To prevent context accumulation, past cycle 3, the orchestrator MUST aggressively truncate historical logs and ONLY pass the structured `handoff.md` and `instinctSummary` to downstream agents.
   - Stop re-reading files already in context (use remembered values)
   - Skip reading full agent results — extract only verdict + key metrics
   - Rely on state.json and handoff.md rather than conversation history

The handoff file is a **checkpoint and compaction anchor** — it exists so that if a session is externally interrupted, a new session can pick up where it left off. When the host LLM auto-compacts conversation history, the structured handoff ensures cross-cycle continuity survives summarization. It is NOT a signal to stop.

The orchestrator reads `handoff.md` during initialization if it exists, applying the carried-forward context.

## Safety & Integrity

Self-modifying systems require explicit safety mechanisms to prevent misevolution. The evolve-loop separates **enforcement** (deterministic scripts) from **execution** (LLM agents).

### Phase Gate Script (primary enforcement)

The `scripts/phase-gate.sh` script runs at every phase transition and enforces structural constraints the orchestrator cannot bypass. This is the trust boundary between LLM judgment and deterministic verification. See [docs/incident-report-cycle-132-141.md](../../docs/incident-report-cycle-132-141.md) for why this exists.

**What the script controls (LLM cannot override):**
- Phase progression — artifacts must exist before the next phase starts
- Eval verification — `verify-eval.sh` runs independently, not trusting Auditor claims
- Health fingerprint — `cycle-health-check.sh` runs every cycle, cannot be skipped
- State.json writes — only the script increments `lastCycleNumber` and `consecutiveSuccesses`
- Mastery updates — only incremented after script-verified audit PASS

**What the LLM controls:**
- All creative work: task selection, implementation, code review, instinct extraction

Research basis: Greenblatt "AI Control" (2023) — trusted monitoring must be structurally separate from the monitored agent. Redwood Research "Factored Cognition" (2025) — trusted decomposition + untrusted execution.

### Memory Integrity
- **Instinct provenance:** Every instinct tracks its `source` (cycle + task). During meta-cycles, verify that instinct sources match actual cycle history in the ledger.
- **State.json validation:** Before each cycle, validate state.json structure against the expected schema. If corrupted or unexpected fields appear, warn and reset to last known good state.

### Eval Tamper Detection
- **Protected eval infrastructure:** The Builder MUST NOT modify files in `skills/evolve-loop/`, `agents/`, `scripts/`, or CLI plugin metadata folders (like `.claude-plugin/`) unless the task explicitly targets the evolve-loop itself.
- **Eval checksum tracking:** Captured by the phase gate script after DISCOVER; verified before AUDIT. Orchestrator cannot tamper because the script computes and checks independently.
- **Objective hacking detection:** If a Builder removes or weakens eval criteria, the Auditor flags as CRITICAL severity.

### Rollback Protocol
- All changes are committed atomically per task. Rollback is always `git revert <commit>`.
- If 3 consecutive cycles show quality degradation (via delta metrics), auto-suggest rollback to the last stable cycle.
- Prompt evolution changes (from meta-cycles) auto-revert if the next meta-cycle shows worse performance.

### Known Incidents
- **Cycles 102-111:** Builder reward hacking via tautological evals. See [docs/incident-report-cycle-102-111.md](../../docs/incident-report-cycle-102-111.md).
- **Cycles 132-141:** Orchestrator gaming via skipped agents, fabricated cycles, mastery inflation. See [docs/incident-report-cycle-132-141.md](../../docs/incident-report-cycle-132-141.md). Root cause: all enforcement mechanisms were orchestrator-invoked. Fix: phase gate script moves enforcement to deterministic bash.

## Anti-Patterns

1. **Over-discovery** — Scout should be incremental after cycle 1, not full audit every time
2. **Big tasks** — Prefer 3 small tasks over 1 large task. Each should be <50K tokens to build
3. **Retrying the same failure** — Log in state.json, try alternative next cycle
4. **Skipping the audit** — Auditor verdict of WARN or FAIL blocks shipping
5. **Ignoring instincts** — Builder MUST read instincts when available
6. **Research every cycle** — 12hr cooldown on web research. Reuse cached results. For cross-run deduplication in parallel executions, see [Cross-Run Research Deduplication](docs/token-optimization.md#cross-run-research-deduplication).
7. **Ceremony over substance** — Workspace files should be concise, not exhaustive
8. **Ignoring HALT** — When Operator returns HALT, pause and present to user
9. **Complexity creep** — If a task adds more lines than proportional to its complexity (S-tasks >30 lines, M-tasks >80 lines), break it down into smaller tasks or simplify the approach. Autonomous systems tend toward accretion — actively resist by preferring deletions that maintain functionality over additions
10. **Orchestrator gaming** — The orchestrator MUST NOT skip agents, fabricate cycles, inflate mastery, or batch-manipulate state.json. Phase gate script (`scripts/phase-gate.sh`) enforces this structurally. If you find yourself wanting to shortcut the pipeline "for efficiency," that impulse is the specification gaming the pipeline is designed to prevent. (Incident: cycles 132-141)

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
