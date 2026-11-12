# Cycle 2328 Dossier

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
**Final verdict:** PASS
**Run ID:** 01M76H6V3HBCDSE71G1SDPG80Y
**Commit:** ae7704f57317d0498e0a06081c16e37aeade3faf
**Committed:** `convergence-policy-v8-signals-and-metrics`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 36s |  |
| triage | plan | PASS | 32s |  |
| tdd | plan | PASS | 47s |  |
| build | build | PASS | 34m34s |  |
| audit | evaluate | PASS | 1m53s |  |
| ship | control | FAIL | 8m8s |  |
| audit | evaluate | PASS | 1m45s |  |
| ship | control | FAIL | 8m8s |  |
| audit | evaluate | PASS | 1m33s |  |
| ship | control | PASS | 8m16s |  |

## Timing

**Total:** 1h6m12s across 10 phases (0 retried) · **Longest:** build 34m34s

| Archetype | Wall-clock |
|-----------|------------|
| build | 34m34s |
| control | 24m32s |
| evaluate | 5m11s |
| plan | 1m54s |
