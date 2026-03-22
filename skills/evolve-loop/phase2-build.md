> Read this file when orchestrating Phase 2 (BUILD). Covers task partitioning, worktree isolation, Builder dispatch, Self-MoA parallel builds, and post-audit verdict handling.

## Contents
- [Task Execution Ordering](#task-execution-ordering--parallelization) — dependency graph, parallel groups
- [Self-MoA Parallel Builds](#self-moa-parallel-builds-m-complexity-tasks) — multi-Builder for M-complexity
- [Build Isolation](#build-isolation-by-projectcontextbuildisolation) — worktree, file-copy, none modes
- [Builder Agent Launch](#builder-agent-launch) — context block, model routing
- [Output Redirection](#output-redirection) — token-saving log strategy
- [Experiment Journal](#experiment-journal) — append-only attempt log
- [Speculative Auditor Execution](#speculative-auditor-execution) — latency optimization
- [Post-Builder Handling](#post-builder-handling) — failure logging, worktree discard
- [Post-Audit Verdict Handling](#post-audit-verdict-handling-per-task) — merge, retry, cleanup
- [Benchmark Delta Check](#benchmark-delta-check-between-audit-and-ship) — regression detection
- [Pre-Ship Integrity Gate](#pre-ship-integrity-gate) — independent verification

# Evolve Loop — Phase 2: BUILD

Each task from the Scout report is built in isolation, with parallel execution for independent tasks and sequential execution for dependent ones.

---

## Task Execution Ordering & Parallelization

Partition the task list into three groups before starting:

| Group | Description |
|-------|-------------|
| Inline tasks | S-complexity tasks eligible for orchestrator inline execution (per inst-007) |
| Worktree tasks (independent) | `filesToModify` lists share NO files with other worktree tasks |
| Worktree tasks (dependent) | Share files with another task; run sequentially after dependency completes |

### Dependency Partitioning Algorithm

```
1. Read filesToModify from each worktree task in the Scout report
2. Build a conflict graph: edge between tasks A and B if filesToModify(A) intersect filesToModify(B) != empty
3. Connected components in the conflict graph form sequential groups
4. Tasks with no edges (no shared files) are independent — build in parallel
5. Within a sequential group, run tasks in Scout's priority order
```

### Execution Sequence

1. Execute all inline tasks first sequentially, committing each before proceeding
2. Launch all independent worktree tasks **in parallel** (each in its own isolated worktree)
3. After all parallel builds complete, audit each independently (also in parallel)
4. Commit results sequentially in Scout's priority order
5. Execute dependent worktree tasks sequentially after their dependencies commit

---

## Self-MoA Parallel Builds (M-complexity tasks)

Run 2-3 Builder agents from the same model with varied approaches, accepting the first passing result. (Research: M1-Parallel [arXiv:2507.08944]; Self-MoA [arXiv:2502.00674].)

| Task Complexity | Self-MoA? | Rationale |
|----------------|-----------|-----------|
| **S** | No — single Builder | Single pass sufficient; Self-MoA wastes tokens |
| **M (<=5 files)** | Optional — 2 Builders | Moderate benefit; use if budget allows |
| **M (>5 files)** | Yes — 2-3 Builders | Complex tasks benefit most from diversity |
| **retry >= 2** | Yes — 3 Builders | Previous approaches failed; maximize diversity |

### Execution Protocol

1. **Launch N Builders in parallel** (N=2 standard M, N=3 retry >= 2), each in own worktree:
   - All receive same task context
   - Each gets a different `selfMoaVariant`: `"A"`, `"B"`, `"C"`
   - Variant A: standard approach
   - Variant B: explicitly different design than first instinct
   - Variant C (if used): minimal approach — smallest diff, fewest files

2. **Early termination**: First Builder passing all eval graders → accept, cancel remaining. If none pass → pass best-scoring to Auditor.

3. **Cost control**: Track `selfMoaUsed` and `selfMoaWinner` in build-report.md. If variant A wins >80% over 5 cycles, reduce to single Builder.

4. **Anti-reward-hacking**: Each variant runs independently. Eval graders (not a reward model) select the winner.

When Self-MoA is active, add to Builder context:
```json
{
  "selfMoaVariant": "A|B|C",
  "selfMoaInstruction": "You are variant <X> in a parallel build. Variant A uses the standard approach. Variant B must choose an explicitly different design. Variant C (if present) uses the minimal diff approach."
}
```

---

## Build Isolation (by `projectContext.buildIsolation`)

| Mode | When | Implementation |
|------|------|---------------|
| `worktree` (default) | Coding domain | **MANDATORY.** Git worktree per task. Prevents interference, parallel conflicts, and partial pollution. |
| `file-copy` | Writing, research, design | Copy project (exclude `.evolve/`), diff and apply back after build. |
| `none` | Explicit opt-out | Builder operates on working directory. Only for append-only tasks. Warn when active. |

### `worktree` Implementation

**Never launch Builder without isolation.** Platform-specific methods:
- **Claude Code:** `Agent` tool with `isolation: "worktree"` (automatic)
- **Gemini CLI / other:** Create worktree manually before spawning agent
- **Generic LLM:** Create worktree, run Builder in new session with cwd set to worktree

Manual worktree creation:
```bash
WORKTREE_DIR=$(mktemp -d)/evolve-build-cycle-<N>-<task-slug>
git worktree add "$WORKTREE_DIR" HEAD
# Launch Builder with cwd set to $WORKTREE_DIR
# After completion:
cd "$WORKTREE_DIR" && git diff HEAD > /tmp/builder.patch
cd <main-repo> && git apply /tmp/builder.patch
git worktree remove "$WORKTREE_DIR"
```

### `file-copy` Implementation

```bash
COPY_DIR=$(mktemp -d)/evolve-build-cycle-<N>-<task-slug>
cp -rp . "$COPY_DIR" && rm -rf "$COPY_DIR/.evolve"
# Launch Builder with cwd set to $COPY_DIR
# After completion, diff and copy changed files back
rm -rf "$COPY_DIR"
```

---

## Builder Agent Launch

Launch **Builder Agent** (model: tier-3 if S-complexity + plan cache hit; tier-1 if strategy == "ultrathink" OR M-complexity + 5+ files OR audit retry >= 2; tier-2 if strategy == "repair"; tier-2 otherwise; **isolation: worktree required**):
- **Platform dispatch:** Claude Code: `Agent` tool with `isolation: "worktree"`, `subagent_type: "general-purpose"`; Other: create worktree manually, launch in worktree directory.
- Prompt: Read `agents/evolve-builder.md` and pass as prompt
- Context:
  ```json
  {
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "evalsPath": ".evolve/evals/",
    "strategy": "<strategy>",
    "instinctSummary": "<from state.json, inline>",
    "cycle": "<N>",
    "task": "<task object from scout-report — includes inline eval graders>",
    "challengeToken": "<$CHALLENGE>",
    "handoffFromScout": "<contents of handoff-scout.json>"
  }
  ```
- Builder reads eval acceptance criteria from the task object. Full eval files from `evalsPath` only if inline graders are missing.

---

## Output Redirection

Redirect stdout/stderr from eval graders, tests, and build commands:
```bash
<command> > $WORKSPACE_PATH/run.log 2>&1
```
Extract results via `grep`/`tail` on `run.log`. Reduces token consumption by 30-50% for verbose output.

---

## Experiment Journal

After each Builder attempt (pass or fail), append to `$WORKSPACE_PATH/experiments.jsonl`:
```jsonl
{"cycle":N,"task":"<slug>","attempt":1,"verdict":"PASS|FAIL","approach":"<1-sentence>","metric":"<eval result or error>"}
```
Scout reads `experiments.jsonl` to avoid re-proposing failed approaches.

---

## Speculative Auditor Execution

Instead of waiting for full Builder completion, start Auditor **speculatively** as soon as Builder commits in worktree. (Research: Sherlock [arXiv:2511.00330] — 48.7% latency reduction.)

- **Use when:** Builder works in a worktree (changes isolated and committed)
- **Skip when:** Builder is inline (S-complexity), or platform lacks concurrent agents

| Outcome | Action |
|---------|--------|
| Auditor PASS | Total time = max(Builder, Auditor) instead of sum |
| Auditor FAIL | Discard worktree, log failure, retry or next task |
| Builder fails after Auditor started | Cancel Auditor, discard both |

Speculative execution does NOT change the quality gate — only *when* Auditor starts, not *what* it checks.

---

## Post-Builder Handling

After Builder completes:
- Read `$WORKSPACE_PATH/build-report.md`
- If FAIL after 3 attempts:
  - Discard worktree: `git worktree remove "$WORKTREE_DIR" --force 2>/dev/null`
  - Log failed approach in state.json under `failedApproaches`:
    ```json
    {
      "feature": "<task name>",
      "approach": "<what was tried>",
      "error": "<error message>",
      "reasoning": "<WHY it failed — root cause>",
      "filesAffected": ["<files involved>"],
      "cycle": "<N>",
      "alternative": "<suggested different approach>"
    }
    ```
  - Skip task, proceed to next (or Phase 3 if last)
- If PASS → proceed to Phase 3 (do NOT merge yet — worktree stays isolated until Auditor passes)

---

## Post-Audit Verdict Handling (per task)

After Auditor completes, read `$WORKSPACE_PATH/audit-report.md`:

| Verdict | Action |
|---------|--------|
| **PASS** | Merge worktree changes and cleanup (see merge options below) |
| **WARN** (MEDIUM issues) | Re-launch Builder in fresh worktree with issues, re-audit (max 3 total) |
| **FAIL** (CRITICAL/HIGH or eval failures) | Re-launch Builder in fresh worktree with issues, re-audit (max 3 total) |
| After 3 failures | Discard worktree, log as failed approach, skip task |

### Merge Options (on PASS)

```bash
# Option A: Platform auto-managed worktree (e.g., Claude Code isolation: "worktree")
git add -A && git commit -m "<type>: <description>"

# Option B: Manual worktree — cherry-pick
BUILDER_SHA=<commit SHA from build-report.md>
git cherry-pick "$BUILDER_SHA"
git worktree remove "$WORKTREE_DIR" --force

# Option C: Manual worktree — patch
cd "$WORKTREE_DIR" && git diff HEAD~1 > /tmp/builder.patch
cd <main-repo> && git apply /tmp/builder.patch
git add -A && git commit -m "<type>: <description>"
git worktree remove "$WORKTREE_DIR" --force
```

Verify: `git status --porcelain` should show no uncommitted changes.

**Worktree cleanup is MANDATORY** after every task:
```bash
git worktree list    # should only show main worktree
git worktree prune   # remove stale worktrees
```

**Update `auditorProfile`** in state.json:
- PASS with no MEDIUM+ issues (first attempt) → increment `consecutiveClean` and `passFirstAttempt`
- WARN, FAIL, or any MEDIUM+ issue → reset `consecutiveClean` to 0

---

## Benchmark Delta Check (between AUDIT and SHIP)

After all tasks pass audit, before committing in Phase 4, verify project quality.

**Exemptions** — skip when:
- `strategy: "repair"` (corrective, not additive)
- First 3 cycles of invocation
- Task labeled `meta` or `infrastructure`

### Delta Check Steps

1. Verify `benchmark-eval.md` integrity: `sha256sum -c $WORKSPACE_PATH/benchmark-eval-checksum.txt`
2. Identify relevant dimensions using [benchmark-eval.md](skills/evolve-loop/benchmark-eval.md) mapping
3. Re-run automated checks for relevant dimensions only
4. Compare to Phase 0 baseline:

| Delta | Action |
|-------|--------|
| Any dimension +2 or more | **Ship.** Log improvement. |
| All dimensions within +/- 1 | **Ship with warning.** Log in operator-log. |
| Any dimension -3 or more | **Block.** Return to Builder with regression details. |

5. On block: 1 retry with regression details passed to Builder
6. On second block: drop task entirely, log as `"dropped: benchmark-regression"`, bandit reward = 0
7. Update `projectBenchmark.dimensions` in state.json

---

## Pre-Ship Integrity Gate

Run deterministic health check and independent eval verification before Phase 4.

1. **Independent eval re-execution:**
   ```bash
   bash scripts/verify-eval.sh .evolve/evals/<task-slug>.md $WORKSPACE_PATH
   ```
   If exit != 0 → HALT: "Independent eval verification failed."

2. **Cycle health fingerprint:**
   ```bash
   bash scripts/cycle-health-check.sh $N $WORKSPACE_PATH
   ```
   Checks 11 signals (ledger completeness, timestamps, artifacts, checksums, challenge tokens, velocity, substance, canaries, hash chain). Any ANOMALY = halt.

3. **Log phase transition:**
   ```json
   {"ts":"<ISO>","cycle":"<N>","role":"orchestrator","type":"phase-transition","data":{
     "from":"audit","to":"ship",
     "challenge":"<$CHALLENGE>",
     "prevHash":"<$PREV_HASH>",
     "healthCheckRan":true,
     "healthCheckHealthy":"<true|false>",
     "verifyEvalPassed":"<true|false>"
   }}
   ```

Proceed to Phase 4 only if both health check and eval verification pass.
