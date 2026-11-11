# Cycle 2252 Dossier

**Goal:** Wave 163.

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

Wave 162 facts (run 20261109T053101Z): 11 of 16 cycles shipped.
- Cycle 2231: final verdict PASS. It shipped. Login walls: 1.
- Cycle 2236: final verdict PASS. It shipped.
- Cycle 2237: final verdict PASS. It shipped.
- Cycle 2238: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) teardown=review_pause: … claude-tmux@balanced=81 -> agy-claude-tmux@balanced=85 met a wall, but rung agy-tmux@balanced failed with no evidence of exhaustion, so the walk is not a pause. Last phase: aborted. It did not ship.
- Cycle 2239: final verdict PASS. It shipped.
- Cycle 2240: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…tDuplicates_TheCountOfCopiesEqualsTheBaseline — pushing would red main; land the green fix or use an explicit t.Skip for an intentionally red-first reproducer. Last phase: aborted. It did not ship.
- Cycle 2241: final verdict PASS. It shipped.
- Cycle 2242: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…t does not need the path. If the task cannot be completed without changing it, still restore it and record that in build-report.md — the item is console work.. Last phase: aborted. It did not ship.
- Cycle 2243: final verdict PASS. It shipped.
- Cycle 2244: final verdict PASS. It shipped.
- Cycle 2245: final verdict PASS. It shipped.
- Cycle 2246: final verdict FAIL. Last phase: retro. It did not ship.
- Cycle 2247: final verdict PASS. It shipped.
- Cycle 2248: final verdict PASS. It shipped.
- Cycle 2249: final verdict FAIL. Termination: cycle aborted in phase retro (abnormal-exit epilogue) cause=…tDuplicates_TheCountOfCopiesEqualsTheBaseline — pushing would red main; land the green fix or use an explicit t.Skip for an intentionally red-first reproducer. Last phase: aborted. It did not ship.
- Cycle 2250: final verdict PASS. It shipped.
- Pull requests merged at this boundary: none.

Earlier waves:
- Wave 159 (run 20261106T154410Z): 0 of 4 cycles shipped.
- Wave 160 (run 20261107T090007Z): 9 of 16 cycles shipped.
- Wave 161 (run 20261108T161932Z): 9 of 14 cycles shipped.
**Final verdict:** FAIL
**Run ID:** 01M7118ETV6RJZCADB5YZGZN84
**Committed:** `gc-legacy-gap-x10-gc-runs-when-a-cycle-seals`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 46s |  |
| triage | plan | PASS | 27s |  |
| spec-verify | plan | PASS | 37s |  |
| tdd | plan | PASS | 48s |  |
| build | build | PASS | 1h22m4s |  |
| audit | evaluate | PASS | 3m56s |  |
| ship | control | FAIL | 6m36s |  |
| audit | evaluate | PASS | 7m29s |  |
| ship | control | FAIL | 11m37s |  |
| ship | control | FAIL | 8m21s |  |
| audit | evaluate | PASS | 8m20s |  |
| ship | control | FAIL | 6m57s |  |
| ship | control | FAIL | 6m47s |  |
| audit | evaluate | PASS | 5m44s |  |
| ship | control | FAIL | 16m15s |  |
| retro | control | PASS | 1m33s |  |

## Timing

**Total:** 2h48m18s across 16 phases (0 retried) · **Longest:** build 1h22m4s

| Archetype | Wall-clock |
|-----------|------------|
| build | 1h22m4s |
| control | 58m5s |
| evaluate | 25m29s |
| plan | 2m39s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|647627e835cc` · **Class:** unknown

- phase ship: ship: native: [GIT_FLEET_REBASE_NEEDED/transient @atomic-ship] ship: fleet ff-merge cycle-cd3ae73e-2252 into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the me


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 2252

