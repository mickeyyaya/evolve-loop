# 2026-11-20: salvage bundles filled the disk and hung a loop test

Related documents: the procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** in wave 182, cycles 2547, 2548, 2550 and 2551 failed. Each audit verdict was PASS, but each ACS verdict had 40 reds, and the orchestrator classified each cycle as `infra-systemic`. The console engaged the loop brake.
- **Root cause:** `.evolve/operator-salvage` held 254 leaves and 187 GB, so the host disk had 2.6% free. The disk guard pauses a new dispatch when free disk is under 5%. A unit test in `cmd/evolve` calls the loop with the production disk guard, so it waited without end. The ACS Go lane spent its 10-minute budget on that one test.
- **Fix:** the console trimmed the Go build cache and deleted 48 salvage leaves whose tip is on `origin/main`. Free disk went from 12 GB to 73 GB, and the test passed in 0.96 s. Two inbox items hold the class fixes.
- **Lesson:** a salvage store with no retention is a disk leak. A unit test that reads the host disk fails on a full host, and the failure looks like a timeout, not like a disk fault.

## Terms used throughout

- **Salvage leaf:** a directory under `.evolve/operator-salvage/` that `evolve gc` writes before it removes a worktree. It holds the tip, a patch, the untracked files and a git bundle.
- **Disk guard:** `looppreflight.DiskGuard`. It pauses a new cycle while free disk is under the pause line (5%).
- **ACS Go lane:** the part of the acceptance suite that runs the Go predicate scopes in order, inside one shared budget (`acs.go_timeout_s`, 10 minutes).

## Timeline

| Time (UTC, 2026-11-19) | Event | Evidence |
|---|---|---|
| 21:39 | Wave 182 starts with 8 lanes. | `wave: launched wave 182` |
| 21:54 | The ACS Go lane of cycle 2551 starts `TestC1270_003`. The test does not finish. | `acs-red-evidence` of cycle 2551 |
| 23:05 | The audit runs the suite a second time. The same test does not finish, and the budget expires again. | `acs-red-evidence` of cycle 2551, second run |
| 23:26 | Cycle 2551 fails: 40 reds, all `go-lane-scope-failed` or `go-lane-incomplete`. Cycle 2550 has the same 40 reds. | `ORCHESTRATOR_SYSTEM_FAILURE`, cycle 2551 |
| 23:27 | Cycle 2547 fails with the same reds. | `ORCHESTRATOR_CYCLE_FAILED`, cycle 2547 |
| 23:30 | The console engages the loop brake after the third failure. | `evolve loop-stop` |
| 23:36 | A goroutine dump of the test on main shows the wait in `looppreflight.waitForInterval`. Free disk is 2.6%. | test run with `-timeout 60s` |
| 23:38 | `evolve gc --only cache` trims 12.5 GB. Free disk is 23 GB, at the 5% line. | gc output |
| 23:39 | Cycle 2548 fails with the same reds. | `ORCHESTRATOR_CYCLE_FAILED`, cycle 2548 |
| 23:41 | The console deletes 48 salvage leaves. Free disk is 73 GB. The test passes in 0.96 s. | `backups/operator-salvage-index-2026-11-20.json` |
| 23:42 | The console files the two class-fix items. | `inbox add` output |

The red evidence of cycle 2551:

```text
predicate scope ./acs/regression/cycle1270 produced no complete evidence: predicate execution omitted declared tests .../TestC1270_003_CmdEvolveNoLongerPaysTheRetryLadder, ...: signal: terminated
incomplete predicate inventory; the Go lane's shared 10m0s budget (acs.go_timeout_s) expired while this scope ran: context deadline exceeded
```

The goroutine dump of the test on main:

```text
goroutine 54 [select]:
github.com/mickeyyaya/evolve-loop/go/internal/looppreflight.waitForInterval(...)
	go/internal/looppreflight/cyclestart.go:51
github.com/mickeyyaya/evolve-loop/go/internal/looppreflight.RunCycleStart(...)
	go/internal/looppreflight/cyclestart.go:37
github.com/mickeyyaya/evolve-loop/go/cmd/evolve.(*loopBatchCoordinator).dispatchSequentialCycle(...)
	go/cmd/evolve/cmd_loop_sequential_dispatch.go:26
github.com/mickeyyaya/evolve-loop/go/cmd/evolve.TestLoop_MaxCyclesExit_ClearsCompletedMarker(...)
	go/cmd/evolve/cmd_loop_finalize_test.go:42
```

## Root cause

**F1.1** `writeSalvageLeaf` (`go/internal/gc/salvage.go`) runs `git bundle create <file> <ref>` with no negative base. Each bundle holds the full history of the repository, about 1.1 GB.
*Implication:* each removed worktree costs 1.1 GB of disk, even when its tip is on main.

**F1.2** No sweep removes a salvage leaf. `evolve gc` writes a leaf before each worktree removal, and the leaves added up from 2026-11-13 to 2026-11-20.
*Implication:* the store grew to 254 leaves and 187 GB, and the host disk had 12 GB free.

**F2.1** `TestLoop_MaxCyclesExit_ClearsCompletedMarker` calls `runLoop`. The loop calls `looppreflight.RunCycleStart` with the production disk guard, which reads the host disk.
*Implication:* on a host under 5% free disk, the test waits in 15-second steps without end.

**F2.2** The ACS predicate `TestC1270_003` runs that test in a nested `go test` with no timeout of its own. `acsassert.SubprocessOutput` (`go/pkg/acsassert/assertions.go`) starts the command with `context.Background()`, and the call passes no `-timeout` flag. The Go lane runs its scopes in order, inside one 10-minute budget.
*Implication:* one hung test spent the whole budget. Each later scope was "not run", which gave 40 reds.

**F2.3** The failure classifier found the shared budget and the verify-lock waits, but not the disk. The disk guard writes its pause to stderr of the nested test, and the red evidence does not show that line.
*Implication:* the signal said `infra-systemic` with no disk cause. A goroutine dump found the cause.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The console trimmed the Go build cache to its 20 GB cap. It deleted 48 salvage leaves: each tip is an ancestor of `origin/main`, and each leaf holds no patch and no untracked files. The index of all 254 leaves is in `backups/operator-salvage-index-2026-11-20.json`. | none: an operator action | none: a one-time recovery |
| A salvage leaf bundles only the commits that no kept ref holds, and gc removes a leaf whose tip is on main | inbox item `gc-salvage-bundles-hold-the-whole-history-with-no-retention` | open |
| The loop unit tests use a stub disk guard, and a disk-guard pause emits a signal with the free percent | inbox item `loop-tests-read-the-real-host-disk-through-the-disk-guard` | open |

## Limits

- 206 salvage leaves stay, about 135 GB. A leaf whose tip is not on main can hold work that no other ref holds, so only the class fix may remove it.
- Free disk is 73 GB. Each new worktree removal still adds a 1.1 GB leaf until the class fix lands.
- Cycles 2546, 2549, 2550, 2552 and 2554 ran some of their gates while the disk was full. Their results can still show the same reds.

## What we learned

- When many cycles fail with the same reds and a timeout, run `df` first.
- A store that a sweep writes needs a retention rule in the same change.
- A unit test reads no host resource that a guard checks. It gets a stub through the test dependencies.
