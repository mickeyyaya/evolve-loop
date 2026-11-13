# Cycle 2343 Dossier

**Goal:** Wave 168.

Work the highest-weight lane-eligible inbox items from start to end: claim, tdd, build, audit and ship. Keep full phase integrity.

Every cycle must:
- commit to a claimed inbox item (an empty top_n must not run the spine);
- bind its audit to the changes tree that it ships;
- disposition its own defect ledger and the inherited ids;
- record a terminal outcome on every exit;
- never change a protected control-plane file with a tool;
- leave no stale worktree.

Routing:
- An item whose declared fix surface is a protected file is console work. A pipeline-integrity item that the operator did not route to the lanes is also console work.
- An item that has `route: lane` is lane work. ADR-0099 document deliverables are lane work.
- A card on a protected surface is a route, not a verdict. The cycle ends as planned no-work, and the item goes to the console.

Craft:
- New and changed code has no comments. Names, types and tests give the intent.
- Write each new document and each new log text in ASD-STE100.
- A git-backed test fixture uses internal/gittest. It never uses a raw `git init`.
- A lane never changes go/internal/sizeratchet/offenders.json or the deadline of a gate.

Evidence:
- To know what a cycle did, read the cycle state and the signals (`evolve wave status`). Do not read the text log.

The measure of this wave is the number of shipped cycles. Keep each cycle shipping.

Wave 167 facts (run 20261112T235603Z): 1 of 4 cycles shipped.
- Cycle 2337: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…helper instead of a copy (docs/architecture/test-system.md §6.4, §6.5):   acs/cycle-N/predicates_test.go:moduleDir = acs/cycle-N/predicates_test.go:repoModule. Last phase: aborted. It did not ship.
- Cycle 2338: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…hase scout: phase scout: scout: bridge: bridge: launch exit=87: [agy-claude-tmux] session killed: evolve-bridge-agy-r01M793F1-c2338-scout-pid42468-n5-1794528311. Last phase: aborted. It did not ship.
- Cycle 2339: final verdict PASS. It shipped.
- Cycle 2340: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…hase scout: phase scout: scout: bridge: bridge: launch exit=87: [agy-claude-tmux] session killed: evolve-bridge-agy-r01M793F1-c2340-scout-pid42466-n5-1794528311. Last phase: aborted. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 164 (run 20261110T083435Z): 8 of 16 cycles shipped.
- Wave 165 (run 20261111T013325Z): 14 of 32 cycles shipped.
- Wave 166 (run 20261111T162732Z): 11 of 16 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M79QE6X1SK2HK5NYV9YX3D1D
**Committed:** `acs-cycle25-neg1-stale-runner-anchor`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 36s |  |
| triage | plan | PASS | 40s |  |
| tdd | plan | PASS | 1m40s |  |
| build | build | FAIL | 25m30s |  |
| retro | control | PASS | 1m55s |  |

## Timing

**Total:** 30m21s across 5 phases (0 retried) · **Longest:** build 25m30s

| Archetype | Wall-clock |
|-----------|------------|
| build | 25m30s |
| control | 1m55s |
| plan | 2m56s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|6c4c17e81730` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff: Explanation Documentation: cited path


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2343

