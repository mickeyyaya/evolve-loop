# Cycle 2368 Dossier

**Goal:** Wave 171.

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

Wave 170 facts (run 20261113T094348Z): 1 of 4 cycles shipped.
- Cycle 2352: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2353: final verdict PASS. It shipped.
- Cycle 2354: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2355: final verdict FAIL. Termination: triage-empty-commitment-claimable-work. Last phase: triage. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 167 (run 20261112T235603Z): 1 of 4 cycles shipped.
- Wave 168 (run 20261113T054547Z): 3 of 4 cycles shipped.
- Wave 169 (run 20261113T084906Z): 1 of 4 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M7BQY17E9XSMZKFD9ZHSSXCK
**Committed:** `handoff-request-reads-plain-files-only`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 45s |  |
| triage | plan | PASS | 38s |  |
| fault-localization | plan | PASS | 34s |  |
| tdd | plan | PASS | 4m45s |  |
| build | build | PASS | 24m13s |  |
| audit | evaluate | FAIL | 3m19s |  |
| tdd | plan | PASS | 3m50s |  |
| build | build | PASS | 35m13s |  |
| audit | evaluate | PASS | 5m51s |  |
| ship | control | FAIL | 12m24s |  |
| audit | evaluate | PASS | 12m59s |  |
| ship | control | FAIL | 8m4s |  |
| audit | evaluate | PASS | 14m2s |  |
| ship | control | FAIL | 7m2s |  |
| retro | control | PASS | 1m59s |  |

## Timing

**Total:** 2h15m38s across 15 phases (0 retried) · **Longest:** build 35m13s

| Archetype | Wall-clock |
|-----------|------------|
| build | 59m26s |
| control | 29m29s |
| evaluate | 36m11s |
| plan | 10m33s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|dfa9d33e4138` · **Class:** unknown

- phase ship: ship: native: [GIT_PUSH_REJECTED/precondition @atomic-ship] ship: origin rejected the push of b3cbdf68361bfd74516dcc97b9a8e0d5e716011f because origin/main diverged; Audit must run again on


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2368

