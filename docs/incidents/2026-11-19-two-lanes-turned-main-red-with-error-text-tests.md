# 2026-11-19: two lanes turned main red with error-text tests

Related documents: the test-audit rules are in `go/internal/testaudit/rules.go`, the build floor is in `go/internal/core/build_floor_reviewer.go`, and the procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** the required CI of main was red from 08:01 UTC in wave 181. Two red runs came from two lane cycles. A red main holds the boundary batch of 3 green console PRs (#1570, #1561, #1564).
- **Root cause:** cycles 2530 and 2533 each added a test that matches on the text of `err.Error()`. The test-audit rule `error-text-match` refuses such a test when its package has no baseline row. The baseline only shrinks, so a new finding is a red tree ratchet.
- **Escape:** the lane build floor does not run the test-audit ratchet. It runs the tests of the changed packages, and `internal/testaudit` is not one of them. Only the CI of main ran `TestTreeRatchet`.
- **Fix:** PR #1572 gives `incidentkb` two sentinel errors and `kbgraph` a typed `RecordParseError`. Three tests now check these with `errors.Is` and `errors.As`: the two red tests and one older `kbgraph` test.
- **Class fix:** the inbox item `build-floor-runs-the-test-audit-ratchet` adds the ratchet to the build floor.
- **Lesson:** a whole-tree ratchet that only CI runs lets each lane turn main red. The floor must run each such ratchet on the changed packages.

## Terms used throughout

- **Tree ratchet:** `TestTreeRatchet` in `go/internal/testaudit`. It runs the test-audit rules on the whole module and compares the findings with the baseline rows.
- **Error-text match:** a test that calls `strings.Contains` (or `==`) on `err.Error()` and a string literal.
- **Build floor:** the deterministic checks that a lane build must pass before the handoff to audit.

## Timeline

| Time (UTC, 2026-11-19) | Event | Evidence |
|---|---|---|
| 06:09 | Main CI on `b32f7063f` is green. | required CI run on `b32f7063f` |
| 08:01 | Cycle 2530 lands `234383500`. Main CI fails in `internal/testaudit`. | run 44484192548 |
| 09:39 | Cycle 2533 lands `05937be92`. Main CI fails again, with a second finding. | run 44498432850 |
| 10:22 | The console finds the red during the boundary check, and reads the failed steps. | `gh run view --log-failed` |
| 10:32 | The console files the class-fix item. | `2026-11-19T10-32-23Z-build-floor-runs-the-test-audit-ratchet.json` |
| 10:38 | PR #1572 ships at `180c8e017`, and the train records it. | `deliverable-pr-1572.json` |

Key log of run 44498432850 (the two other `FAIL` package lines in the log are fixture text that other tests print):

```text
ratchet_tree_test.go:22: testaudit: acs/cycle2533 TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph error-text-match has no baseline row
--- FAIL: TestTreeRatchet (56.83s)
FAIL	github.com/mickeyyaya/evolve-loop/go/internal/testaudit	56.952s
```

Key log of run 44484192548:

```text
ratchet_tree_test.go:22: testaudit: internal/incidentkb TestRender_EmptyAndErrorBranches error-text-match has no baseline row
--- FAIL: TestTreeRatchet (55.56s)
```

## Root cause

**F1.1** `TestRender_EmptyAndErrorBranches` checked `strings.Contains(err.Error(), "end marker")` and `"overlapping"`. `TestC2533_004` checked `strings.Contains(err.Error(), recordID)`.
*Implication:* each test is a new `error-text-match` finding with no baseline row.

**F1.2** `Rebaseline` (`go/internal/testaudit/tree.go:21`) refuses a new row.
*Implication:* a lane cannot accept the finding, and the console cannot either. The tests must change.

**F1.3** On the fix branch, the tree ratchet also flagged the older test `TestBuild_UnparsableIncidentReturnsErrorNamingFile`, which uses the same text match. The CI of main did not flag it.
*Implication:* the fix changes that test too. The cause of the difference is open (see Limits).

**F2.1** `StaticBuildFloorChecks` and `SuiteBuildFloorChecks` (`go/internal/core/build_floor_reviewer.go:35-51`) have no test-audit check.
*Implication:* the lane floor passed both cycles. The first check that saw the finding was the CI of main, after the landing.

**F2.2** The log of each red run has more `FAIL` lines than the one real failure. Lines for `acs/cycle1946`, `acs/cycle1950` and `TestPersonaStopCriterionDedupe_*` are fixture text inside build-floor test output.
*Implication:* the console found the real failure by the `FAIL` line of a package with its own test name above it.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `incidentkb` wraps `ErrMissingMarker` and `ErrOverlappingMarkers`; `kbgraph` returns `*RecordParseError` | #1572 | `internal/testaudit/ratchet_tree_test.go::TestTreeRatchet` (red on `05937be92`, green on the fix) + `incidentkb/incidentkb_test.go::TestRender_EmptyAndErrorBranches` + `acs/cycle2533/predicates_test.go::TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph` + `kbgraph/kbgraph_test.go::TestBuild_UnparsableIncidentReturnsErrorNamingFile` |
| The build floor runs the test-audit ratchet on the changed packages | inbox item `build-floor-runs-the-test-audit-ratchet` | open |

## Limits

- PR #1572 is open. It lands at the boundary of wave 181 through `evolve integration batch --main-red-fix`. This record and its index row name `ErrMissingMarker`, `ErrOverlappingMarkers` and `RecordParseError`. These symbols are on main only after #1572 lands. Thus this record lands in the same batch as #1572, or after it.
- This record does not show why the CI of main did not flag `TestBuild_UnparsableIncidentReturnsErrorNamingFile` (F1.3). The rule has no gate on typed errors, and the test has no baseline row and no guard row.
- Lanes that land before #1572 can add more findings. The boundary check reruns the tree ratchet on the composed tree.
- The inbox item is in the runtime inbox, not in git.

## What we learned

- A ratchet that only CI runs on the whole tree lets a lane turn main red. Each whole-tree ratchet needs a floor check on the changed packages, as the persona-budget check has.
- A test checks an error by its identity (`errors.Is`) or its fields (`errors.As`), never by its text.
- Read the `FAIL` line of a package and the test name above it. Many `FAIL` lines in a CI log are fixture text.
