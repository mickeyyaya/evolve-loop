# Comment history: `acs/cycle298`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle298/predicates_test.go:3` — above `package cycle298`

```text
// Package cycle298 materializes the cycle-298 acceptance criteria for the two
// committed top_n tasks (scout-report.md — L3.4 GC shadow wiring + gc coverage):
//
//	T1  gc-shadow-wiring — wire EVOLVE_GC=off|shadow|enforce into the evolve
//	    loop startup. Two sub-criteria:
//	      (a) EVOLVE_GC is registered in internal/flagregistry (enum, default
//	          "off") AND docs/architecture/control-flags.md is regenerated so
//	          `evolve flags check` reports the index in sync (no drift).
//	      (b) the runGCHook loop hook discovers+plans+logs a manifest in shadow
//	          mode WITHOUT mutating the tree, applies it in enforce mode, no-ops
//	          in off mode, warns (no crash) on an invalid value, and never plans
//	          a LIVE run for deletion.
//	T2  gc-coverage-boost — internal/gc statement coverage is ≥ 95.0% (up from
//	    the 88.8% baseline; the uncovered paths are the safety-critical Apply /
//	    nowLive / protected / dirEntriesOlderThan guards).
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test:
//   - C298_001 reads the real flagregistry.All slice (the SSOT data structure,
//     not a grepped source line) AND runs the real `evolve flags check` binary,
//     so a magic string in a doc cannot satisfy it — only adding the registry
//     row AND regenerating the doc does.
//   - C298_002 runs the real white-box cmd_loop_gc tests (which call the
//     unexported runGCHook against a synthetic .evolve tree and assert on the
//     manifest file + real dir mutations) and asserts on their `--- PASS:`
//     lines, plus a full cmd/evolve suite-green regression axis.
//   - C298_003 runs `go test -coverprofile` over internal/gc and parses the
//     real total: percentage — RED at the 88.8% baseline, GREEN only once the
//     new safety-path tests land.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1(a) EVOLVE_GC registered + flags check in sync           → C298_001
//	T1(b) runGCHook shadow/off/enforce/invalid/missing/live    → C298_002 (+002b)
//	T2    internal/gc coverage ≥ 95.0%                          → C298_003
```
