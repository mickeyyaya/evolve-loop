# Comment history: `acs/cycle329`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle329/predicates_test.go:3` — above `package cycle329`

```text
// Package cycle329 materializes the cycle-329 acceptance criteria for the two
// committed top_n tasks (scout-report.md "## Selected Tasks"):
//
//	verdictcache-write-error-coverage — raise go/internal/verdictcache statement
//	    coverage from the 79.5% baseline to the committed >= 93.0% floor by
//	    exercising the dark error exits of (*Store).write (mkdir + write-temp)
//	    and (*Store).Load (the non-IsNotExist read-error arm), plus the
//	    NewStore nil-now default. The json.MarshalIndent and os.Rename error
//	    arms of write are not in the committed plan, so ~82% — not 100% — is the
//	    practical write() ceiling and the C329_003 floor is set accordingly.
//
//	triagecap-uncovered-fns-coverage — raise go/internal/triagecap statement
//	    coverage from the 86.8% baseline to the committed >= 93.0% floor by
//	    adding direct tests for the four 0.0%-coverage functions: NewReviewer,
//	    readFailedApproaches, CommittedFloorPackages, readWindow.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The coverage gates RUN the
// real package suites in a subprocess under -coverprofile and assert on the
// measured `go tool cover -func` percentages; the suite gates RUN the suites
// under -race and assert on the measured exit code. There is no load-bearing
// source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's new tests can — so the coverage gates are anti-no-op by
//     construction.
//   - An EMPTY repo (no package tests) yields 0% and fails every floor.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis.
//   - C329_006 is the explicit zero-coverage axis (adversarial-testing SKILL
//     §6): each of the four named functions is at 0.0% today, so the gate is RED
//     until Builder's direct tests exercise them — a package-% game that left
//     any of the four dark could not satisfy it.
//
// AC map (1:1 with the scout-report.md per-task "Acceptance Criteria" — the
// "no production code modified" / "only the new test file is added" criteria are
// dispositioned manual+checklist for the Auditor in test-report.md, not as a
// fragile git-diff predicate whose result depends on the harness's phase-commit
// timing):
//
//	verdictcache-write-error-coverage
//	  AC1 package coverage >= 93.0%            → C329_001
//	  AC2 all tests pass (incl. -race)         → C329_002
//	  (anchor) write() error exits exercised   → C329_003
//	triagecap-uncovered-fns-coverage
//	  AC1 package coverage >= 93.0%            → C329_004
//	  AC2 all tests pass (incl. -race)         → C329_005
//	  (anchor) the four 0%-cov funcs exercised → C329_006
//
// Floor binding (R9.3): internal/verdictcache and internal/triagecap are the two
// committed top_n tasks this cycle, so the coverage floors bind committed
// packages. The scout-DEFERRED items (triagecap.init / newCapReviewer,
// adapters/bridge, adapters/ledger, releasepipeline) get ZERO predicates here —
// a floor on a deferred task would starve the committed ones (cycle-280 lesson).
```

### `go/acs/cycle329/predicates_test.go:239` — above `func TestC329_006_TriagecapZeroCovFunctionsExercised(t *testing.T) {`

```text
// --- C329_006 (anchor for AC1): the four 0%-coverage functions are exercised --
//
// Behavioral zero-coverage axis (adversarial-testing SKILL §6). Each of the four
// scout-named functions is at 0.0% today. The gate requires each to clear a 50%
// floor — comfortably reachable by the planned direct tests, but impossible at
// the 0.0% baseline and impossible to satisfy by raising the package % through
// unrelated code. This is the load-bearing anti-no-op binding for task 2: it
// pins the % movement to the TARGET dark functions, not to incidental coverage.
```
