# Cycle 1940 Dossier

**Goal:** Wave 125.

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

Wave 124 facts (run 20261017T174807Z): 2 of 3 cycles shipped.
- Cycle 1937: final verdict PASS. It shipped.
- Cycle 1938: final verdict FAIL. Last phase: aborted. It did not ship.
- Cycle 1939: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Operator notes for wave 125:
- Wave 124 boundary merged #942 (O9: every lane go test gets one CPU budget), #946 (remediation numbered record), #947 (reportdoc indented continuation), #949 (Claude Code lag preflight warning), #950 (carried-ship binding rule), #952 (recalled lessons in the dossier), #955 (resume hook parity).

Earlier waves:
- Wave 121 (run 20261017T093651Z): 2 of 3 cycles shipped.
- Wave 122 (run 20261017T115657Z): 4 of 4 cycles shipped.
- Wave 123 (run 20261017T145443Z): 1 of 4 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M55PJSE0K1J7D7SY5YJ77N35
**Commit:** f66e987a65bdee63ee71ecdc5f020deaf0c242db
**Committed:** `cycle-ship-enforces-cover-changed`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 44s |  |
| triage | plan | PASS | 31s |  |
| tdd | plan | PASS | 1m36s |  |
| build | build | PASS | 42s |  |
| audit | evaluate | FAIL | 4m47s |  |
| tdd | plan | PASS | 4m3s |  |
| build | build | PASS | 7m22s |  |
| audit | evaluate | PASS | 6m9s |  |
| ship | control | PASS | 3m0s |  |

## Timing

**Total:** 28m55s across 9 phases (0 retried) · **Longest:** build 7m22s

| Archetype | Wall-clock |
|-----------|------------|
| build | 8m4s |
| control | 3m0s |
| evaluate | 10m56s |
| plan | 6m55s |
