# Comment history: `acs/cycle300`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle300/predicates_test.go:3` — above `package cycle300`

```text
// Package cycle300 materializes the cycle-300 acceptance criteria for the two
// committed top_n tasks (scout-report.md / triage-report.md — error-path +
// safety-gate coverage of the post-L3.4 reliability surface):
//
//	T1  gc-error-path-coverage — add targeted tests for internal/gc's dark error
//	    paths. Two sub-criteria (scout "Acceptance Criteria Summary"):
//	      (a) the 5 new error-path tests exercise their branches — encoded as
//	          Plan = 100% AND dirEntriesOlderThan = 100%. Plan's only dark blocks
//	          are the EvolveDir-absolute guard (gc.go:125-127) and the nil-Now
//	          wall-clock fallback (gc.go:130-132); dirEntriesOlderThan's only dark
//	          block is the filter-reject callback (gc.go:204-205). Only the guard /
//	          nil-Now / filter-reject tests push these two functions to 100%, so a
//	          dual 100% floor is the precise behavioral encoding of "the error-path
//	          tests exist and drive their branches" — strictly stronger than
//	          "5 tests pass", unsatisfiable by a no-op.
//	      (b) package statement coverage >= 98.0% (95.0% baseline). The 98% floor
//	          additionally forces the Discover missing-runs branch (discover.go:
//	          81-83) and the currentWorkspace read-error branch (discover.go:
//	          141-143) — 5 of the 8 coverable dark blocks (the remaining 3 —
//	          e.Info race, Apply RemoveAll/Rename — are triage-deferred, so 98.14%
//	          is the reachable ceiling and 98.0% leaves a one-block margin).
//	T2  looppreflight-coverage-boost — add targeted tests for the loop readiness
//	    gate's dark safety seams. Two sub-criteria:
//	      (a) the 6 new safety-gate tests exercise their branches — encoded as
//	          CheckLevel.String = 100% (the out-of-range "unknown" branch,
//	          looppreflight.go:60-61) AND checkPipelineStructure = 100% (the three
//	          Halt-accumulation gaps: missing factory gc.go... checks.go:30-32,
//	          profileLister error checks.go:39-41, profileGetter error checks.go:
//	          44-46). checkPipelineStructure is THE gate that refuses to start a
//	          loop whose static wiring is broken, so pinning it to 100% guarantees
//	          every gap-accumulation arm is driven.
//	      (b) package statement coverage >= 93.0% (88.7% baseline). The deferred
//	          newDefaultBootTester closure (14 dark stmts, needs an fs seam) and
//	          the dead PrettyJSON error branch are excluded, leaving 25 coverable
//	          dark stmts (95.76% ceiling); 93.0% needs 16 of them.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) and follow the canonical
// coverage-floor shape proven in cycle298/299: each one RUNS the real package
// test suite under -coverprofile (a real subprocess exercising real code paths)
// and gates on the measured percentage from `go tool cover -func`. A magic
// string in a source file cannot satisfy them — only Builder's new tests, which
// drive the real error branches, move the number. An EMPTY repo (no tests)
// yields 0% and fails every gate, so they are anti-no-op by construction. Each
// floor also Fatals if the package suite is not green (exit != 0), folding in a
// no-regression axis.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", 4 rows):
//
//	T1(a) 5 new gc tests pass  (Plan=100% ∧ dirEntriesOlderThan=100%) → C300_001
//	T1(b) gc total coverage >= 98%                                    → C300_002
//	T2(a) 6 new lpf tests pass (String=100% ∧ checkPipelineStructure=100%) → C300_003
//	T2(b) looppreflight total coverage >= 93%                         → C300_004
//
// Floor binding (R9.3): both packages are committed top_n tasks THIS cycle
// (triage-report.md ## top_n). The triage-deferred items (gc-toctou-delete-error,
// looppreflight-boot-tester-integration) get ZERO predicates here — the floors
// are reachable WITHOUT their branches (gc 98.14% / lpf 95.76% ceilings).
```
