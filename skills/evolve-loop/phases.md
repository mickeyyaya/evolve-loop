# Evolve Loop — Phase Instructions

Detailed orchestrator instructions for each phase. The orchestrator reads this when executing the cycle loop.

**Important:** Every agent context block must include `goal` (string or null). When goal is non-null, agents focus their work toward achieving it. When null, agents perform broad autonomous discovery.

## FOR cycle = 1 to {cycles}:

### Phase 0: MONITOR-INIT (Loop Operator pre-flight)

Launch **Operator Agent** (model: sonnet, subagent_type: `general-purpose`):
- Prompt: Include the `evolve-operator` agent instructions from `~/.claude/agents/evolve-operator.md`
- Context: cycle number, mode=`pre-flight`, state.json contents, paths to workspace/ledger
- The Operator verifies quality gates, eval baseline, rollback path, worktree isolation, cost budget

After Operator completes:
- Read `workspace/loop-operator-log.md`
- If status is `HALT`:
  - Present all issues to the user
  - Wait for user decision: `continue` (override), `fix` (address issues), or `abort` (stop loop)
  - Do NOT proceed automatically past a HALT

---

### Phase 1: DISCOVER (3 agents in parallel)

Launch **three agents in parallel** (single message, three Agent tool calls):

1. **PM Agent** (model: sonnet, subagent_type: `general-purpose`)
   - Prompt: Include the `evolve-pm` agent instructions from `~/.claude/agents/evolve-pm.md`
   - Context: cycle number, projectContext, state.json contents, paths to workspace/ledger/notes, **goal**
   - **With goal:** The PM focuses assessment on dimensions relevant to the goal, identifies what exists and what's missing to achieve it
   - **Without goal:** The PM assesses the project broadly across all 8 dimensions, triages backlog

2. **Researcher Agent** (model: sonnet, subagent_type: `general-purpose`)
   - Prompt: Include the `evolve-researcher` agent instructions from `~/.claude/agents/evolve-researcher.md`
   - Context: cycle number, projectContext, state.json contents (research TTLs), paths to workspace/ledger, **goal**
   - **With goal:** The Researcher searches for best practices, libraries, patterns, and prior art specifically for achieving the goal
   - **Without goal:** The Researcher searches broadly for trends, competitors, security advisories, best practices

3. **Scanner Agent** (model: sonnet, subagent_type: `general-purpose`)
   - Prompt: Include the `evolve-scanner` agent instructions from `~/.claude/agents/evolve-scanner.md`
   - Context: cycle number, projectContext, state.json contents, paths to workspace/ledger, **goal**
   - **With goal:** The Scanner focuses on code areas the goal will likely touch — dependencies, interfaces, test coverage in those areas
   - **Without goal:** The Scanner does a full codebase audit (quality, dependencies, hotspots, coverage)

**WAIT** for all three to complete before proceeding.

After all complete:
- Read `workspace/briefing.md`, `workspace/research-report.md`, and `workspace/scan-report.md` to verify they were written
- Merge any state.json updates from the Researcher (new research queries with TTLs)
- **GATE: Commit** any doc updates: `docs: update project docs with cycle {N} findings`

---

### Phase 2: PLAN

Launch **Planner Agent** (model: opus, subagent_type: `general-purpose`):
- Prompt: Include the `evolve-planner` agent instructions from `~/.claude/agents/evolve-planner.md`
- Context: cycle number, state.json, paths to workspace/ledger/notes, **goal**
- The Planner reads ALL three Phase 1 workspace files (briefing + research-report + scan-report) and synthesizes them
- The Planner also reads instincts from `.claude/evolve/instincts/personal/` if they exist
- The Planner writes eval definitions to `.claude/evolve/evals/` for each selected task
- **With goal:** The Planner MUST select tasks that directly advance the goal. Other improvements are deprioritized.
- **Without goal:** The Planner picks the highest-impact work across all dimensions.

After Planner completes:
- Read `workspace/backlog.md`
- Verify eval definitions were created in `.claude/evolve/evals/`
- If no tasks selected (nothing to do):
  - Increment `nothingToDoCount` in state.json
  - If `nothingToDoCount >= 3` → STOP: "No features left to implement. The project has converged."
  - Otherwise → skip to Phase 7

**GATE: User Approval**
- **Autonomous mode** (bypass-permissions enabled): proceed directly
- **Interactive mode**: Present the selected tasks from `workspace/backlog.md` to the user. Ask for confirmation. If rejected → record in state.json, STOP cycle.

---

### Phase 3: DESIGN

Launch **Architect Agent** (model: opus, subagent_type: `general-purpose`):
- Prompt: Include the `evolve-architect` agent instructions from `~/.claude/agents/evolve-architect.md`
- Context: cycle number, paths to workspace/ledger
- The Architect produces ADRs for significant decisions, implementation order, and testing strategy

After Architect completes:
- Read `workspace/design.md` to verify it was written

---

### Phase 4: BUILD (in worktree)

Launch **Developer Agent** (model: sonnet, subagent_type: `general-purpose`, isolation: `worktree`):
- Prompt: Include the `evolve-developer` agent instructions from `~/.claude/agents/evolve-developer.md`
- Context: cycle number, paths to workspace/ledger, branch name
- The Developer reads instincts before coding, follows eval-driven TDD, and runs de-sloppify pass

After Developer completes:
- Read `workspace/impl-notes.md`
- If status is FAIL after 3 attempts:
  - Log failed approach in state.json under `failedApproaches` with error context and suggested alternative
  - Mark task as rejected with `revisitAfter` = 7 days from now
  - Skip to Phase 7

---

### Phase 4.5: CHECKPOINT (Loop Operator mid-cycle)

Launch **Operator Agent** (model: sonnet, subagent_type: `general-purpose`):
- Prompt: Include the `evolve-operator` agent instructions from `~/.claude/agents/evolve-operator.md`
- Context: cycle number, mode=`checkpoint`, state.json contents, paths to workspace/ledger

After Operator completes:
- Read `workspace/loop-operator-log.md` (appended checkpoint section)
- If status is `HALT`:
  - Present issues to user
  - Wait for user decision: `continue`, `fix`, or `abort`

---

### Phase 5: VERIFY (3 agents in parallel — review barrier)

Launch **three agents in parallel** (single message, three Agent tool calls):

1. **Reviewer Agent** (model: sonnet, subagent_type: `general-purpose`)
   - Prompt: Include the `evolve-reviewer` agent instructions from `~/.claude/agents/evolve-reviewer.md`
   - Context: cycle number, diff command, paths to workspace/ledger
   - **READ-ONLY** — must not edit source code

2. **E2E Runner Agent** (model: sonnet, subagent_type: `general-purpose`)
   - Prompt: Include the `evolve-e2e` agent instructions from `~/.claude/agents/evolve-e2e.md`
   - Context: cycle number, projectContext, diff command, paths to workspace/ledger

3. **Security Reviewer Agent** (model: sonnet, subagent_type: `general-purpose`)
   - Prompt: Include the `evolve-security` agent instructions from `~/.claude/agents/evolve-security.md`
   - Context: cycle number, diff command, paths to workspace/ledger
   - **READ-ONLY** — must not edit source code

**WAIT** for all three to complete.

After all complete:
- Read `workspace/review-report.md`, `workspace/e2e-report.md`, and `workspace/security-report.md`
- If ANY has blocking issues (FAIL verdict):
  - Re-launch Developer agent with fix instructions (include all blocking issues from all reports)
  - Re-run only the failed reviewer(s) — if multiple failed, re-run those in parallel
  - Repeat until all PASS or WARN (max 3 iterations total across VERIFY+EVAL)

---

### Phase 5.5: EVAL (Hard Gate)

Follow the instructions in [eval-runner.md](eval-runner.md).

**Summary:**
1. Read eval definitions from `.claude/evolve/evals/`
2. Run all code graders, regression evals, and acceptance checks
3. Write `workspace/eval-report.md`
4. If PASS → proceed to Phase 6
5. If FAIL → retry loop (re-launch Developer → re-run VERIFY → re-run EVAL, max 3 total)
6. If still FAIL after 3 → log failed approach, skip Phase 6, proceed to Phase 7

Update state.json `evalHistory` with this cycle's results.

---

### Phase 6: SHIP

Launch **Deployer Agent** (model: sonnet, subagent_type: `general-purpose`):
- Prompt: Include the `evolve-deployer` agent instructions from `~/.claude/agents/evolve-deployer.md`
- Context: cycle number, projectContext, state.json, task name, paths to workspace/ledger
- The Deployer verifies eval-report.md verdict is PASS before proceeding

After Deployer completes:
- Read `workspace/deploy-log.md`
- If CI failed after 3 attempts → STOP cycle
- Merge state.json updates (task completion, reset nothingToDoCount)

---

### Phase 7: LOOP+LEARN

1. **Archive workspace** — Copy `workspace/` to `history/cycle-{N}/`:
   ```bash
   cp -r .claude/evolve/workspace .claude/evolve/history/cycle-{N}
   ```

2. **Instinct Extraction (Learning Pass):**
   - Read ALL workspace files from this cycle (briefing, research, scan, backlog, design, impl-notes, review-report, e2e-report, security-report, eval-report, deploy-log)
   - Extract patterns:
     - **Successful patterns** — What worked well? (approaches, tools, libraries)
     - **Failed patterns** — What didn't work? (anti-patterns, pitfalls)
     - **Repeated tool sequences** — Common workflows worth encoding
     - **Domain knowledge** — Project-specific insights
   - Write instinct files to `.claude/evolve/instincts/personal/` in YAML format:
     ```yaml
     name: <pattern-name>
     type: success|failure|workflow|domain
     confidence: 0.5  # starts at 0.5, increases with repetition
     source_cycle: <N>
     observation: <what was observed>
     recommendation: <what to do or avoid>
     ```
   - Update state.json `instinctCount`
   - After 5+ cycles: promote instincts with confidence >= 0.8 to `~/.claude/homunculus/instincts/personal/`

3. **Loop Operator post-cycle:**
   Launch **Operator Agent** (model: sonnet, subagent_type: `general-purpose`):
   - Context: cycle number, mode=`post-cycle`, state.json, paths to workspace/ledger
   - Operator assesses cycle health, detects stalls, provides recommendations
   - Merge operator warnings into state.json

4. **Update notes.md** — Append cycle summary (always append, never overwrite):
   ```markdown
   ## Cycle {N} — {date}
   - **Task:** {what was built}
   - **Coverage:** {X%}
   - **Review:** {verdict} ({X} blocking, {Y} warnings)
   - **E2E:** {verdict} ({acceptance criteria passed}/{total})
   - **Security:** {verdict} ({critical}/{high}/{medium} issues)
   - **Eval:** {verdict} ({passed}/{total} checks)
   - **Research:** {key external findings}
   - **Deploy:** {status}
   - **Instincts extracted:** {count}
   - **Operator:** {recommendations}
   - **Next cycle should consider:** {recommendations}
   ```

5. **Output cycle summary** to user:
   ```
   CYCLE {N} COMPLETE
   ==================
   Task:      {what was built}
   Coverage:  {X%}
   Review:    {verdict}
   E2E:       {verdict}
   Security:  {verdict}
   Eval:      {verdict}
   Research:  {top finding}
   Deploy:    {status}
   Instincts: {count} extracted
   Operator:  {status}
   ```

6. **Check exit conditions** (in order):
   - Cycle limit reached (`cycle >= cycles`) → STOP: "Completed {N}/{cycles} cycles."
   - Completion signal (`nothingToDoCount >= 3`) → STOP: "Project has converged."
   - Context exhaustion → suggest `/evolve-loop {remaining}` in fresh session
   - Otherwise → proceed to next cycle
