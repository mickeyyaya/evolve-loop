# 2026-11-18: faketmux tests need a ps binary that a lane cell denies

Related documents: the code is `go/internal/faketmux/session.go` and `go/internal/faketmux/format.go`. The red main that cycle 2513 tried to fix is in [the ship backstop record](2026-11-18-a-new-tmux-test-turned-main-red-and-the-ship-backstop-did-not-see-it.md).

## Summary

- **Impact:** cycle 2513 built the right fix (a 12-line `TestMain` in `internal/faketmux`) and failed audit with verdict FAIL. 8 faketmux tests fail in every lane cell, so no lane can verify a change to faketmux.
- **Root cause:** `commandOf` (`go/internal/faketmux/session.go:31-32`) runs the external `ps` binary. A lane cell denies it: `fork/exec /bin/ps: operation not permitted`. `go/internal/faketmux/format.go:80` also runs `ps`.
- **Escape:** the acceptance of the item said that `go test -count=1 ./internal/faketmux` must pass. Nobody had run that package suite in a cell before the item was planned. Cycle 2509 had already seen the same 8 failures and filed them as a LOW issue (L3).
- **Fix:** inbox item `faketmux-reaper-needs-no-ps-binary` (open). Commit `c6c8eaedb` added `faketmux/main_test.go`, which is the TestMain fix of the other record. It does not remove the `ps` dependency.
- **Lesson:** a fake must not depend on a host binary that a cell denies. A red predicate that comes from the cell, not from the change, blocks a ship under EGPS.

## Terms used throughout

- **Lane cell:** the sandbox in which a lane agent and its test runs execute.
- **Reaper:** the code in faketmux that finds the process of a pane and ends the panes of a killed test run.
- **EGPS:** the verdict rule that a red predicate blocks the ship.

## Timeline

| Time (UTC, 2026-11-18) | Event | Evidence |
|---|---|---|
| before 15:57:45 | The audit of cycle 2509 (which ships at 15:57:45) lists L3: 8 faketmux process tests fail with `ps` and `kill` denied. The auditor did not run the base commit. | cycle 2509 audit report |
| 17:24:05 | Cycle 2513 builds `go/internal/faketmux/main_test.go` (12 lines). | cycle 2513 signals |
| 17:30:17 | Audit verdict FAIL. ACS predicate `TestC2513_002` is red: 8 tests fail with `fork/exec /bin/ps: operation not permitted`. | audit report, signal `ORCHESTRATOR_PHASE_VERDICT_FAIL` |
| 17:30:17 | `ORCHESTRATOR_AUDIT_REPAIR_DECLINED`: the declared class `infra-systemic` earns no repair round. | signal |
| 17:31:10 | `ORCHESTRATOR_CYCLE_FAILED`. | signal |
| 18:15:34 | The inbox item `faketmux-reaper-needs-no-ps-binary` routes to the console. Lane triage of cycle 2517 dropped it for a protected surface (see [the bounce record](2026-11-18-two-items-bounced-off-the-protected-surface-in-cycles-2516-and-2517.md)). | signal `INBOX_ITEM_ROUTED_CONSOLE` |

## Key logs

The audit of 2513, trimmed:

```text
FAIL .../internal/faketmux 11.617s
8 tests failed: TestSeams_InjectedFailuresSurface,
TestSeams_TheFakeNeverSignalsItselfItsParentOrANonShellLeader,
TestSeams_ClaimAndStartFailLoudlyWhenAStepFails,
TestSeams_TheFakeSignalsOnlyALiveGroupLeaderThatIsNotItsOwnGroup,
TestRun_DisplayMessageNamesTheForegroundProgramOfThePane,
TestRun_KillSessionEndsThePaneAndTheSessionIsGone,
TestClaim_TheExportedPairClaimsAndReapsARunOfTheTempBase,
TestClaimIn_ReapsOnlyTheMarkedRunsWhoseOwnerIsGone
error: fork/exec /bin/ps: operation not permitted
```

The failure dossier of 2513:

```json
{"cycle_id":2513,"recorded_verdict":"FAIL",
 "audit_declared":{"class":"infrastructure-systemic",
  "defects":["H1: ACS TestC2513_002 red - go test ./internal/faketmux fails 8 tests with fork/exec /bin/ps: operation not permitted; identical failures without the change (pre-existing host restriction, not a code defect)"]}}
```

## Root cause

**F2.1** `commandOf` runs `ps -o comm= -p <pid>` through `sysexec.Output` (`go/internal/faketmux/session.go:32`). `runPS` runs `ps -A -o pid= -o ppid= -o comm=` (`go/internal/faketmux/format.go:80`).
*Implication:* any faketmux code path that names the command of a process, and any listing of processes, needs the `ps` binary.

**F2.2** The audit ran the 8 failing tests and then ran the package again with `main_test.go` removed through `go test -overlay`. The same 8 tests failed with the same error. The build report says that the host does not let any process run `/bin/ps`, also outside the sandbox (build-report.md:27-28, as quoted by the audit).
*Implication:* the 12-line change causes none of the failures. The auditor judged the change correct (confidence 0.85) and still gave no PASS.

**F2.3** Acceptance criterion 2 of cycle 2513 was `go test -count=1 ./internal/faketmux` passes. The ACS predicate `TestC2513_002` runs that command. Under EGPS a red predicate blocks the ship.
*Implication:* a correct change did not ship, because the acceptance asked for a package suite that the cell cannot run. The audit says to keep predicate 002 as it is.

**F2.4** The audit of cycle 2509 found the same 8 failures one cycle before. It recorded them as L3 (LOW) and said the cause was the environment, with moderate confidence, because the base commit was not run.
*Implication:* the signal was in the evidence of an earlier cycle before 2513 failed on it. The LOW finding did not become a work item that blocks planning of faketmux tasks.

## What changed

| Change | Pull request or commit | Regression test |
|---|---|---|
| The reaper and the process listing find a process command with no `ps` binary (the build report of 2513 proposes a sysctl or proctree lookup) | inbox item `faketmux-reaper-needs-no-ps-binary` | open |
| `internal/faketmux` gets its `TestMain` | commit `c6c8eaedb` | `TestEveryTmuxTestPackageUsesTheOrphanGuard` |

## Limits

- The audit quotes the build report on the `ps` denial outside a lane cell. The evidence folder does not hold the build report.
- This record does not say how commit `c6c8eaedb` passed its ship with the 8 tests red in a cell. The evidence does not cover it.
- The inbox item is in the runtime inbox, not in git. Its open state is as the request for this record gives it.
- The 8 test names come from the audit report of 2513. The list of 2509 gives the count only.

## What we learned

- Run the package suite of a package inside a cell before a plan makes it an acceptance item.
- A fake that a lane uses to verify a change must not depend on a host binary. Use `proctree` or a system call.
- A LOW finding that blocks every later task of a package is not LOW. Open an item for it at once.
- Do not relax a predicate to pass a ship. Fix the dependency.
