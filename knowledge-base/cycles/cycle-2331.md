# Cycle 2331 Dossier

**Goal:** Wave 166.

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

Wave 165 facts (run 20261111T013325Z): 14 of 32 cycles shipped.
- 12 older cycles are not shown.
- Cycle 2298: final verdict PASS. It shipped.
- Cycle 2299: final verdict PASS. It shipped.
- Cycle 2300: final verdict PASS. It shipped.
- Cycle 2301: final verdict SKIPPED_UNKNOWN. Termination: triage-empty-commitment. Last phase: triage. It did not ship.
- Cycle 2303: no final verdict. It did not ship.
- Cycle 2304: no final verdict. It did not ship.
- Cycle 2305: no final verdict. It did not ship.
- Cycle 2306: no final verdict. It did not ship.
- Cycle 2307: no final verdict. It did not ship.
- Cycle 2308: no final verdict. It did not ship.
- Cycle 2309: no final verdict. It did not ship.
- Cycle 2310: no final verdict. It did not ship.
- Cycle 2311: no final verdict. It did not ship.
- Cycle 2312: no final verdict. It did not ship.
- Cycle 2313: no final verdict. It did not ship.
- Cycle 2314: no final verdict. It did not ship.
- Cycle 2315: no final verdict. It did not ship.
- Cycle 2316: no final verdict. It did not ship.
- Cycle 2317: no final verdict. It did not ship.
- Cycle 2318: no final verdict. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 162 (run 20261109T053101Z): 11 of 16 cycles shipped.
- Wave 163 (run 20261109T204337Z): 14 of 16 cycles shipped.
- Wave 164 (run 20261110T083435Z): 8 of 16 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M76RPP4Y2R85NQF5DZYQC7A3
**Committed:** `loop-goal-include-exclude-selector`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 36s |  |
| triage | plan | PASS | 40s |  |
| spec-verify | plan | PASS | 1m3s |  |
| tdd | plan | PASS | 52s |  |
| retro | control | PASS | 2m1s |  |

## Timing

**Total:** 5m12s across 5 phases (0 retried) · **Longest:** retro 2m1s

| Archetype | Wall-clock |
|-----------|------------|
| control | 2m1s |
| plan | 3m11s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `tdd|gate-block|019f8e480165` · **Class:** gate-block

- review gate: phase "tdd" deliverable rejected after 1 correction(s): tdd handoff floor: 1 helper body copy group(s) in go/acs/cycle2331/ — no later phase can change these files, so fix them now: cal


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2331

