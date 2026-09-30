# Comment history: `acs/cycle320`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle320/predicates_test.go:3` — above `package cycle320`

```text
// Package cycle320 materializes the cycle-320 acceptance criteria for the one
// committed top_n task (triage-report.md "## top_n"):
//
//	phasecoherence-coverage-gaps — raise go/internal/phasecoherence statement
//	    coverage from the 85.2% baseline to the committed >= 90.0% floor by
//	    exercising the dark branches of canonicalRole (42.9% — only the default
//	    lower-casing arm is hit; the seven exact-match arms scout/builder/build/
//	    auditor/audit/intent/memo are dark) and dispatchNone (75.0% — the happy
//	    `dispatch: none` arm is reached via TestCoherence_DispatchNonePersonaExempt
//	    but the error / parse-fail / non-"none" arms are dark), while every
//	    existing phasecoherence test stays green.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). They RUN the real
// phasecoherence test suite in a subprocess under -coverprofile and assert on
// the measured `go tool cover -func` percentages — there is no source-grep:
//
//   - A magic string in a source file cannot move a coverage number. Only
//     Builder's new tests, which actually CALL canonicalRole / dispatchNone with
//     the previously-dark inputs, can.
//   - An EMPTY repo (no tests) yields 0% and fails every floor, so the gates are
//     anti-no-op by construction.
//   - coverFuncOutput Fatals (RED) if the suite does not compile or any test
//     FAILs, so every coverage gate folds in the no-regression axis (AC4).
//   - The per-function 100.0% floors are the ADVERSARIAL axis (adversarial-testing
//     SKILL §6): canonicalRole and dispatchNone are small enough that 100% is
//     reachable, so the floor forces the NEGATIVE/edge cases — dispatchNone's
//     error & non-"none" rejection arms, every exact-match arm of canonicalRole —
//     not just an aggregate number a happy-path-only suite could reach.
//
// AC map (1:1 with the scout-report.md "Acceptance Criteria" list — 4 ACs):
//
//	phasecoherence-coverage-gaps
//	  AC1 package coverage >= 90.0%                      → C320_001
//	  AC2 canonicalRole exact-match branches exercised   → C320_002 (func 100%)
//	  AC3 dispatchNone reached for dispatch:none + arms  → C320_003 (func 100%)
//	  AC4 no regression (existing tests stay green)      → C320_004
//
// Floor binding (R9.3): internal/phasecoherence is the committed top_n task this
// cycle, so the coverage floor (C320_001/002/003) binds a committed package. The
// scout-DEFERRED items (cmd/evolve, modelcatalog Write) get ZERO predicates here
// — authoring a floor on a deferred task would starve the committed one
// (cycle-280 lesson).
```
