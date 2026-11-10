# Cycle 2283 Dossier

**Goal:** Wave 164.

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

Wave 163 facts (run 20261109T204337Z): 14 of 16 cycles shipped.
- Cycle 2252: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…tomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2252 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the merged tree, then re-ship. Last phase: aborted. It did not ship.
- Cycle 2253: final verdict PASS. It shipped.
- Cycle 2254: final verdict PASS. It shipped.
- Cycle 2255: final verdict PASS. It shipped.
- Cycle 2256: final verdict PASS. It shipped.
- Cycle 2257: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2258: final verdict PASS. It shipped.
- Cycle 2259: final verdict PASS. It shipped.
- Cycle 2260: final verdict PASS. It shipped.
- Cycle 2261: final verdict PASS. It shipped.
- Cycle 2262: final verdict PASS. It shipped.
- Cycle 2263: final verdict PASS. It shipped.
- Cycle 2264: final verdict PASS. It shipped.
- Cycle 2265: final verdict PASS. It shipped.
- Cycle 2266: final verdict PASS. It shipped.
- Cycle 2267: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 160 (run 20261107T090007Z): 9 of 16 cycles shipped.
- Wave 161 (run 20261108T161932Z): 9 of 14 cycles shipped.
- Wave 162 (run 20261109T053101Z): 11 of 16 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M73DYY2SR6FY9RNKN0S7YSW4
**Committed:** `gc-legacy-gap-x8-remote-sweep-at-shadow`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 55s |  |
| triage | plan | PASS | 31s |  |
| tdd | plan | PASS | 7m3s |  |
| build | build | FAIL | 20m59s |  |
| retro | control | PASS | 4m23s |  |

## Timing

**Total:** 33m52s across 5 phases (0 retried) · **Longest:** build 20m59s

| Archetype | Wall-clock |
|-----------|------------|
| build | 20m59s |
| control | 4m23s |
| plan | 8m30s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|a684222c7935` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: the tests of the changed packages fai


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2283

