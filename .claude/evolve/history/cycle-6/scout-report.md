# Cycle 6 Scout Report

## Discovery Summary
- Scan mode: incremental (v6.0.0 upgrade focused)
- Files analyzed: 14 (git diff HEAD~20..HEAD, all v5/v6 changed files)
- Research: skipped (cooldown active — queries from 2026-03-13 within 12hr TTL)
- Instincts applied: 3 (inst-004 grep-based evals, inst-007 S-complexity inline, inst-011 version-sync-on-changelog)

## Key Findings

### Version Consistency — HIGH
`marketplace.json` is stuck at version `4.2.0` while `plugin.json` and the CHANGELOG both say `6.0.0`. The inst-011 instinct (version-sync-on-changelog, confidence 0.7) directly predicts this class of drift. The v5.0.0 and v6.0.0 bumps both updated `plugin.json` correctly but skipped `marketplace.json`. This is the third occurrence of version drift — the previous was caught in cycle 5.

### Install Script Stale Version String — MEDIUM
`install.sh` line 101 still prints `"Installing Evolve Loop v4..."`. The plugin went through v5 and v6 without this string being updated. Any user running manual install sees a misleading version announcement. Minor but causes confusion and undermines trust in the install process.

### README Workspace Layout Missing v6 Directories — MEDIUM
The README "Workspace Layout" section (lines 206-223) does not list the three new directories added in v6.0.0: `genes/`, `tools/`, and `instincts/archived/`. These are created by the SKILL.md initialization block and referenced throughout `phases.md`, `agents/evolve-builder.md`, and `docs/genes.md`. A user reading the README to understand the project's file structure gets an incomplete picture. This is a documentation gap introduced by the v6 upgrade.

### `synthesizedTools` Missing from state.json Schema — MEDIUM
The CHANGELOG [6.0.0] entry explicitly lists `synthesizedTools` as a new `state.json` field. The `evolve-builder.md` agent references it (line 102-104) and writes to it. However:
- The SKILL.md initialization block (the fresh-project default schema) does not include `synthesizedTools`.
- `memory-protocol.md`'s state.json schema example also omits it.

This means projects running from a fresh state.json will not have the field pre-initialized, causing the Builder to write to an undefined key. While most JSON parsers tolerate this (it would be added on first write), the schema documentation is inconsistent with the actual runtime behavior.

### Island Model `--island` Flag — LOW (deferred)
`docs/island-model.md` shows usage like `/evolve-loop 3 --island 1` and `/evolve-loop --migrate`, but SKILL.md argument parsing has no handling for `--island` or `--migrate` flags. The island model is marked as "advanced" so this is an expected future gap, not a bug. Deferring.

### `nothingToDoCount` Orphan Field — LOW (deferred)
The state.json init schema in SKILL.md still includes the legacy `nothingToDoCount` at the top level (alongside the v5 `stagnation.nothingToDoCount`). This is a minor schema artifact from the v5 refactor. Low risk — not worth a cycle slot when higher-priority gaps exist.

## Research
Skipped — cooldown active. All queries recorded on 2026-03-13 are within the 12hr TTL. Goal is null (autonomous mode) so no external knowledge needed.

## Selected Tasks

### Task 1: Bump marketplace.json to v6.0.0
- **Slug:** bump-marketplace-version-to-6-0-0
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** Third occurrence of marketplace.json version drift (cycles 5 and now cycle 6). inst-011 directly flags this pattern. One-line fix with high visibility — marketplace.json is what users see when browsing the plugin registry. Blocking this from being highest-priority would violate the graduated instinct policy.
- **Acceptance Criteria:**
  - [ ] `marketplace.json` `plugins[0].version` equals `"6.0.0"`
  - [ ] `plugin.json` and `marketplace.json` versions match
  - [ ] CI workflow passes
- **Files to modify:** `.claude-plugin/marketplace.json`
- **Eval:** written to `evals/bump-marketplace-version-to-6-0-0.md`

### Task 2: Update install.sh version string from v4 to v6
- **Slug:** update-install-sh-version-string
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** `install.sh` prints "Installing Evolve Loop v4..." — two major versions behind. The script survived both v5 and v6 upgrades without this line being updated. Quick fix, meaningful accuracy improvement for users running manual installs.
- **Acceptance Criteria:**
  - [ ] `install.sh` no longer contains "Evolve Loop v4"
  - [ ] `install.sh` contains "Evolve Loop v6" in the manual install echo
  - [ ] CI validation (CI mode) still passes
- **Files to modify:** `install.sh`
- **Eval:** written to `evals/update-install-sh-version-string.md`

### Task 3: Add v6 directories to README workspace layout
- **Slug:** add-v6-dirs-to-readme-workspace-layout
- **Type:** techdebt
- **Complexity:** S
- **Rationale:** README is the project's front door. The workspace layout diagram is actively wrong post-v6 — it omits `genes/`, `tools/`, and `instincts/archived/`. Users who try to understand where the system writes its artifacts will be confused. Three directory entries + brief comments, entirely in README.md.
- **Acceptance Criteria:**
  - [ ] README workspace layout lists `genes/` directory
  - [ ] README workspace layout lists `tools/` directory
  - [ ] README workspace layout lists `instincts/archived/` under instincts/
  - [ ] Each new entry has a brief description comment
- **Files to modify:** `README.md`
- **Eval:** written to `evals/add-v6-dirs-to-readme-workspace-layout.md`

### Task 4: Add `synthesizedTools` to state.json init schema and memory-protocol
- **Slug:** add-synthesizedtools-to-state-schema
- **Type:** stability
- **Complexity:** S
- **Rationale:** The CHANGELOG documents `synthesizedTools` as a new v6 field. The Builder writes to it. But neither the SKILL.md initialization block nor the memory-protocol schema example includes it. This is a schema consistency gap — the authoritative schema docs are out of sync with actual runtime behavior. Fix is small: add `"synthesizedTools": []` to the init block in SKILL.md and add the field to memory-protocol's schema reference.
- **Acceptance Criteria:**
  - [ ] SKILL.md init JSON block contains `"synthesizedTools": []`
  - [ ] `memory-protocol.md` state.json schema includes `synthesizedTools` field with documentation
- **Files to modify:** `skills/evolve-loop/SKILL.md`, `skills/evolve-loop/memory-protocol.md`
- **Eval:** written to `evals/add-synthesizedtools-to-state-schema.md`

## Deferred
- `--island` / `--migrate` argument handling: Island model is explicitly "advanced" and the docs themselves say it's for 20+ cycle projects. Not a gap to fill this cycle.
- `nothingToDoCount` orphan field cleanup: Low-risk schema artifact, outweighed by higher-impact tasks above.
- Instinct global promotion mechanism: Still L complexity, deferred from cycle 4.
