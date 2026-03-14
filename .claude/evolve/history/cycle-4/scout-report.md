# Cycle 4 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 22 (agents/*, skills/evolve-loop/*, docs/*, CHANGELOG.md, README.md, CONTRIBUTING.md, install.sh, .claude-plugin/plugin.json, state.json, notes.md, operator-log.md, all 3 instinct files, CI workflow)
- Research: skipped (all 5 queries within 12hr TTL from 2026-03-13T00:10:00Z)
- Instincts applied: 4 (inst-003 ledger ts format, inst-004 grep-based evals effective, inst-007 orchestrator-as-builder for S tasks, inst-008 plugin manifest conventions)

## Key Findings

### Security/Stability — CRITICAL (3 cycles overdue)
- **Denial-of-wallet guardrails missing.** Deferred since cycle 1. Operator stated in cycle-3 post-cycle log: "further deferral is not acceptable." There is no `maxCyclesPerSession` cap, no `warnAfterCycles` threshold, and no enforcement logic anywhere in the pipeline. A runaway `/evolve-loop 100` invocation could generate 100 Builder+Auditor agent launches with unbounded cost. Cycle-3 scout proposed concrete scope: add two fields to state.json schema in `memory-protocol.md` + enforcement block in `SKILL.md` initialization + Operator check in `phases.md`.

### Instinct System — MEDIUM
- **inst-004 fragmented across two files.** `cycle-1-instincts.yaml` has `inst-004` at confidence 0.6. `cycle-2-instincts.yaml` has `inst-004-update` at confidence 0.8 (the authoritative version). No consolidation has happened. Future cycles reading instincts see both versions — the old 0.6 entry is stale noise. Cycle-4 instinct file should establish canonical state.
- **inst-007 ready for formal policy graduation.** Confidence 0.8 after cycles 2 and 3 both confirming the orchestrator-as-builder pattern. Operator explicitly recommended: "Consider graduating inst-007 to a formal policy." Currently invisible to orchestrators that skip instinct reading.

### CI Status — RESOLVED
- `gh run list` confirms 3 successful CI runs. The `add-ci-workflow` task from cycle 2 is working correctly. No action needed.

### costBudget field — RESOLVED
- Confirmed in cycle-3: state.json does NOT contain `costBudget`. Resolved naturally. No action needed.

### Code Quality — PASS
- All 4 agent files: correct frontmatter. All skill files: present and valid. README, CONTRIBUTING.md, docs/ accurate. No file over 800 lines. No hardcoded secrets.

## Instincts Applied
- **inst-003** (ledger-ts-not-timestamp): Ledger entry below uses `ts` — confirmed correct.
- **inst-004** (grep-based-evals-effective): All 3 eval definitions use grep-based checks.
- **inst-007** (orchestrator-as-builder): Tasks 2 and 3 are S-complexity — orchestrator can implement directly. Task 1 is M-complexity — should spawn Builder.
- **inst-008** (plugin-manifest-must-declare-components): No plugin.json changes needed this cycle — confirmed OK.

## Selected Tasks

### Task 1: Add Denial-of-Wallet Guardrails
- **Slug:** add-denial-of-wallet-guardrails
- **Type:** security
- **Complexity:** M
- **Rationale:** Deferred 3 consecutive cycles. Operator explicitly escalated. The scope is now well-defined: add `maxCyclesPerSession` (default 10) and `warnAfterCycles` (default 5) fields to state.json schema in `memory-protocol.md`, add enforcement block to `SKILL.md` initialization section (warn at `warnAfterCycles`, halt at `maxCyclesPerSession`), add Operator check to `phases.md`, update `state.json` defaults.
- **Acceptance Criteria:**
  - [ ] `memory-protocol.md` state.json schema includes `maxCyclesPerSession` and `warnAfterCycles` fields with descriptions
  - [ ] `SKILL.md` initialization section has cycle count enforcement logic (warn at `warnAfterCycles`, halt at `maxCyclesPerSession`)
  - [ ] `state.json` updated with `maxCyclesPerSession: 10` and `warnAfterCycles: 5` defaults
  - [ ] `phases.md` Operator section references the cycle cap
- **Files to modify:** `skills/evolve-loop/memory-protocol.md`, `skills/evolve-loop/SKILL.md`, `skills/evolve-loop/phases.md`, `.claude/evolve/state.json`
- **Eval:** written to `evals/add-denial-of-wallet-guardrails.md`

### Task 2: Audit and Consolidate Instinct System
- **Slug:** audit-instinct-system-health
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** 11 instincts across 3 fragmented files. `inst-004` appears twice with different confidence values — the 0.6 entry is stale. The cycle-4 instinct file (written during this cycle's LEARN phase) should: (1) carry forward an authoritative consolidated `inst-004` at 0.8, (2) add any new cycle-4 instincts. Old files remain immutable (history), but cycle-4 establishes canonical truth. Also update `state.json` instinctCount to accurate total.
- **Acceptance Criteria:**
  - [ ] `instincts/personal/cycle-4-instincts.yaml` created with authoritative `inst-004` at confidence 0.8 and at least 1 new cycle-4 instinct
  - [ ] No YAML parse errors across all instinct files
  - [ ] `state.json` `instinctCount` updated to accurate total (new instincts added)
- **Files to modify:** `.claude/evolve/instincts/personal/cycle-4-instincts.yaml` (new), `.claude/evolve/state.json`
- **Eval:** written to `evals/audit-instinct-system-health.md`

### Task 3: Graduate inst-007 to Formal Orchestrator Policy
- **Slug:** graduate-inst-007-to-orchestrator-policy
- **Type:** feature
- **Complexity:** S
- **Rationale:** inst-007 ("orchestrator-as-builder for S-complexity tasks") has confidence 0.8 after 2 cycles of confirmation. Currently only discoverable if the orchestrator reads instinct files. Adding it as a formal policy to `SKILL.md` makes it the default behavior — always applied, not just when instincts are read. Also add a `[4.2.0]` CHANGELOG entry to document all cycle-4 changes.
- **Acceptance Criteria:**
  - [ ] `SKILL.md` has a formal "Orchestrator Policies" section (or equivalent) documenting S-complexity inline implementation policy
  - [ ] Policy specifies: S-complexity tasks (<10 lines changed, fully specified with eval definitions) may be implemented by orchestrator directly without spawning Builder agent
  - [ ] `CHANGELOG.md` has a `[4.2.0]` entry documenting denial-of-wallet guardrails, instinct consolidation, and inst-007 graduation
- **Files to modify:** `skills/evolve-loop/SKILL.md`, `CHANGELOG.md`
- **Eval:** written to `evals/graduate-inst-007-to-orchestrator-policy.md`

## Deferred
- **Instinct global promotion mechanism**: README describes "After 5+ cycles, high-confidence instincts promote to global scope" but no mechanism exists. Complexity L — requires defining global instinct schema and promotion workflow. Defer to cycle 5+.
- **CI workflow run history monitoring**: Resolved — confirmed 3 successful runs via `gh run list`.
- **costBudget field**: Resolved — field absent from state.json. History-only appearances are correct.
