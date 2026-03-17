# Cycle 24 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 4 (agents/evolve-scout.md, skills/evolve-loop/SKILL.md, skills/evolve-loop/memory-protocol.md, skills/evolve-loop/phases.md) + agents/evolve-builder.md + agents/evolve-auditor.md
- Research: skipped (cooldown active — last queries at 2026-03-15T15:00:00Z, TTL 12hr)
- Instincts applied: 0 (instinctSummary is empty this cycle)
- **instinctsApplied:** []

## Key Findings

### Features — HIGH (goal-directed: Builder/Auditor/cross-agent improvements)

**Builder has no forward-learning mechanism.**
The Builder reads `instinctSummary` and `planCache` but never produces its own persistent forward-looking signal. After implementing a task, useful implementation-specific knowledge (e.g., "this section of phases.md is unusually brittle near line 412", "the prerequisite check requires a specific evaluatedTasks shape") is lost at cycle end. The `build-report.md` is archived but never re-read by future builders. A lightweight `builder-notes.md` written per cycle and read by Scout in incremental mode would close this loop.

**Auditor applies static strictness regardless of per-type builder track record.**
The Auditor runs the same full checklist on every task regardless of build history. If `feature` tasks have 10 consecutive PASS-first-attempt cycles, the Auditor still runs the identical 5-section checklist. An `auditorProfile` in state.json tracking per-task-type reliability would allow the Auditor to skip well-established checks for proven-reliable types while remaining strict on new or historically-failing types. This mirrors how the bandit mechanism (SKILL.md) learned which task types to prioritize for the Scout.

**No persistent cross-agent message channel.**
Agents communicate only through overwritten workspace files and structured state.json fields. There is no mechanism for the Builder to leave a targeted note for the Scout ("this area needs smaller tasks next time") or for the Auditor to leave a note for the Builder ("remember to validate the YAML frontmatter in agent files"). Each cycle discards these implicit learnings. A lightweight `agent-mailbox.md` in workspace (with a persistent copy appended to `state.json.agentMessages`) would create a durable cross-agent communication channel.

### Techdebt — HIGH (version bump)

**CHANGELOG.md and plugin.json are stale.**
The CHANGELOG shows v6.6.0 as the last entry (dated 2026-03-15), but cycles 22-23 added 6 significant features post-6.6.0:
- Multi-armed bandit task selection (add-bandit-task-selector)
- Counterfactual annotations for deferred tasks (add-counterfactual-cycle-log)
- Semantic task crossover (add-semantic-task-crossover)
- Intrinsic novelty reward (add-intrinsic-novelty-reward)
- Scout decision trace (add-scout-decision-trace)
- Prerequisite task graph (add-prerequisite-task-graph)

These are substantial additions that warrant a 6.7.0 release. SKILL.md header also needs updating (currently says "v6.6").

## Research
Skipped — research cooldown active (queries from 2026-03-15T15:00:00Z have not expired). Goal is partially internal (Builder/Auditor improvements are self-referential); no external research needed for these specific tasks.

## Selected Tasks

### Task 1: Add Builder Retrospective Annotations
- **Slug:** add-builder-retrospective
- **Type:** feature
- **Complexity:** M
- **Rationale:** The Builder currently discards cycle-specific implementation knowledge at cycle end. A retrospective written to `workspace/builder-notes.md` and read by Scout in subsequent incremental cycles creates a direct feedback loop from implementation experience to task planning — parallel to how the Scout's own decision trace feeds the Novelty Critic.
- **Acceptance Criteria:**
  - [ ] `agents/evolve-builder.md` includes a Step 8 (or final step) "Retrospective" that writes `workspace/builder-notes.md` with per-task observations about file fragility, approach surprises, and recommendations for future scouts
  - [ ] `agents/evolve-scout.md` incremental mode reads `builderNotes` from context (analogous to `recentNotes`) and applies it in task selection
  - [ ] `skills/evolve-loop/phases.md` Phase 1 pre-compute context block includes reading `workspace/builder-notes.md` (last cycle) and passing it inline as `builderNotes`
  - [ ] `skills/evolve-loop/memory-protocol.md` workspace file table includes `builder-notes.md` with Owner: Builder
- **Files to modify:** `agents/evolve-builder.md`, `agents/evolve-scout.md`, `skills/evolve-loop/phases.md`, `skills/evolve-loop/memory-protocol.md`
- **Eval:** written to `evals/add-builder-retrospective.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "Retrospective" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md` → expects >= 1
  - `grep -c "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects >= 1
  - `grep -c "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects >= 1
  - `grep -c "builder-notes.md" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects >= 1

---

### Task 2: Add Auditor Adaptive Strictness
- **Slug:** add-auditor-adaptive-strictness
- **Type:** feature
- **Complexity:** M
- **Rationale:** The Auditor's static checklist is appropriate when builder reliability is unknown. But after many cycles of PASS-first-attempt on a task type, applying full strictness to identical patterns wastes tokens and adds latency. An `auditorProfile` tracking per-type reliability enables the Auditor to graduate checks — reducing overhead for proven-reliable types and maintaining/increasing strictness for types with recent failures. This is the Auditor-side analogue of the bandit mechanism on the Scout side.
- **Acceptance Criteria:**
  - [ ] `agents/evolve-auditor.md` includes an "Adaptive Strictness" section describing how to read `auditorProfile` from context and apply reduced check intensity for task types with `passFirstAttempt >= 5`
  - [ ] `skills/evolve-loop/phases.md` Phase 3 (AUDIT) launch context block includes `auditorProfile` from state.json
  - [ ] `skills/evolve-loop/phases.md` Phase 4 (SHIP) state.json update step includes updating `auditorProfile` per-task-type with pass/fail outcome
  - [ ] `skills/evolve-loop/SKILL.md` initialization JSON schema includes `auditorProfile: {}` in the default state
- **Files to modify:** `agents/evolve-auditor.md`, `skills/evolve-loop/phases.md`, `skills/evolve-loop/SKILL.md`
- **Eval:** written to `evals/add-auditor-adaptive-strictness.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "Adaptive Strictness\|auditorProfile\|adaptiveStrictness" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md` → expects >= 2
  - `grep -c "auditorProfile" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects >= 2
  - `grep -c "auditorProfile" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md` → expects >= 1

---

### Task 3: Add Agent Mailbox for Cross-Cycle Messaging
- **Slug:** add-agent-mailbox
- **Type:** feature
- **Complexity:** M
- **Rationale:** Each agent currently loses its implicit cycle-end insights because workspace files are overwritten and state.json fields are structured (no free-form channel). An agent mailbox gives agents a lightweight mechanism to leave typed messages for specific recipients across cycles. This enables Builder→Scout ("prefer smaller tasks in phases.md"), Auditor→Builder ("double-check YAML frontmatter in agent files"), and Operator→Scout ("explore docs/ area more"). Messages are ephemeral by default (consumed once read) but can be marked `persistent` for recurring guidance.
- **Acceptance Criteria:**
  - [ ] `skills/evolve-loop/memory-protocol.md` documents `agent-mailbox.md` in the workspace file table and defines the message schema (from, to, message, type, persistent, cycle)
  - [ ] `agents/evolve-builder.md` includes a step to check the mailbox for messages addressed to "builder" and to post messages addressed to "scout" or "auditor" when warranted
  - [ ] `agents/evolve-auditor.md` includes a step to check the mailbox for messages addressed to "auditor" and to post messages addressed to "builder" when warranted
  - [ ] `agents/evolve-scout.md` incremental mode includes reading mailbox messages addressed to "scout" and incorporating relevant ones into task selection rationale
  - [ ] `skills/evolve-loop/phases.md` Phase 1 pre-compute context reads `agent-mailbox.md` and passes relevant messages inline; Phase 4 SHIP step clears non-persistent messages from mailbox
- **Files to modify:** `skills/evolve-loop/memory-protocol.md`, `agents/evolve-builder.md`, `agents/evolve-auditor.md`, `agents/evolve-scout.md`, `skills/evolve-loop/phases.md`
- **Eval:** written to `evals/add-agent-mailbox.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "agent-mailbox\|mailbox" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects >= 2
  - `grep -c "mailbox" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md` → expects >= 1
  - `grep -c "mailbox" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-auditor.md` → expects >= 1
  - `grep -c "mailbox" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects >= 1
  - `grep -c "mailbox" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects >= 2

---

### Task 4: Bump Version to 6.7.0 with Full Changelog
- **Slug:** bump-version-6-7-0
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** Six features shipped in cycles 22-23 are absent from CHANGELOG.md. The plugin.json version and SKILL.md header are both at 6.6.0/v6.6. This stale documentation state makes it harder to track what version users are running and obscures the pipeline's evolution history. A version bump with proper changelog entries closes the documentation debt accumulated over 6 cycles.
- **Acceptance Criteria:**
  - [ ] `CHANGELOG.md` has a new `[6.7.0]` section documenting all 6 cycle 22-23 features plus the 3 cycle 24 features (Builder retrospective, Auditor adaptive strictness, agent mailbox)
  - [ ] `plugin.json` version field reads `"6.7.0"`
  - [ ] `SKILL.md` header line reads `# Evolve Loop v6.7`
- **Files to modify:** `CHANGELOG.md`, `.claude-plugin/plugin.json`, `skills/evolve-loop/SKILL.md`
- **Eval:** written to `evals/bump-version-6-7-0.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "6.7.0" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md` → expects >= 2
  - `grep -c '"version": "6.7.0"' /Users/danleemh/ai/claude/evolve-loop/.claude-plugin/plugin.json` → expects 1
  - `grep -c "v6.7" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md` → expects 1

## Deferred
- **Cross-agent dependency graph visualization** — would show task prerequisite chains visually; deferred because prerequisite task graph was just added in cycle 23 and needs a few cycles of use before visualization would be meaningful.
- **Builder parallelism (build independent tasks in parallel)** — topology review proposal from meta-cycle; requires significant orchestrator changes; deferred to a dedicated architectural cycle.
