# Cycle 1950 Dossier

**Goal:** Wave 128.

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

Wave 127 facts (run 20261018T104954Z): 2 of 2 cycles shipped.
- Cycle 1943: final verdict PASS. It shipped.
- Cycle 1944: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 124 (run 20261017T174807Z): 2 of 3 cycles shipped.
- Wave 125 (run 20261017T194548Z): 1 of 1 cycles shipped.
- Wave 126 (run 20261017T214840Z): 1 of 2 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M57Q8VGGQXF2XHHMMKVNQVYM
**Committed:** `core-package-coverage-batches-after-1944`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 1m37s |  |
| triage | plan | PASS | 29s |  |
| tdd | plan | PASS | 1m56s |  |
| build | build | PASS | 25s |  |
| retro | control | PASS | 1m56s |  |

## Timing

**Total:** 6m23s across 5 phases (0 retried) · **Longest:** retro 1m56s

| Archetype | Wall-clock |
|-----------|------------|
| build | 25s |
| control | 1m56s |
| plan | 4m2s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `build|gate-block|3c6d40620b62` · **Class:** gate-block

- review gate: phase "build" deliverable rejected after 2 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff: ./acs/cycle1950 (-tags acs): unit tes


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1950

