# Comment history: `acs/cycle316`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle316/predicates_test.go:3` — above `package cycle316`

```text
// Package cycle316 materializes the cycle-316 acceptance criteria for the single
// committed top_n task (scout-report.md "Selected Tasks"):
//
//	clihealth-zero-coverage — add unit tests for the four zero-coverage functions
//	    in go/internal/clihealth/clihealth.go (Benchable, NewBenchEntry, firstLine,
//	    truncateRunes), lifting package statement coverage from the 76.2% baseline
//	    to >= 90%. The four functions are the bench-record composition core: the
//	    runner's bench-writer and the loop's canary both compose entries via
//	    NewBenchEntry (→ firstLine → truncateRunes), and llmroute consults Benchable
//	    to decide whether a classified wall benches the whole CLI family. They were
//	    the only 0.0% rows in `go tool cover -func` for this package.
//
// These predicates are BEHAVIORAL (cycle-85 lesson) and follow the canonical
// coverage-floor shape proven in cycle298/299/300: the coverage gates RUN the real
// internal/clihealth test suite under -coverprofile (a real subprocess exercising
// real code paths) and assert on the measured percentage from `go tool cover
// -func`. A magic string in a source file cannot move the number — only Builder's
// new tests, which actually call the functions, can. An EMPTY repo (no tests)
// yields 0% and fails every floor, so they are anti-no-op by construction. The
// negative-contract gate (C316_004) imports clihealth and calls Benchable directly
// — the strongest behavioral anti-no-op for a one-statement predicate that
// coverage alone cannot force.
//
// coverFuncOutput Fatals (RED) if the clihealth suite does not compile or any test
// FAILs, so every coverage gate folds in the no-regression axis (AC4).
//
// AC map (1:1 with scout-report.md Task 1 "Acceptance criteria", 4 ACs):
//
//	AC1 coverage >= 90%                          → C316_001 (+ green-suite gate = AC4)
//	AC2 none of the 4 functions at 0.0%          → C316_002
//	AC3 edge case (firstLine empty / truncate    → C316_003 (firstLine=100% ∧
//	    multi-byte rune boundary)                       truncateRunes=100%, branch-forcing)
//	AC3 negative case (Benchable rejects          → C316_004 (direct Benchable call)
//	    non-rate_limit)
//	AC4 existing tests still pass                  → green-suite gate shared by
//	                                                  C316_001/002/003 (DRY — no
//	                                                  duplicate suite run)
//
// Floor binding (R9.3): internal/clihealth is the ONLY committed top_n task this
// cycle, so the coverage floor binds a committed package. The triage-deferred
// adapters/ledger seal.go coverage target (scout "Deferred") gets ZERO predicates
// here — authoring a floor on a deferred task would starve the committed one
// (cycle-280 lesson).
```
