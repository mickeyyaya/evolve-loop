# Scout Report — Cycle 5

**Mode:** Incremental discovery (cycle 5 of 5, autonomous)
**Date:** 2026-03-13
**Research:** Skipped — project is a mature Claude Code plugin in a closed domain. All relevant research was performed in cycle 1 (AI agent orchestration, Claude Code plugin patterns, eval-driven development). No architectural decisions remain open that would benefit from fresh external research. TTL expired but search value is negligible at this stage.

---

## Scan Summary

Files analyzed: 12 (changed since cycle 4 + deferred issue owners)
- `skills/evolve-loop/SKILL.md` — boundary semantics check
- `skills/evolve-loop/phases.md` — boundary semantics check
- `.claude-plugin/plugin.json` — version string
- `.claude-plugin/marketplace.json` — version string
- `README.md` — feature coverage
- `CHANGELOG.md` — version reference
- `docs/instincts.md` — promotion path (valid, `~/.claude/homunculus/instincts/personal/` confirmed to exist)
- Instinct files (cycle-1 through cycle-4) — reviewed, no issues

---

## Findings

### 1. Cycle Cap Boundary Asymmetry (deferred from cycle 4)
**Severity:** LOW
**File:** `skills/evolve-loop/phases.md` line 165
**Issue:** Per-cycle check uses `>=` but upfront check in SKILL.md uses `>`.
With `maxCyclesPerSession=10`, the `>=` check in phases.md halts at cycle 10 before it runs, blocking the last allowed cycle. The upfront check correctly allows exactly 10 cycles (`10 > 10` = false). Fix: change phases.md to `>` so both are consistent and the cap means "halt when you exceed the cap."

### 2. Plugin Version Out of Sync
**Severity:** LOW
**Files:** `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`
**Issue:** Both show `"version": "4.1.0"` but CHANGELOG.md has `[4.2.0]` covering cycle 4 features (denial-of-wallet guardrails, orchestrator policies). Stale version strings mislead users installing from the marketplace.

### 3. README Missing Guardrails Feature
**Severity:** LOW
**File:** `README.md` Features section
**Issue:** The 8 bullets listing features omit denial-of-wallet guardrails, which were shipped in v4.2.0. A user reading the README would not know this capability exists. Simple one-line addition.

---

## Not Selected

- **Instinct global promotion mechanism** — L complexity, deferred again. Only 5 cycles run; instincts are project-local patterns. No clear trigger or user need yet.
- **Research TTL refresh** — TTL expired but there is no meaningful research question. Project is feature-complete for the instincts/guardrails phase.
- **homunculus path documentation** — Path confirmed to exist at `~/.claude/homunculus/instincts/personal/`. The existing `docs/instincts.md` reference is accurate. No change needed.

---

## Selected Tasks

### Task 1: fix-cycle-cap-boundary-semantics
- **Slug:** `fix-cycle-cap-boundary-semantics`
- **Complexity:** S
- **Files:** `skills/evolve-loop/phases.md` (1 line change)
- **Description:** Change `>=` to `>` in the per-cycle maxCyclesPerSession check so it matches the upfront check in SKILL.md. Both should use `>` — halt when exceeding the cap, not at the cap cycle itself.
- **Eval:** `.claude/evolve/evals/fix-cycle-cap-boundary-semantics.md`

### Task 2: bump-plugin-version-to-4-2-0
- **Slug:** `bump-plugin-version-to-4-2-0`
- **Complexity:** S
- **Files:** `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json` (1 line each)
- **Description:** Bump version strings from `4.1.0` to `4.2.0` to match CHANGELOG.
- **Eval:** `.claude/evolve/evals/bump-plugin-version-to-4-2-0.md`

### Task 3: add-guardrails-to-readme-features
- **Slug:** `add-guardrails-to-readme-features`
- **Complexity:** S
- **Files:** `README.md` (1 line addition to Features section)
- **Description:** Add denial-of-wallet guardrails bullet to README Features section.
- **Eval:** `.claude/evolve/evals/add-guardrails-to-readme-features.md`

---

## Instincts Applied

- **inst-004** (grep-based-evals-effective) — eval definitions use grep-based checks
- **inst-007** (orchestrator-as-builder) — all 3 tasks are S complexity, fully specified, eligible for inline implementation
- **inst-010** (deferred-security-escalates) — cycle cap boundary fix was deferred 1 cycle (cycle 4), now prioritized as first task
