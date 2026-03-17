# Cycle 23 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 8 (5 changed files + 3 supporting docs for gap analysis)
- Research: skipped (cooldown active — queries from 2026-03-15T15:00:00Z, <12hr TTL)
- Instincts applied: 0 (instinctSummary empty in state.json — pre-existing gap)
- **instinctsApplied:** [] (instinctSummary array is empty; no instincts available for citation this cycle)

## Key Findings

### Features — HIGH (goal: inspiration, next wave)

Cycle 22 shipped three mechanisms: multi-armed bandit selection, counterfactual annotations, and semantic crossover. All are confirmed integrated into the changed files. Cycle 23 analysis maps remaining gaps across the four inspiration axes:

**Bio-evolution gaps still open:**
- Niche construction: absent. The bandit tracks type-level rewards; there is no mechanism tracking which *file areas and approaches* have historically worked in this specific codebase. Organisms modify their environment to amplify fitness — the loop could maintain a `fileExplorationMap` marking which areas have been recently exploited and steer future cycles toward unexplored territory.
- Epigenetics: absent. No mechanism for context-sensitive suppression or amplification of instinct influence (e.g., suppressing feature-instincts when strategy is "harden").
- Fitness landscape navigation: fitnessScore is a scalar. No multi-dimensional topology explored.

**RL gaps still open:**
- Intrinsic motivation / curiosity reward: the loop only rewards extrinsic task completion (shipped = +1). No novelty bonus for exploring new file areas. The bandit can over-exploit familiar territory; a curiosity signal would counteract this.
- Curriculum sequencing: mastery gates complexity but does not order tasks. No notion of "task A should come before task B because B depends on A's scaffolding."
- Credit assignment across time: if a techdebt task in cycle N enables a feature in cycle N+3, that enabling relationship is invisible to the reward model.

**Social learning / interpretability gaps:**
- Decision trace: the Scout's task ranking rationale is narrative (in scout-report). No structured, queryable record of which signals elevated or suppressed each candidate. Meta-cycle reviews cannot detect systematic biases in selection.
- Auditor teaching memo: Auditor critique feeds back reactively (WARN/FAIL re-launch) but not proactively. A prescriptive teaching memo — 1-3 direct suggestions written by the Auditor for future Builders — would create tighter social learning without adding a new agent.

### Code Quality — MEDIUM
- `skills/evolve-loop/phases.md` is now at 659 lines (82% of 800-line threshold). Cycle 22 additions (counterfactual accuracy review in Phase 5) pushed it 8 lines past the cycle 22 estimate of 651. Growth rate: ~8 lines/cycle. At this rate, the threshold will be hit around cycle 26-27. Monitor; propose split in cycle 25 if trend continues.

### Introspection Pass
Reviewing evalHistory delta metrics (cycles 18-22):
- `instinctsExtracted`: 1, 1, 1, 1, 1 — stable at 1 per cycle. Not 0, so no enrichment task triggered.
- `auditIterations`: 1.0 across all 5 cycles — Builder efficiency nominal.
- `successRate`: 1.0 across all 5 cycles — no task-sizing regression.
- `stagnationPatterns`: 0 across all 5 cycles.
- `pendingImprovements`: empty.
- Deferred tasks: none with passed `revisitAfter` dates in evaluatedTasks.
- Dormant instincts: instinctSummary is empty (pre-existing gap) — cannot scan for uncited instincts.

No introspection heuristic triggered. No capability-gap candidates. All signals green.

**Crossover scan (planCache has 4 entries with successCount >= 2):**
- Parent A: `add-section-to-file` (feature, successCount 8) — approach: extend existing doc files with new sections
- Parent B: `docs-update` (techdebt, successCount 6) — approach: update cross-references and schema documentation
- Crossover offspring candidate: `add-exploration-map-with-doc-update` — apply the "extend existing file" approach from Parent A to create a novelty-tracking map, and use the "update schema documentation" approach from Parent B to document it in memory-protocol. This exactly describes Task 1 below. The crossover aligns with independent selection — good signal.

## Selected Tasks

### Task 1: add-intrinsic-novelty-reward
- **Slug:** add-intrinsic-novelty-reward
- **Type:** feature
- **Complexity:** S
- **Source:** codebase analysis + crossover (parents: add-section-to-file, docs-update)
- **Rationale:** The bandit tracks per-type reward history but cannot prevent the loop from over-exploiting the same files cycle after cycle (e.g., SKILL.md and phases.md are touched in nearly every cycle because they're the highest-reward area). Adding a `fileExplorationMap` to state.json — a rolling window of recently touched files — and a novelty-scoring rule to Scout task selection gives the loop an intrinsic motivation signal: tasks that touch files not recently modified receive a small priority boost. This is the cleanest implementation of the RL "curiosity reward" concept: it doesn't replace extrinsic reward (shipping) but shapes exploration behavior. It closes the bio-evolution "niche construction" gap at the file level — the loop learns which territory it's been over-farming and steers away. S complexity: schema addition to memory-protocol.md + selection rule addition to evolve-scout.md + SKILL.md documentation.
- **Acceptance Criteria:**
  - [ ] `memory-protocol.md` documents `fileExplorationMap` in state.json schema: a rolling map of `{filePath: lastTouchedCycle}` entries, capped at 20 entries (oldest evicted first)
  - [ ] `evolve-scout.md` Task Selection section documents the novelty rule: tasks touching files with `lastTouchedCycle <= currentCycle - 3` receive a +1 priority boost (equivalent to bandit boost, non-stacking)
  - [ ] `SKILL.md` documents the novelty boost in the Task Selection priority list (new item after bandit boost: "Novelty bonus: +1 boost for tasks in under-explored file areas")
  - [ ] The novelty rule is described as *complementary* to bandit selection, not a replacement — bandit governs type-level reward, novelty governs file-level exploration
- **Files to modify:** `skills/evolve-loop/SKILL.md`, `skills/evolve-loop/memory-protocol.md`, `agents/evolve-scout.md`
- **Eval:** written to `evals/add-intrinsic-novelty-reward.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "fileExplorationMap\|explorationMap\|novelty" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects count >= 1
  - `grep -c "novelty\|under-explored\|exploration.*boost\|boost.*exploration" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects count >= 1
  - `grep -c "novelty\|under-explored\|fileExploration" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md` → expects count >= 1

### Task 2: add-scout-decision-trace
- **Slug:** add-scout-decision-trace
- **Type:** feature
- **Complexity:** S
- **Source:** codebase analysis (interpretability gap)
- **Rationale:** The Scout's task ranking rationale is currently only expressed in narrative prose within scout-report.md. The meta-cycle review has no structured signal to detect systematic biases — for example, "the Scout consistently underweights stability tasks even when the bandit arm is high" would never surface unless a human reads reports carefully. Adding a `decisionTrace` field to the scout-report output — a structured array capturing each candidate task and the signals that elevated or suppressed it — makes the Scout's reasoning queryable. The meta-cycle's Novelty Critic can read decisionTrace entries to detect repeating suppression patterns (e.g., if "stagnation" is cited as suppression reason for 4+ consecutive cycles on the same file area, the Scout has a blind spot). S complexity: adds a schema entry to memory-protocol.md and an output section to evolve-scout.md's scout-report template.
- **Acceptance Criteria:**
  - [ ] `evolve-scout.md` scout-report template includes a `## Decision Trace` section listing each considered task candidate with: `slug`, `finalDecision` (selected/deferred/rejected), and `signals` array (each signal has `type`: bandit-boost/crossover/novelty/goal-alignment/stagnation/completed/revisitAfter/pendingImprovement, and `direction`: +1/-1)
  - [ ] `memory-protocol.md` documents `decisionTrace` as an optional field on scout-report output — not persisted in state.json (workspace-only), but read by the meta-cycle's Novelty Critic
  - [ ] The Decision Trace section appears in the Scout Output specification within `evolve-scout.md`
- **Files to modify:** `agents/evolve-scout.md`, `skills/evolve-loop/memory-protocol.md`
- **Eval:** written to `evals/add-scout-decision-trace.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "Decision Trace\|decisionTrace\|decision.*trace" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects count >= 1
  - `grep -c "decisionTrace\|Decision Trace\|decision.*trace" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects count >= 1

### Task 3: add-prerequisite-task-graph
- **Slug:** add-prerequisite-task-graph
- **Type:** feature
- **Complexity:** M
- **Source:** codebase analysis (curriculum learning gap)
- **Rationale:** Mastery levels prevent the loop from attempting tasks that are too complex for its current skill level, but they don't sequence tasks. In practice, some tasks are only meaningful if a prior task has been completed — e.g., "add experiment-journal-summary-to-meta-review" only makes sense after an experiment journal exists. Currently the Scout has no way to express this dependency; if the prerequisite was built 10 cycles ago, the context is lost. Adding an optional `prerequisites` field to task candidates — a list of completed task slugs that are preconditions — enables: (1) the Scout to auto-detect when a proposed task is blocked by an incomplete prerequisite and either defer it or propose the prerequisite first, (2) the meta-cycle review to surface "orphan prerequisites" — tasks that were built but whose dependent tasks were never proposed. This is the "prerequisite graph" version of curriculum learning: instead of just gating by abstract complexity, the loop sequences tasks by explicit dependency. M complexity: touches Scout task selection logic, memory-protocol schema, and phases.md deferred task handling.
- **Acceptance Criteria:**
  - [ ] `evolve-scout.md` Task Selection section documents: when proposing a task, the Scout may optionally specify `prerequisites: ["slug-a", "slug-b"]` — if any listed slug is not in `evaluatedTasks` with `decision: "completed"`, the task is automatically deferred with `deferralReason: "prerequisite not met: <slug>"`
  - [ ] `memory-protocol.md` documents `prerequisites` as an optional field on evaluatedTask entries (set when a task is proposed with dependencies)
  - [ ] `skills/evolve-loop/phases.md` Phase 1 (DISCOVER) documents: after Scout completes, orchestrator checks proposed tasks for unmet prerequisites and defers them, logging the prerequisite slug so the Scout can propose it in the next cycle
  - [ ] The prerequisite check is described as a *lightweight suggestion mechanism*, not a hard constraint — the Scout may override prerequisite gates if the task is genuinely independent of the listed prerequisite in the current context
- **Files to modify:** `agents/evolve-scout.md`, `skills/evolve-loop/memory-protocol.md`, `skills/evolve-loop/phases.md`
- **Eval:** written to `evals/add-prerequisite-task-graph.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "prerequisites\|prerequisite" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects count >= 1
  - `grep -c "prerequisites\|prerequisite" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects count >= 1
  - `grep -c "prerequisites\|prerequisite" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects count >= 1

## Deferred

- **Auditor teaching memo** (`add-auditor-teaching-memo`): An S-complexity feature where the Auditor writes a prescriptive 1-3 point teaching memo for future Builders after every audit verdict. Closes the social learning gap. Deferred this cycle to keep within the 3-task cycle budget and prioritize the prerequisite graph (higher architectural impact). Predicted complexity: S. Estimated reward: 0.9. Alternate approach: add a `## Teaching Memo` section to the Auditor output template in `evolve-auditor.md`. Deferral reason: cycle budget (3 tasks selected, adding a 4th would push estimated cycle tokens toward 150-160K and reduce per-task quality).
- **Phases.md split** (`refactor-phases-split`): At 659 lines and rising ~8 lines/cycle, the file will hit the 800-line threshold around cycle 26-27. Deferring until it reaches 700 lines or a natural split point (e.g., the meta-cycle section is already 175 lines and could stand alone as `meta-cycle-phases.md`). Predicted complexity: S. Estimated reward: 0.8. Alternate approach: extract Phase 5 steps 7-10 (meta-cycle, context management, exit conditions) into a separate `phases-meta.md` file, leaving phases.md at ~480 lines. Deferral reason: not yet at threshold; no functionality blocked.
- **Island model activation**: L complexity, requires `--island` argument parser and multi-state file management. Still deferred until a stagnation signal warrants it.
- **Epigenetics mechanism** (`add-strategy-instinct-modulation`): Interesting (strategy-sensitive instinct amplification/suppression), but requires significant changes to how instincts are applied and read. Deferred as M/L complexity requiring clearer design. Predicted complexity: M. Estimated reward: 0.7. Deferral reason: design not yet clear enough; would need a new instinct metadata field and a modulation lookup at both Scout and Builder read time.
