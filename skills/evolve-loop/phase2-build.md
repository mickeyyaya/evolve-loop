# Evolve Loop — Phase 2: BUILD

Detailed orchestrator instructions for the BUILD phase. Each task from the Scout report is built in isolation, with parallel execution for independent tasks and sequential execution for dependent ones.

---

## Task Execution Ordering & Parallelization

Before starting the build loop, partition the task list into three groups:
1. **Inline tasks** — S-complexity tasks eligible for orchestrator inline execution (per inst-007 policy)
2. **Worktree tasks (independent)** — tasks whose `filesToModify` lists share NO files with other worktree tasks
3. **Worktree tasks (dependent)** — tasks that share files with another task; these run sequentially after their dependency completes

### Dependency Partitioning Algorithm

```
1. Read filesToModify from each worktree task in the Scout report
2. Build a conflict graph: edge between tasks A and B if filesToModify(A) ∩ filesToModify(B) ≠ ∅
3. Connected components in the conflict graph form sequential groups
4. Tasks with no edges (no shared files) are independent — build in parallel
5. Within a sequential group, run tasks in Scout's priority order
```

### Execution Sequence

1. Execute all inline tasks first sequentially, committing each before proceeding
2. Launch all independent worktree tasks **in parallel** via your platform's parallel agent mechanism (each in its own isolated worktree — see Build Isolation below)
3. After all parallel builds complete, audit each independently (also in parallel if multiple)
4. Commit results sequentially in Scout's priority order (one at a time)
5. Execute dependent worktree tasks sequentially after their dependencies commit

Parallel execution of worktree tasks drastically reduces build phase latency. Because each task is isolated in its own worktree and the OCC protocol handles state.json conflicts, they can safely be built concurrently.

---

## Self-MoA Parallel Builds (M-complexity tasks)

For M-complexity tasks, the orchestrator MAY use **Self-MoA** (Self Mixture-of-Agents) to improve first-attempt pass rate and reduce latency. This runs 2-3 Builder agents from the same model with varied approaches, accepting the first passing result. (Research basis: M1-Parallel [arXiv:2507.08944] — 2.2x speedup with no accuracy loss; Self-MoA [arXiv:2502.00674] — single-model diversity outperforms multi-model mixing by 6.6%.)

### When to use Self-MoA

| Task Complexity | Self-MoA? | Rationale |
|----------------|-----------|-----------|
| **S** | No — single Builder | Single pass is sufficient; Self-MoA wastes tokens |
| **M (≤5 files)** | Optional — 2 Builders | Moderate benefit; use if budget allows |
| **M (>5 files)** | Yes — 2-3 Builders | High benefit; complex tasks benefit most from diversity |
| **retry attempt ≥ 2** | Yes — 3 Builders | Previous approaches failed; maximize diversity |

### Execution Protocol

1. **Launch N Builder agents in parallel** (N=2 for standard M, N=3 for retry ≥ 2), each in its own worktree:
   - All receive the same task context
   - Each gets a different `selfMoaVariant` field in context: `"A"`, `"B"`, `"C"`
   - Variant A: standard approach (as if single Builder)
   - Variant B: alternative approach — Builder must explicitly choose a different design than its first instinct
   - Variant C (if used): minimal approach — smallest possible diff, fewest files touched

2. **Early termination**: As each Builder completes, immediately run its eval graders:
   - First Builder whose output **passes all eval graders** → accept that result, cancel remaining Builders
   - If no Builder passes on first attempt → collect all outputs, pass the best-scoring one to Auditor for detailed review

3. **Cost control**:
   - Self-MoA multiplies Builder token cost by N (2-3x). Only use when expected benefit outweighs cost.
   - Track `selfMoaUsed: true/false` and `selfMoaWinner: "A"/"B"/"C"` in build-report.md for meta-cycle analysis
   - If Self-MoA winners are consistently variant A (>80% of the time over 5 cycles), reduce to single Builder — diversity isn't helping

4. **Anti-reward-hacking**: Each variant runs independently in its own worktree. The eval graders (not a reward model) select the winner. This avoids proxy reward misalignment because evals test real behavior.

### Builder Context Addition

When Self-MoA is active, add to the Builder context:
```json
{
  "selfMoaVariant": "A|B|C",
  "selfMoaInstruction": "You are variant <X> in a parallel build. Variant A uses the standard approach. Variant B must choose an explicitly different design. Variant C (if present) uses the minimal diff approach."
}
```

The Builder should note its variant in the build report header.

---

For each worktree task:

---

## Build Isolation (by `projectContext.buildIsolation`)

### `worktree` (default — coding domain)

**Worktree isolation is MANDATORY for coding projects.** The orchestrator MUST ensure the Builder operates on an isolated copy of the repository. This prevents:
- Builder changes from interfering with the main working tree during execution
- Multiple Builder runs (if parallelized) from conflicting with each other
- Partial/failed changes from polluting the main branch

**NEVER launch the Builder without isolation.** Platform-specific methods:
- **Claude Code:** Use the `Agent` tool with `isolation: "worktree"` (automatic worktree management)
- **Gemini CLI / other platforms with subagent support:** Create a worktree manually before spawning the agent (see below)
- **Generic LLM / no subagent support:** Create a worktree and run the Builder prompt in a new session with cwd set to the worktree

If the platform does not support automatic worktree isolation, the orchestrator MUST manually create a worktree before launching the Builder:
```bash
WORKTREE_DIR=$(mktemp -d)/evolve-build-cycle-<N>-<task-slug>
git worktree add "$WORKTREE_DIR" HEAD
# Launch Builder with cwd set to $WORKTREE_DIR
# After Builder completes, merge changes back:
cd "$WORKTREE_DIR" && git diff HEAD > /tmp/builder.patch
cd <main-repo> && git apply /tmp/builder.patch
git worktree remove "$WORKTREE_DIR"
```

### `file-copy` (writing, research, design domains)

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

### `none` (explicit opt-out — use with caution)

Builder operates directly on the working directory. Only suitable for append-only tasks where isolation provides no benefit (e.g., writing a new standalone document). The orchestrator should warn when this mode is active.

The orchestrator selects the isolation mode from `projectContext.buildIsolation` (set during initialization step 3). Worktree remains the default fallback if `buildIsolation` is not specified.

---

## Builder Agent Launch

Launch **Builder Agent** (model: per routing table — tier-3 if S-complexity + plan cache hit (execution-only, plan is proven), tier-1 if strategy == "ultrathink" OR M-complexity + 5+ files OR audit retry attempt ≥ 2 (design mistake needs deeper reasoning about WHY it failed), tier-2 if strategy == "repair" (accuracy floor), tier-2 otherwise; **isolation: worktree required**):
- **Platform dispatch:** Claude Code: `Agent` tool with `isolation: "worktree"`, `subagent_type: "general-purpose"`; Other platforms: create worktree manually (see Build Isolation above), then launch agent in the worktree directory.
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

---

## Output Redirection

When Builder runs eval graders, test commands, or build commands, redirect stdout/stderr to `$WORKSPACE_PATH/run.log`:
```bash
<command> > $WORKSPACE_PATH/run.log 2>&1
```
Builder and Auditor extract results via `grep`/`tail` on `run.log` rather than reading full output. This reduces token consumption by 30-50% for verbose build/test output.

---

## Experiment Journal

After each Builder attempt (pass or fail), append a one-line entry to `$WORKSPACE_PATH/experiments.jsonl`:
```jsonl
{"cycle":N,"task":"<slug>","attempt":1,"verdict":"PASS|FAIL","approach":"<1-sentence summary>","metric":"<eval result or error>"}
```
This append-only log ensures every attempt is recorded. Scout reads `experiments.jsonl` to avoid re-proposing similar approaches that already failed.

---

## Post-Builder Handling

## Speculative Auditor Execution (optional — latency optimization)

Instead of waiting for the Builder to fully complete before launching the Auditor, the orchestrator MAY start the Auditor **speculatively** as soon as the Builder commits in its worktree. This overlaps build and audit, reducing cycle latency. (Research basis: Sherlock [arXiv:2511.00330] — 48.7% latency reduction + 18.3% accuracy gain via speculative execution.)

### When to use speculative auditing

- **Use when:** Builder is working in a worktree (changes are isolated and committed)
- **Skip when:** Builder is inline (S-complexity), or platform doesn't support concurrent agents

### Protocol

1. Builder commits in its worktree and writes `build-report.md` with status PASS
2. Orchestrator launches Auditor immediately (concurrently with any remaining Builder self-verification)
3. **If Auditor PASS:** Total cycle time = max(Builder, Auditor) instead of Builder + Auditor
4. **If Auditor FAIL:** Roll back — discard worktree, log failure, proceed to retry or next task
5. **If Builder fails after Auditor started:** Cancel Auditor, discard both

**Safety:** Speculative execution does NOT change the quality gate. The Auditor still runs the full checklist and eval graders. It only changes *when* the Auditor starts, not *what* it checks.

---

After Builder completes:
- Read `$WORKSPACE_PATH/build-report.md`
- If status is FAIL after 3 attempts:
  - **Discard worktree** — the worktree branch contains failed changes, clean it up:
    ```bash
    # If the platform auto-managed the worktree, it may auto-clean on failure
    # Otherwise, remove it manually:
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

## Post-Audit Verdict Handling (per task)

After the Auditor completes for a task:
- Read `$WORKSPACE_PATH/audit-report.md`
- **Verdict handling:**
  - **PASS** → **Merge worktree changes into main and cleanup:**
    ```bash
    # Option A: If platform auto-managed the worktree (e.g., Claude Code isolation: "worktree"),
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

## Benchmark Delta Check (between AUDIT and SHIP)

After all tasks pass audit but before committing in Phase 4, run a targeted benchmark re-evaluation to verify the cycle improved (or at least didn't regress) project quality.

**Exemptions** — skip the delta check entirely when:
- The task has `strategy: "repair"` (repairs are corrective, not additive)
- This is one of the first 3 cycles of this invocation (`remainingCycles > cycles - 3`) — allow the loop to establish a baseline
- The task is explicitly labeled as `meta` or `infrastructure` type

### Delta Check Steps

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

## Pre-Ship Integrity Gate

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
