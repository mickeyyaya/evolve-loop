# 2026-11-20: cycle 2558 turned main red a third time

Related documents: this is the third main-red incident of this class in 2 days. The first is [2026-11-19: four lanes turned main red with test-audit findings](2026-11-19-two-lanes-turned-main-red-with-error-text-tests.md). The second is [2026-11-20: a lane shipped error-text tests and turned main red](2026-11-20-a-lane-shipped-error-text-tests-and-turned-main-red.md). The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** cycle 2558 shipped `dad6bdf34` to main at 11:44 UTC. The test-audit tree ratchet failed on main at `aa909f868`. The main-red stop parked the lanes that were at ship, among them cycles 2563 and 2565.
- **Root cause:** the ACS predicates `TestC2558_005` and `TestC2558_006` matched the text of an error. The lane build floor did not run the test-audit ratchet, so the lane did not see the finding. This is the same escape as in the two earlier records.
- **Fix:** PR #1600 types the two errors (`gc.ErrBranchKept` and `gc.ErrTreeKept`), and the predicates use `errors.Is`. The console landed it alone through `evolve integration batch --main-red-fix 1600`, as batch PR #1602. Main moved to `93775a479`.
- **Lesson:** the class fix of 2026-11-19 was still an inbox item when the third incident came. The class fix is PR #1597 (build floor runs the ratchet). It landed through batch PR #1604 at main 30915d28c.

## Terms used throughout

- **Main-red stop:** a state file that parks the lane ships while main CI is red. A batch with `--main-red-fix` releases it.
- **Test-audit tree ratchet:** `TestTreeRatchet` in `go/internal/testaudit`. It fails on a weak test pattern that has no baseline row.
- **Error-text-match:** a test that compares `err.Error()` with a string literal or a string matcher.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 11:44 | Cycle 2558 ships `dad6bdf34`. | `git log origin/main` (commit time 19:44 +08:00) |
| 13:33 | Main moves to `aa909f868`. The red key is `TestTreeRatchet` for the cycle 2558 rows only. The other FAIL lines in the job log are fixture text. | `git log origin/main`; console note of the main-red stop |
| 13:46 | The main-red stop parks the ship of cycle 2563. | `signals.ndjson:11390` (`ORCHESTRATOR_PHASE_ABORTED`) |
| 14:02 | Batch PR #1602 lands PR #1600. Main is `93775a479`. | `git log origin/main` (commit time 22:02 +08:00) |
| 14:30 | The stop is released. | `signals.ndjson:11438` (`GATE_REPO_MAIN_RED_STOP_RELEASED`) |

The old predicates at `aa909f868`:

```text
acs/cycle2558/predicates_test.go:213: !strings.Contains(err.Error(), "branch kept")
acs/cycle2558/predicates_test.go:232: !strings.Contains(err.Error(), "tree kept")
```

## Root cause

**F1.1** The two predicates told the two refusals apart by the text of the message. `go/internal/gc/branchsweep.go` and `go/internal/gc/salvage.go` had no sentinel error for them.
*Implication:* the test-audit ratchet refuses each test. A new baseline row is not an option.

**F1.2** The lane build floor does not run the test-audit ratchet. Main CI is the first run that sees the finding.
*Implication:* before PR #1597 landed, each wave turned main red again if a lane added a finding. Cycles 2530, 2533, 2540, 2542, 2554 and 2558 show the same escape.

**F1.3** A mid-wave main-red fix is landed alone, because the stop parks the lane ships. The landing moved origin and not the runtime copy of main. See [2026-11-20: a mid-wave main-red landing stranded lane ships](2026-11-20-a-mid-wave-main-red-landing-stranded-lane-ships.md).

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `gc.ErrBranchKept` (`go/internal/gc/branchsweep.go:12`) and `gc.ErrTreeKept` (`go/internal/gc/salvage.go:15`) wrap the refusals. The predicates use `errors.Is` (`go/acs/cycle2558/predicates_test.go:214`, `:233`). | #1600 | `testaudit/ratchet_tree_test.go::TestTreeRatchet` (red on `aa909f868`, green on `93775a479`) |
| The build floor runs the test-audit ratchet | #1597 (landed through batch PR #1604 at main 30915d28c) | `testaudit` ratchet check in the floor; `core/build_testaudit_check_test.go::TestStaticBuildFloorChecks_RefusesANewTestAuditFinding` |
| The ship backstop runs the ratchet | inbox item `ship-backstop-runs-the-test-audit-ratchet` | open |

## Limits

- Before PR #1597 landed, a lane shipped a new test-audit finding with no check.
- The red main runs stay red in the history.
- The cost in lane hours is not measured.

## What we learned

- Read the failed step names and the package FAIL lines before you act. Most FAIL lines in a main CI log are fixture text.
- A recurrence of an open class-fix item raises that item to the first build of the next wave.
