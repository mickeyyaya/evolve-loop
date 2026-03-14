# Cycle 2 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 12 (changed/related since cycle 1)
- Research: skipped (all queries within TTL — all dated 2026-03-13, 7-14 day TTL)
- Instincts applied: 4 (inst-001 through inst-004)

## Key Findings

### Pipeline Integrity — HIGH
- `skills/evolve-loop/eval-runner.md` contains stale v3 terminology throughout: "Phase 5.5", "Phase 6 (SHIP)", "Phase 7 (LOOP+LEARN)", "Developer agent", "Planner in Phase 2". This file is read by the orchestrator to run the eval gate. Stale phase references cause confusion and could lead to incorrect orchestrator behavior (e.g., wrong retry logic, wrong phase numbering in reports).

### State Schema — LOW
- `state.json` has a `costBudget: null` field listed in CHANGELOG v4 as "Removed". This is harmless dead weight but inconsistent with the documented schema in `memory-protocol.md`.

### Stability — MEDIUM
- `install.sh` has no non-interactive/CI mode. Running it in a CI environment or automated test will succeed silently since it has no interactive prompts, but there's no explicit `--ci` flag or `CI=` env var handling to suppress output or enable strict mode. This was deferred from cycle 1 and is needed for the CI workflow task below.

### Infrastructure — MEDIUM
- `.github/workflows/` directory exists but is empty. No CI workflow validates that `install.sh` runs cleanly, file structure is intact, or agent/skill files are well-formed. For a project distributed as a Claude Code plugin, CI is needed for contributor confidence.

## Instincts Applied
- **inst-001** (wt-cli-not-real): Confirmed no `wt` references in new v4 files — no action needed.
- **inst-002** (ecc-overlay-not-copy): v4 removed ECC entirely — pattern confirmed, no action needed.
- **inst-003** (ledger-ts-not-timestamp): All new v4 agent ledger schemas use `ts` — confirmed correct.
- **inst-004** (grep-based-evals-effective): Applying this pattern for all 3 eval definitions below.

## Selected Tasks

### Task 1: Fix eval-runner.md stale v3 references
- **Slug:** fix-eval-runner-stale-refs
- **Type:** stability
- **Complexity:** S
- **Rationale:** eval-runner.md is read by the orchestrator each cycle to run the eval gate. Stale references to v3 phases ("Phase 5.5", "Phase 7", "Developer agent", "Planner") create confusion and may cause wrong retry/skip behavior. This is the highest-priority fix since it directly affects pipeline correctness.
- **Acceptance Criteria:**
  - [ ] No references to "Phase 5.5", "Phase 6 (SHIP)", "Phase 7 (LOOP+LEARN)" in eval-runner.md
  - [ ] No references to "Developer agent" or "Planner in Phase 2" in eval-runner.md
  - [ ] Retry protocol updated to reference v4 roles (Builder) and phases (Phase 3/4/5)
  - [ ] Title/intro updated to describe the file's actual v4 role (part of Auditor's Phase 3)
- **Files to modify:** `skills/evolve-loop/eval-runner.md`
- **Eval:** written to `evals/fix-eval-runner-stale-refs.md`

### Task 2: Add CI/non-interactive mode to install.sh
- **Slug:** add-install-ci-mode
- **Type:** stability
- **Complexity:** S
- **Rationale:** install.sh is the primary test command for this project. CI environments should be able to run it with `CI=true` or `--ci` flag to get machine-readable output (no color codes, explicit exit codes, quiet mode). This unblocks the CI workflow task and enables automated validation.
- **Acceptance Criteria:**
  - [ ] `CI=true ./install.sh` runs without error and exits 0
  - [ ] `./install.sh --ci` is equivalent to `CI=true ./install.sh`
  - [ ] In CI mode, suppress decorative output (emoji-free, no color codes)
  - [ ] Non-CI mode behavior unchanged
- **Files to modify:** `install.sh`
- **Eval:** written to `evals/add-install-ci-mode.md`

### Task 3: Add GitHub Actions CI workflow
- **Slug:** add-ci-workflow
- **Type:** feature
- **Complexity:** M
- **Rationale:** The .github/workflows/ directory exists but is empty. A CI workflow validates install.sh on push/PR, checks file structure integrity (all required agent and skill files present), and ensures the project doesn't silently break for contributors. Deferred from cycle 1, now unblocked by Task 2.
- **Acceptance Criteria:**
  - [ ] `.github/workflows/ci.yml` exists and is valid YAML
  - [ ] Workflow triggers on push and pull_request
  - [ ] Workflow runs `./install.sh` (or `CI=true ./install.sh`)
  - [ ] Workflow checks that all required agent files exist (evolve-scout.md, evolve-builder.md, evolve-auditor.md, evolve-operator.md)
  - [ ] Workflow checks that all required skill files exist (SKILL.md, phases.md, memory-protocol.md, eval-runner.md)
- **Files to modify:** `.github/workflows/ci.yml` (create)
- **Eval:** written to `evals/add-ci-workflow.md`

## Deferred

- **Add instinct system documentation to README:** Low priority — README already has a "Continuous learning" bullet. Full documentation of the instinct YAML schema is better suited for a dedicated doc page. Defer to cycle 3.
- **Denial-of-wallet guardrails:** Requires architectural design (token budget enforcement, cycle cost caps). Complexity M-L, better as a standalone cycle when the pipeline is more stable. Defer to cycle 4+.
- **Remove stale `costBudget` field from state.json:** LOW severity, harmless. Will address opportunistically in a future cycle when state.json is touched for another reason.
