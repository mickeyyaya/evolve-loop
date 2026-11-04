# Cycle 2152 Dossier

**Goal:** Wave 154.

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

Wave 153 facts (run 20261104T002909Z): 0 of 1 cycles shipped.
- Cycle 2151: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…nternal/repocontract 2.567s coverage: 100.0% of statements ok   github.com/mickeyyaya/evolve-loop/go/internal/sysexec 5.112s coverage: 100.0% of statements FAIL. Last phase: aborted. It did not ship.
- Pull requests merged at this boundary: none.

Operator notes for wave 154:
- Resume paused cycles 2144-2150 concurrently through lanes. Cycle 2151 sealed FAIL at the build floor: internal/bridge red under -tags integration with no test name, 6/6 reruns green on the same tree; treat a nameless floor red as infra.

Earlier waves:
- Wave 150 (run 20261102T171134Z): 7 of 12 cycles shipped.
- Wave 151 (run 20261103T022654Z): 11 of 15 cycles shipped.
- Wave 152 (run 20261103T165142Z): 1 of 9 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M6JFNHEBTJTBKJ7TK5D14ZYR
**Committed:** `recovery-retrospective-c4-stage-key-publish`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 46s |  |
| triage | plan | PASS | 36s |  |
| tdd | plan | PASS | 5m22s |  |
| build | build | FAIL | 21m16s |  |
| retro | control | PASS | 8m53s |  |

## Timing

**Total:** 36m52s across 5 phases (0 retried) · **Longest:** build 21m16s

| Archetype | Wall-clock |
|-----------|------------|
| build | 21m16s |
| control | 8m53s |
| plan | 6m43s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|b67d756456e1` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: ./cmd/evolve: unit tests FAIL …26-1


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2152

