# Cycle 2550 Dossier

**Goal:** Wave 182.

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

Wave 181 facts (run 20261119T004314Z): 22 of 23 cycles shipped.
- 3 older cycles are not shown.
- Cycle 2525: final verdict PASS. It shipped.
- Cycle 2526: final verdict PASS. It shipped.
- Cycle 2527: final verdict PASS. It shipped.
- Cycle 2528: final verdict PASS. It shipped.
- Cycle 2529: final verdict PASS. It shipped.
- Cycle 2530: final verdict PASS. It shipped.
- Cycle 2531: final verdict PASS. It shipped.
- Cycle 2532: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…nly greps source text proves nothing about behaviour; rewrite it to run the code before build dispatch. BLOCKING at enforce: 1 of the 1 finding(s), listed first. Last phase: aborted. It did not ship.
- Cycle 2533: final verdict PASS. It shipped.
- Cycle 2534: final verdict PASS. It shipped.
- Cycle 2535: final verdict PASS. It shipped.
- Cycle 2536: final verdict PASS. It shipped.
- Cycle 2537: final verdict PASS. It shipped.
- Cycle 2538: final verdict PASS. It shipped.
- Cycle 2539: final verdict PASS. It shipped.
- Cycle 2540: final verdict PASS. It shipped.
- Cycle 2541: final verdict PASS. It shipped.
- Cycle 2542: final verdict PASS. It shipped.
- Cycle 2543: final verdict PASS. It shipped.
- Cycle 2544: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Operator notes for wave 182:
- Tests: check an error by errors.Is or errors.As, never by err.Error() text; a fakeclitest.Install call names its path at the call (filepath.Join(dir, "tool")), and the tool has a row in go/internal/testaudit/fakes.json. Run go test -count=1 ./internal/testaudit/ before handoff: the tree ratchet turned main red four times in wave 181.

Earlier waves:
- Wave 178 (run 20261117T101039Z): 10 of 11 cycles shipped.
- Wave 179 (run 20261117T184146Z): 11 of 22 cycles shipped.
- Wave 180 (run 20261118T052545Z): 21 of 32 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M7TWBYYWR8VN21QFRH2KQG49
**Commit:** 00af44009b559be59a00a155ae1024d3073d9cfe
**Committed:** `claude-2316-large-paste-submit-wedge`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 42s |  |
| triage | plan | PASS | 36s |  |
| tdd | plan | PASS | 1m29s |  |
| build | build | PASS | 5m12s |  |
| audit | evaluate | FAIL | 2m16s |  |
| tdd | plan | PASS | 2m33s |  |
| build | build | PASS | 6m58s |  |
| audit | evaluate | WARN | 4m19s |  |
| ship | control | FAIL | 27m39s |  |
| ship | control | FAIL | 18m26s |  |
| audit | evaluate | WARN | 4m20s |  |
| ship | control | FAIL | 0s |  |
| ship | control | FAIL | 7m22s |  |
| audit | evaluate | WARN | 1m45s |  |
| ship | control | FAIL | 0s |  |
| ship | control | PASS | 6m24s |  |

## Timing

**Total:** 1h30m2s across 16 phases (0 retried) · **Longest:** ship 27m39s

| Archetype | Wall-clock |
|-----------|------------|
| build | 12m10s |
| control | 59m52s |
| evaluate | 12m40s |
| plan | 5m20s |
