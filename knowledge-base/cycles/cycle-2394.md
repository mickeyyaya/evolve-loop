# Cycle 2394 Dossier

**Goal:** Wave 173.

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

Wave 172 facts (run 20261114T064820Z): 4 of 4 cycles shipped.
- Cycle 2374: final verdict PASS. It shipped.
- Cycle 2375: final verdict PASS. It shipped.
- Cycle 2376: final verdict PASS. It shipped.
- Cycle 2377: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 169 (run 20261113T084906Z): 1 of 4 cycles shipped.
- Wave 170 (run 20261113T094348Z): 1 of 4 cycles shipped.
- Wave 171 (run 20261113T194212Z): 9 of 16 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M7EDRAVT0EH73Y2J37H0JSEC
**Committed:** `arch-decisions-ad2-9-graft-d-an-on-demand-llm-curator`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m11s |  |
| triage | plan | PASS | 1m3s |  |
| tdd | plan | PASS | 40s |  |
| build | build | FAIL | 29m46s |  |
| retro | control | PASS | 2m10s |  |

## Timing

**Total:** 35m51s across 5 phases (0 retried) · **Longest:** build 29m46s

| Archetype | Wall-clock |
|-----------|------------|
| build | 29m46s |
| control | 2m10s |
| plan | 3m55s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|cda2b2eeccaa` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 2 correction(s): build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff: ./acs/cycle2394 (-tags acs): unit tes


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2394

