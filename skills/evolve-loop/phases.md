# Evolve Loop — Phase Instructions

Detailed orchestrator instructions for each phase. Optimized for fast iteration with diverse small/medium tasks per cycle.

**Important:** Every agent context block must include `goal` (string or null) and `strategy` (one of: `balanced`, `innovate`, `harden`, `repair`).

## FOR cycle = {startCycle} to {endCycle}:

### Phase 1: DISCOVER

Launch **Scout Agent** (model: per routing table — sonnet default, haiku for incremental, opus for deep research; subagent_type: `general-purpose`):
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
    "goal": <goal or null>,
    "strategy": <strategy>
  }
  ```

After Scout completes:
- Read `workspace/scout-report.md`
- Verify eval definitions were created in `.claude/evolve/evals/`
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

Launch **Builder Agent** (model: per routing table — sonnet default, opus for complex M tasks, haiku for S-complexity; subagent_type: `general-purpose`, isolation: `worktree`):
- Prompt: Read `agents/evolve-builder.md` and pass as prompt
- Context:
  ```json
  {
    "cycle": <N>,
    "task": <task object from scout-report>,
    "workspacePath": ".claude/evolve/workspace/",
    "ledgerPath": ".claude/evolve/ledger.jsonl",
    "instinctsPath": ".claude/evolve/instincts/personal/",
    "evalsPath": ".claude/evolve/evals/",
    "strategy": <strategy>
  }
  ```

After Builder completes:
- Read `workspace/build-report.md`
- If status is FAIL after 3 attempts:
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

---

### Phase 3: AUDIT

Launch **Auditor Agent** (model: per routing table — sonnet default, opus for security-sensitive, haiku for clean builds; subagent_type: `general-purpose`):
- Prompt: Read `agents/evolve-auditor.md` and pass as prompt
- Context:
  ```json
  {
    "cycle": <N>,
    "workspacePath": ".claude/evolve/workspace/",
    "ledgerPath": ".claude/evolve/ledger.jsonl",
    "evalsPath": ".claude/evolve/evals/",
    "buildReport": ".claude/evolve/workspace/build-report.md",
    "strategy": <strategy>
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
   - Reset `stagnation.nothingToDoCount` to 0
   - Update `lastUpdated`
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
     type: "anti-pattern|successful-pattern|convention|architecture|domain|process|technique"
     category: "episodic|semantic|procedural"
   ```

   **Category assignment:**
   - Episodic: anti-pattern, successful-pattern (things that happened)
   - Semantic: convention, architecture, domain (knowledge about the codebase)
   - Procedural: process, technique (how to do things)

   **Think hard about instincts.** Each one should be specific enough to be actionable in future cycles. "Code should be clean" is useless. "This codebase uses barrel exports in index.ts files — always add new exports there" is useful.

   Update state.json `instinctCount`.

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

3. **Operator Check:**
   Launch **Operator Agent** (model: per routing table — haiku default, sonnet if HALT suspected; subagent_type: `general-purpose`):
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

6. **Meta-Cycle Self-Improvement** (every 5 cycles):
   If `cycle % 5 === 0`, run a meta-evaluation of the evolve-loop's own effectiveness:

   a. **Collect metrics** from the last 5 cycles in `evalHistory` and `ledger.jsonl`:
      - Tasks shipped vs attempted (success rate)
      - Average audit iterations per task (Builder efficiency)
      - Stagnation pattern count
      - Instinct confidence trend (are instincts getting confirmed?)

   b. **Evaluate agent effectiveness** — for each agent, ask:
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

      **Safety constraints:**
      - Only modify non-structural sections (guidance, examples, strategy handling) — never change the agent's tools, model, or core responsibilities
      - Maximum 2 prompt edits per meta-cycle
      - All edits are committed and can be reverted with `git revert`
      - If an evolved prompt leads to worse performance in the next meta-cycle, auto-revert the change

   e. **Apply remaining changes** — update default strategy, token budgets, or other configuration based on meta-review findings. Archive the `meta-review.md` to history.

7. **Context Management (stop-hook pattern):**

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

8. **Exit conditions** (in order):
   - Cycle limit reached → STOP
   - Convergence (`stagnation.nothingToDoCount >= 3`) → STOP
   - Context above 60% after a cycle → write handoff, STOP
   - Otherwise → next cycle
