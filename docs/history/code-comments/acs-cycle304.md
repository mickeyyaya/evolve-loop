# Comment history: `acs/cycle304`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle304/predicates_test.go:3` — above `package cycle304`

```text
// Package cycle304 materializes the cycle-304 acceptance criteria for the single
// committed top_n task (triage-report.md ## top_n — blocker-solo rule, ADR-0046
// Core Principle 5):
//
//	T1  declarative-floor-counter — replace prose-regex floor counting in
//	    internal/triagecap with DECLARATION-primary counting sourced from the
//	    triage-decision.json companion's committed_floors[] array, retaining the
//	    prose counter only as fallback. Closes the phantom-floor class that failed
//	    cycles 301 and 302 (the bullet contract's mandated evidence=/source=scout
//	    tokens and coverage prose collided with real package basenames, inflating
//	    the floor count and making the capacity-clamp correction unsatisfiable).
//
// These predicates are BEHAVIORAL (cycle-85 lesson; cycle281/300 pattern). The
// load-bearing gate RUNS the real internal/triagecap test suite as a subprocess
// (`go test -v -run <the five TDD pins> ./internal/triagecap/`) and asserts on the
// real `--- PASS: <name>` lines the builder's implementation produces. Those five
// tests construct companions, run the real readers, and run the real CapReviewer /
// Recorder against them — a magic string in a .go file can neither produce a named
// PASS line nor make a declaration-primary count match, and an EMPTY repo lacks
// the functions entirely (compile failure → no PASS lines → RED). The config-check
// predicate (schema + persona declare committed_floors) carries an explicit waiver:
// it is an inherent presence check on the agent-facing contract surface, auxiliary
// to the behavioral gate above.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", declarative-
// floor-counter rows):
//
//	C1 Declaration count exact          TestCountFromDeclaration        \
//	C4 Missing companion -> prose       TestCountFallbackToProse         |
//	C3 Divergence -> satisfiable corr.  TestFloorDivergenceCorrective    } C304_001
//	C6 Reviewer uses declared count     TestReviewer_UsesDeclaredFloors  |
//	C7 Recorder uses declared count     TestRecorder_DeclaredFloors     /
//	(contract surface) schema + persona declare committed_floors        -> C304_002
//
// Floor binding (R9.3): declarative-floor-counter is NOT a coverage-floor task —
// it commits zero package coverage floors this cycle, so no coverage-floor
// predicate is authored. The triage-deferred items (evalgate-floor-declarations,
// the Layer 2/3 work, ledger-1740) get ZERO predicates here.
```
