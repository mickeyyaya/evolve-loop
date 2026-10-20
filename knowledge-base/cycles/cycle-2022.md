# Cycle 2022 Dossier

**Goal:** Wave 139.

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

Wave 138 facts (run 20261020T123050Z): 1 of 6 cycles shipped.
- Cycle 2014: final verdict PASS. It shipped.
- Cycle 2015: no final verdict. Last phase: build. It did not ship.
- Cycle 2016: no final verdict. Last phase: build. It did not ship. Quota pauses: 1.
- Cycle 2017: no final verdict. Last phase: build. It did not ship. Quota pauses: 1.
- Cycle 2018: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…a git repository (or any of the parent directories): .git fatal: not a git repository (or any of the parent directories): .git fatal: not a git repository (o…. Last phase: aborted. It did not ship.
- Cycle 2019: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 135 (run 20261020T024201Z): 1 of 4 cycles shipped.
- Wave 136 (run 20261020T042221Z): 0 of 1 cycles shipped.
- Wave 137 (run 20261020T063647Z): 2 of 5 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M5DAHPF0YZV6DNK2Y51A3T40
**Committed:** `cli-ship-manual-paths-lane`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 44s |  |
| triage | plan | PASS | 38s |  |
| tdd | plan | PASS | 4m8s |  |
| build | build | PASS | 11m52s |  |
| retro | control | PASS | 2m17s |  |

## Timing

**Total:** 19m39s across 5 phases (0 retried) · **Longest:** build 11m52s

| Archetype | Wall-clock |
|-----------|------------|
| build | 11m52s |
| control | 2m17s |
| plan | 5m30s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|ca73efa91ae3` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 3 correction(s): build handoff floor: 4 deterministic check failure(s) — fix these exactly before handoff: protected control-plane path changed:


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2022

