# Cycle 2085 Dossier

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
**Final verdict:** FAIL
**Run ID:** 01M6D1D1394AXT2APKJQPGB9DY
**Committed:** `convergence-policy-v5-audit-repair-decide`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | FAIL | 45s |  |
| scout | plan | PASS | 1m12s |  |
| triage | plan | PASS | 45s |  |
| tdd | plan | PASS | 8m45s |  |
| build-planner | plan | WARN | 0s |  |
| build | build | PASS | 14m30s |  |
| audit | evaluate | PASS | 4m3s |  |
| ship | control | FAIL | 15m3s |  |

## Timing

**Total:** 45m3s across 8 phases (1 retried) · **Longest:** ship 15m3s

| Archetype | Wall-clock |
|-----------|------------|
| build | 14m30s |
| control | 15m3s |
| evaluate | 4m3s |
| plan | 11m27s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|cf9f927b78f5` · **Class:** unknown

- cycle aborted in phase ship (abnormal-exit epilogue) cause=…tomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2085 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the mer


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2085

