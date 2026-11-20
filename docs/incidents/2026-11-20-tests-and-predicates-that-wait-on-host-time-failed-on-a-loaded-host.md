# 2026-11-20: tests and predicates that wait on host time failed on a loaded host

Related documents: the retrospective of the cycle is `.evolve/runs/cycle-2560/retrospective-report.md`. The CI wall-time plan is `docs/plans/ci-wall-time-2026-10.md`. The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** two checks failed because they depend on host time. First, cycle 2560 failed in the build floor. Its ACS predicate `TestC2560_006` needed a macOS core test ratio of kernel time to user time below 1.2, and it measured 1.31. Second, main CI run 44781012877 on `b2884a911` failed in the macOS set on `TestRun_StopSignalEndsRunAndClosesFile` of `go/cmd/hostsampler`.
- **Root cause:** each check asserts a result that moves with host load. The ratio changes with load and with `-test.parallel`. The hostsampler test sends the stop signal after a sleep of 80 ms and expects a sample, so a slow runner receives the stop first.
- **Fix:** cycle 2560 went to the retry ladder and then to redesign. The git-start counter and its budget guard are salvaged in the item `core-tests-cut-git-starts-salvage-without-ratio-gate`. The hostsampler test is fixed by the item `hostsampler-stop-test-stops-after-a-counted-read`.
- **Lesson:** an acceptance check must assert counted work, for example the number of process starts. It must not assert a time or a time ratio on a shared host.

## Terms used throughout

- **Counted work:** a number that the code sets, such as process starts or reads. It does not change with host load.
- **Time ratio:** a measure such as kernel time over user time. It changes with load.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| wave 183, time not recorded | Audit round 1 of cycle 2560 measures a ratio of 2.16 for criterion 2, which Triage had not deferred. | `retrospective-report.md` of cycle 2560 (H1) |
| wave 183, time not recorded | The build floor rejects the repair build after 2 corrections. The ratio is 1.31 (user 98.86 s, system 129.60 s). The builder measured 1.27. | `.evolve/runs/cycle-2560/audit-fail-reason.json`, `builder-selfcheck-findings.json` |
| after 17:12 | Run 44781012877 fails in the macOS set. The next two main runs pass with no change to the package. The time is after the commit time of `b2884a911` (01:12 +08:00). | item `hostsampler-stop-test-stops-after-a-counted-read` |

The floor text of cycle 2560:

```text
--- FAIL: TestC2560_006_TheCoreTestBinaryOnMacOSSpendsLessThanOnePointTwoTimesItsUserTimeInTheKernel (86.19s)
    predicates_test.go:331: core system_s/user_s is 1.31 (user 98.86s, sys 129.60s), want below 1.2
```

The hostsampler failure (`go/cmd/hostsampler/main_test.go:83`):

```text
no samples
```

## Root cause

**F1.1** The item copied criterion 2 as an acceptance criterion. The builder data shows about 2 kernel seconds per user second for each spawn. Fewer spawns leave the ratio near the same value. The ratio also moves with `-test.parallel`: 1.38 at 3 and 1.27 at 2 (retrospective of cycle 2560).
*Implication:* the fix surface cannot move the ratio. No phase checked this.

**F1.2** TDD made the ratio a gating ACS predicate. The build floor then rejected work that was otherwise green: the git starts fell from 28256 to 15638.
*Implication:* a gate over a target that cannot be reached is a guaranteed rejection. The failure decision is `intent-malformed`.

**F1.3** The test `TestRun_StopSignalEndsRunAndClosesFile` (`go/cmd/hostsampler/main_test.go:72`) starts a goroutine that sleeps 80 ms and sends SIGTERM. It expects at least one sample at a 10 ms interval.
*Implication:* under `-race` on a loaded runner, the run gets the stop before it writes a sample.

**F1.4** The two failures have one lesson: each check synchronizes on host time and not on work that the code did.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| Salvage the git-start counter and the budget guard from the worktree of cycle 2560. Drop `TestC2560_006`. Defer the ratio with the per-spawn measurement as the reason. | inbox item `core-tests-cut-git-starts-salvage-without-ratio-gate` | owed by the item |
| Triage defers a host-timing ratio criterion that the fix surface cannot move. TDD pins the counted proxy. | inbox item `triage-defer-unreachable-host-timing-ratio-criteria` (named in the retrospective; filing not verified) | owed by the item |
| The stop depends on a counted read, not on a sleep. | inbox item `hostsampler-stop-test-stops-after-a-counted-read` | owed by the item: the test has no `time.Sleep`, and `go test -race -count=50 ./cmd/hostsampler/ -run TestRun_StopSignalEndsRunAndClosesFile` passes |
| The CI plan item for macOS CPU and spawn counts | inbox item `ci-wall-time-c13-macos-cpu-and-spawn-counts` | open |

## Limits

- The host load of the failed runs is not measured in this record. The retrospective records a load of about 25 in the builder data.
- The link of the C13 item to cycle 2560 is not verified. The retrospective names the item `core-tests-cut-git-starts`.

## What we learned

- Write an acceptance criterion about counted work, such as process starts.
- Before a criterion gates a build, probe that the fix surface can move it.
