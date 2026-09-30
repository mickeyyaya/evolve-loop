# Comment history: `acs/cycle348`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle348/predicates_test.go:3` — above `package cycle348`

```text
// Package cycle348 materializes the cycle-348 acceptance criteria for the two
// committed top_n tasks (triage-report.md ## top_n):
//
//	T1  skillcheck-coverage — raise go/internal/skillcheck statement coverage
//	    from 82.7% to >= 90.0% by adding table-driven unit tests for:
//	    nameMismatches (missing skills dir, invalid frontmatter, name!=dir),
//	    parallelSubtaskCount (nil raw, invalid JSON, array value), and
//	    inspect edge cases.
//
//	T2  rollback-default-fns-coverage — raise go/internal/rollback statement
//	    coverage from 86.8% to >= 92.0% by adding tests for deleteRemoteTagWith
//	    and revertAndShipWith using the existing gitexec.Fake seam, plus smoke
//	    tests that call the one-liner default* wrappers.
//
// Predicates are BEHAVIORAL (cycle-85 lesson). Coverage gates run the real
// suites under -coverprofile and assert on `go tool cover -func` output.
// No load-bearing source-grep.
//
// AC map (1:1 with triage top_n items):
//
//	T1.coverage        skillcheck package coverage >= 90.0%          → C348_001
//	T1.namemismatches  nameMismatches function coverage >= 90.0%     → C348_002
//	T2.coverage        rollback package coverage >= 92.0%            → C348_003
//	T2.revertship      revertAndShipWith function coverage >= 80.0%  → C348_004
//
// Floor binding (R9.3): T1/T2 are the two committed top_n tasks this cycle.
// Deferred items (ship-phase-no-deliverable-contract, bridge-integration-tests)
// get ZERO predicates — a floor on a deferred task starves the committed ones
// (cycle-280 lesson).
```
