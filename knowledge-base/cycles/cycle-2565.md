# Cycle 2565 Dossier

**Goal:** Wave 183.

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

Wave 182 facts (run 20261119T213950Z): 7 of 11 cycles shipped.
- Cycle 2546: final verdict PASS. It shipped.
- Cycle 2547: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2548: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2549: final verdict PASS. It shipped.
- Cycle 2550: final verdict PASS. It shipped.
- Cycle 2551: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2552: final verdict PASS. It shipped.
- Cycle 2553: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2554: final verdict PASS. It shipped.
- Cycle 2555: final verdict PASS. It shipped.
- Cycle 2556: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Operator notes for wave 183:
- Inbox review 2026-11-20: 106 proposed drop/merge ids are excluded this wave until the console verifies and applies them at the next boundary.

Earlier waves:
- Wave 179 (run 20261117T184146Z): 11 of 22 cycles shipped.
- Wave 180 (run 20261118T052545Z): 21 of 32 cycles shipped.
- Wave 181 (run 20261119T004314Z): 22 of 23 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M7W8XFSBJAS1536KGCJMVF5H
**Committed:** `integration-batch-ejects-a-pr-whose-head-ci-is-red`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 38s |  |
| triage | plan | PASS | 36s |  |
| tdd | plan | PASS | 6m59s |  |
| build | build | PASS | 31m23s |  |
| audit | evaluate | FAIL | 5m42s |  |
| build | build | PASS | 30m48s |  |
| audit | evaluate | PASS | 2m34s |  |
| ship | control | FAIL | 0s |  |
| ship | control | FAIL | 14m18s |  |
| audit | evaluate | PASS | 2m6s |  |
| ship | control | FAIL | 1s |  |
| ship | control | FAIL | 15m33s |  |
| retro | control | PASS | 2m27s |  |

## Timing

**Total:** 1h53m5s across 13 phases (0 retried) · **Longest:** build 31m23s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h2m11s |
| control | 32m19s |
| evaluate | 10m22s |
| plan | 8m14s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|a58bb25c1d0a` · **Class:** unknown

- phase ship: ship: native: [GIT_PUSH_REJECTED/precondition @atomic-ship] ship: origin rejected the push of a06497b32871300aeb69f5afd432a3c9b82b8824 because origin/main diverged; Audit must run again on


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2565

