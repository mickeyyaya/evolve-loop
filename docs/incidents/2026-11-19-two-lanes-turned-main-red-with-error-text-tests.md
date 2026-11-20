# 2026-11-19: four lanes turned main red with test-audit findings

Related documents: the test-audit rules are in `go/internal/testaudit/rules.go`, the build floor is in `go/internal/core/build_floor_reviewer.go`, and the procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** the required CI of main was red from 08:01 UTC in wave 181. Four lane cycles added the findings. A red main holds the boundary batch of the green console PRs (#1570, #1561, #1564).
- **Root cause:** cycle 2530 added one test, and cycles 2533 and 2540 each added two tests, that match on the text of `err.Error()`. The test-audit rule `error-text-match` refuses such a test when its package has no baseline row. Cycle 2542 added a test that installs a fake `evolve` binary through a variable, which the rule `unverified-fake` refuses. The baseline only shrinks, so a new finding is a red tree ratchet.
- **Escape:** the lane build floor does not run the test-audit ratchet. It runs the tests of the changed packages, and `internal/testaudit` is not one of them. Only the CI of main ran `TestTreeRatchet`.
- **Fix:** PR #1577 gives `incidentkb` two sentinel errors, `kbgraph` a typed `RecordParseError` and `policy` the sentinel `ErrDeadlineBasis`. The five error-text tests now check these with `errors.Is` and `errors.As`. The cycle-start test builds its fake path at the install call, and `testaudit/fakes.json` gets an `evolve` row. PR #1577 replaces PR #1575 (five findings), which replaced PR #1572 (three findings).
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
| 09:39 | Cycle 2533 lands `05937be92`. Main CI fails again, with two more findings. | run 44498432850 |
| 10:22 | The console finds the red during the boundary check, and reads the failed steps. | `gh run view --log-failed` |
| 10:32 | The console files the class-fix item. | `2026-11-19T10-32-23Z-build-floor-runs-the-test-audit-ratchet.json` |
| 10:38 | PR #1572 ships at `180c8e017`, and the train records it. | `deliverable-pr-1572.json` |
| 15:44 | Cycle 2540 lands `e919bfd1d`. Main CI fails with two more findings, five in all. | run 44558284990 |
| 16:49 | PR #1575 rebuilds #1572 on main, fixes all five findings, and replaces #1572. | `deliverable-pr-1575.json` |
| 17:12 | Cycle 2542 lands `48a892d1d`. Main CI fails with one more finding, six in all. | run 44574746432 |
| 19:11 | At the boundary, the console reruns the ratchet on main `3a466367d` and reads the sixth finding, which main CI reported at 17:12. PR #1577 rebuilds #1575 on that main, fixes all six findings, and replaces #1575. | `deliverable-pr-1577.json` |

Key log of run 44574746432:

```text
ratchet_tree_test.go:22: testaudit: acs/cycle2533 TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph error-text-match has no baseline row
    testaudit: acs/cycle2540 TestC2540_006_TheDeadlineBasisRefusesAnyOtherValue error-text-match has no baseline row
    testaudit: cmd/evolve TestCycleStartTrim_LaunchFnRunsFakeTrimAndGuardsDisk unverified-fake has no baseline row
    testaudit: internal/incidentkb TestRender_EmptyAndErrorBranches error-text-match has no baseline row
    testaudit: internal/kbgraph TestBuild_UnparsableIncidentReturnsErrorNamingFile error-text-match has no baseline row
    testaudit: internal/policy TestParse_RefusesADeadlineBasisOtherThanAllOrShipped error-text-match has no baseline row
--- FAIL: TestTreeRatchet (74.14s)
```

Key log of run 44558284990:

```text
ratchet_tree_test.go:22: testaudit: acs/cycle2533 TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph error-text-match has no baseline row
    testaudit: acs/cycle2540 TestC2540_006_TheDeadlineBasisRefusesAnyOtherValue error-text-match has no baseline row
    testaudit: internal/incidentkb TestRender_EmptyAndErrorBranches error-text-match has no baseline row
    testaudit: internal/kbgraph TestBuild_UnparsableIncidentReturnsErrorNamingFile error-text-match has no baseline row
    testaudit: internal/policy TestParse_RefusesADeadlineBasisOtherThanAllOrShipped error-text-match has no baseline row
--- FAIL: TestTreeRatchet (79.84s)
```

Key log of run 44498432850 (the two other `FAIL` package lines in the log are fixture text that other tests print):

```text
ratchet_tree_test.go:22: testaudit: acs/cycle2533 TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph error-text-match has no baseline row
    testaudit: internal/incidentkb TestRender_EmptyAndErrorBranches error-text-match has no baseline row
    testaudit: internal/kbgraph TestBuild_UnparsableIncidentReturnsErrorNamingFile error-text-match has no baseline row
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

**F1.3** Cycle 2533 also added `TestBuild_UnparsableIncidentReturnsErrorNamingFile` in `kbgraph`, with the same text match. The ratchet joins its findings into one error, so only the first finding carries the `ratchet_tree_test.go:22:` prefix. The console first read only the prefixed lines, and missed this finding.
*Implication:* the fix changes that test too. Read each line of a joined error, not only the prefixed line.

**F1.4** Cycle 2540 added `TestParse_RefusesADeadlineBasisOtherThanAllOrShipped` in `policy` and `TestC2540_006` in `acs/cycle2540`. Both checked `strings.Contains(err.Error(), "deadline_basis")`. The cycle landed after PR #1572 was cut.
*Implication:* each lane that lands before the fix can add findings. The fix must be rebuilt on the main of the boundary, and the console must read the ratchet output of that main again.

**F1.5** Cycle 2542 added `TestCycleStartTrim_LaunchFnRunsFakeTrimAndGuardsDisk` in `cmd/evolve`. It called `fakeclitest.Install(t, bin, ...)` with `bin` a variable, and the cycle landed after PR #1575 was cut. The rule `unverified-fake` reads the path only from the call (`pathTail` in `go/internal/testaudit/rules.go`). It needs a `fakes.json` row for the base name, and `evolve` had none.
*Implication:* main CI reported the finding at 17:12. The console read it at the boundary rerun of the ratchet that F1.4 asks for, before #1575 merged.

**F2.1** `StaticBuildFloorChecks` and `SuiteBuildFloorChecks` (`go/internal/core/build_floor_reviewer.go:35-51`) have no test-audit check.
*Implication:* the lane floor passed both cycles. The first check that saw the finding was the CI of main, after the landing.

**F2.2** The log of each red run has more `FAIL` lines than the one real failure. Lines for `acs/cycle1946`, `acs/cycle1950` and `TestPersonaStopCriterionDedupe_*` are fixture text inside build-floor test output.
*Implication:* the console found the real failure by the `FAIL` line of a package with its own test name above it.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `incidentkb` wraps `ErrMissingMarker` and `ErrOverlappingMarkers`; `kbgraph` returns `*RecordParseError` | #1577 (replaces #1575 and #1572) | `internal/testaudit/ratchet_tree_test.go::TestTreeRatchet` (red on `e919bfd1d`, green on the fix) + `incidentkb/incidentkb_test.go::TestRender_EmptyAndErrorBranches` + `acs/cycle2533/predicates_test.go::TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph` + `kbgraph/kbgraph_test.go::TestBuild_UnparsableIncidentReturnsErrorNamingFile` |
| `policy` wraps `ErrDeadlineBasis`; a value that is not a string fails as `*json.UnmarshalTypeError` | #1577 | `internal/testaudit/ratchet_tree_test.go::TestTreeRatchet` + `policy/fleet_orchestrator_deadline_basis_test.go::TestParse_RefusesADeadlineBasisOtherThanAllOrShipped` + `policy/fleet_orchestrator_deadline_basis_test.go::TestParse_RefusesADeadlineBasisThatIsNotAString` + `acs/cycle2540/predicates_test.go::TestC2540_006_TheDeadlineBasisRefusesAnyOtherValue` |
| The cycle-start test builds its fake path at the install call; `testaudit/fakes.json` has an `evolve` row (real side `none`, with a reason) | #1577 | `internal/testaudit/ratchet_tree_test.go::TestTreeRatchet` (red on `3a466367d`, red with the row removed, green on the fix) + `cmd/evolve/cmd_cycle_start_test.go::TestCycleStartTrim_LaunchFnRunsFakeTrimAndGuardsDisk` |
| The build floor runs the test-audit ratchet on the changed packages | inbox item `build-floor-runs-the-test-audit-ratchet` (console PR) | `core/build_testaudit_check_test.go::TestStaticBuildFloorChecks_RefusesANewTestAuditFinding` |

## Limits

- PR #1577 is open. It lands at the boundary of wave 181 through `evolve integration batch --main-red-fix`. This record and its index rows name `ErrMissingMarker`, `ErrOverlappingMarkers`, `RecordParseError`, `ErrDeadlineBasis` and the `evolve` row of `fakes.json`. These are on main only after #1577 lands. Thus this record lands in the same batch as #1577, or after it.
- Lanes that land before #1577 can add more findings, as cycles 2540 and 2542 did. The boundary check reruns the tree ratchet on the composed tree.
- The inbox item is in the runtime inbox, not in git.

## What we learned

- A ratchet that only CI runs on the whole tree lets a lane turn main red. Each whole-tree ratchet needs a floor check on the changed packages, as the persona-budget check has.
- A test checks an error by its identity (`errors.Is`) or its fields (`errors.As`), never by its text.
- Read the `FAIL` line of a package and the test name above it. Many `FAIL` lines in a CI log are fixture text.
- A joined error prints its first finding after the file prefix and each other finding on its own indented line. Read all of these lines.
