# Comment history: `acs/cycle553`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle553/predicates_test.go:3` — above `package cycle553`

```text
// Package cycle553 materialises the cycle-553 acceptance criteria for this
// fleet lane's SOLE `## top_n` task per triage-report.md:
//
//	supervisor-continuous-lane-keeping — wire the cycle-550 fleet.RunPool
//	rolling-lane-pool scheduler into the actual loop dispatcher behind the
//	existing policy.fleet.scheduling knob (add the missing "pool" branch;
//	RunPool currently has zero call sites outside its own package), backfilling
//	a replacement lane on any lane exit while sibling lanes still run, honoring
//	L4 min-width and per-run cycle-state isolation.
//
// Per the AC-Materialization Contract (R9.3 "predicates bind ONLY to triage-
// committed work"), this package predicates ONLY that item. The cycle-553
// scout-report.md proposed two OTHER tasks (acsassert-hermetic-coverage-floor,
// wave-seed-min-width-one-lane); triage-report.md explicitly DROPPED both as
// sibling-lane/out-of-fleet-scope, so they get NO predicate here.
//
// Predicate strategy (behavioral-via-subprocess, the cycle-549 precedent — never
// a source grep): the dispatcher functions under test (shouldRunPool,
// dispatchPoolIteration) live in `package main` (cmd/evolve), which a leaf ACS
// package cannot import. So each predicate drives `go test ./cmd/evolve -run
// <TestName>` as a subprocess over the REAL compiled dispatcher + the in-package
// behavioral tests the TDD engineer authored this cycle
// (cmd/evolve/cmd_loop_pool_test.go), asserting (a) the targeted tests actually
// ran — closes the cycle-85 "no tests to run" degenerate trap — and (b) they
// passed (exit 0), i.e. the wiring exists and behaves. Before the Builder wires
// it, cmd/evolve's test build fails to compile (undefined shouldRunPool /
// dispatchPoolIteration), so `go test` exits non-zero and every predicate is RED
// for the right reason.
//
// In-package behavioral tests these predicates gate on:
//
//	cmd/evolve/cmd_loop_pool_test.go
//	  TestShouldRunPool_GateTable
//	  TestShouldRunWaveAndPool_MutuallyExclusive
//	  TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning
//	  TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch
//	  TestDispatchPoolIteration_WaveConfigInertNoLaunch
//	  TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches
//
// The Builder's role: add shouldRunPool / dispatchPoolIteration / poolPlanFn to
// package main and wire dispatchPoolIteration into cmd_loop.go's batch loop
// (pool branch selected before the wave branch when shouldRunPool fires). Builder
// must NOT modify the test files.
```

### `go/acs/cycle553/predicates_test.go:68` — above `func requireRanAndGreen(t *testing.T, out string, code, min int) {`

```text
// requireRanAndGreen fails the predicate unless the -run filter matched at least
// `min` tests (guards the cycle-85 "no tests to run" degenerate pass) and the
// package exited 0 with no `--- FAIL`.
```
