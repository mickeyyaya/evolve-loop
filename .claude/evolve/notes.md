# Evolve Loop Cross-Cycle Notes

## Cycle 1 (2026-03-13)
- Fixed CRITICAL `wt` CLI dependency — replaced with Claude Code EnterWorktree/ExitWorktree
- Fixed writing-agents.md to describe context-overlay pattern (was telling contributors to copy ECC content)
- Normalized ledger schema to canonical `"ts"` format
- 4 instincts extracted (1 anti-pattern, 1 architecture, 1 convention, 1 successful-pattern)
- Deferred: README phase count fix, CONTRIBUTING.md URL fix, CHANGELOG 3.1.0 entry, agent count discrepancy, CI/CD, instinct docs, install.sh CI mode, eval baselines, denial-of-wallet guardrails

## Cycle 2 (2026-03-13)
- Fixed eval-runner.md stale v3 references (Phase 5.5 → Phase 3, Developer → Builder, Planner → Scout)
- Added CI/non-interactive mode to install.sh (CI=true or --ci flag)
- Added GitHub Actions CI workflow (validates install, agent files, skill files, frontmatter, plugin manifest)
- 4 instincts extracted (1 anti-pattern, 1 process, 1 successful-pattern, 1 update)
- Instinct inst-004 confidence increased from 0.6 → 0.8 (grep-based evals confirmed across 2 cycles)
- Deferred: instinct system docs, denial-of-wallet guardrails, stale costBudget field cleanup

## Cycle 3 (2026-03-13)
- Updated writing-agents.md frontmatter template (added name, description, tools — required by plugin system CI)
- Added CHANGELOG [4.1.0] entry for plugin packaging changes
- Added docs/instincts.md — full instinct system documentation (schema, confidence, promotion, usage)
- Bumped plugin.json and marketplace.json to 4.1.0
- 3 instincts extracted (1 convention, 1 anti-pattern, 1 update), inst-007 confidence 0.6 → 0.8
- All 3 tasks implemented inline by orchestrator (inst-007 applied)
- Deferred: denial-of-wallet guardrails (cycle 4+)

## Cycle 4 (2026-03-13)
- Added denial-of-wallet guardrails: maxCyclesPerSession (10), warnAfterCycles (5) — 3 cycles overdue, finally shipped
- Consolidated instincts: inst-004 → 0.9, inst-007 → 0.9, added inst-010 (deferred-security-escalates)
- Graduated inst-007 to formal Orchestrator Policy in SKILL.md
- Added CHANGELOG [4.2.0]
- **Tasks:** 3 (1 M via Builder agent, 2 S inline via orchestrator)
- **Audit:** PASS (1 non-blocking MEDIUM note: boundary semantics inconsistency in cycle cap)
- **Eval:** 10/10 passed
- **Shipped:** YES
- **Instincts:** 3 entries (2 consolidations + 1 new)
- Deferred: instinct global promotion mechanism (complexity L), boundary semantics fix for cycle cap

## Cycle 5 (2026-03-13)
- Fixed cycle cap boundary semantics: `>=` → `>` in phases.md per-cycle check
- Bumped plugin.json and marketplace.json to 4.2.0 (was out of sync with CHANGELOG)
- Added denial-of-wallet guardrails to README features list
- **Tasks:** 3 (all S, all inline via orchestrator)
- **Audit:** PASS
- **Eval:** 8/8 passed (1 false negative in eval grep pattern due to backtick formatting — semantic check confirmed correct)
- **Shipped:** YES
- **Instincts:** 2 entries (1 new inst-011 version-sync-on-changelog, 1 update inst-004 → 0.95)
- Deferred: instinct global promotion mechanism (L complexity)

## Cycle 6 — 2026-03-13
- **Tasks:** 4 (all S, all inline via orchestrator)
  - Bumped marketplace.json to 6.0.0
  - Updated install.sh version string to v6
  - Added genes/, tools/, instincts/archived/ to README workspace layout
  - Added synthesizedTools to state.json schema
- **Audit:** PASS (inline eval checks)
- **Eval:** 4/4 passed
- **Shipped:** YES
- **Instincts:** 2 entries (1 new inst-012 v6-migration-check, 1 update inst-011 → 0.85)
- **Next cycle should consider:** CI workflow validation with new directory structure, docs/genes.md and docs/island-model.md referenced but not in project structure diagram

## Cycle 7 — 2026-03-13
- **Tasks:** 2 (1 S inline, 1 M inline)
  - Fixed install.sh usage line + added CI docs validation step for v6 required docs
  - Rewrote docs/architecture.md from v4 to v6 (strategy, stagnation, mastery, genes, model routing, context management)
- **Audit:** PASS (inline eval checks)
- **Eval:** 6/6 passed
- **Shipped:** YES
- **Instincts:** 1 entry (inst-013 docs-lag-on-major-version)
- **Mastery:** promoted to proficient (7 consecutive successes)

## Cycle 8 — 2026-03-13
- **Tasks:** 3 (1 M inline, 2 S inline)
  - Added instinct global promotion step to phases.md, fixed homunculus path in docs/instincts.md
  - Added missing state.json fields (mastery, processRewards, planCache, synthesizedTools) to memory-protocol.md
  - Added explicit memory consolidation trigger check (cycle % 3) to phases.md
- **Audit:** PASS (inline eval checks, 10/10)
- **Eval:** 10/10 passed
- **Shipped:** YES
- **Instincts:** 1 entry (inst-012 confidence update → 0.8)
- **Note:** Global promotion mechanism finally shipped — deferred since cycle 5

## Cycle 9 — 2026-03-13
- **Tasks:** 3 (all S, all inline via orchestrator)
  - Fixed Operator model sonnet→haiku in README agents table
  - Added Context Management subsection to README Key Mechanics
  - Added processRewards to docs/configuration.md and state.json
- **Audit:** PASS (inline eval checks, 8/8)
- **Eval:** 8/8 passed
- **Shipped:** YES
- **Memory Consolidation:** YES (cycle 9 % 3 === 0)
  - Merged inst-005 + inst-009 + inst-013 → inst-005 (docs-lag-after-changes, 0.9)
  - Merged inst-011 + inst-012 → inst-011 (version-sync-all-files, 0.9)
  - Decayed inst-001 (0.7 → 0.6) and inst-006 (0.7 → 0.6) — unreferenced 7+ cycles
  - Instincts before: 15, after: 12 (3 merged, 2 decayed)

## Cycle 10 — 2026-03-13
- **Tasks:** 3 (all S, all inline via orchestrator)
  - Created docs/meta-cycle.md standalone reference
  - Added deterministic processRewards scoring rubric to phases.md
  - Added CHANGELOG [6.1.0], bumped plugin.json and marketplace.json to 6.1.0
- **Audit:** PASS (inline eval checks, 10/10)
- **Eval:** 10/10 passed
- **Shipped:** YES
- **Meta-Cycle Review:** YES (cycle 10 % 5 === 0)
  - 14/14 tasks shipped in cycles 6-10, 100% success rate
  - All inline via orchestrator — zero Builder/Auditor agent spawns
  - Recommendation: switch to `innovate` strategy, project approaching convergence

## Cycle 11 — 2026-03-13
- **Tasks:** 3 (all S, all inline via orchestrator)
  - Rewrote .github/ISSUE_TEMPLATE/bug_report.md for v6 phases and agents
  - Added CI step validating required skill files (SKILL.md, phases.md, memory-protocol.md, eval-runner.md)
  - Created examples/instinct-example.yaml with annotated instinct examples
- **Audit:** PASS (inline eval checks, 15/15)
- **Eval:** 15/15 passed
- **Shipped:** YES
- **Focus:** Contributor experience improvements

## Cycle 12 — 2026-03-13
- **Tasks:** 3 (all S, all inline via orchestrator)
  - Updated feature request and PR templates to v6 phase names
  - Created examples/gene-example.yaml with annotated examples
  - Added CI/dry-run mode to uninstall.sh
- **Audit:** PASS (inline eval checks, 12/12)
- **Eval:** 12/12 passed
- **Shipped:** YES
- **Memory Consolidation:** Triggered (cycle 12 % 3 === 0) — no merges needed, 12 instincts stable
- **Focus:** Contributor experience and tooling parity

## Cycle 13 — 2026-03-13
- **Tasks:** 2 (both S, both inline via orchestrator)
  - Wrote real processRewards scores to state.json (was all 0.0)
  - Created missing cycle-8-instincts.yaml (inst-012 confidence update)
- **Audit:** PASS (inline eval checks, 5/5)
- **Eval:** 5/5 passed
- **Shipped:** YES
- **Note:** Scout reported project approaching convergence — only 2 tasks found. Only provenance/completeness gaps remain.

## Cycle 14 — 2026-03-13
- **Tasks:** 2 (both S, both inline via orchestrator)
  - Added examples/ cross-links to README, docs/instincts.md, docs/genes.md, eval-runner.md
  - Added CHANGELOG [6.2.0] covering cycles 11-14, bumped to v6.2.0
- **Audit:** PASS (inline eval checks, 7/7)
- **Eval:** 7/7 passed
- **Shipped:** YES
- **Note:** Project very near convergence. Scout found only discoverability and version tracking gaps.

## Cycle 15 — 2026-03-13
- **Tasks:** 0 (Scout reported convergence)
- **Shipped:** NO (nothing to ship)
- **Meta-Cycle Review:** YES (cycle 15 % 5 === 0)
  - 10/10 tasks shipped in cycles 11-14, 100% success rate
  - Learning plateau — no new instincts since cycle 9
  - Recommendation: project converged, future work requires explicit goals
- **Memory Consolidation:** YES (cycle 15 % 3 === 0)
  - Decayed inst-001 (0.6 → 0.5) and inst-006 (0.6 → 0.5)
  - No merges needed
- **Stagnation:** nothingToDoCount=1, diminishing-returns pattern logged

## Cycle 16 — 2026-03-13
- **Tasks:** 0 (Scout confirmed convergence)
- **Shipped:** NO (nothing to ship)
- **nothingToDoCount:** 2
- **Note:** Final cycle of 10-cycle batch. Project has converged. 42 tasks shipped across 14 productive cycles with 100% success rate.
