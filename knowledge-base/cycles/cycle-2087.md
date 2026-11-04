# Cycle 2087 Dossier

**Goal:** Wave 147.

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

Wave 146 facts (run 20261101T235937Z): 0 of 2 cycles shipped.
- Cycle 2071: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) teardown=submit_wedged: …lanced=85 -> agy-claude-tmux@balanced=85 met a quota wall, but rung claude-tmux@balanced failed with no evidence of exhaustion, so the walk is not a quota pause. Last phase: aborted. It did not ship.
- Cycle 2072: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) teardown=submit_wedged: …lanced=85 -> agy-claude-tmux@balanced=85 met a quota wall, but rung claude-tmux@balanced failed with no evidence of exhaustion, so the walk is not a quota pause. Last phase: aborted. It did not ship.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 143 (run 20261027T021525Z): 9 of 16 cycles shipped.
- Wave 144 (run 20261027T071914Z): 3 of 5 cycles shipped.
- Wave 145 (run 20261027T100547Z): 6 of 13 cycles shipped.
**Final verdict:** PASS
**Run ID:** 01M6D1D6N7EZ9665B7AG8ZEV4R
**Commit:** eb57adbbe5c5a700e6a2b52994b8ee7770be8153
**Committed:** `cli-rebuild-console`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | FAIL | 42s |  |
| scout | plan | PASS | 54s |  |
| triage | plan | PASS | 35s |  |
| tdd | plan | PASS | 3m15s |  |
| build-planner | plan | WARN | 0s |  |
| build | build | PASS | 26m24s |  |
| audit | evaluate | PASS | 2m39s |  |
| ship | control | FAIL | 2m36s |  |
| audit | evaluate | PASS | 3m56s |  |
| ship | control | PASS | 3m1s |  |

## Timing

**Total:** 44m3s across 10 phases (1 retried) · **Longest:** build 26m24s

| Archetype | Wall-clock |
|-----------|------------|
| build | 26m24s |
| control | 5m37s |
| evaluate | 6m35s |
| plan | 5m27s |
