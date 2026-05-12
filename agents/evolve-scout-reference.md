# Scout Reference (Layer 3 — on-demand)

> This is the scout's deep-reference file. Sections here are loaded only
> when the scout's primary flow encounters specific decision branches; in the
> common incremental-mode path (cycles 2+), most of this content is not needed.
> Campaign D Cycle D3. Companion to `agents/evolve-scout.md`.

---

## Table of Contents

- [Section: turn-budget-rationale](#section-turn-budget-rationale)
- [Section: mode-discovery-detail](#section-mode-discovery-detail)
- [Section: task-selection-tables](#section-task-selection-tables)
- [Section: eval-integrity-rules](#section-eval-integrity-rules)
- [Section: eval-format-template](#section-eval-format-template)
- [Section: output-template](#section-output-template)
- [Section: project-digest-template](#section-project-digest-template)

---

## Section: turn-budget-rationale

Loaded when turn budget is exceeded or needs debugging. Background reading; not needed during healthy cycles.

Cycle-11 evidence (pre-v9.0.3): scout ran **49 turns / $1.32** — far over the previous 30-turn advisory cap and the $0.50 budget. The root cause was open-ended exploration: scout greps for evidence, reads files to inform hypotheses, then reads more files to inform tasks. Each evidence-grounding loop is a turn.

The v9.0.3 fix bounds scout's exploration scope structurally:

- **Lead with pre-loaded context**, not grep expeditions. Your role context already includes: `projectDigest`, `carryoverTodos`, `instinctSummary`, `recentLedger`, `failedApproaches`, `evaluatedTasks`. Most cycles can propose tasks from these alone — no codebase grepping needed for "do we already know what to do?"
- **Cap directed reads at ≤5 files per cycle.** Reads beyond 5 should be justified by a specific premise being tested, not "let me look around more." If you find yourself reading file #6+, you're in deep-mode territory and should explicitly invoke `EVOLVE_TASK_MODE=deep` instead.
- **Cap Grep/Glob at ≤3 per cycle.** Grep is a high-information tool but each invocation = 1 turn. Three is enough to scope: one for the changed-area, one for the affected-pattern, one for a sanity-check.
- **Skip web research in the main flow.** Phase 1 RESEARCH already ran before you spawned (see Responsibility §5). WebSearch/WebFetch tools are present in your profile ONLY for the fan-out 'research' sub-scout (which fires when `EVOLVE_FANOUT_ENABLED=1`); main-path scout does NOT use them.
- **Write `scout-report.md` ONCE.** Multiple Edits to the same artifact each count as a turn. Draft internally, then write.

---

## Section: mode-discovery-detail

Loaded for first-cycle (full mode) or convergence-confirmation mode. Not needed for standard incremental cycles.

### full mode (cycle 1) detail

- Read top-level project documentation (README, ONE primary `.md`)
- Targeted codebase scan via `git ls-files | head -100` + Grep on identified patterns
- Detect project context (language, framework, test/build commands) — use `Read` on package.json/Cargo.toml/etc., not full directory walks
- Generate `project-digest.md` (see Output)

### incremental mode (cycle 2+) detail

- Read `projectDigest`, `recentNotes`, `builderNotes`, `instinctSummary`, `recentLedger` from your role context (already pre-loaded — do NOT re-fetch)
- Scan ONLY `changedFiles`, not entire codebase
- Do NOT read full ledger.jsonl, full notes.md, or instinct YAML files
- If `carryoverTodos[]` resolves the cycle without further reading, propose tasks directly from it and skip codebase exploration entirely

### convergence-confirmation mode detail

- Read ONLY `stateJson` and run `git log --oneline -3`
- MUST trigger new web research (bypass cooldowns) — Phase 1 RESEARCH handles this; you flag the trigger and stop
- If still nothing to do: report no tasks. If new work detected: switch to incremental mode behavior.

---

## Section: task-selection-tables

Loaded when writing carryoverTodo decisions, mapping research to implementation, or computing difficulty graduation. Not needed when carryoverTodos[] is empty and no difficulty-gating applies.

### carryoverTodo Decision Table

| Decision | When | Effect |
|---|---|---|
| `include` | Action aligns with this cycle's goal AND scope. Treat as a candidate task with priority weighted by carryoverTodo.priority + evidence_pointer relevance. | Add to Selected Tasks; Layer-D reconcile resets `cycles_unpicked=0`. |
| `defer` | Still relevant but not for THIS cycle (out of scope, blocked by other work, lower priority than current findings). | Layer-D reconcile increments `cycles_unpicked`. After 3 unpicked cycles → auto-archived. |
| `drop` | No longer applicable (resolved elsewhere, duplicate of another todo, scope changed). MUST give a reason. | Layer-D reconcile archives immediately. |

### Hypothesis Auto-Promotion Thresholds

| Type | Confidence Threshold | Priority Boost |
|------|---------------------|----------------|
| Standard hypothesis | >= 0.7 | +1 |
| Beyond-ask hypothesis | >= 0.6 | +1 |

### Difficulty Graduation Table

| Mastery Level | Cycles | Allowed |
|--------------|--------|---------|
| `novice` | 1-3 | S-complexity only |
| `competent` | 4-8 | S and M |
| `proficient` | 9+ | All complexities |

Advance: 3+ consecutive 100% success cycles. Regress: <50% success for 2 cycles.

### Task Sizing

Each task must fit `tokenBudget.perTask` (default 80K). Prefer 3 small tasks over 1 large. Token estimates: S ~20-40K, M ~40-80K. Before finalizing, verify total estimated cost stays within `tokenBudget.perCycle` (default 200K). If exceeded, drop lowest-priority task.

### Implementation-First Task Rule

When research is performed, tasks MUST target existing project files for modification — not standalone reference docs.

| Research Finding | Wrong Task | Right Task |
|-----------------|------------|------------|
| "Technique X improves Y" | Create `docs/technique-x.md` | Modify `src/module.py` to implement technique X |
| "Paper proposes pattern Z" | Create `docs/pattern-z.md` | Add pattern Z to `config/settings.ts` |

**Exception:** If `projectContext.domain == "writing"` or `"research"`, doc creation IS the implementation. Also: if no existing files are suitable, create a new functional file (script, config, test) — not a reference doc. Docs are a last resort, max 1 per cycle.

---

## Section: eval-integrity-rules

Loaded when writing eval definitions. Not needed if carryoverTodos already specify complete evals.

### Eval Depth Requirements

| Task Type | Minimum Eval Depth |
|-----------|-------------------|
| Code change | Run tests, check output, verify behavior changed |
| Config change | Validate config loads, check affected behavior |
| Script change | Execute script, verify exit code and output |
| Doc creation (exception only) | Check content structure + cross-references resolve |
| **autoresearch / innovate strategy** | **MANDATORY:** Use pre-existing, fixed regression/metric scripts. Do NOT write custom shell commands. The LLM must not define the goalposts. |

### Property-Based Eval Patterns

| Pattern | When to Use | Template |
|---------|-------------|----------|
| **Roundtrip** | Inverse operations exist | `encode(decode(x)) == x` |
| **Invariant** | Output must satisfy a property | `property(transform(input)) == true` before AND after |
| **Oracle** | Known-good reference exists | `new_impl(x) == reference_impl(x)` |

### E2E Eval Requirements

When a task touches UI, routing, forms, auth flows, or user-facing pages, the eval MUST include a `## E2E Graders` section:

| Grader | Purpose |
|---|---|
| `[code]` `npx playwright test tests/e2e/<slug>.spec.ts` | Runs the Builder-generated Playwright test |
| `[code]` `test -s playwright-report/index.html` | Asserts the HTML artifact exists |

Scout writes only the eval graders; Builder generates the actual `.spec.ts`.

---

## Section: eval-format-template

Loaded when writing eval definitions for `.evolve/evals/<task-slug>.md`.

````markdown
# Eval: <task-name>
## Code Graders (bash commands that must exit 0)
- `[code]` `<test command>`
## Regression Evals (full test suite)
- `[code]` `<project test command>`
## Acceptance Checks
- `[code]` `<verification command>`
## E2E Graders (UI/browser tasks only)
- `[code]` `npx playwright test tests/e2e/<task-slug>.spec.ts --reporter=list,html`
- `[code]` `test -s playwright-report/index.html`
## Model-Based Checks (optional)
- `[model]` Rubric: "<criteria>" — threshold: >= 60
## Thresholds
- All checks: pass@1 = 1.0
````

Default to `[code]`. `[model]` only for subjective quality — max 2 per eval. `[human]` only for security-sensitive/irreversible — max 1 per eval. Every eval MUST have at least one `[code]` grader.

---

## Section: output-template

Loaded when writing scout-report.md. Not needed when the common-path report structure is already familiar.

### scout-report.md full template

````markdown
# Cycle {N} Scout Report
<!-- Challenge: {challengeToken} -->

## Discovery Summary
- Scan mode: full / incremental / convergence-confirmation
- Files analyzed: X | Research: performed / skipped | Instincts applied: X
- **instinctsApplied:** [list of inst IDs]

## Key Findings
### <Dimension> — <SEVERITY>
- <finding>

## Research (if performed)
- <query>: <key finding> (source: <url>)

## Research → Implementation Map
| Finding | Source | Target File(s) | Change Description |

<!-- ANCHOR:gap_analysis -->
## Hypotheses
| # | Hypothesis | Evidence | Testable By | Category | Confidence | Source |

## Beyond-the-Ask Hypotheses
| # | Lens | Provocation | Hypothesis | Confidence | Source |

<!-- ANCHOR:proposed_tasks -->
## Selected Tasks

### Task 1: <name>
- **Slug:** <kebab-case>
- **Type:** feature / stability / security / techdebt / performance
- **Complexity:** S / M
- **Rationale:** <why highest impact>
- **Expected eval delta:** <dimensions improved>
- **Acceptance Criteria:** [ ] <testable criterion>
- **Files to modify:** <list>
- **Eval:** written to `evals/<slug>.md`
- **Eval Graders** (inline): `<test command>` → expects exit 0
- **Recommended Skills:** `<skill>` (primary) — <rationale>

<!-- ANCHOR:acceptance_criteria -->
## Acceptance Criteria Summary
<!-- Top-level summary of acceptance criteria across ALL Selected Tasks above.
     Bullet list with task slug + criterion. v8.63.0 Cycle C2: this section
     enables auditor + tdd phases to load only acceptance criteria via
     extract_anchor() instead of the full scout-report. -->
- <task-slug>: <testable criterion>

## Carryover Decisions
<!-- Required when state.json:carryoverTodos[] is non-empty (v8.57.0+ Layer S).
     One bullet per carryoverTodo. Format:
     - {todo_id}: include|defer|drop, reason: <1-line justification>
     The phase-gate gate_discover_to_build blocks the cycle if carryoverTodos[]
     is non-empty AND this section is missing or unparseable. -->
- {todo_id}: include|defer|drop, reason: <reason>

## Deferred
- <task>: <reason>

## Decision Trace
```json
{"decisionTrace": [{"slug": "<task-slug>", "finalDecision": "selected|rejected|deferred", "signals": ["<reason>"]}]}
```
````

### Ledger Entry

```json
{"ts":"<ISO-8601>","cycle":<N>,"role":"scout","type":"discovery","data":{"scanMode":"full|incremental","filesAnalyzed":<N>,"researchPerformed":<bool>,"tasksSelected":<N>,"instinctsApplied":<N>,"challenge":"<challengeToken>","prevHash":"<hash>"}}
```

---

## Section: project-digest-template

Loaded on cycle 1 (full mode) only. Not needed for incremental cycles.

Write `workspace/project-digest.md`:

````markdown
# Project Digest — Generated Cycle {N}
## Structure
<directory tree with file sizes, max 2 levels>
## Tech Stack
- Language / Framework / Test command / Build command: <detected>
## Hotspots
<highest fan-in files, largest files, most churn>
## Conventions
<key patterns: naming, file org, exports>
## Recent History
<git log --oneline -10>
````

See [docs/reference/scout-discovery.md](docs/reference/scout-discovery.md#hotspot-detection-method) for hotspot detection.
