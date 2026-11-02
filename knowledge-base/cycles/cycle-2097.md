# Cycle 2097 Dossier

**Goal:** Wave 148.

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

Wave 147 facts (run 20261102T011732Z): 1 of 16 cycles shipped.
- Cycle 2073: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…apabilities: it has no tool use. A manifest written before X9 also lacks parsers, collectors, transcript and context_window; add them or name a base. [bridge…. Last phase: aborted. It did not ship.
- Cycle 2074: final verdict PASS. It shipped.
- Cycle 2075: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) teardown=…n phase build: phase build: build: bridge: bridge: launch exit=85: escalation report written (pattern=exhausted reason=escalate): core: transient bridge failure. Last phase: aborted. It did not ship.
- Cycle 2076: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…-as-an-inbox-item.md   Explanation Documentation: Changed Areas does not explain material path solutions/landing-plans-reconcile-with-adr-0147/recommendation.md. Last phase: aborted. It did not ship.
- Cycle 2077: no final verdict. Last phase: tdd. It did not ship. Quota pauses: 1.
- Cycle 2078: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2079: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2080: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2081: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2082: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2083: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2084: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2085: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2086: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2087: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Cycle 2088: no final verdict. Last phase: scout. It did not ship. Quota pauses: 1.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 144 (run 20261027T071914Z): 3 of 5 cycles shipped.
- Wave 145 (run 20261027T100547Z): 6 of 13 cycles shipped.
- Wave 146 (run 20261101T235937Z): 0 of 2 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M6D9Z1V86K8MVNCSNKWXFAXR
**Committed:** `cli-loop-dry-run-wave-plan-console`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 36s |  |
| triage | plan | PASS | 36s |  |
| spec-verify | plan | PASS | 1m25s |  |
| tdd | plan | PASS | 2m33s |  |
| build | build | PASS | 3m15s |  |
| test-amplification | evaluate | PASS | 1m17s |  |
| audit | evaluate | PASS | 4m46s |  |
| ship | control | FAIL | 3m11s |  |
| audit | evaluate | PASS | 2m44s |  |
| ship | control | FAIL | 2m43s |  |
| build | build | PASS | 44s |  |
| audit | evaluate | PASS | 5m6s |  |
| ship | control | FAIL | 3m53s |  |
| build | build | PASS | 1h5m4s |  |
| audit | evaluate | PASS | 3m14s |  |
| ship | control | FAIL | 2m53s |  |
| build | build | PASS | 21m44s |  |
| audit | evaluate | PASS | 2m43s |  |
| ship | control | FAIL | 4m53s |  |
| build | build | PASS | 16m49s |  |
| audit | evaluate | PASS | 1m44s |  |
| ship | control | FAIL | 2m24s |  |
| retro | control | PASS | 2m26s |  |

## Timing

**Total:** 2h36m44s across 23 phases (0 retried) · **Longest:** build 1h5m4s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h47m37s |
| control | 22m23s |
| evaluate | 21m34s |
| plan | 5m11s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|a04be35f076f` · **Class:** unknown

- phase ship: ship: native: [GIT_FLEET_REBASE_NEEDED/transient @atomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2097 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the me


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2097

