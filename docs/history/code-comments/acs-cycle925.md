# Comment history: `acs/cycle925`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle925/predicates_test.go:3` — above `package cycle925`

```text
// Package cycle925 materializes the cycle-925 acceptance criteria for this
// fleet lane's sole committed task, mechanical-scans-to-native (triage top_n
// id: mechanical-scans-to-native).
//
// Goal: convert the two mechanical scan phases (secret-leak-scan,
// flake-rerun-scan) from full LLM boots to native in-process Go phases, per the
// ship kind:"native" precedent and Rule 5 (deterministic work → code, not LLM
// cycles). The prior attempt (cycle-785) left empty placeholder packages under
// go/internal/phases/{secretleakscan,flakererunscan} and a durable eval whose
// evidence pointed at cycle-785 predicates that were never landed; this cycle
// authors the real predicates and re-points the eval.
//
// Every predicate below EXERCISES THE SYSTEM UNDER TEST — it invokes the scan
// packages' functions, the phasespec validator, or the registry loader and
// asserts on their return values. None are source-grep predicates (the cycle-85
// degenerate-predicate failure mode is avoided). RED today is a compile failure
// on the two empty packages plus the validator rejecting kind:"native"; the
// Builder makes them GREEN by implementing the packages, unblocking the
// validator, and adding the registry phases[] entries — WITHOUT modifying this
// file.
//
// SUT CONTRACT the Builder must implement (see test-report.md handoff):
//
//	package secretleakscan
//	    type Finding struct { Rule, Match string }
//	    func ScanDiff(diff string) []Finding          // scans ADDED lines of a unified git diff
//	    func Verdict(findings []Finding) string        // canonical: "PASS" (0) | "FAIL" (>=1)
//
//	package flakererunscan
//	    type Result struct { Runs, Passes, Failures int; Flaky bool }
//	    func (r Result) Verdict() string               // canonical: "PASS" (consistent-pass) | non-PASS (flaky/failed)
//	    func Rerun(runs int, attempt func(i int) bool) Result  // deterministic: identical attempt fn → identical Result
```
