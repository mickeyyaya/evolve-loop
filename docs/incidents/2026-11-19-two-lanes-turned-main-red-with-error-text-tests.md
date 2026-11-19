# 2026-11-19: three lanes turned main red with error-text tests

Related documents: the test-audit rules are in `go/internal/testaudit/rules.go`, the build floor is in `go/internal/core/build_floor_reviewer.go`, and the procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** the required CI of main was red from 08:01 UTC in wave 181. Three lane cycles added the findings. A red main holds the boundary batch of the green console PRs (#1570, #1561, #1564).
- **Root cause:** cycle 2530 added one test, and cycles 2533 and 2540 each added two tests, that match on the text of `err.Error()`. The test-audit rule `error-text-match` refuses such a test when its package has no baseline row. The baseline only shrinks, so a new finding is a red tree ratchet.
- **Escape:** the lane build floor does not run the test-audit ratchet. It runs the tests of the changed packages, and `internal/testaudit` is not one of them. Only the CI of main ran `TestTreeRatchet`.
- **Fix:** PR #1575 gives `incidentkb` two sentinel errors, `kbgraph` a typed `RecordParseError` and `policy` the sentinel `ErrDeadlineBasis`. The five red tests now check these with `errors.Is` and `errors.As`. PR #1575 replaces PR #1572, which fixed only the first three findings.
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

**F2.1** `StaticBuildFloorChecks` and `SuiteBuildFloorChecks` (`go/internal/core/build_floor_reviewer.go:35-51`) have no test-audit check.
*Implication:* the lane floor passed both cycles. The first check that saw the finding was the CI of main, after the landing.

**F2.2** The log of each red run has more `FAIL` lines than the one real failure. Lines for `acs/cycle1946`, `acs/cycle1950` and `TestPersonaStopCriterionDedupe_*` are fixture text inside build-floor test output.
*Implication:* the console found the real failure by the `FAIL` line of a package with its own test name above it.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `incidentkb` wraps `ErrMissingMarker` and `ErrOverlappingMarkers`; `kbgraph` returns `*RecordParseError` | #1575 (replaces #1572) | `internal/testaudit/ratchet_tree_test.go::TestTreeRatchet` (red on `e919bfd1d`, green on the fix) + `incidentkb/incidentkb_test.go::TestRender_EmptyAndErrorBranches` + `acs/cycle2533/predicates_test.go::TestC2533_004_LegacyAndUnparsableIncidentFilesDoNotBreakTheGraph` + `kbgraph/kbgraph_test.go::TestBuild_UnparsableIncidentReturnsErrorNamingFile` |
| `policy` wraps `ErrDeadlineBasis`; a value that is not a string fails as `*json.UnmarshalTypeError` | #1575 | `internal/testaudit/ratchet_tree_test.go::TestTreeRatchet` + `policy/fleet_orchestrator_deadline_basis_test.go::TestParse_RefusesADeadlineBasisOtherThanAllOrShipped` + `policy/fleet_orchestrator_deadline_basis_test.go::TestParse_RefusesADeadlineBasisThatIsNotAString` + `acs/cycle2540/predicates_test.go::TestC2540_006_TheDeadlineBasisRefusesAnyOtherValue` |
| The build floor runs the test-audit ratchet on the changed packages | inbox item `build-floor-runs-the-test-audit-ratchet` | open |

## Limits

- PR #1575 is open. It lands at the boundary of wave 181 through `evolve integration batch --main-red-fix`. This record and its index rows name `ErrMissingMarker`, `ErrOverlappingMarkers`, `RecordParseError` and `ErrDeadlineBasis`. These symbols are on main only after #1575 lands. Thus this record lands in the same batch as #1575, or after it.
- Lanes that land before #1575 can add more findings, as cycle 2540 did. The boundary check reruns the tree ratchet on the composed tree.
- The inbox item is in the runtime inbox, not in git.

## What we learned

- A ratchet that only CI runs on the whole tree lets a lane turn main red. Each whole-tree ratchet needs a floor check on the changed packages, as the persona-budget check has.
- A test checks an error by its identity (`errors.Is`) or its fields (`errors.As`), never by its text.
- Read the `FAIL` line of a package and the test name above it. Many `FAIL` lines in a CI log are fixture text.
- A joined error prints its first finding after the file prefix and each other finding on its own indented line. Read all of these lines.
