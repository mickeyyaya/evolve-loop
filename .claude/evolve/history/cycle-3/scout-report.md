# Cycle 3 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 18 (changed files from c758816 + deferred items from cycles 1-2)
- Research: skipped (within 12hr TTL — queries dated 2026-03-13T00:10:00Z)
- Instincts applied: 5 (inst-001, inst-003, inst-004, inst-005, inst-007)

## Key Findings

### Documentation — MEDIUM
`docs/writing-agents.md` shows an outdated agent frontmatter template with only `model:` as the required field. Cycle 3 (commit c758816) added `name:`, `description:`, and `tools:` as required frontmatter fields validated by CI. Any contributor following the guide will create agents that fail the CI frontmatter check. This is a direct stale reference from the cycle 3 packaging work (inst-005: major version changes leave stale refs in docs).

### Documentation — MEDIUM
`CHANGELOG.md` has no entry for the cycle 3 plugin packaging commit (c758816). The cycle 3 changes were substantial: `plugin.json` agents/skills arrays, agent frontmatter with name/description/tools, CI manifest validation. The changelog jumps from `[4.0.0]` to nothing. Users and contributors have no release record of these changes.

### Documentation — LOW
The instinct system has 8 entries across two YAML files in `.claude/evolve/instincts/personal/` but no user-facing documentation in `docs/`. `configuration.md` mentions instinct promotion in one line. The YAML schema (id, pattern, description, confidence, source, type), the promotion mechanism, and how to inspect/edit instincts are not documented. The Operator flagged this twice (cycles 1 and 2). Low severity because the pipeline works without it, but it creates onboarding friction.

### State Consistency — LOW
`state.json` `instinctCount` is 8 but the actual unique instinct IDs are 7 (inst-001 through inst-007; the cycle-2 file includes `inst-004-update` which is an update to an existing instinct, not a new one). Harmless but the count is inaccurate.

### Deferred Items Review
- **costBudget field cleanup**: `state.json` does NOT contain `costBudget` — it was already removed. The field only appears in history files and CHANGELOG. Resolved naturally. No action needed.
- **Denial-of-wallet guardrails**: Still M-L complexity. The Operator has flagged this 3 cycles in a row. Proposed scope for cycle 4: add `maxCyclesPerSession` and `warnAfterCycles` to state.json schema + Operator enforcement check.

## Instincts Applied
- **inst-001** (wt-cli-not-real): No new wt references introduced — confirmed clean.
- **inst-003** (ledger-ts-not-timestamp): New ledger entry below uses `ts` — confirmed correct.
- **inst-004** (grep-based-evals-effective): Applying grep-based checks for all 3 task evals.
- **inst-005** (major-version-stale-refs): Triggered discovery of writing-agents.md stale frontmatter template.
- **inst-007** (orchestrator-as-builder): All 3 tasks are S-complexity — orchestrator can implement directly.

## Selected Tasks

### Task 1: Update writing-agents.md frontmatter template
- **Slug:** update-writing-agents-frontmatter-template
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** `writing-agents.md` teaches contributors the wrong agent format — the template shows only `model:` but the real format now requires `name:`, `description:`, `tools:`, and `model:`. Any agent created following the guide will fail CI. Direct fallout from cycle 3 packaging work (inst-005 applies). Small fix, high impact for onboarding.
- **Acceptance Criteria:**
  - [ ] `writing-agents.md` frontmatter template includes `name:`, `description:`, `tools:`, and `model:` fields
  - [ ] The template matches the actual format of the 4 evolve agents
  - [ ] Example frontmatter is consistent with CI validation rules (`grep ^name:`, `grep ^description:`)
- **Files to modify:** `docs/writing-agents.md`
- **Eval:** written to `evals/update-writing-agents-frontmatter-template.md`

### Task 2: Add CHANGELOG entry for cycle 3 plugin packaging
- **Slug:** add-changelog-cycle3-plugin-packaging
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** CHANGELOG.md has no entry for the cycle 3 changes (commit c758816). These changes added `agents`/`skills` arrays to `plugin.json`, updated all 4 agent frontmatter files, added CI manifest validation, and updated README with install commands. Without a changelog entry, users upgrading from 4.0.0 have no release record. Small fix, correct hygiene.
- **Acceptance Criteria:**
  - [ ] CHANGELOG.md has a `[4.1.0]` entry dated 2026-03-13 after `[4.0.0]`
  - [ ] Entry documents plugin.json agents/skills arrays
  - [ ] Entry documents agent frontmatter name/description/tools fields
  - [ ] Entry documents CI manifest validation workflow
- **Files to modify:** `CHANGELOG.md`
- **Eval:** written to `evals/add-changelog-cycle3-plugin-packaging.md`

### Task 3: Add instinct system documentation
- **Slug:** add-instinct-system-docs
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** The instinct system (YAML schema, confidence scoring, promotion path) is undocumented in `docs/`. This has been deferred twice per `notes.md` and the Operator explicitly flagged it 3 cycles in a row. Now that the system is stable (8 instincts, consistent schema, working across 2 cycles), documenting it is low-risk and high-value for onboarding. Creates a new `docs/instincts.md`.
- **Acceptance Criteria:**
  - [ ] `docs/instincts.md` exists
  - [ ] Documents the YAML schema (id, pattern, description, confidence, source, type)
  - [ ] Documents confidence scoring (starts at 0.5, increases with confirmation)
  - [ ] Documents instinct file location (`.claude/evolve/instincts/personal/`)
  - [ ] Documents promotion path after 5+ cycles with confidence >= 0.8
  - [ ] Documents how to inspect and manually edit instincts
- **Files to modify:** `docs/instincts.md` (create new)
- **Eval:** written to `evals/add-instinct-system-docs.md`

## Deferred

- **Denial-of-wallet guardrails**: Still M-L complexity requiring architectural design. Concrete proposal for cycle 4: add `maxCyclesPerSession` and `warnAfterCycles` fields to state.json schema, add Operator check. Defer to cycle 4 with smaller scope.
- **instinctCount accuracy (state.json)**: LOW severity — `instinctCount: 8` includes `inst-004-update` which is an update not a new instinct. Harmless. Will correct naturally when state.json is next updated by the Operator in the LEARN phase.
- **instinct promotion path validation**: `configuration.md` references `~/.claude/instincts/personal/` for promotion after 5+ cycles. Need to confirm this is a real Claude Code path before documenting. Defer until validated.
