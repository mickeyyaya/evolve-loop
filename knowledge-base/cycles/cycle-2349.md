# Cycle 2349 Dossier

**Goal:** Wave 169.

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

Wave 168 facts (run 20261113T054547Z): 3 of 4 cycles shipped.
- Cycle 2342: final verdict PASS. It shipped.
- Cycle 2343: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=… 2.181s ….com/mickeyyaya/evolve-loop/go/acs/cycle-N 0.156s         FAIL         stderr: FAIL FAIL github.com/mickeyyaya/evolve-loop/go/acs/cycle-N 2.181s FAIL. Last phase: aborted. It did not ship.
- Cycle 2344: final verdict PASS. It shipped.
- Cycle 2345: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 165 (run 20261111T013325Z): 14 of 32 cycles shipped.
- Wave 166 (run 20261111T162732Z): 11 of 16 cycles shipped.
- Wave 167 (run 20261112T235603Z): 1 of 4 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M7A1W9SG5EWX68GB2MHPZ9ZH
**Committed:** `rebuild-pr-1254-acs-compile`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 50s |  |
| triage | plan | PASS | 32s |  |
| tdd | plan | PASS | 41s |  |
| retro | control | PASS | 1m43s |  |

## Timing

**Total:** 3m46s across 4 phases (0 retried) · **Longest:** retro 1m43s

| Archetype | Wall-clock |
|-----------|------------|
| control | 1m43s |
| plan | 2m3s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `tdd|gate-block|8512a2303acf` · **Class:** gate-block

- review gate: phase "tdd" deliverable rejected after 1 correction(s): tdd handoff floor: 1 helper body copy group(s) in go/acs/cycle2349/ — no later phase can change these files, so fix them now: cal


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2349

