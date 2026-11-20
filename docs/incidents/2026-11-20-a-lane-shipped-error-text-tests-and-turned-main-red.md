# 2026-11-20: a lane shipped error-text tests and turned main red

Related documents: the first incident of this class is [2026-11-19: four lanes turned main red with test-audit findings](2026-11-19-two-lanes-turned-main-red-with-error-text-tests.md). The test-audit rules are in `go/internal/testaudit/rules.go`, the build floor is in `go/internal/core/build_floor_reviewer.go`, and the procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** cycle 2554 shipped `cd9a257ff` to main at 01:39 UTC. The required CI of that commit and of the two commits after it failed. The boundary batch of wave 182 did not start, because a batch needs a complete and green main CI.
- **Root cause:** the cycle added three tests in `internal/stageevidence` that match error text. The test-audit tree ratchet refuses a new finding that has no baseline row. The lane build floor does not run that ratchet, so the lane did not see the finding before it shipped.
- **Fix:** PR #1586 types the two errors (`ErrTrailingData` and `ErrMissingPackage`), and the tests use `errors.Is`. The PR lands alone through `evolve integration batch --main-red-fix`.
- **Lesson:** a class fix that waits in the inbox lets the same incident come back. A recurrence of an open class-fix item raises that item to the first build of the next wave.

## Terms used throughout

- **Test-audit tree ratchet:** `TestTreeRatchet` in `go/internal/testaudit`. It scans each test for weak patterns and fails on a finding that has no baseline row.
- **Error-text-match:** a test that compares the text of `err.Error()` with a string literal, or with a string matcher such as `strings.Contains`.
- **Build floor:** the checks that a lane build runs before the audit, listed in `DefaultBuildFloorChecks`.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 01:39 | Cycle 2554 (`shadow-contract-k3-stage-verify`) ships `cd9a257ff`. | `git log` of `origin/main` |
| 02:11 | The required CI of `cd9a257ff` fails. The CI runs of `fcf8402ea` and `79f39ba00` fail the same way. | run 44660490892 and the two runs before it |
| 03:19 | Wave 182 ends. | `wave watch` exit |
| 03:21 | The console reads the failed steps of run 44660490892 before the batch. | `gh run view --log-failed` |
| 03:21 to 03:25 | `TestTreeRatchet` fails on a clean `origin/main` worktree with the same three findings. | local run |
| 03:26 | Reviews pass, and the fix ships as `62515644a` in PR #1586. | `agent-code-reviewer-20261120T032508Z-ac1a`, `agent-code-simplifier-20261120T032621Z-2ad2` |

The failure in the CI log:

```text
ratchet_tree_test.go:22: testaudit: internal/stageevidence TestAPICover_NamesAllStageEvidenceExports error-text-match has no baseline row
    testaudit: internal/stageevidence TestDecisionVersion_EdgeAndErrorBranches error-text-match has no baseline row
    testaudit: internal/stageevidence TestHashListedPackages_ErrorBranches error-text-match has no baseline row
--- FAIL: TestTreeRatchet (57.22s)
```

## Root cause

**F1.1** Two tests in `stageevidence_coverfix_test.go` called `strings.Contains(err.Error(), ...)`. The API-cover test compared `broken.Error()` with `""`. Two of the errors were plain text from `errors.New` and `fmt.Errorf`, with no sentinel.
*Implication:* the tests told the two causes apart only by the text of the message.

**F1.2** `Rebaseline` refuses new rows, even when the count does not grow.
*Implication:* the only correct fix is to remove the smell. A new baseline row is not an option.

**F2.1** The lane build floor does not run the test-audit ratchet. This is a recurrence: the 2026-11-19 incident found the same escape, and its class-fix item `build-floor-runs-the-test-audit-ratchet` is open since 2026-11-19 10:32 UTC. No lane took the item in waves 181 and 182. The item changes `build_floor_reviewer.go`, and PR #1583 changes the same file.
*Implication:* main CI was the first check to run the ratchet on the change. Each wave until the item lands can turn main red again, so the console builds the item as soon as PR #1583 lands.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The two errors wrap the sentinels `ErrTrailingData` and `ErrMissingPackage`. The tests use `errors.Is`. The API-cover test checks that two broken-chain positions give two different messages. | #1586 | `testaudit/ratchet_tree_test.go::TestTreeRatchet` (red on `origin/main`, green on the fix) |
| The build floor runs the test-audit ratchet on the changed packages | inbox item `build-floor-runs-the-test-audit-ratchet` (console PR) | `core/build_testaudit_check_test.go::TestStaticBuildFloorChecks_RefusesANewTestAuditFinding` |

## Limits

- Until the floor item lands, a lane can ship a new test-audit finding again.
- The three red main runs stay red in the history. The first green run is the run of the main-red-fix landing.

## What we learned

- Before a batch, read the failed step names and the package FAIL lines. Most FAIL lines in this log were fixture text that repeats in each job.
- Prove a ratchet red on a clean `origin/main` worktree before you trust the fix.
