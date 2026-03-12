# Evolve Loop — Phase Instructions

Detailed orchestrator instructions for each phase. Optimized for fast iteration with diverse small/medium tasks per cycle.

**Important:** Every agent context block must include `goal` (string or null).

## FOR cycle = {startCycle} to {endCycle}:

### Phase 1: DISCOVER

Launch **Scout Agent** (model: sonnet, subagent_type: `general-purpose`):
- Prompt: Read `agents/evolve-scout.md` and pass as prompt
- Context:
  ```json
  {
    "cycle": <N>,
    "projectContext": <auto-detected>,
    "stateJson": <state.json contents>,
    "notesPath": ".claude/evolve/notes.md",
    "workspacePath": ".claude/evolve/workspace/",
    "ledgerPath": ".claude/evolve/ledger.jsonl",
    "instinctsPath": ".claude/evolve/instincts/personal/",
    "goal": <goal or null>
  }
  ```

After Scout completes:
- Read `workspace/scout-report.md`
- Verify eval definitions were created in `.claude/evolve/evals/`
- Merge research query updates into state.json (if research was performed)
- If no tasks selected:
  - Increment `nothingToDoCount` in state.json
  - If `nothingToDoCount >= 3` → STOP: "Project has converged."
  - Otherwise → skip to Phase 5

---

### Phase 2: BUILD (loop per task)

For each task in the Scout's selected task list:

Launch **Builder Agent** (model: sonnet, subagent_type: `general-purpose`, isolation: `worktree`):
- Prompt: Read `agents/evolve-builder.md` and pass as prompt
- Context:
  ```json
  {
    "cycle": <N>,
    "task": <task object from scout-report>,
    "workspacePath": ".claude/evolve/workspace/",
    "ledgerPath": ".claude/evolve/ledger.jsonl",
    "instinctsPath": ".claude/evolve/instincts/personal/",
    "evalsPath": ".claude/evolve/evals/"
  }
  ```

After Builder completes:
- Read `workspace/build-report.md`
- If status is FAIL after 3 attempts:
  - Log failed approach in state.json under `failedApproaches`
  - Skip this task, proceed to next task (or Phase 3 if last task)
- If status is PASS → proceed to Phase 3 for this task

---

### Phase 3: AUDIT

Launch **Auditor Agent** (model: sonnet, subagent_type: `general-purpose`):
- Prompt: Read `agents/evolve-auditor.md` and pass as prompt
- Context:
  ```json
  {
    "cycle": <N>,
    "workspacePath": ".claude/evolve/workspace/",
    "ledgerPath": ".claude/evolve/ledger.jsonl",
    "evalsPath": ".claude/evolve/evals/",
    "buildReport": ".claude/evolve/workspace/build-report.md"
  }
  ```

After Auditor completes:
- Read `workspace/audit-report.md`
- **Verdict handling:**
  - **PASS** → proceed to commit this task
  - **WARN** (MEDIUM issues found) → re-launch Builder with issues, re-audit (max 3 total iterations)
  - **FAIL** (CRITICAL/HIGH or eval failures) → re-launch Builder with issues, re-audit (max 3 total iterations)
  - After 3 failures → log as failed approach, skip this task

**Commit on PASS:**
```bash
git add -A
git commit -m "<type>: <description>"
```

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

4. **Update state.json:**
   - Mark completed tasks in `evaluatedTasks`
   - Update `lastCycleNumber` to current cycle number
   - Reset `nothingToDoCount` to 0
   - Update `lastUpdated`
   - Add eval results to `evalHistory`

---

### Phase 5: LEARN (orchestrator inline + operator)

1. **Archive workspace:**
   ```bash
   mkdir -p .claude/evolve/history/cycle-{N}
   cp .claude/evolve/workspace/*.md .claude/evolve/history/cycle-{N}/
   ```

2. **Instinct Extraction:**
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
     type: "success|failure|domain|process"
   ```

   **Think hard about instincts.** Each one should be specific enough to be actionable in future cycles. "Code should be clean" is useless. "This codebase uses barrel exports in index.ts files — always add new exports there" is useful.

   Update state.json `instinctCount`.

3. **Operator Check:**
   Launch **Operator Agent** (model: sonnet, subagent_type: `general-purpose`):
   - Context: cycle number, mode=`post-cycle`, state.json, paths to workspace/ledger
   - Operator assesses: Did we ship? Are we stalling? Cost concerns? Recommendations?
   - If status is `HALT` → pause and present issues to user

   **Cycle cap check** (inline, before launching Operator):
   - If current cycle number > `maxCyclesPerSession` (from state.json, default 10): HALT — "Cumulative cycle cap reached ({maxCyclesPerSession}). Stop and review before continuing."
   - If current cycle number >= `warnAfterCycles` (from state.json, default 5): include warning in Operator context

   **Update lastCycleNumber** in state.json to the current cycle number after each cycle completes.

4. **Update notes.md** (always append, never overwrite):
   ```markdown
   ## Cycle {N} — {date}
   - **Tasks:** <list of what was built>
   - **Audit:** <verdict>
   - **Eval:** <passed/total>
   - **Shipped:** YES / NO
   - **Instincts:** <count> extracted
   - **Next cycle should consider:** <recommendations>
   ```

5. **Output cycle summary:**
   ```
   CYCLE {N} COMPLETE
   ==================
   Tasks:     <list>
   Audit:     <verdict>
   Eval:      <passed/total>
   Shipped:   YES / NO
   Instincts: <count>
   ```

6. **Exit conditions** (in order):
   - Cycle limit reached → STOP
   - Convergence (`nothingToDoCount >= 3`) → STOP
   - Context exhaustion → suggest continuing in fresh session
   - Otherwise → next cycle
