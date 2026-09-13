# Cycle 1663 Dossier

**Goal:** Pipeline-health verification batch (2026-09-12, two waves): work the highest-weight queued inbox items end-to-end — claim, tdd, build, audit, ship — with full phase integrity, now that the resume worktree teardown (#571), the shipDirect audit binding (#569), resume-path outcome recording (#568), and profile sandbox.write_subpaths grants (#572) are on main. Every cycle must commit to a claimed inbox item (an empty top_n must not run the spine), bind its audit to the changes tree it ships, record a terminal outcome on every exit, and leave no stale worktree behind. Pipeline-integrity and pipeline-repair items come first; ADR-0099 document deliverables are eligible.
**Final verdict:** PASS
**Run ID:** 01M2DTGZVSCT02QCZJPH3SBK6P
**Commit:** 4db205a889490d6788300920062f06fc7068e6b0
**Committed:** `dossier-producer-params-struct`, `lost-ship-dossier-evidence`

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m2s |  |
| triage | plan | PASS | 1m35s |  |
| fault-localization | plan | PASS | 2m28s |  |
| bug-reproduction | evaluate | PASS | 2m42s |  |
| tdd | plan | PASS | 12m19s |  |
| build | build | PASS | 12m42s |  |
| audit | evaluate | PASS | 10m1s |  |
| ship | control | FAIL | 7s |  |
| build | build | PASS | 25m42s |  |
| audit | evaluate | PASS | 7m52s |  |
| ship | control | PASS | 9s |  |

## Timing

**Total:** 1h17m40s across 11 phases (0 retried) · **Longest:** build 25m42s

| Archetype | Wall-clock |
|-----------|------------|
| build | 38m24s |
| control | 17s |
| evaluate | 20m35s |
| plan | 18m24s |
