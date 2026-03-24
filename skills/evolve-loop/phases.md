> Read this file when orchestrating the full evolve-loop cycle. Covers phase gate verification, cycle setup, and all phase transitions (DISCOVER through LEARN).

## Contents
- [Mandatory Phase Gate Verification](#mandatory-phase-gate-verification) — deterministic trust boundary at every transition
- [Phase 0: CALIBRATE](#phase-0-calibrate-once-per-invocation) — project benchmark baseline
- [Cycle Setup](#for-each-cycle-while-remainingcycles--0) — context budget, lean mode, integrity
- [Phase 1: DISCOVER](#phase-1-discover) — Scout launch, task claiming, stagnation detection
- [Context Quality & Handoffs](#context-quality-checklist-cemm) — CEMM checklist, per-phase matrix, handoff format
- [Phase 2: BUILD](#phase-2-build-loop-per-task) — Builder dispatch (see phase2-build.md)
- [Phase 3: AUDIT](#phase-3-audit-parallel) — Auditor launch, eval checksum verification
- [Benchmark Delta & Pre-Ship Gate](#benchmark-delta-check--pre-ship-integrity-gate) — regression detection, health check
- [Phase 4: SHIP](#phase-4-ship) — see phase4-ship.md
- [Phase 5: LEARN](#phase-5-learn) — see phase5-learn.md

# Evolve Loop — Phase Instructions

**Important:** Every agent context block must include `goal` (string or null) and `strategy` (one of: `balanced`, `innovate`, `harden`, `repair`, `ultrathink`).

## Mandatory Phase Gate Verification

Run `scripts/phase-gate.sh` at every phase transition. This deterministic bash script verifies artifacts exist, agents ran, and integrity checks pass. The orchestrator cannot skip it — it is the trust boundary between LLM judgment and structural enforcement.

```bash
bash scripts/phase-gate.sh <gate> <cycle> <workspace_path>
# Gates: discover-to-build, build-to-audit, audit-to-ship, ship-to-learn, cycle-complete
# Exit 0 = proceed, Exit 1 = block, Exit 2 = halt (present to human)
```

| Enforcement | Detail |
|-------------|--------|
| Workspace artifacts | scout/build/audit reports must exist and be fresh (<10 min old) |
| Eval re-execution | `verify-eval.sh` runs independently (not trusting Auditor claims) |
| Cycle health | `cycle-health-check.sh` runs 11 integrity signals |
| State updates | `state.json` cycle number and mastery updated ONLY by the script |
| Tamper detection | Eval checksums verified |

The LLM retains all creative work — task selection, implementation, code review, instinct extraction. The script enforces *process*, the LLM does *substance*.

## Phase 0: CALIBRATE (once per invocation)

Runs **once per `/evolve-loop` invocation**, not per cycle. Establishes a project-level benchmark baseline.

For detailed calibration instructions, see [phase0-calibrate.md](skills/evolve-loop/phase0-calibrate.md).

| Step | Action |
|------|--------|
| Skip check | Skip if `projectBenchmark.lastCalibrated` < 1 hour ago |
| Automated scoring | Run checks from `benchmark-eval.md` for all 8 dimensions |
| Composite | `0.7 * automated + 0.3 * llm` per dimension |
| Store | Write to `state.json.projectBenchmark`, pass `benchmarkWeaknesses` to Scout |
| Tamper detection | Checksum `benchmark-eval.md` for verification in Phase 3-4 |

---

## FOR each cycle (while remainingCycles > 0):

### Eager Context Budget Estimation

At cycle start, **before launching any agent**, estimate total context cost. This enables proactive lean mode entry and Self-MoA decisions. (Research: OPENDEV [arXiv:2603.05344], Token Consumption Prediction [OpenReview:1bUeVB3fov].)

| Task Type | Estimated Tokens |
|-----------|-----------------|
| S-task inline | ~5-10K |
| S-task worktree | ~20-40K |
| M-task worktree | ~40-80K |
| M-task Self-MoA | ~80-160K (N x single) |
| Orchestrator overhead | ~10-15K |

If `estimatedCycleTokens > tokenBudget.perCycle * 0.8`: drop lowest-priority tasks, switch M-tasks to single-Builder, or activate lean mode immediately.

### Lean Mode (cycles 4+ OR budget pressure)

Activated after 3 cycles OR when `budgetPressure == "high"` OR when estimated tokens exceed 80% of budget:

| Optimization | Detail |
|-------------|--------|
| State.json | Read ONCE at cycle start; use in-memory copy thereafter |
| Agent results | Read summary only; read full file only if summary indicates a problem |
| Scout report | Extract task list from agent return value, not by reading scout-report.md |
| Eval checksums | Compute once, verify from memory |
| Benchmark delta | Skip for cycles where all tasks are S-complexity docs-only |

Saves ~15-20K tokens per cycle (from ~50K to ~30K).

### AgentDiet Trajectory Compression

Between phase transitions, compress accumulated context. (Research: AgentDiet [arXiv:2509.23586] — 40-60% input token savings.)

| After Phase | Keep | Remove |
|------------|------|--------|
| DISCOVER | Task list and eval definitions | Full Scout analysis |
| BUILD | build-report.md summary and worktree branch/SHA | Full Builder output |
| AUDIT | Verdict (PASS/WARN/FAIL) and issue list | Full Auditor analysis |
| SHIP | Commit SHA and benchmark delta | Commit details |

**Never prune:** challenge token, eval checksums, failed approach details, stagnation patterns.

### Cycle Integrity Setup

At cycle start, before any agent invocation, establish the integrity layer:

1. **Generate challenge token:**
   ```bash
   CHALLENGE=$(openssl rand -hex 8 2>/dev/null || head -c 8 /dev/urandom | xxd -p)
   ```
   Pass to every agent in their context block. Each agent embeds it in workspace output and ledger entries. `scripts/cycle-health-check.sh` verifies consistency.

2. **Plant canary files:**
   ```bash
   CANARY_VALUE=$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary"
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary-expected"

   if [ ! -f ".evolve/evals/_canary-eval.md" ]; then
     cat > ".evolve/evals/_canary-eval.md" << 'CANARY'
   # Canary Eval (DO NOT REFERENCE)
   This file monitors for eval system probing. Any access = anomaly.
   ## Graders
   - `echo "canary-sentinel-$(date +%s)"`
   CANARY
   fi
   ```

3. **Adaptive strictness decay** (cross-session — research: "Alignment Faking", Anthropic):
   If first cycle of a new invocation, halve all `consecutiveClean` values in `auditorProfile`:
   ```
   for each taskType in auditorProfile:
     auditorProfile[taskType].consecutiveClean = floor(consecutiveClean / 2)
   ```

4. **Random full-audit flag** (blind monitoring):
   ```bash
   RANDOM_CHECK=$((RANDOM % 5))  # 0-4
   FORCE_FULL_AUDIT=false
   if [ "$RANDOM_CHECK" -eq 0 ]; then
     FORCE_FULL_AUDIT=true
   fi
   ```
   Pass `forceFullAudit` in Auditor context. Do NOT log whether triggered.

5. **Hash chain initialization** (tamper-evident ledger — research: SLSA transparency logs, in-toto):
   ```bash
   if command -v sha256sum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | sha256sum | cut -d' ' -f1)
   elif command -v shasum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | shasum -a 256 | cut -d' ' -f1)
   else
     PREV_HASH="unsupported"
   fi
   ```
   Every ledger entry MUST include `"prevHash":"<$PREV_HASH>"`. After writing each entry, update `PREV_HASH` to the hash of the entry just written.

### Atomic Cycle Number Allocation

1. Read `state.json`, note `version = V` and `lastCycleNumber`
2. `claimedCycle = lastCycleNumber + 1`
3. Write `state.json` with `lastCycleNumber = claimedCycle`, `version = V + 1`
4. Re-read `state.json` and verify `version == V + 1`
5. If version mismatch → re-read, re-claim next available number
6. Use `claimedCycle` as this iteration's cycle number `N`
7. Decrement `remainingCycles`

### Phase 1: DISCOVER

**Convergence Short-Circuit** (check BEFORE launching Scout):

| `nothingToDoCount` | Action |
|--------------------|--------|
| `>= 2` | Skip Scout. Jump to Phase 5 with Operator in `"convergence-check"` mode. Operator can reset to 0 if new work detected. |
| `== 1` | **Escalation before convergence:** review last 3 cycles' deferred tasks for combinations, check strategy switch, propose a "radical" task. If viable task found → reset to 0, proceed. If not → launch Scout in `"convergence-confirmation"` mode (reads ONLY state.json + `git log --oneline -3`, MUST trigger new web research). If still nothing → increment to 2, skip to Phase 5. |
| `== 0` | Normal Scout launch. |

**Pre-compute context** (read files once, pass inline slices):
```bash
# Cycle 1: full mode — no digest exists yet
# Cycle 2+: incremental mode — read digest + changed files
if [ -f .evolve/project-digest.md ]; then
  MODE="incremental"
  DIGEST=$(cat .evolve/project-digest.md)
  CHANGED=$(git diff HEAD~1 --name-only 2>/dev/null)
elif [ -f $WORKSPACE_PATH/project-digest.md ]; then
  MODE="incremental"
  DIGEST=$(cat $WORKSPACE_PATH/project-digest.md)
  CHANGED=$(git diff HEAD~1 --name-only 2>/dev/null)
else
  MODE="full"
fi

RECENT_NOTES=$(# extract last 5 "## Cycle" sections from notes.md)
BUILDER_NOTES=$(cat $WORKSPACE_PATH/builder-notes.md 2>/dev/null || cat .evolve/workspace/builder-notes.md 2>/dev/null || echo "")
RECENT_LEDGER=$(tail -3 .evolve/ledger.jsonl)
# instinctSummary and ledgerSummary come from state.json (already read)
```

**Shared values in agent context:** Include Layer 0 core rules from `memory-protocol.md` AND `sharedValues` from `SKILL.md` at the top of all agent context blocks. Place shared values first to maximize KV-cache reuse across parallel agent launches.

**Prompt caching (provider-specific):**

| Provider | Implementation |
|----------|---------------|
| Anthropic API | `cache_control: {"type": "ephemeral"}` on last static block |
| Google Gemini | `cachedContent` resource for static prefix |
| OpenAI | `store: true` on static prefix messages |
| Generic / self-hosted | Place static context first in system prompt |

Key principle: **static fields first, dynamic fields last** — reduces prompt-processing cost by 20-40%.

**Operator brief pre-read:** Check `$WORKSPACE_PATH/next-cycle-brief.json` (own run). Fall back to `.evolve/latest-brief.json` (shared). Pass contents to Scout context if present.

**Launch Scout Agent** (model: tier-1 if cycle 1 or goal-directed cycle <= 2; tier-3 if cycle 4+ with mature bandit data; tier-2 otherwise):
- **Platform dispatch:** Claude Code: `Agent` tool with `subagent_type: "general-purpose"`; Gemini CLI: `spawn_agent`; Generic: new LLM session.
- Prompt: Read `agents/evolve-scout.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static (stable across cycles, maximizes cache reuse) ---
    "projectContext": "<auto-detected>",
    "projectDigest": "<contents of project-digest.md, or null if cycle 1>",
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "goal": "<goal or null>",
    "strategy": "<strategy>",
    // --- Semi-stable (changes slowly) ---
    "instinctSummary": "<from state.json, inline>",
    "stateJson": "<state.json contents — evalHistory trimmed to last 5>",
    // --- Dynamic (changes every cycle) ---
    "cycle": "<N>",
    "mode": "full|incremental|convergence-confirmation",
    "changedFiles": ["<output of git diff HEAD~1 --name-only>"],
    "recentNotes": "<last 5 cycle entries from notes.md>",
    "builderNotes": "<contents of builder-notes.md, or empty>",
    "recentLedger": "<last 3 ledger entries>",
    "benchmarkWeaknesses": "<array of {dimension, score, taskTypeHint}, or empty>",
    "challengeToken": "<$CHALLENGE>",
    "handoffFromOperator": "<contents of handoff-operator.json, or null>"
  }
  ```

Scout MUST write all output files to `$WORKSPACE_PATH`, NOT `.evolve/workspace/`.

**Implementation requirement:** Scout MUST identify target files for modification. Tasks that only create new documentation files are deprioritized unless:
- `projectContext.domain` is `"writing"` or `"research"`
- The goal explicitly requests documentation
- No existing files are suitable for the research finding

When research is performed, Scout writes a `Research → Implementation Map` in the scout-report showing how each finding translates to file changes.

**After Scout completes:**
- Verify `$WORKSPACE_PATH/scout-report.md` exists. If not, check `.evolve/workspace/scout-report.md` and copy.
- Read `$WORKSPACE_PATH/scout-report.md`

**Task Claiming (parallel deduplication):**
1. Read `state.json.evaluatedTasks` (note version V)
2. Filter out tasks with `decision: "selected"` or `"completed"`
3. Write remaining tasks with `decision: "selected"`, `cycle: N`, `runId: $RUN_ID`
4. Write state.json with `version = V + 1`, verify via OCC
5. If conflict → re-read, re-filter, retry (max 3)
6. Only build successfully claimed tasks

**Prerequisite check:** For each task with a `prerequisites` field, verify all listed slugs are `decision: "completed"` in `evaluatedTasks`. Unmet prerequisites → auto-defer with `deferralReason`. Tasks without `prerequisites` are unaffected.

**Eval quality check** (run on THIS cycle's eval files only):
```bash
for TASK_SLUG in <task slugs from scout-report>; do
  bash scripts/eval-quality-check.sh .evolve/evals/${TASK_SLUG}.md
  EVAL_QUALITY_EXIT=$?
  if [ "$EVAL_QUALITY_EXIT" -eq 2 ]; then
    echo "HALT: Level 0 (no-op) commands in ${TASK_SLUG}. Scout must rewrite evals."
  elif [ "$EVAL_QUALITY_EXIT" -eq 1 ]; then
    echo "WARN: Level 1 (tautological) commands in ${TASK_SLUG}. Flagging for Auditor."
  fi
done
```

**Eval checksum capture:**
```bash
sha256sum .evolve/evals/*.md > $WORKSPACE_PATH/eval-checksums.json
```

**If no tasks selected:** increment `stagnation.nothingToDoCount`. If >= 3 → STOP: "Project has converged." Otherwise → skip to Phase 5.

**Stagnation detection** (run after every Scout phase):

| Pattern | Trigger | Action |
|---------|---------|--------|
| Same-file churn | Same files in `failedApproaches` across 2+ cycles | Flag, pass to Scout as avoidance context |
| Same-error repeat | Same error message recurs across cycles | Flag with alternative approach suggestion |
| Diminishing returns | Last 3 cycles each shipped fewer tasks | Flag as diminishing returns |

If 3+ stagnation patterns active simultaneously → trigger Operator HALT.

### Context Quality Checklist (CEMM)

Based on the Context Engineering Maturity Model (arXiv:2603.09619), apply five criteria to every context block before passing to Scout:

| Criterion | Guiding Question |
|-----------|-----------------|
| **Relevance** | Is this context needed by the current phase? |
| **Sufficiency** | Does the context have enough info for the agent to complete its task? |
| **Isolation** | Is this phase's context independent from other phases' internal state? |
| **Economy** | Are we sending the minimum tokens needed? |
| **Provenance** | Can we trace where each piece of context came from? |

Log violations in build report under **Risks** and flag to Auditor.

### Per-Phase Context Selection Matrix

Each agent receives ONLY the fields it needs (Anthropic's **Select** strategy).

| Field | Scout | Builder | Auditor | Operator |
|-------|:-----:|:-------:|:-------:|:--------:|
| `cycle` | Y | Y | Y | Y |
| `strategy` | Y | Y | Y | — |
| `challengeToken` | Y | Y | Y | — |
| `budgetRemaining` | Y | Y | — | Y |
| `instinctSummary` | Y | Y | — | — |
| `stateJson` (full) | Y | — | — | Y |
| `projectContext` | Y | — | — | — |
| `projectDigest` | Y | — | — | — |
| `changedFiles` | Y | — | — | — |
| `recentNotes` | Y | — | — | Y |
| `benchmarkWeaknesses` | Y | — | — | — |
| `task` (from scout-report) | — | Y | — | — |
| `buildReport` | — | — | Y | — |
| `auditorProfile` | — | — | Y | — |
| `recentLedger` | Y | — | Y | Y |

Builder does NOT need stateJson, projectDigest, benchmarkWeaknesses, or recentNotes. Auditor does NOT need instinctSummary or budgetRemaining. Saves ~3-5K tokens per agent invocation.

### Inter-Phase Handoff Format

Each phase writes `$WORKSPACE_PATH/handoff-<phase>.json` for the next phase.

```json
{
  "phase": "<scout|builder|auditor|ship>",
  "cycle": "<N>",
  "findings": "<1-3 sentence summary>",
  "decisions": ["<key decision 1>", "<key decision 2>"],
  "files_modified": ["<path/to/file1>"],
  "next_phase_context": { "<key>": "<value>" }
}
```

| File | Written by | Read by | Key fields |
|------|-----------|---------|------------|
| `handoff-scout.json` | Scout | Builder | `findings` (rationale), `files_modified`, `next_phase_context.taskSlug` |
| `handoff-builder.json` | Builder | Auditor | `findings` (approach), `files_modified`, `decisions`, `next_phase_context.worktreeBranch` |
| `handoff-auditor.json` | Auditor | Orchestrator (Ship) | `findings` (verdict), `decisions` (issues), `next_phase_context.verdict` |

If handoff file absent or malformed, fall back to full report (e.g., `scout-report.md`, `build-report.md`).

---

### Phase Boundary: DISCOVER -> BUILD

```bash
bash scripts/phase-gate.sh discover-to-build $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to BUILD. Any other exit -> HALT.
```

### Phase 2: BUILD (loop per task)

For detailed Phase 2 implementation, see [phase2-build.md](skills/evolve-loop/phase2-build.md).

| Step | Detail |
|------|--------|
| Inline S-tasks | Execute first (per inst-007), commit sequentially |
| Independent worktree tasks | Build in parallel via platform agent mechanism |
| Dependent tasks | Build sequentially within groups (shared files) |
| Isolation | Builder MUST use worktree isolation for coding projects |
| Retries | Max 3 attempts per task; failures logged to state.json |

---

### Phase Boundary: BUILD -> AUDIT

```bash
bash scripts/phase-gate.sh build-to-audit $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to AUDIT. Any other exit -> HALT.
```

### Phase 3: AUDIT (Parallel)

The Auditor reviews Builder changes **in the worktree**. Worktree tasks built in parallel during Phase 2 MUST also be audited in parallel.

**Inline task audit:** Run eval graders before committing. If any grader fails, revert (`git checkout -- <files>`) and retry inline or escalate to Builder agent. Do NOT skip audit for inline tasks.

**Eval checksum verification** (before launching Auditor):
```bash
sha256sum -c $WORKSPACE_PATH/eval-checksums.json
```
If any checksum fails → HALT: "Eval tamper detected."

**Launch Auditor Agent** (model: tier-2 default, tier-1 for security-sensitive, tier-3 for clean builds):
- Prompt: Read `agents/evolve-auditor.md` and pass as prompt
- Context:
  ```json
  {
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "evalsPath": ".evolve/evals/",
    "strategy": "<strategy>",
    "auditorProfile": "<state.json auditorProfile object>",
    "cycle": "<N>",
    "buildReport": "<$WORKSPACE_PATH>/build-report.md",
    "recentLedger": "<last 3 ledger entries>",
    "challengeToken": "<$CHALLENGE>",
    "forceFullAudit": "<$FORCE_FULL_AUDIT>",
    "handoffFromBuilder": "<contents of handoff-builder.json>"
  }
  ```

After Auditor completes, handle verdict per [phase2-build.md](skills/evolve-loop/phase2-build.md) (verdict handling, worktree merge/cleanup, auditor profile updates).

---

### Benchmark Delta Check & Pre-Ship Integrity Gate

For detailed steps, see [phase2-build.md](skills/evolve-loop/phase2-build.md).

| Check | Detail |
|-------|--------|
| Exemptions | Skip for `repair` strategy, first 3 cycles, or `meta`/`infrastructure` tasks |
| Regression | Any dimension -3 or more blocks shipping; 1 retry, then drop |
| Pre-ship gate | `verify-eval.sh` + `cycle-health-check.sh` run independently of all agents |
| Proceed | Only if both health check and eval verification pass |

```bash
bash scripts/phase-gate.sh audit-to-ship $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to SHIP. Exit 1 -> block. Exit 2 -> HALT for human.
```

This is the **primary anti-cheating gate**: runs `verify-eval.sh` independently, runs `cycle-health-check.sh` (11 signals), verifies eval checksums.

### Phase 4: SHIP

For detailed instructions, see [phase4-ship.md](skills/evolve-loop/phase4-ship.md).

```bash
bash scripts/phase-gate.sh ship-to-learn $CYCLE $WORKSPACE_PATH
```
Verifies git is clean and updates `state.json.lastCycleNumber` (the SCRIPT does this, not the LLM).

---

### Phase 5: LEARN

For detailed instructions, see [phase5-learn.md](skills/evolve-loop/phase5-learn.md).

```bash
bash scripts/phase-gate.sh cycle-complete $CYCLE $WORKSPACE_PATH
```
Archives workspace, verifies all 3 reports exist, updates mastery ONLY if audit-report.md contains a genuine PASS verdict.
