# Cycle 2018 Dossier

**Goal:** Wave 138.

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

Wave 137 facts (run 20261020T063647Z): 2 of 5 cycles shipped.
- Cycle 2009: final verdict PASS. It shipped.
- Cycle 2010: no final verdict. Last phase: tdd. It did not ship.
- Cycle 2011: final verdict PASS. It shipped.
- Cycle 2012: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2013: no final verdict. Last phase: build. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 134 (run 20261019T231825Z): 6 of 12 cycles shipped.
- Wave 135 (run 20261020T024201Z): 1 of 4 cycles shipped.
- Wave 136 (run 20261020T042221Z): 0 of 1 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M5CWVMPCARVQ479JXFBW1SY6
**Committed:** `core-one-diagnostics-writer-lane`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 47s |  |
| triage | plan | PASS | 39s |  |
| tdd | plan | PASS | 6m5s |  |
| behavior-baseline | evaluate | PASS | 1m40s |  |
| build | build | PASS | 9m17s |  |
| retro | control | FAIL | 2m15s |  |

## Timing

**Total:** 20m43s across 6 phases (0 retried) · **Longest:** build 9m17s

| Archetype | Wall-clock |
|-----------|------------|
| build | 9m17s |
| control | 2m15s |
| evaluate | 1m40s |
| plan | 7m31s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|54f837499ca2` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 2 correction(s): build handoff floor: 9 deterministic check failure(s) — fix these exactly before handoff: protected control-plane path changed:


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2018

