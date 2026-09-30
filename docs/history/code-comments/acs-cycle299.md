# Comment history: `acs/cycle299`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle299/predicates_test.go:3` — above `package cycle299`

```text
// Package cycle299 materializes the cycle-299 acceptance criteria for the three
// committed top_n tasks (scout-report.md / triage-report.md — coverage +
// reliability hardening of the concurrency campaign's leaf packages):
//
//	T1  flock-tests — add the first test file for internal/adapters/flock (the
//	    blocking cross-process lock that serializes ledger.Append CA.1 +
//	    storage.UpdateState CA.3). Two sub-criteria:
//	      (a) package statement coverage ≥ 90.0% (0% baseline — no test file).
//	      (b) the flockFn error branch is tested — encoded as Lock-function
//	          completeness: 100% of Lock means EVERY error return (MkdirAll,
//	          OpenFile, and the flockFn LOCK_EX seam) plus the release closure
//	          are exercised. Lock is the package's only function, so the floor
//	          and the completeness gate read the same number through different
//	          thresholds: 90% admits one missed error return, 100% admits none —
//	          and only the flockFn-error return + the release path push it from
//	          ~92% to 100%, so this gate is what pins the seam branch.
//	T2  runlease-write-error-coverage — extend runlease_test.go to cover Write's
//	    four atomic-write error paths (CreateTemp/Write/Close/Rename). Two
//	    sub-criteria: (a) package total ≥ 95.0% (70.6% baseline), (b) the Write
//	    function ≥ 80.0% (52.6% baseline — happy-path round-trip only today).
//	T3  sessionrecord-coverage-boost — extend sessionrecord_test.go. Two
//	    sub-criteria: (a) package total ≥ 90.0% (68.8% baseline), (b) the
//	    RunScopeToken function ≥ 90.0% (0.0% baseline — never tested; 90% forces
//	    the >8-char ULID truncation branch, scout hypothesis 3's boundary).
//
// These predicates are BEHAVIORAL (cycle-85 lesson) and follow the canonical
// coverage-floor shape proven in cycle298's C298_003: each one RUNS the real
// package test suite under -coverprofile (a real subprocess exercising real
// code paths) and gates on the measured percentage from `go tool cover -func`.
// A magic string in a source file cannot satisfy them — only Builder's new
// tests, which drive the real error branches, move the number. An EMPTY repo
// (no tests) yields 0% and fails every gate, so they are anti-no-op by
// construction. Each floor also Fatals if the package suite is not green
// (exit != 0), folding in a no-regression axis.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", 6 rows):
//
//	T1(a) flock total coverage ≥ 90%              → C299_001
//	T1(b) flockFn error branch tested (Lock=100%) → C299_002
//	T2(a) runlease total coverage ≥ 95%           → C299_003
//	T2(b) runlease Write coverage ≥ 80%           → C299_004
//	T3(a) sessionrecord total coverage ≥ 90%      → C299_005
//	T3(b) sessionrecord RunScopeToken ≥ 90%       → C299_006
//
// Floor binding (R9.3): all three packages are committed top_n tasks THIS cycle
// (triage-report.md ## top_n) — no deferred-floor predicate here.
```
