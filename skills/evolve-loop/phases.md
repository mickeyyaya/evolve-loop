# Evolve Loop — Phase Instructions

Detailed orchestrator instructions for each phase. Optimized for fast iteration with diverse small/medium tasks per cycle.

**Important:** Every agent context block must include `goal` (string or null) and `strategy` (one of: `balanced`, `innovate`, `harden`, `repair`).

## FOR cycle = {startCycle} to {endCycle}:

### Phase 1: DISCOVER

**Convergence Short-Circuit** (check BEFORE launching Scout):
- Read `stagnation.nothingToDoCount` from state.json:
  - If `>= 2`: Skip Scout entirely. Jump to Phase 5 with Operator in `"convergence-check"` mode. Operator can reset `nothingToDoCount` to 0 if it detects new work (e.g., external changes via `git log`).
  - If `== 1`: **Escalation before convergence** — before running convergence-confirmation, the orchestrator attempts to unlock new work:
    1. Review the last 3 cycles' deferred tasks (from `evaluatedTasks` with `decision: "deferred"`) for items that could be combined into a single task
    2. Check if switching strategy (e.g., `balanced` → `innovate` or `harden`) would surface new task candidates
    3. Propose a "radical" task that challenges an existing assumption or convention in the codebase (think harder — re-read code for new angles)
    If escalation produces a viable task → reset `nothingToDoCount` to 0, proceed with normal Scout launch using the escalated task as a seed.
    If escalation fails → Launch Scout in `"convergence-confirmation"` mode — Scout reads ONLY state.json + `git log --oneline -3` and MUST trigger new web research to find potential external tasks/updates, bypassing any cooldowns. No notes, no ledger, no instincts, no codebase scan. If still nothing found, increment to 2 and skip to Phase 5.
  - If `== 0`: Proceed with normal Scout launch below.

**Pre-compute context** (orchestrator reads files once, passes inline slices):
```bash
# Cycle 1: full mode — no digest exists yet
# Cycle 2+: incremental mode — read digest + changed files
if [ -f .claude/evolve/workspace/project-digest.md ]; then
  MODE="incremental"
  DIGEST=$(cat .claude/evolve/workspace/project-digest.md)
  CHANGED=$(git diff HEAD~1 --name-only 2>/dev/null)
else
  MODE="full"
fi

# Read recent notes (last 5 cycle entries, not full file)
RECENT_NOTES=$(# extract last 5 "## Cycle" sections from notes.md)

# Read builder notes from last cycle (if exists)
BUILDER_NOTES=$(cat .claude/evolve/workspace/builder-notes.md 2>/dev/null || echo "")

# Read recent ledger (last 3 lines)
RECENT_LEDGER=$(tail -3 .claude/evolve/ledger.jsonl)

# instinctSummary and ledgerSummary come from state.json (already read)
```

**Operator brief pre-read:** Before launching Scout, check if `workspace/next-cycle-brief.json` exists (written by the previous cycle's Operator). If present, pass its contents in the Scout context so Scout can apply `recommendedStrategy`, `taskTypeBoosts`, and `avoidAreas` during task selection.

Launch **Scout Agent** (model: per routing table — sonnet default, haiku for incremental, opus for deep research; subagent_type: `general-purpose`):
- Prompt: Read `agents/evolve-scout.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static (stable across cycles, maximizes cache-like reuse) ---
    "projectContext": <auto-detected>,
    "projectDigest": "<contents of project-digest.md, or null if cycle 1>",
    "workspacePath": ".claude/evolve/workspace/",
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
    "builderNotes": "<contents of workspace/builder-notes.md from last cycle, or empty string>",
    "recentLedger": "<last 3 ledger entries, inline>"
  }
  ```

After Scout completes:
- Read `workspace/scout-report.md`
- **Prerequisite check:** For each proposed task that includes a `prerequisites` field, verify all listed slugs appear in `state.json.evaluatedTasks` with `decision: "completed"`. Any task with an unmet prerequisite is automatically deferred: add it to `evaluatedTasks` with `decision: "deferred"` and `deferralReason: "prerequisite not met: <slug>"`, then log the prerequisite slug so the Scout can propose it in the next cycle. Tasks without a `prerequisites` field are unaffected. This check is a lightweight sequencing aid — the Scout may override it by omitting `prerequisites` when a task is genuinely independent of its nominal dependency.
- Verify eval definitions were created in `.claude/evolve/evals/`
- **Eval checksum capture:** Compute `sha256sum` of each eval file in `.claude/evolve/evals/` and store in `workspace/eval-checksums.json`:
  ```bash
  sha256sum .claude/evolve/evals/*.md > .claude/evolve/workspace/eval-checksums.json
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

---

### Phase 2: BUILD (loop per task)

For each task in the Scout's selected task list:

**Worktree isolation is MANDATORY.** The orchestrator MUST launch the Builder with `isolation: "worktree"` so it operates on an isolated copy of the repository. This prevents:
- Builder changes from interfering with the main working tree during execution
- Multiple Builder runs (if parallelized in the future) from conflicting with each other
- Partial/failed changes from polluting the main branch

**NEVER launch the Builder without `isolation: "worktree"`.** If the Agent tool does not support worktree isolation, the orchestrator MUST manually create a worktree before launching the Builder:
```bash
WORKTREE_DIR=$(mktemp -d)/evolve-build-cycle-<N>-<task-slug>
git worktree add "$WORKTREE_DIR" HEAD
# Launch Builder with cwd set to $WORKTREE_DIR
# After Builder completes, merge changes back:
cd "$WORKTREE_DIR" && git diff HEAD > /tmp/builder.patch
cd <main-repo> && git apply /tmp/builder.patch
git worktree remove "$WORKTREE_DIR"
```

Launch **Builder Agent** (model: per routing table — sonnet default, opus for complex M tasks, haiku for S-complexity; subagent_type: `general-purpose`, **isolation: `worktree`**):
- Prompt: Read `agents/evolve-builder.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static ---
    "workspacePath": ".claude/evolve/workspace/",
    "evalsPath": ".claude/evolve/evals/",
    "strategy": <strategy>,
    // --- Semi-stable ---
    "instinctSummary": "<from state.json, inline>",
    // --- Dynamic ---
    "cycle": <N>,
    "task": <task object from scout-report — includes inline eval graders>
  }
  ```
- **Note:** Builder reads eval acceptance criteria from the task object in scout-report.md (inline `Eval Graders` field) instead of reading separate eval files. Builder still reads full eval files from `evalsPath` only if inline graders are missing.

**Output Redirection:** When Builder runs eval graders, test commands, or build commands, redirect stdout/stderr to `.claude/evolve/workspace/run.log`:
```bash
<command> > .claude/evolve/workspace/run.log 2>&1
```
Builder and Auditor extract results via `grep`/`tail` on `run.log` rather than reading full output. This reduces token consumption by 30-50% for verbose build/test output.

**Experiment Journal:** After each Builder attempt (pass or fail), append a one-line entry to `.claude/evolve/workspace/experiments.jsonl`:
```jsonl
{"cycle":N,"task":"<slug>","attempt":1,"verdict":"PASS|FAIL","approach":"<1-sentence summary>","metric":"<eval result or error>"}
```
This append-only log ensures every attempt is recorded. Scout reads `experiments.jsonl` to avoid re-proposing similar approaches that already failed.

After Builder completes:
- Read `workspace/build-report.md`
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

### Phase 3: AUDIT

The Auditor reviews the Builder's changes **in the worktree** (or reads the diff from the build report). The worktree remains intact during audit.

**Eval checksum verification** (before launching Auditor):
Verify that eval files haven't been tampered with since Scout created them:
```bash
sha256sum -c .claude/evolve/workspace/eval-checksums.json
```
If any checksum fails → HALT: "Eval tamper detected — eval file modified after Scout created it. Investigate before proceeding."

Launch **Auditor Agent** (model: per routing table — sonnet default, opus for security-sensitive, haiku for clean builds; subagent_type: `general-purpose`):
- Prompt: Read `agents/evolve-auditor.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static ---
    "workspacePath": ".claude/evolve/workspace/",
    "evalsPath": ".claude/evolve/evals/",
    "strategy": <strategy>,
    // --- Semi-stable ---
    "auditorProfile": "<state.json auditorProfile object>",
    // --- Dynamic ---
    "cycle": <N>,
    "buildReport": ".claude/evolve/workspace/build-report.md",
    "recentLedger": "<last 3 ledger entries, inline>"
  }
  ```

After Auditor completes:
- Read `workspace/audit-report.md`
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

Then proceed to next task (back to Phase 2) or Phase 4 if all tasks done.

---

### Phase 4: SHIP (orchestrator inline — MANDATORY)

No agent needed. The orchestrator handles shipping directly. **This phase is not optional — every cycle MUST commit and push.**

1. **Verify all commits are clean:**
   ```bash
   git status
   git log --oneline -<N>  # verify N commits from this cycle
   ```

2. **Commit any uncommitted changes** (if tasks were implemented inline by orchestrator):
   ```bash
   git add <changed files>
   git commit -m "<type>: <description>"
   ```

3. **Push to remote:**
   ```bash
   git push origin <branch>
   ```
   This is mandatory after every cycle. The cycle is not complete until code is pushed.

4. **Publish plugin update:**
   ```bash
   ./publish.sh
   ```
   This syncs the local plugin cache and registry so all new Claude Code sessions automatically load the latest version. The script auto-detects the version from `plugin.json`, validates source files, populates the cache, and updates the registry. **This step is mandatory after every push** — without it, new sessions will load a stale version.

5. **Clear non-persistent mailbox messages:**
   Remove rows from `workspace/agent-mailbox.md` where `persistent` is `false`. Retain rows where `persistent` is `true` so cross-cycle warnings survive into the next cycle.
   ```bash
   # Filter in-place: keep header rows and persistent=true rows
   grep -v "| false |" .claude/evolve/workspace/agent-mailbox.md > /tmp/mailbox-tmp.md && mv /tmp/mailbox-tmp.md .claude/evolve/workspace/agent-mailbox.md
   ```

6. **Update state.json:**
   - Mark completed tasks in `evaluatedTasks`
   - Update `lastCycleNumber` to current cycle number
   - Reset `stagnation.nothingToDoCount` to 0
   - Update `lastUpdated`
   - **Compute `fitnessScore`** — weighted average of processRewards dimensions as a single "did the project get better?" signal:
     ```json
     "fitnessScore": round(0.25 * discover + 0.30 * build + 0.20 * audit + 0.15 * ship + 0.10 * learn, 2)
     ```
     After computing, compare to previous cycle's fitnessScore in state.json:
     - If fitnessScore decreased for 2 consecutive cycles → set `fitnessRegression: true` in state.json. The Operator reads this as a HALT-worthy signal.
     - If fitnessScore increased or held steady → set `fitnessRegression: false`.
     - Store the score: `"fitnessScore": <value>` and `"fitnessHistory": [<last 3 scores>]` in state.json.

   - **Compute `ledgerSummary`** from ledger.jsonl (aggregated stats so agents never read the full ledger):
     ```json
     "ledgerSummary": {
       "totalEntries": <count>,
       "cycleRange": [<first>, <last>],
       "scoutRuns": <count>,
       "builderRuns": <count>,
       "totalTasksShipped": <sum of tasksShipped across evalHistory>,
       "totalTasksFailed": <sum of failed>,
       "avgTasksPerCycle": <shipped / cycles>
     }
     ```
   - **Trim `evalHistory`** in state.json to keep only the last 5 entries (older data is captured by `ledgerSummary`)
   - Record **process rewards** for each phase this cycle (step-level scoring):
     ```json
     {
       "processRewards": {
         "discovery": <0.0-1.0>,
         "build": <0.0-1.0>,
         "audit": <0.0-1.0>,
         "ship": <0.0-1.0>,
         "learn": <0.0-1.0>,
         "skillEfficiency": <0.0-1.0>
       }
     }
     ```
     **Scoring rubric** — compute each dimension deterministically:

     | Phase | Score = 1.0 | Score = 0.5 | Score = 0.0 |
     |-------|-------------|-------------|-------------|
     | **discover** | All selected tasks shipped | 50%+ tasks shipped | <50% tasks shipped |
     | **build** | All tasks pass audit first attempt | Some tasks need retry | 3+ audit failures |
     | **audit** | No false positives, all evals run | 1 false positive or missing eval | Multiple false positives |
     | **ship** | Clean commit, no post-commit fixes | Minor fixup needed | Failed to push or dirty state |
     | **learn** | Instincts extracted AND at least one instinct cited in scout-report or build-report `instinctsApplied` | Instincts extracted but none cited this cycle | No instincts extracted |
     | **skillEfficiency** | Total skill+agent tokens decreased from `skillMetrics` baseline | Tokens stable (±5% of baseline) | Tokens increased from baseline |

     Process rewards feed into meta-cycle reviews for targeted agent improvement. A consistently low discovery score means the Scout needs attention, not the Builder. A low skillEfficiency score signals prompt bloat that should be addressed.

   - **Update `processRewardsHistory`** — append this cycle's scores to the rolling array, trimming to keep only the last 3 entries:
     ```json
     "processRewardsHistory": [
       {"cycle": <N-2>, ...scores...},
       {"cycle": <N-1>, ...scores...},
       {"cycle": <N>, "discover": <score>, "build": <score>, "audit": <score>, "ship": <score>, "learn": <score>, "skillEfficiency": <score>}
     ]
     ```

   - **Per-cycle remediation check** (self-improvement trigger):
     After computing process rewards, check `processRewardsHistory` for sustained low scores:
     - If any dimension scores below 0.7 for 2+ consecutive entries in the history → append a remediation entry to `state.json.pendingImprovements`:
       ```json
       {"dimension": "<dim>", "score": <latest>, "sustained": true, "suggestedTask": "<what to fix>", "cycle": <N>, "priority": "high"}
       ```
     - Suggested task mapping:
       - `discover < 0.7` → "improve Scout task sizing or relevance"
       - `build < 0.7` → "add Builder guidance or simplify task complexity"
       - `audit < 0.7` → "review eval grader quality and coverage"
       - `ship < 0.7` → "fix commit workflow or git state issues"
       - `learn < 0.7` → "extract instincts from recent successful cycles"
       - `skillEfficiency < 0.7` → "reduce prompt overhead in skill/agent files"
     - Clear resolved entries: if a dimension's score rises above 0.7 for 2 consecutive cycles, remove its pendingImprovements entry

   - Add eval results to `evalHistory` with **delta metrics**:
     ```json
     {
       "cycle": <N>,
       "verdict": "PASS|WARN|FAIL",
       "checks": <total>,
       "passed": <passed>,
       "failed": <failed>,
       "delta": {
         "tasksShipped": <count>,
         "tasksAttempted": <count>,
         "auditIterations": <average iterations per task>,
         "successRate": <shipped / attempted>,
         "instinctsExtracted": <count this cycle>,
         "stagnationPatterns": <active patterns count>
       }
     }
     ```
   - The `delta` object enables trend analysis across cycles. The Operator and meta-cycle review use these metrics to detect improvement or degradation.
   - **Update mastery level:**
     - If `delta.successRate === 1.0` → increment `mastery.consecutiveSuccesses`
     - If `mastery.consecutiveSuccesses >= 3` and level is not `proficient` → advance level, reset counter
     - If `delta.successRate < 0.5` for 2 consecutive cycles → regress level, reset counter

---

### Phase 5: LEARN (orchestrator inline + operator)

1. **Archive workspace:**
   ```bash
   mkdir -p .claude/evolve/history/cycle-{N}
   cp .claude/evolve/workspace/*.md .claude/evolve/history/cycle-{N}/
   # builder-notes.md is included in *.md above; it is NOT cleared here so Phase 1 of the next cycle can read it
   ```

2. **Memory Consolidation Check:**
   Before extracting new instincts, check if consolidation is due:
   ```
   if (cycle % 3 === 0) OR (instinctCount > 20):
     → run Memory Consolidation (see step 3 below)
   ```
   This ensures consolidation runs at predictable intervals and prevents instinct bloat.

3. **Instinct Citation Collection:**
   Before extracting new instincts, collect citation lists from this cycle's workspace files:
   - Read `instinctsApplied` from `scout-report.md` and `build-report.md`
   - Aggregate cited inst IDs into a `citedInstincts` set for this cycle
   - For each cited instinct, increase its confidence by +0.05 (capped at 1.0) — application-driven confidence is more reliable than re-observation
   - Update `instinctSummary` in state.json with new confidence values

3b. **Instinct Extraction Trigger (forced extraction on stall):**
   Before running normal extraction, check if passive extraction has stalled:
   ```
   recentZero = evalHistory.slice(-2).every(c => c.instinctsExtracted === 0)
   if (recentZero):
     → run forced instinct extraction prompt (MemRL/MemEvolve pattern):
       "For each of the last N cycle's tasks, identify:
        (1) what approach was used,
        (2) what the audit found,
        (3) what a future agent should do differently — even under uniform success.
        Write at least one instinct per cycle. No new instincts = extraction stall."
     → this extraction block MUST produce ≥1 instinct before continuing to step 4
   ```
   This forces instinct generation when consecutive cycles with no new instincts
   signal that passive extraction has stalled under uniform success conditions.

4. **Instinct Extraction:**
   Read ALL workspace files from this cycle and think deeply about patterns:

   - **Successful patterns** — What approach worked? Why? Would it work again?
   - **Failed patterns** — What didn't work? What was the root cause? How to avoid it?
   - **Domain knowledge** — What did we learn about this specific codebase?
   - **Process insights** — Was the task sizing right? Were the evals effective?

   Write instinct files to `.claude/evolve/instincts/personal/`:
   ```yaml
   - id: inst-<NNN>
     pattern: "<short-name>"
     description: "<what was learned>"
     confidence: <0.5-1.0>  # starts at 0.5, increases with confirmation
     source: "cycle-<N>/<task-slug>"
     type: "anti-pattern|successful-pattern|convention|architecture|domain|process|technique"
     category: "episodic|semantic|procedural"
   ```

   **Category assignment:**
   - Episodic: anti-pattern, successful-pattern (things that happened)
   - Semantic: convention, architecture, domain (knowledge about the codebase)
   - Procedural: process, technique (how to do things)

   **Think hard about instincts.** Each one should be specific enough to be actionable in future cycles. "Code should be clean" is useless. "This codebase uses barrel exports in index.ts files — always add new exports there" is useful.

   Update state.json `instinctCount`.

   **Update `instinctSummary` in state.json** (compact array so agents read summary instead of all YAML files):
   ```json
   "instinctSummary": [
     {"id": "inst-004", "pattern": "grep-based-evals", "confidence": 0.95, "type": "technique"},
     {"id": "inst-007", "pattern": "inline-s-tasks", "confidence": 0.9, "type": "process", "graduated": true}
   ]
   ```
   Scout and Builder read `instinctSummary` from state.json instead of reading all instinct YAML files. Full instinct files are only read during consolidation (every 3 cycles) or when `instinctCount` has changed since last cycle.

   **Self-Evaluation (LLM-as-a-Judge)** (after instinct extraction):
   Score the cycle on 4 dimensions using a structured rubric. For each dimension, write a chain-of-thought justification BEFORE assigning the score. Binary threshold: ≥0.7 = pass, <0.7 = fail (triggers mandatory instinct extraction for that dimension).

   | Dimension | Guiding questions | Threshold |
   |-----------|------------------|-----------|
   | **Correctness** | Did the build produce the intended behavior? Did evals pass? | ≥0.7 |
   | **Completeness** | Were all acceptance criteria addressed? No missing edge cases? | ≥0.7 |
   | **Novelty** | Did the cycle surface new patterns, techniques, or knowledge? | ≥0.7 |
   | **Efficiency** | Were tokens, attempts, and file changes minimized? Was scope right-sized? | ≥0.7 |

   Scoring protocol:
   1. For each dimension: write 1–2 sentences of step-by-step reasoning (what happened, what evidence exists)
   2. Assign a score 0.0–1.0 based on that justification
   3. If any dimension scores <0.7: extract at least one instinct from that failure before moving on

   Record scores in `workspace/build-report.md` under a `## Self-Evaluation` heading.

   **Gene Extraction** (after instinct extraction):
   If the Builder successfully fixed a recurring error pattern this cycle:
   - Extract the fix as a gene with selector, steps, and validation commands
   - Write to `.claude/evolve/genes/<gene-id>-<name>.yaml`
   - If multiple genes were applied in sequence, bundle as a capsule
   - See [docs/genes.md](docs/genes.md) for schema

   **Instinct global promotion** (check after every instinct extraction):
   For instincts with confidence >= 0.8 that are not project-specific:
   1. Copy to `~/.claude/instincts/personal/<instinct-id>.yaml`
   2. Add `promotedFrom` field with project name and cycle
   3. Log promotion in the ledger as `type: "instinct-promotion"`

   **Memory Consolidation** (every 3 cycles or when instinctCount > 20):
   Review all instinct files and consolidate:

   a. **Cluster similar instincts:** Find instincts with overlapping patterns or descriptions (semantic similarity > 0.85). Merge them into a single higher-level abstraction.
      - Example: `inst-003: "use camelCase for API keys"` + `inst-007: "use camelCase for config fields"` → `inst-003: "use camelCase for all JSON keys in this codebase"` (confidence = max of originals)

   b. **Archive originals:** Move merged instincts to `.claude/evolve/instincts/archived/` with a `supersededBy` field. Never delete — only archive.

   c. **Apply temporal decay:** Instincts not referenced in the last 5 cycles have their confidence reduced by 0.1 per consolidation pass. Instincts reaching confidence < 0.3 are archived as stale.

   d. **Entropy gating:** Before storing a new instinct, check if it adds meaningful information beyond what's already stored. If a new instinct is >90% similar to an existing one, update the existing one's confidence instead of creating a duplicate.

   e. **Write consolidation log** to `workspace/consolidation-log.md`:
      ```markdown
      ## Memory Consolidation — Cycle {N}
      - Instincts before: <count>
      - Merged: <count> clusters
      - Decayed: <count>
      - Archived: <count>
      - Instincts after: <count>
      ```

5. **Counterfactual Accuracy Review** (optional, shadow-run check):
   For any task completed this cycle that previously had a `counterfactual` entry in `evaluatedTasks`, compare the prediction to the actual outcome:
   - Did `predictedComplexity` match the actual complexity?
   - Did `estimatedReward` (predicted) align with the actual build outcome (PASS=1.0, FAIL=0.0, partial=0.5)?
   - Was the `alternateApproach` viable? (Did the Builder use a similar or different path?)

   Log accuracy notes as an instinct if a clear pattern emerges (e.g., "Scout consistently over-estimates complexity for config tasks"). No action required if no completed counterfactuals exist this cycle.

6. **Operator Check:**
   Launch **Operator Agent** (model: per routing table — haiku default, sonnet if HALT suspected; subagent_type: `general-purpose`):
   - Context:
     ```json
     {
       // --- Static ---
       "workspacePath": ".claude/evolve/workspace/",
       // --- Semi-stable ---
       "stateJson": <state.json contents — includes ledgerSummary and instinctSummary>,
       // --- Dynamic ---
       "cycle": <N>,
       "mode": "post-cycle|convergence-check",
       "recentLedger": "<last 5 ledger entries, inline>",
       "recentNotes": "<last 5 cycle entries from notes.md, inline>"
     }
     ```
   - Operator reads `ledgerSummary` and `instinctSummary` from state.json instead of full ledger/instinct files.
   - In `"convergence-check"` mode: Operator checks for external changes (`git log --oneline -3`), new issues, or changed project state. If new work detected, reset `nothingToDoCount` to 0.
   - Operator assesses: Did we ship? Are we stalling? Cost concerns? Recommendations?
   - Operator writes `workspace/next-cycle-brief.json` with `weakestDimension`, `recommendedStrategy`, `taskTypeBoosts`, `avoidAreas`, and `cycle` — consumed by Scout in Phase 1 of the next cycle.
   - If status is `HALT` → pause and present issues to user

   **Cost awareness check** (inline, before launching Operator):
   - If current cycle number >= `warnAfterCycles` (from state.json, default 5): include warning in Operator context

   **Update lastCycleNumber** in state.json to the current cycle number after each cycle completes.

6. **Update notes.md** (rolling window — keeps file size bounded):

   Append the new cycle entry:
   ```markdown
   ## Cycle {N} — {date}
   - **Tasks:** <list of what was built>
   - **Audit:** <verdict>
   - **Eval:** <passed/total>
   - **Shipped:** YES / NO
   - **Instincts:** <count> extracted
   - **Next cycle should consider:** <recommendations>
   ```

   **Notes Compression** (every 5 cycles, aligned with meta-cycle):
   If `cycle % 5 === 0`:
   1. **Pre-compression memory flush** (inspired by OpenClaw's pre-compaction flush):
      Before compressing, extract durable items from old entries into state.json:
      - Deferred tasks → add to `evaluatedTasks` with `decision: "deferred"` and `revisitAfter`
      - Unresolved decisions/blockers → add to `operatorWarnings`
      - Recurring recommendations → validate they're captured in instincts
      This prevents information loss that a ~500-byte summary can't capture.
   2. Compress entries older than 5 cycles into a fixed-size `## Summary` section at the top (~500 bytes: total tasks shipped, key milestones, count of active deferred items)
   3. Rewrite notes.md with: `## Summary (cycles 1 through N-5)` + last 5 cycle entries only
   4. Full history is preserved in `history/cycle-N/` archives
   5. Use haiku model for the compression summarization (it's a straightforward summarization task)

   This caps notes.md at ~5KB regardless of cycle count.

7. **Output cycle summary:**
   ```
   CYCLE {N} COMPLETE
   ==================
   Tasks:     <list>
   Audit:     <verdict>
   Eval:      <passed/total>
   Shipped:   YES / NO
   Instincts: <count>
   ```

7. **Meta-Cycle Self-Improvement** (every 5 cycles):
   If `cycle % 5 === 0`, run a meta-evaluation of the evolve-loop's own effectiveness:

   a. **Collect metrics** from the last 5 cycles in `evalHistory` and `ledger.jsonl`:
      - Tasks shipped vs attempted (success rate)
      - Average audit iterations per task (Builder efficiency)
      - Stagnation pattern count
      - Instinct confidence trend (are instincts getting confirmed?)

   b. **Split-role critique** — use three specialized critic perspectives to avoid blind spots:

      | Critic | Focus | Key Question |
      |--------|-------|-------------|
      | **Efficiency Critic** | Cost, token usage, task sizing, model routing | "Are we spending tokens wisely? Could tasks be smaller?" |
      | **Correctness Critic** | Eval pass rates, audit verdicts, regression trends | "Are we shipping quality code? Are evals catching issues?" |
      | **Novelty Critic** | Instinct diversity, task variety, stagnation patterns | "Are we learning new things? Or repeating the same work?" |

      Each critic reviews the last 5 cycles independently and produces 1-3 findings. The orchestrator synthesizes findings into the meta-review, resolving conflicts by prioritizing correctness > efficiency > novelty.

   c. **Evaluate agent effectiveness** — for each agent, ask:
      - Scout: Are selected tasks the right size? Are they shipping?
      - Builder: How many attempts per task? What's the self-verify pass rate?
      - Auditor: Are WARN/FAIL verdicts being resolved or accumulating?
      - Operator: Are recommendations being followed?

   c. **Propose improvements** — write a `meta-review.md` to the workspace:
      ```markdown
      # Meta-Cycle Review — Cycles {N-4} to {N}

      ## Pipeline Metrics
      - Success rate: X/Y tasks (Z%)
      - Avg audit iterations: N
      - Stagnation patterns: N active
      - Instinct trend: growing/stable/stale

      ## Agent Effectiveness
      | Agent | Assessment | Suggested Change |
      |-------|-----------|-----------------|
      | Scout | ... | ... |
      | Builder | ... | ... |
      | Auditor | ... | ... |
      | Operator | ... | ... |

      ## Recommended Changes
      1. <specific change to agent prompt, strategy, or process>
      ```

   d. **Automated Prompt Evolution** — based on meta-review findings, the orchestrator may refine agent prompts using a critique-synthesize loop:

      1. **Critique:** Identify specific weaknesses in agent prompts based on cycle outcomes. For example, if the Builder frequently needs 3 attempts, its design step may need stronger guidance.
      2. **Synthesize:** Propose specific prompt edits (additions, rewording, new examples) that address the weakness. Each edit must be small and targeted — do not rewrite entire agent definitions.
      3. **Validate:** Before applying, check that the proposed edit doesn't contradict existing instincts or orchestrator policies.
      4. **Apply:** Make the edit to the agent file. Log the change in the meta-review with before/after and rationale.
      5. **Track:** Add a `prompt-evolution` entry to the ledger:
         ```json
         {"ts":"<ISO-8601>","cycle":<N>,"role":"orchestrator","type":"prompt-evolution","data":{"agent":"<name>","section":"<section changed>","rationale":"<why>","change":"<summary>"}}
         ```

      **TextGrad-style optimization:** For each proposed edit, generate a "textual gradient" — a natural language critique describing:
      - What the current prompt produces (observed behavior)
      - What it should produce (desired behavior)
      - The specific text change that bridges the gap (the "gradient")
      - Expected impact on process rewards for the affected phase

      This is more rigorous than free-form critique. The gradient must reference specific prompt text and specific cycle outcomes.

      **Safety constraints:**
      - Only modify non-structural sections (guidance, examples, strategy handling) — never change the agent's tools, model, or core responsibilities
      - Maximum 2 prompt edits per meta-cycle
      - All edits are committed and can be reverted with `git revert`
      - If an evolved prompt leads to worse performance in the next meta-cycle, auto-revert the change

   e. **Self-Generated Evaluation (mutation testing):**

      Test the quality of our evals by generating mutations:
      1. For each task completed in the last 5 cycles, generate 2-3 small code mutations (e.g., remove a validation, change a return value, delete an import)
      2. Run the existing eval graders against the mutated code
      3. If evals DON'T catch a mutation → the eval is weak. Propose stronger eval criteria.
      4. Track **mutation kill rate** (mutations caught / mutations generated)

      ```markdown
      ## Mutation Testing Results
      - Mutations generated: <N>
      - Mutations killed (caught by evals): <N>
      - Kill rate: <percentage>
      - Weak evals identified: <list>
      - Proposed improvements: <list>
      ```

      Target: >80% mutation kill rate. Below 60% triggers eval improvement as a priority task in the next cycle.

   f. **Workflow Topology Review:**

      Evaluate whether the current phase ordering and agent configuration is optimal:

      1. **Phase skip analysis** — were any phases redundant this meta-cycle? (e.g., Auditor always PASS → consider lighter audit)
      2. **Phase merge candidates** — could two phases be combined? (e.g., if Builder self-verify is reliable, reduce Auditor scope)
      3. **Phase addition candidates** — is there a gap? (e.g., if security issues keep recurring, add a dedicated security scan phase)
      4. **Parallel opportunities** — could independent tasks be built in parallel instead of sequentially?

      Propose topology changes in the meta-review:
      ```markdown
      ## Topology Recommendations
      - **Current:** DISCOVER → BUILD → AUDIT → SHIP → LEARN
      - **Proposed:** DISCOVER → BUILD(parallel) → AUDIT(light) → SHIP → LEARN
      - **Rationale:** <why this change would improve performance>
      ```

      **Safety:** Topology changes are proposals only — they require human approval before the orchestrator applies them. Never auto-apply topology changes.

   g. **Apply remaining changes** — update default strategy, token budgets, or other configuration based on meta-review findings. Archive the `meta-review.md` to history.

   h. **Regenerate project digest** — during meta-cycle (every 5 cycles), regenerate `workspace/project-digest.md` to capture any structural changes.

8. **Project Digest Generation** (cycle 1, or every 10 cycles during meta-cycle):

   Generate `.claude/evolve/workspace/project-digest.md` (~2-3KB):
   ```markdown
   # Project Digest — Generated Cycle {N}

   ## Structure
   <project directory tree with file sizes, max 2 levels deep>

   ## Tech Stack
   - Language: <detected>
   - Framework: <detected>
   - Test command: <detected>
   - Build command: <detected>

   ## Hotspots
   <files with highest fan-in: most imported/referenced by other files>
   <largest files by line count>
   <files with most recent churn: git log --format='%H' --follow -- <file> | wc -l>
   These are high-impact targets for Scout task selection — changes here have large blast radius.

   ## Conventions
   <key patterns detected: naming, file org, exports, etc.>

   ## Recent History
   <git log --oneline -10>
   ```

   On cycle 1 (`mode: "full"`): Scout generates this after full codebase scan.
   On cycle 2+: Scout reads this instead of re-scanning. Only changed files (from `changedFiles`) are read directly.

9. **Context Management (stop-hook pattern):**

   After each cycle completes, assess context window usage. If context is above 60% capacity:
   - Write a **cycle handoff file** to `.claude/evolve/workspace/handoff.md`:
     ```markdown
     # Cycle Handoff — Cycle {N}

     ## Session State
     - Cycles completed this session: <N>
     - Strategy: <strategy>
     - Goal: <goal or null>
     - Remaining cycles: <endCycle - currentCycle>

     ## Key Context to Carry Forward
     - Active stagnation patterns: <list>
     - Unresolved operator warnings: <list>
     - Last delta metrics: <summary>

     ## Resume Command
     `/evolve-loop <remaining cycles> [strategy] [goal]`
     ```
   - Output the resume command to the user
   - STOP the current session gracefully

   This prevents context exhaustion mid-cycle. The handoff file ensures the next session has all context needed to continue seamlessly.

10. **Exit conditions** (in order):
   - Cycle limit reached → STOP
   - Convergence (`stagnation.nothingToDoCount >= 3`) → STOP
   - Context above 60% after a cycle → write handoff, STOP
   - Otherwise → next cycle
