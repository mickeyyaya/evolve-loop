# Evolve Loop — Phase Instructions

Detailed orchestrator instructions for each phase. Optimized for fast iteration with diverse small/medium tasks per cycle.

**Important:** Every agent context block must include `goal` (string or null) and `strategy` (one of: `balanced`, `innovate`, `harden`, `repair`, `ultrathink`).

## Phase 0: CALIBRATE (once per invocation)

Runs **once per `/evolve-loop` invocation**, not per cycle. Executes before the first Scout runs. Establishes a project-level benchmark baseline so tasks can be measured against project quality, not just process quality.

### Calibration Deduplication

Before running calibration, check if a recent calibration already exists:
```
if state.json.projectBenchmark.lastCalibrated exists
   AND (now - lastCalibrated) < 1 hour:
   Skip CALIBRATE, use existing benchmark
else:
   Run CALIBRATE normally
```
This prevents redundant benchmark scoring when multiple parallel runs start close together.

### Execution Steps

1. **Run automated checks** from [benchmark-eval.md](skills/evolve-loop/benchmark-eval.md):
   Execute all bash check commands for each of the 8 dimensions. Capture per-dimension automated scores (0-100).

   ```bash
   # Run each dimension's automated checks from benchmark-eval.md
   # Store results in $WORKSPACE_PATH/benchmark-automated.json
   ```

2. **Run LLM judgment pass** (model: per routing table — tier-2 for first calibration of session to establish accurate baseline, tier-3 for subsequent calibrations):
   For each dimension, provide the LLM with:
   - The dimension's rubric from benchmark-eval.md
   - A sample of relevant files (max 3 files per dimension, <200 lines each)
   - The automated score for context

   The LLM outputs a score (0/25/50/75/100) with a 1-sentence justification and a confidence score (0.0-1.0). Use the anchored rubric — scores MUST match one of the anchor points exactly. You MUST actively resist "verbosity bias" (assuming longer files are better) and score strictly on qualitative merit.

3. **Compute per-dimension composite scores:**
   ```
   dimension.composite = round(0.7 * dimension.automated + 0.3 * dimension.llm)
   ```

4. **Compute overall score:**
   ```
   overall = round(mean(all 8 dimension composites), 1)
   ```

5. **Store in state.json** under `projectBenchmark`:
   ```json
   {
     "projectBenchmark": {
       "lastCalibrated": "<ISO-8601>",
       "calibrationCycle": <lastCycleNumber + 1>,
       "overall": <0-100>,
       "dimensions": {
         "documentationCompleteness": {"automated": <N>, "llm": <N>, "composite": <N>},
         "specificationConsistency": {"automated": <N>, "llm": <N>, "composite": <N>},
         "defensiveDesign": {"automated": <N>, "llm": <N>, "composite": <N>},
         "evalInfrastructure": {"automated": <N>, "llm": <N>, "composite": <N>},
         "modularity": {"automated": <N>, "llm": <N>, "composite": <N>},
         "schemaHygiene": {"automated": <N>, "llm": <N>, "composite": <N>},
         "conventionAdherence": {"automated": <N>, "llm": <N>, "composite": <N>},
         "featureCoverage": {"automated": <N>, "llm": <N>, "composite": <N>}
       },
       "history": [],
       "highWaterMarks": {}
     }
   }
   ```

6. **Compare to previous calibration** (if `projectBenchmark.history` is non-empty):
   - Append the previous calibration to `history` (keep last 5 entries)
   - Identify the 2-3 dimensions with the lowest composite scores → these are `benchmarkWeaknesses`
   - **High-water mark tracking:** For each dimension at 80+, record in `highWaterMarks`. If any dimension regresses below `(HWM - 10)`, add a mandatory remediation task to `pendingImprovements`

7. **Write `$WORKSPACE_PATH/benchmark-report.md`:**
   ```markdown
   # Project Benchmark — Calibration at Cycle {lastCycleNumber + 1}

   ## Overall Score: {overall}/100

   | Dimension | Automated | LLM | Composite | Delta |
   |-----------|-----------|-----|-----------|-------|
   | Documentation Completeness | X | X | X | +/-N |
   | Specification Consistency | X | X | X | +/-N |
   | ... | ... | ... | ... | ... |

   ## Weakest Dimensions
   1. <dimension> (score: X) — <1-sentence diagnosis>
   2. <dimension> (score: X) — <1-sentence diagnosis>

   ## High-Water Mark Regressions
   - <dimension>: current X, HWM Y (REMEDIATION REQUIRED)
   ```

8. **Pass `benchmarkWeaknesses` to Scout context** — an array of `{dimension, score, taskTypeHint}` objects derived from the weakest dimensions and the dimension-to-task-type mapping in benchmark-eval.md.

### Benchmark Eval Checksum

Compute and store the checksum of `benchmark-eval.md` during Phase 0:
```bash
sha256sum skills/evolve-loop/benchmark-eval.md > $WORKSPACE_PATH/benchmark-eval-checksum.txt
```
Verify this checksum before every delta check (Phase 3→4 boundary). Builder MUST NOT modify this file.

---

## FOR each cycle (while remainingCycles > 0):

### Lean Mode (cycles 4+ of an invocation)

After the first 3 cycles of an invocation, the orchestrator switches to lean mode to prevent context bloat:
- **State.json**: Read ONCE at cycle start. Do NOT re-read before Phase 4 — use the in-memory copy.
- **Agent results**: Do NOT read full workspace files after agent completion. Instead, the agent's returned summary is sufficient. Only read the full file if the summary indicates a problem.
- **Scout report**: Extract task list from agent return value, not by reading scout-report.md separately.
- **Eval checksums**: Compute once, verify from memory — do not re-read the checksum file.
- **Benchmark delta**: Skip for cycles where all tasks are S-complexity docs-only changes (no behavioral risk).

Lean mode saves ~15-20K tokens per cycle (from ~50K to ~30K) by eliminating redundant file reads that accumulate in the orchestrator's conversation context.

### Cycle Integrity Setup

At the start of each cycle, before any agent is invoked, establish the integrity layer:

1. **Generate challenge token** (proof-of-invocation chain):
   ```bash
   CHALLENGE=$(openssl rand -hex 8 2>/dev/null || head -c 8 /dev/urandom | xxd -p)
   ```
   This token is passed to every agent in their context block. Each agent embeds it in their workspace output files and ledger entries. The `scripts/cycle-health-check.sh` script verifies consistency.

2. **Plant canary files** (honeypot detection — research basis: Canarytokens):
   ```bash
   # Workspace canary: random value that no legitimate agent should touch
   CANARY_VALUE=$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary"
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary-expected"

   # Honeypot eval: fake eval file that should never be referenced
   if [ ! -f ".evolve/evals/_canary-eval.md" ]; then
     cat > ".evolve/evals/_canary-eval.md" << 'CANARY'
   # Canary Eval (DO NOT REFERENCE)
   This file monitors for eval system probing. Any access = anomaly.
   ## Graders
   - `echo "canary-sentinel-$(date +%s)"`
   CANARY
   fi
   ```

3. **Adaptive strictness decay** (cross-session — research basis: "Alignment Faking", Anthropic):
   If this is the first cycle of a new invocation, halve all `consecutiveClean` values in `auditorProfile`:
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
   Pass `forceFullAudit` in the Auditor context. Do NOT log whether this was triggered — blind monitoring means the agent should not know which cycles are scrutinized.

5. **Hash chain initialization** (tamper-evident ledger — research basis: SLSA transparency logs, in-toto):
   Compute the hash of the last ledger entry to seed this cycle's chain:
   ```bash
   if command -v sha256sum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | sha256sum | cut -d' ' -f1)
   elif command -v shasum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | shasum -a 256 | cut -d' ' -f1)
   else
     PREV_HASH="unsupported"
   fi
   ```
   Every ledger entry written during this cycle MUST include `"prevHash":"<$PREV_HASH>"` in its `data` object. After writing each entry, update `PREV_HASH` to the hash of the entry just written. This creates a Merkle chain where inserting, deleting, or modifying entries retroactively breaks the chain. The `scripts/cycle-health-check.sh` verifies chain integrity.

### Atomic Cycle Number Allocation

At the start of each cycle iteration, claim the next cycle number atomically:

1. Read `state.json`, note `version = V` and `lastCycleNumber`
2. `claimedCycle = lastCycleNumber + 1`
3. Write `state.json` with `lastCycleNumber = claimedCycle`, `version = V + 1`
4. Immediately re-read `state.json` and verify `version == V + 1`
5. If version mismatch (another run claimed first) → re-read, re-claim next available number
6. Use `claimedCycle` as this iteration's cycle number `N`
7. Decrement `remainingCycles`

This ensures parallel runs get non-colliding cycle numbers (e.g., Run A gets 8,9 while Run B gets 10,11).

### Phase 1: DISCOVER

**Convergence Short-Circuit** (check BEFORE launching Scout):
- Read `stagnation.nothingToDoCount` from state.json:
  - If `>= 2`: Skip Scout entirely. Jump to Phase 5 with Operator in `"convergence-check"` mode. Operator can reset `nothingToDoCount` to 0 if it detects new work (e.g., external changes via `git log`).
  - If `== 1`: **Escalation before convergence** — before running convergence-confirmation, the orchestrator attempts to unlock new work:
    1. Review the last 3 cycles' deferred tasks (from `evaluatedTasks` with `decision: "deferred"`) for items that could be combined into a single task
    2. Check if switching strategy (e.g., `balanced` → `innovate`, `harden`, or `ultrathink`) would surface new task candidates
    3. Propose a "radical" task that challenges an existing assumption or convention in the codebase (think harder — re-read code for new angles)
    If escalation produces a viable task → reset `nothingToDoCount` to 0, proceed with normal Scout launch using the escalated task as a seed.
    If escalation fails → Launch Scout in `"convergence-confirmation"` mode — Scout reads ONLY state.json + `git log --oneline -3` and MUST trigger new web research to find potential external tasks/updates, bypassing any cooldowns. No notes, no ledger, no instincts, no codebase scan. If still nothing found, increment to 2 and skip to Phase 5.
  - If `== 0`: Proceed with normal Scout launch below.

**Pre-compute context** (orchestrator reads files once, passes inline slices):
```bash
# Cycle 1: full mode — no digest exists yet
# Cycle 2+: incremental mode — read digest + changed files
# Check shared digest first, then run-local
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

# Read recent notes (last 5 cycle entries, not full file)
RECENT_NOTES=$(# extract last 5 "## Cycle" sections from notes.md)

# Read builder notes: own run first, then shared fallback
BUILDER_NOTES=$(cat $WORKSPACE_PATH/builder-notes.md 2>/dev/null || cat .evolve/workspace/builder-notes.md 2>/dev/null || echo "")

# Read recent ledger (last 3 lines)
RECENT_LEDGER=$(tail -3 .evolve/ledger.jsonl)

# instinctSummary and ledgerSummary come from state.json (already read)
```

**Shared values in agent context:** The Layer 0 core rules from `memory-protocol.md` must be included at the top of all agent context blocks. Placing shared values first maximizes KV-cache reuse across parallel agent launches — concurrent builders or auditors that share the same static prefix benefit from prompt cache hits without re-encoding the rules.

**Prompt caching with Anthropic APIs:** When invoking agents via the Anthropic API directly, you can activate server-side prompt caching by marking the static prefix with a cache breakpoint. Only the fields that are stable across cycles need the marker — dynamic per-cycle fields must NOT carry it, because adding `cache_control` to a changing field invalidates the cache every cycle and eliminates any savings.

Apply `cache_control: {"type": "ephemeral"}` to the last static block in the context, which is typically the end of `projectContext` (Layer 0 shared values). Example structure:

```json
[
  {"role": "user", "content": [
    {"type": "text", "text": "<Layer 0 rules + projectContext>",
     "cache_control": {"type": "ephemeral"}},
    {"type": "text", "text": "<instinctSummary — semi-stable, no cache marker>"},
    {"type": "text", "text": "<cycle-specific fields: cycle N, changedFiles, recentNotes>"}
  ]}
]
```

The `ephemeral` breakpoint tells the API to cache all tokens up to and including that block. Subsequent agent calls that share the identical static prefix will get a cache hit, reducing prompt-processing cost by 20-40% on repeated context. See `docs/token-optimization.md` § KV-Cache Prefix Optimization for background on prefix placement strategy.

**Operator brief pre-read:** Before launching Scout, check if `$WORKSPACE_PATH/next-cycle-brief.json` exists (from own run's previous cycle). If not found, fall back to `.evolve/latest-brief.json` (shared, written by most recent run). If present, pass its contents in the Scout context so Scout can apply `recommendedStrategy`, `taskTypeBoosts`, and `avoidAreas` during task selection.

Launch **Scout Agent** (model: per routing table — tier-1 if cycle 1 or goal-directed cycle ≤ 2 (strategic foundation sets trajectory for entire session), tier-3 if cycle 4+ with mature bandit data (3+ arms with pulls ≥ 3, selection becomes data-driven), tier-2 otherwise; subagent_type: `general-purpose`):
- Prompt: Read `agents/evolve-scout.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static (stable across cycles, maximizes cache-like reuse) ---
    "projectContext": <auto-detected>,
    "projectDigest": "<contents of project-digest.md, or null if cycle 1>",
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "goal": <goal or null>,
    "strategy": <strategy>,
    // --- Semi-stable (changes slowly, every few cycles) ---
    "instinctSummary": "<from state.json, inline>",
    "stateJson": <state.json contents — evalHistory trimmed to last 5 entries>,
    // --- Dynamic (changes every cycle) ---
    "cycle": <N>,
    "mode": "full|incremental|convergence-confirmation",
    "changedFiles": ["<output of git diff HEAD~1 --name-only>"],
    "recentNotes": "<last 5 cycle entries from notes.md, inline>",
    "builderNotes": "<contents of builder-notes.md from last cycle, or empty string>",
    "recentLedger": "<last 3 ledger entries, inline>",
    "benchmarkWeaknesses": "<array of {dimension, score, taskTypeHint} from Phase 0, or empty>",
    "challengeToken": "<$CHALLENGE>",
    "handoffFromOperator": "<contents of handoff-operator.json from previous cycle, or null>"
  }
  ```

**IMPORTANT:** The Scout MUST write all output files (scout-report.md, cycle summaries) to `$WORKSPACE_PATH`, NOT to `.evolve/workspace/`. The `workspacePath` is passed in the context block above. If the Scout writes to the wrong location, copy the files to `$WORKSPACE_PATH` before proceeding.

After Scout completes:
- Verify `$WORKSPACE_PATH/scout-report.md` exists. If not, check `.evolve/workspace/scout-report.md` and copy it to `$WORKSPACE_PATH`.
- Read `$WORKSPACE_PATH/scout-report.md`

- **Task Claiming (parallel deduplication):**
  Before building, claim each selected task via OCC protocol to prevent two parallel runs from building the same task:
  1. Read `state.json.evaluatedTasks` (note version V)
  2. Filter out any task whose slug already has `decision: "selected"` or `decision: "completed"`
  3. Write remaining tasks to `evaluatedTasks` with `decision: "selected"`, `cycle: N`, `runId: $RUN_ID`
  4. Write state.json with `version = V + 1`, verify via OCC protocol
  5. If conflict → re-read, re-filter, retry (max 3 retries)
  6. Only build tasks that were successfully claimed

- **Prerequisite check:** For each proposed task that includes a `prerequisites` field, verify all listed slugs appear in `state.json.evaluatedTasks` with `decision: "completed"`. Any task with an unmet prerequisite is automatically deferred: add it to `evaluatedTasks` with `decision: "deferred"` and `deferralReason: "prerequisite not met: <slug>"`, then log the prerequisite slug so the Scout can propose it in the next cycle. Tasks without a `prerequisites` field are unaffected. This check is a lightweight sequencing aid — the Scout may override it by omitting `prerequisites` when a task is genuinely independent of its nominal dependency.
- Verify eval definitions were created in `.evolve/evals/`
- **Eval quality check** (deterministic rigor classification — research basis: cycle 101 tautological eval incident):
  Run only on eval files created for THIS cycle's tasks (not the entire evals directory):
  ```bash
  # Check each task's eval file individually
  for TASK_SLUG in <task slugs from scout-report>; do
    bash scripts/eval-quality-check.sh .evolve/evals/${TASK_SLUG}.md
    EVAL_QUALITY_EXIT=$?
    if [ "$EVAL_QUALITY_EXIT" -eq 2 ]; then
      echo "HALT: Eval quality check found Level 0 (no-op) commands in ${TASK_SLUG}. Scout must rewrite evals."
      # Log to ledger and halt cycle
    elif [ "$EVAL_QUALITY_EXIT" -eq 1 ]; then
      echo "WARN: Eval quality check found Level 1 (tautological) commands in ${TASK_SLUG}. Flagging for Auditor."
      # Pass warnings to Auditor context for deeper review
    fi
  done
  ```
  This catches the exact attack from cycle 101 (`grep -q "string" source.js`) deterministically.
- **Eval checksum capture:** Compute `sha256sum` of each eval file in `.evolve/evals/` and store in `workspace/eval-checksums.json`:
  ```bash
  sha256sum .evolve/evals/*.md > $WORKSPACE_PATH/eval-checksums.json
  ```
  These checksums are verified before Auditor runs evals (Phase 3) to detect tampering.
- Merge research query updates into state.json (if research was performed)
- If no tasks selected:
  - Increment `stagnation.nothingToDoCount` in state.json
  - If `stagnation.nothingToDoCount >= 3` → STOP: "Project has converged."
  - Otherwise → skip to Phase 5

- **Stagnation detection** (run after every Scout phase):
  Check `stagnation.recentPatterns` in state.json for repeated failure patterns:
  1. **Same-file churn** — if the same file(s) appear in `failedApproaches` across 2+ consecutive cycles → flag as stagnation
  2. **Same-error repeat** — if the same error message recurs across cycles → flag with suggestion to try alternative approach
  3. **Diminishing returns** — if the last 3 cycles each shipped fewer tasks than the previous → flag as diminishing returns

  When stagnation is detected, the orchestrator should:
  - Log the pattern in `stagnation.recentPatterns` with type and cycle range
  - Pass it to the Scout as context so it avoids the stagnant area
  - If 3+ stagnation patterns are active simultaneously → trigger Operator HALT

### Context Quality Checklist (CEMM)

Based on the Context Engineering Maturity Model (arXiv:2603.09619), apply these five criteria to every context block assembled in Phase 1 before passing it to the Scout:

1. **Relevance** — Is this context needed by the current phase? _(e.g., do not send the full benchmark report to Scout when Scout only needs `benchmarkWeaknesses`)_
2. **Sufficiency** — Does the context have enough info for the agent to complete its task? _(e.g., Scout needs `recentNotes` + `changedFiles` + `stateJson` to propose meaningful tasks — omitting any one risks shallow proposals)_
3. **Isolation** — Is this phase's context independent from other phases' internal state? _(e.g., Scout context must NOT include Builder worktree paths or Auditor verdicts from the same cycle — those are internal to later phases)_
4. **Economy** — Are we sending the minimum tokens needed? _(e.g., pass `recentLedger` as last 3 lines, not the full ledger; pass instinct compact summary, not raw YAML files)_
5. **Provenance** — Can we trace where each piece of context came from? _(e.g., `builderNotes` is sourced from `$WORKSPACE_PATH/builder-notes.md`, falling back to `.evolve/workspace/builder-notes.md` — always document the source path)_

Violations should be logged in the build report under **Risks** and flagged to the Auditor.

### Inter-Phase Handoff Format

Each phase writes a structured handoff file to `$WORKSPACE_PATH/handoff-<phase>.json` for the next phase to consume. This compact contract eliminates redundant file re-reads across phase boundaries, targeting 40-60% token savings on cross-phase context.

**Schema (`phaseHandoff`):**
```json
{
  "phase": "<scout|builder|auditor|ship>",
  "cycle": <N>,
  "findings": "<1-3 sentence summary of what this phase discovered or verified>",
  "decisions": ["<key decision 1>", "<key decision 2>"],
  "files_modified": ["<path/to/file1>", "<path/to/file2>"],
  "next_phase_context": {
    "<key>": "<value>"
  }
}
```

**File naming convention:** `handoff-<phase>.json` where `<phase>` is the writing phase name (e.g., `handoff-scout.json`, `handoff-builder.json`, `handoff-auditor.json`).

**Ownership table:**

| File | Written by | Read by | Key fields passed |
|------|-----------|---------|-------------------|
| `handoff-scout.json` | Scout | Builder | `findings` (task rationale), `files_modified` (files to touch), `next_phase_context.taskSlug` |
| `handoff-builder.json` | Builder | Auditor | `findings` (approach summary), `files_modified` (changed files list), `decisions` (design choices), `next_phase_context.worktreeBranch` |
| `handoff-auditor.json` | Auditor | Orchestrator (Ship) | `findings` (audit verdict summary), `decisions` (issues found/waived), `next_phase_context.verdict` |

**Usage:** The receiving phase reads the handoff file instead of re-reading the full workspace report. If the handoff file is absent or malformed, fall back to reading the full report (e.g., `scout-report.md`, `build-report.md`).

---

### Phase Boundary: DISCOVER → BUILD

Before entering Phase 2, run precondition assertions:
```bash
# Verify Scout produced substantive output
[ -s "$WORKSPACE_PATH/scout-report.md" ] || { echo "HALT: Scout report missing or empty — Phase 1 may have been skipped"; exit 1; }
```

### Phase 2: BUILD (loop per task)

**Task Execution Ordering & Parallelization:** Before starting the build loop, partition the task list into two groups:
1. **Inline tasks** — S-complexity tasks eligible for orchestrator inline execution (per inst-007 policy)
2. **Worktree tasks** — all other tasks requiring Builder agent in isolated worktree

**Execute all inline tasks first sequentially, committing each before proceeding.** Then execute all worktree tasks **in parallel**. Parallel execution of worktree tasks drastically reduces the latency of the build phase. Because each task is isolated in its own worktree, they can safely be built concurrently.

For each worktree task (running in parallel):

#### Build Isolation (by `projectContext.buildIsolation`)

**`worktree` (default — coding domain):**

**Worktree isolation is MANDATORY for coding projects.** The orchestrator MUST launch the Builder with `isolation: "worktree"` so it operates on an isolated copy of the repository. This prevents:
- Builder changes from interfering with the main working tree during execution
- Multiple Builder runs (if parallelized in the future) from conflicting with each other
- Partial/failed changes from polluting the main branch

**NEVER launch the Builder without isolation.** If the Agent tool does not support worktree isolation, the orchestrator MUST manually create a worktree before launching the Builder:
```bash
WORKTREE_DIR=$(mktemp -d)/evolve-build-cycle-<N>-<task-slug>
git worktree add "$WORKTREE_DIR" HEAD
# Launch Builder with cwd set to $WORKTREE_DIR
# After Builder completes, merge changes back:
cd "$WORKTREE_DIR" && git diff HEAD > /tmp/builder.patch
cd <main-repo> && git apply /tmp/builder.patch
git worktree remove "$WORKTREE_DIR"
```

**`file-copy` (writing, research, design domains):**

For non-git projects (or projects without git worktree support), use file-copy isolation:
```bash
COPY_DIR=$(mktemp -d)/evolve-build-cycle-<N>-<task-slug>
# Copy project files (exclude .evolve/ runtime state)
cp -rp . "$COPY_DIR" && rm -rf "$COPY_DIR/.evolve"
# Launch Builder with cwd set to $COPY_DIR
# After Builder completes, diff and apply changes back:
diff -rq "$COPY_DIR" . --exclude='.evolve' --exclude='.git' > /tmp/changed-files.txt
# For each changed file, copy from COPY_DIR back to main:
while read line; do
  FILE=$(echo "$line" | grep -oP '(?<=differ: ).*(?= and )' || echo "$line" | grep -oP '(?<=Only in '"$COPY_DIR"'/: ).*')
  [ -n "$FILE" ] && cp "$COPY_DIR/$FILE" "./$FILE" 2>/dev/null
done < /tmp/changed-files.txt
rm -rf "$COPY_DIR"
```

**`none` (explicit opt-out — use with caution):**
Builder operates directly on the working directory. Only suitable for append-only tasks where isolation provides no benefit (e.g., writing a new standalone document). The orchestrator should warn when this mode is active.

The orchestrator selects the isolation mode from `projectContext.buildIsolation` (set during initialization step 3). Worktree remains the default fallback if `buildIsolation` is not specified.

Launch **Builder Agent** (model: per routing table — tier-3 if S-complexity + plan cache hit (execution-only, plan is proven), tier-1 if strategy == "ultrathink" OR M-complexity + 5+ files OR audit retry attempt ≥ 2 (design mistake needs deeper reasoning about WHY it failed), tier-2 if strategy == "repair" (accuracy floor), tier-2 otherwise; subagent_type: `general-purpose`, **isolation: `worktree`**):
- Prompt: Read `agents/evolve-builder.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static ---
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "evalsPath": ".evolve/evals/",
    "strategy": <strategy>,
    // --- Semi-stable ---
    "instinctSummary": "<from state.json, inline>",
    // --- Dynamic ---
    "cycle": <N>,
    "task": <task object from scout-report — includes inline eval graders>,
    "challengeToken": "<$CHALLENGE>",
    "handoffFromScout": "<contents of handoff-scout.json>"
  }
  ```
- **Note:** Builder reads eval acceptance criteria from the task object in scout-report.md (inline `Eval Graders` field) instead of reading separate eval files. Builder still reads full eval files from `evalsPath` only if inline graders are missing.

**Output Redirection:** When Builder runs eval graders, test commands, or build commands, redirect stdout/stderr to `$WORKSPACE_PATH/run.log`:
```bash
<command> > $WORKSPACE_PATH/run.log 2>&1
```
Builder and Auditor extract results via `grep`/`tail` on `run.log` rather than reading full output. This reduces token consumption by 30-50% for verbose build/test output.

**Experiment Journal:** After each Builder attempt (pass or fail), append a one-line entry to `$WORKSPACE_PATH/experiments.jsonl`:
```jsonl
{"cycle":N,"task":"<slug>","attempt":1,"verdict":"PASS|FAIL","approach":"<1-sentence summary>","metric":"<eval result or error>"}
```
This append-only log ensures every attempt is recorded. Scout reads `experiments.jsonl` to avoid re-proposing similar approaches that already failed.

After Builder completes:
- Read `$WORKSPACE_PATH/build-report.md`
- If status is FAIL after 3 attempts:
  - **Discard worktree** — the worktree branch contains failed changes, clean it up:
    ```bash
    # If the Agent tool created the worktree, it auto-cleans on failure
    # If manual worktree, remove it:
    git worktree remove "$WORKTREE_DIR" --force 2>/dev/null
    ```
  - Log failed approach in state.json under `failedApproaches` with structured reasoning:
    ```json
    {
      "feature": "<task name>",
      "approach": "<what was tried>",
      "error": "<error message or symptom>",
      "reasoning": "<WHY it failed — root cause analysis, not just the error>",
      "filesAffected": ["<files that were involved>"],
      "cycle": <N>,
      "alternative": "<suggested different approach for next cycle>"
    }
    ```
  - Skip this task, proceed to next task (or Phase 3 if last task)
- If status is PASS → proceed to Phase 3 for this task
  - **Do NOT merge yet** — worktree changes stay isolated until the Auditor passes

---

### Phase Boundary: BUILD → AUDIT

Before entering Phase 3, run precondition assertions:
```bash
# Verify Builder produced substantive output
[ -s "$WORKSPACE_PATH/build-report.md" ] || { echo "HALT: Build report missing or empty — Phase 2 may have been skipped"; exit 1; }
```

### Phase 3: AUDIT (Parallel)

The Auditor reviews the Builder's changes **in the worktree** (or reads the diff from the build report). The worktree remains intact during audit. **Parallelization:** Because worktree tasks are built in parallel during Phase 2, they MUST also be audited in parallel during Phase 3 to minimize latency.

**Inline task audit:** For tasks implemented inline by the orchestrator (per inst-007 S-complexity policy), the orchestrator MUST still run eval graders before committing. At minimum, execute all bash eval graders from the task's eval definition and verify exit 0. If any grader fails, revert the inline changes (`git checkout -- <files>`) and either retry inline or escalate to a Builder agent. Do NOT skip the audit phase for inline tasks — the audit is the quality gate, not the builder.

**Eval checksum verification** (before launching Auditor):
Verify that eval files haven't been tampered with since Scout created them:
```bash
sha256sum -c $WORKSPACE_PATH/eval-checksums.json
```
If any checksum fails → HALT: "Eval tamper detected — eval file modified after Scout created it. Investigate before proceeding."

Launch **Auditor Agent** (model: per routing table — tier-2 default, tier-1 for security-sensitive, tier-3 for clean builds; subagent_type: `general-purpose`):
- Prompt: Read `agents/evolve-auditor.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static ---
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "evalsPath": ".evolve/evals/",
    "strategy": <strategy>,
    // --- Semi-stable ---
    "auditorProfile": "<state.json auditorProfile object>",
    // --- Dynamic ---
    "cycle": <N>,
    "buildReport": "<$WORKSPACE_PATH>/build-report.md",
    "recentLedger": "<last 3 ledger entries, inline>",
    "challengeToken": "<$CHALLENGE>",
    "forceFullAudit": <$FORCE_FULL_AUDIT>,
    "handoffFromBuilder": "<contents of handoff-builder.json>"
  }
  ```

After Auditor completes:
- Read `$WORKSPACE_PATH/audit-report.md`
- **Verdict handling:**
  - **PASS** → **Merge worktree changes into main and cleanup:**
    ```bash
    # Option A: If Agent tool managed the worktree (isolation: "worktree"),
    # changes are already in the working tree — just commit:
    git add -A
    git commit -m "<type>: <description>"

    # Option B: If manual worktree, cherry-pick the Builder's commit:
    BUILDER_SHA=<commit SHA from build-report.md>
    git cherry-pick "$BUILDER_SHA"
    git worktree remove "$WORKTREE_DIR" --force

    # Option C: If manual worktree, apply as patch:
    cd "$WORKTREE_DIR" && git diff HEAD~1 > /tmp/builder.patch
    cd <main-repo> && git apply /tmp/builder.patch
    git add -A && git commit -m "<type>: <description>"
    git worktree remove "$WORKTREE_DIR" --force
    ```
    Verify the merge is clean: `git status --porcelain` should show no uncommitted changes.
  - **WARN** (MEDIUM issues found) → re-launch Builder **in a fresh worktree** with issues, re-audit (max 3 total iterations). Remove the old worktree first.
  - **FAIL** (CRITICAL/HIGH or eval failures) → re-launch Builder **in a fresh worktree** with issues, re-audit (max 3 total iterations). Remove the old worktree first.
  - After 3 failures → **discard worktree**, log as failed approach, skip this task

**Worktree cleanup is MANDATORY.** After every task (pass or fail), verify no orphaned worktrees remain:
```bash
# List worktrees — should only show the main worktree
git worktree list
# If stale worktrees exist, prune them:
git worktree prune
```

**Update `auditorProfile` in state.json after each audit verdict:**
- PASS with no MEDIUM+ issues (first attempt) → increment `auditorProfile.<taskType>.consecutiveClean` and `passFirstAttempt`
- WARN, FAIL, or any MEDIUM+ issue → reset `auditorProfile.<taskType>.consecutiveClean` to 0

Then proceed to next task (back to Phase 2) or the Benchmark Delta Check if all tasks done.

---

### Benchmark Delta Check (between AUDIT and SHIP)

After all tasks pass audit but before committing in Phase 4, run a targeted benchmark re-evaluation to verify the cycle improved (or at least didn't regress) project quality.

**Exemptions** — skip the delta check entirely when:
- The task has `strategy: "repair"` (repairs are corrective, not additive)
- This is one of the first 3 cycles of this invocation (`remainingCycles > cycles - 3`) — allow the loop to establish a baseline
- The task is explicitly labeled as `meta` or `infrastructure` type

#### Delta Check Steps

1. **Verify benchmark-eval.md integrity:**
   ```bash
   sha256sum -c $WORKSPACE_PATH/benchmark-eval-checksum.txt
   ```
   If checksum fails → HALT: "benchmark-eval.md modified — possible tampering."

2. **Identify relevant dimensions** for this cycle's task types using the dimension-to-task-type mapping in [benchmark-eval.md](skills/evolve-loop/benchmark-eval.md). Only re-run checks for relevant dimensions (not all 8).

3. **Re-run automated checks** for relevant dimensions only. Compute new automated scores.

4. **Compare to Phase 0 baseline** (from `state.json.projectBenchmark.dimensions`):

   | Delta | Action |
   |-------|--------|
   | Any dimension improved (+2 or more) | **Ship.** Log improvement in benchmark report. |
   | All dimensions stable (within +/- 1) | **Ship with warning:** "No measurable benchmark improvement this cycle." Log warning in operator-log. |
   | Any dimension regressed (-3 or more) | **Block.** Return to Builder with regression details. |

5. **On block:** The blocked task gets **1 retry**. Pass the regression details to the Builder:
   ```json
   {
     "blockReason": "benchmark-regression",
     "regressedDimensions": [{"dimension": "<name>", "baseline": <N>, "current": <N>, "delta": <N>}],
     "guidance": "Fix the regression before shipping. Focus on: <specific dimension>"
   }
   ```
   Re-run Builder in a fresh worktree → re-audit → re-check delta.

6. **On second block (same task):** Drop the task entirely.
   - Log as `"dropped: benchmark-regression"` in `experiments.jsonl`
   - Add to `evaluatedTasks` with `decision: "dropped"`, `reason: "benchmark regression after retry"`
   - The bandit system records reward = 0 for the dropped task's type
   - Proceed to Phase 4 without this task's changes

7. **Update `projectBenchmark.dimensions`** in state.json with the new scores for re-checked dimensions.

---

### Pre-Ship Integrity Gate

Before entering Phase 4, run the deterministic health check and independent eval verification. These are the system's "security cameras" — they operate independently of all LLM agents.

1. **Independent eval re-execution** (research basis: "AI Control", Greenblatt et al.):
   ```bash
   bash scripts/verify-eval.sh .evolve/evals/<task-slug>.md $WORKSPACE_PATH
   VERIFY_EXIT=$?
   if [ "$VERIFY_EXIT" -ne 0 ]; then
     echo "HALT: Independent eval verification failed — Auditor may have reported false PASS"
     # Compare verify-eval output with audit-report.md for discrepancies
   fi
   ```
   This re-runs every eval command independently. If the Auditor claimed PASS but verify-eval shows failures, the Auditor's report is untrustworthy.

2. **Cycle health fingerprint**:
   ```bash
   bash scripts/cycle-health-check.sh $N $WORKSPACE_PATH
   HEALTH_EXIT=$?
   if [ "$HEALTH_EXIT" -ne 0 ]; then
     echo "HALT: Health check detected anomalies"
     cat $WORKSPACE_PATH/cycle-health.json
     # Present evidence to user and halt
   fi
   ```
   This checks all 11 health signals (ledger completeness, timestamp spacing, artifacts, checksums, challenge tokens, velocity, substance, canaries, hash chain). Any ANOMALY = halt.

3. **Log phase transition** (compliance trail):
   ```json
   {"ts":"<ISO>","cycle":N,"role":"orchestrator","type":"phase-transition","data":{
     "from":"audit","to":"ship",
     "challenge":"<$CHALLENGE>",
     "prevHash":"<$PREV_HASH>",
     "healthCheckRan":true,
     "healthCheckHealthy":<true|false>,
     "verifyEvalPassed":<true|false>
   }}
   ```
   After writing, update `PREV_HASH` to the hash of this entry.

Only proceed to Phase 4 if both the health check and eval verification pass.

### Phase 4: SHIP

For detailed Phase 4 instructions (serial ship lock, domain-specific shipping, state updates, process rewards), see [phase4-ship.md](skills/evolve-loop/phase4-ship.md).

---

### Phase 5: LEARN

For detailed Phase 5 instructions (instinct extraction, memory consolidation, operator check, meta-cycle self-improvement, context management), see [phase5-learn.md](skills/evolve-loop/phase5-learn.md).
