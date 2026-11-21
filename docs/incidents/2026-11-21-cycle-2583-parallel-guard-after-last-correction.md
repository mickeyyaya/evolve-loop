# 2026-11-21: a correction broke a test-structure guard, and no check showed the builder the failing test

The cause comes from the run records of cycles 2583 and 2508 and from the code of the build handoff floor. The skill of this record: [issue-record](../../skills/issue-record/SKILL.md).

## Summary

- **Impact:** cycle 2583 (sync-main removes untracked copies of origin files) failed at the build phase. The phase ran from 08:35:11Z to 12:20:58Z on 2026-11-21, about 3 hours 45 minutes. Cycle 2508 failed the same way on 2026-11-18 (first self-check 15:09:34Z, abort 18:11:08Z). Cycle 2466 shows the same guard failure after 2 corrections. Nothing reached main.
- **Symptom:** the review gate rejected the build after 1 correction. The floor text names `TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial` (`go/cmd/evolve/parallel_default_test.go:13`).
- **Root cause:** the checks that run the whole `./cmd/evolve` suite take 5 to 11 minutes. The builder does not wait for them. The last correction adds tests, and the floor then runs on the new tree with no correction left. The floor text also drops the lines that name the failing test, so a correction cannot fix it.
- **Fix (proposed, not built):** run the fast structure guards of each changed package first, in the static tier of the floor. Keep the detail lines of a failed test in the floor text.
- **Item:** `floor-runs-structure-guards-first-and-keeps-failure-detail` (draft).

## Terms used throughout

- **Floor:** the build handoff floor, `core.BuildHandoffFloor` (`go/internal/core/build_handoff_floor.go:18`). It is a list of deterministic checks on the tree of the builder.
- **Static tier:** the `Run` function of each floor check. **Test tier:** the `Tests` function, which runs the package tests.
- **Self-check:** the check that gives the builder the floor findings while its session is open (`builderSelfCheck.beforeClose`, `go/internal/core/builder_selfcheck.go:80`). The builder can also run it by hand with `evolve phase verify build`.
- **Receipt:** the file `builder-selfcheck-receipt.json` in the workspace. It holds the tree digest and a `green` flag. It is written only when a self-check run ends (`go/internal/cli/phasecmd/build_selfcheck.go:41`).
- **Correction:** a second dispatch of the builder after the review gate rejects its output. The policy `retry.contract_correction_retries` is 1 (`.evolve/policy.json:213`).

## Timeline

Cycle 2583 (`.evolve/runs/cycle-2583/signals.ndjson`):

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-21 08:35:11 | The build phase starts. | `build dispatched` |
| 09:14:26 | The self-check returns 1 finding into the session. It is an STE lint finding. The receipt state is `absent`. | `BUILD_SELFCHECK_FINDINGS`, `builder-selfcheck-findings.json` |
| 10:26:25 | The review gate rejects the first deliverable with 1 finding: `removeUntrackedCopiesOfOrigin` has 83.3% coverage (10 of 12 statements). Correction 1 starts. The package tests passed on this tree. | `ORCHESTRATOR_GATE_CORRECTION` |
| about 10:39 | The builder adds `cmd_syncmain_untracked_test.go` with two tests. It runs only `-run 'SyncMain\|RemoveUntracked'` (22.15 s). It starts `evolve phase verify build` in the background. | pane file `build-...-n7-...-final-pane.txt` |
| 10:48:01 | The session closes after the builder writes `## Correction 1`. The verify run has not ended. No self-check signal exists after 09:14:26. | `build-events.ndjson`, `signals.ndjson` |
| 11:29:26 and 11:56:56 | The final floor is RED twice on the guard. | `BUILD_FLOOR_RETAKE_RED` |
| 12:20:58 | The phase aborts. | `ORCHESTRATOR_PHASE_ABORTED` |

Cycle 2508 (`.evolve/runs/cycle-2508/signals.ndjson`): the self-check returned findings at 15:09:34, 15:58:31, 17:31:16 and 17:50:40. The last one came 52 seconds after the gate correction of 17:49:48. The guard was RED at 15:24:35, 15:32:02, 16:34:09, 17:41:02, 18:01:05 and 18:09:45. The phase aborted at 18:11:08.

The key text, from `audit-fail-reason.json` of cycle 2583 (trimmed to the first block):

```text
review gate: phase "build" deliverable rejected after 1 correction(s): build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff:
  the tests of the changed packages fail under -tags integration, so no changed function is proven:
    --- FAIL: TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial (0.37s)
FAIL	github.com/mickeyyaya/evolve-loop/go/cmd/evolve	663.896s
…6-11-02T00-00-00Z-item-a.json → processing/cycle-8/
FAIL
```

The text has no line that names a test or a file. `failure-decision.json` of cycle 2583 says that a rerun in the worktree printed the two tests `TestRemoveUntrackedCopiesOfOrigin_KeepsATrackedFile` and `TestRemoveUntrackedCopiesOfOrigin_StopsWhenTheDiffFails`. Both lack `t.Parallel()`.

From the findings records:

```text
cycle-2583: "decision": "delivered", "receipt": "absent", 1 finding (ste-lint)
cycle-2508: "decision": "delivered", "receipt": "stale",  1 finding (protected control-plane path)
```

Across the 45 files `builder-selfcheck-findings.json` of cycles 25xx: receipt `absent` in 26, `stale` in 12, `red` in 7. The decision is `delivered` in all 45.

## Root cause

**Symptom.** The final floor of the review gate found a failed guard test. The correction had no budget left.

**Chain.**

1. The guard `TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial` reads the source of the package. It reports each top-level test that does not call `t.Parallel()` first (`go/internal/paralleltest/paralleltest.go:139`). It runs as one test inside `go test ./cmd/evolve`. That whole run took 216 to 696 seconds in these cycles.
2. The first tree of cycle 2583 passed the guard. Correction 1 added two tests without `t.Parallel()`. The guard first failed on the tree of the correction.
3. The builder did not run the whole suite. It ran a filtered run, and its pane says: "I did not run the full suite". It started `evolve phase verify build`, which runs the floor. The session closed on the deliverable write while the run was still open. The receipt for the tree was written later, with `green: false` (tree `bb01897f…`).
4. The self-check of the bridge did not stop this tree. In cycle 2583 no self-check signal follows the correction. Why it did not run, or did not deliver, is unverified. The stderr of the orchestrator is not in the run records.
   - In cycle 2508 the self-check ran after each correction. There the floor returned one static finding (a protected path), and the test tier never ran.
   - `BuildHandoffFloor.Failures` returns the static findings first. It skips the test tier when any exist (`go/internal/core/build_handoff_floor.go:39-44`).
   - So a builder with one small static finding never sees a test failure. The same floor refuses that failure a few minutes later.
5. The review gate runs the floor again after the correction. It found the guard failure. With `contract_correction_retries` set to 1, no correction was left, and the cycle failed.
6. The text of that failure cannot help a correction.
   - `goTestFailureLines` keeps only the lines that start with `--- FAIL: ` or `FAIL` (`go/internal/core/build_floor_retake.go:172-182`).
   - `floorFailureDiagnostic` cuts the output to 400 bytes. It keeps those lines, then `…`, then the last bytes (`go/internal/core/build_floor_reviewer.go:191-206`).
   - The line that names the test and the file follows `--- FAIL:`. Its text is `<test> (<file>): does not call t.Parallel and is not listed as serial`.
   - That line is neither kept nor in the last bytes, so it is dropped. The last bytes are log noise from the inbox mover.

**Escape.** The guard is cheap. Alone it runs in about 13 seconds wall time on a warm build cache. The command was `go test -count=1 -run TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial ./cmd/evolve`, run on 2026-11-21. Test time was 0.89 seconds.

The floor does not run the guard alone. It runs the guard inside the 5 to 11 minute suite, and only as the test tier, which a static finding skips. The self-check that can warn the builder costs the same 5 to 11 minutes, so a builder in a hurry skips it. This is the likely reason for the 26 absent receipts. For cycle 2583 the evidence supports it. For the other 25 cycles it is unverified.

**What the records refute.** The console asked if `beforeClose` runs after a review-gate correction. It does. The correction dispatch reuses the request that holds `BeforeClose` (`go/internal/core/cyclerun_dispatch.go:87`, `go/internal/core/cyclerun_correction.go:177`). Cycle 2508 shows self-check signals after corrections.

So the claim that the last correction reaches the final floor with no self-check is too strong. It holds for cycle 2583 only, and there the cause is unverified. The claim that the correction text dropped the detail lines is confirmed.

**Chosen root cause.** The floor has no cheap tier for structure guards, and its failure text drops the lines that name the failed test. Both parts are needed. A cheap tier alone does not help if the text is unreadable. Readable text alone does not help if the failure appears only after the last correction.

## What changed

Nothing is built. The proposed change and its tests:

| Change | Regression test (must be red without the change) |
|---|---|
| In the static tier of the floor, run only the guard tests and report their full output, for each changed package with a registered guard. The command is `go test -count=1 -run '^TestParallelDefault_' <pkg>`. The self-check and the final floor then show it with the other static findings. Files: `go/internal/core/build_handoff_floor.go`, `go/internal/core/phase_bindings_selfcheck.go`. | A test through `BuildHandoffFloor.Failures`: a fixture worktree adds a test without `t.Parallel()` and carries a static finding too. The result must name the test. The full-suite runner stub must record 0 calls. |
| `goTestFailureLines` also keeps the indented lines that follow each `--- FAIL:` line, up to a cap, and `floorFailureDiagnostic` keeps them before the tail. Files: `go/internal/core/build_floor_retake.go`, `go/internal/core/build_floor_reviewer.go`. | A test in `go/internal/core/build_floor_retake_test.go`: output with a `--- FAIL:` line, the line `does not call t.Parallel and is not listed as serial`, and 20 KB of later noise. The diagnostic must hold the line and stay under its size cap. |
| Index rows for both failure modes, as ❌ until the item lands. | The row names the item. |

## What we learned

- A guard that reads source must not wait behind a 10 minute suite. Run it first, alone, and give its text in full.
- A failure text must name what failed. The lines `--- FAIL:` and `FAIL` alone do not.
- A check that skips the next tier when it finds a static finding hides the failures of that tier. The fast guards belong in the static tier.
- Open, unverified: why no self-check signal follows the correction of cycle 2583, and why 26 of 45 builders never finished a self-check. The item asks for a signal when the self-check is skipped or cut off.

Inbox item: `floor-runs-structure-guards-first-and-keeps-failure-detail`. No self-heal incident id and no PR exist yet.
