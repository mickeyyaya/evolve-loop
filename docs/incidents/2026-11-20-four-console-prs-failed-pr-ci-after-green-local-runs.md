# 2026-11-20: four console PRs failed PR CI after green local runs

Related documents: the whole-tree ratchets of the console ship are in the memory rule `whole_tree_ratchets_before_ship`. The same escape for lanes is in [2026-11-20: cycle 2558 turned main red a third time](2026-11-20-cycle-2558-turned-main-red-a-third-time.md). The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** the 4 console PRs of the wave 183 boundary were red in PR CI, although each had a green local run. Each also had 2 review seats and a commit-gate pass. The PRs are #1597 (build floor runs the test-audit ratchet), #1598 (host-adaptive window), #1599 (disk guard) and #1601 (TDD floor). Each needed a second commit, a second review and a new record.
- **Root cause:** the pre-ship check set of the console did not run each whole-tree gate that CI runs. The hand-written list `repocontract.Packages()` has 15 packages. It has no `testaudit`, `gateregistry` or `sysexec`.
- **Fix:** 4 fix commits, one for each PR. The class fix is the inbox item `ship-backstop-runs-the-test-audit-ratchet`, which the console widened at the boundary of wave 183.
- **Lesson:** a gate that only CI runs fails after the review. The set of gates that the ship backstop runs must be generated from the CI set, not typed by hand.

## Terms used throughout

- **Whole-tree gate:** a test or tool that scans the whole tree, not the changed package. Examples are `apicover`, `gateregistry` and the `sysexec` guard.
- **Ship backstop:** the checks that `evolve ship` runs before it commits. `repocontract.Packages()` lists its packages.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-20, about 14:50 | The console finds the 4 PRs red in PR CI. | console note |
| 2026-11-20, about 15:21 to 15:35 | Four fix commits pass 2 review seats each and re-record. | commit bodies of `81b90b816`, `783562718`, `87cdef501`, `c1fc68cad` |
| 2026-11-20, about 16:30 | All 4 PR heads are green (14 jobs pass and 1 is skipped for each). | console note |

## Failures

| PR | Failed gate | What the gate found | Fix commit |
|---|---|---|---|
| #1597 | step `api-coverage enforce` | the exported method `testaudit.RowError.Unwrap` was named by no test | `81b90b816` |
| #1599 | step `api-coverage enforce` | the exported type `looppreflight.DiskStatus` was named by no test | `783562718` |
| #1598 | `gateregistry` `TestNoStringTypedStageField` and `TestEveryStageKeyHasRegistryEntry` | the policy key `fleet.orchestrator.window.adaptive.stage` was a plain string with no registry seed | `87cdef501` |
| #1598 | `acs/regression/apicover` | the new package `internal/hostpressure` was not in `go/.apicover-enforce` | `87cdef501` |
| #1601 | `TestPipelineCode_StartsEveryProcessThroughCommand` | `pkg/acsassert/predicate_kit.go` started a process with `exec.Command` | `c1fc68cad` |

## Root cause

**F1.1** `repocontract.Packages()` (`go/internal/repocontract/repocontract.go:21`) lists the packages that the ship backstop tests. It has `rawgitratchet` and `sizeratchet` but not `testaudit`, `gateregistry` or `sysexec`.
*Implication:* the three gates that failed here ran in CI only.

**F1.2** The `apicover` gate counts a type only when the body of a test names it (console note of 2026-11-20).
*Implication:* a local coverage run does not prove the `apicover` count.

**F1.3** The review seats read the diff. They did not run the gates that CI runs. Their PASS does not cover a whole-tree gate.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `TestRowError_Unwrap` names `RowError.Unwrap` | #1597 | `testaudit::TestRowError_Unwrap` |
| `TestDiskStatus_MeasureReportsPausedFreePctThresholdAndPath` names `DiskStatus` | #1599 | the test of that name in `looppreflight` |
| A named `AdaptiveStage` type, a registry seed, and the package in `go/.apicover-enforce` | #1598 | `policy::TestAdaptiveStage_ShadowAndEnforceAreTheOnlyAcceptedValues`; the `gateregistry` tests above |
| `ExitedPID` starts its process through `sysexec.Command` | #1601 | the `sysexec` guard above; `acsassert::TestExitedPID_ReturnsTheIDOfAProcessThatHasExited` |
| The ship backstop runs the whole-tree gates of CI. `repocontract.Packages()` gets `testaudit`, `gateregistry` and `sysexec`, and `apicover-enforce` runs on changed packages. | inbox item `ship-backstop-runs-the-test-audit-ratchet` | owed by the item |
| One typed stage vocabulary for the fleet stage keys | inbox item `fleet-stage-keys-share-one-typed-stage-vocabulary` (architecture finding, MEDIUM) | owed by the item |

## Limits

- The PRs #1597, #1598, #1599 and #1601 landed through batch PR #1604 at main 30915d28c.
- The widening of the item is a console action at the boundary. Its landing is not verified here.

## What we learned

- A green local run of the changed packages does not prove the whole tree.
- Generate the backstop list from the CI list. A hand-written list of 15 packages was already stale.
