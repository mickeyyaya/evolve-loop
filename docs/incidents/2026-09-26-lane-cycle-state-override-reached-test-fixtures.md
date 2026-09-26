# 2026-09-26 — a lane's cycle-state override reached every test fixture it spawned

**Class:** pipeline, isolation. A process-wide override meant for one lane was honored by every process the lane spawned, including test binaries that had their own evolve dir.
**Surface:** `core.ResolveCycleStatePath` and `paths.Resolve`, which is to say every cycle-state reader and writer, the storage adapter included.
**Found by:** the console, root-causing cycle 1700's audit FAIL under the two-zero-ship-cycles halt rule.
**Related:** [2026-09-14 — the ship repo-contract gate ran `go test` in the lane's IPC environment](2026-09-14-ship-gate-inherits-the-lane-ipc-env.md), the same leak through a different spawner.

## What happened

Cycle 1700 took the item `dashboard-unchanged-root-seq-test-flakes-under-load`. The builder serialized `Server.refresh`. The auditor's narrative said PASS, and EGPS forced FAIL with two of the cycle's own predicates red. The first ran `go test -race` over `internal/dashboard` under parallel load; the second was a plain coverage run. Both red runs showed tests reading other tests' fixtures:
- `TestReadLoop_NoCycleStateIsQuiet` found CycleID 1606, which another test had written;
- `TestServer_UnchangedRootDoesNotBumpSeq` saw its sequence move without a change.

The repair round was declined because the audit declared no failure class, and the cycle sealed FAIL.

The same predicates pass on a quiet console tree. The package passes on main, and so does main with cycle 1700's change applied.

## Root cause

`cyclerun.go` sets `EVOLVE_CYCLE_STATE_FILE=<run dir>/cycle-state.json` process-wide in every fleet lane, so the lane's orchestrator and its guard hooks agree on the lane's own state. `core.ResolveCycleStatePath` and `paths.Resolve` returned that path for any evolve dir. The storage adapter writes through the same resolver, and so does the dashboard test helper `writeCycleState`.

Every `go test` a lane spawns inherits the variable: the audit's EGPS suite (`acssuite` passes `os.Environ()`), the CI-parity gates, and any test an agent runs in its pane. Every fixture in that test binary then wrote to one file, the lane's live cycle state. Two effects followed:
- **Tests read each other's fixtures.** The dashboard suite, which cycle 1700's item called "flaky under load", was flaky only inside a lane.
- **Tests overwrote the lane's live state.** The negative control on main ran `EVOLVE_CYCLE_STATE_FILE=<fake lane file> go test ./internal/dashboard/ ./internal/guards/`. It failed nine tests and left the fake lane file holding a guards fixture: `cycle_id 107, phase scout`. A lane that runs the guards suite can therefore replace its own state with a fixture whose CycleID is 0, which the role guard reads as "outside a cycle".

The repository had met this twice and patched it locally each time:
- `statejson_symlink_test.go` and `reset_symlink_test.go` clear the variable with the note "never write the live lane's cycle-state";
- #612's incident added `ipcenv.Scrub` at the ship gate and at core's four `go test` spawners.

Per-spawner scrubbing cannot close the class. The acs suite, `acsrunner`, `ciparity`, `releasepreflight` and every agent pane spawn tests too, and a pane must keep the variable for its own hooks.

## Fix

`paths.CycleStateFileFor(evolveDir, override)` is the one rule, and both resolvers delegate to it. The override applies only when it lies inside the evolve dir being resolved; otherwise the evolve dir's own `cycle-state.json` applies.
- A lane's orchestrator resolves its project's `.evolve`, and its override is under `.evolve/runs/cycle-N/`, so it is honored exactly as before.
- A test binary resolves `t.TempDir()/.evolve`, which does not hold the lane's file, so every fixture gets its own file however the test was spawned.
- Guard hooks in a cycle worktree resolve `<worktree>/.evolve`, whose `cycle-state.json` is a symlink to the run's `run.json`. They never depended on the override.
- The scoping rule makes one assumption explicit: a lane's storage must resolve against `<project-root>/.evolve`, the dir that holds its override. `evolve cycle run` now refuses a fleet lane given any other `--evolve-dir` (`fleetLaneEvolveDirOK`, pinned by `TestRunCycleRun_AFleetLaneRefusesAnEvolveDirOutsideItsProjectRoot`) rather than letting it fall back to the shared file. No launcher passes one today.
- Resume's per-run checkpoint discovery used to stand down whenever the variable was set. It now stands down only when an override governs this evolve dir, which is when the resolved path is not the evolve dir's own file. Another tree's override no longer hides this tree's checkpoints. Four `TestLoadResumeState_*` tests failed on main with a lane override exported, and now pass.

## Regression coverage

- `TestWriteCycleState_AFixtureNeverWritesAnotherTreesLiveLaneFile` (storage): a fixture write leaves a live lane file byte-identical.
- `TestResolveCycleStatePath_AnOverrideOutsideTheEvolveDirIsNotItsFile` and `TestResolveCycleStatePath_ALaneOverrideInsideItsEvolveDirIsHonored` (core), the latter including a sibling directory that shares the evolve dir's prefix.
- `TestResolve_CycleStateOverrideAppliesOnlyInsideItsEvolveDir` and `TestCycleStateFileFor_TheOverrideGovernsOnlyItsOwnEvolveDir` (paths). The table covers a sibling sharing the prefix, the evolve dir itself, a `..` escape and a relative override.
- `TestLoadResumeState_AnotherTreesLaneOverrideDoesNotStopDiscovery` (core).
- Mutation checks: honoring the override unconditionally turns the paths, storage and core tests red, and a string-prefix containment check fails the sibling case.
- The whole module passes with a lane override exported, and the exported file is untouched afterwards.

## Follow-ups

- `acssuite-lane-env-policy`: the EGPS suite still passes the lane's `EVOLVE_FLEET` and scope keys to predicates. That is the 2026-09-14 symptom through another door, and it needs an allowlist, since some predicates read `EVOLVE_*` on purpose.
- `audit-gate-forced-fail-earns-no-repair`: when a deterministic gate overrides a PASS narrative, the audit declares no failure class, so the cycle gets no repair round even though the red predicates name exactly what to fix.
