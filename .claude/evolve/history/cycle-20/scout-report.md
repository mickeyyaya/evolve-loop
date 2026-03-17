# Cycle 20 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 3 (changedFiles: evolve-scout.md, phases.md, memory-protocol.md) + CHANGELOG.md + docs/architecture.md
- Research: skipped (cooldown active — last queries at 2026-03-14T12:00:00Z, TTL 12hr not expired)
- Instincts applied: 3 (inst-013 progressive-disclosure, inst-014 activate-dormant-infrastructure, inst-015 signal-to-action-wiring)

---

## Key Findings

### Features — HIGH
- **Self-building capability gap:** The Scout introspection pass (cycle 19) detects pipeline degradation from evalHistory metrics (auditIterations, instinctsExtracted, stagnationPatterns). It does NOT identify *new capabilities the loop lacks*. No mechanism exists to scan the deferred task backlog (prior scout-reports' "Deferred" sections), unimplemented doc references (e.g., island-model.md is documented but dormant), or instincts that have never triggered action. The loop cannot currently ask "what should I be able to do that I cannot yet do?" — only "what is degrading that I already do?"

### Docs — HIGH
- **CHANGELOG missing cycles 17-19 features:** v6.4.0 (latest) documents prompt deduplication and skillEfficiency. But three significant self-improvement features shipped in cycles 17-19 are absent: `processRewardsHistory` (rolling 3-entry reward trend window), `pendingImprovements` remediation loop (per-cycle auto-task generation from sustained low scores), and the Scout introspection pass (5 heuristics for pipeline self-improvement proposals). These should be the v6.5.0 entry. This is the meta-cycle documentation task.

### Docs — MEDIUM
- **docs/architecture.md stale re: self-improvement infrastructure:** The architecture doc covers pipeline phases, agents, stagnation detection, and mastery graduation — but does not mention the self-improvement stack added in cycles 17-19 (processRewardsHistory for trend detection, pendingImprovements auto-task generation, Scout introspection pass). The "Continuous Learning" design principle section and "Shared Memory Architecture" section both need to reflect this.

---

## Research
Skipped — research cache is fresh (TTL < 12hr). Goal is internal feature development; prior research results from cycle 19 (self-improving agent systems, meta-learning prompt evolution) are directly applicable.

---

## Selected Tasks

### Task 1: CHANGELOG v6.5.0 for Self-Improvement Infrastructure
- **Slug:** `add-changelog-v6-5-0-self-improvement`
- **Type:** techdebt
- **Complexity:** S
- **Source:** meta-cycle documentation pass (cycle 20 is a meta-cycle trigger)
- **Rationale:** Three features shipped in cycles 17-19 are undocumented in CHANGELOG: processRewardsHistory, pendingImprovements remediation loop, and Scout introspection pass. These represent the core of the "self-improving pipeline" capability added in this version range. At a meta-cycle boundary, documenting what the loop has become is essential context for users and future cycles. S complexity: one file, ~30 lines added.
- **Acceptance Criteria:**
  - [ ] CHANGELOG.md contains a `## [6.5.0]` section above `## [6.4.0]`
  - [ ] v6.5.0 entry documents `processRewardsHistory` (rolling 3-entry trend window)
  - [ ] v6.5.0 entry documents `pendingImprovements` remediation loop with per-cycle trigger logic
  - [ ] v6.5.0 entry documents Scout introspection pass with at least 3 heuristics mentioned
  - [ ] Version date is 2026-03-14
- **Files to modify:**
  - `/Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- **Eval:** written to `evals/add-changelog-v6-5-0-self-improvement.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -n "^\#\# \[6\.5\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md` → expects exit 0 (match found)
  - `grep -n "processRewardsHistory" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md` → expects at least 1 match
  - `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md` → expects at least 1 match
  - `grep -n "[Ii]ntrospection" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md` → expects at least 1 match

---

### Task 2: Self-Building Capability Gap Scanner in Scout
- **Slug:** `add-scout-capability-gap-scanner`
- **Type:** feature
- **Complexity:** M
- **Source:** goal-directed (self-building new features), instinct inst-014 (activate-dormant-infrastructure)
- **Rationale:** The introspection pass detects pipeline *degradation* but not capability *gaps*. The loop cannot currently identify what new things it should be able to do. This task adds a "Capability Gap Scanner" step to the Scout's introspection pass. The scanner checks three gap signals: (1) deferred tasks in `stateJson.evaluatedTasks` with `decision: "deferred"` whose `revisitAfter` has passed — propose as candidates; (2) dormant instincts (confidence >= 0.6 in instinctSummary that have never been `graduated: true` and have been present for 3+ cycles) — surface as potential feature drivers; (3) docs-referenced features that are described as "dormant" or "not yet activated" in the project (e.g., island-model.md). This closes the gap between "what degraded?" (introspection) and "what don't we have yet?" (capability scan). Directly advances the self-building goal.
- **Acceptance Criteria:**
  - [ ] evolve-scout.md Introspection Pass section contains a "Capability Gap Scanner" sub-section
  - [ ] Scanner checks for deferred tasks in `stateJson.evaluatedTasks` with `revisitAfter` that has passed
  - [ ] Scanner checks for dormant instincts (confidence >= 0.6, not graduated, present for 3+ cycles)
  - [ ] Tasks generated by the scanner are labeled `source: "capability-gap"` in the scout report
  - [ ] The capability gap scanner fires ONLY in the introspection pass (does not add new file reads beyond what's already in the Scout context)
- **Files to modify:**
  - `/Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` (extend Introspection Pass section)
- **Eval:** written to `evals/add-scout-capability-gap-scanner.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -n "Capability Gap\|capability.gap\|capability gap" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 1 match
  - `grep -n "revisitAfter\|deferred.*task\|task.*deferred" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 1 match
  - `grep -n "dormant.*instinct\|instinct.*dormant\|graduated" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 1 match
  - `grep -n "source.*capability-gap\|capability-gap.*source" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects at least 1 match

---

### Task 3: Update docs/architecture.md for Self-Improvement Infrastructure
- **Slug:** `update-architecture-docs-self-improvement`
- **Type:** techdebt
- **Complexity:** S
- **Source:** meta-cycle documentation pass
- **Rationale:** docs/architecture.md is the primary reference for understanding what the loop does and how it works. It currently documents phases, agents, stagnation, mastery, and token optimization — but is silent on the self-improvement stack added in cycles 17-19. The "Continuous Learning" design principle says only "each cycle extracts instincts" — it doesn't mention the feedback loop from process rewards to task proposals. The "Shared Memory Architecture" section mentions `planCache` and `synthesizedTools` in Layer 3 but omits `processRewardsHistory` and `pendingImprovements`. S complexity: targeted additions to 2 sections, ~20 lines.
- **Acceptance Criteria:**
  - [ ] `docs/architecture.md` Design Principles "Continuous Learning" section mentions processRewardsHistory trend detection and pendingImprovements auto-task generation
  - [ ] `docs/architecture.md` Shared Memory Architecture Layer 3 entry mentions `processRewardsHistory` and `pendingImprovements`
  - [ ] `docs/architecture.md` mentions the Scout introspection pass in the Agents table or pipeline description
- **Files to modify:**
  - `/Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- **Eval:** written to `evals/update-architecture-docs-self-improvement.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -n "processRewardsHistory" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md` → expects at least 1 match
  - `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md` → expects at least 1 match
  - `grep -n "[Ii]ntrospection" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md` → expects at least 1 match

---

## Deferred

- **CHANGELOG install.sh version string update** — install.sh still shows 6.4.0; could update to 6.5.0 once CHANGELOG is done. Deferred: low priority, can be batched into the Task 1 build or a follow-on cycle.
- **Island model activation** (M-L complexity): docs/island-model.md is referenced but fully dormant. The capability gap scanner (Task 2) will surface this as a future candidate via the dormant-instinct signal once inst-014 (activate-dormant-infrastructure, confidence 0.6) is flagged. Deferring direct activation — it exceeds this cycle's scope.
- **EvoPrompt automated prompt mutation** (L complexity): Deferred from cycle 19, still out of scope. Complexity exceeds budget.
- **Meta-cycle topology review**: The last meta-review (cycles 11-15) proposed no topology changes. No new topology signals this cycle. Skipping.
