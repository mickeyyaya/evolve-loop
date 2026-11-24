# Cycle 2615 Dossier

**Goal:** Wave 188.

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

Wave 187 facts (run 20261122T121257Z): 1 of 2 cycles shipped.
- Cycle 2606: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…ALiveProcessHolds skip-hides-error has no baseline row — remove the weak pattern (for error text: a sentinel error and errors.Is); Rebaseline refuses new rows. Last phase: aborted. It did not ship.
- Cycle 2607: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 184 (run 20261120T204107Z): 5 of 8 cycles shipped.
- Wave 185 (run 20261121T081737Z): 4 of 9 cycles shipped.
- Wave 186 (run 20261121T192937Z): 10 of 13 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M84ERQTS7KAWGH3S7KPPSHG7
**Committed:** `integration-batch-waits-for-the-ci-run-to-register`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 1m13s |  |
| triage | plan | PASS | 1m21s |  |
| tdd | plan | PASS | 3m34s |  |
| build | build | FAIL | 6m18s |  |
| retro | control | PASS | 1m50s |  |

## Timing

**Total:** 14m16s across 5 phases (0 retried) · **Longest:** build 6m18s

| Archetype | Wall-clock |
|-----------|------------|
| build | 6m18s |
| control | 1m50s |
| plan | 6m8s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|e5f4456a329a` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff: test-audit ratchet: testaudit: acs/cy


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2615

