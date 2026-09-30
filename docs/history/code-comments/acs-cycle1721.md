# Comment history: `acs/cycle1721`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1721/predicates_test.go:3` — above `package cycle1721`

```text
// Package cycle1721 materializes the acceptance criteria of fleet lane
// fleet-pool-test-wallclock-flake: the pool backfill test
// (TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning in
// go/cmd/evolve/cmd_loop_pool_test.go) must not fail merely because a pool lane
// is slow to start, and any time budget it keeps must be a named, commented
// declaration rather than a bare literal.
//
// 001 is mutation-based (the cycle-1720 shape): `go test -overlay` delays every
// lane's launch callback by slowHandoffSeconds, standing in for the scheduling
// latency of a contended CI runner, and no byte of the tree changes. 002 runs
// every test in the file under -race -count=50. 003 and 004 inspect the test
// file's syntax tree, because the criteria they encode are statements about
// that source; a fixture self-test proves the inspector flags the bad shapes.
//
// Flaky-shape hygiene: every subprocess names ONE package, narrows with -run,
// and is bound to the test's own deadline; no wall-clock bound is asserted.
```
