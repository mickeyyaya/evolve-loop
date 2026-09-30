# Comment history: `acs/cycle552`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle552/predicates_test.go:3` — above `package cycle552`

```text
// Package cycle552 materialises the cycle-552 acceptance criteria for this
// fleet lane's sole `## top_n` task per triage-report.md:
//
//	eliminate-sequential-fallback-min-width-lane — verify and close the
//	residual test-coverage gap on the already-landed (cycle-547) min-width
//	fleet-dispatch repair. Per triage-report.md's Rationale, the repair
//	itself (forceOneLaneDispatch / dispatchIteration) is already correctly
//	implemented and independently unit-tested; the gap is that nothing
//	exercises the RunLoop call-site wiring (cmd_loop.go's batch for-loop)
//	that connects them: the `fleetCfg.Count > 1 && waveCfg.Count <= 1` guard,
//	the one-lane launcher construction, and the WARN-vs-dispatch branching
//	that decides continue-vs-sequential-fallback. That wiring could silently
//	regress (an inverted guard, or the call site deleted in an unrelated
//	refactor) without any existing test catching it.
//
// Per triage-report.md's "Fleet scope note", the scout-proposed
// runlease-pid-aware-liveness and memo-overlay-merge-activation tasks are
// OUT OF SCOPE for this fleet-scoped lane (assigned to a different lane's
// triage pass) and are NOT predicated here (AC-Materialization Contract
// R9.3: predicates bind ONLY to triage-committed `## top_n` work).
//
// Predicate strategy (mirrors cycle547/549/550): BEHAVIORAL predicates drive
// the system under test through its in-package RED tests via subprocess
// `go test`, asserting a non-degenerate pass (requireTestsRan closes the
// cycle-85 "no tests to run" trap) — never a source grep. The in-package
// tests were authored by the TDD engineer this cycle:
//
//	cmd/evolve/cmd_loop_wave_minwidth_wiring_test.go (new minWidthRepair contract)
//
// RED today: the package fails to BUILD (minWidthRepair is undefined) — a
// subprocess `go test` exits non-zero with a compile error, which is exactly
// what every predicate below asserts against. The Builder extracts
// minWidthRepair from RunLoop's inline switch (byte-identical stderr
// messages + control flow) and wires the call site through it; it must not
// modify the tests.
```
